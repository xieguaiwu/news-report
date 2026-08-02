package report

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"news-report/internal/classify"
	"news-report/internal/config"
	"news-report/internal/fetch"
	"news-report/internal/sources"
)

// 构造两个 feed 服务器：一个新闻、一个体育（用于测试分类与去重）。
func testServers(t *testing.T) (*httptest.Server, *httptest.Server) {
	t.Helper()
	feedA := `<?xml version="1.0"?><rss version="2.0"><channel>
<item><title>EU approves new sanctions package</title><link>https://a.example/1</link><pubDate>` +
		time.Now().Add(-2*time.Hour).Format(time.RFC1123Z) + `</pubDate></item>
<item><title>Central bank raises interest rates</title><link>https://a.example/2</link><pubDate>` +
		time.Now().Add(-3*time.Hour).Format(time.RFC1123Z) + `</pubDate></item>
<item><title>EU approves new sanctions package</title><link>https://a.example/1-dup</link><pubDate>` +
		time.Now().Add(-2*time.Hour).Format(time.RFC1123Z) + `</pubDate></item>
</channel></rss>`
	feedB := `<?xml version="1.0"?><rss version="2.0"><channel>
<item><title>Football team wins championship</title><link>https://b.example/sport</link><pubDate>` +
		time.Now().Add(-1*time.Hour).Format(time.RFC1123Z) + `</pubDate></item>
<item><title>Semiconductor fab expansion announced</title><link>https://b.example/chips</link><pubDate>` +
		time.Now().Add(-4*time.Hour).Format(time.RFC1123Z) + `</pubDate></item>
</channel></rss>`
	s1 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, feedA)
	}))
	s2 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, feedB)
	}))
	return s1, s2
}

// 只启用两个指定来源，其余全部禁用；seen-store 隔离到临时目录。
func cfgWithOnly(t *testing.T, ids ...string) *config.Config {
	cfg := config.Default()
	cfg.TimeoutSec = 5
	cfg.Retries = 0
	cfg.Concurrency = 4
	cfg.CacheDir = t.TempDir()
	only := map[string]bool{}
	for _, id := range ids {
		only[id] = true
	}
	for _, s := range sources.Defaults() {
		if !only[s.ID] {
			cfg.Sources[s.ID] = config.SourceOverride{Enabled: boolPtr(false)}
		}
	}
	return cfg
}

func TestRunPipeline(t *testing.T) {
	s1, s2 := testServers(t)
	defer s1.Close()
	defer s2.Close()

	cfg := cfgWithOnly(t, "bbc-world", "guardian-world")
	cfg.Sources["bbc-world"] = config.SourceOverride{Feeds: []string{s1.URL}}
	cfg.Sources["guardian-world"] = config.SourceOverride{Feeds: []string{s2.URL}}

	fetcher := fetch.New(fetch.Options{Timeout: 5 * time.Second, Retries: 0, NoRobots: true})
	rep, err := Run(context.Background(), cfg, fetcher, Options{})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	if rep.RawCount != 5 {
		t.Errorf("应抓取 5 条（含 1 条重复），实际 %d", rep.RawCount)
	}
	if rep.DupRemoved != 1 {
		t.Errorf("应去重 1 条，实际 %d", rep.DupRemoved)
	}
	var cats = map[classify.Category]bool{}
	for _, it := range rep.Items {
		cats[it.Category] = true
	}
	if !cats[classify.Politics] || !cats[classify.Economy] || !cats[classify.Industry] {
		t.Errorf("三类专注新闻都应出现: %v", cats)
	}
	// 默认模式：other 条目出现在末尾（strict 时隐藏）
	if !cats[classify.Other] {
		t.Error("默认模式应包含 other 条目（置于末尾）")
	}
	if len(rep.Items) > 0 && rep.Items[0].Category != classify.Politics {
		t.Errorf("politics 应按分类顺序排最前，实际 %s", rep.Items[0].Category)
	}
	for _, st := range rep.SourceStats {
		if !st.OK {
			t.Errorf("测试来源应全部成功: %+v", st)
		}
	}
}

