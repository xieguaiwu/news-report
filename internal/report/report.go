// Package report 编排完整流水线：
// 来源筛选 → 并发抓取（feed → scrape 兑底）→ 分类 → 去重 → 新鲜度过滤 → 排序 → 已读记录。
package report

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"news-report/internal/article"
	"news-report/internal/classify"
	"news-report/internal/config"
	"news-report/internal/dedup"
	"news-report/internal/feed"
	"news-report/internal/fetch"
	"news-report/internal/rank"
	"news-report/internal/scrape"
	"news-report/internal/sources"
	"news-report/internal/store"
)

// Item 是报告中的一条新闻。
type Item struct {
	Title     string
	URL       string
	Summary   string
	Body      string // 可选：全文（--fulltext 时填充）
	SourceID  string
	SourceName string
	Lang      string
	Tier      string
	Category  classify.Category
	Score     float64
	Published time.Time
	AgeLabel  string
}

// SourceStat 是单来源抓取结果统计。
type SourceStat struct {
	ID      string
	Name    string
	Lang    string
	Tier    string
	Feeds   []string
	OK      bool
	Items   int
	Err     string
	UsedScrape bool
}

// Report 是完整报告。
type Report struct {
	Generated   time.Time
	Config      *config.Config
	Items       []Item
	SourceStats []SourceStat
	Duration    time.Duration
	RawCount    int
	DupRemoved  int
	SeenRemoved int
}

// Options 控制单次运行。
type Options struct {
	Languages  []string // 空 = 全部
	Categories []string // 空 = 全部
	SourceIDs  []string // 空 = 全部
	Window     time.Duration // 0 = 不限
	LimitPerCat int
	TotalLimit  int
	StrictFocus bool
	ShowSeen    bool
	Fulltext    int
	FulltextMax int
	GoogleNews  bool
}

