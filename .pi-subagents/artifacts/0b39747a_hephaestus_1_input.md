# Task for hephaestus

根据下面的实现计划，在 news-report 项目中实现 LLM 集成功能。

实现计划如下：
我已经完整阅读了项目代码，现在产出详细实现计划。

---

# news-report LLM 集成方案 — 详细实现计划

## 一、整体架构

```
                    ┌───────────┐
                    │  main.go  │  (新增 --llm-model / --no-llm 标志)
                    └─────┬─────┘
                          │
     ┌────────────────────┼────────────────────┐
     │                    │                    │
┌────▼─────┐     ┌───────▼────────┐    ┌──────▼──────┐
│ config   │     │  internal/llm/ │    │  tui.go     │
│ +LLM 段  │     │  client.go     │    │ +t/x/d 键   │
└──────────┘     │  cache.go      │    │ +翻译/摘要  │
                 │  translate.go  │    │ +解读状态   │
                 │  analyze.go    │    └─────────────┘
                 └───────────────┘
```

新增一个独立的 `internal/llm/` 包，包含 OpenAI 兼容客户端、缓存、翻译/摘要/解读三个操作。TUI 通过消息机制异步调用 llm。

---

## 二、`internal/llm/` 包设计

### 2.1 文件结构（4 个文件 + 1 个测试）

| 文件 | 职责 | 估算行数 |
|------|------|---------|
| `internal/llm/client.go` | HTTP 客户端、OpenAI 兼容协议、env 变量解析、降级 | ~200 |
| `internal/llm/cache.go` | 输出缓存：sha256 key → JSON 文件，TTL 可配 | ~150 |
| `internal/llm/translate.go` | 分块翻译逻辑（段落分块、重叠、系统提示词） | ~120 |
| `internal/llm/analyze.go` | 摘要（5点要点）和深度解读（4段式） | ~100 |
| `internal/llm/llm_test.go` | 单元测试（mock HTTP server） | ~200 |
| **合计** | | **~770** |

### 2.2 `client.go` 接口设计

```go
// Package llm 提供 OpenAI 兼容 /chat/completions 客户端及新闻专用操作。
package llm

// Client 是 LLM 客户端。
type Client struct {
    cfg    LLMConfig
    http   *http.Client
    cache  *Cache
}

// LLMConfig 来自 config.Config 的 LLM 段（值拷贝，避免外部修改）。
type LLMConfig struct {
    BaseURL    string        // 默认 https://api.openai.com/v1
    APIKey     string        // 已解析 {env:VAR} 后的实际 key，空 = 未配置
    Model      string        // 默认 gpt-4o-mini
    TargetLang string        // 翻译目标语言，默认 zh
    Timeout    time.Duration // 默认 60s
    MaxChars   int           // 翻译前截断字数，0=不限，默认 4000
    ChunkSize  int           // 分块大小（字），默认 1500
    Overlap    int           // 块间重叠（字），默认 50
}

// New 创建客户端。apiKey 为空时客户端仍可创建但 Chat() 返回 ErrNoKey。
func New(cfg LLMConfig, cacheDir string) *Client

// Available 返回是否已配置 API key（用于 UI 判断是否显示 LLM 功能）。
func (c *Client) Available() bool

// Chat 发送单轮对话，返回助手回复文本。
func (c *Client) Chat(ctx context.Context, systemPrompt, userMessage string) (string, error)
```

**关键设计决策**：

1. **无 key 优雅降级**：`New()` 总是成功创建，`Chat()` 在 key 为空时返回 `ErrNoKey` sentinel error。调用方据此显示可操作的提示信息，不 crash。

2. **OpenAI 兼容协议**：POST `{BaseURL}/chat/completions`，body 格式：
   ```json
   {
     "model": "...",
     "messages": [{"role":"system","content":"..."}, {"role":"user","content":"..."}],
     "temperature": 0.3,
     "max_tokens": 2048
   }
   ```
   - 请求头：`Authorization: Bearer {APIKey}`, `Content-Type: application/json`
   - 非 2xx 响应：解析 OpenAI 错误格式 `{"error":{"message":"..."}}`，返回 wrapped error
   - 重试：同 fetch 包策略，仅 5xx/网络错误重试 2 次

