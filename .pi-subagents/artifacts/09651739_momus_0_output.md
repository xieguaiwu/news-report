Now I have all the information needed. Let me compile the final review.

---

# Momus Code Review — news-report v0.2.0

---

## CRITICAL

### CRITICAL-1: `internal/tui/tui.go:268-278` — readerView 按字节切片导致 UTF-8 多字节字符被截断

**证据**:
```go
// tui.go:268-278
visible := m.reader.body
start := m.reader.offset
if start > len(visible) {
    start = len(visible)
}
end := start + (m.height-6)*3
if end > len(visible) {
    end = len(visible)
}
sb.WriteString(styleReader.Render(visible[start:end]))
```

`m.reader.offset` 是字节偏移量（scrollDown 使用 `len(r.body)` 做边界判断，scrollUp/Down 按整数字节递增），但 `visible[start:end]` 是 Go 的字节切片。当中文（CJK 3 字节/字）、德语法语（变音 2 字节 `ß=2, ü=2`）出现在 body 中时，`start` 可能切在某个 rune 的中间字节，导致渲染出乱码/替换字符（U+FFFD）。

**同时影响**:
- `tui.go:365` `r.offset = len(r.body)` — Home/End 用字节长度做偏移
- `tui.go:370` 相同的字节偏移比较

**修复建议**: 将 `readerState.body` 预先转为 `[]rune`（或在 readerView 中用 `[]rune(visible)` 按字符索引切片），offset 也改为 rune 偏移。注意内存开销——完整文章可能很大，可考虑只存 `[]rune` 或按需转换当前窗口。

---

## HIGH

### HIGH-1: `internal/classify/classify.go:79-87,269-281` — "federal" 单 token (uspolitics ×2) 与 "Federal Reserve" 财经新闻误分类

**证据**:
词表中 `"federal"` 属于 `uspolitics`（带 ×2 权重），`"federalreserve"` 是 `economy` 的复合 token。但 go 的 tokenizer 按 `unicode.IsLetter` 切分——输入 "Federal Reserve" 被切为 `["federal", "reserve"]`，**永远无法匹配** `"federalreserve"`。

```
输入: "Federal Reserve raises interest rates ahead of election"
tokenize → ["federal", "reserve", "raises", "interest", "rates", "ahead", "of", "election"]

Token-based:
  uspolitics: "federal" → us = 1×2 = 2
  politics: "election" → pol = 1
  economy: 全部 token 均不命中（"federalreserve" 不匹配 "federal"）

Multi-word:
  multiEconomy: "interest rates" → eco += 2

最终: us=2, pol=1, eco=2 → tie-break 顺序 USPolitics 在前 → 分类为 USPolitics
期望: Economy
```

虽然多词短语 `"interest rates"` 为 Economy 加了 2 分，但 USPolitics 的 `"federal"×2` 同样得了 2 分，而 tie-break（`{USPolitics, us}` 排在 `{Politics, pol}` 之前）让 USPolitics 获胜。

**影响**: 所有提及 Federal Reserve 的财经文章（"Fed rate decision", "Federal Reserve outlook" 等）在同时提及 elections 或无其他强信号时会误分类为 USPolitics。

**修复建议**: 
- 方案 A（推荐）: 在 economy 单 token 中增加 `"fed"`（已有）并增加 `"reserve"`（如果单 token 太泛，就只靠多词短语，但同时降低 uspolitics 的 `"federal"` 权重到 ×1）
- 方案 B: 在 tie-break 策略中，如果 us 和 eco 同分且 multiUSPolitics 未命中（即 US 专属多词信号缺失），降低 us 权重
- 方案 C: 拆分 `"federal"` 为更精确的 US 政治信号（如 `"federal government"`, `"federal court"` 等多词），`"federal"` 单独不带 ×2

### HIGH-2: `internal/feed/feed.go:134` — CST 时区歧义与注释矛盾

**证据**:
```go
// 注释: "歧义的（如 IST）不收录"
// 但实际上收录了 CST:
"CST": -6 * 3600, "CDT": -5 * 3600,
```

CST 至少有三个含义:
- US Central Standard Time: UTC-6 ✓（当前选择）
- China Standard Time: UTC+8 ✗
- Cuba Standard Time: UTC-5 ✗

当前项目聚焦"欧美权威媒体"，US CST 是正确的默认解释。但项目 v0.2.0 已支持中文来源（zh 语言），一旦中文 RSS feed 中出现 CST +8，会被错误解析为 -6，产生 14 小时偏差。

**修复建议**: 要么将注释改为明确说明"CST 按 US Central 处理（项目欧美媒体定位）"，要么从表中移除 CST（让它回退到 Go 默认处理，虽然 Go 也会按偏移 0 解析但至少不会产生静默的 14h 错误）。

---

## MEDIUM

### MEDIUM-1: `internal/main.go:322-340` — reorderArgs 将 `--`（POSIX 选项终止符）误认为 flag

**证据**:
```go
if strings.HasPrefix(a, "-") && a != "-" {
    ordered = append(ordered, a)
```
`--` 满足 `HasPrefix("--", "-")` 且 `"--" != "-"`，会被归入 flag 组。在 `fs.Parse()` 中 `--` 是 end-of-flags 标记，之后的所有参数变为 positional——**这恰好和 reorderArgs 把 `--` 放前面的意图一致**。但如果用户在 `--` 后跟随了 flag（如 `news-report find -- --lang en query`），reorderArgs 会把 `--lang` 也前置，而 `fs.Parse` 看到 `--` 后就停止解析 flag 了，`--lang` 变成 positional。结果：`--lang` 成为搜索词的一部分。

