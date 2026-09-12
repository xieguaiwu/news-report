# ASTOCK_NOTES — A股舆情（astock）技术注记

> `news-report astock` 子命令的实现注记：端点、字段、限制、凭据注入法、代理要求。
> 本文不含任何凭据值。

## 1. 功能与文件

| 文件 | 职责 |
|---|---|
| `internal/astock/sources.go` | 东方财富个股新闻（getListInfo + search-api 双端点）+ 巨潮公告（hisAnnouncement）；指数退避、4xx 不重试、UA、失败降级 |
| `internal/astock/symresolve.go` | 代码/简称 → sym（sz000001 形式，与 SHARK 对齐）；全量列表缓存 `data/stock_list.csv`（TTL 7 天，原子替换写） |
| `internal/astock/llmscore.go` | OpenAI 兼容打分客户端；temperature=0 强制 JSON；并发≤4、单条 30s、重试≤3；`PromptVersion = "astock-llmscore-v1"` |
| `astock_cmd.go` | astock 子命令实现（main.go 仅 switch 接线 + usage） |
| `scripts/astock_env.sh` | 凭据注入（见 §4） |

## 2. 数据源端点

### 2.1 东方财富个股新闻（source_type=news）

- **主列表** `GET https://np-listapi.eastmoney.com/comm/web/getListInfo`
  - 参数：`client=web&mTypeAndCode=<secid>&type=1&pageSize=N&pageIndex=1`
  - **secid 规则：`1.`+代码 = 沪，`0.`+代码 = 深/北**（`astock.SecID`：sh600000→1.600000，sz000001→0.000001）
  - 响应字段：`data.list[]{Art_ShowTime, Art_Code, Art_Title, Art_Url}`（无正文 → text=标题）
- **检索** `GET https://search-api-web.eastmoney.com/search/jsonp`（akshare `stock_news_em` 同源）
  - `cb=<回调名>&param=<URL 编码 JSON>`，JSON 内 `keyword=6位代码`、`type=["cmsArticleWebOld"]`
  - 响应为 JSONP，需剥 `cb( )` 壳；字段：`result.cmsArticleWebOld[]{date,code,title,content,mediaName,url}`
  - **限制**：检索结果是"正文含该代码"的文章，含盘后交易表格类噪声；与主列表的 Art_Code 空间**基本不相交**（实测同股交集为空），故两端点产出**并集去重**而非按 ID 合并
- 合并策略：主列表在前（相关性高）+ 检索补充（带正文/媒体名），按 URL 与标题双键去重
- UA：Chrome 126 形态 UA + `news-report-astock/1.0` 后缀；Referer 非必需

### 2.2 巨潮资讯公告（source_type=announcement）

- **orgId 前置**：`GET http://www.cninfo.com.cn/new/data/szse_stock.json`（单请求 6000+ 条：code/zwjc/orgId/category）——同时是 symresolve 的**主列表源**（过滤 `category ∈ {A股, CDR}`，排除 B股）
- **公告列表** `POST http://www.cninfo.com.cn/new/hisAnnouncement/query`（form-urlencoded）
  - 必需：`stock=<code>,<orgId>`（无 orgId 查不到）；`column=szse/sse` 按市场传（实测两值均可跨市返回）；`pageNum/pageSize/tabName=fulltext/isHLtitle=true`；`seDate` 可空
  - 头：`X-Requested-With: XMLHttpRequest` + Referer 搜索页
  - 响应字段：`announcements[]{secCode, secName, announcementTitle(含 <em> 高亮), adjunctUrl, announcementTime(ms 时间戳)}`
  - URL 拼接：`http://static.cninfo.com.cn/` + `adjunctUrl`（PDF）
  - 日期：ms 时间戳 → Asia/Shanghai（UTC+8）→ `YYYY-MM-DD`

## 3. 日期口径

JSONL `date` 均为**新闻/公告自身日期**（北京时间），非抓取时刻。