3. **`{env:VAR}` 解析**：在 `config.Load()` 阶段由 config 包统一处理，llm 包接收已解析的纯文本 key。解析逻辑：
   - 正则 `\{env:([A-Za-z_][A-Za-z0-9_]*)\}` 匹配
   - 调用 `os.Getenv(VAR)` 替换
   - 环境变量不存在 → 替换为空字符串（静默，由 Chat 时 ErrNoKey 降级）

4. **依赖**：仅使用标准库 `net/http` + `encoding/json` + `crypto/sha256`，不引入第三方 HTTP 库。

### 2.3 `cache.go` 缓存设计

```go
// Cache 是 LLM 输出缓存，存于 ~/.cache/news-report/llm/。
type Cache struct {
    dir    string
    ttl    time.Duration  // 默认 7 天
    maxMB  int64
    mu     sync.RWMutex
}

// CacheEntry 是一条缓存。
type CacheEntry struct {
    Key       string `json:"key"`        // sha256(URL+lang+instruction)
    Response  string `json:"response"`
    CreatedAt int64  `json:"created_at"` // unix 秒
}

func NewCache(dir string, ttl time.Duration, maxMB int64) *Cache
func (c *Cache) Get(key string) (string, bool)   // 命中返回 response，过期返回 false
func (c *Cache) Set(key string, response string) error
func (c *Cache) Purge() (int, error)              // LRU 淘汰至 ≤ maxMB
```

**缓存 key 生成规则**（在调用方生成，不在 cache 包内）：

```go
func cacheKey(url, lang, instruction string) string {
    payload := url + "|" + lang + "|" + instruction
    return sha256Hex(payload)  // 复用项目中已有的 sha256 图案（或用 crypto/sha256）
}
```

三种指令类型常量：
- `"translate"` — 翻译
- `"summary"` — 摘要（5 要点）
- `"brief"` — 深度解读

**缓存 TTL**：7 天。翻译/分析结果半衰期长，缓存命中即零成本。

**缓存目录**：`~/.cache/news-report/llm/`，与 `feed_cache`、`articles` 同级。`news-report cache stat/clear` 命令同步纳入统计和清理。

**注意**：不重用现有 `cache.Cache`——它按 tier 区分 TTL 且使用 `{sha256}.json` 扁平命名加 `Entry` 结构含 ETag/LastMod，语义不适合 LLM 输出缓存。LLM 缓存更简单：统一 TTL + 无 HTTP 元数据。两个缓存可共存于不同子目录。

### 2.4 `translate.go` 翻译逻辑

```go
// ErrNoKey 表示未配置 API key。
var ErrNoKey = errors.New("llm: 未配置 API key，请在 config.yaml 中设置 llm.api_key")

// Translate 翻译文本到目标语言。
// 自动分块：≤ MaxChars 的文本按段落边界分块，块大小 ≤ ChunkSize，重叠 Overlap 字符。
// 每个块独立调用 Chat，结果拼接返回。
func (c *Client) Translate(ctx context.Context, text, sourceLang string) (string, error)

// chunkText 按段落边界分块，确保每块 ≤ maxChars，重叠 overlapChars。
func chunkText(text string, maxChars, overlapChars int) []string
```

**系统提示词**（硬编码常量）：
```
你是一名专业新闻译者。将以下新闻文本翻译成中文。要求：
1. 保留专有名词原文（人名、地名、机构名首次出现时保留原文并括号标注中文）
2. 保留所有数字、百分比、日期精确不变
3. 保留直接引语的原意和语气
4. 专业术语准确翻译
5. 仅输出译文，不要加任何解释或注释
```

