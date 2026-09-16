package crypto

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

const goplusFixture = `{"code":1,"message":"OK","result":{"0xabc":{
 "buy_tax":"0.05","sell_tax":"0.10","creator_address":"0xdead","creator_percent":"0.12",
 "is_open_source":"1","cannot_sell_all":"0","holder_count":"1533"}}}`

func TestGoPlusParsesFixture(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("contract_addresses") != "0xabc" {
			t.Errorf("bad query: %s", r.URL.RawQuery)
		}
		_, _ = w.Write([]byte(goplusFixture))
	}))
	defer ts.Close()

	it, err := goPlusAt(context.Background(), ts.Client(), ts.URL, GoPlusChainBSC, "0xabc")
	if err != nil {
		t.Fatalf("goplus: %v", err)
	}
	if it.Metrics["sell_tax"] != 0.10 || it.Metrics["creator_percent"] != 0.12 {
		t.Fatalf("metrics: %+v", it.Metrics)
	}
	if it.Metrics["holder_count"] != 1533 {
		t.Fatalf("holder_count: %v", it.Metrics["holder_count"])
	}
	if it.Kind != KindSafety || it.Chain != ChainBSC {
		t.Fatalf("bad item: %+v", it)
	}
}

func TestGoPlusRejectsUnsupportedChain(t *testing.T) {
	_, err := GoPlusTokenSecurity(context.Background(), http.DefaultClient, 999, "0xabc")
	if err == nil {
		t.Fatal("expected error for unsupported chain id")
	}
}
