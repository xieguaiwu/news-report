# Task for hephaestus

修复 news-report 项目 TUI 弹窗（popup/overlayPopup）的排版残留 bug。

项目路径：/home/xieguiawu/Desktop/go-projects/news-report
主文件：internal/tui/tui.go

需要修复的 3 个问题：

### Bug 1：终端缩放时弹窗不复原
当用户在弹窗打开时缩放终端，`WindowSizeMsg` 更新 `m.width/m.height` 并重折行 reader，但不清理弹窗的 `p.lines`。下次渲染时 `overlayPopup` 看到 `len(p.lines) > 0`，使用旧宽度的折行结果，导致弹窗宽度不对。

修复：在 `WindowSizeMsg` 处理中，如果 `m.popup != nil`，清除 `m.popup.lines = nil`。

### Bug 2：帮助栏文本可能溢出
`drawLine("Esc 关闭  ↑↓ 滚动  PgUp/PgDn 翻页")` 的文本约 30 个字符。如果弹窗很窄（如 `contentW < 30`），帮助文本超出边框。虽然这种情况罕见（弹窗最小 40），但应做截断保护。

修复：在 `drawLine` 内或调用前检查宽度，超长时截断。

### Bug 3：标题栏宽度与底边框不对齐
标题栏计算：`contentW - titleW - 1` 个 `─`，加上 `"┌─ " + title + " "` = 4 + titleW + rem = contentW + 3 个字符。
底边框：`"└" + strings.Repeat("─", contentW) + "┘"` = contentW + 2 个字符。
内容行：`"│ " + text(padded to contentW) + " │"` = contentW + 4 个字符。

三行宽度不一致！标题栏=contentW+3，底边框=contentW+2，内容行=contentW+4。

修复：统一使用 `contentW + 4` 宽度（`"│ " + content + " │"`的格式）。标题栏应为 `"┌─ " + title + " " + dashes + "┐"` 总宽 contentW+4。底边框应为 `"└" + dashes + "┘"` 总宽 contentW+4。

### 要求：
- 先 read 文件理解上下文，然后 edit 精确修改
- 写完编译验证：go build ./...
- 运行测试：go test -race ./internal/tui/ -count=1
- 部署：make install
- 不要重写整个函数，只做精准修改
- 输出中文简要说明修改了什么

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