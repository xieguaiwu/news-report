# Task for momus

对 Go 项目 /home/xieguiawu/Desktop/go-projects/news-report 的 v0.2.0 新增/修改代码做独立只读审查（不修改文件）。重点审查范围（相对 v0.1.0 的新增）：

1. internal/classify/classify.go：新增 zh 词表（繁简、子串匹配分支 `if lang == "zh"`）、uspolitics 分类（强信号 ×2、multi ×4、tie-break 顺序 USPolitics 在前）、init() 词表预规范化——检查：zh 分支与 token 分支的分数累计是否一致、uspolitics 强信号是否引入误分类、init() 修改全局 map 的并发安全性
2. internal/tui/tui.go：bubbletea 模型——检查：readerMsg/saveMsg 异步消息与模型状态的一致性、saveCmd 闭包捕获 m 的竞态（捕获的是值还是指针）、reader 滚动边界、View 渲染的 rune/字节切片问题（readerView 里 visible[start:end] 按字节切！body 含中文/德法变音会切断 UTF-8）
3. internal/gnews/gnews.go：URL 构造（QueryEscape）、splitTitleSource 的 LastIndex 剥离、域名归一化
4. main.go：reorderArgs 参数重排逻辑（位置参数与 flag 值混排的边界）、runFind/runUI 错误处理
5. internal/feed/feed.go：时区缩写映射表（tzOffsets 的歧义：CST 在表里是 -6，但中国标准时间也是 CST +8——确认注释/决策是否合理）、BOM 剥离、rssSource 解析
6. internal/article/article.go：looksPaywalled 启发式（400KB 截断、hits>=2 阈值是否合理、误判风险）
7. internal/report/report.go：FetchFulltext 的 truncateRunes、catOrder 加入 uspolitics 后排序正确性

输出：按 CRITICAL/HIGH/MEDIUM/LOW 分级，每条给 文件:行号 + 问题 + 修复建议。特别注意 TUI readerView 的字节切片问题与 classify init() 并发问题。只报告不修改。

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