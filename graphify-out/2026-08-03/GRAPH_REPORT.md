# Graph Report - news-report  (2026-08-03)

## Corpus Check
- 31 files · ~18,850 words
- Verdict: corpus is large enough that graph structure adds value.

## Summary
- 290 nodes · 582 edges · 16 communities (14 shown, 2 thin omitted)
- Extraction: 85% EXTRACTED · 15% INFERRED · 0% AMBIGUOUS · INFERRED: 90 edges (avg confidence: 0.8)
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

## God Nodes (most connected - your core abstractions)
1. `Run()` - 21 edges
2. `Fetcher` - 20 edges
3. `Parse()` - 18 edges
4. `Extract()` - 14 edges
5. `Classify()` - 13 edges
6. `Config` - 13 edges
7. `New()` - 12 edges
8. `Report` - 12 edges
9. `Defaults()` - 12 edges
10. `Load()` - 10 edges

## Surprising Connections (you probably didn't know these)
- `feedCount()` --calls--> `Parse()`  [INFERRED]
  main.go → internal/feed/feed.go
- `runReport()` --calls--> `Terminal()`  [INFERRED]
  main.go → internal/output/output.go
- `runReport()` --calls--> `Markdown()`  [INFERRED]
  main.go → internal/output/output.go
- `runReport()` --calls--> `Run()`  [INFERRED]
  main.go → internal/report/report.go
- `runSources()` --calls--> `Defaults()`  [INFERRED]
  main.go → internal/sources/sources.go

## Import Cycles
- None detected.

## Communities (16 total, 2 thin omitted)

### Community 0 - "Fetcher"
Cohesion: 0.09
Nodes (31): Client, Fetcher, Options, robotsRule, StatusError, globToRegexp(), Context, Duration (+23 more)

### Community 1 - "main.go"
Cohesion: 0.13
Nodes (27): Config, SourceOverride, Default(), expandPath(), Duration, Load(), boolPtr(), T (+19 more)

### Community 2 - "Parse"
Cohesion: 0.16
Nodes (25): atomFeed, atomItem, atomLink, Item, rdfFeed, rdfItem, rssChannel, rssFeed (+17 more)

### Community 3 - "Run"
Cohesion: 0.20
Nodes (21): collectSource(), Context, Duration, Time, Run(), sourceWeight(), boolPtr(), cfgWithOnly() (+13 more)

### Community 4 - "Extract"
Cohesion: 0.15
Nodes (18): Article, errString, statusError, clean(), errHTTP(), Extract(), fallbackExtract(), fallbackTitle() (+10 more)

### Community 5 - "Terminal"
Cohesion: 0.21
Nodes (18): bold(), cResetIf(), dim(), JSON(), Markdown(), Terminal(), firstLine(), T (+10 more)

### Community 6 - "Classify"
Cohesion: 0.22
Nodes (17): Category, keywordSet, Result, Classify(), contains(), init(), normalize(), normList() (+9 more)

### Community 7 - "Defaults"
Cohesion: 0.26
Nodes (16): buildSources(), Defaults(), GoogleNewsFeeds(), itoa(), src(), T, TestBrookingsScrapeFallback(), TestDefaultsCoverage() (+8 more)

### Community 8 - "news-report · 欧美权威媒体新闻聚合器"
Cohesion: 0.11
Nodes (16): Compliance, Configuration, Highlights, news-report — Western News Aggregator, Quick Start, Tests, news-report · 欧美权威媒体新闻聚合器, 合规说明 (+8 more)

### Community 9 - "Store"
Cohesion: 0.20
Nodes (11): Mutex, Hash(), New(), T, TestHashStable(), TestNewCorruptFile(), TestStoreAddHas(), TestStorePersist() (+3 more)

### Community 10 - "Score"
Cohesion: 0.32
Nodes (10): AgeLabel(), Duration, itoa(), Score(), T, TestAgeLabel(), TestScoreCategorizedBonus(), TestScoreFreshness() (+2 more)

### Community 11 - "Extract"
Cohesion: 0.30
Nodes (10): collapse(), Extract(), resolveURL(), T, TestExtractEmpty(), TestExtractHeuristic(), TestExtractLinkPattern(), TestExtractWithSelector() (+2 more)

### Community 12 - "IsDuplicate"
Cohesion: 0.36
Nodes (8): IsDuplicate(), Jaccard(), NormalizeTitle(), T, TestIsDuplicate(), TestJaccard(), TestNormalizeTitle(), tokenize()

### Community 13 - "Code Review: news-report v0.1.0"
Cohesion: 0.29
Nodes (6): Code Review: news-report v0.1.0, CRITICAL (3 issues), HIGH (5 issues), LOW (6 issues), MEDIUM (7 issues), Test Quality Assessment

## Knowledge Gaps
- **22 isolated node(s):** `news-report`, `keywordSet`, `Acceptance Contract`, `CRITICAL (3 issues)`, `HIGH (5 issues)` (+17 more)
  These have ≤1 connection - possible missing edges or undocumented components.
- **2 thin communities (<3 nodes) omitted from report** — run `graphify query` to explore isolated nodes.

## Suggested Questions
_Questions this graph is uniquely positioned to answer:_

- **Why does `Run()` connect `Run` to `Fetcher`, `main.go`, `Classify`, `Defaults`, `Score`, `IsDuplicate`?**
  _High betweenness centrality (0.348) - this node is a cross-community bridge._
- **Why does `Fetcher` connect `Fetcher` to `Run`, `Extract`?**
  _High betweenness centrality (0.230) - this node is a cross-community bridge._
- **Why does `Parse()` connect `Parse` to `Fetcher`, `main.go`, `Run`, `Extract`, `Extract`?**
  _High betweenness centrality (0.228) - this node is a cross-community bridge._
- **Are the 9 inferred relationships involving `Run()` (e.g. with `Classify()` and `IsDuplicate()`) actually correct?**
  _`Run()` has 9 INFERRED edges - model-reasoned connections that need verification._
- **Are the 12 inferred relationships involving `Parse()` (e.g. with `.bytesWithUA()` and `mustURL()`) actually correct?**
  _`Parse()` has 12 INFERRED edges - model-reasoned connections that need verification._
- **Are the 5 inferred relationships involving `Extract()` (e.g. with `LangHeader()` and `TestExtractEmptyPage()`) actually correct?**
  _`Extract()` has 5 INFERRED edges - model-reasoned connections that need verification._
- **Are the 8 inferred relationships involving `Classify()` (e.g. with `TestClassifyAccentNormalization()` and `TestClassifyConfidence()`) actually correct?**
  _`Classify()` has 8 INFERRED edges - model-reasoned connections that need verification._