# Graph Report - news-report  (2026-08-14)

## Corpus Check
- 87 files · ~63,055 words
- Verdict: corpus is large enough that graph structure adds value.

## Summary
- 842 nodes · 1525 edges · 50 communities (37 shown, 13 thin omitted)
- Extraction: 87% EXTRACTED · 13% INFERRED · 0% AMBIGUOUS · INFERRED: 195 edges (avg confidence: 0.8)
- Token cost: 0 input · 0 output

## Graph Freshness
- Built from commit: `53171349`
- Run `git rev-parse HEAD` and compare to check if the graph is stale.
- Run `graphify update .` after code changes (no API cost).

## Community Hubs (Navigation)
- Fetcher
- main.go
- Parse
- Run
- Extract
- output_test.go
- Classify
- HTTP Feed 响应缓存层架构设计
- news-report · 欧美权威媒体新闻聚合器
- Store
- Score
- Extract
- IsDuplicate
- Code Review: news-report v0.1.0
- Task for momus
- news-report
- tui.go
- Search
- tui_test.go
- Google News RSS 搜索接口调研报告（find 转载功能）
- 验证结果
- Task for librarian
- Task for librarian
- Momus Code Review — news-report v0.2.0
- Task for momus
- 已实现 ≡ 符合项
- Cache
- Task for momus
- Task for prometheus
- s2t.go
- llm_test.go
- news-report LLM 集成方案 — 详细实现计划
- news-report LLM 集成方案 — 详细实现计划
- Client
- 按审查重点的结论
- Task for hephaestus
- Task for hephaestus
- f8df5ac5_artistry_0_output.md
- 修改摘要
- TestTimestamp
- Task for momus
- Task for prometheus
- Task for hephaestus
- Task for hephaestus
- Task for artistry

## God Nodes (most connected - your core abstractions)
1. `Model` - 34 edges
2. `Fetcher` - 29 edges
3. `Parse()` - 21 edges
4. `Run()` - 21 edges
5. `Config` - 19 edges
6. `sampleModel()` - 19 edges
7. `Classify()` - 16 edges
8. `Default()` - 16 edges
9. `Extract()` - 15 edges
10. `Load()` - 15 edges

## Surprising Connections (you probably didn't know these)
- `feedCount()` --calls--> `Parse()`  [INFERRED]
  main.go → internal/feed/feed.go
- `runFind()` --calls--> `Search()`  [INFERRED]
  main.go → internal/gnews/gnews.go
- `runFind()` --calls--> `ExcludeOriginal()`  [INFERRED]
  main.go → internal/gnews/gnews.go
- `runReport()` --calls--> `Terminal()`  [INFERRED]
  main.go → internal/output/output.go
- `runReport()` --calls--> `Markdown()`  [INFERRED]
  main.go → internal/output/output.go

## Import Cycles
- None detected.

## Communities (50 total, 13 thin omitted)

### Community 0 - "Fetcher"
Cohesion: 0.09
Nodes (31): Fetcher, Options, robotsRule, StatusError, globToRegexp(), Client, Context, Duration (+23 more)

### Community 1 - "main.go"
Cohesion: 0.13
Nodes (37): Default(), Load(), boolPtr(), T, TestDefaultConfig(), TestDefaultLLMConfig(), TestLLMMaxCharsValidation(), TestLLMParamInConfigFile() (+29 more)

### Community 2 - "Parse"
Cohesion: 0.15
Nodes (28): atomFeed, atomItem, atomLink, Item, rdfFeed, rdfItem, rssChannel, rssFeed (+20 more)

### Community 3 - "Run"
Cohesion: 0.08
Nodes (46): Config, LLMConfig, SourceOverride, expandPath(), Duration, resolveEnv(), buildSources(), collectSource() (+38 more)

### Community 4 - "Extract"
Cohesion: 0.14
Nodes (21): Article, errString, statusError, clean(), errHTTP(), Extract(), fallbackExtract(), fallbackTitle() (+13 more)

### Community 5 - "output_test.go"
Cohesion: 0.19
Nodes (23): bold(), cResetIf(), dim(), JSON(), Markdown(), Terminal(), firstLine(), T (+15 more)

### Community 6 - "Classify"
Cohesion: 0.21
Nodes (19): keywordSet, Result, Classify(), contains(), init(), normalize(), normList(), T (+11 more)

### Community 7 - "HTTP Feed 响应缓存层架构设计"
Cohesion: 0.04
Nodes (48): 10.1 单元测试 (`cache_test.go`), 10.2 集成测试, 10. 测试策略, 11. 文件清单, 12. 实施步骤, 1.1 当前问题, 1.2 设计目标, 1. 概述 (+40 more)

