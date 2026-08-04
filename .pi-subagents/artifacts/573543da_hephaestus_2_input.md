# Task for hephaestus

更新 ~/prompt_boilerplates/Coding/improvement-loop.md，融入 news-report 项目在 subagent 编排过程中的经验教训。

请先通读该文件了解现有结构，然后在合适位置新增一节「§5：编排实践经验」。

内容应包括以下从实际 bug 修复中提取的经验：

1. **Chain 调用必须传 `clarify: false`**：不传会导致交互式 TUI 等待确认，阻塞编排流程。

2. **`timeoutMs` 必须显式设置**：每个 subagent 调用都必须根据任务复杂度设置合理的超时值。
   - 轻量（explore/quick/librarian）: 300s
   - 中等（deep/momus/oracle/prometheus）: 600s
   - 重量（hephaestus/ultrabrain）: 900s
   - Chain：各步之和，最低 600s

3. **资源感知调度**：在发起任何 subagent tasks/chain 之前，必须先执行 `pi-resmon --recommend` 并根据 `ACTION` 字段调整并行策略。

4. **Edit 批量操作全有或全无**：调用 edit 工具时，如果 edits 数组中任意一项的 oldText 匹配失败，整个批次的编辑都不会应用（原子性）。因此分批操作时应确保每批中的 oldText 都能在文件中唯一定位。

5. **视图级按键处理完整性**：在 TUI 中，新增功能键（如 `o`, `c`）必须在所有视图处理函数中实现——列表视图 AND 阅读器视图 AND 过滤视图。常见遗漏：只在 `handleListKey` 中加了处理，忘记 `handleReaderKey`。

6. **字符串存储类型的选择**：对于需要光标编辑的输入文本，直接用 `[]rune` 存储比用 `string` + byte 索引更安全、代码更简洁。这避免了所有与 UTF-8 多字节编码相关的边界错误。

每个条目用简洁的问题描述 + 教训提炼 + 反例/正例格式。

同时：
- 更新文件头 version 号（如 1.2.0 → 1.3.0）
- 更新目录/总览部分（如有）

输出中文。

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