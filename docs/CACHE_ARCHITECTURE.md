# HTTP Feed 响应缓存层架构设计

> **版本**: v1.0 (2026-08-03)  
> **状态**: 设计草案  
> **目标**: 为 65+ feed 源引入弹性 TTL 的 HTTP 响应缓存，使二次运行耗时从 ~40s 降至 <2s（全命中场景）

---

## 1. 概述

### 1.1 当前问题

`report.Run()` 的并发抓取阶段（`collectSource`）对每个启用的 feed URL 发起 HTTP GET，总计 65+ 源、120+ feed URL。每次运行都全量重抓，无任何缓存：
- **首次运行**: ~40s（网络 IO 绑定）
- **二次运行**: 仍是 ~40s（没有复用）

### 1.2 设计目标

1. **分层弹性 TTL**：wire 5–10min / legacy 10–15min / specialist 30–60min / zh 10min
2. **HTTP 标准遵从**：解析 Cache-Control: max-age；发送 If-Modified-Since / If-None-Match；304 返回缓存
3. **空间管理**：单条 ≤500KB，总目录 ≤50MB（可配置），超出时 LRU 淘汰
4. **非侵入集成**：包装 `*fetch.Fetcher`，不改动 `tryOnce`（私有方法）
5. **并发安全**：多 goroutine 并发访问无竞态、无文件损坏
6. **降级友好**：磁盘满、解析失败等异常时，自动回退到实时抓取

---

## 2. 包设计

### 2.1 新包 `internal/cache`

```
internal/cache/
  cache.go       // FeedCache 结构体、New()、Get()、Clear()、inflight 去重
  entry.go       // cacheEntry 结构体、序列化/反序列化、TTL 计算
  lru.go         // 空间统计、LRU 淘汰、目录扫描
  cache_test.go  // 单元测试
```

### 2.2 核心类型

```go
// FeedCache 是可并发使用的 HTTP feed 缓存层，包装 *fetch.Fetcher。
type FeedCache struct {
    fetcher     *fetch.Fetcher
    dir         string        // ~/.cache/news-report/feed_cache/
    maxSize     int64         // 总目录上限（字节），默认 50MB
    maxEntry    int64         // 单条上限（字节），默认 500KB
    ttlOverride time.Duration // 0 = 按 tier 计算；>0 = 统一覆写所有 TTL
    noCache     bool          // true = 跳过缓存（--no-cache）

    mu       sync.Mutex
    inflight map[string]chan *fetchResult // inflight 去重
}

type fetchResult struct {
    body []byte
    err  error
}
```

### 2.3 构造函数

```go
func New(fetcher *fetch.Fetcher, dir string, opts CacheOptions) *FeedCache

type CacheOptions struct {
    MaxSizeMB    int           // 0 = 默认 50
    MaxEntryKB   int           // 0 = 默认 500
    TTLMinutes   int           // 0 = 按 tier（见 §3.2）；>0 = 统一覆写
    NoCache      bool          // true = --no-cache（完全跳过缓存层）
}
```

示例调用（`main.go`）：

```go
cache := cache.New(fetcher, filepath.Join(cfg.CachePath(), "feed_cache"), cache.CacheOptions{
    MaxSizeMB:  cfg.CacheMaxMB,
    MaxEntryKB: cfg.CacheMaxEntryKB,
    TTLMinutes: cfg.CacheTTLMinutes,
    NoCache:    *noCacheFlag,
})
```

---

## 3. 缓存条目格式与 TTL 策略

### 3.1 磁盘存储格式

**路径**: `~/.cache/news-report/feed_cache/{source_id}_{feed_index}.json`

示例：
- `reuters-world_0.json`
- `bbc-world_0.json`
- `cna-politics_0.json`

`feed_index` 是源在 `source.Feeds` 中的 0-based 索引，与 `collectSource` 的遍历顺序一致。

> **设计权衡**: 文件名基于 `source_id + feed_index` 而非 URL hash。  
> **优点**: 人类可读、需求明确指定此格式。  
> **风险**: 若来源的 feed 列表在版本间变更（增删/重排），旧文件名变为孤儿。孤儿文件会随 LRU 淘汰自然清理，不造成功能性错误——仅短暂浪费少量磁盘空间。

