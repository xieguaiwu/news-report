package report

import (
	"testing"
	"time"
)

func TestTimestamp(t *testing.T) {
	// Zero time should return "--"
	it := Item{Published: time.Time{}}
	if ts := it.Timestamp(); ts != "--" {
		t.Errorf("零值时间应返回 \"--\"，实际 %q", ts)
	}

	// Known time should return formatted
	tm := time.Date(2026, 8, 3, 14, 30, 0, 0, time.UTC)
	it = Item{Published: tm}
	if ts := it.Timestamp(); ts != "08-03 14:30" {
		t.Errorf("已知时间应返回 \"08-03 14:30\"，实际 %q", ts)
	}
}
