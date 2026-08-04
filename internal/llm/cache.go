// Package llm — 输出缓存：sha256(URL+lang+instruction) → JSON 文件缓存，
// TTL 7 天，LRU 淘汰至 ≤ maxMB。
// 独立于 feed_cache 目录，存于 ~/.cache/news-report/llm/。
package llm

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"
)

// Cache 是 LLM 输出缓存，并发安全。
type Cache struct {
	dir   string
	ttl   time.Duration // 默认 7 天
	maxMB int64
	mu    sync.RWMutex
}

// CacheEntry 是一条缓存记录。
type CacheEntry struct {
	Key       string `json:"key"` // sha256(url|lang|instruction)
	Response  string `json:"response"`
	CreatedAt int64  `json:"created_at"` // unix 秒
}

// 三种指令类型常量，用于生成缓存 key。
const (
	InstTranslate = "translate"
	InstSummary   = "summary"
	InstBrief     = "brief"
)

// NewCache 创建 LLM 输出缓存。
// dir: 缓存目录（如 ~/.cache/news-report/llm/）
// ttl: 缓存有效期，0 则默认 7 天
// maxMB: 磁盘上限，0 则不限制
func NewCache(dir string, ttl time.Duration, maxMB int64) *Cache {
	if ttl <= 0 {
		ttl = 7 * 24 * time.Hour
	}
	_ = os.MkdirAll(dir, 0o755)
	return &Cache{
		dir:   dir,
		ttl:   ttl,
		maxMB: maxMB,
	}
}

// Get 读取缓存条目；过期或不存在返回 ("", false)。
func (c *Cache) Get(key string) (string, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	data, err := os.ReadFile(c.path(key))
	if err != nil {
		return "", false
	}
	var e CacheEntry
	if json.Unmarshal(data, &e) != nil {
		return "", false
	}
	age := time.Since(time.Unix(e.CreatedAt, 0))
	if age > c.ttl {
		_ = os.Remove(c.path(key)) // 过期清理
		return "", false
	}
	return e.Response, true
}

// Set 写入缓存（如果 response 为空则跳过）。
func (c *Cache) Set(key, response string) error {
	if response == "" {
		return nil
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	e := CacheEntry{
		Key:       key,
		Response:  response,
		CreatedAt: time.Now().Unix(),
	}
	data, err := json.Marshal(e)
	if err != nil {
		return err
	}
	tmp := c.path(key) + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, c.path(key))
}

// Purge LRU 淘汰最旧文件至总大小 ≤ maxMB；返回删除文件数。
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
	limit := c.maxMB * 1024 * 1024 // maxMB 单位为 MB，total 为字节
	if total <= limit || limit <= 0 {
		return 0, nil
	}
	sort.Slice(files, func(i, j int) bool { return files[i].ts < files[j].ts })
	removed := 0
	for _, f := range files {
		if total <= limit {
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

// path 返回缓存文件路径。
func (c *Cache) path(key string) string {
	return filepath.Join(c.dir, key+".json")
}

// CacheKey 生成缓存 key：sha256(url|lang|instruction) 的十六进制表示。
func CacheKey(url, lang, instruction string) string {
	h := sha256.Sum256([]byte(url + "|" + lang + "|" + instruction))
	return fmt.Sprintf("%x", h)
}