**分块策略详述**：
1. 取文本前 `MaxChars`（默认 4000）个字符
2. 按 `\n\n` 分段（段落）
3. 贪心合并段落：当前块 + 下一段 > `ChunkSize`（默认 1500）则闭合当前块
4. 块间重叠：下一块的起始位置回退 `Overlap`（默认 50）字符，而非从下一段开始。这需要在段落级分块后，在文本级做重叠窗口——简化做法：直接对段落序列做滑动拼接，不精确回退到字符级。
5. 每个块调用 `Chat(ctx, systemPrompt, chunk)` 
6. 拼接结果：块间用 `\n\n` 连接

**简化后分块算法**（避免过度工程）：
```
输入: text, maxChars=4000, chunkSize=1500, overlap=50
1. body = text[:maxChars]
2. paragraphs = split(body, "\n\n")
3. chunks = []
4. cur = ""
5. for p in paragraphs:
     if len([]rune(cur)) + len([]rune(p)) > chunkSize && cur != "":
         chunks.append(cur)
         // 重叠：取 cur 末尾 overlap 字符作为新 cur 的开头
         overlapText = lastNChars(cur, overlap)
         cur = overlapText + p
     else:
         cur += "\n\n" + p (if cur != "")
6. if cur != "": chunks.append(cur)
7. 对每个 chunk 调 Chat，结果 join "\n\n"
```

### 2.5 `analyze.go` 摘要与解读

```go
// Summarize 生成 5 条要点摘要（每条 ≤25 字）。
func (c *Client) Summarize(ctx context.Context, title, body string) ([]string, error)

// Brief 生成四段式深度解读。
// 返回结构：{Background, Positions, Impact, Outlook}
func (c *Client) Brief(ctx context.Context, title, body string) (*BriefResult, error)

type BriefResult struct {
    Background string // 背景：事件来龙去脉
    Positions  string // 各方立场与利益分析
    Impact     string // 影响：对政策/市场/国际关系的可能影响
    Outlook    string // 后续关注点
}
```

**摘要系统提示词**：
```
你是一名新闻编辑。阅读以下新闻，提取 5 条核心要点。每条 ≤25 个汉字。
用数字序号 1. 2. 3. 4. 5. 列出，不要其他内容。
```

**解读系统提示词**：
```
你是一名资深国际新闻分析师。请对以下新闻进行四段式深度解读：

第一段「背景」：简述事件背景和来龙去脉
第二段「各方立场」：分析涉及各方的立场与利益诉求
第三段「影响」：分析事件对政策、市场、国际关系的可能影响
第四段「后续关注」：指出读者应持续关注的后续发展

每段 100-200 字。用 "## 背景"、"## 各方立场"、"## 影响"、"## 后续关注" 作为段落标题。
```

**无正文处理**：Summarize/Brief 调用前由 TUI 层判断 `item.Body` 是否为空，为空则先调用 `article.Extract()` 获取正文。这个逻辑在 TUI 层做（fetchThenLLM 消息），不在 llm 包内——llm 包是纯函数，不混合 fetch 职责。

---

## 三、`config.go` 变更

### 3.1 新增 LLMConfig 字段

在 `Config` struct 末尾新增：

```go
type Config struct {
    // ... 现有字段保持不变 ...

    LLM LLMConfig `yaml:"llm"`
}

type LLMConfig struct {
    BaseURL    string `yaml:"base_url"`    // 默认 https://api.openai.com/v1
    APIKey     string `yaml:"api_key"`     // 支持 {env:OPENAI_API_KEY}
    Model      string `yaml:"model"`       // 默认 gpt-4o-mini
    TargetLang string `yaml:"target_lang"` // 默认 zh
    TimeoutSec int    `yaml:"timeout"`     // 默认 60
    MaxChars   int    `yaml:"max_chars"`   // 翻译截断字数，默认 4000
}
```

### 3.2 Default() 新增默认值

```go
LLM: LLMConfig{
    BaseURL:    "https://api.openai.com/v1",
    APIKey:     "",  // 空 = 未配置，运行时优雅降级
    Model:      "gpt-4o-mini",
    TargetLang: "zh",
    TimeoutSec: 60,
    MaxChars:   4000,
},
```

