package tui

import (
	"fmt"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"news-report/internal/classify"
	"news-report/internal/config"
	"news-report/internal/fetch"
	"news-report/internal/report"
)

func sampleModel() Model {
	cfg := config.Default()
	now := time.Now()
	rep := &report.Report{
		Generated: now,
		Config:    cfg,
		Items: []report.Item{
			{Title: "Senate passes bill", URL: "https://a.example/1", SourceName: "NPR", Lang: "en",
				Category: classify.USPolitics, AgeLabel: "2h", Score: 1.0, Published: now},
			{Title: "Trump signs executive order", URL: "https://a.example/4", SourceName: "NPR", Lang: "en",
				Category: classify.USPolitics, AgeLabel: "1h", Score: 0.95, Published: now},
			{Title: "EU sanctions package", URL: "https://a.example/2", SourceName: "BBC", Lang: "en",
				Category: classify.Politics, AgeLabel: "3h", Score: 0.9, Published: now},
			{Title: "台積電擴產", URL: "https://a.example/3", SourceName: "中央社", Lang: "zh",
				Category: classify.Industry, AgeLabel: "1h", Score: 0.8, Published: now},
		},
		SourceStats: []report.SourceStat{{ID: "npr", OK: true}},
	}
	m := Model{cfg: cfg, fetcher: fetch.New(fetch.Options{NoRobots: true}), loading: false}
	m.rep = rep
	m.buildTabs()
	return m
}

func TestBuildTabs(t *testing.T) {
	m := sampleModel()
	if len(m.cats) != 3 {
		t.Fatalf("应有 3 个分类 Tab，实际 %d: %v", len(m.cats), m.cats)
	}
	// 按配置顺序：uspolitics 在前
	if m.cats[0] != classify.USPolitics {
		t.Errorf("第一个 Tab 应为 uspolitics，实际 %v", m.cats[0])
	}
}

func TestListNavigation(t *testing.T) {
	m := sampleModel()
	// 向下移动（uspolitics 有 2 条）
	m2 := asModel(m.handleListKey(tea.KeyMsg{Type: tea.KeyDown}))
	if m2.cursor != 1 {
		t.Errorf("向下移动后 cursor 应为 1，实际 %d", m2.cursor)
	}
	// 到底后不再移动
	m2b := asModel(m2.handleListKey(tea.KeyMsg{Type: tea.KeyDown}))
	if m2b.cursor != 1 {
		t.Errorf("cursor 不应越过末尾，实际 %d", m2b.cursor)
	}
	// 切 Tab
	m3 := asModel(m.handleListKey(tea.KeyMsg{Type: tea.KeyTab}))
	if m3.tab != 1 {
		t.Errorf("Tab 切换后应为 1，实际 %d", m3.tab)
	}
	if m3.cursor != 0 {
		t.Errorf("切 Tab 后 cursor 应重置为 0，实际 %d", m3.cursor)
	}
	// 循环
	m4 := asModel(m3.handleListKey(tea.KeyMsg{Type: tea.KeyTab}))
	m4 = asModel(m4.handleListKey(tea.KeyMsg{Type: tea.KeyTab}))
	if m4.tab != 0 {
		t.Errorf("Tab 应循环回 0，实际 %d", m4.tab)
	}
}

func TestFilter(t *testing.T) {
	m := sampleModel()
	// 切到 Politics tab（含 EU sanctions 条目）
	m = asModel(m.handleListKey(tea.KeyMsg{Type: tea.KeyTab}))
	// 进入过滤
	m2 := asModel(m.handleListKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}}))
	if m2.view != viewFilter {
		t.Fatalf("按 / 应进入过滤视图")
	}
	// 输入关键词
	m3 := asModel(m2.handleFilterKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'E', 'U'}}))
	if string(m3.filter) != "EU" {
		t.Errorf("过滤词错误: %q", string(m3.filter))
	}
	// 确认过滤
	m4 := asModel(m3.handleFilterKey(tea.KeyMsg{Type: tea.KeyEnter}))
	if m4.view != viewList {
		t.Errorf("Enter 应返回列表视图")
	}
	items := m4.visibleItems()
	if len(items) != 1 || items[0].Title != "EU sanctions package" {
		t.Errorf("过滤结果错误: %+v", items)
	}
}

