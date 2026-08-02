# Task for momus

对比 news-report 的 interactive TUI（internal/tui/tui.go） 与 ~/prompt_boilerplates/Coding/interactive-cli-design.md 规范，输出缺失项 checklist。

规范文件已加载在上下文中。只读代码：
- internal/tui/tui.go（模型/Update/View/handleKey/handleFilterKey/handleReaderKey）
- internal/tui/tui_test.go（现有测试）

按 §§1-5 逐条对照，输出：
- P0（强制缺失）：键位绑定缺失、搜索 n/N 导航缺失、帮助 ? 缺失
- P1（界面缺失）：位置指示器、空格翻页、NO_COLOR 遵守
- P2（测试缺失）：PTY 测试、非交互降级测试
- 已有 ≡ 符合项（列出已实现的规范条目）

格式：Markdown checklist，每项标注 文件:行号 与优先级。只读，不写代码。

## Acceptance Contract
Acceptance level: checked
Completion is not accepted from prose alone. End with a structured acceptance report.

Criteria:
- criterion-1: Implement the requested change without widening scope

Required evidence: changed-files, tests-added, commands-run, residual-risks, no-staged-files

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