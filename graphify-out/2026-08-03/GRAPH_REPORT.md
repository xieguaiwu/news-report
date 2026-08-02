# Graph Report - news-report  (2026-08-03)

## Corpus Check
- 43 files · ~27,318 words
- Verdict: corpus is large enough that graph structure adds value.

## Summary
- 407 nodes · 818 edges · 23 communities (19 shown, 4 thin omitted)
- Extraction: 88% EXTRACTED · 12% INFERRED · 0% AMBIGUOUS · INFERRED: 102 edges (avg confidence: 0.8)
- Token cost: 0 input · 0 output

## Graph Freshness
- Built from commit: `382120c5`
- Run `git rev-parse HEAD` and compare to check if the graph is stale.
- Run `graphify update .` after code changes (no API cost).

## Community Hubs (Navigation)
- Fetcher
- main.go
- Parse
- Run
- Extract
- Terminal
- Classify
- Defaults
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
- sampleModel
- Google News RSS 搜索接口调研报告（find 转载功能）
- 验证结果
- Task for librarian
- Task for librarian

## God Nodes (most connected - your core abstractions)
1. `Model` - 28 edges
2. `Fetcher` - 27 edges
3. `Parse()` - 21 edges
4. `Run()` - 20 edges
5. `Config` - 17 edges
6. `Extract()` - 15 edges
7. `Report` - 14 edges
8. `Classify()` - 12 edges
9. `Load()` - 12 edges
10. `New()` - 12 edges

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

## Communities (23 total, 4 thin omitted)

### Community 0 - "Fetcher"
Cohesion: 0.09
Nodes (31): Client, Fetcher, Options, robotsRule, StatusError, globToRegexp(), Context, Duration (+23 more)

### Community 1 - "main.go"
Cohesion: 0.18
Nodes (26): Default(), Load(), boolPtr(), T, TestDefaultConfig(), TestLoadAndOverride(), TestLoadMissingFileReturnsDefaults(), TestSaveRoundTrip() (+18 more)

### Community 2 - "Parse"
Cohesion: 0.15
Nodes (28): atomFeed, atomItem, atomLink, Item, rdfFeed, rdfItem, rssChannel, rssFeed (+20 more)

### Community 3 - "Run"
Cohesion: 0.20
Nodes (21): collectSource(), Context, Duration, Time, Run(), sourceWeight(), boolPtr(), cfgWithOnly() (+13 more)

### Community 4 - "Extract"
Cohesion: 0.15
Nodes (19): Article, errString, statusError, clean(), errHTTP(), Extract(), fallbackExtract(), fallbackTitle() (+11 more)

### Community 5 - "Terminal"
Cohesion: 0.21
Nodes (18): bold(), cResetIf(), dim(), JSON(), Markdown(), Terminal(), firstLine(), T (+10 more)

### Community 6 - "Classify"
Cohesion: 0.24
Nodes (15): keywordSet, Result, Classify(), contains(), init(), normalize(), normList(), T (+7 more)

### Community 7 - "Defaults"
Cohesion: 0.26
Nodes (16): buildSources(), Defaults(), GoogleNewsFeeds(), itoa(), src(), T, TestBrookingsScrapeFallback(), TestDefaultsCoverage() (+8 more)

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
Cohesion: 0.09
Nodes (26): Builder, Category, Config, SourceOverride, expandPath(), Duration, catColorIcon(), fetchReaderCmd() (+18 more)

### Community 17 - "Search"
Cohesion: 0.22
Nodes (15): Result, ExcludeOriginal(), Context, Time, langParams(), normalizeDomain(), Search(), splitTitleSource() (+7 more)

### Community 18 - "sampleModel"
Cohesion: 0.38
Nodes (12): asModel(), Cmd, T, sampleModel(), TestBuildTabs(), TestCatCount(), TestFilter(), TestListNavigation() (+4 more)

### Community 19 - "Google News RSS 搜索接口调研报告（find 转载功能）"
Cohesion: 0.29
Nodes (6): 1. URL 格式与引号短语 ✓ 实测可用, 2. RSS 结构（channel/item 实测）, 3. 跳转机制实测（curl -sI / -L）, 4. 限流实测, 5. 建议实现方式（find 转载）, Google News RSS 搜索接口调研报告（find 转载功能）

### Community 20 - "验证结果"
Cohesion: 0.40
Nodes (4): 台湾媒体（zh）, 结论, 美国政治（uspolitics）, 验证结果

## Knowledge Gaps
- **51 isolated node(s):** `news-report`, `keywordSet`, `readerMsg`, `saveMsg`, `Acceptance Contract` (+46 more)
  These have ≤1 connection - possible missing edges or undocumented components.
- **4 thin communities (<3 nodes) omitted from report** — run `graphify query` to explore isolated nodes.

## Suggested Questions
_Questions this graph is uniquely positioned to answer:_

- **Why does `Fetcher` connect `Fetcher` to `Model`, `Search`, `Run`, `Extract`?**
  _High betweenness centrality (0.240) - this node is a cross-community bridge._
- **Why does `Run()` connect `Run` to `Fetcher`, `Classify`, `Defaults`, `Score`, `IsDuplicate`, `Model`?**
  _High betweenness centrality (0.186) - this node is a cross-community bridge._
- **Why does `Parse()` connect `Parse` to `Fetcher`, `main.go`, `Run`, `Extract`, `Extract`, `Search`?**
  _High betweenness centrality (0.167) - this node is a cross-community bridge._
- **Are the 15 inferred relationships involving `Parse()` (e.g. with `.bytesWithUA()` and `mustURL()`) actually correct?**
  _`Parse()` has 15 INFERRED edges - model-reasoned connections that need verification._
- **Are the 8 inferred relationships involving `Run()` (e.g. with `Classify()` and `IsDuplicate()`) actually correct?**
  _`Run()` has 8 INFERRED edges - model-reasoned connections that need verification._
- **What connects `news-report`, `keywordSet`, `readerMsg` to the rest of the system?**
  _51 weakly-connected nodes found - possible documentation gaps or missing edges._
- **Should `Fetcher` be split into smaller, more focused modules?**
  _Cohesion score 0.09371980676328502 - nodes in this community are weakly interconnected._