# Task for hephaestus

更新 ~/prompt_boilerplates/Coding/interactive-cli-design.md，融入 news-report 项目的 TUI 实践经验。

请先通读该文件了解现有结构和风格，然后在以下位置做增强：

### 新增内容 A：§1 文本输入规范 — 新增 §1.6 CJK 多字节输入安全

在 §1.5 之后新增，内容：
- **核心规则**：输入框的光标定位和编辑操作必须基于 rune（字符）而非 byte（字节）
- **Rune 级 cursor**：将 filter 存储为 `[]rune`，cursor 整数为 rune 索引，left/right 移动 1 个 rune
- **退格/删除**：基于 rune 切片 (`append(s[:cursor-1], s[cursor:]...)`)，不用 byte 切片
- **插入字符**：多字节字符（如中文 3 字节）插入后 cursor += len([]rune(inserted))
- **终端列宽注意**：CJK 字符占 2 列，但 cursor 仍然按 rune 定位，显示偏移由终端处理
- 给出 Go 代码片段（错误 vs 正确）

### 新增内容 B：§2 信息密集界面 — 新增 §2.7 阅读器/文本显示窗口

内容：
- **禁止硬编码估算**：不要用 `(height-6)*3` 这种魔法数字估算每行字符数
- **正确的窗口计算**：`visLines := height - headerLines - helpLines`；整个文本预折行后按行切片
- **行级滚动模式（推荐）**：全文按终端宽度预折行 → 拆分为 `[]string` → 按行索引偏移
  - 好处：滚动步长精确对齐屏幕行，无需估算
  - 窗口缩放时需重新折行（在 `tea.WindowSizeMsg` 中处理）
- **段落保护**：压缩空白前用占位符保护 `\n\n`
- **scrollDown 上限**：`max(0, len(lines)-1)`，不是 `len(rawBody)`
- Go 代码对比（错误 vs 正确）

### 新增内容 C：§2.3 搜索 — 扩展搜索小节

在现有 §2.3 后面追加简繁一致性子节：
- **简繁一致性**：中文字符映射表（s2t/t2s）归一化搜索词和文本后再匹配
- **双向匹配**：先原始匹配，失败后双方归一化（通常统一转简体）再匹配
- 给出 Go 代码示例

### 新增内容 D：新 §6 外部工具集成

在 §5（验收清单）之前新增 §6，内容：
- **从 TUI 打开浏览器 / 复制到剪贴板**：需重定向 stdin/stdout/stderr 到 `/dev/null` 避免 SIGPIPE
- **剪贴板自动检测**：Wayland → `wl-copy`，X11 → `xclip`，macOS → `pbcopy`
- **按键覆盖检查**：每个视图（列表/阅读/过滤/弹窗）都必须处理相同的快捷键（如 `o`, `c`）

### 变更清单
- 所有新增章节保持与现有格式一致的风格
- 更新文件头 version 号（如 1.1.0 → 1.2.0）
- 更新 §5 验收清单：新增针对 CJK 安全、行级滚动、外部工具的检查项
- 更新交叉引用表

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