# DEVELOPMENT.md — news-report 开发文档

## 架构

```
┌──────────────────────────── main.go ────────────────────────────┐
│  子命令: report(默认) / ui / read / find / sources / init        │
└─────────────────────────────────────────────────────────────────┘
        │
        ▼
┌─ report.Run() 流水线 ───────────────────────────────────────────┐
│  1. buildSources: 配置 × 来源注册表（语言/分类/ID 过滤）          │
│  2. 并发抓取: worker pool（sem 限流，默认 12）                    │
│     collectSource: feeds 依次尝试 → 全败时 scrape 兜底            │
│  3. classify: 四语关键词分类（uspolitics/politics/economy/       │
│     industry/other，负面词过滤）                                 │
│  4. 新鲜度窗口过滤（cfg.Minutes）                                │
│  5. dedup: 规范化标题 + Jaccard（同语言 0.6 / 跨语言 0.7）        │
│  6. seen-store 过滤（~/.cache/news-report/seen.json）            │
│  7. rank: 权重 × exp(-age/halflife) + 相关性 + 分类加成          │
│  8. 数量限制（分类内 + 总量，0=不限制）                          │
│  9. FetchFulltext（--fulltext N，rune 截断）                     │
└─────────────────────────────────────────────────────────────────┘
```

### 包职责

| 包 | 职责 | 关键契约 |
|---|---|---|
| `internal/config` | 配置加载/校验（默认值 < 用户文件 < CLI） | yaml 只覆盖出现字段；languages ∈ {en,de,fr,zh}；categories ∈ {uspolitics,politics,economy,industry,edu-policy} |
| `internal/sources` | 内置来源注册表 | Source{ID,Name,Lang,Tier,Weight,Feeds,Scrape,Optional,Enabled,UserAgent}；`src()` 助手 + 特殊源字面量 |
| `internal/fetch` | HTTP 获取 | 手动设置 Accept-Encoding 会禁用 Go 自动解压（禁止）；`Bytes`/`BytesUA`/`BytesNoRobots`；robots 按 UA 分组、inflight channel 防并发重复抓取、24h 缓存；`StatusError` 类型化错误 |
| `internal/feed` | RSS/Atom/RDF 解析 | BOM 剥离；时区缩写偏移表优先于 Go 默认（Go 把未知缩写当 UTC）；`<source>` 元素（Google News）；无 link 时仅回退 http(s) ID |
| `internal/scrape` | HTML 兜底抓取 | 返回空切片（非 nil）；junkRe 过滤导航/话题页 |
| `internal/classify` | 四语五分类 | zh 走子串匹配（无空格语言）；uspolitics/edu-policy 强信号 ×2、multi ×4；init() 预规范化所有词表；折行/截断/弹窗排版全部按显示列（CJK=2 列） |
| `internal/dedup` | 去重 | NormalizeTitle 保留拉丁扩展字符 |
| `internal/rank` | 排序 | Score(weight, kw, categorized, age, halflife) |
| `internal/store` | 已读记录 | JSON 原子写（tmp+rename）；损坏文件重建 |
| `internal/article` | 全文提取 | go-readability → goquery 回退 → 付费墙检测（特征≥2 命中） |
| `internal/gnews` | Google News 搜索 | 引号短语；`<source url>` 域名；个人用途许可（跳过 robots）；gnewsBaseURL 可注入测试 |
| `internal/tui` | bubbletea 界面 | 模型纯函数可测；每分类 Tab；reader 滚动；弹窗叠加层按显示列排版（truncateCells/sliceCells/padCells，CJK 安全） |
| `internal/output` | 终端/Markdown/JSON | truncate 必须 rune-aware；NO_COLOR 支持 |

## 数据流

```
feed 条目 (Item{Title,URL,Published,Summary,SourceID,Lang,SourceName,SourceURL})
  → classify.Classify → (Category, Score, Confidence)
  → dedup.IsDuplicate（保留靠前来源）
  → rank.Score → 排序
  → report.Item{...Category, Score, AgeLabel, Body}
  → output（终端/Markdown/JSON）
```

## 测试

```bash
make test          # go test -race ./...（14 包）
make smoke         # 真实联网冒烟（6 源）
```