**内容格式** (JSON):

```json
{
  "url": "https://feeds.bbci.co.uk/news/world/rss.xml",
  "body_b64": "PD94bWwgdmVyc2lvbj0iMS4w...",
  "status_code": 200,
  "etag": "\"abc123def456\"",
  "last_modified": "Mon, 01 Jan 2024 12:00:00 GMT",
  "cache_control": "max-age=300, public",
  "max_age_sec": 300,
  "content_type": "application/rss+xml; charset=UTF-8",
  "fetched_at": "2024-01-01T12:00:00Z",
  "accessed_at": "2024-01-01T12:05:00Z"
}
```

**字段说明**:

| 字段 | 类型 | 描述 |
|------|------|------|
| `url` | string | 原始请求 URL（调试/审计用） |
| `body_b64` | string | Base64 编码的响应体（~33% 膨胀，对 5–200KB feed 可接受） |
| `status_code` | int | HTTP 状态码（正常 200） |
| `etag` | string | 服务端 ETag 值（空 = 无） |
| `last_modified` | string | 服务端 Last-Modified（空 = 无） |
| `cache_control` | string | Cache-Control 原始头部 |
| `max_age_sec` | int | 从 Cache-Control 解析的 max-age 秒数（0 = 未指定） |
| `content_type` | string | Content-Type 头部 |
| `fetched_at` | RFC3339 | 首次抓取时间（UTC），写入后不变 |
| `accessed_at` | RFC3339 | 最后访问时间（UTC），每次命中时刷新，作为 LRU 排序键 |

**Base64 编码说明**: 选择 Base64 而非原始二进制是为了保持纯 JSON 格式（便于调试、无分隔符解析）。33% 膨胀在 50MB 总量下约浪费 ~12MB——且仅当接近容量上限时才会触发淘汰，日常使用影响小。

### 3.2 TTL 分层策略

```
┌───────────┬──────────────────┬────────────────────────────────┐
│ Tier      │ 默认 TTL         │ 理由                           │
├───────────┼──────────────────┼────────────────────────────────┤
│ wire      │ 7 min (420s)     │ 通讯社更新频率最高（几分钟级）  │
│ legacy    │ 12 min (720s)    │ 大报 15–30min 更新间隔         │
│ specialist│ 45 min (2700s)   │ 智库/央行每日或每周发布        │
│ zh        │ 10 min (600s)    │ 台湾媒体（中央社/自由时报）     │
└───────────┴──────────────────┴────────────────────────────────┘
```

**TTL 计算规则** (优先级从高到低):

1. 若 `Config.CacheTTLMinutes > 0` → 统一使用该值（覆盖所有层）
2. 若 `source.Lang == "zh"` → 使用 `zhTTL = 10 min`
3. 否则按 `source.Tier` 查表：`wireTTL / legacyTTL / specialistTTL`
4. 若服务端返回 `Cache-Control: max-age=N` → `effectiveTTL = min(tierTTL, N)`（尊重服务端更短的过期时间）
5. 若服务端没有 max-age → 仅使用 tier TTL

**为什么 zh 独立于 tier**: 当前所有 zh 源均为 `TierLegacy`，但需求指定 zh 单独 10min TTL。若未来新增 `TierWire` 的 zh 源，当前规则仍会统一给 zh 10min——这是因为中文读者对时效敏感度可能更高，且台湾媒体 RSS 更新频率通常不固定。

### 3.3 HTTP 条件请求集成

