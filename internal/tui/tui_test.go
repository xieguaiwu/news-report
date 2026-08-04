package tui

import (
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

func TestWrapLinesCJK(t *testing.T) {
	// CJK 文本不应被截断为非法 UTF-8
	// 8 字以 4 宽折行
	input := "台積電擴計畫預計"
	got := wrapLines(input, 4)
	// 不应出现替换字符（U+FFFD）
	if strings.Contains(got, "\ufffd") {
		t.Errorf("wrapLines 不应产生替换字符: %q", got)
	}
	// ceil(8/4) = 2 行
	lines := strings.Split(strings.TrimRight(got, "\n"), "\n")
	if len(lines) != 2 {
		t.Errorf("8 字以 4 宽折行应有 2 行，实际 %d: %q", len(lines), lines)
	}
	// 验证每行不超过 4 个 rune
	for i, line := range lines {
		if len([]rune(line)) > 4 {
			t.Errorf("第 %d 行超过 4 字: %q", i+1, line)
		}
	}
}

func TestWrapLinesMixed(t *testing.T) {
	// 混合中英文 + 换行保留
	input := "line one\nline two"
	got := wrapLines(input, 80)
	if !strings.Contains(got, "line one\nline two") {
		t.Errorf("wrapLines 应保留换行: %q", got)
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
