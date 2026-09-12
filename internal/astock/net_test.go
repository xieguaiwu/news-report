//go:build net

// 网络集成用例：仅 -tags=net 时运行（验收 C 规定）。
//   /usr/local/go/bin/go test ./internal/astock/... -tags=net -v
package astock

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// TestNetFetchStockListCNInfo 巨潮全量列表（主源）。
func TestNetFetchStockListCNInfo(t *testing.T) {
	src := NewSource()
	src.SetLogf(t.Logf)
	entries, err := src.FetchStockListCNInfo(context.Background())
	if err != nil {
		t.Fatalf("FetchStockListCNInfo: %v", err)
	}
	if len(entries) < 5000 {
		t.Errorf("条数 = %d, want ≥5000", len(entries))
	}
	var found bool
	for _, e := range entries {
		if e.Code == "000001" && e.OrgID != "" {
			found = true
		}
	}
	if !found {
		t.Error("未找到 000001（含 orgId）")
	}
}

// TestNetLoadStockList 缓存 + 远端装配。
func TestNetLoadStockList(t *testing.T) {
	src := NewSource()
	src.SetLogf(t.Logf)
	path := filepath.Join(t.TempDir(), "stock_list.csv")
	res, err := LoadStockList(path, src)
	if err != nil {
		t.Fatalf("LoadStockList: %v", err)
	}
	if res.Len() < 5000 {
		t.Errorf("条数 = %d", res.Len())
	}
	for _, in := range []string{"000001", "000166", "300059", "600000", "平安银行", "SZ000166"} {
		sym, err := res.Resolve(in)
		if err != nil {
			t.Errorf("Resolve(%q): %v", in, err)
		} else {
			t.Logf("Resolve(%q) = %s", in, sym)
		}
	}
}

// TestNetFetchNews 东财个股新闻（secid 沪深分辨 + 正文合并）。
func TestNetFetchNews(t *testing.T) {
	src := NewSource()
	src.SetLogf(t.Logf)
	for _, sym := range []string{"sz000001", "sh600000"} {
		items, err := src.FetchNews(context.Background(), sym, 5)
		if err != nil {
			t.Fatalf("FetchNews(%s): %v", sym, err)
		}
		if len(items) == 0 {
			t.Errorf("FetchNews(%s) 空结果", sym)
			continue
		}
		it := items[0]
		if it.Date == "" || it.Title == "" || it.URL == "" || it.SourceType != SourceTypeNews {
			t.Errorf("字段缺失: %+v", it)
		}
		t.Logf("%s: %d 条 | 首条 [%s] %s (%s)", sym, len(items), it.Date, it.Title, it.Media)
	}
}

// TestNetFetchAnnouncements 巨潮公告（000166 申万宏源）。
func TestNetFetchAnnouncements(t *testing.T) {
	src := NewSource()
	src.SetLogf(t.Logf)
	items, err := src.FetchAnnouncements(context.Background(), "sz000166", "qsgn0000301", 5)
	if err != nil {
		t.Fatalf("FetchAnnouncements: %v", err)
	}
	if len(items) == 0 {
		t.Fatal("空结果")
	}
	it := items[0]
	if it.Date == "" || it.Title == "" || it.URL == "" || it.SourceType != SourceTypeAnnouncement {
		t.Errorf("字段缺失: %+v", it)
	}
	t.Logf("sz000166: %d 条 | 首条 [%s] %s", len(items), it.Date, it.Title)
}

// TestNetScoreOneSmoke 真实打分（需先 source scripts/astock_env.sh；无凭据则跳过）。
func TestNetScoreOneSmoke(t *testing.T) {
	cfg := ScorerConfigFromEnv()
	if !cfg.Available() {
		t.Skip("未注入 ASTOCK_LLM_*（source scripts/astock_env.sh 后重跑）")
	}
	if cfg.Model == "" {
		cfg.Model = DefaultModel
	}
	s := NewScorer(cfg)
	s.SetLogf(t.Logf)
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	sc, err := s.ScoreOne(ctx, testItem())
	if err != nil {
		t.Fatalf("ScoreOne: %v", err)
	}
	t.Logf("打分结果: %+v", sc)
	if sc.Kind == "" || sc.SourceTier == "" {
		t.Errorf("枚举字段为空: %+v", sc)
	}
	if os.Getenv("CI") != "" && (sc.Tone < -2 || sc.Tone > 2) {
		t.Errorf("tone 越界: %d", sc.Tone)
	}
}
