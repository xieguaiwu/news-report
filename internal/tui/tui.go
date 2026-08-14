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
	"github.com/mattn/go-runewidth"

	"news-report/internal/article"
	"news-report/internal/cache"
	"news-report/internal/classify"
	"news-report/internal/config"
	"news-report/internal/fetch"
	"news-report/internal/llm"
	"news-report/internal/report"
	"news-report/internal/s2t"
	"news-report/internal/sources"
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
	classify.EduPolicy:  lipgloss.Color("45"),
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
	cfg      *config.Config
	fetcher  *fetch.Fetcher
	opts     report.Options
	showHelp bool

	rep    *report.Report
	cats   []classify.Category // Tab 顺序（配置分类 + other）
	tab    int
	cursor int

	filter       []rune
	filtering    bool
	filterCursor int // rune index
	noColor      bool

	view   view
	reader *readerState

	// articleCache 缓存已提取的正文（URL→text），避免同一会话内重复抓取
	articleCache map[string]string

	// artCache 磁盘级文章缓存，跨会话复用（TTL 24h）
	artCache *cache.Cache

	// ── LLM 集成 ──
	llm *llm.Client // 由 cfg 初始化，nil 表示未配置

	// 翻译状态（嵌入阅读器上下文）
	translating  bool   // 翻译进行中（阅读器当前显示译文）
	translated   string // 已缓存的翻译结果
	originalBody string // 翻译前原文副本（用于 t/Esc 恢复原文）

	// 摘要/解读弹出层
	popup *popupState // nil = 无弹出

	err     string
	status  string
	loading bool
	width   int
	height  int
}

type readerState struct {
	item   *report.Item
	body   string   // 原始提取文本（翻译状态下为译文）
	lines  []string // 按终端宽度预折行的显示行
	offset int      // 行偏移（0-based，显示 lines[offset:offset+visLines]）
	err    string
}

// popupState 是摘要/解读弹出层的状态。
type popupState struct {
	title   string   // 弹窗标题
	content string   // 渲染后的文本
	loading bool     // 加载中
	err     string   // 错误信息
	lines   []string // 预折行
	offset  int      // 行偏移
}

// Run 启动 TUI（阻塞直到退出）。
func Run(ctx context.Context, cfg *config.Config, fetcher *fetch.Fetcher, opts report.Options) error {
	m := New(cfg, fetcher, opts)
	m.noColor = os.Getenv("NO_COLOR") != ""
	p := tea.NewProgram(m, tea.WithAltScreen())
	_, err := p.Run()
	return err
}

