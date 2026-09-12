package astock

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func newTestScorer(ts *httptest.Server) *Scorer {
	s := NewScorer(ScorerConfig{BaseURL: ts.URL, APIKey: "test-key", Model: "test-model", Timeout: 3 * time.Second})
	s.SetLogf(func(string, ...any) {})
	return s
}

func testItem() NewsItem {
	return NewsItem{Date: "2026-09-12", Sym: "sz000001", SourceType: "news",
		Title: "某银行获政府补助1.2亿元", Text: "公司公告称收到补助并计入当期损益。", URL: "http://x/1"}
}

// chatStub 构造 /chat/completions stub。
func chatStub(handler func(w http.ResponseWriter, r *http.Request, body scoreChatRequest)) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body scoreChatRequest
		_ = json.NewDecoder(r.Body).Decode(&body)
		handler(w, r, body)
	}))
}

func writeChat(w http.ResponseWriter, content string) {
	_ = json.NewEncoder(w).Encode(map[string]any{
		"choices": []map[string]any{
			{"message": map[string]any{"content": content}, "finish_reason": "stop"},
		},
	})
}

// TestScoreOneOK 正常打分 + 断言 temperature=0 / json_object / reasoning_effort。
func TestScoreOneOK(t *testing.T) {
	ts := chatStub(func(w http.ResponseWriter, r *http.Request, body scoreChatRequest) {
		if body.Temperature != 0 {
			t.Errorf("temperature = %v, want 0", body.Temperature)
		}
		if body.ResponseFormat == nil || body.ResponseFormat.Type != "json_object" {
			t.Errorf("response_format 缺失")
		}
		if body.ReasoningEffort != "low" {
			t.Errorf("reasoning_effort = %q, want low（glm-5.3-flash 始终思考）", body.ReasoningEffort)
		}
		if !strings.Contains(body.Messages[0].Content, "black_score") {
			t.Errorf("system prompt 缺字段说明")
		}
		writeChat(w, `{"tone":1,"kind":"业绩","specificity":0.9,"source_tier":"官方","black_score":0.1}`)
	})
	defer ts.Close()
	sc, err := newTestScorer(ts).ScoreOne(context.Background(), testItem())
	if err != nil {
		t.Fatal(err)
	}
	if sc.Tone != 1 || sc.Kind != "业绩" || sc.Specificity != 0.9 || sc.SourceTier != "官方" || sc.BlackScore != 0.1 {
		t.Errorf("打分错误: %+v", sc)
	}
}

// TestParseScoreVariants 兼容嵌套对象 / 围栏 / 杂文本 / 越界值。
func TestParseScoreVariants(t *testing.T) {
	cases := []struct {
		in    string
		tone  int
		kind  string
		spec  float64
		tier  string
		black float64
		isErr bool
	}{
		{`{"tone":1,"kind":"业绩","specificity":0.9,"source_tier":"官方","black_score":0.1}`, 1, "业绩", 0.9, "官方", 0.1, false},
		{`{"answer":{"tone":-2,"kind":"监管","specificity":1,"source_tier":"官方","black_score":0}}`, -2, "监管", 1, "官方", 0, false}, // 嵌套
		{"```json\n{\"tone\":0,\"kind\":\"其他\",\"specificity\":0.5,\"source_tier\":\"媒体\",\"black_score\":0.5}\n```", 0, "其他", 0.5, "媒体", 0.5, false},
		{`前置说明 {"tone":99,"kind":"无效枚举","specificity":-3,"source_tier":"x","black_score":7} 后缀`, 2, "其他", 0, "不明", 1, false}, // 夹紧
		{`{"tone":"1","kind":"研报","specificity":"0.7","source_tier":"媒体","black_score":"0.2"}`, 1, "研报", 0.7, "媒体", 0.2, false},  // 字符串数字
		{`不是 JSON`, 0, "", 0, "", 0, true},
		{`{"kind":"业绩"}`, 0, "", 0, "", 0, true}, // 缺 tone
	}
	for i, c := range cases {
		sc, err := parseScore(c.in)
		if c.isErr {
			if err == nil {
				t.Errorf("样本 %d 期望报错", i+1)
			}
			continue
		}
		if err != nil {
			t.Errorf("样本 %d: %v", i+1, err)
			continue
		}
		if sc.Tone != c.tone || sc.Kind != c.kind || sc.SourceTier != c.tier {
			t.Errorf("样本 %d: got %+v want tone=%d kind=%s tier=%s", i+1, sc, c.tone, c.kind, c.tier)
		}
		if sc.Specificity != c.spec || sc.BlackScore != c.black {
			t.Errorf("样本 %d: got spec=%v black=%v want %v/%v", i+1, sc.Specificity, sc.BlackScore, c.spec, c.black)
		}
	}
}