```
Get(url) 流程:

1. 读取磁盘缓存 → cacheEntry
   ├─ 文件不存在 → 转步骤 5（冷抓取）
   ├─ JSON 解析失败 → 删除损坏文件 → 转步骤 5
   └─ 成功

2. 计算 TTL:
   now - cacheEntry.fetched_at > effectiveTTL？
   ├─ 否（未过期）→ 刷新 accessed_at → 返回缓存 body
   └─ 是（已过期）→ 转步骤 3

3. 检查是否有条件请求凭证:
   ├─ cacheEntry.etag != "" → 条件请求（If-None-Match）
   ├─ cacheEntry.last_modified != "" → 条件请求（If-Modified-Since）
   └─ 两者皆无 → 转步骤 5（无条件重抓）

4. 发送条件请求:
   req.Header.Set("If-None-Match", cacheEntry.etag)
   req.Header.Set("If-Modified-Since", cacheEntry.last_modified)
   resp, err := fetcher.Do(req)
   ├─ err != nil → 返回缓存 body（兜底：网络故障时优先服务过期缓存）
   ├─ resp.StatusCode == 304 → 刷新 fetched_at/accessed_at → 返回缓存 body
   └─ resp.StatusCode == 200 → 读取新 body → 转步骤 6（更新缓存）

5. 冷抓取（miss 或过期且无条件凭证）:
   resp, err := fetcher.Do(req)（无缓存相关 header）
   ├─ err != nil → 返回 nil, err
   └─ 200 → 读取 body

6. 写入缓存:
   ├─ body 为空 → 跳过写入
   ├─ len(body) > maxEntry → 跳过写入（日志警告）
   │   注意：仍返回 body 给调用方——只是不缓存超大响应
   └─ 正常 → 序列化 → 原子写（tmp + rename）
        → 检查总目录大小 → 必要时 LRU 淘汰（见 §4）

7. 返回 body
```

**条件请求的 Accept 头设置**:  
在步骤 4/5 中构建 `*http.Request` 时，必须复现 `tryOnce` 的 Accept 头逻辑：
```
Accept: application/rss+xml, application/atom+xml, application/xml, text/xml, text/html;q=0.9, */*;q=0.5
```
且**不手动设置 Accept-Encoding**（原因同现有代码注释：Go transport 自动处理）。

---

## 4. LRU 淘汰策略

### 4.1 触发时机

每次成功写入新缓存条目后，异步检查总目录大小。若 `totalSize > maxSize`，触发淘汰。

**伪代码**:

```go
func (c *FeedCache) evictIfNeeded() {
    c.mu.Lock()
    defer c.mu.Unlock()

    // 冷却期：距上次扫描 < 1s 则跳过（避免并发写入时重复扫描）
    if time.Since(c.lastScan) < time.Second {
        return
    }
    c.lastScan = time.Now()

    entries := c.scanDir()               // 读取目录，获取 (path, size, accessed_at)
    totalSize := sum(entries.size)

    if totalSize <= c.maxSize {
        return
    }

    // 按 accessed_at 升序排序（最旧在前）
    sort.Slice(entries, func(i, j int) bool {
        return entries[i].accessedAt.Before(entries[j].accessedAt)
    })

    targetSize := c.maxSize * 80 / 100   // 淘汰到 80% 容量（滞回，防抖动）
    for _, e := range entries {
        if totalSize <= targetSize {
            break
        }
        os.Remove(e.path)
        totalSize -= e.size
    }
}
```

### 4.2 淘汰信号来源

每个缓存文件的 `accessed_at` 字段作为 LRU 主键：
- 每次 `Get()` 命中（未过期或 304）时，更新文件的 `accessed_at` 为当前 UTC 时间
- 写入新缓存时，`accessed_at` 与 `fetched_at` 均设为当前时间
- 淘汰扫描时，按 `accessed_at` 升序删除（最久未访问的最先被淘汰）

> **为什么不使用文件系统 mtime**: mtime 需要 `os.Chtimes` 系统调用（每次命中增加一次磁盘写入），而 `accessed_at` 已经包含在 JSON 中，命中时仅需重写文件。两者磁盘 IO 相同，但 `accessed_at` 字段更可移植（跨文件系统、备份恢复后仍准确）。

### 4.3 扫描效率

假设 50MB 总容量、平均 30KB/条，约 1700 个缓存文件。一次 `filepath.Walk` + JSON 反序列化约需 10–50ms（取决于磁盘和文件系统缓存），对整体运行时间影响可忽略。

### 4.4 并发写入保护

`evictIfNeeded` 持有 `c.mu` 锁。在扫描和删除期间，新的 `Get()` 可能被短暂阻塞。由于：
- 冷却期（1s）限制扫描频率
- 淘汰操作仅在写入时触发（而非读取时）
- 高并发下多数 Get 命中缓存（无需写入、无需淘汰）

实际阻塞概率极低。

---

## 5. 并发安全设计

### 5.1 文件级安全

