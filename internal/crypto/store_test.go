package crypto

import (
	"os"
	"path/filepath"
	"strings"
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

func TestReadJSONLRoundTripWithWriteAtomic(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "out.jsonl")
	tone := 1
	in := []Row{
		{SchemaVersion: SchemaVersion, Chain: ChainBSC, Kind: KindMarket, TokenAddr: "0xa", EventAt: 1, CollectedAt: 2, Tone: &tone},
		{SchemaVersion: SchemaVersion, Chain: ChainNone, Kind: KindAttention, Source: "weibo", EventAt: 3, CollectedAt: 4},
	}
	if err := WriteJSONLAtomic(p, in); err != nil {
		t.Fatalf("write: %v", err)
	}
	got, err := ReadJSONL(p)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("want 2 got %d", len(got))
	}
	if got[0].TokenAddr != "0xa" || got[0].Tone == nil || *got[0].Tone != 1 {
		t.Fatalf("row0 mismatch: %+v", got[0])
	}
	if got[1].Kind != KindAttention || got[1].Chain != ChainNone {
		t.Fatalf("row1 mismatch: %+v", got[1])
	}
}

func TestWriteJSONLAtomicReplacesNotAppends(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "out.jsonl")
	r := Row{SchemaVersion: SchemaVersion, Chain: ChainBSC, Kind: KindMarket, EventAt: 1, CollectedAt: 1}
	if err := WriteJSONLAtomic(p, []Row{r}); err != nil {
		t.Fatal(err)
	}
	if err := WriteJSONLAtomic(p, []Row{r}); err != nil {
		t.Fatal(err)
	}
	got, _ := ReadJSONL(p)
	if len(got) != 1 {
		t.Fatalf("atomic write must replace, got %d rows", len(got))
	}
	if _, err := os.Stat(p + ".tmp"); !os.IsNotExist(err) {
		t.Fatal("tmp file must be cleaned up by rename")
	}
}

func TestReadJSONLErrorsWithLineNumber(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "bad.jsonl")
	_ = os.WriteFile(p, []byte("{\"a\":1}\nnot-json\n"), 0o644)
	_, err := ReadJSONL(p)
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "line 2") {
		t.Fatalf("error must carry the line number: %v", err)
	}
}

func TestReadJSONLMissingFileErrors(t *testing.T) {
	if _, err := ReadJSONL(filepath.Join(t.TempDir(), "nope.jsonl")); err == nil {
		t.Fatal("expected error for missing file")
	}
}

func TestRowToItemCarriesEventTime(t *testing.T) {
	r := Row{EventAt: 1786808311, Chain: ChainBSC, TokenAddr: "0xa", Title: "t", Metrics: map[string]float64{"price_usd": 1}}
	it := RowToItem(r)
	if it.ObservedAt != 1786808311 || it.Chain != ChainBSC || it.Title != "t" {
		t.Fatalf("mismatch: %+v", it)
	}
	if it.Metrics["price_usd"] != 1 {
		t.Fatal("metrics not carried")
	}
}

func TestTgOffsetRoundTrip(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "state", "tg_offset.json")
	got, err := LoadTgOffset(p)
	if err != nil || got != 0 {
		t.Fatalf("missing file must yield 0,nil; got %d,%v", got, err)
	}
	if err := SaveTgOffset(p, 4242); err != nil {
		t.Fatalf("save: %v", err)
	}
	got, err = LoadTgOffset(p)
	if err != nil || got != 4242 {
		t.Fatalf("want 4242,nil got %d,%v", got, err)
	}
}
