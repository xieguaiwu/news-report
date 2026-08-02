// Package store 维护已报告条目的持久化记录（JSON 文件），跨运行去重。
package store

import (
	"encoding/json"
	"hash/fnv"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// Store 是线程安全的已见记录。
type Store struct {
	mu    sync.Mutex
	path  string
	items map[string]int64 // url hash -> unix 秒
}

// New 创建并加载记录（文件不存在时为空）。
func New(path string) (*Store, error) {
	s := &Store{path: path, items: map[string]int64{}}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return s, nil
		}
		return nil, err
	}
	if len(data) == 0 {
		return s, nil
	}
	if err := json.Unmarshal(data, &s.items); err != nil {
		// 记录损坏时不致命：重建
		s.items = map[string]int64{}
	}
	return s, nil
}

// Hash 计算 URL 的稳定哈希。
func Hash(url string) string {
	h := fnv.New64a()
	h.Write([]byte(url))
	return u64toa(h.Sum64())
}

// Has 判断 URL 是否已见。
func (s *Store) Has(url string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, ok := s.items[Hash(url)]
	return ok
}

// Add 记录 URL 已见。
func (s *Store) Add(url string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.items[Hash(url)] = time.Now().Unix()
}

// Prune 删除超过 days 天的记录，返回删除数。
func (s *Store) Prune(days int) int {
	if days <= 0 {
		return 0
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	cutoff := time.Now().Add(-time.Duration(days) * 24 * time.Hour).Unix()
	removed := 0
	for k, ts := range s.items {
		if ts < cutoff {
			delete(s.items, k)
			removed++
		}
	}
	return removed
}

// Save 原子写盘。
func (s *Store) Save() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	data, err := json.Marshal(s.items)
	if err != nil {
		return err
	}
	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, s.path)
}

func u64toa(n uint64) string {
	const digits = "0123456789abcdef"
	var buf [16]byte
	for i := 15; i >= 0; i-- {
		buf[i] = digits[n&0xf]
		n >>= 4
	}
	return string(buf[:])
}
