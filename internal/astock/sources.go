package astock

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// ── 端点常量（东方财富 / 巨潮资讯）─────────────────────────────

const (
	emListAPI   = "https://np-listapi.eastmoney.com/comm/web/getListInfo"     // 个股资讯（secid 型）
	emSearchAPI = "https://search-api-web.eastmoney.com/search/jsonp"          // stock_news_em 型检索（正文补充）
	emCListURL  = "https://push2.eastmoney.com/api/qt/clist/get"               // 全量列表（备源）
	cninfoQuery = "http://www.cninfo.com.cn/new/hisAnnouncement/query"         // 公告列表
	cninfoStockListURL = "http://www.cninfo.com.cn/new/data/szse_stock.json"   // 全量列表（主源，含 orgId）
	cninfoStatic       = "http://static.cninfo.com.cn/"                        // 公告 PDF 前缀

	defaultUA = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 " +
		"(KHTML, like Gecko) Chrome/126.0.0.0 Safari/537.36 news-report-astock/1.0"

	maxRetryAttempts = 3                // 1 次首发 + ≤2 次重试
	retryBaseBackoff = 700 * time.Millisecond
	maxBodyBytes     = 32 << 20
)

// Source 是 A 股数据源客户端（东方财富 + 巨潮资讯）。
// 代理：bai 网关场景走 HTTPS_PROXY；行情/公告源国内可直连，http.ProxyFromEnvironment
// 对两者均生效（本机代理对国内站点通常直连规则放行，不影响）。
type Source struct {
	http *http.Client
	ua   string
	logf func(format string, args ...any)

	// 以下 URL 仅供测试替换（httptest），生产用常量。
	EMListAPI   string
	EMSearchAPI string
	EMCListURL  string
	CNQuery     string
	CNStockList string
}

// NewSource 创建数据源客户端。HTTP 客户端经 http.ProxyFromEnvironment 自动识别
// HTTP(S)_PROXY / NO_PROXY 环境变量。
func NewSource() *Source {
	return &Source{
		http: &http.Client{
			Timeout: 20 * time.Second,
			Transport: &http.Transport{
				Proxy:               http.ProxyFromEnvironment,
				TLSHandshakeTimeout: 10 * time.Second,
			},
		},
		ua:   defaultUA,
		logf: func(string, ...any) {},
	}
}

// SetLogf 注入日志函数（失败降级时记日志）。
func (s *Source) SetLogf(f func(format string, args ...any)) {
	if f != nil {
		s.logf = f
	}
}

// do 发起请求：指数退避重试（429/5xx/网络错误），其余 4xx 不重试。
// 失败由调用方优雅降级。
func (s *Source) do(ctx context.Context, req *http.Request) ([]byte, error) {
	req.Header.Set("User-Agent", s.ua)
	req.Header.Set("Accept", "application/json, text/plain, */*")
	var lastErr error
	backoff := retryBaseBackoff
	for attempt := 1; attempt <= maxRetryAttempts; attempt++ {
		if attempt > 1 {
			s.logf("astock: 第 %d 次重试 %s（退避 %s）", attempt, req.URL.Host, backoff)
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(backoff):
			}
			backoff *= 2 // 指数退避 0.7s → 1.4s
		}
		resp, err := s.http.Do(req)
		if err != nil {
			lastErr = fmt.Errorf("网络错误: %w", err)
			s.logf("astock: %s 请求失败（第 %d 次）: %v", req.URL, attempt, lastErr)
			continue
		}
		body, err := io.ReadAll(io.LimitReader(resp.Body, maxBodyBytes))
		resp.Body.Close()
		if err != nil {
			lastErr = fmt.Errorf("读取响应失败: %w", err)
			s.logf("astock: %s 响应读取失败（第 %d 次）: %v", req.URL, attempt, lastErr)
			continue
		}
		sc := resp.StatusCode
		switch {
		case sc >= 200 && sc < 300:
			return body, nil
		case sc == http.StatusTooManyRequests || sc >= 500:
			lastErr = fmt.Errorf("HTTP %d", sc)
			s.logf("astock: %s 返回 %d（第 %d 次，可重试）", req.URL.Host, sc, attempt)
		default:
			// 其余 4xx 不重试
			return nil, fmt.Errorf("astock: HTTP %d（4xx 不重试）", sc)
		}
	}
	return nil, lastErr
}

func (s *Source) get(ctx context.Context, rawURL string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, err
	}
	return s.do(ctx, req)
}

func (s *Source) postForm(ctx context.Context, rawURL string, form url.Values, referer string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, rawURL, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded; charset=UTF-8")
	req.Header.Set("X-Requested-With", "XMLHttpRequest")
	if referer != "" {
		req.Header.Set("Referer", referer)
	}
	return s.do(ctx, req)
}