// New 创建模型（视图层会立即发起异步抓取）。
func New(cfg *config.Config, fetcher *fetch.Fetcher, opts report.Options) Model {
	m := Model{
		cfg:          cfg,
		fetcher:      fetcher,
		opts:         opts,
		loading:      true,
		articleCache: map[string]string{},
	}
	// 磁盘文章缓存：TTL 24h，与 feed_cache 同目录
	if cfg.CacheMaxMB > 0 && !cfg.NoCache {
		m.artCache = cache.New(cfg.CachePath()+"/articles", cfg.CacheMaxMB*1024*1024, 24*time.Hour)
		_, _ = m.artCache.Purge() // 启动时清理超限旧文件
	}
	// LLM 客户端（key 为空时客户端仍创建但 Available()=false，优雅降级）
	if cfg.LLM.TimeoutSec > 0 {
		llmCfg := llm.LLMConfig{
			BaseURL:    cfg.LLM.BaseURL,
			APIKey:     cfg.LLM.APIKey,
			Model:      cfg.LLM.Model,
			TargetLang: cfg.LLM.TargetLang,
			Timeout:    time.Duration(cfg.LLM.TimeoutSec) * time.Second,
			MaxChars:   cfg.LLM.MaxChars,
			ChunkSize:  1500,
			Overlap:    50,
		}
		m.llm = llm.New(llmCfg, cfg.CachePath()+"/llm")
		if m.llm.Cache() != nil {
			_, _ = m.llm.Cache().Purge() // 启动时清理超限旧文件
		}
	}
	return m
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

// ── LLM 消息类型 ─────────────────────────────────────────────

type llmTranslateMsg struct {
	body string // 翻译结果
	err  error
}

type llmSummaryMsg struct {
	points []string
	err    error
}

type llmBriefMsg struct {
	brief *llm.BriefResult
	err   error
}

// ── LLM 异步命令 ─────────────────────────────────────────────

// llmTranslateCmdCached 异步翻译，带缓存支持。
func llmTranslateCmdCached(client *llm.Client, body, lang, cacheURL string) tea.Cmd {
	return func() tea.Msg {
		// 查缓存
		if client.Cache() != nil {
			key := llm.CacheKey(cacheURL, lang, llm.InstTranslate)
			if cached, ok := client.Cache().Get(key); ok {
				return llmTranslateMsg{body: cached}
			}
		}
		ctx, cancel := context.WithTimeout(context.Background(), client.Timeout())
		defer cancel()
		result, err := client.Translate(ctx, body, lang)
		// 写缓存（成功时）
		if err == nil && result != "" && client.Cache() != nil {
			key := llm.CacheKey(cacheURL, lang, llm.InstTranslate)
			_ = client.Cache().Set(key, result)
		}
		return llmTranslateMsg{body: result, err: err}
	}
}

// llmSummaryCmd 异步生成摘要（自动抓取正文如果 item.Body 为空）。
func llmSummaryCmd(client *llm.Client, it *report.Item, artCache *cache.Cache, f *fetch.Fetcher) tea.Cmd {
	return func() tea.Msg {
		// 查缓存
		if client.Cache() != nil {
			key := llm.CacheKey(it.URL, it.Lang, llm.InstSummary)
			if cached, ok := client.Cache().Get(key); ok {
				points := parseNumberedLinesCompat(cached)
				return llmSummaryMsg{points: points}
			}
		}
		body := it.Body
		if body == "" {
			// 无正文：先抓取
			var artBody string
			if artCache != nil {
				if e := artCache.Get(it.URL, sources.TierSpecialist); e != nil {
					artBody = strings.TrimSpace(string(e.Body))
				}
			}
			if artBody == "" {
				ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
				art, err := article.Extract(ctx, f, it.URL, it.Lang)
				cancel()
				if err != nil {
					return llmSummaryMsg{err: fmt.Errorf("获取正文失败: %w", err)}
				}
				artBody = strings.TrimSpace(art.Text)
				if artCache != nil && artBody != "" {
					_ = artCache.Set(it.URL, []byte(artBody), "", "")
				}
			}
			body = artBody
		}
		ctx, cancel := context.WithTimeout(context.Background(), client.Timeout())
		defer cancel()
		points, err := client.Summarize(ctx, it.Title, body)
		// 写缓存
		if err == nil && client.Cache() != nil {
			key := llm.CacheKey(it.URL, it.Lang, llm.InstSummary)
			_ = client.Cache().Set(key, strings.Join(points, "\n"))
		}
		return llmSummaryMsg{points: points, err: err}
	}
}

// llmBriefCmd 异步生成深度解读（自动抓取正文如果 item.Body 为空）。
func llmBriefCmd(client *llm.Client, it *report.Item, artCache *cache.Cache, f *fetch.Fetcher) tea.Cmd {
	return func() tea.Msg {
		// 查缓存
		if client.Cache() != nil {
			key := llm.CacheKey(it.URL, it.Lang, llm.InstBrief)
			if cached, ok := client.Cache().Get(key); ok {
				brief := parseBriefResult(cached)
				return llmBriefMsg{brief: brief}
			}
		}
		body := it.Body
		if body == "" {
			var artBody string
			if artCache != nil {
				if e := artCache.Get(it.URL, sources.TierSpecialist); e != nil {
					artBody = strings.TrimSpace(string(e.Body))
				}
			}
			if artBody == "" {
				ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
				art, err := article.Extract(ctx, f, it.URL, it.Lang)
				cancel()
				if err != nil {
					return llmBriefMsg{err: fmt.Errorf("获取正文失败: %w", err)}
				}
				artBody = strings.TrimSpace(art.Text)
				if artCache != nil && artBody != "" {
					_ = artCache.Set(it.URL, []byte(artBody), "", "")
				}
			}
			body = artBody
		}
		ctx, cancel := context.WithTimeout(context.Background(), client.Timeout())
		defer cancel()
		brief, err := client.Brief(ctx, it.Title, body)
		// 写缓存
		if err == nil && client.Cache() != nil {
			key := llm.CacheKey(it.URL, it.Lang, llm.InstBrief)
			_ = client.Cache().Set(key, formatBriefForCache(brief))
		}
		return llmBriefMsg{brief: brief, err: err}
	}
}

// formatBriefForCache 将 BriefResult 序列化为可缓存字符串。
func formatBriefForCache(b *llm.BriefResult) string {
	return fmt.Sprintf("## 背景\n%s\n\n## 各方立场\n%s\n\n## 影响\n%s\n\n## 后续关注\n%s",
		b.Background, b.Positions, b.Impact, b.Outlook)
}

// parseNumberedLinesCompat 从缓存字符串解析摘要要点（复用 llm 包的解析逻辑）。
func parseNumberedLinesCompat(text string) []string {
	lines := strings.Split(strings.TrimSpace(text), "\n")
	var out []string
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		runes := []rune(trimmed)
		if len(runes) >= 2 &&
			runes[0] >= '1' && runes[0] <= '5' &&
			(runes[1] == '.' || runes[1] == ')' || runes[1] == '、') {
			out = append(out, trimmed)
		}
	}
	if len(out) == 0 && text != "" {
		out = []string{text}
	}
	return out
}

// parseBriefResult 从缓存字符串解析 BriefResult。
func parseBriefResult(text string) *llm.BriefResult {
	r := &llm.BriefResult{}
	currentKey := ""
	var sb strings.Builder
	for _, line := range strings.Split(text, "\n") {
		trimmed := strings.TrimSpace(line)
		switch trimmed {
		case "## 背景", "## 各方立场", "## 影响", "## 后续关注":
			setBriefSection(r, currentKey, sb.String())
			currentKey = trimmed
			sb.Reset()
			continue
		}
		if currentKey != "" {
			if sb.Len() > 0 {
				sb.WriteByte('\n')
			}
			sb.WriteString(line)
		}
	}
	setBriefSection(r, currentKey, sb.String())
	return r
}

