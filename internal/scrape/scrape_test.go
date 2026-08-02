package scrape

import (
	"strings"
	"testing"

	"news-report/internal/feed"
)

const sampleHTML = `<!DOCTYPE html>
<html><head><title>News Home</title></head>
<body>
<nav><a href="/about">About Us</a><a href="mailto:x@y.com">Contact</a></nav>
<article>
  <h2><a href="/story/1">EU and US reach breakthrough trade agreement</a></h2>
  <p>The deal covers tariffs and steel.</p>
</article>
<article>
  <h3><a href="https://example.com/story/2">Central bank raises rates again</a></h3>
</article>
<article>
  <h2><a href="/story/1">EU and US reach breakthrough trade agreement</a></h2>
</article>
<div><a href="/login">Login</a></div>
<p><a href="/short">Too short</a></p>
</body></html>`

func TestExtractHeuristic(t *testing.T) {
	items, err := Extract(sampleHTML, "https://example.com/", Rule{SourceID: "test", Lang: "en"})
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("期望 2 条（去重后），实际 %d", len(items))
	}
	if items[0].URL != "https://example.com/story/1" {
		t.Errorf("相对 URL 应解析为绝对: %q", items[0].URL)
	}
	if items[1].URL != "https://example.com/story/2" {
		t.Errorf("绝对 URL 应保留: %q", items[1].URL)
	}
	if items[0].SourceID != "test" || items[0].Lang != "en" {
		t.Errorf("元数据未传递: %+v", items[0])
	}
}

func TestExtractWithSelector(t *testing.T) {
	r := Rule{SourceID: "t", Lang: "en", Selector: "article h3 a[href]", MaxItems: 5}
	items, err := Extract(sampleHTML, "https://example.com/", r)
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("选择器应只匹配 1 条，实际 %d", len(items))
	}
	if items[0].Title != "Central bank raises rates again" {
		t.Errorf("标题错误: %q", items[0].Title)
	}
}

func TestExtractLinkPattern(t *testing.T) {
	r := Rule{SourceID: "t", Lang: "en", LinkPattern: `^https://example\.com/story/\d+$`}
	items, err := Extract(sampleHTML, "https://example.com/", r)
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}
	for _, it := range items {
		if !strings.Contains(it.URL, "/story/") {
			t.Errorf("link pattern 过滤失效: %q", it.URL)
		}
	}
}

func TestExtractEmpty(t *testing.T) {
	items, err := Extract("<html><body><p>no links here</p></body></html>", "https://x.com/", Rule{})
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}
	if items != nil && len(items) != 0 {
		t.Errorf("无链接应返回空: %+v", items)
	}
}

var _ = feed.Item{} // 确保 feed 导入保持（未来类型引用）
