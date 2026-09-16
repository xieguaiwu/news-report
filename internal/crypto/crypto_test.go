package crypto

import (
	"encoding/json"
	"testing"
)

func TestRowJSONRoundTrip(t *testing.T) {
	tone := 2
	row := Row{
		SchemaVersion: SchemaVersion,
		EventAt:       1786808311,
		CollectedAt:   1786808311,
		Chain:         "bsc",
		TokenAddr:     "0x3604B5c377124d2180C4fB791953fc8431a90111",
		Symbol:        "niulai",
		Source:        "dexscreener",
		Kind:          "market",
		Title:         "pair created",
		URL:           "https://dexscreener.com/bsc/0xc91d0280d92e2c89f2c6a984ddafa6cca39c7643",
		Tone:          &tone,
		ScoredAt:      "2026-09-16T14:00:00Z",
		Model:         "qwen3.8-flash",
		PromptVersion: "crypto-attention-v1",
	}
	b, err := json.Marshal(row)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var back Row
	if err := json.Unmarshal(b, &back); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if back.TokenAddr != row.TokenAddr || back.SchemaVersion != SchemaVersion {
		t.Fatalf("round trip mismatch: %+v", back)
	}
	if back.Tone == nil || *back.Tone != 2 {
		t.Fatalf("tone lost in round trip: %+v", back.Tone)
	}
}

func TestValidateRejectsBadChain(t *testing.T) {
	row := Row{SchemaVersion: SchemaVersion, Chain: "ethereum", TokenAddr: "0x1"}
	if err := row.Validate(); err == nil {
		t.Fatal("expected error for unsupported chain")
	}
}

func TestValidateAcceptsBSCAndSolana(t *testing.T) {
	for _, c := range []string{"bsc", "solana"} {
		row := Row{SchemaVersion: SchemaVersion, Chain: c, Kind: KindMarket, TokenAddr: "x", EventAt: 1, CollectedAt: 1}
		if err := row.Validate(); err != nil {
			t.Fatalf("chain %s: %v", c, err)
		}
	}
}

func TestValidateRejectsBadKind(t *testing.T) {
	row := Row{SchemaVersion: SchemaVersion, Chain: ChainBSC, Kind: "nope", TokenAddr: "x", EventAt: 1, CollectedAt: 1}
	if err := row.Validate(); err == nil {
		t.Fatal("expected error for unsupported kind")
	}
}

func TestValidateAttentionRowCarriesNoChain(t *testing.T) {
	ok := Row{SchemaVersion: SchemaVersion, Chain: ChainNone, Kind: KindAttention, Source: "weibo", EventAt: 1, CollectedAt: 1}
	if err := ok.Validate(); err != nil {
		t.Fatalf("attention row without chain must be valid: %v", err)
	}
	bad := Row{SchemaVersion: SchemaVersion, Chain: ChainBSC, Kind: KindAttention, EventAt: 1, CollectedAt: 1}
	if err := bad.Validate(); err == nil {
		t.Fatal("attention row with a chain must be rejected")
	}
	missing := Row{SchemaVersion: SchemaVersion, Kind: KindMarket, TokenAddr: "x"}
	if err := missing.Validate(); err == nil {
		t.Fatal("missing timestamps must be rejected")
	}
}
