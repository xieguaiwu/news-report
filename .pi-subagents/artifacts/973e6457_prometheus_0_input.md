# Task for prometheus

为 news-report 项目设计 HTTP feed 响应缓存层的架构方案。项目当前问题：每次运行重新抓取所有 65+ 源的 feed（~40s），无任何缓存。

需求：
1. 弹性缓存有效期——不同来源按层级（wire/legacy/specialist）有不同 TTL：wire 5-10min、legacy 10-15min、specialist 30-60min、zh 10min
2. 尊重 HTTP 标准：如果服务端返回 Cache-Control: max-age=N 或 Last-Modified/ETag，发 If-Modified-Since/If-None-Match，304 用缓存
3. 缓存空间管理：单条最大 500KB，总缓存目录不超过可配置上限（默认 50MB），超过时 LRU 淘汰
4. 磁盘存储：~/.cache/news-report/feed_cache/{source_id}_{feed_index}.json
5. CLI：--no-cache 强制刷新，news-report cache clear 清缓存
6. 配置：可选 cache_ttl_minutes 统一覆写所有源 TTL

输出：Markdown 架构文档，含：
- 数据结构（缓存条目格式、存储格式）
- 集成点（collectSource 如何改造）
- 并发安全设计（多源并发访问缓存）
- LRU 实现策略
- HTTP 条件请求集成
- 配置字段扩展
- 边界情况处理（磁盘满、损坏文件、并发写）

只读分析，不写代码。

## Acceptance Contract
Acceptance level: attested
Completion is not accepted from prose alone. End with a structured acceptance report.

Criteria:
- criterion-1: Return a concise result and residual risks when applicable

Required evidence: manual-notes, residual-risks

Finish with a fenced JSON block tagged `acceptance-report` in this shape:
Use empty arrays when no items apply; array fields contain strings unless object entries are shown.
`criteriaSatisfied[].status` must be exactly one of: satisfied, not-satisfied, not-applicable.
`commandsRun[].result` must be exactly one of: passed, failed, not-run.
`manualNotes` and `notes` are optional strings; an empty string means no note and does not satisfy `manual-notes` evidence.
```acceptance-report
{
  "criteriaSatisfied": [
    {
      "id": "criterion-1",
      "status": "satisfied",
      "evidence": "specific proof"
    }
  ],
  "changedFiles": [
    "src/file.ts"
  ],
  "testsAddedOrUpdated": [
    "test/file.test.ts"
  ],
  "commandsRun": [
    {
      "command": "command",
      "result": "passed",
      "summary": "short result"
    }
  ],
  "validationOutput": [
    "validation output or concise summary"
  ],
  "residualRisks": [
    "none"
  ],
  "noStagedFiles": true,
  "diffSummary": "short description of the diff",
  "reviewFindings": [
    "blocker: file.ts:12 - issue found, or no blockers"
  ],
  "manualNotes": "anything else the parent should know"
}
```