func TestReaderFlow(t *testing.T) {
	m := sampleModel()
	// 选中条目进入阅读
	m2, cmd := m.handleListKey(tea.KeyMsg{Type: tea.KeyEnter})
	if m2.(Model).view != viewReader {
		t.Fatalf("Enter 应进入阅读视图")
	}
	if cmd == nil {
		t.Error("阅读视图应发起全文加载命令")
	}
	// Esc 返回
	m3 := asModel(m2.(Model).handleReaderKey(tea.KeyMsg{Type: tea.KeyEsc}))
	if m3.view != viewList {
		t.Errorf("Esc 应返回列表")
	}
}

func TestViewRenders(t *testing.T) {
	m := sampleModel()
	m.width, m.height = 100, 30
	v := m.View()
	// 当前 tab 条目 + 所有 tab 标签
	for _, want := range []string{"news-report", "USPOLITICS", "POLITICS", "INDUSTRY", "Senate passes bill"} {
		if !strings.Contains(v, want) {
			t.Errorf("列表视图缺少 %q", want)
		}
	}
	// 其他分类条目不在当前视图（按 tab 渲染）
	if strings.Contains(v, "台積電擴產") {
		t.Error("Industry 条目不应出现在 uspolitics 视图")
	}
	// 切到 Industry tab 后应出现
	m2 := asModel(m.handleListKey(tea.KeyMsg{Type: tea.KeyTab}))
	m2 = asModel(m2.handleListKey(tea.KeyMsg{Type: tea.KeyTab}))
	if !strings.Contains(m2.View(), "台積電擴產") {
		t.Error("Industry tab 应显示台積電條目")
	}
	if !strings.Contains(v, "Enter 阅读") {
		t.Errorf("帮助栏缺失: %q", v)
	}
}

func asModel(m tea.Model, _ tea.Cmd) Model { return m.(Model) }

func TestCatCount(t *testing.T) {
	m := sampleModel()
	if n := m.catCount(classify.USPolitics); n != 2 {
		t.Errorf("uspolitics 计数应为 2，实际 %d", n)
	}
}

func TestQuit(t *testing.T) {
	m := sampleModel()
	_, cmd := m.handleListKey(tea.KeyMsg{Type: tea.KeyCtrlC})
	if cmd == nil {
		t.Error("Ctrl+C 应产生退出命令")
	}
}

func TestScroll(t *testing.T) {
	// 基于行的滚动：10 行文本，offset 从 5 开始
	r := &readerState{body: "line0\nline1\nline2\nline3\nline4\nline5\nline6\nline7\nline8\nline9",
		lines:  []string{"line0", "line1", "line2", "line3", "line4", "line5", "line6", "line7", "line8", "line9"},
		offset: 5}
	r.scrollUp(2, 10)
	if r.offset != 3 {
		t.Errorf("上滚 2 行后 offset 应为 3，实际 %d", r.offset)
	}
	r.scrollUp(100, 10)
	if r.offset != 0 {
		t.Errorf("上滚不应越界，实际 %d", r.offset)
	}
	r.scrollDown(1000, 10)
	if r.offset != 9 {
		t.Errorf("下滚不应越界到末行 9，实际 %d", r.offset)
	}
}

func TestSplitWrapped(t *testing.T) {
	lines := splitWrapped("hello world\n\nbye", 80)
	// 应包含 "hello world", "", "bye"
	if len(lines) < 3 {
		t.Fatalf("splitWrapped 应保留空行: %v", lines)
	}
	if lines[0] != "hello world" || lines[1] != "" || lines[2] != "bye" {
		t.Errorf("splitWrapped 结果不符: %v", lines)
	}
}