**原子写**: 所有缓存文件使用 `tmp + os.Rename` 模式（与 `internal/store/store.go` 一致）：

```go
tmp := path + ".tmp"
os.WriteFile(tmp, data, 0o644)
os.Rename(tmp, path)
```

`os.Rename` 在 Linux 上是原子操作（同一文件系统内）。读者要么看到旧文件，要么看到新文件，不会看到部分写入。

### 5.2 并发去重 (Inflight Dedup)

多个 goroutine 可能同时请求相同的 feed URL（例如两个来源配置了相同的 feed URL，或同一来源的 `collectSource` 逻辑未来改为并发尝试多个 feed）。为避免重复抓取，使用 inflight map：

```go
func (c *FeedCache) Get(ctx context.Context, key string, ...) ([]byte, error) {
    // 1. 检查磁盘缓存（无锁快速路径）
    if entry, ok := c.readEntry(key); ok && !c.isExpired(entry) {
        return entry.body, nil
    }

    // 2. 检查是否已有正在进行的抓取
    c.mu.Lock()
    ch, inflight := c.inflight[key]
    if inflight {
        c.mu.Unlock()
        select {
        case result := <-ch:
            return result.body, result.err
        case <-ctx.Done():
            return nil, ctx.Err()
        }
    }
    ch = make(chan *fetchResult, 1)
    c.inflight[key] = ch
    c.mu.Unlock()

    // 3. 执行抓取（唯一执行者）
    body, err := c.doFetch(ctx, key, ...)
    ch <- &fetchResult{body, err}

    // 4. 清理
    c.mu.Lock()
    delete(c.inflight, key)
    c.mu.Unlock()

    return body, err
}
```

此模式与 `fetch.Fetcher.allowed()` 中的 `busy` channel 一致（现有代码中用于 robots.txt 防重复抓取）。

### 5.3 锁策略总结

| 操作 | 锁 | 说明 |
|------|-----|------|
| 磁盘读取 | 无锁 | 原子读（文件系统保证） |
| inflight 查询/注册 | `c.mu` | 极短临界区 |
| 磁盘写入（tmp+rename） | 无锁 | 原子写；inflight 保证同一 key 只有一个 writer |
| LRU 扫描 + 删除 | `c.mu` | 与 inflight 共享同锁；冷却期限制频率 |

---

## 6. 集成点改造

### 6.1 `collectSource` 改造

**当前代码** (`internal/report/report.go:267-277`):

```go
for _, feedURL := range s.Feeds {
    sctx, cancel := context.WithTimeout(ctx, 30*time.Second)
    var data []byte
    var err error
    if s.UserAgent != "" {
        data, err = f.BytesUA(sctx, feedURL, s.UserAgent)
    } else {
        data, err = f.Bytes(sctx, feedURL)
    }
    cancel()
    // ...
}
```

**改造后** (伪代码):

```go
for i, feedURL := range s.Feeds {
    sctx, cancel := context.WithTimeout(ctx, 30*time.Second)
    cacheKey := fmt.Sprintf("%s_%d", s.ID, i)
    data, err := cache.Get(sctx, feedURL, s.Tier, s.Lang, s.UserAgent, cacheKey)
    cancel()
    // ... data 和 err 后续逻辑不变
}
```

**`collectSource` 函数签名变更**:

```go
// Before:
func collectSource(ctx context.Context, f *fetch.Fetcher, s sources.Source, retries int) ([]feed.Item, SourceStat)

// After:
func collectSource(ctx context.Context, cache *cache.FeedCache, s sources.Source, retries int) ([]feed.Item, SourceStat)
```

### 6.2 `report.Run()` 改造

`report.Run()` 新增 `cache *cache.FeedCache` 参数，传递给 `collectSource`。`FeedCache` 的创建在 `main.go` 中完成（与 `fetch.Fetcher` 同级）。

### 6.3 `Fetcher` 不受影响

`internal/fetch/fetch.go` 无需任何修改。`FeedCache.Get()` 内部通过以下方式调用 Fetcher：
- 冷抓取：构建 `*http.Request` → `fetcher.Do(req)` → 读取 body
- 条件请求：构建带 `If-None-Match` / `If-Modified-Since` 的 `*http.Request` → `fetcher.Do(req)` → 处理 304/200

