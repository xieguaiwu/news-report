package output

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"news-report/internal/classify"
	"news-report/internal/config"
	"news-report/internal/report"
)

func sampleReport() *report.Report {
	now := time.Now()
	return &report.Report{
		Generated:  now,
		Config:     config.Default(),
		RawCount:   100,
		DupRemoved: 5,
		Items: []report.Item{
			{
				Title:      "EU approves new sanctions package with umlauts: Überwachung & Économie",
				URL:        "https://example.com/1",
				SourceName: "BBC World", Lang: "en", Tier: "legacy",
				Category: classify.Politics, Score: 1.0,
				Published: now.Add(-2 * time.Hour), AgeLabel: "2h",
			},
			{
				Title:      "Central bank raises rates",
				URL:        "https://example.com/2",
				Summary:    "The ECB decided to raise interest rates by 25 basis points.",
				SourceName: "FAZ Wirtschaft", Lang: "de", Tier: "legacy",
				Category: classify.Economy, Score: 0.9,
				Published: now.Add(-1 * time.Hour), AgeLabel: "1h",
			},
		},
		SourceStats: []report.SourceStat{
			{ID: "bbc-world", Name: "BBC World", Lang: "en", Tier: "legacy", OK: true, Items: 60},
			{ID: "faz-wirtschaft", Name: "FAZ Wirtschaft", Lang: "de", Tier: "legacy", OK: false, Err: "HTTP 500"},
		},
		Duration: 2 * time.Second,
	}
}

func TestTerminal(t *testing.T) {
	var buf bytes.Buffer
	Terminal(&buf, sampleReport(), false)
	out := buf.String()
	for _, want := range []string{"POLITICS", "ECONOMY", "EU approves", "BBC World", "2h", "example.com/1"} {
		if !strings.Contains(out, want) {
			t.Errorf("终端输出缺少 %q", want)
		}
	}
	if !strings.Contains(out, "来源 1 OK / 1 失败") {
		t.Errorf("统计行错误: %q", firstLine(out))
	}
	if strings.Contains(out, "\x1b[") {
		t.Error("color=false 不应输出 ANSI 转义")
	}
}

func TestTerminalColor(t *testing.T) {
	var buf bytes.Buffer
	Terminal(&buf, sampleReport(), true)
	if !strings.Contains(buf.String(), "\x1b[") {
		t.Error("color=true 应输出 ANSI 转义")
	}
}

func TestMarkdown(t *testing.T) {
	var buf bytes.Buffer
	Markdown(&buf, sampleReport())
	out := buf.String()
	for _, want := range []string{
		"# News Report", "POLITICS", "EU approves", "原文链接", "来源统计",
		"| BBC World |", "✗ HTTP 500", "news-report v0.1.0",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("Markdown 输出缺少 %q", want)
		}
	}
}

func TestJSON(t *testing.T) {
	var buf bytes.Buffer
	if err := JSON(&buf, sampleReport()); err != nil {
		t.Fatalf("JSON: %v", err)
	}
	var decoded map[string]any
	if err := json.Unmarshal(buf.Bytes(), &decoded); err != nil {
		t.Fatalf("JSON 解析失败: %v", err)
	}
	items, ok := decoded["Items"].([]any)
	if !ok || len(items) != 2 {
		t.Errorf("Items 字段异常: %v", decoded["Items"])
	}
	if decoded["RawCount"].(float64) != 100 {
		t.Errorf("RawCount 错误: %v", decoded["RawCount"])
	}
	// 摘要字段应包含分类与链接
	first := items[0].(map[string]any)
	if first["Title"] == "" || first["URL"] == "" {
		t.Errorf("条目缺少 Title/URL: %v", first)
	}
}

func TestTruncateRunes(t *testing.T) {
	// 德语/法语多字节字符：按字符截断不能切断 UTF-8
	s := "Überwachung: Économie française — 中文"
	if got := truncate(s, 10); len([]rune(got)) > 11 {
		t.Errorf("truncate 应限制在 11 个 rune 内: %q (%d runes)", got, len([]rune(got)))
	}
	if got := truncate(s, 0); got != s {
		t.Errorf("truncate(0) 应返回原串")
	}
	if got := truncate(s, 1000); got != s {
		t.Errorf("truncate(大数) 应返回原串")
	}
	// 不产生无效 UTF-8
	if got := truncate(s, 7); !strings.Contains(got, "…") || !validUTF8(got) {
		t.Errorf("截断结果应为合法 UTF-8: %q", got)
	}
}

func validUTF8(s string) bool {
	return strings.ToValidUTF8(s, "\uFFFD") == s
}

func firstLine(s string) string {
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return s[:i]
	}
	return s
}