func TestReaderCacheHit(t *testing.T) {
	m := sampleModel()
	m.articleCache = map[string]string{"https://a.example/1": "cached body text"}
	// 选中第一条目按 Enter
	m2, cmd := m.handleListKey(tea.KeyMsg{Type: tea.KeyEnter})
	m = m2.(Model)
	if m.view != viewReader {
		t.Fatal("应进入阅读视图")
	}
	// 缓存命中时不应发起 fetch 命令
	if cmd != nil {
		t.Error("缓存命中时不应发起抓取")
	}
	if m.reader.body != "cached body text" {
		t.Errorf("应显示缓存正文，实际 %q", m.reader.body)
	}
	if len(m.reader.lines) == 0 {
		t.Error("缓存命中时也应预折行")
	}
}

func TestFilterCursorNavigation(t *testing.T) {
	m := sampleModel()
	m = asModel(m.handleListKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}}))
	// type "ab"
	m = asModel(m.handleFilterKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a', 'b'}}))
	if string(m.filter) != "ab" || m.filterCursor != 2 {
		t.Errorf("输入后 filter=%q cursor=%d", string(m.filter), m.filterCursor)
	}
	// left arrow
	m = asModel(m.handleFilterKey(tea.KeyMsg{Type: tea.KeyLeft}))
	if m.filterCursor != 1 {
		t.Errorf("left 后 cursor=1, 实际 %d", m.filterCursor)
	}
	// insert at cursor
	m = asModel(m.handleFilterKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'x'}}))
	if string(m.filter) != "axb" || m.filterCursor != 2 {
		t.Errorf("插入后 filter=%q cursor=%d", string(m.filter), m.filterCursor)
	}
	// backspace at cursor
	m = asModel(m.handleFilterKey(tea.KeyMsg{Type: tea.KeyBackspace}))
	if string(m.filter) != "ab" || m.filterCursor != 1 {
		t.Errorf("backspace 后 filter=%q cursor=%d", string(m.filter), m.filterCursor)
	}
	// home
	m = asModel(m.handleFilterKey(tea.KeyMsg{Type: tea.KeyHome}))
	if m.filterCursor != 0 {
		t.Errorf("home 后 cursor=0, 实际 %d", m.filterCursor)
	}
	// Ctrl+U (delete to start) — cursor 在 0，无可删内容
	m = asModel(m.handleFilterKey(tea.KeyMsg{Type: tea.KeyCtrlU}))
	if string(m.filter) != "ab" || m.filterCursor != 0 {
		t.Errorf("Ctrl+U 后 filter=%q cursor=%d", string(m.filter), m.filterCursor)
	}
	// 移动光标到末尾再 Ctrl+U（应清空）
	m = asModel(m.handleFilterKey(tea.KeyMsg{Type: tea.KeyEnd}))
	m = asModel(m.handleFilterKey(tea.KeyMsg{Type: tea.KeyCtrlU}))
	if len(m.filter) != 0 || m.filterCursor != 0 {
		t.Errorf("Ctrl+U at end 后 filter=%q cursor=%d", string(m.filter), m.filterCursor)
	}
}

func TestFilterCtrlW(t *testing.T) {
	m := sampleModel()
	m = asModel(m.handleListKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}}))
	// type "hello world"
	for _, r := range "hello world test" {
		m = asModel(m.handleFilterKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}}))
	}
	// cursor at end, Ctrl+W deletes "test" (last word)
	m = asModel(m.handleFilterKey(tea.KeyMsg{Type: tea.KeyCtrlW}))
	if string(m.filter) != "hello world " || m.filterCursor != 12 {
		t.Errorf("Ctrl+W 后 filter=%q cursor=%d", string(m.filter), m.filterCursor)
	}
}

func TestFilterEscape(t *testing.T) {
	m := sampleModel()
	m = asModel(m.handleListKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}}))
	m = asModel(m.handleFilterKey(tea.KeyMsg{Type: tea.KeyEsc}))
	if m.view != viewList || len(m.filter) != 0 {
		t.Errorf("Esc 应退出过滤: view=%v filter=%q", m.view, string(m.filter))
	}
}

