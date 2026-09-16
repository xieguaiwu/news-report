package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"news-report/internal/crypto"
)

const cryptoHTTPTimeout = 25 * time.Second

func cryptoUsage(w io.Writer) {
	fmt.Fprint(w, `用法: news-report crypto [选项]

选项:
  --chain string       链: bsc | solana (默认 bsc)
  --sym value          代币地址或符号，可重复或逗号分隔
  --query string       按关键词搜索交易对
  --attention-only     只跑注意力源（微博/Telegram）
  --limit int          每个源最多取几条 (默认 50)
  --out string         JSONL 输出路径 (默认 out/crypto_<UTC 日期>.jsonl)
  --fetch-only         只抓取，不调用 LLM 打分
  --score-only         对已有 JSONL 补打分
  --in string          --score-only 的输入路径
`)
}

func runCrypto(args []string) int {
	fs := flag.NewFlagSet("crypto", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	fs.Usage = func() { cryptoUsage(os.Stderr) }

	var syms multiFlag
	chain := fs.String("chain", crypto.ChainBSC, "链: bsc | solana")
	query := fs.String("query", "", "按关键词搜索交易对")
	attentionOnly := fs.Bool("attention-only", false, "只跑注意力源")
	limit := fs.Int("limit", 50, "每个源最多取几条")
	out := fs.String("out", "", "JSONL 输出路径")
	fetchOnly := fs.Bool("fetch-only", false, "只抓取不打分")
	scoreOnly := fs.Bool("score-only", false, "对已有 JSONL 补打分")
	in := fs.String("in", "", "--score-only 的输入路径")
	fs.Var(&syms, "sym", "代币地址或符号，可重复或逗号分隔")

	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *scoreOnly {
		if *in == "" {
			fmt.Fprintln(os.Stderr, "crypto: --score-only 需要 --in")
			return 2
		}
		return cryptoScoreOnly(*in)
	}
	if *attentionOnly && len(syms) > 0 {
		fmt.Fprintln(os.Stderr, "crypto: --attention-only 与 --sym 互斥")
		return 2
	}
	if !*attentionOnly && len(syms) == 0 && *query == "" {
		cryptoUsage(os.Stderr)
		return 2
	}
	if *out == "" {
		*out = fmt.Sprintf("out/crypto_%s.jsonl", time.Now().UTC().Format("20060102"))
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	client := &http.Client{Timeout: cryptoHTTPTimeout}

	items, err := cryptoCollect(ctx, client, *chain, syms, *query, *attentionOnly, *limit)
	if err != nil {
		fmt.Fprintf(os.Stderr, "crypto: 采集失败: %v\n", err)
		return 1
	}

	scorer := crypto.NewScorer(crypto.ScorerConfigFromEnv())
	// 凭据缺失是硬告警，不是静默降级（GC 1 / G1）。
	if !scorer.Available() {
		fmt.Fprintln(os.Stderr, "crypto: 警告: 未配置 LLM 凭据（CRYPTO_LLM_API_KEY）——本次为 fetch-only")
	}
	if *fetchOnly || !scorer.Available() {
		scorer = nil
	}

	now := time.Now().Unix()
	rows := make([]crypto.Row, 0, len(items))
	for _, it := range items {
		var sc *crypto.Score
		if scorer != nil {
			if got, err := scorer.ScoreOne(ctx, it); err != nil {
				fmt.Fprintf(os.Stderr, "crypto: 打分失败 %s: %v\n", it.Title, err)
			} else {
				sc = &got
			}
		}
		rows = append(rows, rowFromItem(it, sc, now))
	}
	rows = crypto.DedupRows(rows)
	if err := crypto.AppendJSONL(*out, rows); err != nil {
		fmt.Fprintf(os.Stderr, "crypto: 写入失败: %v\n", err)
		return 1
	}
	fmt.Printf("crypto: %d 条 → %s\n", len(rows), *out)
	return 0
}

// rowFromItem 把原始观测转成 JSONL 行。
// EventAt 取事件自身时间；CollectedAt 取本次采集时刻（GC 7）。
func rowFromItem(it crypto.AttentionItem, sc *crypto.Score, collectedAt int64) crypto.Row {
	r := crypto.Row{
		SchemaVersion: crypto.SchemaVersion,
		EventAt:       it.ObservedAt,
		CollectedAt:   collectedAt,
		Chain:         it.Chain,
		TokenAddr:     it.TokenAddr,
		Symbol:        it.Symbol,
		Source:        it.Source,
		Kind:          it.Kind,
		Title:         it.Title,
		Text:          it.Text,
		URL:           it.URL,
		Metrics:       it.Metrics,
		ScoredAt:      time.Now().UTC().Format(time.RFC3339),
		Model:         crypto.DefaultModel,
		PromptVersion: crypto.PromptVersion,
	}
	if r.EventAt == 0 {
		r.EventAt = collectedAt // 无事件时间的源：event_at = collected_at
	}
	if sc != nil {
		tone, shill, spec, black := sc.Tone, sc.ShillScore, sc.Specificity, sc.BlackScore
		r.Tone, r.ShillScore, r.Specificity, r.BlackScore = &tone, &shill, &spec, &black
		r.Narrative, r.SourceTier = sc.Narrative, sc.SourceTier
	} else {
		r.Model = crypto.MissingKeyModel
	}
	return r
}

// cryptoCollect 按模式采集。三种模式互斥，见 usage。
func cryptoCollect(
	ctx context.Context, client *http.Client, chain string,
	syms []string, query string, attentionOnly bool, limit int,
) ([]crypto.AttentionItem, error) {
	var out []crypto.AttentionItem

	if attentionOnly {
		if items, err := crypto.WeiboHotSearch(ctx, client); err != nil {
			return nil, fmt.Errorf("weibo: %w", err)
		} else {
			out = append(out, crypto.FilterCryptoKeywords(items)...)
		}
		token := os.Getenv("CRYPTO_TG_BOT_TOKEN")
		if token == "" {
			return out, fmt.Errorf("Telegram 凭据缺失（CRYPTO_TG_BOT_TOKEN）：注意力腿实际只跑了微博")
		}
		if items, _, err := crypto.TelegramUpdates(ctx, client, token, 0); err != nil {
			return out, fmt.Errorf("telegram: %w", err)
		} else {
			out = append(out, crypto.FilterCryptoKeywords(items)...)
		}
		return out, nil
	}

	if query != "" {
		items, err := crypto.SearchPairs(ctx, client, query)
		if err != nil {
			return nil, fmt.Errorf("dexscreener search: %w", err)
		}
		out = append(out, capItems(items, limit)...)
	}
	for _, sym := range syms {
		items, err := crypto.PairsByChain(ctx, client, chain, sym)
		if err != nil {
			fmt.Fprintf(os.Stderr, "crypto: %s 行情获取失败: %v\n", sym, err)
			continue
		}
		out = append(out, capItems(items, limit)...)
		for _, it := range items { // 安全腿：对每个代币地址查一次
			chainID := crypto.GoPlusChainBSC
			hpChain := "bsc"
			if chain == crypto.ChainSolana {
				chainID, hpChain = crypto.GoPlusChainSolana, "solana"
			}
			if sec, err := crypto.GoPlusTokenSecurity(ctx, client, chainID, it.TokenAddr); err == nil {
				out = append(out, sec)
			} else {
				fmt.Fprintf(os.Stderr, "crypto: goplus %s: %v\n", it.TokenAddr, err)
			}
			if sec, err := crypto.HoneypotCheck(ctx, client, hpChain, it.TokenAddr); err == nil {
				out = append(out, sec)
			} else {
				fmt.Fprintf(os.Stderr, "crypto: honeypot %s: %v\n", it.TokenAddr, err)
			}
		}
	}
	return out, nil
}

func capItems(items []crypto.AttentionItem, limit int) []crypto.AttentionItem {
	if limit > 0 && len(items) > limit {
		return items[:limit]
	}
	return items
}

// cryptoScoreOnly 对已有 JSONL 补打分。T2 完成后再接线：读 JSONL → 逐行重打分。
func cryptoScoreOnly(path string) int {
	fmt.Fprintf(os.Stderr, "crypto: --score-only 待接线（依赖 JSONL 读取器，见跨腿契约任务）: %s\n", path)
	return 1
}
