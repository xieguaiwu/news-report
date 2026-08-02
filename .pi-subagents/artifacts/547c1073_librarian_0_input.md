# Task for librarian

验证以下 RSS feed URL 是否真实可用（用 curl -s -A 'Mozilla/5.0' -o /dev/null -w '%{http_code} %{content_type}' 逐个检查，网络代理可用：环境变量 HTTP_PROXY 已设）。目标是给一个新闻聚合器添加台湾中文媒体与美国政治媒体源，需要找到当前实际可用的 feed URL。逐个测试并输出表格：URL | HTTP状态 | content-type | 是否有条目（可 curl 前 500 字节看是否 XML）。

台湾媒体（zh）:
1. 中央社 CNA 国际/两岸 RSS: https://www.cna.com.tw/rss/aipl.xml（试这个和其他可能的路径 https://www.cna.com.tw/rss/ 目录下 firstnews.xml / world.xml / finance.xml 等）
2. 中央广播电台 RTI: https://www.rti.org.tw/rss/ 或 https://www.rti.org.tw/rss/aipl 等（探查目录）
3. 联合报/自由时报/工商时报 是否有 RSS（探查常见路径 /rss.xml、/rss 等，不强制）

美国政治（uspolitics）:
4. NPR Politics: https://feeds.npr.org/1014/rss.xml
5. Roll Call: https://rollcall.com/feed/ 或 https://www.rollcall.com/feed/
6. Politico US Politics: https://www.politico.com/rss/politics.xml 或 https://www.politico.com/feed
7. AP US politics hub: https://apnews.com/hub/politics/feed 或 https://apnews.com/politics/feed
8. Axios: https://axios.com/feed 或 https://www.axios.com/feed
9. ABC News Politics: https://abcnews.go.com/abcnews/topstories 或 politics RSS（探查）

最后给结论：哪些可用（给出确切 URL）、哪些不可用。中文输出，简洁表格。

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