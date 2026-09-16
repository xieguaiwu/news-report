package crypto

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

const honeypotFixture = `{"isHoneypot":false,"simulationResult":{"buyTax":5.0,"sellTax":10.0},
 "contractCode":{"openSource":true,"proxy":false},
 "summary":{"risk":"low"},"pair":{"pair":"0xc91d"}}`

func TestHoneypotParsesFixture(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("chain") != "bsc" {
			t.Errorf("bad chain: %s", r.URL.RawQuery)
		}
		if r.URL.Query().Get("address") != "0xabc" {
			t.Errorf("bad address: %s", r.URL.RawQuery)
		}
		_, _ = w.Write([]byte(honeypotFixture))
	}))
	defer ts.Close()

	it, err := honeypotCheckAt(context.Background(), ts.Client(), ts.URL, "bsc", "0xabc")
	if err != nil {
		t.Fatalf("honeypot: %v", err)
	}
	if it.Metrics["is_honeypot"] != 0 {
		t.Fatalf("is_honeypot: %v", it.Metrics["is_honeypot"])
	}
	if it.Metrics["buy_tax"] != 0.05 || it.Metrics["sell_tax"] != 0.10 {
		t.Fatalf("tax mapping (percent→fraction): %+v", it.Metrics)
	}
	if it.Metrics["open_source"] != 1 || it.Metrics["risk_level"] != 0 {
		t.Fatalf("contract/risk: %+v", it.Metrics)
	}
	if it.Source != "honeypot" || it.Kind != KindSafety {
		t.Fatalf("bad item: %+v", it)
	}
}

func TestHoneypotUnknownRiskMapsToMinusOne(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"isHoneypot":true,"simulationResult":{"buyTax":0,"sellTax":0},"contractCode":{"openSource":false,"proxy":true},"summary":{"risk":"weird"}}`))
	}))
	defer ts.Close()
	it, err := honeypotCheckAt(context.Background(), ts.Client(), ts.URL, "bsc", "0x1")
	if err != nil {
		t.Fatalf("honeypot: %v", err)
	}
	if it.Metrics["risk_level"] != -1 {
		t.Fatalf("unknown risk must be -1, got %v", it.Metrics["risk_level"])
	}
}
