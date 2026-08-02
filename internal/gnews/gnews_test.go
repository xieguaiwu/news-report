package gnews

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"news-report/internal/fetch"
)

// 模拟 Google News RSS 响应（含 source 元素与标题后缀）。
const fakeGNewsXML = `<?xml version="1.0" encoding="UTF-8"?>
<rss version="2.0">
<channel>
<item>
  <title>EU proposes new sanctions package - Reuters</title>
  <link>https://news.google.com/rss/articles/CBMiabc</link>
  <guid isPermaLink="false">CBMiabc</guid>
  <pubDate>Fri, 31 Jul 2026 05:30:00 GMT</pubDate>
  <description>&lt;a href="https://news.google.com/rss/articles/CBMiabc"&gt;EU proposes new sanctions package&lt;/a&gt;</description>
  <source url="https://www.reuters.com">Reuters</source>
</item>
<item>
  <title>EU sanctions: what it means for industry - euractiv.com</title>
  <link>https://news.google.com/rss/articles/CBMixyz</link>
  <pubDate>Sat, 01 Aug 2026 10:00:00 GMT</pubDate>
  <source url="https://www.euractiv.com">euractiv.com</source>
</item>
</channel>
</rss>`

func TestSearch(t *testing.T) {
	orig := gnewsBaseURL
	defer func() { gnewsBaseURL = orig }()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query().Get("q")
		if q != `"EU sanctions"` {
			t.Errorf("q 参数应为解码后的引号短语，实际 %q", q)
		}
		if r.URL.Query().Get("hl") != "en-US" {
			t.Errorf("hl 参数错误: %q", r.URL.Query().Get("hl"))
		}
		w.Header().Set("Content-Type", "application/xml")
		w.Write([]byte(fakeGNewsXML))
	}))
	defer srv.Close()
	gnewsBaseURL = srv.URL

	f := fetch.New(fetch.Options{Timeout: 5 * time.Second, Retries: 0})
	results, err := Search(context.Background(), f, "EU sanctions", "en")
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("期望 2 条结果，实际 %d", len(results))
	}
	r0 := results[0]
	if r0.Title != "EU proposes new sanctions package" {
		t.Errorf("标题应剥离来源后缀: %q", r0.Title)
	}
	if r0.SourceName != "Reuters" || r0.SourceURL != "https://www.reuters.com" {
		t.Errorf("source 字段提取失败: %q / %q", r0.SourceName, r0.SourceURL)
	}
	if r0.Link != "https://news.google.com/rss/articles/CBMiabc" {
		t.Errorf("Link 错误: %q", r0.Link)
	}
	want := time.Date(2026, 7, 31, 5, 30, 0, 0, time.UTC)
	if !r0.Published.Equal(want) {
		t.Errorf("Published 错误: %v，期望 %v", r0.Published, want)
	}
}

func TestSplitTitleSource(t *testing.T) {
	title, src := splitTitleSource("EU proposes new sanctions package - Reuters")
	if title != "EU proposes new sanctions package" || src != "Reuters" {
		t.Errorf("拆分错误: %q / %q", title, src)
	}
	title, src = splitTitleSource("EU proposes new sanctions package")
	if title != "EU proposes new sanctions package" || src != "" {
		t.Errorf("无来源后缀时应原样返回: %q / %q", title, src)
	}
	// 标题内部含 " - " 时只剥离最后一段
	title, _ = splitTitleSource("Interview - EU sanctions - Politico")
	if title != "Interview - EU sanctions" {
		t.Errorf("应只剥离最后一段: %q", title)
	}
}

func TestExcludeOriginal(t *testing.T) {
	results := []Result{
		{Title: "a", SourceURL: "https://www.reuters.com"},
		{Title: "b", SourceURL: "https://www.euractiv.com"},
		{Title: "c", SourceURL: "https://news.bbc.co.uk/article"},
	}
	results = ExcludeOriginal(results, "reuters.com")
	if !results[0].IsOriginal {
		t.Error("reuters.com 结果应标记为原站")
	}
	if results[1].IsOriginal || results[2].IsOriginal {
		t.Error("其他来源不应标记")
	}
	// www 前缀与路径应被归一化
	results = ExcludeOriginal(results, "www.euractiv.com")
	if !results[1].IsOriginal {
		t.Error("www.euractiv.com 应匹配 euractiv.com")
	}
}

func TestNormalizeDomain(t *testing.T) {
	cases := []struct{ in, want string }{
		{"https://www.reuters.com", "reuters.com"},
		{"http://news.bbc.co.uk/article", "news.bbc.co.uk"},
		{"EURACTIV.com", "euractiv.com"},
		{"", ""},
	}
	for _, c := range cases {
		if got := normalizeDomain(c.in); got != c.want {
			t.Errorf("normalizeDomain(%q) = %q，期望 %q", c.in, got, c.want)
		}
	}
}

func TestLangParams(t *testing.T) {
	cases := []struct{ lang, hl, gl string }{
		{"en", "en-US", "US"},
		{"de", "de-DE", "DE"},
		{"fr", "fr-FR", "FR"},
		{"zh", "zh-TW", "TW"},
		{"xx", "en-US", "US"},
	}
	for _, c := range cases {
		hl, gl := langParams(c.lang)
		if hl != c.hl || gl != c.gl {
			t.Errorf("langParams(%q) = %s/%s，期望 %s/%s", c.lang, hl, gl, c.hl, c.gl)
		}
	}
}