### 3.3 Load() 中解析 `{env:VAR}`

在 `Load()` 中反序列化后，调用新增的 `resolveEnvRefs`：

```go
func (c *Config) resolveEnvRefs() {
    c.LLM.APIKey = resolveEnv(c.LLM.APIKey)
    c.LLM.BaseURL = resolveEnv(c.LLM.BaseURL)
    // 未来可扩展：Proxy 等
}

var envRefRe = regexp.MustCompile(`\{env:([A-Za-z_][A-Za-z0-9_]*)\}`)

func resolveEnv(s string) string {
    return envRefRe.ReplaceAllStringFunc(s, func(match string) string {
        varName := match[5 : len(match)-1] // 去掉 {env: 和 }
        return os.Getenv(varName)
    })
}
```

### 3.4 config.yaml 示例

```yaml
llm:
  base_url: "https://api.openai.com/v1"    # 或任何兼容端点
  api_key: "{env:OPENAI_API_KEY}"          # 从环境变量读取，不写明文
  model: "gpt-4o-mini"
  target_lang: "zh"
  timeout: 60
  max_chars: 4000
```

### 3.5 Validate() 新增校验

```go
// llm 段全部可选，只校验已填字段的合法性
if c.LLM.TimeoutSec < 0 || c.LLM.TimeoutSec > 300 {
    return errors.New("llm.timeout 必须在 0-300 之间")
}
if c.LLM.MaxChars < 0 {
    return errors.New("llm.max_chars 不能为负数")
}
```

### 3.6 新增 CLI 标志（main.go runUI）

```go
// ui 子命令新增
llmModel := fs.String("llm-model", "", "LLM 模型覆写（如 gpt-4o）")
noLLM := fs.Bool("no-llm", false, "禁用 LLM 功能")
```

传递到 `cfg.LLM.Model` 覆写，`noLLM` 通过清空 `cfg.LLM.APIKey` 实现。

---

## 四、TUI 集成方案（`tui.go` 变更）

### 4.1 Model 新增字段

```go
type Model struct {
    // ... 现有字段 ...

    llm     *llm.Client     // 由 cfg 初始化，nil 表示未配置
    llmDone chan struct{}   // 用于等待 llm goroutine 退出（defer close）

    // 翻译状态（嵌入 readerState 或在 Model 层）
    translating bool          // 翻译进行中
    translated  string        // 已缓存的翻译结果
    originalBody string       // 翻译前原文副本（用于 Esc 复原）

    // 摘要/解读弹出层
    popup       *popupState   // nil = 无弹出
}

type popupState struct {
    title   string
    content string   // 渲染后的文本
    loading bool
    err     string
    lines   []string // 预折行
    offset  int
}
```

### 4.2 新增消息类型

```go
type llmTranslateMsg struct {
    body string   // 翻译结果
    err  error
}

type llmSummaryMsg struct {
    points []string
    err    error
}

type llmBriefMsg struct {
    brief *llm.BriefResult
    err   error
}
```

### 4.3 新增异步命令

```go
func llmTranslateCmd(client *llm.Client, body, lang string) tea.Cmd {
    return func() tea.Msg {
        ctx, cancel := context.WithTimeout(context.Background(), client.Timeout())
        defer cancel()
        result, err := client.Translate(ctx, body, lang)
        return llmTranslateMsg{body: result, err: err}
    }
}

func llmSummaryCmd(client *llm.Client, item *report.Item, artCache *cache.Cache, f *fetch.Fetcher) tea.Cmd {
    return func() tea.Msg {
        body := item.Body
        if body == "" {
            // 无正文：先抓取
            art, err := article.Extract(context.Background(), f, item.URL, item.Lang)
            if err != nil {
                return llmSummaryMsg{err: fmt.Errorf("获取正文失败: %w", err)}
            }
            body = art.Text
        }
        ctx, cancel := context.WithTimeout(context.Background(), client.Timeout())
        defer cancel()
        points, err := client.Summarize(ctx, item.Title, body)
        return llmSummaryMsg{points: points, err: err}
    }
}

func llmBriefCmd(client *llm.Client, item *report.Item, artCache *cache.Cache, f *fetch.Fetcher) tea.Cmd {
    // 同 llmSummaryCmd 模式，调用 client.Brief()
}
```

