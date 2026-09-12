package main

// astock 子命令：A股舆情分析（抓取东方财富个股新闻 + 巨潮公告 → LLM 打分 → JSONL）。
// 凭据注入见 scripts/astock_env.sh（ASTOCK_LLM_BASE_URL / ASTOCK_LLM_API_KEY）；
// bai 网关需代理（source /root/.pi/env 后 Go net/http 自动走 HTTPS_PROXY）。

import (
	"bufio"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"news-report/internal/astock"
)

// multiFlag 支持 --sym 重复传参与逗号分隔多值。
type multiFlag []string

func (m *multiFlag) String() string { return strings.Join(*m, ",") }
func (m *multiFlag) Set(v string) error {
	for _, p := range strings.Split(v, ",") {
		if p = strings.TrimSpace(p); p != "" {
			*m = append(*m, p)
		}
	}
	return nil
}

func runAstock(args []string) {
	fs := flag.NewFlagSet("astock", flag.ExitOnError)
	var (
		syms      multiFlag
		fetchOnly = fs.Bool("fetch-only", false, "只抓取不打分")
		scoreOnly = fs.Bool("score-only", false, "对已有 JSONL 中未打分行补打分（输入用 --in，默认 --out）")
		limit     = fs.Int("limit", 10, "每源每 sym 条数上限")
		outPath   = fs.String("out", "", "输出 JSONL（默认 out/astock_news_<date>.jsonl）")
		inPath    = fs.String("in", "", "--score-only 输入文件（默认取 --out）")
	)
	fs.Var(&syms, "sym", "股票代码或简称，可重复或逗号分隔（默认 000001,000166,300059）")
	fs.Usage = func() {}
	_ = fs.Parse(args)
	if len(syms) == 0 {
		syms = []string{"000001", "000166", "300059"}
	}
	date := time.Now().Format("20060102")
	if *outPath == "" {
		*outPath = filepath.Join("out", "astock_news_"+date+".jsonl")
	}
	log.SetFlags(log.Ltime)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if *scoreOnly {
		runAstockScoreOnly(ctx, *inPath, *outPath)
		return
	}

	src := astock.NewSource()
	src.SetLogf(log.Printf)

	log.Printf("astock: 加载股票列表…")
	res, err := astock.LoadStockList(filepath.Join("data", "stock_list.csv"), src)
	if err != nil {
		log.Printf("astock: 股票列表加载失败: %v", err)
		fatal(err)
	}
	log.Printf("astock: 股票列表 %d 条（data/stock_list.csv）", res.Len())

	// ── 抓取 ─────────────────────────────────────────────
	var items []astock.NewsItem
	seen := map[string]bool{}
	fetchFail := 0
	for _, raw := range syms {
		sym, err := res.Resolve(raw)
		if err != nil {
			log.Printf("astock: ✗ %s: %v", raw, err)
			fetchFail++
			continue
		}
		news, err := src.FetchNews(ctx, sym, *limit)
		if err != nil {
			log.Printf("astock: ⚠ %s 东财新闻降级: %v", sym, err)
			fetchFail++
		}
		for _, it := range news {
			addItem(&items, it, seen)
		}
		if orgID := res.OrgID(sym); orgID == "" {
			log.Printf("astock: ⚠ %s 无 orgId，跳过公告源", sym)
		} else if ann, err := src.FetchAnnouncements(ctx, sym, orgID, *limit); err != nil {
			log.Printf("astock: ⚠ %s 巨潮公告降级: %v", sym, err)
			fetchFail++
		} else {
			for _, it := range ann {
				addItem(&items, it, seen)
			}
		}
		log.Printf("astock: %s 抓取完成（累计 %d 条）", sym, len(items))
	}
	if len(items) == 0 {
		fmt.Println("astock: 未抓到任何条目")
		os.Exit(1)
	}

	// ── 打分 ─────────────────────────────────────────────
	rows := make([]astock.Row, len(items))
	if *fetchOnly {
		for i, it := range items {
			rows[i] = unscoredRow(it, "")
		}
		log.Printf("astock: --fetch-only，跳过打分")
	} else {
		cfg := astock.ScorerConfigFromEnv()
		if !cfg.Available() {
			log.Printf("astock: ⚠ 未注入打分凭据（source scripts/astock_env.sh），降级为 fetch-only，model=%s", astock.MissingKeyModel)
			for i, it := range items {
				rows[i] = unscoredRow(it, astock.MissingKeyModel)
			}
		} else {
			scorer := astock.NewScorer(cfg)
			scorer.SetLogf(log.Printf)
			log.Printf("astock: 打分 %d 条（model=%s, prompt=%s, 并发≤4）…", len(items), cfg.Model, astock.PromptVersion)
			results := scorer.ScoreBatch(ctx, items)
			okCnt, failCnt := 0, 0
			for i, r := range results {
				if r.Err != nil {
					log.Printf("astock: ⚠ 打分失败 #%d %q: %v", i, truncateTitle(items[i].Title), r.Err)
					rows[i] = unscoredRow(items[i], "")
					failCnt++
					continue
				}
				rows[i] = scoredRow(items[i], r.Score, cfg.Model)
				okCnt++
			}
			log.Printf("astock: 打分完成 成功 %d / 失败 %d", okCnt, failCnt)
		}
	}

	if err := writeJSONL(*outPath, rows); err != nil {
		fatal(err)
	}
	log.Printf("astock: ✓ 输出 %s（%d 行，抓取失败事件 %d）", *outPath, len(rows), fetchFail)
}

