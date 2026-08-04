# Graph Report - news-report  (2026-08-03)

## Corpus Check
- 57 files · ~38,135 words
- Verdict: corpus is large enough that graph structure adds value.

## Summary
- 562 nodes · 1047 edges · 31 communities (24 shown, 7 thin omitted)
- Extraction: 88% EXTRACTED · 12% INFERRED · 0% AMBIGUOUS · INFERRED: 128 edges (avg confidence: 0.8)
- Token cost: 0 input · 0 output

## Graph Freshness
- Built from commit: `909e25b2`
- Run `git rev-parse HEAD` and compare to check if the graph is stale.
- Run `graphify update .` after code changes (no API cost).

## Community Hubs (Navigation)
- Fetcher
- main.go
- Parse
- Run
- Extract
- Default
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
- Model
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

## God Nodes (most connected - your core abstractions)
1. `Model` - 31 edges
2. `Fetcher` - 27 edges
3. `Parse()` - 21 edges
4. `Run()` - 20 edges
5. `Config` - 17 edges
6. `sampleModel()` - 16 edges
7. `Extract()` - 15 edges
8. `Report` - 14 edges
9. `Default()` - 13 edges
10. `Load()` - 13 edges

## Surprising Connections (you probably didn't know these)
- `runInit()` --calls--> `Default()`  [INFERRED]
  main.go → internal/config/config.go
- `feedCount()` --calls--> `Parse()`  [INFERRED]
  main.go → internal/feed/feed.go
- `runFind()` --calls--> `Search()`  [INFERRED]
  main.go → internal/gnews/gnews.go
- `runFind()` --calls--> `ExcludeOriginal()`  [INFERRED]
  main.go → internal/gnews/gnews.go
- `runReport()` --calls--> `Terminal()`  [INFERRED]
  main.go → internal/output/output.go

## Import Cycles
- None detected.

## Communities (31 total, 7 thin omitted)

### Community 0 - "Fetcher"
Cohesion: 0.09
Nodes (31): Client, Fetcher, Options, robotsRule, StatusError, globToRegexp(), Context, Duration (+23 more)

### Community 1 - "main.go"
Cohesion: 0.16
Nodes (23): Config, SourceOverride, expandPath(), Duration, Load(), applyFlags(), fatal(), fatalMsg() (+15 more)

### Community 2 - "Parse"
Cohesion: 0.15
Nodes (28): atomFeed, atomItem, atomLink, Item, rdfFeed, rdfItem, rssChannel, rssFeed (+20 more)

### Community 3 - "Run"
Cohesion: 0.12
Nodes (38): buildSources(), collectSource(), Context, Duration, Time, Run(), sourceWeight(), boolPtr() (+30 more)

### Community 4 - "Extract"
Cohesion: 0.14
Nodes (21): Article, errString, statusError, clean(), errHTTP(), Extract(), fallbackExtract(), fallbackTitle() (+13 more)

### Community 5 - "Default"
Cohesion: 0.13
Nodes (31): Default(), boolPtr(), T, TestDefaultConfig(), TestLoadAndOverride(), TestLoadMissingFileReturnsDefaults(), TestSaveRoundTrip(), TestValidateErrors() (+23 more)

### Community 6 - "Classify"
Cohesion: 0.24
Nodes (15): keywordSet, Result, Classify(), contains(), init(), normalize(), normList(), T (+7 more)

### Community 7 - "HTTP Feed 响应缓存层架构设计"
Cohesion: 0.04
Nodes (48): 10.1 单元测试 (`cache_test.go`), 10.2 集成测试, 10. 测试策略, 11. 文件清单, 12. 实施步骤, 1.1 当前问题, 1.2 设计目标, 1. 概述 (+40 more)

### Community 8 - "news-report · 欧美权威媒体新闻聚合器"
Cohesion: 0.05
Nodes (37): DEVELOPMENT.md — news-report 开发文档, v0.1.0 (2026-08-02), v0.2.0 (2026-08-03), 包职责, 变更日志, 已知限制与决策记录, 数据流, 架构 (+29 more)

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

### Community 16 - "Model"
Cohesion: 0.11
Nodes (25): Builder, Category, catColorIcon(), deleteWordBackwardRunes(), fetchReaderCmd(), fetchReportCmd(), Cmd, Context (+17 more)

### Community 17 - "Search"
Cohesion: 0.22
Nodes (15): Result, ExcludeOriginal(), Context, Time, langParams(), normalizeDomain(), Search(), splitTitleSource() (+7 more)

### Community 18 - "tui_test.go"
Cohesion: 0.25
Nodes (21): asModel(), Cmd, T, sampleModel(), TestBuildTabs(), TestCatCount(), TestFilter(), TestFilterCtrlW() (+13 more)

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
Nodes (14): Cache, Entry, Duration, hexByte(), New(), sha256Sum(), T, TestCacheClear() (+6 more)

### Community 30 - "s2t.go"
Cohesion: 0.31
Nodes (7): Normalize(), T, TestNormalize(), TestToSimplified(), TestToTraditional(), ToSimplified(), ToTraditional()

## Knowledge Gaps
- **121 isolated node(s):** `news-report`, `keywordSet`, `readerMsg`, `saveMsg`, `Acceptance Contract` (+116 more)
  These have ≤1 connection - possible missing edges or undocumented components.
- **7 thin communities (<3 nodes) omitted from report** — run `graphify query` to explore isolated nodes.

## Suggested Questions
_Questions this graph is uniquely positioned to answer:_

- **Why does `Fetcher` connect `Fetcher` to `Model`, `Search`, `Run`, `Extract`?**
  _High betweenness centrality (0.173) - this node is a cross-community bridge._
- **Why does `Run()` connect `Run` to `Fetcher`, `main.go`, `Classify`, `Score`, `IsDuplicate`, `Model`?**
  _High betweenness centrality (0.116) - this node is a cross-community bridge._
- **Why does `Parse()` connect `Parse` to `Fetcher`, `main.go`, `Run`, `Extract`, `Extract`, `Search`?**
  _High betweenness centrality (0.107) - this node is a cross-community bridge._
- **Are the 15 inferred relationships involving `Parse()` (e.g. with `.bytesWithUA()` and `mustURL()`) actually correct?**
  _`Parse()` has 15 INFERRED edges - model-reasoned connections that need verification._
- **Are the 8 inferred relationships involving `Run()` (e.g. with `Classify()` and `IsDuplicate()`) actually correct?**
  _`Run()` has 8 INFERRED edges - model-reasoned connections that need verification._
- **What connects `news-report`, `keywordSet`, `readerMsg` to the rest of the system?**
  _121 weakly-connected nodes found - possible documentation gaps or missing edges._
- **Should `Fetcher` be split into smaller, more focused modules?**
  _Cohesion score 0.09371980676328502 - nodes in this community are weakly interconnected._