## 4. 凭据注入法（零落盘）

```bash
source scripts/astock_env.sh   # export ASTOCK_LLM_BASE_URL / ASTOCK_LLM_API_KEY / ASTOCK_LLM_MODEL
```

解析顺序（脚本内完成，值不 echo）：
1. 读 `/root/.pi/agent/models.json` 的 `providers.bai` 条目 → `baseUrl` + `apiKey`；
2. `apiKey` 是 `$BAI_API_KEY` 形式的**环境变量引用**（models.json 中非字面 key）→ 从环境解析；
3. 环境缺失 → 回退 `/root/.pi/agent/auth.json` 的 `bai.key`（同为 pi 凭据库，实测本机 bash 环境走此分支）；
4. 模型默认 `glm-5.3-flash`（bai 免费档），可用 `ASTOCK_LLM_MODEL` 覆盖。

红线：key 值禁止 echo/落盘/提交；脚本自身 `chmod 600`；程序内仅从上述两个环境变量读凭据。

## 5. 代理要求

- **bai 网关直连被封锁**：必须走 HTTPS_PROXY（本机 `127.0.0.1:7897`，`source /root/.pi/env` 设置）。Go `net/http` 经 `http.ProxyFromEnvironment` 自动识别 `HTTPS_PROXY`/`NO_PROXY`，程序无需显式配置。
- 东财/巨潮为国内端点，本机代理的直连规则放行，不影响。

## 6. LLM 打分（llmscore.go）

- 输出 schema（temperature=0 + `response_format={"type":"json_object"}` 双保险）：
  - `tone`: 整数 -2..2（极空..极多）；`kind`: 业绩|监管|重组|传闻|研报|自媒体|其他
  - `specificity`: 0..1 信息具体程度；`source_tier`: 官方|媒体|自媒体|不明
  - `black_score`: 0..1 **低级黑判定**（论据缺失+情绪化渲染+恐慌/亢奋诱导，越满足越高）
- **glm-5.3-flash 始终思考**：`thinking.type` 参数被网关拒（400001「不支持关闭思考」），必须 `reasoning_effort=low`（实测 3s/条、reasoning_tokens=0）；`max_tokens` 需 ≥800（否则思考耗尽预算致 content 为空、finish=length）
- 兼容降级链（400 时逐级去字段重试）：`reasoning_effort` → `response_format`
- 解析兼容：markdown 围栏、前后杂文本、**模型把字段包进一层嵌套对象**（如 `{"answer":{...}}`，实测出现过）；越界值夹紧、非法枚举回退默认
- 限速：bai 免费档有速率限制，实测 53 条并发 4 出现数次 429/5xx，由重试（退避 1s→3s）吸收，最终 0 失败

## 7. 备用通道（未启用，仅注释）

dashscope/qwen token-plan compatible-mode：`models.json providers.qwen`
（baseUrl `https://token-plan.cn-beijing.maas.aliyuncs.com/compatible-mode/v1`，模型 `qwen3.8-flash`，
同为 `$VAR` 引用解析）。切换：`source scripts/astock_env.sh` 后覆盖三个 `ASTOCK_LLM_*` 变量即可，
程序端无需改动（OpenAI 兼容）。注意 qwen3.8-flash 无 `reasoning_effort` 依赖，若网关拒收该字段，
客户端的 400 降级链会自动去掉后重试。

## 8. 已知限制

- 东财主列表无正文摘要（text=标题）；检索端点带正文但有噪声（盘后表格类），由 LLM 按 specificity/black_score 甄别
- 巨潮公告仅标题（正文为 PDF，未解析）——公告标题本身信息密度高，tone 判别足够
- 北交所（bj，43/83/87/92 段）可解析 sym，但东财 secid 的北交所路由（`0.`+code）未逐一实测
- 全量列表缓存 7 天；新股上市后建议 `gio trash data/stock_list.csv` 强制刷新
- JSONL `out/` 已加入 .gitignore
