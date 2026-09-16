# CRYPTO_NOTES — 加密 meme 币（crypto）技术注记

> `news-report crypto` 子命令的实现注记：端点、字段、限制、凭据、代理要求、踩坑。
> 本文不含任何凭据值。定位与 `ASTOCK_NOTES.md` 平行。

## 1. 功能与文件

| 文件 | 职责 |
|---|---|
| `internal/crypto/crypto.go` | 类型与 JSONL schema（`SchemaVersion = "crypto-attention-v1"`） |
| `internal/crypto/llmscore.go` | 打分器。由 `scripts/port_astock_scorer.py` 从 astock 版机械移植（改 prompt + 字段） |
| `internal/crypto/sources_dex.go` | DexScreener 行情；`getWithRetry` / `getWithRetryHeaders` 与共享令牌桶 |
| `internal/crypto/sources_safety.go` | GoPlus + Honeypot.is；`parseFloatOrZero` |
| `internal/crypto/sources_attention.go` | 微博热搜 + Telegram Bot API（私群回退） |
| `internal/crypto/sources_tgweb.go` | **Telegram 公开频道网页预览**（零凭据，主力） |
| `internal/crypto/store.go` | JSONL 追加/原子写、去重、Telegram offset 持久化 |
| `crypto_cmd.go` | 子命令接线（flag / collect / score / JSONL） |
| `scripts/crypto_env.sh` | 凭据注入（600）；rbw-first |
| `scripts/crypto_daily.sh` / `crypto_snapshot.sh` | cron 入口（cron 已装本机） |
| `config/crypto_watchlist.txt` | 代币白名单 |
| `config/crypto_tg_channels.txt` | Telegram 公开频道白名单 |

## 2. 数据源

### 2.1 DexScreener（`source=dexscreener`，`kind=market`）

- **两个端点响应形状不同**（实测 2026-09-16）：
  - `/token-pairs/v1/{chain}/{token}` → **裸数组** `[{...}]`
  - `/latest/dex/search?q=` → 对象 `{"schemaVersion":..., "pairs":[...]}`
  - 两侧都必须兼容。`PairsByChain` 先按数组解、失败再按对象解。
- 取用字段：`pairCreatedAt`（ms → `event_at`）、`liquidity.usd`、`fdv`、`marketCap`、
  `volume.h24`、`txns.h24.buys/sells`、`priceUsd`、`priceChange.h1/h24`。
- 无需 key。**官方未公布限速** → 客户端内建共享令牌桶 ≤2 req/s。
- `/latest/dex/search` **按相关度排序，不是按时间**——不能用来发现新池子（见 §4 坑 1）。

### 2.2 GoPlus Token Security（`source=goplus`，`kind=safety`）

- `GET https://api.gopluslabs.io/api/v1/token_security/{chain_id}?contract_addresses={addr}`
- **chain_id 别写错**：BSC = `56`，Solana 主网 = `101`；错值返回 `code:2022 "chain not supported"`。
- 字段：`buy_tax` / `sell_tax`（**已是小数**）、`creator_address` / `creator_percent`、
  `is_open_source`、`cannot_sell_all`、`holder_count`。
- 无需 key。

### 2.3 Honeypot.is v2（`source=honeypot`，`kind=safety`）

- `GET https://api.honeypot.is/v2/IsHoneypot?address={addr}&chain={chain}`
- 字段：`isHoneypot`、`simulationResult.buyTax/sellTax`、`contractCode.openSource/proxy`、`summary.risk`。
- **单位差异**：Honeypot 的 tax 是**百分数**（`5.0`），GoPlus 的是**小数**（`"0.05"`）。
  在 `Metrics` 中**统一为小数**——转换在各采集器内部完成。
- `risk_level` 映射：`low=0 / medium=1 / high=2`；**未知档位记 -1**，便于下游区分「低风险」与「未返回」。
- 无需 key。

### 2.4 微博热搜（`source=weibo`，`kind=attention`）

- `GET https://weibo.com/ajax/side/hotSearch`
- **必须带 `Referer: https://weibo.com/`**。实测：只带 UA 或不带 UA 一律 `403 {"error":"Forbidden"}`。
  `sources_attention.go` 的 `weiboHeaders` 落盘了这套头（UA 用浏览器型 + Referer + Accept + Accept-Language）。
- 返回全站榜（约 52 条），**不是加密榜** → 入库前过 `FilterCryptoKeywords`。
- **实测覆盖极低**：52 条热搜命中加密关键词 **0 条**。该源可用但与 BSC meme 叙事基本不重叠。

### 2.5 Telegram 公开频道网页预览（`source=telegram_web`，`kind=attention`）★主力

- `GET https://t.me/s/<channel>`（`?before=<msg_id>` 翻页）
- **不需要 bot、不需要账号、不需要 API key。** 这是 Telegram 给公开频道提供的网页预览。
- HTML 结构（goquery 选择器）：
  - 消息块 `div.tgme_widget_message[data-post]`，`data-post="<channel>/<msgid>"`
  - 时间 `<time datetime="...">`（ISO8601 → `event_at`）
  - 正文 `div.tgme_widget_message_text`（多个块需拼接；`<br>` → `\n`）
  - 浏览量 `span.tgme_widget_message_views`（`9.05K` / `1.2M` 形态 → `parseViews`）
  - 翻页游标 `a[data-before]`
