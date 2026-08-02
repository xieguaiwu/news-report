# Task for librarian

调研 Google News RSS 搜索接口的用法，用于实现新闻聚合器的 'find 转载' 功能（搜索文章标题找免费转载/镜像）。

需要确认：
1. Google News RSS search URL 的确切格式：https://news.google.com/rss/search?q=QUERY&hl=xx&gl=xx&ceid=xx —— QUERY 是否支持 URL 编码的引号短语（如 %22exact+phrase%22）？返回的 <link> 是 news.google.com/rss/articles/... 跳转链接还是原站链接？
2. 有没有办法拿到原站 URL（比如 link 重定向到原站，或者 <description> 里有原站域名）？用 curl 实测：curl -s 'https://news.google.com/rss/search?q=%22EU+sanctions+package%22&hl=en-US&gl=US&ceid=US:en' 看返回 XML 结构，并 curl -sI 一个 rss/articles 链接看 Location 是否到原站。
3. 限流情况：连续请求 N 次是否 429？
4. 输出：Google News RSS 结构说明（channel/item 字段：title/link/pubDate/description/source），跳转机制实测结果，建议的实现方式（是否需要在 fetch 时跟随重定向拿原站 URL）。中文输出，简洁。

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