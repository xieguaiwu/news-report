package astock

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
)

// PromptVersion 打分提示词版本号（prompt 文本有变必须更新此常量，便于 JSONL 追溯）。
const PromptVersion = "astock-llmscore-v1"

// DefaultModel 默认打分模型：bai 网关 glm-5.3-flash（免费通道）。
// 备用通道 dashscope/qwen（token-plan compatible-mode）仅注释说明，见 ASTOCK_NOTES.md。
const DefaultModel = "glm-5.3-flash"

// 凭据环境变量（由 scripts/astock_env.sh 从 /root/.pi/agent/models.json 的 bai 条目
// 解析导出；值不 echo、不落盘、不提交）。
const (
	EnvBaseURL = "ASTOCK_LLM_BASE_URL"
	EnvAPIKey  = "ASTOCK_LLM_API_KEY"
	EnvModel   = "ASTOCK_LLM_MODEL"
)

// 打分参数约束。
const (
	scoreTimeout    = 30 * time.Second // 单条超时
	scoreConcurrency = 4                // 并发上限
	scoreMaxRetry   = 3                // 总尝试次数（含首发）
	scoreMaxTokens  = 800
	scoreTextLimit  = 1200 // 送入 LLM 的文本上限（字符）
	scoreRetryWait  = 1 * time.Second
)

// ScorerConfig 打分客户端配置。
type ScorerConfig struct {
	BaseURL string // 如 https://api.b.ai/v1（无尾斜杠）
	APIKey  string
	Model   string
	Timeout time.Duration // 默认 30s
}

// ScorerConfigFromEnv 从环境变量读配置（scripts/astock_env.sh 注入）。
func ScorerConfigFromEnv() ScorerConfig {
	return ScorerConfig{
		BaseURL: strings.TrimRight(os.Getenv(EnvBaseURL), "/"),
		APIKey:  os.Getenv(EnvAPIKey),
		Model:   os.Getenv(EnvModel),
	}
}

// Available 凭据是否已注入。
func (c ScorerConfig) Available() bool { return c.BaseURL != "" && c.APIKey != "" }

// Scorer 是 OpenAI 兼容 /chat/completions 打分客户端。
type Scorer struct {
	cfg  ScorerConfig
	http *http.Client
	logf func(format string, args ...any)
}

// NewScorer 创建客户端。
func NewScorer(cfg ScorerConfig) *Scorer {
	if cfg.Model == "" {
		cfg.Model = DefaultModel
	}
	if cfg.Timeout <= 0 {
		cfg.Timeout = scoreTimeout
	}
	return &Scorer{
		cfg:  cfg,
		http: &http.Client{Timeout: cfg.Timeout + 10*time.Second}, // HTTP 层留缓冲，业务超时用 ctx
		logf: func(string, ...any) {},
	}
}

// SetLogf 注入日志。
func (s *Scorer) SetLogf(f func(format string, args ...any)) {
	if f != nil {
		s.logf = f
	}
}

// Available 凭据是否已注入。
func (s *Scorer) Available() bool { return s.cfg.Available() }

// scorePrompt 是写死的打分提示词（改动须同步 PromptVersion）。
const scoreSystemPrompt = `你是A股新闻/公告舆情打分器。对给定的新闻或公告文本，只输出一个 JSON 对象（无其他文字、无 markdown 代码块），字段如下：
- "tone": 整数 -2..2，舆情倾向：-2 极度利空，-1 偏空，0 中性，1 偏多，2 极度利多
- "kind": 从以下枚举中选一：业绩、监管、重组、传闻、研报、自媒体、其他
- "specificity": 0..1 小数，信息具体程度：有具体数字/主体/时间≈0.8-1，空话套话/情绪渲染≈0-0.3
- "source_tier": 从以下枚举中选一：官方（公告/监管/交易所）、媒体（署名新闻机构）、自媒体（股吧/公众号腔调）、不明
- "black_score": 0..1 小数，"低级黑"程度：同时满足 论据缺失 + 情绪化渲染 + 恐慌/亢奋诱导 则趋近 1；客观陈述趋近 0`

func scoreUserPrompt(it NewsItem) string {
	text := it.Text
	if text == "" {
		text = it.Title
	}
	if len([]rune(text)) > scoreTextLimit {
		text = string([]rune(text)[:scoreTextLimit]) + "…"
	}
	return fmt.Sprintf("新闻来源类型：%s\n日期：%s\n标题：%s\n正文：%s\n\n请输出打分 JSON 对象。",
		it.SourceType, it.Date, it.Title, text)
}

// ── 请求/响应结构 ─────────────────────────────────────────────

