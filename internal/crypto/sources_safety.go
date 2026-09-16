package crypto

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

const (
	GoPlusChainBSC     = 56
	GoPlusChainSolana  = 101
	goPlusAPIHost      = "https://api.gopluslabs.io"
	goPlusSecurityPath = "/api/v1/token_security/%d"
	honeypotURLv2      = "https://api.honeypot.is/v2/IsHoneypot"
)

type goPlusResp struct {
	Code   int                        `json:"code"`
	Result map[string]goPlusTokenInfo `json:"result"`
}

type goPlusTokenInfo struct {
	BuyTax         string `json:"buy_tax"`
	SellTax        string `json:"sell_tax"`
	CreatorAddress string `json:"creator_address"`
	CreatorPercent string `json:"creator_percent"`
	IsOpenSource   string `json:"is_open_source"`
	CannotSellAll  string `json:"cannot_sell_all"`
	HolderCount    string `json:"holder_count"`
}

// GoPlusTokenSecurity 查询代币安全字段。chainID 用 GoPlusChainBSC / GoPlusChainSolana。
func GoPlusTokenSecurity(ctx context.Context, c *http.Client, chainID int, tokenAddr string) (AttentionItem, error) {
	if chainID != GoPlusChainBSC && chainID != GoPlusChainSolana {
		return AttentionItem{}, fmt.Errorf("goplus: unsupported chain id %d", chainID)
	}
	return goPlusAt(ctx, c, goPlusAPIHost, chainID, tokenAddr)
}

// goPlusAt 的 host 可注入，供测试替换端点。
func goPlusAt(ctx context.Context, c *http.Client, host string, chainID int, tokenAddr string) (AttentionItem, error) {
	rawURL := host + fmt.Sprintf(goPlusSecurityPath, chainID) +
		"?contract_addresses=" + url.QueryEscape(tokenAddr)
	body, err := getWithRetry(ctx, c, rawURL)
	if err != nil {
		return AttentionItem{}, err
	}
	var resp goPlusResp
	if err := json.Unmarshal(body, &resp); err != nil {
		return AttentionItem{}, fmt.Errorf("goplus decode: %w", err)
	}
	info, ok := resp.Result[tokenAddr]
	if !ok {
		for _, v := range resp.Result {
			info, ok = v, true
			break
		}
	}
	if !ok {
		return AttentionItem{}, fmt.Errorf("goplus: token %s not in result", tokenAddr)
	}
	chain := ChainBSC
	if chainID == GoPlusChainSolana {
		chain = ChainSolana
	}
	return AttentionItem{
		ObservedAt: time.Now().Unix(), // 安全检测无独立事件时间
		Chain:      chain,
		TokenAddr:  tokenAddr,
		Symbol:     tokenAddr,
		Source:     "goplus",
		Kind:       KindSafety,
		Title:      "token security",
		URL:        rawURL,
		Metrics: map[string]float64{
			"buy_tax":         parseFloatOrZero(info.BuyTax),
			"sell_tax":        parseFloatOrZero(info.SellTax),
			"creator_percent": parseFloatOrZero(info.CreatorPercent),
			"is_open_source":  parseFloatOrZero(info.IsOpenSource),
			"cannot_sell_all": parseFloatOrZero(info.CannotSellAll),
			"holder_count":    parseFloatOrZero(info.HolderCount),
		},
	}, nil
}

func parseFloatOrZero(s string) float64 {
	v, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0
	}
	return v
}

// honeypotRiskLevel 把风险档映射为数值。未知档位记 -1，便于下游区分「低风险」与「未返回」。
var honeypotRiskLevel = map[string]float64{"low": 0, "medium": 1, "high": 2}

type honeypotResp struct {
	IsHoneypot       bool `json:"isHoneypot"`
	SimulationResult struct {
		BuyTax  float64 `json:"buyTax"`
		SellTax float64 `json:"sellTax"`
	} `json:"simulationResult"`
	ContractCode struct {
		OpenSource bool `json:"openSource"`
		Proxy      bool `json:"proxy"`
	} `json:"contractCode"`
	Summary struct {
		Risk string `json:"risk"`
	} `json:"summary"`
}

// HoneypotCheck 查询 Honeypot.is v2。chain 取 "bsc" 或 "solana"。
func HoneypotCheck(ctx context.Context, c *http.Client, chain, tokenAddr string) (AttentionItem, error) {
	return honeypotCheckAt(ctx, c, honeypotURLv2, chain, tokenAddr)
}

func honeypotCheckAt(ctx context.Context, c *http.Client, base, chain, tokenAddr string) (AttentionItem, error) {
	u := base + "?address=" + url.QueryEscape(tokenAddr) + "&chain=" + url.QueryEscape(chain)
	body, err := getWithRetry(ctx, c, u)
	if err != nil {
		return AttentionItem{}, err
	}
	var resp honeypotResp
	if err := json.Unmarshal(body, &resp); err != nil {
		return AttentionItem{}, fmt.Errorf("honeypot decode: %w", err)
	}
	var hp float64
	if resp.IsHoneypot {
		hp = 1
	}
	var open float64
	if resp.ContractCode.OpenSource {
		open = 1
	}
	risk, ok := honeypotRiskLevel[resp.Summary.Risk]
	if !ok {
		risk = -1
	}
	return AttentionItem{
		ObservedAt: time.Now().Unix(), // 安全检测无独立事件时间
		Chain:      chain,
		TokenAddr:  tokenAddr,
		Symbol:     tokenAddr,
		Source:     "honeypot",
		Kind:       KindSafety,
		Title:      "honeypot check",
		URL:        u,
		Metrics: map[string]float64{
			"is_honeypot": hp,
			"buy_tax":     resp.SimulationResult.BuyTax / 100,
			"sell_tax":    resp.SimulationResult.SellTax / 100,
			"open_source": open,
			"risk_level":  risk,
		},
	}, nil
}
