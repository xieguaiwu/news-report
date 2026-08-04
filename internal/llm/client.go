// Package llm 提供 OpenAI 兼容 /chat/completions 客户端及新闻专用操作：
// 翻译（分块）、摘要（5 要点）、深度解读（四段式）。
// 无 API key 时优雅降级（Chat 返回 ErrNoKey），不阻塞主流程。
package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// ErrNoKey 表示未配置 API key。调用方据此显示可操作提示，不 crash。
var ErrNoKey = errors.New("llm: 未配置 API key，请在 config.yaml 中设置 llm.api_key")

// LLMConfig 来自 config.Config 的 LLM 段（值拷贝，避免外部修改）。
type LLMConfig struct {
	BaseURL    string        // 默认 https://api.openai.com/v1
	APIKey     string        // 已解析 {env:VAR} 后的实际 key，空 = 未配置
	Model      string        // 默认 gpt-4o-mini
	TargetLang string        // 翻译目标语言，默认 zh
	Timeout    time.Duration // 默认 60s
	MaxChars   int           // 翻译前截断字数，0 = 不限，默认 4000
	ChunkSize  int           // 分块大小（字），默认 1500
	Overlap    int           // 块间重叠（字），默认 50
}

// Client 是 LLM 客户端，封装 HTTP 通信、缓存和重试逻辑。
type Client struct {
	cfg   LLMConfig
	http  *http.Client
	cache *Cache
}

// New 创建客户端。apiKey 为空时客户端仍可创建但 Chat() 返回 ErrNoKey。
func New(cfg LLMConfig, cacheDir string) *Client {
	c := &Client{
		cfg:  cfg,
		http: &http.Client{Timeout: cfg.Timeout},
	}
	if cacheDir != "" {
		c.cache = NewCache(cacheDir, 7*24*time.Hour, 20) // 默认 7 天 TTL，20MB
	}
	return c
}

// Available 返回是否已配置 API key（用于 UI 判断是否显示 LLM 功能）。
func (c *Client) Available() bool {
	return c.cfg.APIKey != ""
}

// Timeout 返回超时时间（暴露给调用方创建 context）。
func (c *Client) Timeout() time.Duration {
	return c.cfg.Timeout
}

// Config 返回配置副本（只读用途）。
func (c *Client) Config() LLMConfig {
	return c.cfg
}

// Cache 返回底层缓存实例（供 cache stat 使用）。
func (c *Client) Cache() *Cache {
	return c.cache
}

// ── 底层 Chat 接口 ────────────────────────────────────────────

// chatRequest 是 OpenAI 兼容的请求体。
type chatRequest struct {
	Model       string        `json:"model"`
	Messages    []chatMessage `json:"messages"`
	Temperature float64       `json:"temperature"`
	MaxTokens   int           `json:"max_tokens"`
}

type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// chatResponse 是 OpenAI 兼容的成功响应体。
type chatResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
}

// chatError 是 OpenAI 兼容的错误响应体。
type chatError struct {
	Error struct {
		Message string `json:"message"`
		Type    string `json:"type"`
	} `json:"error"`
}

// Chat 发送单轮对话，返回助手回复文本。
// ctx 由调用方控制超时和取消。
func (c *Client) Chat(ctx context.Context, systemPrompt, userMessage string) (string, error) {
	if !c.Available() {
		return "", ErrNoKey
	}
	req := chatRequest{
		Model: c.cfg.Model,
		Messages: []chatMessage{
			{Role: "system", Content: systemPrompt},
			{Role: "user", Content: userMessage},
		},
		Temperature: 0.3,
		MaxTokens:   2048,
	}
	body, err := json.Marshal(req)
	if err != nil {
		return "", fmt.Errorf("llm: 序列化请求失败: %w", err)
	}

	// 重试逻辑：5xx / 网络错误重试 2 次，4xx 不重试
	const maxRetries = 2
	var lastErr error
	for attempt := 0; attempt <= maxRetries; attempt++ {
		if attempt > 0 {
			// 指数退避: 500ms, 2s
			select {
			case <-ctx.Done():
				return "", ctx.Err()
			case <-time.After(time.Duration(1<<(attempt*2)) * 250 * time.Millisecond):
			}
		}
		resp, err := c.doChat(ctx, body)
		if err == nil {
			return resp, nil
		}
		lastErr = err
		// 4xx 不重试（401/403/429 等）
		if isClientError(err) {
			break
		}
	}
	return "", lastErr
}

// doChat 执行单次 HTTP 请求并解析响应。
func (c *Client) doChat(ctx context.Context, body []byte) (string, error) {
	url := c.cfg.BaseURL + "/chat/completions"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("llm: 创建请求失败: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.cfg.APIKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return "", fmt.Errorf("llm: 请求失败: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20)) // 1MB 上限
	if err != nil {
		return "", fmt.Errorf("llm: 读取响应失败: %w", err)
	}

	if resp.StatusCode >= 400 {
		ce := chatError{}
		if json.Unmarshal(respBody, &ce) == nil && ce.Error.Message != "" {
			return "", fmt.Errorf("llm: HTTP %d (%s)", resp.StatusCode, ce.Error.Message)
		}
		return "", fmt.Errorf("llm: HTTP %d", resp.StatusCode)
	}

	cr := chatResponse{}
	if err := json.Unmarshal(respBody, &cr); err != nil {
		return "", fmt.Errorf("llm: 解析响应失败: %w", err)
	}
	if len(cr.Choices) == 0 {
		return "", errors.New("llm: 模型返回空响应")
	}
	result := cr.Choices[0].Message.Content
	if result == "" {
		return "", errors.New("llm: 模型返回空内容")
	}
	return result, nil
}

// isClientError 判断错误是否为 4xx 客户端错误或上下文取消（不应重试）。
func isClientError(err error) bool {
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
		return true
	}
	// 检查是否包含 4xx 状态码（错误格式为 "llm: HTTP 4xx"）
	return strings.Contains(err.Error(), "HTTP 4")
}
