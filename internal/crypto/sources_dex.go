package crypto

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"sync"
	"time"
)

const (
	dexSearchURL     = "https://api.dexscreener.com/latest/dex/search"
	dexTokenPairsURL = "https://api.dexscreener.com/token-pairs/v1/%s/%s"
)

// tokenBucket 是最小令牌桶，用于自保限速（DexScreener 未公布限额）。
type tokenBucket struct {
	mu       sync.Mutex
	interval time.Duration
	last     time.Time
}

func newTokenBucket(perSecond float64) *tokenBucket {
	if perSecond <= 0 {
		perSecond = 2
	}
	return &tokenBucket{interval: time.Duration(float64(time.Second) / perSecond)}
}

func (b *tokenBucket) wait() {
	b.mu.Lock()
	defer b.mu.Unlock()
	if d := b.interval - time.Since(b.last); d > 0 {
		time.Sleep(d)
	}
	b.last = time.Now()
}

// dexBurst 是包级共享桶，所有 DexScreener 调用经它限速。
var dexBurst = newTokenBucket(2)

type dexPair struct {
	ChainID       string  `json:"chainId"`
	DexID         string  `json:"dexId"`
	PairAddress   string  `json:"pairAddress"`
	PriceUsd      string  `json:"priceUsd"`
	FDV           float64 `json:"fdv"`
	MarketCap     float64 `json:"marketCap"`
	PairCreatedAt int64   `json:"pairCreatedAt"` // ms
	URL           string  `json:"url"`
	BaseToken     struct {
		Address string `json:"address"`
		Symbol  string `json:"symbol"`
	} `json:"baseToken"`
	Liquidity struct {
		USD float64 `json:"usd"`
	} `json:"liquidity"`
	Volume struct {
		H24 float64 `json:"h24"`
	} `json:"volume"`
	PriceChange struct {
		H1  float64 `json:"h1"`
		H24 float64 `json:"h24"`
	} `json:"priceChange"`
	Txns struct {
		H24 struct {
			Buys  int `json:"buys"`
			Sells int `json:"sells"`
		} `json:"h24"`
	} `json:"txns"`
}

type dexResp struct {
	Pairs []dexPair `json:"pairs"`
}

// SearchPairs 按关键词检索交易对。
func SearchPairs(ctx context.Context, c *http.Client, query string) ([]AttentionItem, error) {
	return searchPairsAt(ctx, c, dexSearchURL+"?q="+url.QueryEscape(query))
}

// PairsByChain 拉取代币在某链上的全部交易对。
func PairsByChain(ctx context.Context, c *http.Client, chain, tokenAddr string) ([]AttentionItem, error) {
	u := fmt.Sprintf(dexTokenPairsURL, url.PathEscape(chain), url.PathEscape(tokenAddr))
	body, err := getWithRetry(ctx, c, u)
	if err != nil {
		return nil, err
	}
	// 该端点的响应有两种已观测形态：裸数组与 {"pairs": [...]}。两者都要兼容。
	var pairs []dexPair
	if err := json.Unmarshal(body, &pairs); err != nil {
		var wrapped dexResp
		if err2 := json.Unmarshal(body, &wrapped); err2 != nil {
			return nil, fmt.Errorf("dexscreener decode: %w", err2)
		}
		pairs = wrapped.Pairs
	}
	return pairsToItems(pairs), nil
}

// searchPairsAt 是 SearchPairs 的可注入端点版本（供测试）。
func searchPairsAt(ctx context.Context, c *http.Client, rawURL string) ([]AttentionItem, error) {
	body, err := getWithRetry(ctx, c, rawURL)
	if err != nil {
		return nil, err
	}
	var resp dexResp
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("dexscreener decode: %w", err)
	}
	return pairsToItems(resp.Pairs), nil
}

func pairsToItems(pairs []dexPair) []AttentionItem {
	out := make([]AttentionItem, 0, len(pairs))
	for _, p := range pairs {
		out = append(out, pairToItem(p))
	}
	return out
}

func pairToItem(p dexPair) AttentionItem {
	price, _ := strconv.ParseFloat(p.PriceUsd, 64)
	return AttentionItem{
		ObservedAt: p.PairCreatedAt / 1000, // 事件时间；抓取时刻由 T6 写 Row.CollectedAt
		Chain:      p.ChainID,
		TokenAddr:  p.BaseToken.Address,
		Symbol:     p.BaseToken.Symbol,
		Source:     "dexscreener",
		Kind:       KindMarket,
		Title:      "pair " + p.DexID + " " + p.BaseToken.Symbol,
		URL:        p.URL,
		Metrics: map[string]float64{
			"liquidity_usd":    p.Liquidity.USD,
			"fdv":              p.FDV,
			"market_cap":       p.MarketCap,
			"volume_24h":       p.Volume.H24,
			"buys_24h":         float64(p.Txns.H24.Buys),
			"sells_24h":        float64(p.Txns.H24.Sells),
			"price_usd":        price,
			"price_change_1h":  p.PriceChange.H1,
			"price_change_24h": p.PriceChange.H24,
		},
	}
}

// getWithRetry 带退避重试。429/5xx 重试（1s→3s→9s），其他 4xx 不重试。
// 调用前过共享令牌桶（≤2 req/s）。
func getWithRetry(ctx context.Context, c *http.Client, rawURL string) ([]byte, error) {
	return getWithRetryHeaders(ctx, c, rawURL, nil)
}

// getWithRetryHeaders 同 getWithRetry，但允许附加请求头（会覆盖默认值）。
// 某些源缺特定头会直接 403——实测微博网页接口不带 Referer 就是 403 Forbidden。
func getWithRetryHeaders(ctx context.Context, c *http.Client, rawURL string, extra map[string]string) ([]byte, error) {
	backoff := time.Second
	var lastErr error
	for attempt := 0; attempt < 4; attempt++ {
		dexBurst.wait()
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("User-Agent", userAgent)
		req.Header.Set("Accept", "application/json")
		for k, v := range extra {
			req.Header.Set(k, v)
		}
		resp, err := c.Do(req)
		if err != nil {
			lastErr = err
		} else {
			body, _ := io.ReadAll(io.LimitReader(resp.Body, maxBodyBytes))
			_ = resp.Body.Close()
			switch {
			case resp.StatusCode == http.StatusOK:
				return body, nil
			case resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500:
				lastErr = fmt.Errorf("http %d: %s", resp.StatusCode, truncate(string(body), 200))
			default:
				return nil, fmt.Errorf("http %d (no retry): %s", resp.StatusCode, truncate(string(body), 200))
			}
		}
		if attempt < 3 {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(backoff):
			}
			backoff *= 3
		}
	}
	return nil, lastErr
}
