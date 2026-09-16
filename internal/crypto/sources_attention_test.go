package crypto

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestFilterCryptoKeywords(t *testing.T) {
	in := []AttentionItem{
		{Symbol: "加密", Title: "比特币突破新高"},
		{Symbol: "娱乐", Title: "某明星恋情曝光"},
		{Symbol: "meme", Title: "狗狗币又涨了"},
		{Symbol: "政治", Title: "阿根廷总统回应"},
	}
	got := FilterCryptoKeywords(in)
	if len(got) != 2 {
		t.Fatalf("want 2 got %d: %+v", len(got), got)
	}
}

func TestWeiboHotSearchParsesFixture(t *testing.T) {
	fixture := `{"ok":1,"data":{"realtime":[
	 {"word":"比特币","num":100,"raw_hot":2000000},
	 {"word":"天气","num":50,"raw_hot":100000}]}}`
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("User-Agent") == "" {
			t.Error("missing UA")
		}
		_, _ = w.Write([]byte(fixture))
	}))
	defer ts.Close()

	items, err := weiboHotSearchAt(context.Background(), ts.Client(), ts.URL)
	if err != nil {
		t.Fatalf("weibo: %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("want 2 got %d", len(items))
	}
	if items[0].Metrics["raw_hot"] != 2000000 {
		t.Fatalf("raw_hot: %v", items[0].Metrics["raw_hot"])
	}
	if items[0].Kind != KindAttention || items[0].Source != "weibo" {
		t.Fatalf("bad item: %+v", items[0])
	}
}

func TestTelegramNeverLeaksTokenIntoItem(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"ok":true,"result":[{"update_id":7,"channel_post":{"date":1786808311,"text":"$NIULAI to the moon","chat":{"title":"meme calls"}}}]}`))
	}))
	defer ts.Close()

	items, _, err := telegramUpdatesAt(context.Background(), ts.Client(), ts.URL, 0)
	if err != nil {
		t.Fatalf("tg: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("want 1 got %d", len(items))
	}
	if items[0].URL != "" {
		t.Fatalf("url must be empty (token leak vector): %q", items[0].URL)
	}
	if items[0].ObservedAt != 1786808311 {
		t.Fatalf("observed_at: %d", items[0].ObservedAt)
	}
}

func TestWeiboRequestCarriesReferer(t *testing.T) {
	var gotReferer, gotUA string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotReferer = r.Header.Get("Referer")
		gotUA = r.Header.Get("User-Agent")
		_, _ = w.Write([]byte(`{"ok":1,"data":{"realtime":[]}}`))
	}))
	defer ts.Close()
	if _, err := weiboHotSearchAt(context.Background(), ts.Client(), ts.URL); err != nil {
		t.Fatalf("weibo: %v", err)
	}
	if gotReferer != "https://weibo.com/" {
		t.Fatalf("微博接口缺 Referer 会 403，实际=%q", gotReferer)
	}
	if !strings.Contains(gotUA, "Mozilla") {
		t.Fatalf("微博需要浏览器型 UA，实际=%q", gotUA)
	}
}