// emListURL / emSearchURL / cnQuery / cnStock 返回生效 URL（测试覆盖优先）。
func (s *Source) emListURL() string {
	if s.EMListAPI != "" {
		return s.EMListAPI
	}
	return emListAPI
}
func (s *Source) emSearchURL() string {
	if s.EMSearchAPI != "" {
		return s.EMSearchAPI
	}
	return emSearchAPI
}
func (s *Source) cnQueryURL() string {
	if s.CNQuery != "" {
		return s.CNQuery
	}
	return cninfoQuery
}
func (s *Source) cnStockListURL() string {
	if s.CNStockList != "" {
		return s.CNStockList
	}
	return cninfoStockListURL
}
func (s *Source) emCList() string {
	if s.EMCListURL != "" {
		return s.EMCListURL
	}
	return emCListURL
}

// ── 东方财富个股新闻（stock_news_em 型）────────────────────────

type emListResp struct {
	Code int    `json:"code"`
	Msg  string `json:"message"`
	Data struct {
		Total int `json:"totle_hits"` // 上游拼写如此
		List  []struct {
			ShowTime string `json:"Art_ShowTime"`
			Code     string `json:"Art_Code"`
			Title    string `json:"Art_Title"`
			URL      string `json:"Art_Url"`
		} `json:"list"`
	} `json:"data"`
}

type emSearchItem struct {
	Date      string `json:"date"`
	Code      string `json:"code"`
	Title     string `json:"title"`
	Content   string `json:"content"`
	MediaName string `json:"mediaName"`
	URL       string `json:"url"`
}

type emSearchResp struct {
	Code   int    `json:"code"`
	Result struct {
		CMSArticle []emSearchItem `json:"cmsArticleWebOld"`
	} `json:"result"`
}

// FetchNews 抓取个股新闻（东方财富，stock_news_em 型）：
//   - 主列表 getListInfo（secid=1.沪 / 0.深 分辨市场）：个股资讯流，相关性高（无正文字段，text=标题）；
//   - 检索 search-api-web（keyword=代码，即 akshare stock_news_em 同源）：带正文摘要与媒体名；
//   - 按 URL/标题去重合并；检索失败仅记日志不阻塞。任一必需步骤失败时返回错误，
//     调用方优雅降级（跳过该 sym 的该源）。
func (s *Source) FetchNews(ctx context.Context, sym string, limit int) ([]NewsItem, error) {
	secid, err := SecID(sym)
	if err != nil {
		return nil, err
	}
	if limit <= 0 {
		limit = 10
	}
	listURL := fmt.Sprintf("%s?client=web&mTypeAndCode=%s&type=1&pageSize=%d&pageIndex=1",
		s.emListURL(), url.QueryEscape(secid), limit)
	body, err := s.get(ctx, listURL)
	if err != nil {
		return nil, fmt.Errorf("astock: 东财新闻(%s)失败: %w", sym, err)
	}
	var lr emListResp
	if err := json.Unmarshal(body, &lr); err != nil {
		return nil, fmt.Errorf("astock: 东财新闻(%s)解析失败: %w", sym, err)
	}
	if lr.Code != 1 || lr.Data.List == nil {
		return nil, fmt.Errorf("astock: 东财新闻(%s)接口返回异常: code=%d msg=%q", sym, lr.Code, lr.Msg)
	}

	seen := map[string]bool{}
	items := make([]NewsItem, 0, 2*limit)
	add := func(it NewsItem) {
		if it.URL == "" || seen["u:"+it.URL] || seen["t:"+it.Title] {
			return
		}
		seen["u:"+it.URL] = true
		seen["t:"+it.Title] = true
		items = append(items, it)
	}

	// ① 主列表（个股资讯流，text=标题）
	for _, it := range lr.Data.List {
		add(NewsItem{
			Date: emDate(it.ShowTime), Sym: sym, SourceType: SourceTypeNews,
			Title: stripEm(it.Title), Text: stripEm(it.Title), URL: it.URL,
		})
	}

	// ② 检索补充（stock_news_em 型，带正文/媒体名；best-effort）
	sr, err := s.searchNews(ctx, sym, limit)
	if err != nil {
		s.logf("astock: 东财正文检索(%s)失败（仅主列表降级）: %v", sym, err)
	} else {
		for _, c := range sr {
			title := stripEm(c.Title)
			text := stripEm(c.Content)
			if text == "" {
				text = title
			}
			add(NewsItem{
				Date: emDate(c.Date), Sym: sym, SourceType: SourceTypeNews,
				Title: title, Text: text, URL: c.URL, Media: c.MediaName,
			})
		}
	}
	return items, nil
}