func TestRunStrictFocus(t *testing.T) {
	s1, s2 := testServers(t)
	defer s1.Close()
	defer s2.Close()

	cfg := cfgWithOnly(t, "bbc-world", "guardian-world")
	cfg.Sources["bbc-world"] = config.SourceOverride{Feeds: []string{s1.URL}}
	cfg.Sources["guardian-world"] = config.SourceOverride{Feeds: []string{s2.URL}}

	fetcher := fetch.New(fetch.Options{Timeout: 5 * time.Second, Retries: 0, NoRobots: true})
	rep, err := Run(context.Background(), cfg, fetcher, Options{StrictFocus: true})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	for _, it := range rep.Items {
		if it.Category == classify.Other {
			t.Errorf("strict 模式不应出现 other: %q", it.Title)
		}
	}
	if len(rep.Items) != 3 {
		t.Errorf("strict 应剩 3 条（去重后 3 条专注），实际 %d", len(rep.Items))
	}
}

func TestRunSeenStore(t *testing.T) {
	s1, s2 := testServers(t)
	defer s1.Close()
	defer s2.Close()

	cfg := cfgWithOnly(t, "bbc-world", "guardian-world")
	cfg.Sources["bbc-world"] = config.SourceOverride{Feeds: []string{s1.URL}}
	cfg.Sources["guardian-world"] = config.SourceOverride{Feeds: []string{s2.URL}}

	fetcher := fetch.New(fetch.Options{Timeout: 5 * time.Second, Retries: 0, NoRobots: true})

	// 第一次运行：全部是新条目
	rep1, err := Run(context.Background(), cfg, fetcher, Options{})
	if err != nil {
		t.Fatalf("Run#1: %v", err)
	}
	first := len(rep1.Items)
	if first == 0 {
		t.Fatal("第一次运行应有条目")
	}

	// 第二次运行：全部已见 → 应全部被 seen-store 过滤
	rep2, err := Run(context.Background(), cfg, fetcher, Options{})
	if err != nil {
		t.Fatalf("Run#2: %v", err)
	}
	if len(rep2.Items) != 0 {
		t.Errorf("第二次运行应过滤全部已见条目，实际 %d", len(rep2.Items))
	}
	if rep2.SeenRemoved != first {
		t.Errorf("SeenRemoved 应等于 %d，实际 %d", first, rep2.SeenRemoved)
	}

	// --show-seen 应恢复显示
	rep3, err := Run(context.Background(), cfg, fetcher, Options{ShowSeen: true})
	if err != nil {
		t.Fatalf("Run#3: %v", err)
	}
	if len(rep3.Items) != first {
		t.Errorf("ShowSeen 应恢复 %d 条，实际 %d", first, len(rep3.Items))
	}
}

func TestRunFeedFailureNonFatal(t *testing.T) {
	dead := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(500)
	}))
	defer dead.Close()

	cfg := cfgWithOnly(t, "bbc-world", "guardian-world")
	cfg.Sources["bbc-world"] = config.SourceOverride{Feeds: []string{dead.URL}}
	s2 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `<?xml version="1.0"?><rss version="2.0"><channel>
<item><title>Trade deal signed</title><link>https://x.example/1</link><pubDate>`+
			time.Now().Format(time.RFC1123Z)+`</pubDate></item>
</channel></rss>`)
	}))
	defer s2.Close()
	cfg.Sources["guardian-world"] = config.SourceOverride{Feeds: []string{s2.URL}}

	fetcher := fetch.New(fetch.Options{Timeout: 3 * time.Second, Retries: 0, NoRobots: true})
	rep, err := Run(context.Background(), cfg, fetcher, Options{})
	if err != nil {
		t.Fatalf("单源失败不应致命: %v", err)
	}
	if len(rep.Items) == 0 {
		t.Fatal("另一个来源应正常产出")
	}
	failed := false
	for _, st := range rep.SourceStats {
		if st.ID == "bbc-world" && !st.OK {
			failed = true
		}
	}
	if !failed {
		t.Error("失败的来源应记录 Err 状态")
	}
}

func TestSourceWeight(t *testing.T) {
	list := sources.Defaults()
	if w := sourceWeight("bbc-world", list); w != 0.9 {
		t.Errorf("legacy 权重应为 0.9，实际 %v", w)
	}
	if w := sourceWeight("reuters-world", list); w != 1.0 {
		t.Errorf("wire 权重应为 1.0，实际 %v", w)
	}
	if w := sourceWeight("brookings", list); w != 1.0 {
		t.Errorf("specialist 权重应为 1.0，实际 %v", w)
	}
	if w := sourceWeight("nonexistent", list); w != 0.8 {
		t.Errorf("未知来源应回退 0.8，实际 %v", w)
	}
}

func boolPtr(b bool) *bool { return &b }