- 实测：3 个公开频道均 200，各 20 条消息，字段可完整抽取。
- **与 Bot API 的取舍**：网页预览零凭据 + 可翻历史（`getUpdates` 只给最近 24h），
  代价是仅公开频道 + 依赖 Telegram 不改版式。**本项目的监控目标都是公开的，故选网页预览。**
- 频道白名单：`config/crypto_tg_channels.txt`（默认只放非喊单类公开频道）。

### 2.6 Telegram Bot API（`source=telegram`，`kind=attention`）— 私群回退

- `GET https://api.telegram.org/bot{token}/getUpdates`
- 仅在需要读**私有**群时使用。Token 走 `rbw get api/telegram-bot`（rbw 中当前**无此条目**）。
- **offset 必须持久化**（`state/tg_offset.json`）：否则每轮从 0 起拉最近 24h，
  同一批消息被重复追加，「提及量」被轮次频率放大。
- **token 不得出现在 JSONL/日志里**：本采集器刻意把 `URL` 字段留空（token 在 URL 中是泄漏向量）。

## 3. LLM 打分

- 默认模型 **`qwen3.8-flash`**（`~/.pi/agent/models.json` 记录的**唯一确认 0-Credits 通道**）。
- ⚠️ 原默认 `glm-5.3-flash` 的限时免费期**已于 2026-09-12 结束**——用它当默认值会走付费通道。
- 输出字段：`tone(-2..2)` / `narrative` / `shill_score(0..1)` / `specificity(0..1)` /
  `source_tier` / `black_score(0..1)`。
- 确定性：`temperature=0` + `prompt_version` 入库。
- **实测耗时 ≈27s/条**（glm 是 0.6s/条但计费）。批量打分需按此估时。
- 无凭据时**降级 fetch-only** 并在 stderr 告警，`model` 字段写 `MISSING_KEY`。

## 4. 踩坑库（全部实测）

| # | 坑 | 现象 | 处置 |
|---|---|---|---|
| 1 | DexScreener `/latest/dex/search` 按**相关度**排序而非时间 | 用它做「新池子雷达」→ 210 条结果里 24h 内新池 **0 条**（雷达恒零） | 发现新池子改用 GeckoTerminal `/networks/{net}/new_pools`（腿 B） |
| 2 | `/token-pairs/v1/` 返回**裸数组** | `'list' object has no attribute 'get'`（Python 侧） | Go 侧 `PairsByChain` 两种形状都兼容 |
| 3 | 微博缺 `Referer` → 403 | 只带 UA 或不带都是 `403 Forbidden` | `weiboHeaders` 补齐浏览器头 |
| 4 | `read -r addr` 读**整行** | watchlist 行尾有 `# 注释` 时，整行被当地址 → 采集恒 0 条 | 改 `read -r addr _rest` |
| 5 | 部分源失败时整体 `return 1` | 「微博成功 + TG 失败」把成功的一半也丢了 | 有数据则告警保留，仅 0 条时失败 |
| 6 | cron 脚本无执行权限 | 装了 cron 也跑不起来 | `chmod +x`（计划里漏了，只有 `crypto_env.sh` 做了 600） |
| 7 | GeckoTerminal 无 UA 被 Cloudflare 拦 | 返回挑战页 HTML 而非数据 | 必须设 `User-Agent` |

## 5. 凭据与代理

- **LLM**：`scripts/crypto_env.sh` 从 `~/.pi/agent/models.json` 的 `bai` 条目解析
  （`apiKey` 是 `$VAR` 引用 → 环境取值 → 回退 `auth.json`）。值不 echo、不落盘、不提交。
- **Telegram Bot token（可选）**：优先 `rbw get api/telegram-bot`；环境变量已设则不覆盖；
  缺条目时优雅降级（rc=0 + stderr 提示），**不影响其他源**。
  - 一次性准备（仅私群需要）：`@BotFather` → `/newbot` → `rbw add api/telegram-bot` → 把 bot 拉进群。
  - **公开频道不需要这一步。**
- **代理**：出网经 `HTTPS_PROXY`（默认 `http://127.0.0.1:7897`）。
  Telegram / Google 系必须走代理；东财/巨潮等国内端点走直连规则。

## 6. 已知限制

- 微博热搜与 BSC meme 叙事基本不重叠（实测 0/52）。
- Google Trends（腿 B）只有日粒度，且 `pytrends` 是非官方 scraper。
- 公共 RPC 分块扫描会触发限流（腿 B 的持币扫描需节流）。
- `qwen3.8-flash` 约 27s/条。
- 无 `--score-only` 之外的批量并发控制（打分器内建并发 ≤4，但 `--score-only` 目前串行）。

## 7. 边界

- **只做公开数据观测，不含任何交易执行**（无私钥、无签名、无下单路径）。
- 声明见 `README.md` 的 crypto 小节（不构成投资建议；具体辖区的合规评估由使用者自负）。

*最后更新：2026-09-16*
