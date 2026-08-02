# news-report · 欧美权威媒体新闻聚合器

自动从 **欧美权威新闻媒体**（英/德/法三语）抓取最新资讯，专注 **国际政策（政治）· 经济形势（金融）· 产业发展** 三大领域。

[English](README_EN.md)

## 特性

- **广度 × 深度兼顾的来源矩阵（53 个内置源）**
  - `wire` 通讯社：路透、美联社（最快最广，权重 1.0）
  - `legacy` 老牌媒体：BBC、卫报、纽约时报、WSJ、华盛顿邮报、Le Monde、Le Figaro、FAZ、Die Zeit、Spiegel、Handelsblatt、Süddeutsche、DW、France 24…
  - `specialist` 机构智库：IMF、美联储、ECB、联合国新闻、EIA、布鲁金斯、CFR、PIIE、CSIS、Chatham House、Politico EU、EURACTIV…
  - 可选 `--google-news` 聚合源（额外广度）
- **三语分类引擎**：politics / economy / industry 关键词分类（en/de/fr 独立词表，含多词短语、负面词过滤娱乐体育）
- **出色的网页信息获取能力**
  - RSS 2.0 / Atom / RDF 自动识别，多候选 feed 依次尝试
  - HTML scrape 兜底（goquery 启发式 + 选择器 + 链接模式过滤）
  - `read` 子命令：go-readability 全文提取（广告/导航剥离），失败自动回退
  - gzip、重定向、指数退避重试（4xx 不重试）、UA 可配、robots.txt 尊重（按 UA 分组，支持站点白名单如 UN 的 Feedfetcher-Google）
- **智能去重**：规范化标题 + Jaccard 相似度聚类（跨语言阈值自适应）
- **新鲜度排序**：来源权重 × 指数衰减（半衰期可配）+ 相关性加分
- **已读记录**：JSON 缓存（`~/.cache/news-report/seen.json`），重复运行只报新条目；`--show-seen` 可回看
- **三种输出**：彩色终端 / Markdown 报告（含全文摘要折叠、来源统计表）/ JSON
- **深度模式**：`--fulltext N` 自动抓取每分类 Top-N 全文

## 安装

```bash
cd ~/Desktop/go-projects/news-report
make build          # 或 go build -o bin/news-report .
make install        # 安装到 ~/.local/bin
```

## 快速开始

```bash
news-report                            # 最近 24h 三语全分类报告（终端）
news-report --out markdown --outfile report.md
news-report --strict                   # 只显示专注分类（隐藏 other）
news-report --lang de,fr --cat economy # 只看德法经济新闻
news-report --minutes 720 --limit 5    # 12 小时窗口，每类 5 条
news-report --fulltext 3               # 每类 Top-3 抓取全文
news-report read <url> --lang fr       # 深度阅读单篇文章
news-report sources --live             # 实测所有来源可用性
news-report init                       # 生成默认配置
```

## 配置

配置文件 `~/.config/news-report/config.yaml`（`news-report init` 生成，`--config` 可指定其他路径）：

```yaml
languages: [en, de, fr]        # 语言
categories: [politics, economy, industry]
minutes: 1440                  # 新鲜度窗口（分钟）
limit_per_category: 12
total_limit: 80
concurrency: 12
timeout_seconds: 15
retries: 2
halflife_hours: 12             # 新鲜度衰减半衰期
proxy: ""                      # 留空 = 环境变量 (HTTP_PROXY 等)
cache_dir: ~/.cache/news-report
store_days: 7                  # 已读记录保留天数
show_seen: false
strict_focus: false
google_news: false
fulltext: 0
fulltext_max_chars: 3000
sources:
  bbc-world:
    enabled: true
    weight: 0.9
    # feeds: [自定义 URL 列表]
```

## 消息源一览

| 语言 | 通讯社 (wire) | 老牌媒体 (legacy) | 机构/智库 (specialist) |
|---|---|---|---|
| **EN** | Reuters×3、AP | BBC×3、Guardian×3、NYT×4、WSJ×3、WaPo×2、Economist | Politico EU、EURACTIV、The Hill、Brookings、CFR、PIIE、CSIS、Chatham House、UN News、EIA、IMF、Fed、ECB |
| **DE** | — | DW×2、Tagesschau、Spiegel×2、Zeit×3、FAZ×2、Handelsblatt×2、SZ×2 | — |
| **FR** | — | Le Monde×3、Le Figaro×2、France 24×2、RFI、Les Échos、Le Point、franceinfo | — |

> 标注 `(候选)` 的源 feed 地址可能变动或受反爬限制（Cloudflare 403），失败自动静默降级；
> EIA 的 robots.txt 自身禁止 `/rss`（默认会被 robots 拦截，`--no-robots` 可绕过）；
> UN News 使用站点白名单 UA `Feedfetcher-Google/1.0`（其 robots 对 /feed 显式放行）。

## 架构

```
main.go (CLI)
  └─ report.Run() 流水线
        ├─ sources   来源注册表（层级/语言/权重/候选 feed）
        ├─ fetch     HTTP：代理/UA/超时/重试/robots(按UA分组)
        ├─ feed      RSS/Atom/RDF 解析（多时间格式容错）
        ├─ scrape    HTML 兜底（选择器 + 链接模式过滤）
        ├─ classify  三语政治/经济/产业分类
        ├─ dedup     规范化标题 + Jaccard
        ├─ rank      权重 × 新鲜度指数衰减
        ├─ store     已读记录（JSON 持久化）
        ├─ article   go-readability 全文提取 + 回退
        └─ output    终端 / Markdown / JSON
```

## 测试

```bash
make test    # 全部离线单测（feed 解析、分类、去重、排序、robots、流水线）
make smoke   # 真实联网冒烟
```

## 合规说明

- 默认遵守 robots.txt（按 UA 分组精确匹配，缓存 24h）；`--no-robots` 可关闭
- 单请求超时 + 重试退避 + 并发上限，避免对来源站造成压力
- 仅抓取公开 RSS/公开页面，不绕过登录墙；付费媒体（FT、Bloomberg）未收录

## 常见问题

- **网络受限**：工具读取 `HTTP_PROXY/HTTPS_PROXY` 环境变量，或 `--proxy http://127.0.0.1:7897`
- **某来源总是失败**：`news-report sources --live` 实测；可在配置里给该源换 `feeds` 或 `enabled: false`
- **想只看新增**：默认已按已读记录过滤；`--show-seen` 显示全部
- **分类不准**：词表在 `internal/classify/classify.go`，可自行增删关键词
