// Package llm — 单元测试：mock HTTP server 覆盖客户端和缓存核心逻辑。
package llm

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// ── 客户端测试 ─────────────────────────────────────────────────

func TestClientAvailable(t *testing.T) {
	c := New(LLMConfig{APIKey: "sk-test"}, "")
	if !c.Available() {
		t.Error("有 key 时应 Available()==true")
	}
	c2 := New(LLMConfig{APIKey: ""}, "")
	if c2.Available() {
		t.Error("无 key 时应 Available()==false")
	}
}

func TestChatNoKey(t *testing.T) {
	c := New(LLMConfig{APIKey: ""}, "")
	_, err := c.Chat(context.Background(), "prompt", "msg")
	if err != ErrNoKey {
		t.Errorf("无 key 时 Chat 应返回 ErrNoKey，实际 %v", err)
	}
}

func TestTranslateNoKey(t *testing.T) {
	c := New(LLMConfig{APIKey: ""}, "")
	_, err := c.Translate(context.Background(), "hello", "en")
	if err != ErrNoKey {
		t.Errorf("无 key 时 Translate 应返回 ErrNoKey，实际 %v", err)
	}
}

func TestSummarizeNoKey(t *testing.T) {
	c := New(LLMConfig{APIKey: ""}, "")
	_, err := c.Summarize(context.Background(), "title", "body")
	if err != ErrNoKey {
		t.Errorf("无 key 时 Summarize 应返回 ErrNoKey，实际 %v", err)
	}
}

func TestBriefNoKey(t *testing.T) {
	c := New(LLMConfig{APIKey: ""}, "")
	_, err := c.Brief(context.Background(), "title", "body")
	if err != ErrNoKey {
		t.Errorf("无 key 时 Brief 应返回 ErrNoKey，实际 %v", err)
	}
}

func TestChatSuccess(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 验证请求格式
		if r.Method != http.MethodPost {
			t.Errorf("期望 POST，实际 %s", r.Method)
		}
		if !strings.HasSuffix(r.URL.Path, "/chat/completions") {
			t.Errorf("期望 /chat/completions，实际 %s", r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer sk-test" {
			t.Errorf("Authorization 头不正确")
		}
		var req chatRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Errorf("请求体解析失败: %v", err)
		}
		if req.Model != "test-model" {
			t.Errorf("Model 应为 test-model，实际 %s", req.Model)
		}

		// 返回成功响应
		json.NewEncoder(w).Encode(chatResponse{
			Choices: []struct {
				Message struct {
					Content string `json:"content"`
				} `json:"message"`
			}{
				{Message: struct {
					Content string `json:"content"`
				}{Content: "这是测试回复"}},
			},
		})
	}))
	defer srv.Close()

	c := New(LLMConfig{
		BaseURL:    srv.URL,
		APIKey:     "sk-test",
		Model:      "test-model",
		Timeout:    10 * time.Second,
		MaxChars:   4000,
		ChunkSize:  1500,
		Overlap:    50,
		TargetLang: "zh",
	}, "")

	resp, err := c.Chat(context.Background(), "system", "hello")
	if err != nil {
		t.Fatalf("Chat 失败: %v", err)
	}
	if resp != "这是测试回复" {
		t.Errorf("期望 '这是测试回复'，实际 %q", resp)
	}
}

func TestChatServerError(t *testing.T) {
	callCount := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		if callCount <= 2 {
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(chatError{Error: struct {
				Message string `json:"message"`
				Type    string `json:"type"`
			}{Message: "server error", Type: "server_error"}})
			return
		}
		// 第 3 次成功
		json.NewEncoder(w).Encode(chatResponse{
			Choices: []struct {
				Message struct {
					Content string `json:"content"`
				} `json:"message"`
			}{
				{Message: struct {
					Content string `json:"content"`
				}{Content: "retry ok"}},
			},
		})
	}))
	defer srv.Close()

	c := New(LLMConfig{
		BaseURL: srv.URL,
		APIKey:  "sk-test",
		Model:   "test-model",
		Timeout: 5 * time.Second,
	}, "")
	resp, err := c.Chat(context.Background(), "system", "hello")
	if err != nil {
		t.Fatalf("重试后应成功: %v", err)
	}
	if resp != "retry ok" {
		t.Errorf("期望 'retry ok'，实际 %q", resp)
	}
	if callCount != 3 {
		t.Errorf("期望调用 3 次（2 次失败+1 次成功），实际 %d", callCount)
	}
}

