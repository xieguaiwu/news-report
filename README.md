# news-report · 欧美权威媒体新闻聚合器

自动从 **欧美权威新闻媒体**（英/德/法/中文四语）抓取最新资讯，专注 **美国政治 · 国际政策 · 经济形势 · 产业发展 · 教育/人才政策** 五大领域。

[English](README_EN.md)

## 特性

- **广度 × 深度兼顾的来源矩阵（65+ 内置源）**
  - `wire` 通讯社：路透、美联社（最快最广，权重 1.0）
  - `legacy` 老牌媒体：BBC、卫报、纽约时报、WSJ、华盛顿邮报、Le Monde、Le Figaro、FAZ、Die Zeit、Spiegel、Handelsblatt、Süddeutsche、DW、France 24、中央社、自由时报…
  - `specialist` 机构智库：IMF、美联储、ECB、联合国新闻、EIA、布鲁金斯、CFR、PIIE、CSIS、Chatham House、Politico EU、EURACTIV、NPR、Roll Call…
  - 可选 `--google-news` 聚合源（额外广度）
- **四语五分类引擎**：uspolitics / politics / economy / industry / edu-policy 关键词分类
  - en/de/fr 独立词表 + **中文词表**（繁简双向，子串匹配适配无空格语言）
  - edu-policy 覆盖国际学生/签证/实习/高校/STEM 人才流动（中关学生签证限制等）
  - 美国政治专属词（senate/congress/白宮/參議院…）强信号权重，自动区分美国本国政治与国际政策
- **出色的网页信息获取能力**
  - RSS 2.0 / Atom / RDF 自动识别（含 UTF-8 BOM 剥离），多候选 feed 依次尝试
  - HTML scrape 兜底（goquery 启发式 + 选择器 + 链接模式过滤）
  - `read` 子命令：go-readability 全文提取 + **付费墙检测**（识别 NYT/WSJ/Economist 等软墙并提示）
  - gzip、重定向、指数退避重试（4xx 不重试）、UA 可配、robots.txt 尊重（按 UA 分组）
- **付费墙解决方案**：`find` 子命令按标题短语搜索 Google News，列出免费转载/镜像候选（含原站域名标记）
- **交互式 TUI**：`ui` 子命令——分类 Tab 导航、键盘选择、Enter 全文阅读、`o` 浏览器打开、`/` 搜索过滤、`s` 导出 Markdown
  - **AI 解读**（配置 `llm.api_key` 后可用）：`t` 全文翻译、`x` 五点摘要、`d` 深度解读（弹窗叠加层）
  - 弹窗/阅读器全链路**显示宽度感知排版**：CJK 全角按 2 列计算，任意终端尺寸下边框对齐、不溢出
- **智能去重**：规范化标题 + Jaccard 相似度聚类（跨语言阈值自适应）
- **新鲜度排序**：来源权重 × 指数衰减（半衰期可配）+ 相关性加分
- **已读记录**：JSON 缓存，重复运行只报新条目；`--show-seen` 可回看
- **三种输出**：彩色终端 / Markdown 报告 / JSON

## 安装

### 方式一：源码构建

```bash
cd ~/Desktop/go-projects/news-report
make build          # 或 go build -o bin/news-report .
make install        # 安装到 ~/.local/bin
```

### 方式二：GitHub Release