// runAstockScoreOnly 读入 JSONL，对未打分行补打分后重写。
func runAstockScoreOnly(ctx context.Context, inPath, outPath string) {
	if inPath == "" {
		inPath = outPath
	}
	rows, err := readJSONL(inPath)
	if err != nil {
		fatal(err)
	}
	var targets []int
	var items []astock.NewsItem
	for i, r := range rows {
		if r.ScoredAt == "" && r.Title != "" {
			targets = append(targets, i)
			items = append(items, astock.NewsItem{
				Date: r.Date, Sym: r.Sym, SourceType: r.SourceType,
				Title: r.Title, Text: r.Text, URL: r.URL,
			})
		}
	}
	if len(items) == 0 {
		fmt.Printf("astock: %s 无未打分行\n", inPath)
		return
	}
	cfg := astock.ScorerConfigFromEnv()
	if !cfg.Available() {
		fatal(fmt.Errorf("未注入打分凭据（source scripts/astock_env.sh）"))
	}
	scorer := astock.NewScorer(cfg)
	scorer.SetLogf(log.Printf)
	log.Printf("astock: --score-only 补打分 %d 条…", len(items))
	results := scorer.ScoreBatch(ctx, items)
	for j, r := range results {
		idx := targets[j]
		if r.Err != nil {
			log.Printf("astock: ⚠ 打分失败 #%d: %v", idx, r.Err)
			continue
		}
		rows[idx] = scoredRow(items[j], r.Score, cfg.Model)
	}
	if err := writeJSONL(outPath, rows); err != nil {
		fatal(err)
	}
	log.Printf("astock: ✓ 重写 %s（%d 行）", outPath, len(rows))
}

// addItem 去重加入（按 URL 与 标题 双键）。
func addItem(items *[]astock.NewsItem, it astock.NewsItem, seen map[string]bool) {
	for _, k := range []string{"u:" + it.URL, "t:" + it.Title} {
		if seen[k] {
			return
		}
	}
	seen["u:"+it.URL] = true
	seen["t:"+it.Title] = true
	*items = append(*items, it)
}

func unscoredRow(it astock.NewsItem, model string) astock.Row {
	return astock.Row{
		Date: it.Date, Sym: it.Sym, SourceType: it.SourceType,
		Title: it.Title, Text: it.Text, URL: it.URL, Model: model,
	}
}

func scoredRow(it astock.NewsItem, sc astock.Score, model string) astock.Row {
	r := unscoredRow(it, "")
	tone, spec, black := sc.Tone, sc.Specificity, sc.BlackScore
	r.Tone, r.Kind, r.Specificity, r.SourceTier, r.BlackScore = &tone, sc.Kind, &spec, sc.SourceTier, &black
	r.ScoredAt = time.Now().Format(time.RFC3339)
	r.Model = model
	r.PromptVersion = astock.PromptVersion
	return r
}

// writeJSONL 落盘（建目录、逐行 JSON）。
func writeJSONL(path string, rows []astock.Row) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	w := bufio.NewWriter(f)
	enc := json.NewEncoder(w)
	for _, r := range rows {
		if err := enc.Encode(r); err != nil {
			return err
		}
	}
	return w.Flush()
}

// readJSONL 读入行（容忍个别坏行）。
func readJSONL(path string) ([]astock.Row, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("打开 %s: %w", path, err)
	}
	defer f.Close()
	var rows []astock.Row
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 1024*1024), 1024*1024)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		var r astock.Row
		if err := json.Unmarshal([]byte(line), &r); err != nil {
			log.Printf("astock: ⚠ 跳过坏行: %v", err)
			continue
		}
		rows = append(rows, r)
	}
	return rows, sc.Err()
}

func truncateTitle(s string) string {
	rs := []rune(s)
	if len(rs) > 24 {
		return string(rs[:24]) + "…"
	}
	return s
}
