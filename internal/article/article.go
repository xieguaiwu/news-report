// Package article 提供深度阅读能力：抓取网页并提取正文（go-readability），
// 失败时回退到 goquery 的启发式正文提取。支持 en/de/fr 的 Accept-Language。
package article

import (
	"context"
	"io"
	"net/url"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	readability "github.com/go-shiori/go-readability"

	"news-report/internal/fetch"
)

// Article 是提取后的文章。
type Article struct {
	Title       string
	URL         string
	Byline      string
	Excerpt     string
	Text        string
	Published   string
	SiteName    string
	ExtractedAt time.Time
}

// Extract 抓取并提取正文。
func Extract(ctx context.Context, f *fetch.Fetcher, rawURL, lang string) (*Article, error) {
	req, err := f.Request(ctx, rawURL)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept-Language", fetch.LangHeader(lang))
	resp, err := f.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return nil, errHTTP(resp.StatusCode)
	}
	// 读取受限大小（文章页通常 < 2MB，上限 8MB）
	body, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil {
		return nil, err
	}
	html := string(body)

	art, err := readability.FromReader(strings.NewReader(html), mustURL(rawURL))
	if err == nil && len(strings.TrimSpace(art.TextContent)) > 200 {
		pub := ""
		if art.PublishedTime != nil {
			pub = art.PublishedTime.Format("2006-01-02 15:04")
		}
		return &Article{
			Title:       clean(art.Title),
			URL:         rawURL,
			Byline:      clean(art.Byline),
			Excerpt:     clean(art.Excerpt),
			Text:        clean(art.TextContent),
			Published:   pub,
			SiteName:    clean(art.SiteName),
			ExtractedAt: time.Now(),
		}, nil
	}
	// 回退：goquery 启发式
	text := fallbackExtract(html)
	if len(strings.TrimSpace(text)) < 100 {
		// 提取不到正文：可能是付费墙（常见于 NYT/WSJ/Economist/Handelsblatt 等软墙）
		if looksPaywalled(html) {
			return nil, ErrPaywall
		}
		return nil, errNoContent
	}
	title := fallbackTitle(html)
	return &Article{
		Title:       title,
		URL:         rawURL,
		Text:        text,
		ExtractedAt: time.Now(),
	}, nil
}

// errHTTP 构造 HTTP 状态错误。
type statusError int

func (e statusError) Error() string { return "HTTP " + itoa(int(e)) }

func errHTTP(code int) error { return statusError(code) }

// ErrPaywall 表示文章被付费墙拦截（订阅/注册才能阅读全文）。
var ErrPaywall = errString("该文章可能被付费墙拦截——试试 news-report find '<标题关键词>' 找免费转载")

var errNoContent = errString("无法提取正文（页面为空或需要登录）")

// looksPaywalled 启发式检测付费墙：提取失败 + 页面存在付费特征标记。
var paywallMarkers = []string{
	"class=\"paywall", "data-paywall", "window.paywall", "isPaywall",
	"registration-wall", "registration_required", "subscriber-only", "subscriber only",
	"metered", "article locked", "access denied", "free articles left",
	"premium article", "premium content", "subscribe to read", "login to read",
	"register to read", "continue reading with subscription",
}

func looksPaywalled(html string) bool {
	lower := strings.ToLower(html)
	if len(lower) > 400<<10 {
		lower = lower[:400<<10] // 只看前 400KB（页面头部特征最密集）
	}
	hits := 0
	for _, m := range paywallMarkers {
		if strings.Contains(lower, m) {
			hits++
		}
	}
	return hits >= 2
}

type errString string

func (e errString) Error() string { return string(e) }

// fallbackExtract 启发式正文提取：优先 main/article，去掉脚本与导航。
func fallbackExtract(html string) string {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		return ""
	}
	doc.Find("script, style, nav, header, footer, aside, form, iframe, noscript").Remove()
	sel := doc.Find("main article, article, main, .article-body, .story-body, .article__content, .post-content")
	if sel.Length() == 0 {
		sel = doc.Find("body")
	}
	var sb strings.Builder
	sel.First().Find("p, h1, h2, h3, blockquote, li").Each(func(_ int, s *goquery.Selection) {
		txt := strings.TrimSpace(s.Text())
		if len(txt) > 40 {
			sb.WriteString(txt)
			sb.WriteString("\n\n")
		}
	})
	return strings.TrimSpace(sb.String())
}

func fallbackTitle(html string) string {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		return ""
	}
	t := strings.TrimSpace(doc.Find("h1").First().Text())
	if t != "" {
		return t
	}
	return strings.TrimSpace(doc.Find("title").First().Text())
}

// clean 压缩空白。
func clean(s string) string {
	return strings.Join(strings.Fields(s), " ")
}

func mustURL(s string) *url.URL {
	u, err := url.Parse(s)
	if err != nil {
		return &url.URL{}
	}
	return u
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b [8]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	return string(b[i:])
}