从 [Releases](https://github.com/xieguaiwu/news-report/releases) 下载 `news-report-<version>-linux-amd64.tar.gz`：

```bash
tar xzf news-report-*-linux-amd64.tar.gz
sudo install -Dm755 news-report /usr/local/bin/news-report
```

### 方式三：COPR（Fedora）

```bash
sudo dnf copr enable xieguaiwu/news-report
sudo dnf install news-report
```

## 快速开始

```bash
news-report                              # 最近 24h 四语全分类报告
news-report ui                           # 交互式界面（←→ 切分类，Enter 阅读全文）
news-report read <url> --lang de         # 深度阅读单篇文章
news-report find "EU sanctions Russia"   # 付费墙文章找免费转载
news-report --out markdown --outfile report.md
news-report --cat edu-policy --lang en,zh      # 只看教育/人才政策（国际学生/签证/实习）
news-report --strict                     # 只显示专注分类
news-report --lang zh,en --cat uspolitics # 只看中美政治
news-report --fulltext 3                 # 每类 Top-3 抓取全文
news-report sources --live               # 实测所有来源可用性
news-report init                         # 生成默认配置
```

## 交互界面（TUI）

```
📰 news-report  2026-08-03 00:39 ｜ 45 来源 · 320 条
 USPOLITICS (12)  POLITICS (28)  ECONOMY (31)  INDUSTRY (24)  OTHER (18)
▸ ● Trump signs executive order — NPR Politics · 1h · EN
   ● 台積電先進製程產能滿載 — 中央社 財經 · 2h · ZH
   ...
←→ 分类  ↑↓ 选择  Enter 阅读  o 浏览器打开  / 搜索  s 导出  r 刷新  q 退出
```

| 键 | 功能 |
|---|---|
| `←`/`→` 或 `Tab` | 切换分类 |
| `↑`/`↓` 或 `j`/`k` | 移动选择 |
| `Enter` | 抓取并阅读全文（`Esc` 返回） |
| `t` | AI 翻译当前正文（再按恢复原文；需配置 llm.api_key） |
| `x` | 列表视图：AI 生成 5 点摘要（弹窗显示） |
| `d` | 阅读器视图：AI 深度解读（弹窗显示） |
| `o` | 浏览器打开当前条目 |
| `/` | 标题/来源过滤 |
| `s` | 导出当前分类为 Markdown |
| `r` | 重新抓取 |
| `q`/`Ctrl+C` | 退出 |

## 付费墙与转载

部分来源有软付费墙（NYT/WSJ/Economist/FAZ/Handelsblatt 部分文章）：RSS 摘要可读、全文需订阅。

- `read` 命令自动检测付费墙并提示
- `find "标题关键词"` 通过 Google News 聚合搜索免费转载/镜像：

```bash
news-report find "EU sanctions package Russia" --lang en --limit 8
# 输出候选：来源域名 + 发布时间 + 跳转链接（浏览器打开自动转原文）
# 付费墙文章优先选 [原站] 以外的转载媒体
```

> 免费完整来源（无需订阅）：AP、BBC、Guardian、DW、Le Monde（多数）、France 24、UN、IMF、Fed、ECB、Brookings、NPR、Roll Call、中央社、自由时报等。

## 配置

配置文件 `~/.config/news-report/config.yaml`（`news-report init` 生成，`--config` 可指定其他路径）：

```yaml
languages: [en, de, fr, zh]      # 语言
categories: [uspolitics, politics, economy, industry, edu-policy]  # 分类（顺序即报告顺序）
minutes: 1440                    # 新鲜度窗口（分钟）
limit_per_category: 12
total_limit: 80
concurrency: 12
timeout_seconds: 15
retries: 2
halflife_hours: 12
proxy: ""                        # 留空 = 环境变量 (HTTP_PROXY 等)
cache_dir: ~/.cache/news-report
store_days: 7
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
llm:                              # 可选：AI 翻译/摘要/解读（t / x / d 键）
  api_key: "{env:DEEPSEEK_API_KEY}"   # 支持 {env:VAR} 环境变量引用
  base_url: https://api.deepseek.com/v1
  model: deepseek-chat
  target_lang: zh                  # 翻译目标语言
```

## 消息源一览（65+）

| 语言 | 通讯社 (wire) | 老牌媒体 (legacy) | 机构/智库 (specialist) |
|---|---|---|---|
| **EN** | Reuters×3、AP | BBC×3、Guardian×4、NYT×4、WSJ×3、WaPo×2、Economist | Politico EU、EURACTIV、The Hill、Brookings、CFR、PIIE、CSIS、Chatham House、UN News、EIA、IMF、Fed、ECB、**NPR Politics、Roll Call、ABC News、Inside Higher Ed、The PIE News、The Conversation、Hechinger Report、EdSurge**（教育/人才） |
| **DE** | — | DW×2、Tagesschau、Spiegel×2、Zeit×3、FAZ×2、Handelsblatt×2、SZ×2 | — |
| **FR** | — | Le Monde×3、Le Figaro×2、France 24×2、RFI、Les Échos、Le Point、franceinfo | — |
| **ZH** | — | **中央社×4**（兩岸/國際/大陸/財經）、**自由時報×3**（政治/國際/財經） | — |

> 完整清单与可用性状态见 [docs/SOURCES.md](docs/SOURCES.md)。
> 标注 `(候选)` 的源可能受 Cloudflare 反爬（403）或 robots 限制，失败自动静默降级；
> EIA 的 robots.txt 自身禁止 `/rss`（`--no-robots` 可绕过）；
> UN News 使用站点白名单 UA `Feedfetcher-Google/1.0`。

## 架构

```
main.go (CLI: report / ui / read / find / sources / init)
  ├─ report.Run() 流水线
  │     sources → fetch → feed/scrape → classify → dedup → rank → store → output
  ├─ tui        bubbletea 交互界面（Tab 导航 / 全文阅读 / 过滤 / 导出）
  ├─ gnews      Google News RSS 搜索（find 转载）
  └─ article    go-readability 全文提取 + 付费墙检测
```

详细架构与开发说明见 [DEVELOPMENT.md](DEVELOPMENT.md)。

## 测试

```bash
make test    # 17 包离线单测（含 -race）：feed/分类/去重/排序/robots/流水线/TUI/gnews/llm
make smoke   # 真实联网冒烟
```

## 合规说明

- 默认遵守 robots.txt（按 UA 分组精确匹配，缓存 24h，并发单次抓取）；`--no-robots` 可关闭
- Google News RSS 端点：其版权声明明确许可「个人非商业用途的 feed reader」，find 功能按此使用
- 单请求超时 + 重试退避 + 并发上限；仅抓取公开 RSS/公开页面，不绕过登录墙
- 付费媒体（FT、Bloomberg）未收录

## 常见问题

- **网络受限**：读取 `HTTP_PROXY/HTTPS_PROXY` 环境变量，或 `--proxy http://127.0.0.1:7897`
- **某来源总是失败**：`news-report sources --live` 实测；可在配置里给该源换 `feeds` 或 `enabled: false`
- **想只看新增**：默认已按已读记录过滤；`--show-seen` 显示全部
- **分类不准**：词表在 `internal/classify/classify.go`（四语五分类），可自行增删关键词
- **中文显示乱码**：确认终端 UTF-8；台湾源标题为繁体