func TestNextFilterMatch(t *testing.T) {
	m := sampleModel()
	// 在 uspolitics tab（2 条：Senate passes bill, Trump signs executive order）
	// 添加过滤
	m = asModel(m.handleListKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}}))
	m = asModel(m.handleFilterKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'T', 'r', 'u', 'm', 'p'}}))
	m = asModel(m.handleFilterKey(tea.KeyMsg{Type: tea.KeyEnter}))

	// 过滤后仅 1 条（Trump signs executive order），n/N 应循环到自身
	if len(m.visibleItems()) != 1 {
		t.Fatalf("过滤后应为 1 条，实际 %d", len(m.visibleItems()))
	}
	// n 推进
	n := nextFilterMatch(m, 0, +1)
	if n != 0 {
		t.Errorf("单条时 n 应回到 0，实际 %d", n)
	}
	// N 回退
	n = nextFilterMatch(m, 0, -1)
	if n != 0 {
		t.Errorf("单条时 N 应回到 0，实际 %d", n)
	}

	// 多条：切到 politics tab（1 条），过滤为空时 n/N
	m2 := sampleModel()
	m2 = asModel(m2.handleListKey(tea.KeyMsg{Type: tea.KeyTab})) // -> politics
	m2 = asModel(m2.handleListKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}}))
	m2 = asModel(m2.handleFilterKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'E', 'U'}}))
	m2 = asModel(m2.handleFilterKey(tea.KeyMsg{Type: tea.KeyEnter}))
	// 1 条过滤结果，n/N 循环
	if nextFilterMatch(m2, 0, +1) != 0 || nextFilterMatch(m2, 0, -1) != 0 {
		t.Error("1 条时 n/N 应均回 0")
	}

	// 空过滤
	m3 := sampleModel()
	m3 = asModel(m3.handleListKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}}))
	m3 = asModel(m3.handleFilterKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'z', 'z', 'z'}}))
	m3 = asModel(m3.handleFilterKey(tea.KeyMsg{Type: tea.KeyEnter}))
	if len(m3.visibleItems()) != 0 {
		t.Fatal("应无匹配条目")
	}
	// 空时 nextFilterMatch 应保持原 cursor
	if nextFilterMatch(m3, 5, +1) != 5 {
		t.Error("空列表时不应改变 cursor")
	}
}

func TestMatchFilterChinese(t *testing.T) {
	// 简繁一致性：搜索简体应找到繁体标题
	if !matchFilter("國際新聞", "国际") {
		t.Error("search '国际' should match '國際新聞'")
	}
	if !matchFilter("台湾经济", "臺灣經濟") {
		t.Error("search '臺灣經濟' should match '台湾经济'")
	}
	if !matchFilter("选举结果", "選舉") {
		t.Error("search '選舉' should match '选举结果'")
	}
	// 普通英文
	if !matchFilter("Hello World", "hello") {
		t.Error("search 'hello' should match 'Hello World'")
	}
	// 无匹配
	if matchFilter("國際新聞", "xyz") {
		t.Error("search 'xyz' should not match '國際新聞'")
	}
}

func TestWrapCellsCJK(t *testing.T) {
	// CJK 文本不应被截断为非法 UTF-8
	// 8 个字 = 16 列，按 4 列折行 → 每行 2 个字，共 4 行
	input := "台積電擴計畫預計"
	got := wrapCells(input, 4)
	if strings.Contains(got, "\ufffd") {
		t.Errorf("wrapCells 不应产生替换字符: %q", got)
	}
	lines := strings.Split(strings.TrimRight(got, "\n"), "\n")
	if len(lines) != 4 {
		t.Fatalf("8 字 16 列按 4 列折行应有 4 行，实际 %d: %q", len(lines), lines)
	}
	for i, line := range lines {
		if cellWidth(line) > 4 {
			t.Errorf("第 %d 行超过 4 列: %q (%d 列)", i+1, line, cellWidth(line))
		}
	}
}