// setBriefSection 根据段落标题写入 BriefResult 对应字段。
func setBriefSection(r *llm.BriefResult, key, content string) {
	content = strings.TrimSpace(content)
	if content == "" {
		return
	}
	switch key {
	case "## 背景":
		r.Background = content
	case "## 各方立场":
		r.Positions = content
	case "## 影响":
		r.Impact = content
	case "## 后续关注":
		r.Outlook = content
	}
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

func fetchReaderCmd(f *fetch.Fetcher, item *report.Item, artCache *cache.Cache) tea.Cmd {
	return func() tea.Msg {
		// 1. 磁盘缓存命中：免抓取直接返回
		if artCache != nil {
			if e := artCache.Get(item.URL, sources.TierSpecialist); e != nil {
				body := strings.TrimSpace(string(e.Body))
				if body != "" {
					return readerMsg{body: body}
				}
			}
		}
		// 2. 抓取并提取正文
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		art, err := article.Extract(ctx, f, item.URL, item.Lang)
		if err != nil {
			return readerMsg{body: "", err: err}
		}
		body := strings.TrimSpace(art.Text)
		// 3. 写入磁盘缓存（TTL 24h）
		if artCache != nil && body != "" {
			_ = artCache.Set(item.URL, []byte(body), "", "")
		}
		return readerMsg{body: body}
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
		oldW := m.width
		m.width, m.height = msg.Width, msg.Height
		// 终端宽度变化时重新折行阅读器内容
		if m.reader != nil && m.reader.body != "" && m.width != oldW {
			m.reader.lines = splitWrapped(m.reader.body, readerWidth(m.width))
			if m.reader.offset >= len(m.reader.lines) {
				m.reader.offset = max(0, len(m.reader.lines)-1)
			}
		}
		// Bug1: 缩放时弹窗折行残留，旧宽度 lines 导致渲染错位
		if m.popup != nil {
			m.popup.lines = nil
		}
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
				m.reader.lines = splitWrapped(msg.body, readerWidth(m.width))
				m.reader.offset = 0
				// 缓存正文，避免会话内重复抓取
				if m.reader.item != nil && msg.body != "" {
					if m.articleCache == nil {
						m.articleCache = map[string]string{}
					}
					m.articleCache[m.reader.item.URL] = msg.body
				}
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

	case llmTranslateMsg:
		m.status = ""
		if msg.err != nil {
			m.status = "翻译失败: " + msg.err.Error()
			m.translating = false
			if m.originalBody != "" && m.reader != nil {
				m.reader.body = m.originalBody
				m.reader.lines = splitWrapped(m.originalBody, readerWidth(m.width))
			}
		} else {
			m.translated = msg.body
			if m.reader != nil {
				m.reader.body = msg.body
				m.reader.lines = splitWrapped(msg.body, readerWidth(m.width))
				m.reader.offset = 0
			}
			m.status = "翻译完成 | 按 t/Esc 回原文"
		}
		return m, nil

	case llmSummaryMsg:
		if m.popup != nil {
			m.popup.loading = false
			if msg.err != nil {
				m.popup.err = msg.err.Error()
			} else {
				var sb strings.Builder
				for i, p := range msg.points {
					fmt.Fprintf(&sb, "%d. %s\n", i+1, p)
				}
				m.popup.content = sanitizeLLMText(sb.String())
				m.popup.lines = nil // 在 overlayPopup 渲染时按弹窗宽度折行
				m.popup.offset = 0
				m.status = "摘要生成完毕"
			}
		}
		return m, nil

	case llmBriefMsg:
		if m.popup != nil {
			m.popup.loading = false
			if msg.err != nil {
				m.popup.err = msg.err.Error()
			} else {
				b := msg.brief
				m.popup.content = sanitizeLLMText(fmt.Sprintf("## 背景\n%s\n\n## 各方立场\n%s\n\n## 影响\n%s\n\n## 后续关注\n%s",
					b.Background, b.Positions, b.Impact, b.Outlook))
				m.popup.lines = nil // 在 overlayPopup 渲染时按弹窗宽度折行
				m.popup.offset = 0
				m.status = "解读生成完毕"
			}
		}
		return m, nil

	case tea.KeyMsg:
		return m.handleKey(msg)
	}
	return m, nil
}

func (m Model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// 弹窗优先拦截按键
	if m.popup != nil {
		contentH := popupContentH(m.height)
		switch msg.String() {
		case "esc", "q":
			m.popup = nil
			return m, nil
		case "up", "k":
			popupScroll(m.popup, -1, contentH)
			return m, nil
		case "down", "j":
			popupScroll(m.popup, +1, contentH)
			return m, nil
		case "pgup":
			popupScroll(m.popup, -contentH, contentH)
			return m, nil
		case "pgdown", " ":
			popupScroll(m.popup, +contentH, contentH)
			return m, nil
		case "home":
			m.popup.offset = 0
			return m, nil
		case "end":
			m.popup.offset = popupMaxOffset(m.popup, contentH)
			return m, nil
		}
		return m, nil
	}

	switch m.view {
	case viewFilter:
		return m.handleFilterKey(msg)
	case viewReader:
		return m.handleReaderKey(msg)
	default:
		return m.handleListKey(msg)
	}
}

// popupMaxOffset 返回弹窗可滚动的最大行偏移（最后一行恰好落在窗口底端）。
func popupMaxOffset(p *popupState, contentH int) int {
	maxOff := len(p.lines) - contentH
	if maxOff < 0 {
		maxOff = 0
	}
	return maxOff
}

// popupScroll 按行滚动弹窗内容，offset 钳位在 [0, 最后一行可见] 区间。
func popupScroll(p *popupState, delta, contentH int) {
	if len(p.lines) == 0 {
		p.offset = 0
		return
	}
	p.offset += delta
	maxOff := popupMaxOffset(p, contentH)
	if p.offset < 0 {
		p.offset = 0
	}
	if p.offset > maxOff {
		p.offset = maxOff
	}
}

func (m Model) handleFilterKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.view = viewList
		m.filtering = false
		m.filter = nil
		m.filterCursor = 0
	case "enter":
		m.view = viewList
		m.filtering = false
	case "backspace":
		if m.filterCursor > 0 {
			m.filter = append(m.filter[:m.filterCursor-1], m.filter[m.filterCursor:]...)
			m.filterCursor--
		}
	case "delete":
		if m.filterCursor < len(m.filter) {
			m.filter = append(m.filter[:m.filterCursor], m.filter[m.filterCursor+1:]...)
		}
	case "left":
		if m.filterCursor > 0 {
			m.filterCursor--
		}
	case "right":
		if m.filterCursor < len(m.filter) {
			m.filterCursor++
		}
	case "home":
		m.filterCursor = 0
	case "end":
		m.filterCursor = len(m.filter)
	case "ctrl+w":
		m.filter, m.filterCursor = deleteWordBackwardRunes(m.filter, m.filterCursor)
	case "ctrl+u":
		m.filter = m.filter[m.filterCursor:]
		m.filterCursor = 0
	case "ctrl+k":
		m.filter = m.filter[:m.filterCursor]
	case "ctrl+c", "q":
		return m, tea.Quit
	default:
		if msg.Type == tea.KeyRunes {
			r := msg.Runes
			ins := make([]rune, m.filterCursor+len(r)+len(m.filter[m.filterCursor:]))
			copy(ins, m.filter[:m.filterCursor])
			copy(ins[m.filterCursor:], r)
			copy(ins[m.filterCursor+len(r):], m.filter[m.filterCursor:])
			m.filter = ins
			m.filterCursor += len(r)
		}
	}
	m.cursor = 0
	return m, nil
}