`tryOnce` 仍然私有且不改动。

---

## 7. 命令行与配置扩展

### 7.1 CLI 新增

**`--no-cache`** flag（`runReport`、`runUI`、`runSources --live`）:

```
news-report --no-cache           # 强制跳过所有缓存
news-report ui --no-cache        # TUI 也支持
news-report sources --live --no-cache  # 实测可用性也支持
```

当 `--no-cache` 设定时，`FeedCache.Get()` 跳过所有缓存读写，直接调用 `fetcher.Do()`。

**`news-report cache clear`** 子命令:

```
news-report cache clear           # 清空 feed 缓存目录
```

在 `main.go` 的 switch 中新增：

```go
case "cache":
    if len(os.Args) > 2 && os.Args[2] == "clear" {
        runCacheClear(os.Args[3:])
    } else {
        fmt.Println("用法: news-report cache clear")
    }
```

实现：`os.RemoveAll(cacheDir)` + `os.MkdirAll(cacheDir, 0o755)`。

### 7.2 配置字段扩展

在 `Config` 结构体中新增以下字段 (`internal/config/config.go`):

```go
type Config struct {
    // ... 现有字段 ...

    // 缓存配置（v0.3.0+）
    CacheTTLMinutes  int `yaml:"cache_ttl_minutes"`   // 0 = 按 tier 分层；>0 = 统一覆写
    CacheMaxMB       int `yaml:"cache_max_mb"`        // 0 = 默认 50
    CacheMaxEntryKB  int `yaml:"cache_max_entry_kb"`  // 0 = 默认 500
}
```

**`Default()` 中新增**:

```go
CacheTTLMinutes: 0,   // 使用 tier 分层
CacheMaxMB:      50,  // 50MB
CacheMaxEntryKB: 500, // 500KB
```

**`Validate()` 中新增**:

```go
if c.CacheTTLMinutes < 0 || c.CacheTTLMinutes > 1440 {
    return errors.New("cache_ttl_minutes 必须在 0-1440 之间（0=分层，最大 24h）")
}
if c.CacheMaxMB < 0 || c.CacheMaxMB > 1024 {
    return errors.New("cache_max_mb 必须在 0-1024 之间（0=默认 50）")
}
if c.CacheMaxEntryKB < 0 || c.CacheMaxEntryKB > 10240 {
    return errors.New("cache_max_entry_kb 必须在 0-10240 之间（0=默认 500）")
}
```

**`init` 子命令**: 生成的默认 `config.yaml` 将包含注释的三个新字段。

### 7.3 `main.go` 变更汇总

| 位置 | 变更 |
|------|------|
| `runReport()` | 创建 `FeedCache`；新增 `--no-cache` flag；`noCache` 传递给 `report.Run()` |
| `runUI()` | 创建 `FeedCache`；新增 `--no-cache` flag；传递给 `report.Run()` |
| `runSources()` | 创建 `FeedCache`（仅 `--live` 时）；新增 `--no-cache` flag |
| `runCacheClear()` | 新函数：清空 `feed_cache/` 目录 |
| `main()` switch | 新增 `"cache"` case |
| `usage()` | 增加 `--no-cache` 和 `news-report cache clear` 说明 |

---

## 8. 边界情况处理

### 8.1 磁盘满

**场景**: `os.WriteFile(tmp, data, ...)` 返回 `ENOSPC`。

**处理**: 
- 捕获错误，`fmt.Fprintf(os.Stderr, "警告: 缓存写入失败（磁盘满？）: %v\n", err)`
- 不崩溃，不返回错误给调用方——已获取的 body 正常返回给 `collectSource`
- 缓存只是加速手段，不可用时应优雅降级

### 8.2 损坏的缓存文件

**场景**: `os.ReadFile` 成功但 `json.Unmarshal` 失败（文件被截断、手动编辑错误等）。

**处理**:
- `os.Remove(path)` 删除损坏文件
- 返回 `false`（cache miss），触发冷抓取
- 与 `internal/store/store.go` 的 corrupt file 处理策略一致

### 8.3 并发写入同一文件

**场景**: 两个 goroutine 同时 cache miss 同一 URL（在 inflight 去重未生效的极端情况下，或不同 `key` 对应同一 `url`）。