func TestChatClientErrorNoRetry(t *testing.T) {
	callCount := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(chatError{Error: struct {
			Message string `json:"message"`
			Type    string `json:"type"`
		}{Message: "invalid api key", Type: "invalid_request_error"}})
	}))
	defer srv.Close()

	c := New(LLMConfig{
		BaseURL: srv.URL,
		APIKey:  "sk-bad",
		Model:   "test-model",
		Timeout: 5 * time.Second,
	}, "")
	_, err := c.Chat(context.Background(), "system", "hello")
	if err == nil {
		t.Fatal("401 应返回错误")
	}
	if !strings.Contains(err.Error(), "401") {
		t.Errorf("错误应包含 401，实际 %v", err)
	}
	if callCount != 1 {
		t.Errorf("401 不应重试，实际调用 %d 次", callCount)
	}
}

// ── 翻译分块测试 ────────────────────────────────────────────────

func TestChunkText(t *testing.T) {
	// 构造 3 个段落，每段 500 字
	p1 := strings.Repeat("a", 500)
	p2 := strings.Repeat("b", 500)
	p3 := strings.Repeat("c", 500)
	text := p1 + "\n\n" + p2 + "\n\n" + p3

	// chunkSize=1000, overlap=50 → p1(500)+p2(500)=1002>1000，第一块=仅 p1
	// 第二块=last50(p1)+p2+p3=552+2+500>1000→不合并p3，所以第三块含 p3
	// 总共 3 个块
	chunks := chunkText(text, 1000, 50)
	if len(chunks) < 1 {
		t.Fatalf("至少应有 1 个块，实际 %d", len(chunks))
	}
	// 第一块应包含 p1
	if !strings.Contains(chunks[0], p1) {
		t.Errorf("第一块应包含 p1")
	}
	// 某一块应包含 p3
	found := false
	for _, c := range chunks {
		if strings.Contains(c, p3) {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("应有块包含 p3")
	}
}

func TestChunkTextSmall(t *testing.T) {
	text := "短文本，不分块"
	chunks := chunkText(text, 1500, 50)
	if len(chunks) != 1 {
		t.Fatalf("期望 1 个块，实际 %d", len(chunks))
	}
	if chunks[0] != text {
		t.Errorf("内容应为原文本")
	}
}

func TestChunkTextEmpty(t *testing.T) {
	chunks := chunkText("", 1500, 50)
	if len(chunks) != 0 {
		t.Errorf("空文本应无块，实际 %d 块", len(chunks))
	}
}

func TestLastNRunes(t *testing.T) {
	s := "Hello世界！"
	// runes: H e l l o 世 界 ！ (8 个)，最后 3 个是 "世界！"
	result := lastNRunes(s, 3)
	if result != "世界！" {
		t.Errorf("lastNRunes('Hello世界！', 3) = %q，期望 '世界！'", result)
	}
	// 小于总长度时取末尾部分
	result2 := lastNRunes(s, 2)
	if result2 != "界！" {
		t.Errorf("lastNRunes('Hello世界！', 2) = %q，期望 '界！'", result2)
	}
}

// ── 缓存测试 ────────────────────────────────────────────────────

func TestCacheSetGet(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "llm_cache")
	cc := NewCache(dir, 1*time.Hour, 10)
	defer cc.Clear()

	key := CacheKey("https://example.com", "en", InstTranslate)
	if err := cc.Set(key, "翻译结果"); err != nil {
		t.Fatalf("Set: %v", err)
	}
	got, ok := cc.Get(key)
	if !ok {
		t.Fatal("应命中缓存")
	}
	if got != "翻译结果" {
		t.Errorf("期望 '翻译结果'，实际 %q", got)
	}
}