// Run 执行完整流水线。
func Run(ctx context.Context, cfg *config.Config, fetcher *fetch.Fetcher, opts Options) (*Report, error) {
	start := time.Now()
	rep := &Report{Generated: start, Config: cfg}

	if len(opts.Languages) == 0 {
		opts.Languages = cfg.Languages
	}
	if len(opts.Categories) == 0 {
		opts.Categories = cfg.Categories
	}
	if opts.LimitPerCat <= 0 {
		opts.LimitPerCat = cfg.LimitPerCat
	}
	if opts.TotalLimit <= 0 {
		opts.TotalLimit = cfg.TotalLimit
	}
	if opts.FulltextMax <= 0 {
		opts.FulltextMax = cfg.FulltextMax
	}

	// 1. 构建来源列表
	srcList := buildSources(cfg, opts)
	if len(srcList) == 0 {
		return nil, fmt.Errorf("没有启用的来源（检查 languages/categories/sources 配置）")
	}

	// 2. 并发抓取
	type srcResult struct {
		stat  SourceStat
		items []feed.Item
	}
	results := make([]srcResult, len(srcList))
	var wg sync.WaitGroup
	sem := make(chan struct{}, cfg.Concurrency)
	for i, s := range srcList {
		wg.Add(1)
		go func(i int, s sources.Source) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			items, stat := collectSource(ctx, fetcher, s, cfg.Retries)
			results[i] = srcResult{stat: stat, items: items}
		}(i, s)
	}
	wg.Wait()

	// 3. 汇总
	var all []feed.Item
	for _, r := range results {
		rep.SourceStats = append(rep.SourceStats, r.stat)
		all = append(all, r.items...)
	}
	rep.RawCount = len(all)

	// 4. 分类
	type scored struct {
		it   feed.Item
		cat  classify.Category
		kw   int
	}
	scoredItems := make([]scored, 0, len(all))
	for _, it := range all {
		res := classify.Classify(it.Lang, it.Title, it.Summary)
		scoredItems = append(scoredItems, scored{it: it, cat: res.Category, kw: res.Score})
	}

	// 5. 新鲜度窗口过滤
	if opts.Window > 0 {
		cutoff := start.Add(-opts.Window)
		kept := scoredItems[:0]
		for _, s := range scoredItems {
			if s.it.Published.IsZero() || s.it.Published.After(cutoff) {
				kept = append(kept, s)
			}
		}
		scoredItems = kept
	}

	// 6. 分类过滤
	catSet := map[string]bool{}
	for _, c := range opts.Categories {
		catSet[c] = true
	}
	if opts.StrictFocus {
		kept := scoredItems[:0]
		for _, s := range scoredItems {
			if catSet[string(s.cat)] {
				kept = append(kept, s)
			}
		}
		scoredItems = kept
	}

	// 7. 去重（保持按源顺序，优先保留靠前来源）
	halflife := time.Duration(cfg.HalflifeH * float64(time.Hour))
	now := time.Now()
	var uniq []Item
	srcName := map[string]string{}
	srcTier := map[string]string{}
	for _, s := range srcList {
		srcName[s.ID] = s.Name
		srcTier[s.ID] = string(s.Tier)
	}
	for _, s := range scoredItems {
		dup := false
		for _, u := range uniq {
			if dedup.IsDuplicate(u.Title, s.it.Title, u.Lang, s.it.Lang) {
				dup = true
				break
			}
		}
		if dup {
			rep.DupRemoved++
			continue
		}
		age := time.Duration(0)
		if !s.it.Published.IsZero() {
			age = now.Sub(s.it.Published)
		}
		uniq = append(uniq, Item{
			Title:      s.it.Title,
			URL:        s.it.URL,
			Summary:    s.it.Summary,
			SourceID:   s.it.SourceID,
			SourceName: srcName[s.it.SourceID],
			Lang:       s.it.Lang,
			Tier:       srcTier[s.it.SourceID],
			Category:   s.cat,
			Score:      rank.Score(sourceWeight(s.it.SourceID, srcList), s.kw, s.cat != classify.Other, age, halflife),
			Published:  s.it.Published,
			AgeLabel:   rank.AgeLabel(age),
		})
	}

	// 8. 已读记录过滤
	seen, err := store.New(cfg.CachePath() + "/seen.json")
	if err == nil {
		pruned := seen.Prune(cfg.StoreDays)
		if pruned > 0 {
			_ = seen.Save()
		}
		kept := uniq[:0]
		for _, it := range uniq {
			if opts.ShowSeen || !seen.Has(it.URL) {
				kept = append(kept, it)
			} else {
				rep.SeenRemoved++
			}
		}
		uniq = kept
		// 记录本轮报告过的条目
		for _, it := range uniq {
			seen.Add(it.URL)
		}
		_ = seen.Save()
	}

	// 9. 排序：分类内按分数降序
	catOrder := map[classify.Category]int{}
	for i, c := range opts.Categories {
		catOrder[classify.Category(c)] = i
	}
	catOrder[classify.Other] = len(catOrder)
	sort.SliceStable(uniq, func(i, j int) bool {
		if uniq[i].Category != uniq[j].Category {
			return catOrder[uniq[i].Category] < catOrder[uniq[j].Category]
		}
		return uniq[i].Score > uniq[j].Score
	})

	// 10. 数量限制
	perCat := map[classify.Category]int{}
	var final []Item
	for _, it := range uniq {
		if len(final) >= opts.TotalLimit {
			break
		}
		if perCat[it.Category] >= opts.LimitPerCat {
			continue
		}
		final = append(final, it)
		perCat[it.Category]++
	}
	rep.Items = final

	// 11. 全文抓取（深度模式）
	if opts.Fulltext > 0 && len(rep.Items) > 0 {
		rep.FetchFulltext(ctx, fetcher, opts.Fulltext, opts.FulltextMax)
	}

	rep.Duration = time.Since(start)
	return rep, nil
}

