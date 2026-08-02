// Package scrape 提供无 feed 来源的 HTML 兜底抓取：从索引页提取新闻链接。
package scrape

import (
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"

	"news-report/internal/feed"
)

// Rule 描述一个抓取规则。
type Rule struct {
	URL         string // 抓取目标页
	Selector    string // goquery 选择器；空 = 启发式
	LinkPattern string // 链接正则过滤（可选）
	MaxItems    int    // 上限（0 = 默认 25）
	Lang        string
	SourceID    string
}

var defaultMaxItems = 25

// junkRe 过滤明显非新闻链接。
var junkRe = regexp.MustCompile(`(?i)(^javascript:|mailto:|tel:|#|/login|/signup|/subscribe|/newsletter|/search\?|/tag/|/category/|/author/|/about|/contact|/privacy|/terms|/jobs|/advertise|/rss|/feed$|/wp-|/topics/|/experts/|/events/|/people/|/programs/|\.pdf$|\.jpg|\.png)`)

// Extract 从 HTML 中提取新闻条目列表。
func Extract(html, ruleURL string, r Rule) ([]feed.Item, error) {
	if r.MaxItems <= 0 {
		r.MaxItems = defaultMaxItems
	}
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		return nil, err
	}

	patternRe, err := regexp.Compile(r.LinkPattern)
	if err != nil {
		return nil, err
	}

	var items []feed.Item
	seen := map[string]bool{}

	selector := r.Selector
	if selector == "" {
		selector = "article a[href], h2 a[href], h3 a[href], h4 a[href]"
	}
	doc.Find(selector).Each(func(_ int, sel *goquery.Selection) {
		if len(items) >= r.MaxItems {
			return
		}
		href, ok := sel.Attr("href")
		if !ok || href == "" {
			return
		}
		title := strings.TrimSpace(sel.Text())
		if len(title) < 12 {
			return // 标题太短，大概率是导航链接
		}
		abs := resolveURL(ruleURL, href)
		if abs == "" || seen[abs] {
			return
		}
		if junkRe.MatchString(abs) {
			return
		}
		if r.LinkPattern != "" && !patternRe.MatchString(abs) {
			return
		}
		seen[abs] = true
		items = append(items, feed.Item{
			Title:     collapse(title),
			URL:       abs,
			Published: time.Now(), // 索引页无时间信息，视为最新
			SourceID:  r.SourceID,
			Lang:      r.Lang,
		})
	})

	if len(items) == 0 {
		return nil, nil
	}
	return items, nil
}

func resolveURL(base, ref string) string {
	bu, err := url.Parse(base)
	if err != nil {
		return ref
	}
	rv, err := url.Parse(ref)
	if err != nil {
		return ref
	}
	return bu.ResolveReference(rv).String()
}

func collapse(s string) string {
	return strings.Join(strings.Fields(s), " ")
}