func (m Model) handleReaderKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc", "q":
		// 翻译状态下 Esc 先恢复原文
		if m.translating {
			m.reader.body = m.originalBody
			m.reader.lines = splitWrapped(m.originalBody, readerWidth(m.width))
			m.translating = false
			m.status = "已恢复原文"
			return m, nil
		}
		m.view = viewList
		m.reader = nil
		m.translating = false
	case "t":
		// 翻译当前正文
		if m.llm == nil || !m.llm.Available() {
			m.status = "LLM 未配置，请在 config.yaml 设置 llm.api_key"
			return m, nil
		}
		if m.reader == nil || m.reader.body == "" {
			return m, nil
		}
		// 如果已经是翻译状态，再按 t 恢复原文
		if m.translating {
			m.reader.body = m.originalBody
			m.reader.lines = splitWrapped(m.originalBody, readerWidth(m.width))
			m.translating = false
			m.status = "已恢复原文"
			return m, nil
		}
		// 检查是否已有缓存翻译
		if m.translated != "" {
			m.originalBody = m.reader.body
			m.reader.body = m.translated
			m.reader.lines = splitWrapped(m.translated, readerWidth(m.width))
			m.reader.offset = 0
			m.translating = true
			m.status = "译文（缓存）| 按 t/Esc 回原文"
			return m, nil
		}
		// 发起异步翻译
		m.originalBody = m.reader.body
		m.translating = true
		m.status = "翻译中…"
		cacheURL := ""
		if m.reader.item != nil {
			cacheURL = m.reader.item.URL
		}
		return m, llmTranslateCmdCached(m.llm, m.reader.body, m.reader.item.Lang, cacheURL)
	case "d":
		// 深度解读（结果显示在弹窗中）
		if m.llm == nil || !m.llm.Available() {
			m.status = "LLM 未配置，请在 config.yaml 设置 llm.api_key"
			return m, nil
		}
		if m.reader == nil || m.reader.item == nil {
			return m, nil
		}
		m.popup = &popupState{title: "解读: " + m.reader.item.Title, loading: true}
		return m, llmBriefCmd(m.llm, m.reader.item, m.artCache, m.fetcher)
	case "up", "k":
		m.reader.scrollUp(1, len(m.reader.lines))
	case "down", "j":
		m.reader.scrollDown(1, len(m.reader.lines))
	case "pgup":
		m.reader.scrollUp(max(1, m.height-5), len(m.reader.lines))
	case "pgdown", " ":
		m.reader.scrollDown(max(1, m.height-5), len(m.reader.lines))
	case "home":
		m.reader.offset = 0
	case "end":
		vis := max(1, m.height-5)
		m.reader.offset = max(0, len(m.reader.lines)-vis)
	case "o":
		if m.reader != nil && m.reader.item != nil {
			url := m.reader.item.URL
			openBrowser(url)
			m.status = "浏览器打开: " + url
		}
	case "c":
		if m.reader != nil && m.reader.item != nil {
			if err := copyToClipboard(m.reader.item.URL); err != nil {
				m.status = "复制失败: " + err.Error()
			} else {
				m.status = "已复制: " + m.reader.item.URL
			}
		}
	case "ctrl+c":
		return m, tea.Quit
	}
	return m, nil
}

