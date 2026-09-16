// internal/crypto
//
// Package crypto 为 news-report 提供加密 meme 币注意力监测能力：
//
//   - 数据源：DexScreener / GeckoTerminal（行情）、GoPlus / Honeypot.is（安全）、
//     Telegram / 微博热搜（注意力）。
//   - LLM 结构化打分：OpenAI 兼容客户端（移植自 internal/astock/llmscore.go）。
//   - JSONL 归档：一行一条观测，供 Memekrieg 侧消费。
//
// 边界：本包只做公开数据观测。禁止加入任何交易执行路径。
//
// 代理要求：出网经 HTTPS_PROXY（Go net/http 自动识别 http.ProxyFromEnvironment）。
// 凭据注入：仅经环境变量 CRYPTO_LLM_BASE_URL / CRYPTO_LLM_API_KEY / CRYPTO_TG_BOT_TOKEN。
package crypto

import (
	"fmt"
)

const SchemaVersion = "crypto-attention-v1"

// userAgent 是 crypto 包的共享出网 User-Agent（GeckoTerminal 无 UA 会被 Cloudflare 拦）。
const userAgent = "news-report-crypto/1.0 (+https://github.com/xieguaiwu/news-report)"

// maxBodyBytes 是 HTTP 响应体读取上限（防内存炸裂）。
const maxBodyBytes = 8 << 20

// 支持的链标识。ChainNone 仅适用于 Kind==KindAttention。
const (
	ChainBSC    = "bsc"
	ChainSolana = "solana"
	ChainNone   = ""
)

// 观测类型。
const (
	KindMarket    = "market"    // 行情/池子
	KindSafety    = "safety"    // 安全检测
	KindAttention = "attention" // 社交注意力
)

// MissingKeyModel 是打分通道不可用时写入 Row.Model 的哨兵值。
const MissingKeyModel = "MISSING_KEY"

// AttentionItem 是一条未打分的原始观测。
type AttentionItem struct {
	ObservedAt int64              `json:"observed_at"` // UTC 秒
	Chain      string             `json:"chain"`
	TokenAddr  string             `json:"token_addr"`
	Symbol     string             `json:"symbol"`
	Source     string             `json:"source"` // dexscreener|geckoterminal|goplus|honeypot|telegram|weibo
	Kind       string             `json:"kind"`
	Title      string             `json:"title"`
	Text       string             `json:"text"`
	URL        string             `json:"url"`
	Metrics    map[string]float64 `json:"metrics,omitempty"`
}

// Score 是 LLM 打分结果。字段语义对齐 astock，新增 Narrative 与 ShillScore。
type Score struct {
	Tone        int     `json:"tone"`         // -2..2
	Narrative   string  `json:"narrative"`    // 叙事标签，如 "牛市来了" / "动物币"
	ShillScore  float64 `json:"shill_score"`  // 0..1 喊单/蛊惑强度
	Specificity float64 `json:"specificity"`  // 0..1 信息具体程度
	SourceTier  string  `json:"source_tier"`  // 官方|媒体|自媒体|不明
	BlackScore  float64 `json:"black_score"`  // 0..1 低级黑
}

// Row 是 JSONL 输出的一行。未打分行 Tone/ShillScore/Specificity/BlackScore 为 null。
//
// 时间语义（GC 7）：EventAt = 事件自身时间；CollectedAt = 采集时刻。两列均不得缺失。
// 对无事件时间的源（微博热搜、安全检测）令 EventAt = CollectedAt。
type Row struct {
	SchemaVersion string             `json:"schema_version"`
	EventAt       int64              `json:"event_at"`
	CollectedAt   int64              `json:"collected_at"`
	Chain         string             `json:"chain"`
	TokenAddr     string             `json:"token_addr"`
	Symbol        string             `json:"symbol"`
	Source        string             `json:"source"`
	Kind          string             `json:"kind"`
	Title         string             `json:"title"`
	Text          string             `json:"text"`
	URL           string             `json:"url"`
	Metrics       map[string]float64 `json:"metrics,omitempty"`
	Tone          *int               `json:"tone"`
	Narrative     string             `json:"narrative"`
	ShillScore    *float64           `json:"shill_score"`
	Specificity   *float64           `json:"specificity"`
	SourceTier    string             `json:"source_tier"`
	BlackScore    *float64           `json:"black_score"`
	ScoredAt      string             `json:"scored_at"`
	Model         string             `json:"model"`
	PromptVersion string             `json:"prompt_version"`
}

func (r Row) Validate() error {
	if r.SchemaVersion != SchemaVersion {
		return fmt.Errorf("schema_version: got %q want %q", r.SchemaVersion, SchemaVersion)
	}
	switch r.Kind {
	case KindMarket, KindSafety, KindAttention:
	default:
		return fmt.Errorf("unsupported kind %q", r.Kind)
	}
	switch r.Chain {
	case ChainBSC, ChainSolana:
		if r.Kind == KindAttention {
			return fmt.Errorf("attention rows must not carry a chain, got %q", r.Chain)
		}
	case ChainNone:
		if r.Kind != KindAttention {
			return fmt.Errorf("chain required for kind %q", r.Kind)
		}
	default:
		return fmt.Errorf("unsupported chain %q", r.Chain)
	}
	if r.CollectedAt <= 0 {
		return fmt.Errorf("collected_at required")
	}
	if r.EventAt <= 0 {
		return fmt.Errorf("event_at required")
	}
	return nil
}
