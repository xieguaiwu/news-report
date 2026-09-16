package crypto

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func chatStub(t *testing.T, content string, status int) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if status != http.StatusOK {
			w.WriteHeader(status)
			_, _ = w.Write([]byte(`{"error":{"message":"boom"}}`))
			return
		}
		resp := map[string]any{
			"choices": []map[string]any{
				{"message": map[string]any{"content": content}, "finish_reason": "stop"},
			},
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
}

func TestScoreOneOK(t *testing.T) {
	ts := chatStub(t, `{"tone":2,"narrative":"牛市来了","shill_score":0.8,"specificity":0.5,"source_tier":"自媒体","black_score":0.1}`, http.StatusOK)
	defer ts.Close()
	s := NewScorer(ScorerConfig{BaseURL: ts.URL, APIKey: "k", Model: "m"})
	got, err := s.ScoreOne(context.Background(), AttentionItem{Title: "牛来了"})
	if err != nil {
		t.Fatalf("score: %v", err)
	}
	if got.Tone != 2 || got.Narrative != "牛市来了" || got.ShillScore != 0.8 {
		t.Fatalf("unexpected score: %+v", got)
	}
}

func TestScoreOneNestedObject(t *testing.T) {
	ts := chatStub(t, "```json\n{\"answer\":{\"tone\":-1,\"narrative\":\"骗局\",\"shill_score\":0,\"specificity\":0.2,\"source_tier\":\"不明\",\"black_score\":0.9}}\n```", http.StatusOK)
	defer ts.Close()
	s := NewScorer(ScorerConfig{BaseURL: ts.URL, APIKey: "k", Model: "m"})
	got, err := s.ScoreOne(context.Background(), AttentionItem{Title: "x"})
	if err != nil {
		t.Fatalf("score: %v", err)
	}
	if got.Tone != -1 || got.BlackScore != 0.9 {
		t.Fatalf("nested parse failed: %+v", got)
	}
}

func TestScoreOneClampsOutOfRange(t *testing.T) {
	ts := chatStub(t, `{"tone":9,"narrative":"x","shill_score":1.7,"specificity":-0.3,"source_tier":"zzz","black_score":2}`, http.StatusOK)
	defer ts.Close()
	s := NewScorer(ScorerConfig{BaseURL: ts.URL, APIKey: "k", Model: "m"})
	got, err := s.ScoreOne(context.Background(), AttentionItem{Title: "x"})
	if err != nil {
		t.Fatalf("score: %v", err)
	}
	if got.Tone != 2 {
		t.Fatalf("tone not clamped: %d", got.Tone)
	}
	if got.ShillScore != 1.0 {
		t.Fatalf("shill not clamped: %v", got.ShillScore)
	}
	if got.SourceTier != "不明" {
		t.Fatalf("enum fallback failed: %q", got.SourceTier)
	}
}

func TestScorerUnavailableWithoutKey(t *testing.T) {
	s := NewScorer(ScorerConfig{BaseURL: "http://x", APIKey: ""})
	if s.Available() {
		t.Fatal("expected unavailable")
	}
}