### 4.4 键位处理变更

**`handleListKey` 新增**：

```go
case "x":
    // 列表视图：对当前条目生成摘要
    if !m.llm.Available() {
        m.status = "LLM 未配置，请在 config.yaml 设置 llm.api_key"
        return m, nil
    }
    items := m.visibleItems()
    if m.cursor < len(items) {
        it := items[m.cursor]
        m.popup = &popupState{title: "摘要: " + it.Title, loading: true}
        return m, llmSummaryCmd(m.llm, &it, m.artCache, m.fetcher)
    }

// 注：x 键当前无绑定，不会冲突
```

**`handleReaderKey` 新增**：

```go
case "t":
    // 翻译当前正文
    if !m.llm.Available() {
        m.status = "LLM 未配置"
        return m, nil
    }
    if m.reader == nil || m.reader.body == "" {
        return m, nil
    }
    // 如果已经是翻译状态，Esc 恢复原文
    if m.translating {
        m.reader.body = m.originalBody
        m.reader.lines = splitWrapped(m.originalBody, readerWidth(m.width))
        m.translating = false
        m.status = "已恢复原文"
        return m, nil
    }
    // 检查是否已有缓存翻译
    if m.translated != "" {
        m.originalBody = m.reader.body
        m.reader.body = m.translated
        m.reader.lines = splitWrapped(m.translated, readerWidth(m.width))
        m.translating = true
        m.status = "译文（缓存）| 按 t/Esc 回原文"
        return m, nil
    }
    // 发起异步翻译
    m.originalBody = m.reader.body
    m.translating = true
    m.status = "翻译中…"
    return m, llmTranslateCmd(m.llm, m.reader.body, m.reader.item.Lang)

case "d":
    // 深度解读（类似 t 的逻辑，但结果显示在弹窗中）
    if !m.llm.Available() {
        m.status = "LLM 未配置"
        return m, nil
    }
    if m.reader == nil || m.reader.item == nil {
        return m, nil
    }
    m.popup = &popupState{title: "解读: " + m.reader.item.Title, loading: true}
    return m, llmBriefCmd(m.llm, m.reader.item, m.artCache, m.fetcher)
```

**`t` 键 Esc 回原文逻辑**：
- 当 `m.translating == true` 且 `m.view == viewReader` 时，`Esc` 的行为改为恢复原文而不是退出阅读器。修改 `handleReaderKey` 中 Esc 分支：
  ```go
  case "esc":
      if m.translating {
          m.reader.body = m.originalBody
          m.reader.lines = splitWrapped(m.originalBody, readerWidth(m.width))
          m.translating = false
          m.status = "已恢复原文"
          return m, nil
      }
      m.view = viewList
      m.reader = nil
  ```

### 4.5 Update 中新增消息处理

```go
case llmTranslateMsg:
    m.status = ""
    if msg.err != nil {
        m.status = "翻译失败: " + msg.err.Error()
        m.translating = false
        if m.originalBody != "" {
            m.reader.body = m.originalBody
            m.reader.lines = splitWrapped(m.originalBody, readerWidth(m.width))
        }
    } else {
        m.translated = msg.body
        if m.reader != nil {
            m.reader.body = msg.body
            m.reader.lines = splitWrapped(msg.body, readerWidth(m.width))
            m.reader.offset = 0
        }
        m.status = "翻译完成 | 按 t/Esc 回原文"
    }

case llmSummaryMsg:
    m.popup.loading = false
    if msg.err != nil {
        m.popup.err = msg.err.Error()
    } else {
        var sb strings.Builder
        for i, p := range msg.points {
            fmt.Fprintf(&sb, "%d. %s\n", i+1, p)
        }
        m.popup.content = sb.String()
        m.popup.lines = strings.Split(m.popup.content, "\n")
    }

case llmBriefMsg:
    m.popup.loading = false
    if msg.err != nil {
        m.popup.err = msg.err.Error()
    } else {
        b := msg.brief
        m.popup.content = fmt.Sprintf("## 背景\n%s\n\n## 各方立场\n%s\n\n## 影响\n%s\n\n## 后续关注\n%s",
            b.Background, b.Positions, b.Impact, b.Outlook)
        m.popup.lines = splitWrapped(m.popup.content, readerWidth(m.width))
    }
```

