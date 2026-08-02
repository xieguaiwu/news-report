package article

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"news-report/internal/fetch"
)

// 文章页 fixture：带导航/广告噪音，正文集中在一个 <article>。
const articleHTML = `<!DOCTYPE html>
<html>
<head><title>Test News Site - EU Trade Deal Explained</title></head>
<body>
<nav><a href="/">Home</a><a href="/politics">Politics</a><a href="/sports">Sports</a></nav>
<header><h1>EU Trade Deal Explained: What It Means for Industry</h1>
<p class="byline">By Jane Reporter</p>
<time>2026-08-01</time></header>
<aside class="ad"><a href="/subscribe">Subscribe now</a></aside>
<article class="story-body">
<p>The European Union has signed a landmark trade agreement with the United States, ending years of tariff disputes between the two largest trading partners.</p>
<p>Under the terms of the deal, both sides will remove duties on industrial goods including semiconductors, steel, and electric vehicles over the next five years.</p>
<p>Industry groups welcomed the agreement, saying it would stabilize supply chains and boost manufacturing output across the Atlantic.</p>
<p>Analysts noted that the pact also addresses export controls on advanced technology, a sensitive issue in recent negotiations.</p>
<p>The agreement now heads to national parliaments for ratification, with votes expected later this year.</p>
</article>
<footer>Copyright 2026 Test News</footer>
</body></html>`

func TestExtractReadability(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		http.ServeContent(w, r, "index.html", time.Now(), strings.NewReader(articleHTML))
	}))
	defer srv.Close()

	f := fetch.New(fetch.Options{Timeout: 5_000_000_000, Retries: 0, NoRobots: true})
	art, err := Extract(context.Background(), f, srv.URL, "en")
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}
	if art.Title == "" {
		t.Error("标题不应为空")
	}
	if !strings.Contains(art.Text, "landmark trade agreement") {
		t.Error("正文应包含关键内容")
	}
	if strings.Contains(art.Text, "Subscribe now") {
		t.Error("正文不应包含广告内容")
	}
	if len(art.Text) < 300 {
		t.Errorf("正文过短: %d 字符", len(art.Text))
	}
}

func TestExtractFallback(t *testing.T) {
	// 无法被 readability 识别的最小页面 → 回退启发式
	html := `<html><head><title>Fallback Page</title></head><body>
<div class="article-body">
<p>This is a sufficiently long paragraph about industrial policy and manufacturing capacity that should be captured by the fallback extractor.</p>
<p>Another paragraph discussing economic growth and investment decisions across the European manufacturing sector.</p>
</div></body></html>`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		http.ServeContent(w, r, "x.html", time.Now(), strings.NewReader(html))
	}))
	defer srv.Close()

	f := fetch.New(fetch.Options{Timeout: 5_000_000_000, Retries: 0, NoRobots: true})
	art, err := Extract(context.Background(), f, srv.URL, "en")
	if err != nil {
		t.Fatalf("Extract fallback: %v", err)
	}
	if !strings.Contains(art.Text, "industrial policy") {
		t.Errorf("回退提取失败: %q", art.Text)
	}
}

func TestExtractHTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	defer srv.Close()
	f := fetch.New(fetch.Options{Timeout: 5_000_000_000, Retries: 0, NoRobots: true})
	if _, err := Extract(context.Background(), f, srv.URL, "en"); err == nil {
		t.Error("403 应报错")
	}
}

func TestExtractEmptyPage(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		http.ServeContent(w, r, "x.html", time.Now(), strings.NewReader("<html><body></body></html>"))
	}))
	defer srv.Close()
	f := fetch.New(fetch.Options{Timeout: 5_000_000_000, Retries: 0, NoRobots: true})
	if _, err := Extract(context.Background(), f, srv.URL, "en"); err == nil {
		t.Error("空页面应报错")
	}
}