type scoreChatRequest struct {
	Model           string          `json:"model"`
	Temperature     float64         `json:"temperature"` // 强制 0
	MaxTokens       int             `json:"max_tokens"`
	ReasoningEffort string          `json:"reasoning_effort,omitempty"` // glm-5.3-flash 始终思考，用 low 限耗
	ResponseFormat  *scoreRespFmt   `json:"response_format,omitempty"`
	Messages        []scoreChatMsg  `json:"messages"`
}

type scoreRespFmt struct {
	Type string `json:"type"` // json_object
}

type scoreChatMsg struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type scoreChatResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
		FinishReason string `json:"finish_reason"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error"`
}

// ScoreBatch 并发打分（并发 ≤ scoreConcurrency，单条超时 scoreTimeout）。
// 返回与 items 等长的结果切片；失败项 Err 非空、Score 为零值。
func (s *Scorer) ScoreBatch(ctx context.Context, items []NewsItem) []ScoreResult {
	results := make([]ScoreResult, len(items))
	if len(items) == 0 {
		return results
	}
	sem := make(chan struct{}, scoreConcurrency)
	var wg sync.WaitGroup
	for i := range items {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			ictx, cancel := context.WithTimeout(ctx, scoreTimeout)
			defer cancel()
			sc, err := s.ScoreOne(ictx, items[i])
			results[i] = ScoreResult{Index: i, Score: sc, Err: err}
		}(i)
	}
	wg.Wait()
	return results
}

// ScoreResult 单条打分结果。
type ScoreResult struct {
	Index int
	Score Score
	Err   error
}

// ScoreOne 打分一条：总尝试 ≤ scoreMaxRetry；
// 429/5xx/网络错误 → 指数退避重试；400 → 逐级降级请求体（去 reasoning_effort /
// 去 response_format，网关兼容性兜底）；成功响应但内容为空（思考耗尽 token）→ 重试。
func (s *Scorer) ScoreOne(ctx context.Context, it NewsItem) (Score, error) {
	var zero Score
	if !s.Available() {
		return zero, errors.New("astock: 未配置打分凭据（ASTOCK_LLM_API_KEY）")
	}
	base := scoreChatRequest{
		Model:           s.cfg.Model,
		Temperature:     0,
		MaxTokens:       scoreMaxTokens,
		ReasoningEffort: "low",
		ResponseFormat:  &scoreRespFmt{Type: "json_object"},
		Messages: []scoreChatMsg{
			{Role: "system", Content: scoreSystemPrompt},
			{Role: "user", Content: scoreUserPrompt(it)},
		},
	}
	var lastErr error
	backoff := scoreRetryWait
	for attempt := 1; attempt <= scoreMaxRetry; attempt++ {
		if attempt > 1 {
			s.logf("astock: 打分重试第 %d 次（退避 %s）", attempt, backoff)
			select {
			case <-ctx.Done():
				return zero, ctx.Err()
			case <-time.After(backoff):
			}
			backoff *= 3
		}
		// 400 → 逐级降级字段（兼容网关差异）；其余错误原样重试
		sc, err := s.attempt(ctx, base)
		if err == nil {
			return sc, nil
		}
		var httpErr *scoreHTTPError
		if errors.As(err, &httpErr) && httpErr.StatusCode == http.StatusBadRequest {
			if base.ResponseFormat != nil || base.ReasoningEffort != "" {
				if base.ReasoningEffort != "" {
					base.ReasoningEffort = "" // 第 1 级降级：去 reasoning_effort
				} else {
					base.ResponseFormat = nil // 第 2 级降级：去 response_format
				}
			}
			lastErr = err
			continue // 降级不占退避
		}
		lastErr = err
	}
	return zero, lastErr
}

// scoreHTTPError 带 HTTP 状态码的错误。
type scoreHTTPError struct {
	StatusCode int
	Body       string
}

func (e *scoreHTTPError) Error() string {
	return fmt.Sprintf("astock: 打分接口 HTTP %d: %s", e.StatusCode, truncate(e.Body, 200))
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}

