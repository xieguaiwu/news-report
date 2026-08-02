// Package tui 提供交互式终端界面（bubbletea）：
// 分类 Tab 导航 → 条目列表 → Enter 全文阅读 / o 浏览器打开 / / 搜索过滤 / s 导出 / r 刷新。
package tui

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"news-report/internal/article"
	"news-report/internal/classify"
	"news-report/internal/config"
	"news-report/internal/fetch"
	"news-report/internal/report"
)

// ── 样式 ─────────────────────────────────────────────────────

var (
	styleTitle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("212")).
			Padding(0, 1)
	styleTabSel = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("0")).
			Background(lipgloss.Color("212")).Padding(0, 1)
	styleTab    = lipgloss.NewStyle().Foreground(lipgloss.Color("244")).Padding(0, 1)
	styleItem   = lipgloss.NewStyle().Padding(0, 1)
	styleCursor = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("212"))
	styleDim    = lipgloss.NewStyle().Foreground(lipgloss.Color("245"))
	styleHelp   = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	styleErr    = lipgloss.NewStyle().Foreground(lipgloss.Color("196"))
	styleReader = lipgloss.NewStyle().Padding(0, 2)
)

var catColor = map[classify.Category]lipgloss.Color{
	classify.USPolitics: lipgloss.Color("205"),
	classify.Politics:   lipgloss.Color("39"),
	classify.Economy:    lipgloss.Color("42"),
	classify.Industry:   lipgloss.Color("220"),
	classify.Other:      lipgloss.Color("245"),
}

// ── 模型 ─────────────────────────────────────────────────────

type view int

const (
	viewList view = iota
	viewReader
	viewFilter
)

// Model 是 TUI 根模型。
type Model struct {
	cfg     *config.Config
	fetcher *fetch.Fetcher
	opts    report.Options

	rep    *report.Report
	cats   []classify.Category // Tab 顺序（配置分类 + other）
	tab    int
	cursor int

	filter    string
	filtering bool

	view   view
	reader *readerState

	err     string
	status  string
	loading bool
	width   int
	height  int
}

type readerState struct {
	item   *report.Item
	body   string
	offset int
	err    string
}

// Run 启动 TUI（阻塞直到退出）。
func Run(ctx context.Context, cfg *config.Config, fetcher *fetch.Fetcher, opts report.Options) error {
	m := New(cfg, fetcher, opts)
	p := tea.NewProgram(m, tea.WithAltScreen())
	_, err := p.Run()
	return err
}

// New 创建模型（视图层会立即发起异步抓取）。
func New(cfg *config.Config, fetcher *fetch.Fetcher, opts report.Options) Model {
	return Model{
		cfg:     cfg,
		fetcher: fetcher,
		opts:    opts,
		loading: true,
	}
}

// Init 启动时发起抓取。
func (m Model) Init() tea.Cmd {
	return fetchReportCmd(m.cfg, m.fetcher, m.opts)
}

// ── 消息与命令 ────────────────────────────────────────────────

type reportMsg struct {
	rep *report.Report
	err error
}

type readerMsg struct {
	idx  int
	body string
	err  error
}

type saveMsg struct {
	path string
	err  error
}

func fetchReportCmd(cfg *config.Config, f *fetch.Fetcher, opts report.Options) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
		defer cancel()
		if len(opts.Languages) == 0 {
			opts.Languages = cfg.Languages
		}
		if len(opts.Categories) == 0 {
			opts.Categories = cfg.Categories
		}
		if opts.Window == 0 {
			opts.Window = cfg.Window()
		}
		if opts.LimitPerCat <= 0 {
			opts.LimitPerCat = cfg.LimitPerCat
		}
		if opts.TotalLimit <= 0 {
			opts.TotalLimit = cfg.TotalLimit
		}
		opts.StrictFocus = cfg.StrictFocus || opts.StrictFocus
		opts.ShowSeen = true // TUI 中始终显示全部（用户自己导航）
		rep, err := report.Run(ctx, cfg, f, opts)
		return reportMsg{rep: rep, err: err}
	}
}

