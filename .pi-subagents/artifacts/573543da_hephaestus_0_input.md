# Task for hephaestus

更新 ~/prompt_boilerplates/Coding/development-quality-gates.md，新增「关卡 12：字符串 / Rune 安全」。

这个关卡的来源是 news-report 项目（Go CLI 新闻聚合器）中反复出现的 bug 模式。请先通读该文件了解现有 11 个关卡的格式风格，然后在关卡 11 之后新增关卡 12。

要求：
- 保持与现有关卡一致的格式：关卡标题 → 一句话核心问题 → 具体检查清单 → 典型违规示例 → 作业要求
- 关卡 12 的内容应覆盖以下实际遇到的 bug 模式：

1. **Byte vs Rune 切片**：Go 中 `s[:n]` 是按 byte 索引，中文字符每个占 3 字节，按 byte 切片会切断多字节字符产生乱码。应该先用 `[]rune(s)` 转换再切片。

2. **Rune 安全的 cursor 定位**：在输入框中定位光标时，left/right 每次移动 1 byte 会在中文中落入字符中间，导致后续操作（退格/删除）损坏字符串。应将输入字符串存储为 `[]rune`，cursor 为 rune 索引。

3. **字符串长度语义混淆**：`len(s)` 是字节数，`len([]rune(s))` 是字符数。用于截断/显示窗口计算时必须使用后者。

4. **空格压缩破坏结构**：`strings.Join(strings.Fields(s), " ")` 会将 `\n\n` 段落分隔压缩成单空格，导致文章段落完全折叠。需用占位符保护结构标记。

5. **简繁中文搜索**：`strings.Contains` 对简繁中文视为不同字符串。需要用字符映射表（s2t/t2s）做归一化后再匹配。

6. **显示窗口容量估算**：禁止使用硬编码魔法数字（如 `*3`）估算每行可显示的字符数。应从实际终端尺寸计算：`可视行数 × (宽度 - 边距)`。

每个 bug 模式用简洁的示例代码对比（错误 vs 正确），不要长篇描述。

同时更新文件头部的 version 号（如 1.3.0 → 1.4.0）和关卡总览列表（加入关卡 12）。

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