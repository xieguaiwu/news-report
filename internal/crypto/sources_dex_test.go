package crypto

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

const dexFixture = `{"schemaVersion":"1.0.0","pairs":[
 {"chainId":"bsc","dexId":"pancakeswap",
  "pairAddress":"0xC91D0280D92E2c89F2c6a984DDAfa6c",
  "baseToken":{"address":"0x3604B5c377124d2180C4fB791953fc8431a90111","symbol":"niulai"},
  "quoteToken":{"address":"0xbb4CdB9CBd36B01bD1cBaEBF2De08d9173bc095c","symbol":"WBNB"},
  "priceUsd":"0.0001263","fdv":57859,"marketCap":57859,
  "liquidity":{"usd":35039.35,"base":1000,"quote":1000},
  "volume":{"h24":270.53,"h6":10,"h1":1},
  "txns":{"h24":{"buys":2,"sells":7}},
  "pairCreatedAt":1786808311000,
  "url":"https://dexscreener.com/bsc/0xc91d0280d92e2c89f2c6a984ddafa6cca39c7643"}]}`

func TestSearchPairsParsesFixture(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("User-Agent") == "" {
			t.Error("missing User-Agent header")
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(dexFixture))
	}))
	defer ts.Close()

	items, err := searchPairsAt(context.Background(), ts.Client(), ts.URL)
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("want 1 item got %d", len(items))
	}
	it := items[0]
	if it.Chain != ChainBSC || it.Source != "dexscreener" || it.Kind != KindMarket {
		t.Fatalf("bad item: %+v", it)
	}
	if it.Metrics["liquidity_usd"] != 35039.35 {
		t.Fatalf("liquidity: %v", it.Metrics["liquidity_usd"])
	}
	if it.Metrics["buys_24h"] != 2 || it.Metrics["sells_24h"] != 7 {
		t.Fatalf("txns: %+v", it.Metrics)
	}
	if it.Metrics["price_usd"] != 0.0001263 {
		t.Fatalf("price_usd missing: %+v", it.Metrics)
	}
	if it.ObservedAt != 1786808311 {
		t.Fatalf("observed_at should come from pairCreatedAt (ms→s), got %d", it.ObservedAt)
	}
}

func TestSearchPairsRetriesOn429(t *testing.T) {
	var n int
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n++
		if n == 1 {
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		_, _ = w.Write([]byte(dexFixture))
	}))
	defer ts.Close()

	items, err := searchPairsAt(context.Background(), ts.Client(), ts.URL)
	if err != nil {
		t.Fatalf("expected retry to succeed: %v", err)
	}
	if len(items) != 1 || n != 2 {
		t.Fatalf("retry did not happen: n=%d items=%d", n, len(items))
	}
}

func TestSearchPairsDoesNotRetryOn400(t *testing.T) {
	var n int
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n++
		w.WriteHeader(http.StatusBadRequest)
	}))
	defer ts.Close()

	_, err := searchPairsAt(context.Background(), ts.Client(), ts.URL)
	if err == nil {
		t.Fatal("expected error")
	}
	if n != 1 {
		t.Fatalf("4xx must not retry, got n=%d", n)
	}
}