### 4.6 View 中弹窗渲染

在 `listView()` 或 `readerView()` 末尾，如果 `m.popup != nil`，叠加一个居中的弹出层：

```go
if m.popup != nil {
    return overlayPopup(baseView, m.popup, m.width, m.height)
}
```

弹窗样式：半透明背景遮罩 + 居中边框窗口 + 标题栏 + 可滚动内容 + Esc 关闭。复用现有 `styleReader` / `styleTitle` 等样式。

弹窗按键（在 `handleKey` 最外层优先拦截）：
- `Esc`：关闭弹窗 `m.popup = nil`
- `↑↓/PgUp/PgDn`：滚动弹窗内容

### 4.7 helpView 新增键位

```
t         翻译当前正文（再按 Esc 恢复原文）
x         生成 5 点摘要（列表视图）
d         深度解读（阅读器视图）
```

---

## 五、缓存策略细节

### 5.1 LLM 缓存独立于 Feed 缓存

| 属性 | Feed 缓存 (`feed_cache/`) | LLM 缓存 (`llm/`) |
|------|--------------------------|-------------------|
| Key | `sha256(url)` | `sha256(url+lang+instruction)` |
| TTL | 按 tier 6-30min | 统一 7 天 |
| 存储 | JSON `{url,body,fetched,etag,lastmod}` | JSON `{key,response,created_at}` |
| 淘汰 | LRU 按 mtime（`Purge()`） | 同模式 LRU |
| 大小限制 | `cache_max_mb` (50MB) | `cache_max_mb` 共享 or 独立 20MB |

**建议**：LLM 缓存使用独立的 `llm_max_mb` 配置（默认 20MB），因为 LLM 输出量小（每次几百到几千字），20MB 足以缓存数百条结果。如果不想新增配置字段，可以共享 `cache_max_mb` 但语法上要确保 `cache.Clear()` 能区分两个缓存目录。

### 5.2 缓存命中流程

```
TUI 按 t 键
  → Model 持有 llm.Client（含 cache 引用）
  → llmTranslateCmd 内:
     1. 生成 cacheKey = sha256(url + "|" + lang + "|translate")
     2. cache.Get(cacheKey) → 命中直接返回（tea.Cmd 立即返回结果）
     3. 未命中 → HTTP Chat() → 成功后 cache.Set(cacheKey, response)
```

注意：缓存查找在 `llmTranslateCmd` 内部（goroutine 内），不在 bubbletea Update 中，避免阻塞渲染循环。

### 5.3 cache stat 集成

`main.go` 的 `cacheStat()` 函数新增一行：

```go
{"llm", "LLM 翻译/摘要缓存（TTL 7 天）"},
```

`runCache clear` 新增清理目标：

```go
for _, sub := range []string{"feed_cache", "articles", "llm", "seen.json"} {
```

---

## 六、错误处理和降级逻辑

### 6.1 错误层级

