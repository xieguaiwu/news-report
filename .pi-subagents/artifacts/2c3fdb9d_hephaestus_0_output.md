无暂存文件。修复完成，以下是摘要和验收报告。

---

## 修改摘要

### Bug 1 — 终端缩放时弹窗不复原
在 `WindowSizeMsg` handler 中，缩放终端时 `m.reader.lines` 重折行后，新增 `if m.popup != nil { m.popup.lines = nil }`。下次 `overlayPopup` 渲染时发现 `lines == nil`，按新弹窗宽度重新折行，避免旧宽度残留导致排版错位。

### Bug 2 — 帮助栏文本溢出保护
`drawLine` 闭包原先未处理 `text` 宽度 > `contentW` 的情况，`rem` 为负时既不补齐也不截断，文本撑出右侧边框。修复为：先计算 `lipgloss.Width(text)`，超出 `contentW` 时按 rune 逐个累加宽度，在超限处截断，保证 `"│ text │"` 不越界。

### Bug 3 — 底边框宽度不对齐
底边框原本 `"└" + Repeat("─", contentW) + "┘"` 总宽 `contentW + 2`，而内容行和标题栏均为 `contentW + 4`。改为 `Repeat("─", contentW + 2)` 使底边框补齐至 `contentW + 4`，与标题栏、内容行统一对齐。