**处理**:
- 两个 writer 各自执行 `WriteFile(tmp) + Rename(tmp, path)`
- 最后一个 `Rename` 胜出，之前写入的 tmp 文件可能成为孤儿（`.tmp` 后缀）
- 不会产生损坏的正式文件（`os.Rename` 是原子的）
- 孤儿 `.tmp` 文件积累问题：在 `evictIfNeeded` 扫描时，忽略 `.tmp` 后缀的文件（不参与 LRU），同时可选性地清理超过 1 小时的 `.tmp` 文件

### 8.4 并发淘汰与写入

**场景**: Writer A 正在写入新文件，Writer B 触发淘汰并可能删除 A 刚写入的文件。

**处理**:
- `evictIfNeeded` 持有 `c.mu` 锁
- 写入操作在 `evictIfNeeded` 返回后才释放 inflight channel
- 但有一个窗口：A 完成 `Rename` → 释放 inflight → B 进入 `Get` → 写入新条目 → 触发淘汰 → A 的文件可能刚被淘汰

**缓解**: 
- 淘汰到 80% 容量（滞回）意味着 A 的文件大概率不在淘汰集合内
- 冷却期（1s）限制淘汰频率，减少竞争窗口
- 即使被淘汰也无功能错误：下次 Get 会冷抓取

### 8.5 超大响应

**场景**: 某个 feed 返回 >500KB 的响应。

**处理**:
- `len(body) > c.maxEntry` → 不写入缓存
- `fmt.Fprintf(os.Stderr, "警告: 跳过缓存 %s (%d 字节超过 %d 上限)\n", url, len(body), c.maxEntry)`
- 仍然返回 body 给调用方（不影响功能，只是不缓存）

### 8.6 空响应体

**场景**: feed 返回 `200 OK` 但 body 为空（`len(body) == 0`）。

**处理**: 不写入缓存。解析层（`feed.Parse`）会返回 `"空内容"` 错误，这是正确的降级路径。

### 8.7 HTTP 错误状态码

**场景**: 服务端返回 4xx/5xx。

**处理**: 不缓存。仅 `200` 和 `304` 被缓存。`collectSource` 的错误处理逻辑不变。

### 8.8 网络错误时是否返回过期缓存

**场景**: TTL 已过期，发送条件请求，但网络不可达（`fetcher.Do` 返回错误）。

**处理**: 返回过期缓存（stale-while-revalidate）。策略：
- 条件请求失败 → `fmt.Fprintf(os.Stderr, "警告: 条件请求失败，使用过期缓存 %s: %v\n", url, err)`
- 返回 `cacheEntry.body`（过期但仍有参考价值）
- 不更新 `fetched_at`（下次仍会尝试条件请求）

### 8.9 首次运行（无缓存目录）

**场景**: `~/.cache/news-report/feed_cache/` 不存在。

**处理**: `os.MkdirAll(c.dir, 0o755)` 在 `New()` 或首次 `Get()` 时调用（与 `store.New()` 模式一致）。

### 8.10 `--no-cache` 下的行为

**场景**: 用户指定 `--no-cache`。

**处理**: `FeedCache.Get()` 完全跳过缓存层，直接调用 `fetcher.Do()`。不读取缓存、不写入缓存、不触发淘汰。inflight 去重也跳过（因为没有持久化需要保护）。

---

## 9. 性能预期

| 场景 | 首次运行 | 二次运行（全命中） | 二次运行（50% 命中） |
|------|----------|-------------------|---------------------|
| 缓存读取 | 0 (miss) | ~1700 次 × ~0.1ms ≈ 170ms | ~850 次 ≈ 85ms |
| 条件请求 | 0 | ~0–30 次（仅 Legacy tier 可能过期） | ~15 次 ≈ 5s |
| 冷抓取 | 120+ feeds ≈ 40s | ~0 | ~60 feeds ≈ 20s |
| **总耗时** | ~40s | **<2s** | ~25s |

> 条件请求的响应时间取决于服务端处理速度（通常 <200ms），远快于完整响应下载。

---

## 10. 测试策略

### 10.1 单元测试 (`cache_test.go`)

