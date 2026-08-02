// Package gnews 提供 Google News RSS 搜索接口，用于「找转载」：
// 付费墙文章按标题短语搜索免费转载/镜像（Google News 聚合大量来源副本）。
// 注意：Google News 的 <link> 是跳转链接（SPA 内 JS 跳转），curl -L 无法直达原站，
// 因此输出 Google 链接 + <source> 域名，由用户在浏览器打开。
package gnews

import (
	"context"
	"fmt"
	"net/url"
	"strings"
	"time"

	"news-report/internal/feed"
	"news-report/internal/fetch"
)

// Result 是一条转载/报道候选。
type Result struct {
	Title      string // 报道标题（已剥离来源后缀）
	SourceName string // 来源媒体名
	SourceURL  string // 来源域名（<source url>，仅域名无路径）
	Link       string // Google News 跳转链接
	Published  time.Time
	IsOriginal bool // 是否命中 exclude 域名（原站自身）
}

// gnewsBaseURL 便于测试替换；生产固定 Google News。
var gnewsBaseURL = "https://news.google.com"

// Search 按关键词/短语搜索 Google News。
// lang 影响 hl/gl/ceid 参数（en/de/fr/zh）。
func Search(ctx context.Context, f *fetch.Fetcher, query, lang string) ([]Result, error) {
	hl, gl := langParams(lang)
	escaped := url.QueryEscape(`"` + query + `"`)
	u := fmt.Sprintf("%s/rss/search?q=%s&hl=%s&gl=%s&ceid=%s:%s",
		gnewsBaseURL, escaped, hl, gl, gl, hl)

	sctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	// Google News RSS 端点版权声明明确许可"个人非商业用途的 feed reader"使用，
	// 其 robots.txt 的 /rss 规则面向通用爬虫而非 RSS 阅读器——故此处跳过 robots 检查。
	data, err := f.BytesNoRobots(sctx, u, "Mozilla/5.0 (compatible; news-report/"+version()+"; RSS reader)")
	if err != nil {
		return nil, fmt.Errorf("搜索 Google News: %w", err)
	}
	items, err := feed.Parse(data)
	if err != nil {
		return nil, fmt.Errorf("解析 Google News 结果: %w", err)
	}
	results := make([]Result, 0, len(items))
	for _, it := range items {
		title, srcName := splitTitleSource(it.Title)
		if it.SourceName != "" {
			srcName = it.SourceName
		}
		results = append(results, Result{
			Title:      title,
			SourceName: srcName,
			SourceURL:  it.SourceURL,
			Link:       it.URL,
			Published:  it.Published,
		})
	}
	return results, nil
}

// ExcludeOriginal 标记与 excludeDomain 同域的结果为原站（用于排除原站自身）。
func ExcludeOriginal(results []Result, excludeDomain string) []Result {
	ex := normalizeDomain(excludeDomain)
	for i := range results {
		if ex != "" && normalizeDomain(results[i].SourceURL) == ex {
			results[i].IsOriginal = true
		}
	}
	return results
}

func langParams(lang string) (hl, gl string) {
	switch lang {
	case "de":
		return "de-DE", "DE"
	case "fr":
		return "fr-FR", "FR"
	case "zh":
		return "zh-TW", "TW"
	default:
		return "en-US", "US"
	}
}

// splitTitleSource 剥离 Google News 标题的 " - 来源名" 后缀。
func splitTitleSource(t string) (title, source string) {
	if i := strings.LastIndex(t, " - "); i > 0 {
		// 只剥离最后一段（来源名通常无 " - "）
		return strings.TrimSpace(t[:i]), strings.TrimSpace(t[i+3:])
	}
	return t, ""
}

func normalizeDomain(d string) string {
	d = strings.TrimSpace(strings.ToLower(d))
	d = strings.TrimPrefix(d, "http://")
	d = strings.TrimPrefix(d, "https://")
	d = strings.TrimPrefix(d, "www.")
	if i := strings.IndexByte(d, '/'); i >= 0 {
		d = d[:i]
	}
	return d
}

func version() string { return "0.1.0" }