**实际情况**: read/find/ui 子命令几乎不涉及 `--` 用例，风险低，但作为通用参数重排函数这是逻辑缺陷。

**修复建议**: 在 reorderArgs 中检测到 `--` 时直接标记后续全部为 positional。

### MEDIUM-2: `internal/classify/classify.go:170` — `textLower` 冗余计算

```go
text := strings.ToLower(title + " " + summary)
text = normalize(text)   // normalize 不引入大写
// ...
textLower := strings.ToLower(text)  // ← 冗余：text 已全部小写
```

对于 en/de/fr 每次都多做一次无意义的 `ToLower`。zh 分支则无影响（中文无大小写）。性能影响可忽略，但代码可读性受影响（读者会困惑为什么有两次 lowering）。

**修复建议**: 删除 `textLower`，zh 分支直接用 `text`。

### MEDIUM-3: `internal/gnews/gnews.go:98-106` — zh 硬编码为 zh-TW/TW，可能影响搜索结果地域偏向

```go
case "zh":
    return "zh-TW", "TW"
```

Google News 的 `hl`/`gl` 参数影响搜索结果的地域排序和内容偏向。zh-TW/TW 会让搜索结果偏向台湾媒体视角，对于本项目"欧美权威媒体"的定位，可能遗漏其他华语来源（如新加坡联合早报、港媒等在国际新闻上的覆盖）。

**修复建议**: 考虑支持 zh-CN 或至少增加注释说明此决策。

### MEDIUM-4: 全局 `Version` 常量/函数均停留在 `0.1.0`

**证据**:
- `config/config.go:18`: `Version = "0.1.0"`
- `gnews/gnews.go:112`: `return "0.1.0"`
- `output/output.go:190`: `return "0.1.0"`
- `output/output_test.go:78`: 测试期望 `"news-report v0.1.0"`

所有版本声明均未更新到 v0.2.0，导致 User-Agent、版本输出均展示过时版本号。

**修复建议**: 统一更新到 `"0.2.0"`（含测试中的期望值）。

---

## LOW

### LOW-1: `internal/report/report.go:171-181` — catOrder 零值语义导致未指定分类与首个指定分类排序同级

**证据**:
```go
catOrder := map[classify.Category]int{}
for i, c := range opts.Categories {
    catOrder[classify.Category(c)] = i
}
catOrder[classify.Other] = len(catOrder)
```

当 `opts.Categories = ["economy"]` 且未启用 `StrictFocus` 时，`catOrder["economy"] = 0`，但 `catOrder["politics"]` 返回 map 的零值 `0`。因此 politics 和 economy 排在同一优先级，用户筛选意图未在排序中体现。

**实际上**: 因为 `buildSources` 不做分类过滤（分类过滤只在 step 6 由 `StrictFocus` 控制），且 `StrictFocus` 默认 false，所以非 strict 模式下的 `--cat` 确实只是"建议"语义。当前行为可视为设计选择。

**修复建议**: 若期望 `--cat` 在非 strict 下也有排序效果，将未指定分类映射到 `len(catOrder) + 100`。

### LOW-2: `internal/article/article.go:97` — looksPaywalled 的 `"access denied"` 标记可能误触发于 CDN/WAF 错误页

```go
"access denied", ...
```

某些 CDN/WAF（Cloudflare、Akamai）返回的 403 页面也包含 "access denied" 文本。如果此类页面同时包含 "subscribe" 文本（如 newsletter CTA），可能触发 2 个标记从而误判为付费墙。

**实际上**: the `looksPaywalled` 只在 readability 失败 AND fallback 提取 <100 字符时调用，此时 HTML 大概率有价值。且需要 2 个标记同时命中，误判概率很低。

**修复建议**: 可接受当前风险，或增加更精确的检查（如只在前 N KB 检查，或加入反向标记 "not a paywall" indicators）。

### LOW-3: `internal/classify/classify.go:8-23` — init() 中重复调用 `sets[lang] = ks` 但 `ks` 是值拷贝

```go
for lang, ks := range sets {
    ks.politics = normList(ks.politics)
    // ...
    sets[lang] = ks  // ← 写入修改后的值拷贝
}
```

逻辑正确（范围迭代的 `ks` 是值拷贝，必须写回），但有更清晰的写法——直接索引修改。当前写法虽对但容易让读者误以为 ks 是引用。无功能 bug。

**修复建议**: 可改为 `sets[lang].politics = normList(sets[lang].politics)` 直接索引修改（但需要临时变量持有 map 索引结果）。

---

## 并发安全性确认

| 组件 | 状态 |
|------|------|
| `classify.init()` 修改 `sets` 全局 map | ✅ 安全（init 单 goroutine 在 main 之前完成，之后 sets 只读） |
| `Classify()` 读取 `sets` | ✅ 只读，无竞态 |
| `tui.saveCmd` 闭包捕获 `m Model`（值） | ✅ 安全（值拷贝，共享的 `*report.Report` 在 `Run()` 中每次返回新对象） |
| `report.FetchFulltext` goroutine 写 `it.Body` | ✅ 安全（每个 goroutine 写独立 Item，无共享写） |
| `runSources` goroutine 发送到 channel | ✅ 契约明确（每个 goroutine 恰好 send 一次，接收循环按 len(targets) 计数） |

---