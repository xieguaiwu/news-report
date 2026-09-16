//go:build net

package crypto

import (
	"context"
	"net/http"
	"os"
	"testing"
	"time"
)

func TestNetDexScreenerSearch(t *testing.T) {
	c := &http.Client{Timeout: 20 * time.Second}
	items, err := SearchPairs(context.Background(), c, "NIULAI")
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if len(items) == 0 {
		t.Fatal("expected at least one NIULAI pair")
	}
	t.Logf("found %d pairs, first=%s/%s", len(items), items[0].Chain, items[0].TokenAddr)
	_ = os.Stdout.Sync()
}