| 层级 | 场景 | 处理 |
|------|------|------|
| **配置层** | API key 未设置 | `Client.Available()==false`，TUI 显示"LLM 未配置，请在 config.yaml 设置 llm.api_key" |
| **网络层** | 超时 / 5xx / DNS 失败 | 重试 2 次（同 fetch 包策略），最终失败显示"翻译失败: timeout"等 |
| **协议层** | 4xx（401/403/429） | 不重试，显示具体错误如"API key 无效 (401)" |
| **内容层** | 模型返回空响应 | 显示"翻译结果为空，可能模型不支持该语言" |
| **内容层** | 响应过长被截断 | max_tokens=2048 兜底，不处理 |
| **缓存层** | 磁盘满 / 权限错误 | 静默跳过缓存写入，不影响主流程 |

### 6.2 降级矩阵

```
用户按 t 键
  ├─ llm == nil (未配置)         → 状态栏提示，不阻塞
  ├─ !llm.Available() (key空)    → 状态栏提示，不阻塞
  ├─ 缓存命中                     → 即时显示译文，零成本
  ├─ 缓存未命中 → Chat()
  │    ├─ 成功                   → 显示译文 + 写缓存
  │    ├─ 网络超时                → 显示"翻译超时，请重试"
  │    ├─ 401/403                → 显示"API key 无效，请检查配置"
  │    └─ 其他错误               → 显示具体错误信息
  └─ 翻译过程中按 t 再次         → 恢复原文（状态机保护）
```

### 6.3 状态栏信息设计

```
正常状态：不显示或显示上次操作结果（5s 后自动清除）
翻译中："翻译中…"
翻译完成："翻译完成 | 按 t/Esc 回原文"
翻译失败："翻译失败: <具体错误>"
摘要中："正在生成摘要…"
解读中："正在生成解读…"
LLM 未配置："LLM 未配置，请在 ~/.config/news-report/config.yaml 中设置 llm.api_key"
```

---

## 七、预计新增/修改文件清单

### 新增文件（5 个）

| 文件 | 行数 | 说明 |
|------|------|------|
| `internal/llm/client.go` | ~200 | HTTP 客户端 + Chat 方法 + env 解析 |
| `internal/llm/cache.go` | ~150 | 输出缓存 |
| `internal/llm/translate.go` | ~120 | 翻译 + 分块 |
| `internal/llm/analyze.go` | ~100 | 摘要 + 解读 |
| `internal/llm/llm_test.go` | ~200 | 测试（mock HTTP server + 缓存测试） |
| **新增合计** | **~770** | |

### 修改文件（4 个）

| 文件 | 变更行数 | 说明 |
|------|---------|------|
| `internal/config/config.go` | +50, -0 | 新增 LLMConfig 结构体 + Default 值 + Validate 校验 + `{env:VAR}` 解析 |
| `internal/config/config_test.go` | +30, -0 | LLM 默认值、env 解析测试 |
| `internal/tui/tui.go` | +200, -10 | Model 新增 llm 字段、新消息类型、新键位处理、弹窗渲染 |
| `main.go` | +30, -5 | runUI 新增 --llm-model/--no-llm 标志、cache stat 纳入 llm 目录、usage 更新 |
| **修改合计** | **~310** | |

### 总计

- 新增 ~770 行
- 修改 ~310 行
- 总变更 ~1080 行

---

## 八、实现顺序建议

| 阶段 | 内容 | 依赖 | 可独立测试 |
|------|------|------|-----------|
| **P1** | `internal/llm/client.go` + `cache.go` — 客户端和缓存层 | 无 | mock HTTP server |
| **P2** | `config.go` LLM 段 — 配置结构和解析 | P1 | 配置加载测试 |
| **P3** | `internal/llm/translate.go` — 翻译逻辑 | P1 | 单元测试（mock） |
| **P4** | `internal/llm/analyze.go` — 摘要和解读 | P1 | 单元测试（mock） |
| **P5** | `tui.go` t 键翻译 | P3 | 手动 TUI 测试 |
| **P6** | `tui.go` x/d 键摘要/解读 + 弹窗 | P4 | 手动 TUI 测试 |
| **P7** | `main.go` CLI 标志 + cache stat 集成 | P2 | 端到端测试 |

P1-P4 可在不碰 TUI 的情况下完成并测试，P5-P7 是集成层。

