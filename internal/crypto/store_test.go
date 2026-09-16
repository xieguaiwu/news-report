package crypto

import (
	"os"
	"path/filepath"
	"testing"
)

func TestAppendJSONLAppendsNotOverwrites(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "out.jsonl")
	tone := 1
	r1 := Row{SchemaVersion: SchemaVersion, Chain: ChainBSC, Kind: KindMarket, TokenAddr: "0xa", EventAt: 1, CollectedAt: 1, Tone: &tone}
	r2 := Row{SchemaVersion: SchemaVersion, Chain: ChainBSC, Kind: KindMarket, TokenAddr: "0xb", EventAt: 1, CollectedAt: 1}
	if err := AppendJSONL(p, []Row{r1}); err != nil {
		t.Fatalf("append 1: %v", err)
	}
	if err := AppendJSONL(p, []Row{r2}); err != nil {
		t.Fatalf("append 2: %v", err)
	}
	b, _ := os.ReadFile(p)
	if n := countLines(b); n != 2 {
		t.Fatalf("want 2 lines got %d: %s", n, b)
	}
	if b[len(b)-1] != '\n' {
		t.Fatal("file must end with newline")
	}
}

func TestDedupRows(t *testing.T) {
	r := Row{SchemaVersion: SchemaVersion, Chain: ChainBSC, Kind: KindMarket, TokenAddr: "0xa", Source: "dexscreener", EventAt: 1, CollectedAt: 10}
	sameEventLaterRound := r
	sameEventLaterRound.CollectedAt = 999 // 同一事件的后续采集轮次：应被去重
	different := Row{SchemaVersion: SchemaVersion, Chain: ChainBSC, Kind: KindMarket, TokenAddr: "0xb", Source: "dexscreener", EventAt: 1, CollectedAt: 10}
	got := DedupRows([]Row{r, sameEventLaterRound, different})
	if len(got) != 2 {
		t.Fatalf("want 2 got %d: %+v", len(got), got)
	}
}

func countLines(b []byte) int {
	n := 0
	for _, c := range b {
		if c == '\n' {
			n++
		}
	}
	return n
}