// attempt 发起一次打分请求并解析。
func (s *Scorer) attempt(ctx context.Context, req scoreChatRequest) (Score, error) {
	var zero Score
	buf, err := json.Marshal(req)
	if err != nil {
		return zero, err
	}
	hreq, err := http.NewRequestWithContext(ctx, http.MethodPost, s.cfg.BaseURL+"/chat/completions", bytes.NewReader(buf))
	if err != nil {
		return zero, err
	}
	hreq.Header.Set("Content-Type", "application/json")
	hreq.Header.Set("Authorization", "Bearer "+s.cfg.APIKey)
	hreq.Header.Set("User-Agent", defaultUA)

	hresp, err := s.http.Do(hreq)
	if err != nil {
		return zero, fmt.Errorf("网络错误: %w", err)
	}
	body, err := io.ReadAll(io.LimitReader(hresp.Body, maxBodyBytes))
	hresp.Body.Close()
	if err != nil {
		return zero, fmt.Errorf("读取响应失败: %w", err)
	}
	sc := hresp.StatusCode
	if sc == http.StatusTooManyRequests || sc >= 500 {
		return zero, &scoreHTTPError{StatusCode: sc, Body: string(body)} // 可重试
	}
	if sc >= 400 {
		return zero, &scoreHTTPError{StatusCode: sc, Body: string(body)} // 4xx（含 400 降级、其余终态）
	}
	var cr scoreChatResponse
	if err := json.Unmarshal(body, &cr); err != nil {
		return zero, fmt.Errorf("响应解析失败: %w", err)
	}
	if cr.Error != nil && cr.Error.Message != "" {
		return zero, fmt.Errorf("接口错误: %s", cr.Error.Message)
	}
	if len(cr.Choices) == 0 {
		return zero, errors.New("响应无 choices")
	}
	content := strings.TrimSpace(cr.Choices[0].Message.Content)
	if content == "" {
		// 思考耗尽 token / 网关截断 → 当可重试错误处理
		return zero, errors.New("打分响应为空（finish=" + cr.Choices[0].FinishReason + "）")
	}
	return parseScore(content)
}

var jsonFenceRe = regexp.MustCompile("(?s)```(?:json)?\\s*(.*?)```")

// parseScore 从模型输出提取并校验打分 JSON。
// 兼容：markdown 代码围栏、前后杂文本、以及模型把字段包进一层嵌套对象（如 {"answer":{...}}）。
func parseScore(content string) (Score, error) {
	var zero Score
	c := strings.TrimSpace(content)
	if m := jsonFenceRe.FindStringSubmatch(c); m != nil {
		c = strings.TrimSpace(m[1])
	}
	// 截取首个 '{' 到末个 '}'
	if i := strings.IndexByte(c, '{'); i >= 0 {
		if j := strings.LastIndexByte(c, '}'); j > i {
			c = c[i : j+1]
		}
	}
	var raw map[string]any
	if err := json.Unmarshal([]byte(c), &raw); err != nil {
		return zero, fmt.Errorf("打分 JSON 解析失败: %w", err)
	}
	// 嵌套一层：找唯一含 tone 键的子对象
	if _, ok := raw["tone"]; !ok {
		for _, v := range raw {
			if sub, ok := v.(map[string]any); ok {
				if _, has := sub["tone"]; has {
					raw = sub
					break
				}
			}
		}
	}
	sc := Score{
		Tone:        clampInt(raw["tone"], -2, 2),
		Kind:        enumOf(raw["kind"], []string{"业绩", "监管", "重组", "传闻", "研报", "自媒体", "其他"}, "其他"),
		Specificity: clampFloat(raw["specificity"]),
		SourceTier:  enumOf(raw["source_tier"], []string{"官方", "媒体", "自媒体", "不明"}, "不明"),
		BlackScore:  clampFloat(raw["black_score"]),
	}
	if raw["tone"] == nil {
		return zero, fmt.Errorf("打分 JSON 缺少 tone 字段: %s", truncate(c, 120))
	}
	return sc, nil
}

// clampInt 容错取整数并夹到 [lo,hi]（接受 float64 / json.Number / 数字字符串）。
func clampInt(v any, lo, hi int) int {
	f, ok := toFloat(v)
	if !ok {
		return 0
	}
	n := int(math.Round(f))
	if n < lo {
		return lo
	}
	if n > hi {
		return hi
	}
	return n
}

// clampFloat 容错取小数并夹到 [0,1]。
func clampFloat(v any) float64 {
	f, ok := toFloat(v)
	if !ok {
		return 0
	}
	if f < 0 {
		return 0
	}
	if f > 1 {
		return 1
	}
	return f
}

func toFloat(v any) (float64, bool) {
	switch x := v.(type) {
	case float64:
		return x, true
	case int:
		return float64(x), true
	case json.Number:
		f, err := x.Float64()
		return f, err == nil
	case string:
		f, err := strconv.ParseFloat(strings.TrimSpace(x), 64)
		return f, err == nil
	}
	return 0, false
}

// enumOf 枚举校验（含 trim），非法回退默认值。
func enumOf(v any, allowed []string, def string) string {
	s, ok := v.(string)
	if !ok {
		return def
	}
	s = strings.TrimSpace(s)
	for _, a := range allowed {
		if s == a {
			return s
		}
	}
	return def
}