func fetchReaderCmd(f *fetch.Fetcher, item *report.Item) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		art, err := article.Extract(ctx, f, item.URL, item.Lang)
		if err != nil {
			return readerMsg{body: "", err: err}
		}
		return readerMsg{body: art.Text}
	}
}

func saveCmd(m Model) tea.Cmd {
	return func() tea.Msg {
		path := fmt.Sprintf("news-report-%s-%s.md", m.currentCat(), time.Now().Format("2006-01-02"))
		// 保存当前分类条目为 Markdown
		var sb strings.Builder
		writeCategoryMarkdown(&sb, m, m.currentCat())
		if err := os.WriteFile(path, []byte(sb.String()), 0o644); err != nil {
			return saveMsg{err: err}
		}
		return saveMsg{path: path}
	}
}

// ── Update ───────────────────────────────────────────────────

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		return m, nil

	case reportMsg:
		m.loading = false
		if msg.err != nil {
			m.err = msg.err.Error()
			return m, nil
		}
		m.rep = msg.rep
		m.buildTabs()
		return m, nil

	case readerMsg:
		if m.reader != nil {
			if msg.err != nil {
				m.reader.err = msg.err.Error()
			} else {
				m.reader.body = msg.body
				m.reader.err = ""
			}
		}
		return m, nil

	case saveMsg:
		if msg.err != nil {
			m.status = "保存失败: " + msg.err.Error()
		} else {
			m.status = "已保存: " + msg.path
		}
		return m, nil

	case tea.KeyMsg:
		return m.handleKey(msg)
	}
	return m, nil
}

func (m Model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch m.view {
	case viewFilter:
		return m.handleFilterKey(msg)
	case viewReader:
		return m.handleReaderKey(msg)
	default:
		return m.handleListKey(msg)
	}
}

func (m Model) handleFilterKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc", "enter":
		m.view = viewList
		m.filtering = false
	case "backspace":
		if len(m.filter) > 0 {
			m.filter = m.filter[:len(m.filter)-1]
		}
	case "ctrl+c", "q":
		return m, tea.Quit
	default:
		if msg.Type == tea.KeyRunes {
			m.filter += string(msg.Runes)
		}
	}
	m.cursor = 0
	return m, nil
}

func (m Model) handleReaderKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc", "q":
		m.view = viewList
		m.reader = nil
	case "up", "k":
		m.reader.scrollUp(3)
	case "down", "j":
		m.reader.scrollDown(3)
	case "pgup":
		m.reader.scrollUp(m.height - 6)
	case "pgdown", " ":
		m.reader.scrollDown(m.height - 6)
	case "home":
		m.reader.offset = 0
	case "end":
		m.reader.offset = len(m.reader.body)
	case "ctrl+c":
		return m, tea.Quit
	}
	return m, nil
}

func (m Model) handleListKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	items := m.visibleItems()
	switch msg.String() {
	case "ctrl+c", "q":
		return m, tea.Quit
	case "tab", "right", "l":
		if len(m.cats) > 0 {
			m.tab = (m.tab + 1) % len(m.cats)
			m.cursor = 0
		}
	case "shift+tab", "left", "h":
		if len(m.cats) > 0 {
			m.tab = (m.tab - 1 + len(m.cats)) % len(m.cats)
			m.cursor = 0
		}
	case "up", "k":
		if m.cursor > 0 {
			m.cursor--
		}
	case "down", "j":
		if m.cursor < len(items)-1 {
			m.cursor++
		}
	case "enter":
		if len(items) > 0 && m.cursor < len(items) {
			it := items[m.cursor]
			m.reader = &readerState{item: &it, offset: 0, err: "加载中…"}
			m.view = viewReader
			return m, fetchReaderCmd(m.fetcher, &it)
		}
	case "o":
		if len(items) > 0 && m.cursor < len(items) {
			openBrowser(items[m.cursor].URL)
		}
	case "/":
		m.view = viewFilter
		m.filtering = true
		m.filter = ""
	case "r":
		m.loading = true
		m.err = ""
		return m, fetchReportCmd(m.cfg, m.fetcher, m.opts)
	case "s":
		return m, saveCmd(m)
	}
	return m, nil
}