func TestWrapCellsMixed(t *testing.T) {
	// 混合中英文 + 换行保留
	input := "line one\nline two"
	got := wrapCells(input, 80)
	if !strings.Contains(got, "line one\nline two") {
		t.Errorf("wrapCells 应保留换行: %q", got)
	}
	// 中文+英文混合折行：宽度 8 列，"hello " 6 列 + 你 2 列 = 8
	lines := splitWrapped("hello 你好世界", 8)
	if len(lines) != 2 || lines[0] != "hello" || lines[1] != "你好世界" {
		t.Errorf("混合折行错误: %v", lines)
	}
}

func TestWrapCellsLongWord(t *testing.T) {
	// 超长词（如 URL）按列硬断
	lines := splitWrapped("https://example.com/a/really/long/path", 10)
	if len(lines) < 4 {
		t.Fatalf("长 URL 应硬断多行，实际 %d: %v", len(lines), lines)
	}
	for _, l := range lines {
		if cellWidth(l) > 10 {
			t.Errorf("硬断行超宽: %q (%d 列)", l, cellWidth(l))
		}
	}
}

func TestTruncateCells(t *testing.T) {
	cases := []struct {
		in   string
		max  int
		want string
	}{
		{"hello", 5, "hello"},
		{"hello", 4, "hel…"},
		{"你好世界", 5, "你好…"}, // 你好=4列 + …=1列
		{"你好世界", 6, "你好…"}, // 世=2列放不下，让位省略号
		{"ab", 1, "…"},
		{"a", 1, "a"},
		{"", 5, ""},
	}
	for _, c := range cases {
		got := truncateCells(c.in, c.max)
		if got != c.want {
			t.Errorf("truncateCells(%q, %d) = %q，期望 %q", c.in, c.max, got, c.want)
		}
		if cellWidth(got) > c.max {
			t.Errorf("truncateCells(%q, %d) 结果超宽: %q (%d 列)", c.in, c.max, got, cellWidth(got))
		}
	}
}

func TestSliceCells(t *testing.T) {
	cases := []struct {
		s        string
		from, to int
		want     string
	}{
		{"abc", 1, 3, "bc"},
		{"你a好", 0, 2, "你"},  // 你占 0-2 列
		{"你a好", 2, 3, "a"},  // a 占 2-3 列
		{"你a好", 2, 5, "a好"}, // a + 好(2列)
		{"你a好", 1, 4, "a"},  // 跨边界的 你 整体丢弃
		{"你好世界", 0, 3, "你"}, // 3 列内只放得下 你(2列)
		{"", 0, 5, ""},
		{"abc", 5, 10, ""},
	}
	for _, c := range cases {
		got := sliceCells(c.s, c.from, c.to)
		if got != c.want {
			t.Errorf("sliceCells(%q, %d, %d) = %q，期望 %q", c.s, c.from, c.to, got, c.want)
		}
	}
}

func TestPadCells(t *testing.T) {
	if got := padCells("你好", 6); cellWidth(got) != 6 || !strings.HasSuffix(got, "  ") {
		t.Errorf("padCells(你好, 6) = %q (%d 列)", got, cellWidth(got))
	}
	if got := padCells("abcdef", 4); got != "abc…" {
		t.Errorf("padCells 超长应截断: %q", got)
	}
}

func TestSanitizeLLMText(t *testing.T) {
	in := "## 背景\n**要点一**\n- 项目A\n- 项目B\n```\ncode\n```\n`引用`"
	got := sanitizeLLMText(in)
	for _, bad := range []string{"##", "**", "```", "`"} {
		if strings.Contains(got, bad) {
			t.Errorf("sanitizeLLMText 未清除 %q: %q", bad, got)
		}
	}
	if !strings.Contains(got, "背景\n要点一\n• 项目A\n• 项目B") {
		t.Errorf("sanitizeLLMText 结构错误: %q", got)
	}
}