// TestScoreOneRetryOn429 429 → 退避重试 → 成功。
func TestScoreOneRetryOn429(t *testing.T) {
	var hits int32
	ts := chatStub(func(w http.ResponseWriter, r *http.Request, body scoreChatRequest) {
		if atomic.AddInt32(&hits, 1) == 1 {
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		writeChat(w, `{"tone":0,"kind":"其他","specificity":0.5,"source_tier":"媒体","black_score":0.5}`)
	})
	defer ts.Close()
	sc, err := newTestScorer(ts).ScoreOne(context.Background(), testItem())
	if err != nil {
		t.Fatal(err)
	}
	if sc.Tone != 0 {
		t.Errorf("tone = %d", sc.Tone)
	}
	if atomic.LoadInt32(&hits) != 2 {
		t.Errorf("请求次数 = %d", atomic.LoadInt32(&hits))
	}
}

// TestScoreOneDegradeOn400 400 → 去 reasoning_effort → 再 400 → 去 response_format → 成功。
func TestScoreOneDegradeOn400(t *testing.T) {
	var step int32
	var seen []scoreChatRequest
	ts := chatStub(func(w http.ResponseWriter, r *http.Request, body scoreChatRequest) {
		n := atomic.AddInt32(&step, 1)
		seen = append(seen, body)
		if n < 3 {
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(`{"error":{"message":"不支持"}}`))
			return
		}
		writeChat(w, `{"tone":1,"kind":"重组","specificity":0.8,"source_tier":"官方","black_score":0}`)
	})
	defer ts.Close()
	sc, err := newTestScorer(ts).ScoreOne(context.Background(), testItem())
	if err != nil {
		t.Fatal(err)
	}
	if sc.Kind != "重组" {
		t.Errorf("kind = %q", sc.Kind)
	}
	if len(seen) != 3 {
		t.Fatalf("请求次数 = %d", len(seen))
	}
	if seen[0].ReasoningEffort == "" || seen[1].ReasoningEffort != "" {
		t.Errorf("第 1 级降级未去 reasoning_effort: %v / %v", seen[0].ReasoningEffort, seen[1].ReasoningEffort)
	}
	if seen[1].ResponseFormat == nil || seen[2].ResponseFormat != nil {
		t.Errorf("第 2 级降级未去 response_format")
	}
}

// TestScoreOneEmptyContentRetry 空内容（思考耗尽 token）→ 重试成功。
func TestScoreOneEmptyContentRetry(t *testing.T) {
	var hits int32
	ts := chatStub(func(w http.ResponseWriter, r *http.Request, body scoreChatRequest) {
		if atomic.AddInt32(&hits, 1) == 1 {
			_ = json.NewEncoder(w).Encode(map[string]any{
				"choices": []map[string]any{{"message": map[string]any{"content": ""}, "finish_reason": "length"}}})
			return
		}
		writeChat(w, `{"tone":-1,"kind":"传闻","specificity":0.3,"source_tier":"自媒体","black_score":0.8}`)
	})
	defer ts.Close()
	sc, err := newTestScorer(ts).ScoreOne(context.Background(), testItem())
	if err != nil {
		t.Fatal(err)
	}
	if sc.BlackScore != 0.8 {
		t.Errorf("black_score = %v", sc.BlackScore)
	}
}

// TestScoreOneAuthFail 401 → 4xx 终态（降级字段仍会尝试但最终失败）。
func TestScoreOneAuthFail(t *testing.T) {
	ts := chatStub(func(w http.ResponseWriter, r *http.Request, body scoreChatRequest) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error":{"message":"Invalid token"}}`))
	})
	defer ts.Close()
	_, err := newTestScorer(ts).ScoreOne(context.Background(), testItem())
	if err == nil {
		t.Fatal("期望鉴权失败报错")
	}
}

// TestScoreBatchConcurrency 并发 ≤4 + 单条超时生效。
func TestScoreBatchConcurrency(t *testing.T) {
	var cur, peak int32
	ts := chatStub(func(w http.ResponseWriter, r *http.Request, body scoreChatRequest) {
		n := atomic.AddInt32(&cur, 1)
		for {
			p := atomic.LoadInt32(&peak)
			if n <= p || atomic.CompareAndSwapInt32(&peak, p, n) {
				break
			}
		}
		time.Sleep(50 * time.Millisecond)
		atomic.AddInt32(&cur, -1)
		writeChat(w, `{"tone":0,"kind":"其他","specificity":0.5,"source_tier":"不明","black_score":0.5}`)
	})
	defer ts.Close()
	items := make([]NewsItem, 20)
	for i := range items {
		items[i] = testItem()
	}
	start := time.Now()
	results := newTestScorer(ts).ScoreBatch(context.Background(), items)
	if len(results) != 20 {
		t.Fatalf("结果数 = %d", len(results))
	}
	for i, r := range results {
		if r.Err != nil {
			t.Fatalf("第 %d 条失败: %v", i, r.Err)
		}
	}
	if atomic.LoadInt32(&peak) > scoreConcurrency {
		t.Errorf("峰值并发 %d > %d", peak, scoreConcurrency)
	}
	// 20 条 / 并发 4 / 每条 50ms ≈ 250ms；若串行需 1s+
	if elapsed := time.Since(start); elapsed > 900*time.Millisecond {
		t.Errorf("疑似未并发: %v", elapsed)
	}
}

// TestScoreBatchTimeout 单条 30s 超时在慢服务端触发（用短 ctx 模拟）。
func TestScoreBatchTimeout(t *testing.T) {
	ts := chatStub(func(w http.ResponseWriter, r *http.Request, body scoreChatRequest) {
		select {
		case <-r.Context().Done():
			return
		case <-time.After(2 * time.Second):
			writeChat(w, `{"tone":0,"kind":"其他","specificity":0.5,"source_tier":"不明","black_score":0.5}`)
		}
	})
	defer ts.Close()
	s := newTestScorer(ts)
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	_, err := s.ScoreOne(ctx, testItem())
	if err == nil {
		t.Fatal("期望超时报错")
	}
}

// TestScorerUnavailable 未注入凭据。
func TestScorerUnavailable(t *testing.T) {
	s := NewScorer(ScorerConfig{})
	if s.Available() {
		t.Error("空配置不应可用")
	}
	_, err := s.ScoreOne(context.Background(), testItem())
	if !errors.Is(err, err) || err == nil {
		t.Fatal("未配置凭据应报错")
	}
	if fmt.Sprint(err) == "" {
		t.Fatal("错误信息为空")
	}
}