// ── 视图辅助 ─────────────────────────────────────────────────

func (m *Model) buildTabs() {
	if m.rep == nil {
		return
	}
	order := map[classify.Category]int{}
	for i, c := range m.cfg.Categories {
		order[classify.Category(c)] = i
	}
	order[classify.Other] = 999
	m.cats = nil
	seen := map[classify.Category]bool{}
	for _, it := range m.rep.Items {
		if !seen[it.Category] {
			seen[it.Category] = true
			m.cats = append(m.cats, it.Category)
		}
	}
	// 按配置顺序稳定排序
	for i := 0; i < len(m.cats); i++ {
		for j := i + 1; j < len(m.cats); j++ {
			if order[m.cats[j]] < order[m.cats[i]] {
				m.cats[i], m.cats[j] = m.cats[j], m.cats[i]
			}
		}
	}
	if len(m.cats) == 0 {
		m.cats = []classify.Category{classify.Other}
	}
	if m.tab >= len(m.cats) {
		m.tab = 0
	}
}

func (m Model) currentCat() classify.Category {
	if m.tab < 0 || m.tab >= len(m.cats) || m.rep == nil {
		return classify.Other
	}
	return m.cats[m.tab]
}

func (m Model) currentItems() []report.Item {
	cat := m.currentCat()
	var out []report.Item
	for _, it := range m.rep.Items {
		if it.Category == cat {
			out = append(out, it)
		}
	}
	return out
}

func (m Model) visibleItems() []report.Item {
	items := m.currentItems()
	if m.filter == "" {
		return items
	}
	fl := strings.ToLower(m.filter)
	var out []report.Item
	for _, it := range items {
		if strings.Contains(strings.ToLower(it.Title), fl) ||
			strings.Contains(strings.ToLower(it.SourceName), fl) {
			out = append(out, it)
		}
	}
	return out
}

func (m Model) catCount(cat classify.Category) int {
	if m.rep == nil {
		return 0
	}
	n := 0
	for _, it := range m.rep.Items {
		if it.Category == cat {
			n++
		}
	}
	return n
}

// ── View ─────────────────────────────────────────────────────

func (m Model) View() string {
	if m.loading {
		return styleTitle.Render("news-report") + "\n\n  正在抓取新闻…\n"
	}
	if m.rep == nil {
		return styleTitle.Render("news-report") + "\n\n" + styleErr.Render(m.err) +
			"\n\n  按 r 重试，q 退出\n"
	}
	switch m.view {
	case viewReader:
		return m.readerView()
	case viewFilter:
		return m.filterView()
	default:
		return m.listView()
	}
}