// TestOverlayPopupCJKAlignment 验证弹窗在 CJK 底图/标题下边框对齐、行宽不超终端。
func TestOverlayPopupCJKAlignment(t *testing.T) {
	base := "📰 news-report 2026-08-14 ｜ 3 来源 · 4 条\n" +
		" USPOLITICS (2)  POLITICS (1)\n" +
		"● 美国参议院通过重大预算改革法案 — NPR · 2h · EN\n" +
		"● 台積電擴產計畫正式啟動 — 中央社 · 1h · ZH\n" +
		"● EU approves new sanctions package — BBC · 3h · EN\n"
	p := &popupState{
		title:   "解读: 美国政府对中国留学生实习限制升级 科技公司纷纷调整招聘政策",
		content: "## 背景\n美国国务院宣布将积极撤销部分中国学生签证。\n\n## 影响\n科技公司实习岗位招聘收紧，芯片与AI领域最明显。\n",
	}
	termW, termH := 60, 20
	out := overlayPopup(base, p, termW, termH)
	rows := strings.Split(strings.TrimRight(out, "\n"), "\n")
	// 弹窗行数 = 标题 1 + 内容 contentH + 帮助 1 + 底边 1；底图不足行时末尾空行被 Trim 掉
	if len(rows) > termH {
		t.Fatalf("输出行数 %d 超过终端 %d", len(rows), termH)
	}
	popupRowCount := 0
	for _, row := range rows {
		if strings.ContainsAny(stripANSI(row), "┌│└") {
			popupRowCount++
		}
	}
	wantPopupRows := 1 + popupContentH(termH) + 2
	if popupRowCount != wantPopupRows {
		t.Fatalf("弹窗应有 %d 行，实际 %d", wantPopupRows, popupRowCount)
	}

	// 弹窗宽度：70% of 60 = 42 列，居中 startCol = 9
	pw := 42
	startCol := (termW - pw) / 2
	var popupLeft, popupRight int
	found := false
	for _, row := range rows {
		plain := stripANSI(row)
		if w := cellWidth(plain); w > termW {
			t.Errorf("行宽 %d 超过终端 %d: %q", w, termW, plain)
		}
		hasLeft := strings.ContainsAny(plain, "┌│└")
		hasRight := strings.ContainsAny(plain, "┐│┘")
		if hasLeft && hasRight {
			// IndexAny 返回字节偏移，须换算为显示列
			l := cellWidth(plain[:strings.IndexAny(plain, "┌│└")])
			r := cellWidth(plain[:strings.LastIndexAny(plain, "┐│┘")])
			if !found {
				popupLeft, popupRight, found = l, r, true
			}
			if l != popupLeft || r != popupRight {
				t.Errorf("弹窗边框未对齐: 左 %d/%d 右 %d/%d 行=%q", l, popupLeft, r, popupRight, plain)
			}
		}
	}
	if !found {
		t.Fatal("未找到弹窗边框行")
	}
	if popupLeft != startCol || popupRight-popupLeft+1 != pw {
		t.Errorf("弹窗位置/宽度错误: 左=%d 右=%d（期望左=%d 宽=%d）", popupLeft, popupRight, startCol, pw)
	}
	// 标题栏内 dash 填充必须恰好到达 ┐ 前一列
	for _, row := range rows {
		plain := stripANSI(row)
		if strings.Contains(plain, "┌") && strings.Contains(plain, "┐") {
			if got := cellWidth(plain[:strings.Index(plain, "┐")+1]); got != popupRight+1 {
				t.Errorf("标题栏宽度错误 %d != %d: %q", got, popupRight+1, plain)
			}
		}
	}
}

// TestOverlayPopupOffsetClamp 验证滚动偏移渲染时被钳位。
func TestOverlayPopupOffsetClamp(t *testing.T) {
	var lines []string
	for i := 0; i < 30; i++ {
		lines = append(lines, fmt.Sprintf("第 %d 行", i))
	}
	p := &popupState{title: "T", content: strings.Join(lines, "\n"), lines: lines, offset: 25}
	// 渲染后 offset 应被钳位到 len(30)-contentH
	out := overlayPopup("base\n", p, 60, 20)
	_ = out
	// contentH: ph=16, contentH=12 → maxOff=18
	if p.offset != 18 {
		t.Errorf("offset 应钳位到 18，实际 %d", p.offset)
	}
}

