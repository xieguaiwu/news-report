# ASTOCK_PROGRESS — A股舆情能力升级进度日志

任务：news-report（Go 新闻聚合器）新增 internal/astock 包 + astock 子命令。
执行：本 agent（bai/glm-5.3-flash 免费通道），全程零付费。开始 2026-09-12。

## 时间线

- [19:2x] 项目勘察完成：module news-report，Go 1.25.4；internal/llm 已有 OpenAI 兼容客户端模式可参考；fetch 走 http.ProxyFromEnvironment（与任务代理要求一致）；main.go switch 子命令模式；.gitignore 已忽略 /data/。
- [进行中] 阶段 1：探测东方财富个股新闻（secid 型）与巨潮 hisAnnouncement 端点。

## 端点探测结论（19:3x，全部实测通过）

| 端点 | 用途 | 关键参数/字段 |
|---|---|---|
| push2.eastmoney.com/api/qt/clist/get | 全量列表（备选） | 单页上限 100，需分页；fs=m:0+t:6,m:0+t:80,m:1+t:2,m:1+t:23,m:0+t:81+s:2048 |
| www.cninfo.com.cn/new/data/szse_stock.json | 全量列表（**主源**） | 单请求 6251 条含 orgId；category 过滤 A股/CDR，排除 B股 |
| np-listapi.eastmoney.com/comm/web/getListInfo | 东财个股新闻 | mTypeAndCode=1.沪/0.深+code；Art_ShowTime/Art_Code/Art_Title/Art_Url |
| search-api-web.eastmoney.com/search/jsonp | 东财正文补充（stock_news_em 型） | jsonp 需剥壳；result.cmsArticleWebOld[]{date,code,title,content,mediaName,url}；Art_Code 与 getListInfo 同 ID 空间，可按 code 合并正文 |
| www.cninfo.com.cn/new/hisAnnouncement/query | 巨潮公告 | POST form：stock=code,orgId、column=szse/sse（实测两列均可跨市返回）、seDate 可空；announcementTime=ms 时间戳；adjunctUrl→http://static.cninfo.com.cn/<path> |
| api.b.ai/v1 chat/completions（经 HTTPS_PROXY） | 打分通道 glm-5.3-flash | temperature=0+response_format json_object ✓；**模型始终思考**：thinking.type 被拒(400001)，必须 reasoning_effort=low（high/max 可选）；max_tokens 需 ≥800；键值解析 models.json `$BAI_API_KEY` env 引用 → 环境缺失回退 auth.json bai.key；响应可能把字段包进一层嵌套对象（如 {"answer":{...}}），解析需兼容 |

凭据口径：models.json bai.apiKey 是 `$BAI_API_KEY` 环境变量引用（非字面 key，bash 环境实测 UNSET，pi 启动时自解析）；astock_env.sh 解析顺序 = models.json 读 `$VAR` 名 → 环境取值 → 回退 /root/.pi/agent/auth.json 的 bai.key（同为 pi 凭据库）。全程不 echo/不落盘 key 值。

## 实施记录

- symresolve：Resolve 规则 = 显式前缀权威 > 列表查码 > 代码段推断；简称退化匹配（去 */ST/退）。测试逮到 2 个 bug 并修复：①输入未做退化归一（*ST 简称匹配失败）②clampInt 对负数截断错误（-2→-1，改 math.Round）
- sources：FetchNews 双端点设计——实测证明 getListInfo 与 search-api 的 Art_Code 空间**不相交**（同股交集为空），放弃按 ID 合并正文，改为两端点产出并集（URL+标题双键去重），检索端点补充正文/媒体名。测试 fixture 曾漏 JSONP 收尾括号致误报，已修
- llmscore：glm-5.3-flash 实测「始终思考」——thinking.type 参数网关拒收，必须 reasoning_effort=low；max_tokens≥800 防 content 空（finish=length 视为可重试）；400 逐级降级（去 reasoning_effort→去 response_format）；解析兼容一层嵌套（实测出现 {"answer":{...}}）
- main：astock 子命令接线 main.go switch + usage；实现独立 astock_cmd.go；--sym 支持 multiFlag（重复+逗号）；无凭据自动降级 fetch-only 并标 model=MISSING_KEY
- astock_env.sh（600）：models.json bai.apiKey 实为 `$BAI_API_KEY` 环境引用（bash 环境 UNSET）→ env 解析失败回退 auth.json bai.key；自动补 source /root/.pi/env；零回显

## 验收结果

- A `go build ./...` PASS；B `go vet ./...` PASS（含 -tags=net）
- C 单测 12 项全绿（默认标签 21.7s；net 标签 5.9s：巨潮列表 6172 条/东财新闻 10 条/巨潮公告 5 条/真实打分 3.04s 全过）
- D 冒烟 out/astock_smoke.jsonl：53 行全部已打分（glm-5.3-flash，53 成功/0 失败，429 由重试吸收）；text≠title 17 行（检索正文）；tone 分布 -1:5/0:29/1:17/2:2；source_type news:35/announcement:18；fetch-only 与 score-only 回路各验证 1 轮（score-only 9/9）
- E ASTOCK_NOTES.md；F README 顶部「A股舆情（astock）」小节
- 整仓测试：除预存的 internal/config TestLLMParamInConfigFile（依赖环境 DEEPSEEK_API_KEY，设 dummy 即过，与本次改动零耦合）外全绿
- 卫生：/out/ 加入 .gitignore；测试产物已 gio trash