---

## 九、关键边界条件和注意事项

1. **Go 标准库优先**：整个 llm 包仅用 `net/http` + `encoding/json` + `crypto/sha256`，不引入 OpenAI SDK 或第三方 HTTP 库，保持项目"零重依赖"风格。

2. **并发安全**：`Client` 可被多个 goroutine 同时调用（从 bubbletea 的 tea.Cmd 并发），`Cache` 已有 `sync.RWMutex` 保护。

3. **翻译与原文切换**：`t` 键在翻译完成后再按一次不应重新翻译，而是回到原文。使用 `m.translating` bool 标志 + `m.originalBody` 备份实现。按 Esc 也回原文（同时退出阅读器—仅当未在翻译状态时）。

4. **弹窗与阅读器共存**：`d` 键在阅读器中触发解读弹窗，阅读器仍在背后，弹窗关闭后回到阅读器。这需要 `m.view` 保持 `viewReader` 不变，仅 `m.popup != nil` 决定是否叠加弹窗。

5. **摘要无正文处理**：`x` 键在列表视图触发，此时 `item.Body` 大概率为空（因为列表项来自 feed 摘要，不会预抓全文）。因此 `llmSummaryCmd` 内需先 `article.Extract()` 获取正文。这个 fetch 可以复用 `m.artCache` 缓存的文章（之前在阅读器中抓过的）。

6. **跨平台**：llm 包不涉及平台特定代码，TUI 集成也仅使用 bubbletea 抽象。

7. **测试策略**：mock HTTP server 返回预设 JSON，验证 Chat/Translate/Summarize/Brief 的请求格式和解析正确性。缓存测试使用临时目录。

8. **项目已有 sha256**：`cache.go` 中使用的是自定义简化 hash 函数（对文件名唯一性足够）。LLM 缓存的 key 同样可用此函数，保持一致。或者改用 `crypto/sha256` 标准库以获得真正的 SHA-256——考虑到 LLM 缓存 key 需要跨会话稳定，标准 sha256 更合适。

---

要求：
1. 严格按照计划实现，如有不合理之处请修正并说明
2. 写完代码后必须运行 go build ./... 和 go test -race ./... 确保全部通过
3. 部署最新二进制到 ~/.local/bin/news-report（make install）
4. 所有新增代码需包含中文注释
5. 缓存文件统一放在 ~/.cache/news-report/llm/ 下
6. LLM 请求失败时优雅降级，不影响现有功能
7. 测试文件命名规范 xxx_test.go，覆盖核心逻辑

## Acceptance Contract
Acceptance level: attested
Completion is not accepted from prose alone. End with a structured acceptance report.

Criteria:
- criterion-1: Return concrete findings with file paths and severity when applicable

Required evidence: review-findings, residual-risks

Finish with a fenced JSON block tagged `acceptance-report` in this shape:
Use empty arrays when no items apply; array fields contain strings unless object entries are shown.
`criteriaSatisfied[].status` must be exactly one of: satisfied, not-satisfied, not-applicable.
`commandsRun[].result` must be exactly one of: passed, failed, not-run.
`manualNotes` and `notes` are optional strings; an empty string means no note and does not satisfy `manual-notes` evidence.
```acceptance-report
{
  "criteriaSatisfied": [
    {
      "id": "criterion-1",
      "status": "satisfied",
      "evidence": "specific proof"
    }
  ],
  "changedFiles": [
    "src/file.ts"
  ],
  "testsAddedOrUpdated": [
    "test/file.test.ts"
  ],
  "commandsRun": [
    {
      "command": "command",
      "result": "passed",
      "summary": "short result"
    }
  ],
  "validationOutput": [
    "validation output or concise summary"
  ],
  "residualRisks": [
    "none"
  ],
  "noStagedFiles": true,
  "diffSummary": "short description of the diff",
  "reviewFindings": [
    "blocker: file.ts:12 - issue found, or no blockers"
  ],
  "manualNotes": "anything else the parent should know"
}
```