package sources

import (
	"strings"
	"testing"
)

func TestDefaultsCoverage(t *testing.T) {
	all := Defaults()
	if len(all) < 40 {
		t.Errorf("内置来源应 ≥40 个，实际 %d", len(all))
	}
	langs := map[string]int{}
	tiers := map[Tier]int{}
	seen := map[string]bool{}
	for _, s := range all {
		if s.ID == "" || s.Name == "" {
			t.Errorf("来源 ID/名称不能为空: %+v", s)
		}
		if seen[s.ID] {
			t.Errorf("来源 ID 重复: %s", s.ID)
		}
		seen[s.ID] = true
		if !s.Enabled {
			t.Errorf("内置来源默认应启用: %s", s.ID)
		}
		langs[s.Lang]++
		tiers[s.Tier]++
		switch s.Lang {
		case "en", "de", "fr":
		default:
			t.Errorf("非法语言 %q: %s", s.Lang, s.ID)
		}
		if s.Weight <= 0 || s.Weight > 1 {
			t.Errorf("权重越界: %s = %v", s.ID, s.Weight)
		}
		if len(s.Feeds) == 0 && s.Scrape == nil {
			t.Errorf("来源必须有 feed 或 scrape: %s", s.ID)
		}
	}
	for l := range map[string]bool{"en": true, "de": true, "fr": true} {
		if langs[l] == 0 {
			t.Errorf("缺少 %s 语言来源", l)
		}
	}
	for tier := range map[Tier]bool{TierWire: true, TierLegacy: true, TierSpecialist: true} {
		if tiers[tier] == 0 {
			t.Errorf("缺少 %s 层级来源", tier)
		}
	}
}

func TestDefaultsSorted(t *testing.T) {
	all := Defaults()
	for i := 1; i < len(all); i++ {
		if all[i-1].Lang > all[i].Lang {
			t.Errorf("来源未按语言排序: %s > %s", all[i-1].ID, all[i].ID)
		}
	}
}

func TestWeightFor(t *testing.T) {
	if w := weightFor(TierWire); w != 1.0 {
		t.Errorf("wire 权重应为 1.0，实际 %v", w)
	}
	if w := weightFor(TierLegacy); w != 0.9 {
		t.Errorf("legacy 权重应为 0.9，实际 %v", w)
	}
	if w := weightFor(TierSpecialist); w != 1.0 {
		t.Errorf("specialist 权重应为 1.0，实际 %v", w)
	}
}

func TestGoogleNewsFeeds(t *testing.T) {
	feeds := GoogleNewsFeeds()
	// 3 分类 × 3 语言 × 2 查询 = 18 个源
	if len(feeds) != 18 {
		t.Fatalf("GoogleNewsFeeds 应为 18 个，实际 %d", len(feeds))
	}
	ids := map[string]bool{}
	for _, f := range feeds {
		if ids[f.ID] {
			t.Errorf("GoogleNews 源 ID 重复: %s", f.ID)
		}
		ids[f.ID] = true
		if len(f.Feeds) != 1 || !strings.Contains(f.Feeds[0], "news.google.com/rss/search") {
			t.Errorf("GoogleNews feed URL 异常: %+v", f)
		}
		if f.Weight != 0.75 {
			t.Errorf("GoogleNews 权重应为 0.75，实际 %v", f.Weight)
		}
	}
}

func TestUNNewsUserAgent(t *testing.T) {
	all := Defaults()
	for _, s := range all {
		if s.ID == "un-news" {
			if s.UserAgent != "Feedfetcher-Google/1.0" {
				t.Errorf("un-news 应使用 Feedfetcher-Google UA，实际 %q", s.UserAgent)
			}
			return
		}
	}
	t.Error("缺少 un-news 来源")
}

func TestBrookingsScrapeFallback(t *testing.T) {
	all := Defaults()
	for _, s := range all {
		if s.ID == "brookings" {
			if s.Scrape == nil || !strings.Contains(s.Scrape.LinkPattern, "articles") {
				t.Errorf("brookings 应有 scrape 兑底且带文章链接模式: %+v", s.Scrape)
			}
			return
		}
	}
	t.Error("缺少 brookings 来源")
}