func (m Model) handleListKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	items := m.visibleItems()
	switch msg.String() {
	case "x":
		// 列表视图：对当前条目生成摘要
		if m.llm == nil || !m.llm.Available() {
			m.status = "LLM 未配置，请在 config.yaml 设置 llm.api_key"
			return m, nil
		}
		if len(items) > 0 && m.cursor < len(items) {
			it := items[m.cursor]
			m.popup = &popupState{title: "摘要: " + it.Title, loading: true}
			return m, llmSummaryCmd(m.llm, &it, m.artCache, m.fetcher)
		}
	case "ctrl+c", "q":
		return m, tea.Quit
	case "?":
		m.showHelp = !m.showHelp
		return m, nil
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
			// 缓存命中：直接显示，免再抓取
			if m.articleCache != nil {
				if body, ok := m.articleCache[it.URL]; ok && body != "" {
					m.reader.body = body
					m.reader.err = ""
					m.reader.lines = splitWrapped(body, readerWidth(m.width))
					m.reader.offset = 0
					return m, nil
				}
			}
			return m, fetchReaderCmd(m.fetcher, &it, m.artCache)
		}
	case "o":
		if len(items) > 0 && m.cursor < len(items) {
			url := items[m.cursor].URL
			openBrowser(url)
			m.status = "浏览器打开: " + url
		}
	case "/":
		m.view = viewFilter
		m.filtering = true
		m.filter = nil
		m.filterCursor = 0
	case "n":
		if len(m.filter) > 0 {
			m.cursor = nextFilterMatch(m, m.cursor, +1)
		}
	case "N":
		if len(m.filter) > 0 {
			m.cursor = nextFilterMatch(m, m.cursor, -1)
		}
	case "r":
		m.loading = true
		m.err = ""
		return m, fetchReportCmd(m.cfg, m.fetcher, m.opts)
	case "s":
		return m, saveCmd(m)
	case "c":
		if len(items) > 0 && m.cursor < len(items) {
			if err := copyToClipboard(items[m.cursor].URL); err != nil {
				m.status = "复制失败: " + err.Error()
			} else {
				m.status = "已复制: " + items[m.cursor].URL
			}
		}
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
	if len(m.filter) == 0 {
		return items
	}
	fl := strings.ToLower(string(m.filter))
	var out []report.Item
	for _, it := range items {
		if matchFilter(it.Title, fl) ||
			matchFilter(it.SourceName, fl) {
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

func (m Model) viewNoColor() string {
	if m.showHelp {
		return m.helpView()
	}
	if m.loading {
		return "news-report\n\n  正在抓取新闻…\n"
	}
	if m.rep == nil {
		return "news-report\n\n" + m.err + "\n\n  按 r 重试，q 退出\n"
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

func (m Model) View() string {
	var base string
	if m.noColor {
		base = stripANSI(m.viewNoColor())
	} else if m.showHelp {
		base = m.helpView()
	} else if m.loading {
		base = styleTitle.Render("news-report") + "\n\n  正在抓取新闻…\n"
	} else if m.rep == nil {
		base = styleTitle.Render("news-report") + "\n\n" + styleErr.Render(m.err) +
			"\n\n  按 r 重试，q 退出\n"
	} else {
		switch m.view {
		case viewReader:
			base = m.readerView()
		case viewFilter:
			base = m.filterView()
		default:
			base = m.listView()
		}
	}
	// 弹窗叠加
	if m.popup != nil {
		return overlayPopup(base, m.popup, m.width, m.height)
	}
	return base
}

func (m Model) listView() string {
	var sb strings.Builder
	titleLine := "📰 news-report " + m.rep.Generated.Format("2006-01-02 15:04") +
		fmt.Sprintf(" ｜ %d 来源 · %d 条", len(m.rep.SourceStats), len(m.rep.Items))
	sb.WriteString(styleTitle.Render(truncateCells(titleLine, m.width-2)) + "\n")

	// Tab 行（逐个累计宽度，超出终端宽即截断剩余 Tab，样式完整保留）
	var tabSB strings.Builder
	tabW := 0
	for _, c := range m.cats {
		label := strings.ToUpper(string(c))
		tab := fmt.Sprintf(" %s (%d) ", label, m.catCount(c))
		tw := cellWidth(tab)
		if tabW+tw > m.width {
			break
		}
		if c == m.currentCat() {
			tabSB.WriteString(styleTabSel.Render(tab))
		} else {
			tabSB.WriteString(styleTab.Render(tab))
		}
		tabW += tw
	}
	sb.WriteString(tabSB.String() + "\n")

	// 条目列表（滚动窗口）
	items := m.visibleItems()
	top := m.cursor - (m.height-8)/2
	if top < 0 {
		top = 0
	}
	bottom := top + m.height - 8
	// 前缀占位：光标 2 列 + 图标 1 列 + 空格 1 列 = 4 列
	avail := m.width - 4
	if avail < 8 {
		avail = 8
	}
	for i, it := range items {
		if i < top || i > bottom {
			continue
		}
		icon := catColorIcon(it.Category, "●")
		suffix := " — " + it.SourceName + " · " + it.Timestamp() + " · " + strings.ToUpper(it.Lang)
		suffixW := cellWidth(suffix)
		titleW := avail - suffixW
		if titleW < 1 {
			titleW = 1
		}
		title := truncateCells(it.Title, titleW)
		if suffixW > avail {
			suffix = truncateCells(suffix, avail)
		}
		line := icon + " " + title + styleDim.Render(suffix)
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
		sb.WriteString(styleErr.Render(truncateCells("⚠ "+m.err, m.width)) + "\n")
	}
	if m.status != "" {
		sb.WriteString(styleDim.Render(truncateCells(m.status, m.width)) + "\n")
	}
	sb.WriteString(styleHelp.Render(truncateCells(
		"←→ 分类  ↑↓ 选择  Enter 阅读  o 浏览器  c 复制链接  / 搜索  s 导出  r 刷新  q 退出", m.width)))
	return sb.String()
}

func (m Model) filterView() string {
	base := m.listView()
	// 搜索输入行（含光标指示）
	p := string(m.filter)
	if m.filterCursor > 0 && m.filterCursor <= len(m.filter) {
		runes := []rune(p)
		p = string(runes[:m.filterCursor]) + "\u258C" + string(runes[m.filterCursor:])
	} else {
		p = "\u258C" + p
	}
	// 在底部叠加搜索输入行
	prompt := styleCursor.Render("搜索: ") + p
	return base + "\n" + prompt
}

func (m Model) readerView() string {
	if m.reader == nil {
		return m.listView()
	}
	it := m.reader.item
	var sb strings.Builder
	// 标题/来源行按终端宽度截断（避免长标题/长 URL 硬换行破坏行数计算）
	titleW := m.width - 2 // styleTitle 左右 padding 各 1
	if titleW < 10 {
		titleW = 10
	}
	sb.WriteString(styleTitle.Render(truncateCells("📄 "+it.Title, titleW)) + "\n")
	metaLine1 := fmt.Sprintf("%s · %s · %s ago · %s",
		it.SourceName, strings.ToUpper(it.Lang), it.AgeLabel, it.Published.Format("2006-01-02 15:04"))
	sb.WriteString(styleDim.Render(truncateCells(metaLine1, m.width)) + "\n")
	sb.WriteString(styleDim.Render(truncateCells(it.URL, m.width)) + "\n\n")
	if m.reader.err != "" {
		errStr := m.reader.err
		hint := ""
		if strings.Contains(errStr, "403") || strings.Contains(errStr, "Forbidden") {
			hint = "\n提示：按 o 在浏览器中打开原文"
		} else if strings.Contains(errStr, "付费墙") || strings.Contains(errStr, "paywall") {
			hint = "\n提示：试试 news-report find '标题' 找免费转载"
		}
		sb.WriteString(styleErr.Render(truncateCells(errStr+hint, m.width)) + "\n")
	}
	if m.reader.body != "" && len(m.reader.lines) > 0 {
		// 行级显示：从预折行 lines[offset:] 取 visLines 行
		visLines := m.height - 5 // 减去标题行 + 来源行 + 帮助栏
		if visLines < 1 {
			visLines = 1
		}
		start := m.reader.offset
		if start < 0 {
			start = 0
		}
		if start >= len(m.reader.lines) {
			start = len(m.reader.lines) - 1
		}
		end := start + visLines
		if end > len(m.reader.lines) {
			end = len(m.reader.lines)
		}
		visible := strings.Join(m.reader.lines[start:end], "\n")
		sb.WriteString(styleReader.Render(visible))
		sb.WriteString("\n")
	}
	sb.WriteString(styleHelp.Render(truncateCells("↑↓ 滚动  PgUp/PgDn 翻页  Home/End 首尾  o 浏览器  t 翻译  d 解读  c 复制链接  Esc 返回  q 退出", m.width)))
	if m.status != "" {
		sb.WriteString("\n" + styleDim.Render(truncateCells(m.status, m.width)))
	}
	return sb.String()
}

func stripANSI(s string) string {
	var b strings.Builder
	inEsc := false
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c == 0x1b {
			inEsc = true
			continue
		}
		if inEsc {
			if c >= 0x40 && c <= 0x7e {
				inEsc = false
			}
			continue
		}
		b.WriteByte(c)
	}
	return b.String()
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

// ── reader 滚动（offset 为行索引，基于预折行 lines） ──────

func (r *readerState) scrollUp(n, _ int) {
	if r.offset > 0 {
		r.offset -= n
		if r.offset < 0 {
			r.offset = 0
		}
	}
}

func (r *readerState) scrollDown(n, totalLines int) {
	limit := totalLines - 1
	if limit < 0 {
		limit = 0
	}
	r.offset += n
	if r.offset > limit {
		r.offset = limit
	}
}

// readerWidth 返回阅读器每行可用宽度。
func readerWidth(termWidth int) int {
	w := termWidth - 4 // 减去 styleReader 两边 padding
	if w < 10 {
		w = 10
	}
	return w
}

// splitWrapped 将文本按给定显示宽度折行后拆分为显示行切片。
func splitWrapped(text string, width int) []string {
	return strings.Split(strings.TrimRight(wrapCells(text, width), "\n"), "\n")
}

// ── 帮助视图 ─────────────────────────────────────────────────

func (m Model) helpView() string {
	var sb strings.Builder
	sb.WriteString(styleTitle.Render("键位帮助") + "\n\n")
	for _, row := range [][2]string{
		{"tab / h,l", "切换分类"},
		{"up/down / j,k", "选择条目"},
		{"Enter", "阅读全文"},
		{"/", "搜索过滤（Esc 退出）"},
		{"o / c", "浏览器打开 / 复制链接"},
		{"t", "翻译当前正文（再按恢复原文）"},
		{"x", "生成 5 点摘要（列表视图）"},
		{"d", "深度解读（阅读器视图）"},
		{"s", "导出当前分类 Markdown"},
		{"r", "重新抓取"},
		{"q", "退出"},
		{"?", "本帮助"},
		{"PgUp/PgDn / 空格/b", "阅读器翻页"},
		{"Home/End", "阅读器首/尾"},
	} {
		key := styleCursor.Render(row[0])
		// 按显示宽度对齐（ANSI 与 CJK 感知），避免 fmt %-*s 计入转义字节
		pad := 30 - cellWidth(stripANSI(key))
		if pad < 1 {
			pad = 1
		}
		sb.WriteString("  " + key + strings.Repeat(" ", pad) + row[1] + "\n")
	}
	sb.WriteString("\n按任意键返回\n")
	return sb.String()
}

// wrapCells 将长行按指定显示宽度折行（列数感知：CJK 全角=2 列）。
// ASCII 词优先整词断行；超长词（含长 URL）按列硬断；\t 展开为 4 空格；保留空行。
func wrapCells(text string, width int) string {
	if width < 2 {
		width = 2
	}
	var sb strings.Builder
	for _, line := range strings.Split(text, "\n") {
		line = strings.ReplaceAll(line, "\t", "    ")
		words := strings.Fields(line)
		if len(words) == 0 {
			sb.WriteByte('\n')
			continue
		}
		var cur strings.Builder
		curW, first := 0, true
		flush := func() {
			if cur.Len() > 0 {
				sb.WriteString(cur.String())
				sb.WriteByte('\n')
				cur.Reset()
				curW, first = 0, true
			}
		}
		for _, w := range words {
			wW := cellWidth(w)
			if wW > width {
				// 超长词：先冲刷当前行，再按列硬断该词
				flush()
				var b strings.Builder
				bw := 0
				for _, r := range w {
					rw := runeCells(r)
					if bw+rw > width && b.Len() > 0 {
						sb.WriteString(b.String())
						sb.WriteByte('\n')
						b.Reset()
						bw = 0
					}
					b.WriteRune(r)
					bw += rw
				}
				cur = b
				curW, first = bw, false
				continue
			}
			add := wW
			if !first {
				add++ // 空格占 1 列
			}
			if curW+add > width {
				flush()
				cur.WriteString(w)
				curW, first = wW, false
			} else if first {
				cur.WriteString(w)
				curW, first = wW, false
			} else {
				cur.WriteByte(' ')
				cur.WriteString(w)
				curW += add
			}
		}
		flush()
	}
	return sb.String()
}

// ── 单元格宽度感知工具（CJK 全角=2 列，ANSI=0 列） ──────

// cellWidth 返回字符串在终端上占用的显示宽度（列数，ANSI 感知）。
func cellWidth(s string) int {
	return lipgloss.Width(s)
}

// runeCells 返回单个 rune 的显示宽度（控制字符/组合符为 0）。
func runeCells(r rune) int {
	w := runewidth.RuneWidth(r)
	if w < 0 {
		return 0
	}
	return w
}

// cutCells 返回 s 的前 n 个显示列（rune 边界对齐，结果宽度 ≤ n）。
func cutCells(s string, n int) string {
	return sliceCells(s, 0, n)
}

// sliceCells 返回 s 中显示宽度位于 [from, to) 的子串（rune 边界对齐）。
// 跨边界的宽字符整体丢弃，保证结果左边界恰好落在 from 列。
func sliceCells(s string, from, to int) string {
	if from < 0 {
		from = 0
	}
	if to <= from {
		return ""
	}
	var sb strings.Builder
	w := 0
	for _, r := range s {
		rw := runeCells(r)
		if rw == 0 {
			// 零宽字符挂靠当前列
			if w >= from && w < to {
				sb.WriteRune(r)
			}
			continue
		}
		if w >= to {
			break
		}
		if w >= from && w+rw <= to {
			sb.WriteRune(r)
		}
		w += rw
	}
	return sb.String()
}

// truncateCells 将 s 截断到 ≤ maxCells 列并追加省略号（省略号计入宽度）。
func truncateCells(s string, maxCells int) string {
	if maxCells < 1 {
		return ""
	}
	if cellWidth(s) <= maxCells {
		return s
	}
	if maxCells == 1 {
		return "…"
	}
	return cutCells(s, maxCells-1) + "…"
}

// padCells 将 s 补齐/截断到恰好 width 列。
func padCells(s string, width int) string {
	t := truncateCells(s, width)
	rem := width - cellWidth(t)
	if rem > 0 {
		t += strings.Repeat(" ", rem)
	}
	return t
}

// sanitizeLLMText 清理 LLM 输出中的 markdown 标记（保留文字与段落结构）。
func sanitizeLLMText(s string) string {
	var out []string
	for _, l := range strings.Split(s, "\n") {
		l = strings.TrimRight(l, " ")
		if strings.HasPrefix(strings.TrimSpace(l), "```") {
			continue // 代码围栏
		}
		l = strings.TrimPrefix(l, "### ")
		l = strings.TrimPrefix(l, "## ")
		l = strings.TrimPrefix(l, "# ")
		l = strings.ReplaceAll(l, "**", "")
		l = strings.ReplaceAll(l, "__", "")
		l = strings.ReplaceAll(l, "`", "")
		if strings.HasPrefix(l, "- ") || strings.HasPrefix(l, "* ") {
			l = "• " + l[2:]
		}
		out = append(out, l)
	}
	return strings.Join(out, "\n")
}

// popupContentH 返回弹窗内容区高度（与 overlayPopup 渲染保持一致）。
func popupContentH(termH int) int {
	if termH < 6 {
		termH = 6
	}
	ph := termH * 8 / 10
	if ph < 6 {
		ph = 6
	}
	if ph > termH-2 {
		ph = termH - 2
	}
	contentH := ph - 4
	if contentH < 1 {
		contentH = 1
	}
	return contentH
}

// deleteWordBackward 删前一词（rune 版）。
func deleteWordBackwardRunes(s []rune, cursor int) ([]rune, int) {
	if cursor == 0 {
		return s, 0
	}
	i := cursor
	// 跳过空白
	for i > 0 && s[i-1] == ' ' {
		i--
	}
	// 跳过非空白词
	for i > 0 && s[i-1] != ' ' {
		i--
	}
	return append(s[:i], s[cursor:]...), i
}

// ── 浏览器打开 ───────────────────────────────────────────────

// nextFilterMatch 返回过滤后列表中下/上一个匹配项索引。
func nextFilterMatch(m Model, cur int, delta int) int {
	items := m.visibleItems()
	if len(items) == 0 {
		return cur
	}
	return (cur + delta + len(items)) % len(items)
}

// matchFilter 检查 text 是否匹配 filter，支持简繁中文双向匹配。
func matchFilter(text, filter string) bool {
	if strings.Contains(strings.ToLower(text), filter) {
		return true
	}
	// 简繁一致性：将 text 统一转简体后再匹配
	textNorm := s2t.Normalize(text)
	filterNorm := s2t.Normalize(filter)
	return strings.Contains(strings.ToLower(textNorm), strings.ToLower(filterNorm))
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func copyToClipboard(text string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("pbcopy")
	case "windows":
		cmd = exec.Command("clip")
	default:
		// Linux: Wayland 优先 wl-copy，X11 优先 xclip，否则尝试两者
		if os.Getenv("WAYLAND_DISPLAY") != "" {
			if _, err := exec.LookPath("wl-copy"); err == nil {
				cmd = exec.Command("wl-copy")
			}
		}
		if cmd == nil {
			if _, err := exec.LookPath("xclip"); err == nil {
				cmd = exec.Command("xclip", "-selection", "clipboard")
			}
		}
		if cmd == nil {
			if _, err := exec.LookPath("wl-copy"); err == nil {
				cmd = exec.Command("wl-copy")
			}
		}
	}
	if cmd == nil {
		return fmt.Errorf("未找到剪贴板工具（安装 xclip 或 wl-copy）")
	}
	cmd.Stdin = strings.NewReader(text)
	// 同 openBrowser，避免 TUI 模式下 pipe 信号
	devnull, _ := os.OpenFile(os.DevNull, os.O_WRONLY, 0)
	if devnull != nil {
		defer devnull.Close()
		cmd.Stdout = devnull
		cmd.Stderr = devnull
	}
	return cmd.Run()
}

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
	devnull, _ := os.OpenFile(os.DevNull, os.O_WRONLY, 0)
	if devnull != nil {
		defer devnull.Close()
		cmd.Stdin = devnull
		cmd.Stdout = devnull
		cmd.Stderr = devnull
	}
	_ = cmd.Start()
}

// ── 弹窗叠加 ─────────────────────────────────────────────────

// overlayPopup 将弹窗叠加在基础视图之上。
// 弹窗居中显示，带边框和标题，可滚动，半透明背景遮罩。
// 全部宽度计算基于显示列数（CJK=2 列）；底图先剥 ANSI 再按列切片，避免错位。
func overlayPopup(base string, p *popupState, termW, termH int) string {
	if termW < 20 {
		termW = 20
	}
	if termH < 6 {
		termH = 6
	}

	// 弹窗大小：取终端 70% 宽、80% 高
	pw := termW * 7 / 10
	if pw < 40 {
		pw = 40
	}
	if pw > termW-4 {
		pw = termW - 4
	}
	contentW := pw - 4 // 内容区宽度（-4 = │ + 空格 + ... + 空格 + │）
	contentH := popupContentH(termH)

	// 预折行：按弹窗内容宽度（列数感知）重新折行
	var lines []string
	if p.loading || p.err != "" || p.content == "" {
		p.lines = nil
	} else if len(p.lines) == 0 {
		lines = splitWrapped(p.content, contentW)
		p.lines = lines
	} else {
		lines = p.lines
	}

	// 滚动偏移 clamp 到可见窗口
	maxOff := len(lines) - contentH
	if maxOff < 0 {
		maxOff = 0
	}
	if p.offset > maxOff {
		p.offset = maxOff
	}
	if p.offset < 0 {
		p.offset = 0
	}

	var popupSB strings.Builder

	// 标题栏：┌─ 标题 ────────┐（全宽 = contentW+4，列数感知）
	title := truncateCells(p.title, contentW-1)
	titleW := cellWidth(title)
	popupSB.WriteString("┌─ " + title + " ")
	dashRemain := contentW - titleW - 1
	if dashRemain < 0 {
		dashRemain = 0
	}
	popupSB.WriteString(strings.Repeat("─", dashRemain) + "┐\n")

	// 画一行（按显示列截断 + 补齐到 contentW，保证右边界对齐）
	drawLine := func(text string) {
		t := truncateCells(text, contentW)
		popupSB.WriteString("│ ")
		popupSB.WriteString(t)
		rem := contentW - cellWidth(t)
		if rem > 0 {
			popupSB.WriteString(strings.Repeat(" ", rem))
		}
		popupSB.WriteString(" │\n")
	}

	if p.loading {
		for i := 0; i < contentH; i++ {
			if i == contentH/2 {
				drawLine("  正在生成，请稍候…")
			} else {
				drawLine("")
			}
		}
	} else if p.err != "" {
		drawLine("错误: " + p.err)
		for i := 1; i < contentH; i++ {
			drawLine("")
		}
	} else if len(lines) > 0 {
		start := p.offset
		if start < 0 {
			start = 0
		}
		if start > maxOff {
			start = maxOff
		}
		for i := 0; i < contentH; i++ {
			li := start + i
			if li < len(lines) {
				drawLine(lines[li])
			} else {
				drawLine("")
			}
		}
	} else {
		for i := 0; i < contentH; i++ {
			drawLine("")
		}
	}

	// 底部帮助
	drawLine("Esc 关闭  ↑↓ 滚动  PgUp/PgDn 翻页")

	// 底边框（全宽 = contentW+4，与标题栏和内容行对齐）
	popupSB.WriteString("└" + strings.Repeat("─", contentW+2) + "┘\n")

	popup := strings.TrimRight(popupSB.String(), "\n")
	popupLines := strings.Split(popup, "\n")
	baseLines := strings.Split(base, "\n")

	startRow := (termH - len(popupLines)) / 2
	if startRow < 0 {
		startRow = 0
	}
	startCol := (termW - (contentW + 4)) / 2
	if startCol < 0 {
		startCol = 0
	}

	var resultSB strings.Builder
	for row := 0; row < termH; row++ {
		if row >= startRow && row < startRow+len(popupLines) {
			popupLine := popupLines[row-startRow]
			var lineSB strings.Builder
			if row < len(baseLines) {
				// 底图行：先剥 ANSI 再做列级切片（CJK 安全），左右残段统一调暗
				plain := stripANSI(baseLines[row])
				left := padCells(cutCells(plain, startCol), startCol)
				rightStart := startCol + cellWidth(popupLine)
				right := ""
				if rightStart < cellWidth(plain) {
					right = sliceCells(plain, rightStart, cellWidth(plain))
				}
				lineSB.WriteString(styleDim.Render(left))
				lineSB.WriteString(popupLine)
				lineSB.WriteString(styleDim.Render(right))
			} else {
				if startCol > 0 {
					lineSB.WriteString(strings.Repeat(" ", startCol))
				}
				lineSB.WriteString(popupLine)
			}
			resultSB.WriteString(lineSB.String() + "\n")
		} else if row < len(baseLines) {
			resultSB.WriteString(styleDim.Render(baseLines[row]) + "\n")
		} else {
			resultSB.WriteString("\n")
		}
	}
	return resultSB.String()
}