func TestCacheMiss(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "llm_cache")
	cc := NewCache(dir, 1*time.Hour, 10)
	defer cc.Clear()

	_, ok := cc.Get("no-such-key")
	if ok {
		t.Error("不存在的 key 不应命中")
	}
}

func TestCacheExpiry(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "llm_cache")
	cc := NewCache(dir, 10*time.Millisecond, 10)
	defer cc.Clear()

	key := CacheKey("https://example.com", "en", InstSummary)
	if err := cc.Set(key, "摘要内容"); err != nil {
		t.Fatalf("Set: %v", err)
	}
	// 等待过期
	time.Sleep(20 * time.Millisecond)
	_, ok := cc.Get(key)
	if ok {
		t.Error("过期缓存应不命中")
	}
}

func TestCacheKeyDeterministic(t *testing.T) {
	k1 := CacheKey("https://example.com", "en", InstTranslate)
	k2 := CacheKey("https://example.com", "en", InstTranslate)
	if k1 != k2 {
		t.Errorf("相同输入应产生相同 key: %q vs %q", k1, k2)
	}
	k3 := CacheKey("https://other.com", "en", InstTranslate)
	if k1 == k3 {
		t.Error("不同 URL 应产生不同 key")
	}
	k4 := CacheKey("https://example.com", "de", InstTranslate)
	if k1 == k4 {
		t.Error("不同语言应产生不同 key")
	}
}

func TestCacheKeyLength(t *testing.T) {
	key := CacheKey("https://example.com", "en", InstTranslate)
	if len(key) != 64 {
		t.Errorf("SHA256 hex key 长度应为 64，实际 %d", len(key))
	}
}

// ── 摘要解析测试 ────────────────────────────────────────────────

func TestParseNumberedLines(t *testing.T) {
	input := "1. 第一要点\n2. 第二要点\n3. 第三要点\n4. 第四要点\n5. 第五要点"
	result := parseNumberedLines(input)
	if len(result) != 5 {
		t.Fatalf("期望 5 条，实际 %d: %v", len(result), result)
	}
}

func TestParseNumberedLinesPartial(t *testing.T) {
	input := "1. 只有三条\n2. 第二条\n3. 最后一条"
	result := parseNumberedLines(input)
	if len(result) != 3 {
		t.Fatalf("期望 3 条，实际 %d", len(result))
	}
}

func TestParseNumberedLinesFallback(t *testing.T) {
	input := "没有序号行的自由文本"
	result := parseNumberedLines(input)
	if len(result) != 1 {
		t.Fatalf("期望 1 条 fallback，实际 %d", len(result))
	}
	if result[0] != input {
		t.Errorf("fallback 应为原文，实际 %q", result[0])
	}
}

// ── 解读解析测试 ────────────────────────────────────────────────

func TestParseBriefResult(t *testing.T) {
	input := `## 背景
这是背景描述。

## 各方立场
甲方立场是xxx。
乙方立场是yyy。

## 影响
可能的影响分析。

## 后续关注
建议关注abc和def。`

	result := parseBriefResult(input)
	if !strings.Contains(result.Background, "背景描述") {
		t.Errorf("背景解析错误: %q", result.Background)
	}
	if !strings.Contains(result.Positions, "甲方立场") {
		t.Errorf("立场解析错误: %q", result.Positions)
	}
	if !strings.Contains(result.Impact, "影响分析") {
		t.Errorf("影响解析错误: %q", result.Impact)
	}
	if !strings.Contains(result.Outlook, "abc") {
		t.Errorf("后续解析错误: %q", result.Outlook)
	}
}

