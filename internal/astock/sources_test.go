package astock

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

func newTestSource() *Source {
	s := NewSource()
	var logs []string
	s.SetLogf(func(f string, a ...any) { logs = append(logs, f) })
	_ = logs
	return s
}

// TestGetRetryOn5xxThenSuccess 5xx → 指数退避重试 → 成功。
func TestGetRetryOn5xxThenSuccess(t *testing.T) {
	var hits int32
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if atomic.AddInt32(&hits, 1) <= 2 {
			w.WriteHeader(http.StatusBadGateway) // 502
			return
		}
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer ts.Close()
	s := newTestSource()
	body, err := s.get(context.Background(), ts.URL)
	if err != nil {
		t.Fatalf("期望重试后成功: %v", err)
	}
	if string(body) != `{"ok":true}` {
		t.Errorf("body = %q", body)
	}
	if got := atomic.LoadInt32(&hits); got != 3 {
		t.Errorf("请求次数 = %d, want 3", got)
	}
}

// TestGetNoRetryOn4xx 其余 4xx 不重试。
func TestGetNoRetryOn4xx(t *testing.T) {
	var hits int32
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&hits, 1)
		w.WriteHeader(http.StatusForbidden)
	}))
	defer ts.Close()
	s := newTestSource()
	_, err := s.get(context.Background(), ts.URL)
	if err == nil || !strings.Contains(err.Error(), "403") {
		t.Fatalf("期望 403 错误: %v", err)
	}
	if got := atomic.LoadInt32(&hits); got != 1 {
		t.Errorf("请求次数 = %d, want 1（4xx 不重试）", got)
	}
}

// TestGetRetryOn429 429 可重试。
func TestGetRetryOn429(t *testing.T) {
	var hits int32
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if atomic.AddInt32(&hits, 1) == 1 {
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		_, _ = w.Write([]byte(`ok`))
	}))
	defer ts.Close()
	s := newTestSource()
	if _, err := s.get(context.Background(), ts.URL); err != nil {
		t.Fatalf("429 后重试应成功: %v", err)
	}
	if got := atomic.LoadInt32(&hits); got != 2 {
		t.Errorf("请求次数 = %d, want 2", got)
	}
}

// TestGetExhausted 重试耗尽返回错误（优雅降级入口）。
func TestGetExhausted(t *testing.T) {
	var hits int32
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&hits, 1)
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer ts.Close()
	s := newTestSource()
	_, err := s.get(context.Background(), ts.URL)
	if err == nil {
		t.Fatal("期望耗尽后报错")
	}
	if got := atomic.LoadInt32(&hits); got != maxRetryAttempts {
		t.Errorf("请求次数 = %d, want %d", got, maxRetryAttempts)
	}
}

// TestGetSendsUA 必须带 UA 头。
func TestGetSendsUA(t *testing.T) {
	var ua string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ua = r.Header.Get("User-Agent")
		_, _ = w.Write([]byte(`{}`))
	}))
	defer ts.Close()
	s := newTestSource()
	if _, err := s.get(context.Background(), ts.URL); err != nil {
		t.Fatal(err)
	}
	if ua == "" || strings.Contains(ua, "Go-http-client") {
		t.Errorf("UA 头异常: %q", ua)
	}
}

// TestStripJSONP jsonp 剥壳。
func TestStripJSONP(t *testing.T) {
	in := `astockCb({"code":0,"msg":"OK","result":{}})`
	out, err := stripJSONP(in)
	if err != nil || !strings.HasPrefix(out, `{"code"`) {
		t.Errorf("stripJSONP = %q, %v", out, err)
	}
	if _, err := stripJSONP("not jsonp"); err == nil {
		t.Error("非 jsonp 应报错")
	}
}

// TestFetchNewsMergeDedup 主列表与检索条目去重合并（同 URL 去重、异 URL 都保留）。
func TestFetchNewsMergeDedup(t *testing.T) {
	listBody := `{"code":1,"message":"success","data":{"totle_hits":2,"list":[
		{"Art_ShowTime":"2026-09-12 02:40:07","Art_Code":"A1","Art_Title":"<em>标题一</em>","Art_Url":"http://finance.eastmoney.com/a/1.html"},
		{"Art_ShowTime":"2026-09-11 22:48:05","Art_Code":"A2","Art_Title":"标题二","Art_Url":"http://finance.eastmoney.com/a/2.html"}]}}`
	searchBody := `astockCb({"code":0,"msg":"OK","result":{"cmsArticleWebOld":[
		{"date":"2026-09-12 03:40:00","code":"B1","title":"标题一","content":"正文内容甲","mediaName":"证券时报网","url":"http://finance.eastmoney.com/a/1.html"},
		{"date":"2026-09-12 04:40:00","code":"B2","title":"<em>检索</em>标题三","content":"正文内容乙","mediaName":"财联社","url":"http://finance.eastmoney.com/a/3.html"}]}})`
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasPrefix(r.URL.Path, "/comm/web/getListInfo"):
			if !strings.Contains(r.URL.RawQuery, "mTypeAndCode=0.000166") {
				t.Errorf("secid 错误: %s", r.URL.RawQuery)
			}
			_, _ = w.Write([]byte(listBody))
		case strings.HasPrefix(r.URL.Path, "/search/jsonp"):
			_, _ = w.Write([]byte(searchBody))
		default:
			t.Errorf("意外路径: %s", r.URL.Path)
		}
	}))
	defer ts.Close()
	s := newTestSource()
	s.EMListAPI = ts.URL + "/comm/web/getListInfo"
	s.EMSearchAPI = ts.URL + "/search/jsonp"
	items, err := s.FetchNews(context.Background(), "sz000166", 10)
	if err != nil {
		t.Fatalf("FetchNews: %v", err)
	}
	// 标题一与主列表重复（同 URL）→ 去重；检索 B2 新增
	if len(items) != 3 {
		t.Fatalf("条数 = %d, want 3", len(items))
	}
	if items[0].Title != "标题一" || items[0].Text != "标题一" {
		t.Errorf("主列表条目错误: %+v", items[0])
	}
	third := items[2]
	if third.Title != "检索标题三" || third.Text != "正文内容乙" || third.Media != "财联社" {
		t.Errorf("检索条目错误: %+v", third)
	}
	if third.Date != "2026-09-12" || third.Sym != "sz000166" || third.SourceType != SourceTypeNews {
		t.Errorf("字段错误: %+v", third)
	}
}

