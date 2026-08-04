# Task for momus

审查 news-report 项目中刚实现的 LLM 集成代码。

审查重点：
1. API key 安全：是否硬编码？是否使用 {env:VAR} 引用？
2. 错误处理：LLM 不可用时是否优雅降级（不崩溃、不影响现有功能）？
3. 缓存策略：是否正确缓存？TTL 是否合理？
4. 并发安全：map/slice 是否有竞态？
5. TUI 键盘事件：是否与现有按键冲突？
6. 代码质量：是否符合项目现有风格？注释是否充分？
7. 测试覆盖：新增代码是否有测试？

请阅读所有新增/修改的文件，然后给出审查报告。
如果发现 bug，直接修复后重新验证 go build 和 go test。
输出中文。

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