### Community 8 - "news-report · 欧美权威媒体新闻聚合器"
Cohesion: 0.05
Nodes (40): DEVELOPMENT.md — news-report 开发文档, v0.1.0 (2026-08-02), v0.2.0 (2026-08-03), v0.5.0 (2026-08-14), v0.5.1 (2026-08-14), 包职责, 变更日志, 已知限制与决策记录 (+32 more)

### Community 9 - "Store"
Cohesion: 0.20
Nodes (11): Mutex, Hash(), New(), T, TestHashStable(), TestNewCorruptFile(), TestStoreAddHas(), TestStorePersist() (+3 more)

### Community 10 - "Score"
Cohesion: 0.32
Nodes (10): AgeLabel(), Duration, itoa(), Score(), T, TestAgeLabel(), TestScoreCategorizedBonus(), TestScoreFreshness() (+2 more)

### Community 11 - "Extract"
Cohesion: 0.30
Nodes (10): collapse(), Extract(), Item, resolveURL(), T, TestExtractEmpty(), TestExtractHeuristic(), TestExtractLinkPattern() (+2 more)

### Community 12 - "IsDuplicate"
Cohesion: 0.36
Nodes (8): IsDuplicate(), Jaccard(), NormalizeTitle(), T, TestIsDuplicate(), TestJaccard(), TestNormalizeTitle(), tokenize()

### Community 13 - "Code Review: news-report v0.1.0"
Cohesion: 0.29
Nodes (6): Code Review: news-report v0.1.0, CRITICAL (3 issues), HIGH (5 issues), LOW (6 issues), MEDIUM (7 issues), Test Quality Assessment

### Community 16 - "tui.go"
Cohesion: 0.09
Nodes (39): Builder, Category, catColorIcon(), copyToClipboard(), deleteWordBackwardRunes(), fetchReaderCmd(), fetchReportCmd(), Cache (+31 more)

### Community 17 - "Search"
Cohesion: 0.22
Nodes (15): Result, ExcludeOriginal(), Context, Time, langParams(), normalizeDomain(), Search(), splitTitleSource() (+7 more)

### Community 18 - "tui_test.go"
Cohesion: 0.13
Nodes (42): cellWidth(), cutCells(), overlayPopup(), padCells(), popupContentH(), runeCells(), sliceCells(), splitWrapped() (+34 more)

### Community 19 - "Google News RSS 搜索接口调研报告（find 转载功能）"
Cohesion: 0.29
Nodes (6): 1. URL 格式与引号短语 ✓ 实测可用, 2. RSS 结构（channel/item 实测）, 3. 跳转机制实测（curl -sI / -L）, 4. 限流实测, 5. 建议实现方式（find 转载）, Google News RSS 搜索接口调研报告（find 转载功能）

### Community 20 - "验证结果"
Cohesion: 0.40
Nodes (4): 台湾媒体（zh）, 结论, 美国政治（uspolitics）, 验证结果

### Community 23 - "Momus Code Review — news-report v0.2.0"
Cohesion: 0.12
Nodes (16): CRITICAL, CRITICAL-1: `internal/tui/tui.go:268-278` — readerView 按字节切片导致 UTF-8 多字节字符被截断, HIGH, HIGH-1: `internal/classify/classify.go:79-87,269-281` — "federal" 单 token (uspolitics ×2) 与 "Federal Reserve" 财经新闻误分类, HIGH-2: `internal/feed/feed.go:134` — CST 时区歧义与注释矛盾, LOW, LOW-1: `internal/report/report.go:171-181` — catOrder 零值语义导致未指定分类与首个指定分类排序同级, LOW-2: `internal/article/article.go:97` — looksPaywalled 的 `"access denied"` 标记可能误触发于 CDN/WAF 错误页 (+8 more)

### Community 25 - "已实现 ≡ 符合项"
Cohesion: 0.08
Nodes (24): §1.1 光标移动键位 — 过滤输入框, §1.2 文本编辑键位 — 过滤输入框, §2.1 翻页, §2.1 翻页, §2.2 选项导航, §2.2 选项导航与选择, §2.3 搜索, §2.3 搜索 (+16 more)

### Community 26 - "Cache"
Cohesion: 0.18
Nodes (14): Cache, Entry, Duration, RWMutex, hexByte(), New(), sha256Sum(), T (+6 more)

### Community 30 - "s2t.go"
Cohesion: 0.27
Nodes (8): Normalize(), T, TestNormalize(), TestToSimplified(), TestToTraditional(), ToSimplified(), ToTraditional(), matchFilter()

### Community 31 - "llm_test.go"
Cohesion: 0.06
Nodes (51): buildArticleMsg(), Context, Client, parseBriefResult(), parseNumberedLines(), CacheKey(), Duration, RWMutex (+43 more)