func TestPopupScrollClamp(t *testing.T) {
	p := &popupState{lines: nil}
	popupScroll(p, 5, 10)
	if p.offset != 0 {
		t.Errorf("空 lines 时 offset 应为 0，实际 %d", p.offset)
	}
	p.lines = make([]string, 20)
	popupScroll(p, 100, 10)
	if p.offset != 10 {
		t.Errorf("下滚应钳位到 20-10=10，实际 %d", p.offset)
	}
	popupScroll(p, -100, 10)
	if p.offset != 0 {
		t.Errorf("上滚应钳位到 0，实际 %d", p.offset)
	}
	if got := popupMaxOffset(p, 10); got != 10 {
		t.Errorf("popupMaxOffset 应为 10，实际 %d", got)
	}
	p.lines = make([]string, 5)
	if got := popupMaxOffset(p, 10); got != 0 {
		t.Errorf("内容不足一屏时 maxOffset 应为 0，实际 %d", got)
	}
}

// TestReaderViewNoOverflow 验证窄终端下阅读器所有行不超终端宽度。
func TestReaderViewNoOverflow(t *testing.T) {
	m := sampleModel()
	m.width, m.height = 40, 20
	longTitle := "美国政府对中国留学生的实习签证限制持续升级引发科技行业广泛担忧"
	longURL := "https://example.com/very/long/path/to/an/article/page"
	body := "中文正文內容重複中文正文內容重複中文正文內容重複中文正文內容重複中文正文內容"
	m.view = viewReader
	m.reader = &readerState{
		item:  &report.Item{Title: longTitle, URL: longURL, SourceName: "中央社", Lang: "zh", AgeLabel: "2h", Published: time.Now()},
		body:  body,
		lines: splitWrapped(body, readerWidth(m.width)),
	}
	v := m.View()
	for _, row := range strings.Split(strings.TrimRight(v, "\n"), "\n") {
		plain := stripANSI(row)
		if w := cellWidth(plain); w > m.width {
			t.Errorf("阅读器行宽 %d 超过终端 %d: %q", w, m.width, plain)
		}
	}
}

// TestHelpViewAlign 验证帮助视图两列对齐（ANSI 不参与宽度计算）。
func TestHelpViewAlign(t *testing.T) {
	m := sampleModel()
	m.width, m.height = 80, 30
	m.showHelp = true
	v := m.View()
	rows := strings.Split(strings.TrimRight(v, "\n"), "\n")
	descs := []string{"切换分类", "选择条目", "阅读全文", "搜索过滤", "浏览器打开", "翻译当前正文",
		"生成 5 点摘要", "深度解读", "导出当前分类", "重新抓取", "本帮助", "阅读器翻页", "阅读器首/尾"}
	var cols []int
	for _, desc := range descs {
		for _, row := range rows {
			plain := stripANSI(row)
			if i := strings.Index(plain, desc); i >= 0 {
				cols = append(cols, cellWidth(plain[:i]))
				break
			}
		}
	}
	if len(cols) < 5 {
		t.Fatalf("找到的帮助行数不足: %d", len(cols))
	}
	for _, c := range cols[1:] {
		if c != cols[0] {
			t.Errorf("帮助描述列未对齐: %d != %d", c, cols[0])
		}
	}
}

func TestHelpOverlay(t *testing.T) {
	m := sampleModel()
	m = asModel(m.handleListKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'?'}}))
	if !m.showHelp {
		t.Error("? 应打开帮助")
	}
	v := m.View()
	for _, want := range []string{"键位帮助", "切换分类", "Enter", "浏览器打开"} {
		if !strings.Contains(v, want) {
			t.Errorf("帮助视图缺少 %q", want)
		}
	}
	m = asModel(m.handleListKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'?'}}))
	if m.showHelp {
		t.Error("再次 ? 应关闭帮助")
	}
}