// TestFetchNewsSearchFailDegrade 检索失败 → 主列表仍返回（正文=标题）。
func TestFetchNewsSearchFailDegrade(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/search/jsonp") {
			w.WriteHeader(http.StatusForbidden) // 4xx
			return
		}
		_, _ = w.Write([]byte(`{"code":1,"message":"success","data":{"totle_hits":1,"list":[
			{"Art_ShowTime":"2026-09-12 02:40:07","Art_Code":"A1","Art_Title":"标题一","Art_Url":"http://x/1"}]}}`))
	}))
	defer ts.Close()
	s := newTestSource()
	s.EMListAPI = ts.URL + "/comm/web/getListInfo"
	s.EMSearchAPI = ts.URL + "/search/jsonp"
	items, err := s.FetchNews(context.Background(), "sz000001", 5)
	if err != nil {
		t.Fatalf("检索失败应降级不阻塞: %v", err)
	}
	if len(items) != 1 || items[0].Text != "标题一" {
		t.Errorf("降级结果错误: %+v", items)
	}
}

// TestFetchNewsListFail 主接口失败 → 返回错误（调用方降级）。
func TestFetchNewsListFail(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer ts.Close()
	s := newTestSource()
	s.EMListAPI = ts.URL + "/list"
	s.EMSearchAPI = ts.URL + "/search"
	items, err := s.FetchNews(context.Background(), "sz000001", 5)
	if err == nil {
		t.Fatal("期望报错")
	}
	if items != nil {
		t.Errorf("期望空列表, got %+v", items)
	}
}

// TestFetchAnnouncements 巨潮公告解析（含 POST form 断言）。
func TestFetchAnnouncements(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %s, want POST", r.Method)
		}
		if err := r.ParseForm(); err != nil {
			t.Fatal(err)
		}
		if got := r.FormValue("stock"); got != "000166,qsgn0000301" {
			t.Errorf("stock = %q", got)
		}
		if got := r.FormValue("column"); got != "szse" {
			t.Errorf("column = %q", got)
		}
		if got := r.Form.Get("User-Agent"); got != "" {
		}
		_, _ = w.Write([]byte(`{"totalAnnouncement":2,"hasMore":true,"announcements":[
			{"secCode":"000166","secName":"申万宏源","announcementTitle":"<em>关于</em>某某公告","adjunctUrl":"finalpage/2026-09-12/1225559211.PDF","announcementTime":1789142400000},
			{"secCode":"000166","secName":"申万宏源","announcementTitle":"第二条","adjunctUrl":"finalpage/2026-09-11/1.PDF","announcementTime":1789056000000}]}`))
	}))
	defer ts.Close()
	s := newTestSource()
	s.CNQuery = ts.URL
	items, err := s.FetchAnnouncements(context.Background(), "sz000166", "qsgn0000301", 10)
	if err != nil {
		t.Fatalf("FetchAnnouncements: %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("条数 = %d", len(items))
	}
	if items[0].Title != "关于某某公告" {
		t.Errorf("标题剥 em 失败: %q", items[0].Title)
	}
	if items[0].URL != cninfoStatic+"finalpage/2026-09-12/1225559211.PDF" {
		t.Errorf("URL 拼接错误: %q", items[0].URL)
	}
	if items[0].Date != "2026-09-12" || items[0].SourceType != SourceTypeAnnouncement {
		t.Errorf("字段错误: %+v", items[0])
	}
}

// TestFetchAnnouncementsNoOrgID 缺 orgId → 报错。
func TestFetchAnnouncementsNoOrgID(t *testing.T) {
	s := newTestSource()
	if _, err := s.FetchAnnouncements(context.Background(), "sz000001", "", 5); err == nil {
		t.Fatal("缺 orgId 应报错")
	}
}
