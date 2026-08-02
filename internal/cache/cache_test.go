package cache

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"news-report/internal/sources"
)

func TestCacheSetGet(t *testing.T) {
	dir := t.TempDir()
	c := New(dir, 50, 0)
	url := "https://example.com/feed"
	if err := c.Set(url, []byte("test body"), "etag123", "Mon, 01 Aug 2026"); err != nil {
		t.Fatalf("Set: %v", err)
	}
	e := c.Get(url, sources.TierLegacy)
	if e == nil {
		t.Fatal("Get 返回 nil")
	}
	if string(e.Body) != "test body" {
		t.Errorf("body: %q", e.Body)
	}
	if e.ETag != "etag123" || e.LastMod != "Mon, 01 Aug 2026" {
		t.Errorf("headers: %+v", e)
	}
}

func TestCacheExpiry(t *testing.T) {
	dir := t.TempDir()
	c := New(dir, 50, 10*time.Millisecond) // 统一 TTL 10ms
	_ = c.Set("https://x.com/f", []byte("x"), "", "")
	time.Sleep(20 * time.Millisecond)
	if e := c.Get("https://x.com/f", sources.TierLegacy); e != nil {
		t.Error("过期缓存应返回 nil")
	}
}

func TestCacheTierTTL(t *testing.T) {
	dir := t.TempDir()
	c := New(dir, 50, 0) // 无统一 TTL，用 tier
	// Wire TTL=6min — 不设更小的方便测试，直接测 tier 映射
	_ = c.Set("https://w.com/f", []byte("w"), "", "")
	e := c.Get("https://w.com/f", sources.TierWire)
	if e == nil {
		t.Fatal("wire 缓存 6min 内应有效")
	}
	// specialist 同理
	_ = c.Set("https://s.com/f", []byte("s"), "", "")
	e2 := c.Get("https://s.com/f", sources.TierSpecialist)
	if e2 == nil {
		t.Fatal("specialist 缓存 30min 内应有效")
	}
}

func TestCacheClear(t *testing.T) {
	dir := t.TempDir()
	c := New(dir, 50, 0)
	_ = c.Set("https://a.com/1", []byte("1"), "", "")
	_ = c.Set("https://a.com/2", []byte("2"), "", "")
	if n, _ := os.ReadDir(dir); len(n) < 2 {
		t.Fatal("写缓存失败")
	}
	if err := c.Clear(); err != nil {
		t.Fatal(err)
	}
	if n, _ := os.ReadDir(dir); len(n) != 0 {
		t.Error("Clear 后应为空")
	}
}

func TestCachePurge(t *testing.T) {
	dir := t.TempDir()
	c := New(dir, 1, 0) // 1 字节上限 → 立即淘汰
	_ = c.Set("https://a.com/1", []byte("aaa"), "", "")
	_ = c.Set("https://a.com/2", []byte("bbb"), "", "")
	n, err := c.Purge()
	if err != nil {
		t.Fatal(err)
	}
	if n == 0 {
		t.Error("超限应淘汰")
	}
}

func TestCachePath(t *testing.T) {
	c := New("/tmp/test", 50, 0)
	p := c.path("https://example.com/feed")
	if filepath.Ext(p) != ".json" {
		t.Errorf("缓存文件名应以 .json 结尾: %q", p)
	}
	if filepath.Dir(p) != "/tmp/test" {
		t.Errorf("缓存应在指定目录: %q", p)
	}
}