| 包 | 覆盖 |
|---|---|
| feed | RSS/Atom/RDF 解析、日期 20+ 布局、BOM、CDATA、source 元素、垃圾输入 |
| classify | 四语用例、uspolitics 强弱信号、繁简中文、负面过滤、置信度 |
| fetch | gzip、重定向、5xx 重试/4xx 不重试、robots 分组/禁用/通配符、超时 |
| report | 全流水线（httptest）、strict、seen-store、单源失败非致命 |
| dedup/rank/store | 边界（空、负、溢出防护） |
| output | 三格式渲染、rune 截断、无颜色模式 |
| gnews | 引号短语、source 提取、域名归一化、原站标记 |
| tui | Tab 构建/导航/过滤/阅读流/渲染/滚动 |
| sources | 注册表完整性（语言/层级/权重/ID 唯一）、UN UA、Brookings scrape |
| config | 默认值、覆盖合并、校验错误、round-trip |

## 已知限制与决策记录

- **zh 分类用子串匹配**：中文无空格分词，子串匹配 2+ 字关键词；误报率可接受，词表在 classify.go 可调
- **uspolitics 权重 ×2**：senate/congress/trump 等几乎不出现于他国语境；bill/legislation 等通用词留在 politics
- **Google News robots**：RSS 端点版权声明许可个人 feed reader 使用，find 固定跳过 robots（代码注释有依据）
- **EIA robots 自禁 /rss**：尊重 robots → 该源默认失败，标记 optional，--no-robots 可绕过
- **时区缩写**：CEST/JST 等常见缩写查表纠正偏移；未收录缩写（WEST 等）按 UTC
- **付费墙检测是启发式**：特征命中 ≥2 判定；误判时 read 仍输出原文（检测只在提取 <100 字符时触发）
- **find 的链接是 Google 跳转链接**：普通 HTTP 跟随停在 SPA，需浏览器/JS 跳转；不实现无头浏览器解码（脆弱且重）

## 变更日志

### v0.5.1 (2026-08-14)
- **TUI 排版全面修复（显示宽度感知）**：新增 `wrapCells/truncateCells/padCells/sliceCells/cutCells/popupContentH` 工具，全链路按终端显示列（CJK=2 列）计算
  - 修复弹窗标题栏用 rune 数计宽导致 CJK 标题下 dash 填充错位
  - 修复弹窗底图叠加用列宽索引 rune 切片（CJK 底图错位/重叠/ANSI 切坏）
  - 修复 wrapLines 按 rune 数折行导致中文行宽 2 倍溢出（阅读器/弹窗内容被截）
  - 修复 helpView `%-*s` 把 ANSI 转义计入宽度导致两列错位
  - 修复列表/阅读器长标题、长 URL、Tab 行、状态栏溢出硬换行
  - 弹窗滚动钳位到“最后一行可见”，空 lines 不再产生 offset=-1；LLM 输出 markdown 标记清理（sanitizeLLMText）
  - 新增 9 个排版单测（弹窗 CJK 边框对齐/阅读器无溢出/帮助对齐/滚动钳位等）
- **`--cat` 硬过滤**：CLI 显式传 `--cat` 时真正只输出指定分类（此前仅影响排序，与 README “只看” 不符）；新增 CatFilter 选项与回归测试

### v0.5.0 (2026-08-14)
- 新增 `edu-policy` 分类（教育/人才政策）：国际学生、签证、实习、高校、STEM 人才流动
  - 四语词表（en/de/fr/zh 繁简双向）+ 多词短语强信号；**阈值 ≥4 分才入选**（防“顺带提一句学生/scholar”类传记新闻误入）
  - 新增 6 个教育源（全部实测可用）：Guardian Education、Inside Higher Ed、The PIE News、The Conversation Education、Hechinger Report、EdSurge（UWN/Chronicle feed 失效已移除）
  - Google News 聚合源同步加 edu-policy 查询对（18→24）
- 输出/TUI 增加 edu-policy 配色（青）与 🎓 图标

### v0.2.0 (2026-08-03)
- 新增 `zh` 语言（台湾：中央社×4、自由时报×3）+ 中文四分类词表（繁简）
- 新增 `uspolitics` 分类（美国本国政治）+ NPR/Roll Call/ABC 源
- 新增 `ui` TUI（bubbletea：Tab 导航/全文阅读/过滤/导出/浏览器打开）
- 新增 `find` 付费墙转载搜索（gnews 包）
- `read` 付费墙检测提示；`--sources` 支持 TUI；修复 read/find 参数重排（flag 停止解析问题）
- 来源 53 → 65+；测试 12 → 14 包

### v0.1.0 (2026-08-02)
- 初始版本：53 源、三语分类、RSS/scrape/readability、去重排序、seen-store、三输出