func (m Model) listView() string {
	var sb strings.Builder
	sb.WriteString(styleTitle.Render("📰 news-report") + " " +
		styleDim.Render(m.rep.Generated.Format("2006-01-02 15:04")+
			fmt.Sprintf(" ｜ %d 来源 · %d 条", len(m.rep.SourceStats), len(m.rep.Items))) + "\n")

	// Tab 行
	for _, c := range m.cats {
		label := strings.ToUpper(string(c))
		tab := fmt.Sprintf(" %s (%d) ", label, m.catCount(c))
		if c == m.currentCat() {
			sb.WriteString(styleTabSel.Render(tab))
		} else {
			sb.WriteString(styleTab.Render(tab))
		}
	}
	sb.WriteString("\n")

	// 条目列表（滚动窗口）
	items := m.visibleItems()
	top := m.cursor - (m.height-8)/2
	if top < 0 {
		top = 0
	}
	bottom := top + m.height - 8
	for i, it := range items {
		if i < top || i > bottom {
			continue
		}
		line := fmt.Sprintf("%s %s%s", catColorIcon(it.Category, "●"), it.Title,
			styleDim.Render(fmt.Sprintf(" — %s · %s · %s", it.SourceName, it.AgeLabel, strings.ToUpper(it.Lang))))
		if i == m.cursor {
			sb.WriteString(styleCursor.Render("▸ ") + line + "\n")
		} else {
			sb.WriteString(styleItem.Render("  ") + line + "\n")
		}
	}
	if len(items) == 0 {
		sb.WriteString(styleDim.Render("  （无条目）\n"))
	}
	sb.WriteString("\n")

	// 状态/帮助栏
	if m.err != "" {
		sb.WriteString(styleErr.Render("⚠ "+m.err) + "\n")
	}
	if m.status != "" {
		sb.WriteString(styleDim.Render(m.status) + "\n")
	}
	sb.WriteString(styleHelp.Render(
		"←→ 分类  ↑↓ 选择  Enter 阅读  o 浏览器打开  / 搜索  s 导出  r 刷新  q 退出"))
	return sb.String()
}

func (m Model) filterView() string {
	base := m.listView()
	// 在底部叠加搜索输入行
	prompt := styleCursor.Render("搜索: ") + m.filter + "▌"
	return base + "\n" + prompt
}

func (m Model) readerView() string {
	if m.reader == nil {
		return m.listView()
	}
	it := m.reader.item
	var sb strings.Builder
	sb.WriteString(styleTitle.Render("📄 "+it.Title) + "\n")
	sb.WriteString(styleDim.Render(fmt.Sprintf("%s · %s · %s ago · %s\n\n",
		it.SourceName, strings.ToUpper(it.Lang), it.AgeLabel, it.URL)))
	if m.reader.err != "" {
		sb.WriteString(styleErr.Render(m.reader.err) + "\n")
	}
	if m.reader.body != "" {
		visible := m.reader.body
		start := m.reader.offset
		if start > len(visible) {
			start = len(visible)
		}
		end := start + (m.height-6)*3 // 按行高估算显示量（rune 近似）
		if end > len(visible) {
			end = len(visible)
		}
		sb.WriteString(styleReader.Render(visible[start:end]))
		sb.WriteString("\n")
	}
	sb.WriteString(styleHelp.Render("↑↓ 滚动  PgUp/PgDn 翻页  Home/End 首尾  Esc 返回  q 退出"))
	return sb.String()
}

func catColorIcon(cat classify.Category, sym string) string {
	c := catColor[cat]
	return lipgloss.NewStyle().Foreground(c).Render(sym)
}

func writeCategoryMarkdown(sb *strings.Builder, m Model, cat classify.Category) {
	items := m.currentItems()
	fmt.Fprintf(sb, "# %s — %s\n\n", strings.ToUpper(string(cat)), time.Now().Format("2006-01-02 15:04"))
	for i, it := range items {
		fmt.Fprintf(sb, "**%d. %s**  \n`%s` · `%s` · %s ago  \n[原文链接](%s)\n\n",
			i+1, it.Title, it.SourceName, strings.ToUpper(it.Lang), it.AgeLabel, it.URL)
	}
}

// ── reader 滚动 ──────────────────────────────────────────────

func (r *readerState) scrollUp(n int) {
	if r.offset > 0 {
		r.offset -= n
		if r.offset < 0 {
			r.offset = 0
		}
	}
}

func (r *readerState) scrollDown(n int) {
	if r.offset < len(r.body) {
		r.offset += n
		if r.offset > len(r.body) {
			r.offset = len(r.body)
		}
	}
}

// ── 浏览器打开 ───────────────────────────────────────────────

func openBrowser(url string) {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	default:
		cmd = exec.Command("xdg-open", url)
	}
	_ = cmd.Start()
}
