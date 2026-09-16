package crypto

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const (
	weiboHotSearchURL = "https://weibo.com/ajax/side/hotSearch"
	telegramAPIBase   = "https://api.telegram.org"
)

// cryptoKeywords 是注意力源的关键词过滤表。命中任一即保留。
var cryptoKeywords = []string{
	"比特币", "以太", "加密", "币圈", "区块链", "牛市", "牛来", "稳定币",
	"bitcoin", "btc", "eth", "crypto", "meme", "token", "defi", "airdrop",
	"狗狗币", "doge", "shib", "pepe", "solana", "bnb", "币安",
}

// FilterCryptoKeywords 保留标题或符号命中关键词的条目。
func FilterCryptoKeywords(items []AttentionItem) []AttentionItem {
	out := make([]AttentionItem, 0, len(items))
	for _, it := range items {
		hay := strings.ToLower(it.Title + " " + it.Symbol + " " + it.Text)
		for _, kw := range cryptoKeywords {
			if strings.Contains(hay, strings.ToLower(kw)) {
				out = append(out, it)
				break
			}
		}
	}
	return out
}

type weiboResp struct {
	Data struct {
		Realtime []struct {
			Word   string `json:"word"`
			Num    int    `json:"num"`
			RawHot int    `json:"raw_hot"`
		} `json:"realtime"`
	} `json:"data"`
}

// WeiboHotSearch 拉取微博全站热搜榜（未过滤）。
func WeiboHotSearch(ctx context.Context, c *http.Client) ([]AttentionItem, error) {
	return weiboHotSearchAt(ctx, c, weiboHotSearchURL)
}

func weiboHotSearchAt(ctx context.Context, c *http.Client, rawURL string) ([]AttentionItem, error) {
	body, err := getWithRetry(ctx, c, rawURL)
	if err != nil {
		return nil, err
	}
	var resp weiboResp
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("weibo decode: %w", err)
	}
	now := time.Now().Unix()
	out := make([]AttentionItem, 0, len(resp.Data.Realtime))
	for _, w := range resp.Data.Realtime {
		out = append(out, AttentionItem{
			ObservedAt: now,
			Chain:      "",
			Symbol:     w.Word,
			Source:     "weibo",
			Kind:       KindAttention,
			Title:      w.Word,
			Metrics: map[string]float64{
				"rank_num": float64(w.Num),
				"raw_hot":  float64(w.RawHot),
			},
		})
	}
	return out, nil
}

type tgResp struct {
	OK     bool `json:"ok"`
	Result []struct {
		UpdateID    int64 `json:"update_id"`
		ChannelPost *struct {
			Date int64  `json:"date"`
			Text string `json:"text"`
			Chat struct {
				Title string `json:"title"`
			} `json:"chat"`
		} `json:"channel_post"`
	} `json:"result"`
}

// TelegramUpdates 拉取频道更新。返回条目与下一次使用的 offset。
// URL 字段刻意留空——Bot token 出现在 URL 中是泄漏向量。
func TelegramUpdates(ctx context.Context, c *http.Client, botToken string, offset int64) ([]AttentionItem, int64, error) {
	u := fmt.Sprintf("%s/bot%s/getUpdates?timeout=0&offset=%d", telegramAPIBase, botToken, offset)
	return telegramUpdatesAt(ctx, c, u, offset)
}

func telegramUpdatesAt(ctx context.Context, c *http.Client, rawURL string, offset int64) ([]AttentionItem, int64, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, offset, err
	}
	req.Header.Set("User-Agent", userAgent)
	resp, err := c.Do(req)
	if err != nil {
		return nil, offset, err
	}
	defer func() { _, _ = io.Copy(io.Discard, resp.Body); _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		// 不回显 body——可能含 token 片段
		return nil, offset, fmt.Errorf("telegram http %d", resp.StatusCode)
	}
	var tr tgResp
	if err := json.NewDecoder(io.LimitReader(resp.Body, maxBodyBytes)).Decode(&tr); err != nil {
		return nil, offset, fmt.Errorf("telegram decode: %w", err)
	}
	next := offset
	out := make([]AttentionItem, 0, len(tr.Result))
	for _, u := range tr.Result {
		if u.UpdateID >= next {
			next = u.UpdateID + 1
		}
		if u.ChannelPost == nil {
			continue
		}
		out = append(out, AttentionItem{
			ObservedAt: u.ChannelPost.Date,
			Symbol:     u.ChannelPost.Chat.Title,
			Source:     "telegram",
			Kind:       KindAttention,
			Title:      u.ChannelPost.Chat.Title,
			Text:       u.ChannelPost.Text,
			URL:        "", // 刻意留空
		})
	}
	return out, next, nil
}
