All 18 packages pass with `-race`. Here is my complete review report.

---

# LLM 集成代码审查报告

## 审查范围

新增文件：`internal/llm/`（client.go、cache.go、translate.go、analyze.go、llm_test.go）、`internal/s2t/`（s2t.go、s2t_test.go）
修改文件：`internal/config/config.go`、`internal/tui/tui.go`、`main.go`、`internal/article/article.go`、`internal/report/report.go` 及对应测试

## 按审查重点的结论

### 1. API key 安全 ✅
- 无硬编码 key。默认配置 `APIKey: ""`，运行时优雅降级。
- 支持 `{env:VAR}` 语法（`config.resolveEnvRefs`），正则 `\{env:([A-Za-z_][A-Za-z0-9_]*)\}` 提取环境变量。未设置的环境变量替换为空字符串（有测试覆盖）。
- key 仅用于 `Authorization: Bearer` 头，不写入日志/错误消息。`--no-llm` flag 可运行时禁用。

### 2. 错误处理 / 优雅降级 ✅
- `ErrNoKey` 哨兵错误设计良好。`Chat`/`Translate`/`Summarize`/`Brief` 均先检查 `Available()`。
- TUI 中所有 LLM 入口（`t`/`d`/`x` 键）先检查 `m.llm == nil || !m.llm.Available()`，显示可操作提示而非崩溃。
- 异步命令通过 `llmTranslateMsg`/`llmSummaryMsg`/`llmBriefMsg` 回传错误，`Update` 中设置 `m.status`/`popup.err`，不影响现有功能。

### 3. 缓存策略 ⚠️ → 已修复
- **HIGH bug（已修复）**：`llm.Cache.Purge` 将 `total`（字节）与 `c.maxMB`（MB 数值）直接比较。默认 `maxMB=20` 意味着阈值仅 20 字节——每次 TUI 启动调用 `Purge()` 会清空几乎全部 7 天 TTL 缓存。修复：`limit := c.maxMB * 1024 * 1024`。
- 缓存 key 设计合理：`sha256(url|lang|instruction)`，翻译用 URL、摘要/解读用 URL+指令区分。
- TTL 7 天合理。原子写入（`.tmp` + `Rename`）正确。

### 4. 并发安全 ✅
- `llm.Cache` 用 `sync.RWMutex` 保护所有磁盘操作。
- `cache.Cache`（feed 缓存）同样有 mutex，被 goroutine（`llmSummaryCmd`/`llmBriefCmd`/`fetchReaderCmd`）和主模型并发访问，安全。
- `m.articleCache`（内存 map）仅在 `Update` 中访问（bubbletea 单线程），无竞态。`-race` 全通过。

### 5. TUI 键盘事件 ✅
- 新增键：列表 `x`（摘要）、`c`（复制）；阅读器 `t`（翻译）、`d`（解读）、`o`（浏览器）、`c`（复制）。无与现有键冲突。
- 弹窗拦截 `esc`/`q`/`j`/`k`/`pgup`/`pgdown`/`space`/`home`/`end`，其余键吞掉（返回 nil），逻辑清晰。
- `nextFilterMatch` 原实现循环 `len(items)` 次后回到原位（实际是 no-op bug），新实现改为单步推进——这是 bug 修复。

### 6. 代码质量 ⚠️ → 已修复
- **已修复**：`contains`/`searchString` 手写重复 `strings.Contains` → 改用 stdlib。
- **已修复**：`llmTranslateCmd` 死代码（含 `_ = key` 无意义逻辑）已删除。
- **已修复**：`copyToClipboard`/`openBrowser` 中 `devnull` 文件描述符泄漏 → 加 `defer devnull.Close()`。
- **已修复**：弹窗底边框 `└─┘` 宽度 `pw` 与内容行 `pw+2` 不对齐 → 改为 `pw`。
- **已修复**：弹窗错误行/帮助行用 `fmt.Sprintf("%-*s", styleErr.Render(...))` 导致 ANSI 转义字节计入宽度，右边框错位 → 改用 `lipgloss.Style.Width()` ANSI 感知填充。
- 残留建议（未修复，非阻塞）：`tui.go` 中 `parseNumberedLinesCompat`/`parseBriefResult`/`setBriefSection` 与 `llm` 包内同名函数逻辑重复，应从 `llm` 包导出复用以防漂移；`func max` 遮蔽 Go 1.25 内建（可删除）。

### 7. 测试覆盖 ⚠️ → 已修复
- **已修复**：`TestTranslateEmptyText` 无任何断言（`if err == ErrNoKey {}` 空块 + `_ = result`）→ 补充 `err == nil` 和 `result == ""` 断言。
- **已修复**：`TestCachePurge` 用 200KB 文件 + maxMB=1 验证淘汰，实际验证的是 bug 行为（200KB 字节 > 1）→ 改为 2MB 文件真正触发淘汰，新增 `TestCachePurgeUnderLimit` 验证未超限不淘汰。
- 覆盖良好：mock HTTP server 覆盖成功/5xx 重试/4xx 不重试；缓存命中/过期/key 确定性；分块/摘要/解读解析；简繁搜索匹配；行级滚动；缓存命中免抓取。

### 额外发现（非 LLM 核心，低优先级）
- `s2t.go` 映射表有 ~29 个恒等映射（如 `'刚': '刚'`、`'当': '当'`），简→繁方向转换错误（应为 `'剛'`/`'當'`）。不影响搜索（搜索用 `Normalize`=繁→简，`t2s` 反向表正确），但 `ToTraditional` 对这些字无效。

## 修复清单

| 严重度 | 文件 | 问题 | 修复 |
|--------|------|------|------|
| HIGH | `llm/cache.go:124` | Purge 字节与 MB 比较，缓存被清空 | `limit := c.maxMB*1024*1024` |
| MEDIUM | `tui/tui.go:1420` | 弹窗底边框比内容窄 2 字符 | `pw-2` → `pw` |
| MEDIUM | `tui/tui.go:1387,1419` | ANSI 转义字节计入 `%-*s` 宽度，边框错位 | 改用 `lipgloss.Width()` |
| LOW | `tui/tui.go:1303,1322` | `devnull` fd 泄漏 | `defer devnull.Close()` |
| LOW | `llm/client.go:198` | 手写 `contains`/`searchString` | `strings.Contains` |
| LOW | `tui/tui.go:209` | `llmTranslateCmd` 死代码 | 删除 |
| LOW | `llm/llm_test.go` | `TestTranslateEmptyText` 无断言 | 补断言 |
| LOW | `llm/llm_test.go` | `TestCachePurge` 验证 bug 行为 | 改用 2MB + 新增 under-limit 测试 |