### Community 32 - "news-report LLM 集成方案 — 详细实现计划"
Cohesion: 0.05
Nodes (39): 2.1 文件结构（4 个文件 + 1 个测试）, 2.2 `client.go` 接口设计, 2.3 `cache.go` 缓存设计, 2.4 `translate.go` 翻译逻辑, 2.5 `analyze.go` 摘要与解读, 3.1 新增 LLMConfig 字段, 3.2 Default() 新增默认值, 3.3 Load() 中解析 `{env:VAR}` (+31 more)

### Community 33 - "news-report LLM 集成方案 — 详细实现计划"
Cohesion: 0.05
Nodes (37): 2.1 文件结构（4 个文件 + 1 个测试）, 2.2 `client.go` 接口设计, 2.3 `cache.go` 缓存设计, 2.4 `translate.go` 翻译逻辑, 2.5 `analyze.go` 摘要与解读, 3.1 新增 LLMConfig 字段, 3.2 Default() 新增默认值, 3.3 Load() 中解析 `{env:VAR}` (+29 more)

### Community 34 - "Client"
Cohesion: 0.19
Nodes (10): Cache, Context, Duration, Client, isClientError(), chatError, chatMessage, chatRequest (+2 more)

### Community 35 - "按审查重点的结论"
Cohesion: 0.15
Nodes (12): 1. API key 安全 ✅, 2. 错误处理 / 优雅降级 ✅, 3. 缓存策略 ⚠️ → 已修复, 4. 并发安全 ✅, 5. TUI 键盘事件 ✅, 6. 代码质量 ⚠️ → 已修复, 7. 测试覆盖 ⚠️ → 已修复, LLM 集成代码审查报告 (+4 more)

### Community 36 - "Task for hephaestus"
Cohesion: 0.25
Nodes (7): Acceptance Contract, Task for hephaestus, 变更清单, 新增内容 A：§1 文本输入规范 — 新增 §1.6 CJK 多字节输入安全, 新增内容 B：§2 信息密集界面 — 新增 §2.7 阅读器/文本显示窗口, 新增内容 C：§2.3 搜索 — 扩展搜索小节, 新增内容 D：新 §6 外部工具集成

### Community 37 - "Task for hephaestus"
Cohesion: 0.29
Nodes (6): Acceptance Contract, Bug 1：终端缩放时弹窗不复原, Bug 2：帮助栏文本可能溢出, Bug 3：标题栏宽度与底边框不对齐, Task for hephaestus, 要求：

### Community 38 - "f8df5ac5_artistry_0_output.md"
Cohesion: 0.33
Nodes (5): 方案 1：`t` 一键全文翻译（译）, 方案 2：`x` 摘要 / `d` 解读（速览两键）, 方案 3：`dig` — 付费墙深挖, 方案 4：`digest` — 每日 AI 简报, 方案 5：`v` 多语视角 — 同一事件跨媒体对比

### Community 39 - "修改摘要"
Cohesion: 0.40
Nodes (4): Bug 1 — 终端缩放时弹窗不复原, Bug 2 — 帮助栏文本溢出保护, Bug 3 — 底边框宽度不对齐, 修改摘要

## Knowledge Gaps
- **227 isolated node(s):** `news-report`, `keywordSet`, `LLMConfig`, `CacheEntry`, `chatResponse` (+222 more)
  These have ≤1 connection - possible missing edges or undocumented components.
- **13 thin communities (<3 nodes) omitted from report** — run `graphify query` to explore isolated nodes.

## Suggested Questions
_Questions this graph is uniquely positioned to answer:_

- **Why does `Fetcher` connect `Fetcher` to `tui.go`, `Search`, `Run`, `Extract`?**
  _High betweenness centrality (0.160) - this node is a cross-community bridge._
- **Why does `CacheKey()` connect `llm_test.go` to `tui.go`?**
  _High betweenness centrality (0.081) - this node is a cross-community bridge._
- **Why does `Run()` connect `Run` to `Fetcher`, `Classify`, `Score`, `IsDuplicate`, `tui.go`?**
  _High betweenness centrality (0.076) - this node is a cross-community bridge._
- **Are the 15 inferred relationships involving `Parse()` (e.g. with `.bytesWithUA()` and `mustURL()`) actually correct?**
  _`Parse()` has 15 INFERRED edges - model-reasoned connections that need verification._
- **What connects `news-report`, `keywordSet`, `LLMConfig` to the rest of the system?**
  _227 weakly-connected nodes found - possible documentation gaps or missing edges._
- **Should `Fetcher` be split into smaller, more focused modules?**
  _Cohesion score 0.09371980676328502 - nodes in this community are weakly interconnected._
- **Should `main.go` be split into smaller, more focused modules?**
  _Cohesion score 0.13090418353576247 - nodes in this community are weakly interconnected._