// searchNews 调 search-api-web jsonp 检索。
func (s *Source) searchNews(ctx context.Context, sym string, limit int) ([]emSearchItem, error) {
	_, code, ok := SplitSym(sym)
	if !ok {
		return nil, fmt.Errorf("非法 sym %q", sym)
	}
	param := fmt.Sprintf(`{"uid":"","keyword":%q,"type":["cmsArticleWebOld"],"client":"web","clientType":"web","clientVersion":"curr","param":{"cmsArticleWebOld":{"searchScope":"default","sort":"default","pageIndex":1,"pageSize":%d,"preTag":"<em>","postTag":"</em>"}}}`,
		code, limit)
	u := fmt.Sprintf("%s?cb=astockCb&param=%s", s.emSearchURL(), url.QueryEscape(param))
	body, err := s.get(ctx, u)
	if err != nil {
		return nil, err
	}
	inner, err := stripJSONP(string(body))
	if err != nil {
		return nil, err
	}
	var sr emSearchResp
	if err := json.Unmarshal([]byte(inner), &sr); err != nil {
		return nil, err
	}
	if sr.Code != 0 {
		return nil, fmt.Errorf("检索接口 code=%d", sr.Code)
	}
	return sr.Result.CMSArticle, nil
}

// stripJSONP 剥掉 "cb(...)" 包裹。
func stripJSONP(s string) (string, error) {
	i := strings.IndexByte(s, '(')
	j := strings.LastIndexByte(s, ')')
	if i < 0 || j <= i {
		return "", errors.New("astock: 响应不是合法 JSONP")
	}
	return s[i+1 : j], nil
}

// stripEm 去掉东财/巨潮标题高亮标签 <em></em>。
func stripEm(s string) string {
	return strings.NewReplacer("<em>", "", "</em>", "").Replace(strings.TrimSpace(s))
}

// emDate 从 "2026-09-12 02:40:07"（北京时间）取日期部分。
func emDate(showTime string) string {
	if len(showTime) >= 10 {
		return showTime[:10]
	}
	return showTime
}

// ── 巨潮资讯公告列表 ──────────────────────────────────────────

type cnQueryResp struct {
	TotalAnnouncement int `json:"totalAnnouncement"`
	Announcements     []struct {
		SecCode           string `json:"secCode"`
		SecName           string `json:"secName"`
		AnnouncementTitle string `json:"announcementTitle"`
		AdjunctURL        string `json:"adjunctUrl"`
		AnnouncementTime  int64  `json:"announcementTime"` // ms 时间戳
	} `json:"announcements"`
}

// beijingTZ 北京时间（公告时间戳的展示时区）。
var beijingTZ = time.FixedZone("CST", 8*3600)

// FetchAnnouncements 抓取巨潮公告列表。orgID 为空时返回错误（无法构造查询）。
// column 按市场选择（szse/sse），实测两值均可跨市返回，此处按规范传。
func (s *Source) FetchAnnouncements(ctx context.Context, sym, orgID string, limit int) ([]NewsItem, error) {
	_, code, ok := SplitSym(sym)
	if !ok {
		return nil, fmt.Errorf("astock: 非法 sym %q", sym)
	}
	if orgID == "" {
		return nil, fmt.Errorf("astock: %s 缺少巨潮 orgId，无法查询公告", sym)
	}
	if limit <= 0 {
		limit = 10
	}
	column := "szse"
	m, _, _ := SplitSym(sym)
	if m == MarketSH {
		column = "sse"
	}
	form := url.Values{
		"pageNum":     {"1"},
		"pageSize":    {fmt.Sprint(limit)},
		"column":      {column},
		"tabName":     {"fulltext"},
		"plate":       {""},
		"stock":       {code + "," + orgID},
		"searchkey":   {""},
		"secid":       {""},
		"category":    {""},
		"trade":       {""},
		"seDate":      {""},
		"sortName":    {""},
		"sortType":    {""},
		"isHLtitle":   {"true"},
	}
	body, err := s.postForm(ctx, s.cnQueryURL(), form, "http://www.cninfo.com.cn/new/commonUrl/pageOfSearch?url=disclosure/list/search")
	if err != nil {
		return nil, fmt.Errorf("astock: 巨潮公告(%s)失败: %w", sym, err)
	}
	var qr cnQueryResp
	if err := json.Unmarshal(body, &qr); err != nil {
		return nil, fmt.Errorf("astock: 巨潮公告(%s)解析失败: %w", sym, err)
	}
	items := make([]NewsItem, 0, len(qr.Announcements))
	for _, a := range qr.Announcements {
		if a.AnnouncementTitle == "" || a.AdjunctURL == "" {
			continue
		}
		items = append(items, NewsItem{
			Date:       time.UnixMilli(a.AnnouncementTime).In(beijingTZ).Format("2006-01-02"),
			Sym:        sym,
			SourceType: SourceTypeAnnouncement,
			Title:      stripEm(a.AnnouncementTitle),
			Text:       stripEm(a.AnnouncementTitle), // 公告仅标题（正文为 PDF，不解析）
			URL:        cninfoStatic + a.AdjunctURL,
		})
	}
	return items, nil
}
