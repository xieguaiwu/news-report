package store

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestStoreAddHas(t *testing.T) {
	s := &Store{path: filepath.Join(t.TempDir(), "seen.json"), items: map[string]int64{}}
	if s.Has("https://example.com/a") {
		t.Error("新 store 不应包含任何 URL")
	}
	s.Add("https://example.com/a")
	s.Add("https://example.com/b")
	if !s.Has("https://example.com/a") {
		t.Error("Add 后应能 Has 到")
	}
	if s.Has("https://example.com/c") {
		t.Error("未添加的 URL 不应命中")
	}
}

func TestStorePersist(t *testing.T) {
	path := filepath.Join(t.TempDir(), "seen.json")
	s1 := &Store{path: path, items: map[string]int64{}}
	s1.Add("https://example.com/x")
	if err := s1.Save(); err != nil {
		t.Fatalf("Save: %v", err)
	}
	s2, err := New(path)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if !s2.Has("https://example.com/x") {
		t.Error("重载后应保留记录")
	}
}

func TestStorePrune(t *testing.T) {
	s := &Store{items: map[string]int64{
		"old": time.Now().Add(-30 * 24 * time.Hour).Unix(),
		"new": time.Now().Add(-time.Hour).Unix(),
	}}
	removed := s.Prune(7)
	if removed != 1 {
		t.Errorf("应删除 1 条过期记录，实际 %d", removed)
	}
	if _, ok := s.items["old"]; ok {
		t.Error("过期记录应被删除")
	}
	if _, ok := s.items["new"]; !ok {
		t.Error("新记录不应被删除")
	}
}

func TestHashStable(t *testing.T) {
	if Hash("https://a.com/1") != Hash("https://a.com/1") {
		t.Error("相同 URL 哈希应稳定")
	}
	if Hash("https://a.com/1") == Hash("https://a.com/2") {
		t.Error("不同 URL 哈希应不同")
	}
}

func TestNewCorruptFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "seen.json")
	if err := os.WriteFile(path, []byte("{corrupt"), 0o644); err != nil {
		t.Fatal(err)
	}
	s, err := New(path)
	if err != nil {
		t.Fatalf("损坏文件不应致命: %v", err)
	}
	if len(s.items) != 0 {
		t.Error("损坏文件应重建为空记录")
	}
}
