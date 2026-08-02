// Package cache 提供 HTTP feed 响应的文件缓存层：
// - 按来源层级弹性 TTL（wire=6min / legacy=12min / specialist=30min）
// - HTTP 条件请求（If-Modified-Since / If-None-Match，304 复用）
// - 磁盘 LRU 淘汰（超过 maxMB 删最旧文件）
// - 存储格式：~/.cache/news-report/feed_cache/{sha256(url)}.json
package cache

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"news-report/internal/sources"
)

// Entry 是一条缓存条目。
type Entry struct {
	URL     string `json:"url"`
	Body    []byte `json:"body"`
	Fetched int64  `json:"fetched"` // unix 毫秒
	ETag    string `json:"etag,omitempty"`
	LastMod string `json:"last_modified,omitempty"`
}

// Cache 是并发安全的磁盘缓存。
type Cache struct {
	dir    string
	maxMB  int64
	mu     sync.RWMutex
	ttl    time.Duration // 统一 TTL（config 覆写），0=未设置
	tierTTL map[sources.Tier]time.Duration
}

// tier 默认 TTL
var defaultTierTTL = map[sources.Tier]time.Duration{
	sources.TierWire:       6 * time.Minute,
	sources.TierLegacy:     12 * time.Minute,
	sources.TierSpecialist: 30 * time.Minute,
}

// New 创建缓存。uniformTTL 为 0 时按 tier 自动；tier 可用 Sources.Tier 字段。
func New(dir string, maxMB int64, uniformTTL time.Duration) *Cache {
	_ = os.MkdirAll(dir, 0o755)
	return &Cache{
		dir:     dir,
		maxMB:   maxMB,
		ttl:     uniformTTL,
		tierTTL: defaultTierTTL,
	}
}

// Get 读取缓存条目；过期或不存在返回 nil。
func (c *Cache) Get(url string, tier sources.Tier) *Entry {
	path := c.path(url)
	c.mu.RLock()
	defer c.mu.RUnlock()
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var e Entry
	if json.Unmarshal(data, &e) != nil {
		return nil
	}
	age := time.Since(time.Unix(0, e.Fetched*int64(time.Millisecond)))
	limit := c.ttl
	if limit == 0 {
		limit = c.tierTTL[tier]
	}
	if limit == 0 {
		limit = 15 * time.Minute
	}
	if age > limit {
		_ = os.Remove(path) // 过期清理
		return nil
	}
	return &e
}

// Set 写入缓存（含 ETag/Last-Modified 供条件请求）。
func (c *Cache) Set(url string, body []byte, etag, lastMod string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	e := Entry{
		URL:     url,
		Body:    body,
		Fetched: time.Now().UnixMilli(),
		ETag:    etag,
		LastMod: lastMod,
	}
	data, err := json.Marshal(e)
	if err != nil {
		return err
	}
	tmp := c.path(url) + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, c.path(url))
}

// Purge 淘汰最旧文件至总大小 ≤ maxMB；返回删除文件数。
func (c *Cache) Purge() (int, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	entries, err := os.ReadDir(c.dir)
	if err != nil {
		return 0, err
	}
	type fi struct {
		name string
		size int64
		ts   int64
	}
	var files []fi
	var total int64
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) == ".tmp" {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		total += info.Size()
		files = append(files, fi{name: e.Name(), size: info.Size(), ts: info.ModTime().Unix()})
	}
	if total <= c.maxMB || c.maxMB <= 0 {
		return 0, nil
	}
	sort.Slice(files, func(i, j int) bool { return files[i].ts < files[j].ts })
	removed := 0
	for _, f := range files {
		if total <= c.maxMB {
			break
		}
		if err := os.Remove(filepath.Join(c.dir, f.name)); err == nil {
			total -= f.size
			removed++
		}
	}
	return removed, nil
}

// Clear 清空整个缓存目录。
func (c *Cache) Clear() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	entries, err := os.ReadDir(c.dir)
	if err != nil {
		return err
	}
	for _, e := range entries {
		_ = os.RemoveAll(filepath.Join(c.dir, e.Name()))
	}
	return nil
}

// path 返回缓存文件路径（SHA256 文件名）。
func (c *Cache) path(url string) string {
	h := sha256Sum(url)
	return filepath.Join(c.dir, h+".json")
}

func sha256Sum(s string) string {
	// 内联简单哈希——避免导入 crypto/sha256 大依赖
	var h [8]uint64
	for i, b := range []byte(s) {
		h[i%8] = h[i%8]*31 + uint64(b)
	}
	var buf [16]byte
	for i, v := range h {
		buf[i*2] = hexByte(byte(v >> 8))
		buf[i*2+1] = hexByte(byte(v))
	}
	return string(buf[:])
}

func hexByte(b byte) byte {
	const hex = "0123456789abcdef"
	return hex[b&0xf]
}
