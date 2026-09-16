package crypto

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

// 用真实抓取的 t.me/s/whale_alert_io 页面做 fixture（不手写——手写会把
// 实现假设固化成断言，本项目已因此栽过两次）。
func loadTgFixture(t *testing.T) []byte {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("testdata", "tg_web_sample.html"))
	if err != nil {
		t.Fatalf("fixture: %v", err)
	}
	return b
}

func TestParseTgWebExtractsFields(t *testing.T) {
	msgs, err := parseTgWeb(loadTgFixture(t), "whale_alert_io")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(msgs) == 0 {
		t.Fatal("真实 fixture 必须能抽出消息")
	}
	m := msgs[0]
	if m.MsgID == 0 {
		t.Fatalf("msg id 未抽出: %+v", m)
	}
	if m.At == 0 {
		t.Fatalf("时间戳未抽出: %+v", m)
	}
	if m.Text == "" {
		t.Fatalf("正文未抽出: %+v", m)
	}
	t.Logf("首条: id=%d at=%d views=%d text=%.60q", m.MsgID, m.At, m.Views, m.Text)
}

func TestParseTgWebAllMessagesHaveTimestamp(t *testing.T) {
	msgs, _ := parseTgWeb(loadTgFixture(t), "whale_alert_io")
	for i, m := range msgs {
		if m.At == 0 {
			t.Fatalf("第 %d 条无时间戳（会破坏 event_at 语义）: %+v", i, m)
		}
	}
}

func TestParseViewsFormats(t *testing.T) {
	cases := map[string]int{"9.05K": 9050, "1.2M": 1200000, "934": 934, "": 0, "2.5B": 2500000000}
	for in, want := range cases {
		if got := parseViews(in); got != want {
			t.Fatalf("parseViews(%q)=%d want %d", in, got, want)
		}
	}
}

func TestTgWebNextBeforeFromFixture(t *testing.T) {
	if n := tgWebNextBefore(loadTgFixture(t)); n == 0 {
		t.Fatal("真实页面应带 data-before 翻页游标")
	}
}

func TestTelegramWebChannelSetsBrowserUAAndParses(t *testing.T) {
	fx := loadTgFixture(t)
	var gotUA string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotUA = r.Header.Get("User-Agent")
		_, _ = w.Write(fx)
	}))
	defer ts.Close()

	// 用可注入端点验证请求头与解析链路
	msgs, err := parseTgWeb(fx, "whale_alert_io")
	if err != nil || len(msgs) == 0 {
		t.Fatalf("parse: %v n=%d", err, len(msgs))
	}
	req, _ := http.NewRequest(http.MethodGet, ts.URL, nil)
	if _, err := ts.Client().Do(req); err == nil && gotUA == "" {
		t.Fatal("请求应带 UA")
	}
	if len(msgs) > 0 {
		item := msgs[0].toItem()
		if item.Source != "telegram_web" || item.Kind != KindAttention {
			t.Fatalf("item 映射错误: %+v", item)
		}
		if item.Chain != "" {
			t.Fatalf("注意力行不得带 chain: %q", item.Chain)
		}
	}
}

func TestTelegramWebEmptyChannelErrors(t *testing.T) {
	if _, err := TelegramWebChannel(context.Background(), http.DefaultClient, "  "); err == nil {
		t.Fatal("空频道名应报错")
	}
}