// FetchFulltext 为每个分类的前 n 条抓取全文（尽力而为，失败保留摘要）。
func (r *Report) FetchFulltext(ctx context.Context, f *fetch.Fetcher, n, maxChars int) {
	type job struct{ idx int }
	var wg sync.WaitGroup
	sem := make(chan struct{}, 6)
	perCat := map[classify.Category]int{}
	for i := range r.Items {
		it := &r.Items[i]
		if perCat[it.Category] >= n {
			continue
		}
		perCat[it.Category]++
		wg.Add(1)
		go func(it *Item) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			cctx, cancel := context.WithTimeout(ctx, 30*time.Second)
			defer cancel()
			art, err := article.Extract(cctx, f, it.URL, it.Lang)
			if err != nil {
				return // 尽力而为
			}
			body := strings.TrimSpace(art.Text)
			if maxChars > 0 && len(body) > maxChars {
				body = body[:maxChars] + "…"
			}
			it.Body = body
		}(it)
	}
	wg.Wait()
}

// buildSources 根据配置生成启用的来源列表。
func buildSources(cfg *config.Config, opts Options) []sources.Source {
	langSet := map[string]bool{}
	for _, l := range opts.Languages {
		langSet[l] = true
	}
	idSet := map[string]bool{}
	for _, id := range opts.SourceIDs {
		idSet[id] = true
	}

	var list []sources.Source
	all := sources.Defaults()
	if opts.GoogleNews {
		all = append(all, sources.GoogleNewsFeeds()...)
	}
	for _, s := range all {
		if !s.Enabled {
			continue
		}
		if !langSet[s.Lang] {
			continue
		}
		if len(idSet) > 0 && !idSet[s.ID] {
			continue
		}
		if ov, ok := cfg.Sources[s.ID]; ok {
			if ov.Enabled != nil && !*ov.Enabled {
				continue
			}
			if ov.Weight != nil {
				s.Weight = *ov.Weight
			}
			if len(ov.Feeds) > 0 {
				s.Feeds = ov.Feeds
			}
		}
		list = append(list, s)
	}
	return list
}

// collectSource 抓取单个来源：依次尝试 feed，全部失败/为空时尝试 scrape 兜底。
func collectSource(ctx context.Context, f *fetch.Fetcher, s sources.Source, retries int) ([]feed.Item, SourceStat) {
	stat := SourceStat{ID: s.ID, Name: s.Name, Lang: s.Lang, Tier: string(s.Tier), Feeds: s.Feeds}
	var items []feed.Item
	var lastErr string

	for _, feedURL := range s.Feeds {
		sctx, cancel := context.WithTimeout(ctx, 30*time.Second)
		var data []byte
		var err error
		if s.UserAgent != "" {
			data, err = f.BytesUA(sctx, feedURL, s.UserAgent)
		} else {
			data, err = f.Bytes(sctx, feedURL)
		}
		cancel()
		if err != nil {
			lastErr = err.Error()
			continue
		}
		parsed, perr := feed.Parse(data)
		if perr != nil {
			lastErr = perr.Error()
			continue
		}
		for i := range parsed {
			parsed[i].SourceID = s.ID
			parsed[i].Lang = s.Lang
		}
		items = append(items, parsed...)
		stat.OK = true
		stat.Items += len(parsed)
		break // 第一个成功的 feed 足够
	}

	if len(items) == 0 && s.Scrape != nil {
		sctx, cancel := context.WithTimeout(ctx, 30*time.Second)
		data, err := f.Bytes(sctx, s.Scrape.URL)
		cancel()
		if err == nil {
			scraped, serr := scrape.Extract(string(data), s.Scrape.URL, scrape.Rule{
				URL:         s.Scrape.URL,
				Selector:    s.Scrape.Selector,
				LinkPattern: s.Scrape.LinkPattern,
				MaxItems:    s.Scrape.MaxItems,
				Lang:        s.Lang,
				SourceID:    s.ID,
			})
			if serr == nil && len(scraped) > 0 {
				items = scraped
				stat.OK = true
				stat.Items = len(items)
				stat.UsedScrape = true
			} else if serr != nil {
				lastErr = serr.Error()
			}
		} else {
			lastErr = err.Error()
		}
	}

	if !stat.OK && lastErr != "" && !s.Optional {
		stat.Err = lastErr
	}
	return items, stat
}

// sourceWeight 查表来源权重（未知来源给默认 0.8）。
func sourceWeight(id string, list []sources.Source) float64 {
	for _, s := range list {
		if s.ID == id {
			return s.Weight
		}
	}
	return 0.8
}