| 测试 | 覆盖 |
|------|------|
| `TestGet_Miss` | 磁盘缓存不存在 → 触发冷抓取 |
| `TestGet_Hit_Fresh` | TTL 内命中 → 返回缓存，不发起网络请求 |
| `TestGet_Hit_Expired_WithETag_304` | 过期但有 ETag → 条件请求 → 304 → 返回缓存 |
| `TestGet_Hit_Expired_WithETag_200` | 过期但有 ETag → 条件请求 → 200 → 返回新 body |
| `TestGet_Hit_Expired_NoValidator` | 过期且无 ETag/LM → 无条件重抓 |
| `TestGet_StaleOnError` | 条件请求网络错误 → 返回过期缓存（降级） |
| `TestGet_NoCache` | `--no-cache` 模式 → 始终冷抓取 |
| `TestTTL_TierWire` | wire 源 TTL = 7 min |
| `TestTTL_TierLegacy` | legacy 源 TTL = 12 min |
| `TestTTL_TierSpecialist` | specialist 源 TTL = 45 min |
| `TestTTL_Zh` | zh 语言源 TTL = 10 min（无论 tier） |
| `TestTTL_Override` | `CacheTTLMinutes = 5` 覆盖所有 |
| `TestTTL_ServerMaxAge` | 服务端 max-age=180 → min(tierTTL, 180) |
| `TestEvict_LRU` | 写满目录 → 最旧条目被淘汰 |
| `TestEvict_Hysteresis` | 淘汰到 80% 容量停止 |
| `TestEvict_EmptyDir` | 空目录不报错 |
| `TestEntry_TooLarge` | >500KB 不写入缓存 |
| `TestEntry_Corrupt` | 损坏 JSON → 删除 → 冷抓取 |
| `TestInflight_Dedup` | 并发请求同一 URL → 仅一次网络调用 |
| `TestClear` | Clear() 删除所有文件 |

### 10.2 集成测试

- **Smoke test** (`make smoke-cache`): 运行两次 `news-report --minutes 10 --limit 1`，验证第二次 `--no-cache` 与缓存模式的时间差异。
- **Mock HTTP server** 在单元测试中提供可控的响应（200/304/error/各种 Cache-Control 头）。

---

## 11. 文件清单

### 新增文件

| 文件 | 描述 |
|------|------|
| `internal/cache/cache.go` | FeedCache 结构体、New、Get、Clear、inflight 去重 |
| `internal/cache/entry.go` | cacheEntry 结构体、序列化/反序列化、TTL 计算函数 |
| `internal/cache/lru.go` | 目录扫描、大小统计、LRU 淘汰 |
| `internal/cache/cache_test.go` | 单元测试（≥15 个 case） |
| `docs/CACHE_ARCHITECTURE.md` | 本文档 |

### 修改文件

| 文件 | 变更 |
|------|------|
| `internal/config/config.go` | 新增 `CacheTTLMinutes`、`CacheMaxMB`、`CacheMaxEntryKB` 字段 + 默认值 + 校验 |
| `internal/report/report.go` | `Run()` 接受 `*cache.FeedCache` 参数；`collectSource` 使用 `cache.Get` |
| `main.go` | 创建 `FeedCache`；新增 `--no-cache` flag；新增 `cache clear` 子命令；`usage()` 更新 |
| `go.mod` | 无新依赖（全部使用标准库：`encoding/json`、`encoding/base64`、`sync`、`os`、`path/filepath`、`sort`、`time`） |

---

## 12. 实施步骤

| 步骤 | 内容 | 预估工时 |
|------|------|---------|
| 1 | 创建 `internal/cache/entry.go`：数据结构 + TTL 逻辑 | 30 min |
| 2 | 创建 `internal/cache/lru.go`：目录扫描 + 淘汰 | 30 min |
| 3 | 创建 `internal/cache/cache.go`：Get / New / Clear / inflight | 60 min |
| 4 | 修改 `internal/config/config.go`：新增字段 | 15 min |
| 5 | 修改 `internal/report/report.go`：集成 FeedCache | 20 min |
| 6 | 修改 `main.go`：CLI + 创建 FeedCache | 30 min |
| 7 | 编写 `internal/cache/cache_test.go`：测试 | 60 min |
| 8 | 集成测试 + 端到端验证 | 30 min |
| **合计** | | **~4.5 h** |
