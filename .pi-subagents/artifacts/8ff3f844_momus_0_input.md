# Task for momus

对 Go 项目 /home/xieguiawu/Desktop/go-projects/news-report 做独立代码审查（只读，不修改任何文件）。这是一个新闻聚合器 CLI：从欧美权威媒体（英/德/法）抓取政治/经济/产业新闻，含 RSS/Atom/RDF 解析、HTML scrape 兑底、go-readability 全文提取、三语关键词分类、Jaccard 去重、权重×新鲜度排序、seen-store、robots.txt 尊重（按 UA 分组）。

审查重点：
1. 并发正确性：internal/report/report.go 的 worker pool（WaitGroup + sem）、seen-store 并发、FetchFulltext 的 map 写入是否安全
2. 数据竞争与 panic 风险：内部包全部（internal/fetch、feed、classify、dedup、rank、store、article、scrape、config、output、main.go）
3. 错误处理：fetch 重试逻辑（4xx 不重试、5xx 重试）、robots 判定（无法判定时允许）、feed 解析失败降级是否合理
4. 边界情况：feed 时间解析（未知时区）、空输入、超长响应、负数/零值配置
5. 安全：SSRF/路径穿越（URL 处理）、敏感信息硬编码、日志泄露
6. 测试质量：internal/*/ 下测试是否真实断言（quality gate 6）
7. 契约问题：跨包调用是否一致（如 Sources.Source 新增字段后所有构造路径是否正确）

输出：按严重度分级（CRITICAL/HIGH/MEDIUM/LOW）列出问题，每条给文件:行号、问题描述、修复建议。不要修改代码，只报告。

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