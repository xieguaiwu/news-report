已通读 `main.go`、`internal/tui/tui.go`（handleReaderKey、fetchReaderCmd 异步模式）、`internal/config/config.go`、`internal/gnews`、`internal/article` 后，产出 5 个方案。所有方案共享一个**共同底座**（下文统一核算进改动量）：

**底座：`internal/llm` 通用接入层**
- 新增 `internal/llm/llm.go`：OpenAI 兼容 `/chat/completions` 客户端（DeepSeek / OpenAI / 本地 Ollama 通吃），`config.yaml` 新增 `llm:` 段：`base_url`、`api_key`（支持 `{env:OPENAI_API_KEY}` 引用，不硬编码，符合既有密钥安全约定）、`model`、`target_lang`（默认 zh）、`timeout`。
- 无 key / 请求失败 → 优雅降级：提示"配置 LLM 后可翻译（`news-report init` 重新生成配置）"，现有功能不受影响。
- 所有 LLM 输出按 `sha256(url+lang+指令)` 落盘 `~/.cache/news-report/llm/`，命中零成本。

---

## 方案 1：`t` 一键全文翻译（译）

**概述**：阅读器里按 `t`，整篇新闻即时翻译成中文（目标语言可配），译文可一键复制。

**用户场景**：用户浏览德语 Tab 看到 FAZ 一篇关于 ECB 加息的文章，Enter 进入全文阅读，读到一半按 `t` → 顶部出现 "译中…" 指示（复用现有 loading 模式）→ 数秒后正文替换为流畅中文，专有名词（ECB、Bundesbank）保留原文。按 `c` 复制的是译文而非链接（新增 `C` 键复制译文，`c` 语义不变）。按 `Esc` 返回原文。同一文章再次进入直接命中译文缓存，秒开。

**实现要点**：
- `handleReaderKey`（tui.go:343）新增 `"t"` 分支：以 `fetchReaderCmd` 同款异步 `tea.Cmd` 模式发起 `translateMsg`，不阻塞 UI。
- 分块翻译：默认截取前 4000 字符（`llm.translate_max_chars` 可调），按段落分块 ≤1500 字符、块间重叠 50 字符避免断句；系统提示词固定角色："专业新闻译者，保留专有名词/数字/引语，输出 target_lang"。
- 缓存 key = `sha256(URL+原文语言+目标语言)`，TTL 7 天。
- CLI 侧同步补一个 flag：`news-report read <url> --lang de --translate`（复用 `runRead` 现有管线，body 后接 LLM 调用）。

**改动量：少量**（新增 llm 客户端 + 1 键 + 1 flag + 缓存；reader view 加一个 mode 字段）

## 方案 2：`x` 摘要 / `d` 解读（速览两键）

**概述**：列表按 `x` 看本条 5 条要点摘要，阅读器按 `d` 看四段式深度解读（背景/关键方立场/影响/后续关注点）。

**用户场景**：早晨 7 点扫列表，对每篇拿不准要不要细读的新闻按 `x`，5 条要点 + 一句"值得读吗"判断，30 秒过完 20 条。遇到重磅新闻（如美联储决议）按 `d`：LLM 补上背景（上次会议、市场预期）、各方立场（联储 vs 华尔街）、对人民币/港股的可能影响、接下来盯什么数据——这是 RSS 摘要给不了的"解读"。

**实现要点**：
- 列表/阅读器两处各加一个键；`m.reader.item` 已携带 URL/标题/语言，正文从 `m.articleCache` 或已抓取的 `reader.body` 取，**零额外抓取**。
- `x`：prompt 要求 ≤5 条 bullet、每条 ≤25 字、结论先行；`d`：四段模板 + 每条 ≤50 字，输出缓存同方案 1。
- 视图复用 reader 渲染（新 mode 字段区分原文/译文/摘要/解读，Esc 逐级回退）。
- 无正文时（列表直接按 x）自动先触发 `fetchReaderCmd` 再转 LLM，一条链路串起来。

**改动量：少量**（两个 prompt + 两个键 + view mode，全在 llm/tui 内）

## 方案 3：`dig` — 付费墙深挖

**概述**：一条命令搞定"被墙 → 找转载 → 读转载 → AI 讲人话"全链路：`news-report dig <url>`。