// ── buildArticleMsg 测试 ─────────────────────────────────────────

func TestBuildArticleMsg(t *testing.T) {
	msg := buildArticleMsg("测试标题", "")
	if msg != "标题：测试标题" {
		t.Errorf("无正文时格式错误: %q", msg)
	}
	msg2 := buildArticleMsg("测试标题", "短正文")
	if !strings.Contains(msg2, "标题：测试标题") || !strings.Contains(msg2, "短正文") {
		t.Errorf("有正文时格式错误: %q", msg2)
	}
}

// ── 集成：mock 翻译调用 ─────────────────────────────────────────

func TestTranslateWithMock(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(chatResponse{
			Choices: []struct {
				Message struct {
					Content string `json:"content"`
				} `json:"message"`
			}{
				{Message: struct {
					Content string `json:"content"`
				}{Content: "翻译后的文本"}},
			},
		})
	}))
	defer srv.Close()

	c := New(LLMConfig{
		BaseURL:   srv.URL,
		APIKey:    "sk-test",
		Model:     "test-model",
		Timeout:   10 * time.Second,
		MaxChars:  4000,
		ChunkSize: 1500,
		Overlap:   50,
	}, "")

	result, err := c.Translate(context.Background(), "Hello world", "en")
	if err != nil {
		t.Fatalf("Translate 失败: %v", err)
	}
	if result != "翻译后的文本" {
		t.Errorf("期望 '翻译后的文本'，实际 %q", result)
	}
}

// ── Parge 测试 ──────────────────────────────────────────────────

func TestCachePurge(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "llm_cache")
	cc := NewCache(dir, 1*time.Hour, 1) // 1MB 上限
	defer cc.Clear()

	// 写入 >1MB 的文件，应触发 LRU 淘汰
	bigValue := strings.Repeat("x", 1024*1024*2) // 2MB
	key := CacheKey("https://example.com", "en", InstBrief)
	if err := cc.Set(key, bigValue); err != nil {
		t.Fatalf("Set: %v", err)
	}

	// Purge 应清空（2MB > 1MB 上限）
	n, err := cc.Purge()
	if err != nil {
		t.Fatalf("Purge: %v", err)
	}
	if n != 1 {
		t.Errorf("应删除 1 个文件，实际 %d", n)
	}
	_, ok := cc.Get(key)
	if ok {
		t.Error("purge 后应不命中")
	}
}

func TestCachePurgeUnderLimit(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "llm_cache")
	cc := NewCache(dir, 1*time.Hour, 10) // 10MB 上限
	defer cc.Clear()

	// 写入 <10MB 的文件，不应触发淘汰
	smallValue := strings.Repeat("x", 1024*512) // 512KB
	key := CacheKey("https://example.com", "en", InstBrief)
	if err := cc.Set(key, smallValue); err != nil {
		t.Fatalf("Set: %v", err)
	}
	n, err := cc.Purge()
	if err != nil {
		t.Fatalf("Purge: %v", err)
	}
	if n != 0 {
		t.Errorf("512KB < 10MB 上限不应淘汰，实际删除 %d", n)
	}
	_, ok := cc.Get(key)
	if !ok {
		t.Error("未超限时缓存应保留")
	}
}

// ── 空文本翻译 ──────────────────────────────────────────────────

func TestTranslateEmptyText(t *testing.T) {
	c := New(LLMConfig{APIKey: "sk-test", MaxChars: 4000}, "")
	result, err := c.Translate(context.Background(), "", "en")
	if err != nil {
		t.Fatalf("空文本翻译不应报错，实际 %v", err)
	}
	if result != "" {
		t.Errorf("空文本翻译应返回空字符串，实际 %q", result)
	}
}

// ── 清理 ────────────────────────────────────────────────────────

func TestMain(m *testing.M) {
	// 确保测试不遗留缓存文件
	os.Exit(m.Run())
}