**用户场景**：WSJ 一篇《Chipmakers Race for 2nm》卡在付费墙。现在要手动 `find` 抄关键词、挑转载、再 `read`。有了 dig：直接 `news-report dig "https://wsj.com/..."` → 程序自动识别付费墙 → 用标题搜 Google News 转载 → 自动跳过原站 → 依次尝试抓取前 3 个转载全文 → LLM 输出 300 字摘要 + "转载自 XX（原文质量可能打折）"标注 + 最佳转载链接。全程一个命令、一条输出。

**实现要点**：
- 新子命令，纯管道装配，**全部复用现有件**：`article.Extract`（判定 `ErrPaywall`）→ `gnews.Search` + `ExcludeOriginal`（现成）→ 对候选逐一 `article.Extract`（复用 `runRead` 抓取逻辑）→ `llm.Summarize(title+正文前 3000 字)`。
- flags：`--lang zh`（摘要语言）、`--limit N`（试抓候选数，默认 3）、`--no-ai`（降级为纯链接列表，即现在的 find 行为）。
- 摘要输出 markdown 骨架（标题/要点/转载源/链接），可直接 `--outfile` 落盘。

**改动量：中等**（新子命令 ~250 行，但 80% 是现有函数装配，无新领域逻辑）

## 方案 4：`digest` — 每日 AI 简报

**概述**：单命令生成"今天四域发生了什么"的 5 分钟中文简报：每类一段总结 + 全站 Top 结论 + 关键条目链接。

**用户场景**：用户每天 8:00 跑 `news-report digest --outfile ~/notes/today.md`（cron 或 systemd timer 自动化），通勤路上只读这一份文件：经济段 3 句话讲清当日央行/市场大事，每条结论带原文链接，想深挖再 `news-report ui` 或 `read`。出差时 `digest --lang en` 生成英文版发同事。

**实现要点**：
- 新子命令，复用 `report.Run` 管线（`--fulltext 3` 已具备每类抓 Top-3 全文的能力，直接启用）。
- 阶段 1：每类把 Top-3 全文（或摘要）各压缩成 60 字"条目一句话"；阶段 2：四类总结 + 全局"今日最重要一件事"（两类 LLM 调用，分步保证不超上下文）。
- 输出 markdown：标题带日期、分节、每条含 `[来源域名](链接)`；`--lang zh|en` 控制输出语言；失败降级为现有纯列表报告（不空手而归）。
- 自动化：README 补一行 cron 示例；输出文件天然可被 `s` 导出思路复用。

**改动量：中等**（新子命令 + 两级 prompt 编排，抓取部分全复用）

## 方案 5：`v` 多语视角 — 同一事件跨媒体对比

**概述**：按 `v`，LLM 找出同一事件在德/法/中/英四语媒体的报道，并输出"各方视角差异"对比——这是四语聚合器独有的杀手锏。

**用户场景**：读者在美国政治 Tab 看到 Reuters 关于对华关税的报道，按 `v` → 程序把当前报告里同分类的条目（含语言标签）交给 LLM，判定哪些是同一事件 → 输出：

```
🇺🇸 路透：聚焦关税对通胀的影响（政策面）
🇩🇪 FAZ：关注欧洲企业供应链冲击（产业面）
🇨🇳 中央社：强调中方反制表态（立场面）
→ 差异点：美媒谈国内成本，德媒谈产业外迁，华媒谈博弈筹码
```

每条附链接，Enter 直接跳读对应语言原文。用户瞬间获得"西方媒体如何讲中国 / 中国媒体如何讲美国"的框架。

**实现要点**：
- 键 `v` 同时挂在列表和阅读器；**零额外抓取**——数据就是当前 `report.Items`（标题+摘要+语言+URL 已在内存），按同分类过滤后取每语言 Top-3 标题送 LLM。
- prompt：给"标题+语言+来源"清单，要求 (1) 判定同事件 (2) 按语言分组各写一句视角 (3) 输出差异点 ≤3 条；结果存 `m.llmCache`（内存）+ 磁盘缓存。
- 无 LLM 时降级：按语言分组显示候选标题列表（纯机械分组，仍有用）。
- 风险控制：LLM 判定加 `temperature=0.2`，输出强制 JSON（标题索引数组），解析失败即降级为分组列表。

**改动量：中等**（核心是"同事件判定"的 prompt + JSON 解析 + 新视图；无网络层改动）

---

**组合建议**（一句话）：管道 `digest 日报 → ui 扫列表 → x 速览 → t 精读 → v 对比`，五键覆盖"知道发生了什么 → 决定读什么 → 读懂 → 读透"；成本控制靠三件事——LLM 输出全缓存、`translate_max_chars` 默认 4000、无 key 自动降级，本地 Ollama 可零成本跑通全部功能。