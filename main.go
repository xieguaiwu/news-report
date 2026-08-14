// news-report — 欧美权威媒体新闻聚合器（英/德/法/中，专注国际政策/美国政治/经济/产业）。
//
// 用法：
//
//	news-report                    抓取并输出终端报告（默认最近 24h）
//	news-report ui                 交互式终端界面（TUI）
//	news-report read <url>         深度阅读：抓取网页并提取正文
//	news-report find "关键词"       搜索 Google News 找付费文章的免费转载/镜像
//	news-report sources            列出消息源
//	news-report sources --live     实测消息源可用性
//	news-report init               生成默认配置文件
//	news-report version            版本信息
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"news-report/internal/article"
	"news-report/internal/config"
	"news-report/internal/feed"
	"news-report/internal/fetch"
	"news-report/internal/gnews"
	"news-report/internal/output"
	"news-report/internal/report"
	"news-report/internal/sources"
	"news-report/internal/tui"
)

func main() {
	if len(os.Args) < 2 {
		runReport(os.Args[1:])
		return
	}
	switch os.Args[1] {
	case "read":
		runRead(os.Args[2:])
	case "find":
		runFind(os.Args[2:])
	case "ui", "tui":
		runUI(os.Args[2:])
	case "sources":
		runSources(os.Args[2:])
	case "init":
		runInit(os.Args[2:])
	case "cache":
		runCache(os.Args[2:])
	case "version", "-v", "--version":
		fmt.Printf("news-report v%s\n", config.Version)
	case "help", "-h", "--help":
		usage()
	default:
		runReport(os.Args[1:])
	}
}

func usage() {
	fmt.Print(`news-report — 欧美权威媒体新闻聚合器（EN/DE/FR/ZH · 美国政治/国际政策/经济/产业）

用法:
  news-report [flags]               抓取并生成报告（默认终端输出）
  news-report ui                    交互式终端界面（TUI）
  news-report read <url> [flags]    深度阅读：抓取网页并提取正文
  news-report find "关键词" [flags]  搜索免费转载/镜像（付费墙文章）
  news-report sources [--live]      列出消息源；--live 实测可用性
  news-report cache [stat|clear]    查看缓存状态 / 清空缓存
  news-report init                  生成默认配置文件 (~/.config/news-report/config.yaml)

报告 flags:
  --lang en,de,fr,zh    语言过滤（默认全部）
  --cat uspolitics,politics,economy,industry   分类过滤（默认全部）
  --sources id1,id2      仅抓取指定来源
  --minutes N            新鲜度窗口（分钟，默认 1440；0 = 不限）
  --limit N              每分类条数上限（默认 12）
  --total N              总条数上限（默认 80）
  --strict               只显示专注分类（隐藏 other）
  --fulltext N           每个分类抓取前 N 条全文（深度模式）
  --show-seen            重复显示已报告条目
  --google-news          启用谷歌新闻聚合源（广度补充）
  --out terminal|markdown|json   输出格式（默认 terminal）
  --outfile PATH         同时写入文件
  --proxy URL            代理地址（默认环境变量）
  --concurrency N        并发数（默认 12）
  --timeout N            单请求超时秒数（默认 15）
  --no-color             禁用终端颜色
  --no-robots            跳过 robots.txt 检查
  --config PATH          配置文件路径

read flags:
  --lang en|de|fr|zh     Accept-Language（默认 en）
  --max-chars N          截断正文长度（0 = 不截断）

find flags:
  --lang en|de|fr|zh     搜索语言（默认 en）
  --limit N              结果上限（默认 10）
  --exclude DOMAIN       排除原站域名（标记原站）
`)
}

// ── 报告 ──────────────────────────────────────────────────────

func runReport(args []string) {
	fs := flag.NewFlagSet("report", flag.ExitOnError)
	var (
		langs     = fs.String("lang", "", "")
		cats      = fs.String("cat", "", "")
		srcIDs    = fs.String("sources", "", "")
		minutes   = fs.Int("minutes", -1, "")
		limit     = fs.Int("limit", 0, "")
		total     = fs.Int("total", 0, "")
		strict    = fs.Bool("strict", false, "")
		fulltext  = fs.Int("fulltext", 0, "")
		showSeen  = fs.Bool("show-seen", false, "")
		gn        = fs.Bool("google-news", false, "")
		outFormat = fs.String("out", "terminal", "")
		outfile   = fs.String("outfile", "", "")
		proxy     = fs.String("proxy", "", "")
		conc      = fs.Int("concurrency", 0, "")
		timeout   = fs.Int("timeout", 0, "")
		noColor   = fs.Bool("no-color", false, "")
		noRobots  = fs.Bool("no-robots", false, "")
		noCache   = fs.Bool("no-cache", false, "")
		cfgPath   = fs.String("config", "", "")
	)
	fs.Usage = func() {}
	_ = fs.Parse(args)

	cfg, err := config.Load(*cfgPath)
	if err != nil {
		fatal(err)
	}
	applyFlags(cfg, langs, cats, srcIDs, minutes, limit, total, strict, fulltext,
		showSeen, gn, proxy, conc, timeout)
	if err := cfg.Validate(); err != nil {
		fatal(err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	fetcher := fetch.New(fetch.Options{
		Proxy:     cfg.Proxy,
		UserAgent: cfg.UserAgent,
		Timeout:   time.Duration(cfg.TimeoutSec) * time.Second,
		Retries:   cfg.Retries,
		NoRobots:  *noRobots,
	})
	if *noCache {
		cfg.NoCache = true
	}

	opts := report.Options{
		Window:      cfg.Window(),
		LimitPerCat: cfg.LimitPerCat,
		TotalLimit:  cfg.TotalLimit,
		StrictFocus: cfg.StrictFocus || *strict,
		ShowSeen:    cfg.ShowSeen || *showSeen,
		Fulltext:    *fulltext,
		FulltextMax: cfg.FulltextMax,
		GoogleNews:  cfg.GoogleNews || *gn,
	}
	if *langs != "" {
		opts.Languages = split(*langs)
	}
	if *cats != "" {
		opts.Categories = split(*cats)
		opts.CatFilter = true // CLI 显式 --cat = 硬过滤（只看这些分类）
	}
	if *srcIDs != "" {
		opts.SourceIDs = split(*srcIDs)
	}

	rep, err := report.Run(ctx, cfg, fetcher, opts)
	if err != nil {
		fatal(err)
	}

	color := !*noColor && os.Getenv("NO_COLOR") == "" && isTerminal()
	switch *outFormat {
	case "json":
		if err := output.JSON(os.Stdout, rep); err != nil {
			fatal(err)
		}
	case "markdown", "md":
		output.Markdown(os.Stdout, rep)
	default:
		output.Terminal(os.Stdout, rep, color)
	}
	if *outfile != "" {
		f, err := os.Create(*outfile)
		if err != nil {
			fatal(err)
		}
		switch *outFormat {
		case "json":
			_ = output.JSON(f, rep)
		case "markdown", "md":
			output.Markdown(f, rep)
		default:
			output.Terminal(f, rep, false)
		}
		f.Close()
		fmt.Fprintf(os.Stderr, "已写入 %s\n", *outfile)
	}
}

func applyFlags(cfg *config.Config, langs, cats, srcIDs *string, minutes, limit, total *int,
	strict *bool, fulltext *int, showSeen, gn *bool, proxy *string, conc, timeout *int) {
	// 注意：sources 与分类的过滤在 report.Options 中处理；这里只做配置层覆盖
	if *minutes >= 0 {
		cfg.Minutes = *minutes
	}
	if *limit > 0 {
		cfg.LimitPerCat = *limit
	}
	if *total > 0 {
		cfg.TotalLimit = *total
	}
	if *proxy != "" {
		cfg.Proxy = *proxy
	}
	if *conc > 0 {
		cfg.Concurrency = *conc
	}
	if *timeout > 0 {
		cfg.TimeoutSec = *timeout
	}
	_ = langs
	_ = cats
	_ = srcIDs
	_ = strict
	_ = fulltext
	_ = showSeen
	_ = gn
}

// ── read ──────────────────────────────────────────────────────

func runRead(args []string) {
	fs := flag.NewFlagSet("read", flag.ExitOnError)
	lang := fs.String("lang", "en", "")
	maxChars := fs.Int("max-chars", 0, "")
	proxy := fs.String("proxy", "", "")
	cfgPath := fs.String("config", "", "")
	args = reorderArgs(args)
	_ = fs.Parse(args)
	if len(fs.Args()) < 1 {
		fatalMsg("用法: news-report read <url> [--lang en|de|fr|zh] [--max-chars N]")
	}
	url := fs.Args()[0]
	if !strings.HasPrefix(url, "http://") && !strings.HasPrefix(url, "https://") {
		fatalMsg("URL 必须以 http(s):// 开头")
	}
	cfg, err := config.Load(*cfgPath)
	if err != nil {
		fatal(err)
	}
	if *proxy != "" {
		cfg.Proxy = *proxy
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	fetcher := fetch.New(fetch.Options{
		Proxy:     cfg.Proxy,
		UserAgent: cfg.UserAgent,
		Timeout:   20 * time.Second,
		Retries:   2,
	})
	art, err := article.Extract(ctx, fetcher, url, *lang)
	if err != nil {
		if errors.Is(err, article.ErrPaywall) {
			fmt.Fprintf(os.Stderr, "提示: %v\n", err)
			os.Exit(1)
		}
		fatal(err)
	}
	fmt.Printf("# %s\n\n", art.Title)
	if art.Byline != "" {
		fmt.Printf("**%s**\n\n", art.Byline)
	}
	fmt.Printf("来源: %s\n", url)
	if art.Published != "" {
		fmt.Printf("发布时间: %s\n", art.Published)
	}
	fmt.Printf("提取: %s\n\n", art.ExtractedAt.Format("2006-01-02 15:04"))
	body := art.Text
	if *maxChars > 0 && len(body) > *maxChars {
		body = body[:*maxChars] + "…"
	}
	fmt.Println(body)
}

// ── find（付费墙转载搜索） ────────────────────────────────────

func runFind(args []string) {
	fs := flag.NewFlagSet("find", flag.ExitOnError)
	lang := fs.String("lang", "en", "")
	limit := fs.Int("limit", 10, "")
	exclude := fs.String("exclude", "", "")
	cfgPath := fs.String("config", "", "")
	args = reorderArgs(args)
	_ = fs.Parse(args)
	if len(fs.Args()) < 1 {
		fatalMsg("用法: news-report find \"标题关键词\" [--lang en|de|fr|zh] [--exclude 原站域名]")
	}
	query := strings.TrimSpace(strings.Join(fs.Args(), " "))
	if query == "" {
		fatalMsg("搜索词不能为空")
	}

	cfg, err := config.Load(*cfgPath)
	if err != nil {
		fatal(err)
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	fetcher := fetch.New(fetch.Options{
		Proxy:     cfg.Proxy,
		UserAgent: cfg.UserAgent,
		Timeout:   15 * time.Second,
		Retries:   2,
	})

	results, err := gnews.Search(ctx, fetcher, query, *lang)
	if err != nil {
		fatal(err)
	}
	results = gnews.ExcludeOriginal(results, *exclude)
	if *limit > 0 && len(results) > *limit {
		results = results[:*limit]
	}
	if len(results) == 0 {
		fmt.Println("未找到相关报道")
		return
	}
	fmt.Printf("🔍 「%s」 的报道/转载候选（%d 条）：\n\n", query, len(results))
	for i, r := range results {
		mark := ""
		if r.IsOriginal {
			mark = " [原站]"
		}
		age := ""
		if !r.Published.IsZero() {
			age = " · " + r.Published.Format("01-02 15:04")
		}
		fmt.Printf("%2d. %s%s\n    %s%s%s\n", i+1, r.Title, mark, r.SourceName, age, r.SourceURL)
		fmt.Printf("    %s\n", r.Link)
	}
	fmt.Printf("\n提示: 链接为 Google News 跳转链接，浏览器打开后自动跳转原文；\n")
	fmt.Printf("      付费墙文章优先选 [原站] 以外的转载媒体。\n")
}

// ── ui（TUI） ────────────────────────────────────────────────

func runUI(args []string) {
	fs := flag.NewFlagSet("ui", flag.ExitOnError)
	langs := fs.String("lang", "", "")
	cats := fs.String("cat", "", "")
	srcIDs := fs.String("sources", "", "")
	minutes := fs.Int("minutes", -1, "")
	proxy := fs.String("proxy", "", "")
	cfgPath := fs.String("config", "", "")
	llmModel := fs.String("llm-model", "", "LLM 模型覆写（如 gpt-4o）")
	noLLM := fs.Bool("no-llm", false, "禁用 LLM 功能")
	args = reorderArgs(args)
	_ = fs.Parse(args)

	cfg, err := config.Load(*cfgPath)
	if err != nil {
		fatal(err)
	}
	if *langs != "" {
		cfg.Languages = split(*langs)
	}
	if *cats != "" {
		cfg.Categories = split(*cats)
	}
	if *minutes >= 0 {
		cfg.Minutes = *minutes
	}
	if *proxy != "" {
		cfg.Proxy = *proxy
	}
	// LLM CLI 覆写
	if *noLLM {
		cfg.LLM.APIKey = ""
	}
	if *llmModel != "" {
		cfg.LLM.Model = *llmModel
	}
	if err := cfg.Validate(); err != nil {
		fatal(err)
	}
	fetcher := fetch.New(fetch.Options{
		Proxy:     cfg.Proxy,
		UserAgent: cfg.UserAgent,
		Timeout:   time.Duration(cfg.TimeoutSec) * time.Second,
		Retries:   cfg.Retries,
	})
	opts := report.Options{}
	if *srcIDs != "" {
		opts.SourceIDs = split(*srcIDs)
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := tui.Run(ctx, cfg, fetcher, opts); err != nil {
		fatal(err)
	}
}

// ── cache ────────────────────────────────────────────────────

func runCache(args []string) {
	if len(args) == 0 {
		fmt.Println("用法:")
		fmt.Println("  news-report cache stat    查看缓存状态")
		fmt.Println("  news-report cache clear   清空全部缓存（feed + 文章 + 已读记录）")
		return
	}
	cfg, _ := config.Load("")
	cd := cfg.CachePath()

	switch args[0] {
	case "stat":
		cacheStat(cd)
	case "clear":
		var removed int64
		for _, sub := range []string{"feed_cache", "articles", "llm", "seen.json"} {
			target := cd + "/" + sub
			sz := dirSize(target)
			if sz > 0 {
				if err := os.RemoveAll(target); err == nil {
					removed += sz
					fmt.Printf("  已清空 %s (%s)\n", sub, formatBytes(sz))
				} else {
					fmt.Printf("  清空失败 %s: %v\n", sub, err)
				}
			}
		}
		if removed > 0 {
			fmt.Printf("\n共释放 %s\n", formatBytes(removed))
		} else {
			fmt.Println("缓存已为空")
		}
	default:
		fmt.Printf("未知子命令: %s\n", args[0])
		fmt.Println("用法: news-report cache [stat|clear]")
	}
}

// ── sources ───────────────────────────────────────────────────

func runSources(args []string) {
	fs := flag.NewFlagSet("sources", flag.ExitOnError)
	live := fs.Bool("live", false, "")
	lang := fs.String("lang", "", "")
	cfgPath := fs.String("config", "", "")
	_ = fs.Parse(args)
	cfg, err := config.Load(*cfgPath)
	if err != nil {
		fatal(err)
	}

	all := sources.Defaults()
	enabled := map[string]bool{}
	for _, s := range all {
		enabled[s.ID] = true
		if ov, ok := cfg.Sources[s.ID]; ok && ov.Enabled != nil {
			enabled[s.ID] = *ov.Enabled
		}
	}

	fmt.Printf("%-28s %-24s %-4s %-10s %-6s %s\n", "ID", "名称", "语言", "层级", "权重", "Feed 数")
	fmt.Println(strings.Repeat("─", 90))
	for _, s := range all {
		if *lang != "" && s.Lang != *lang {
			continue
		}
		mark := "✓"
		if !enabled[s.ID] {
			mark = "✗"
		}
		opt := ""
		if s.Optional {
			opt = " (候选)"
		}
		fmt.Printf("%-28s %-24s %-4s %-10s %-6.2f %d%s %s\n",
			s.ID, trunc(s.Name, 24), s.Lang, s.Tier, s.Weight, len(s.Feeds), opt, mark)
	}

	if *live {
		fmt.Printf("\n—— 实测可用性 ——\n")
		fetcher := fetch.New(fetch.Options{UserAgent: cfg.UserAgent, Timeout: 8 * time.Second, Retries: 0, NoRobots: true})
		ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
		defer stop()
		lctx, cancel := context.WithTimeout(ctx, 90*time.Second) // 总超时，防死锁/挂死
		defer cancel()

		ids := map[string]bool{}
		for _, id := range split(fs.Arg(0)) {
			ids[id] = true
		}
		type result struct {
			id  string
			ok  bool
			err string
			n   int
		}
		var targets []sources.Source
		for _, s := range all {
			if len(ids) > 0 && !ids[s.ID] {
				continue
			}
			if !enabled[s.ID] {
				continue
			}
			targets = append(targets, s)
		}
		// 契约：每个 goroutine 恰好向 results 发送一次（成功或失败），
		// 接收循环按 len(targets) 计数——若未来改为逐 feed 发送必须同步修改。
		results := make(chan result, len(targets))
		for _, s := range targets {
			go func(s sources.Source) {
				r := result{id: s.ID}
				for _, u := range s.Feeds {
					cctx, cancel := context.WithTimeout(lctx, 8*time.Second)
					data, err := fetcher.Bytes(cctx, u)
					cancel()
					if err != nil {
						r.err = err.Error()
						continue
					}
					if n := feedCount(data); n > 0 {
						r.ok, r.n = true, n
						break
					}
					r.err = "无条目"
				}
				results <- r // 每个 goroutine 恰好发送一次
			}(s)
		}
		for i := 0; i < len(targets); i++ {
			select {
			case r := <-results:
				if r.ok {
					fmt.Printf("  ✓ %-28s %d 条\n", r.id, r.n)
				} else {
					fmt.Printf("  ✗ %-28s %s\n", r.id, trunc(r.err, 60))
				}
			case <-lctx.Done():
				fmt.Println("  … 已超时，跳过剩余检查")
				return
			}
		}
	}
}

func feedCount(data []byte) int {
	items, err := feed.Parse(data)
	if err != nil {
		return 0
	}
	return len(items)
}

// ── init ──────────────────────────────────────────────────────

func runInit(args []string) {
	fs := flag.NewFlagSet("init", flag.ExitOnError)
	path := fs.String("config", "", "")
	_ = fs.Parse(args)
	cfg := config.Default()
	if err := cfg.Save(*path); err != nil {
		fatal(err)
	}
	fmt.Printf("已生成默认配置: %s\n", resolvePath(*path))
	fmt.Println("可编辑：languages / categories / sources.<id>.enabled 等字段")
}

func resolvePath(p string) string {
	if p == "" {
		p = config.DefaultConfigPath
	}
	if strings.HasPrefix(p, "~/") {
		if home, err := os.UserHomeDir(); err == nil {
			return home + p[1:]
		}
	}
	return p
}

// ── 工具 ──────────────────────────────────────────────────────

// reorderArgs 把「位置参数在前、flags 在后」的参数序列重排为 flags 在前，
// 解决 Go flag 包在第一个非 flag 参数处停止解析的问题（如 read <url> --lang de）。
func reorderArgs(args []string) []string {
	var ordered, pos []string
	endOfFlags := false
	for i := 0; i < len(args); {
		a := args[i]
		if a == "--" {
			endOfFlags = true
			pos = append(pos, a)
			i++
			continue
		}
		if !endOfFlags && strings.HasPrefix(a, "-") && a != "-" {
			ordered = append(ordered, a)
			i++
			// flag 值：下一个参数若非 flag 开头则视为本 flag 的值
			if i < len(args) && !strings.HasPrefix(args[i], "-") {
				ordered = append(ordered, args[i])
				i++
			}
		} else {
			pos = append(pos, a)
			i++
		}
	}
	return append(ordered, pos...)
}

func split(s string) []string {
	parts := strings.Split(s, ",")
	out := parts[:0]
	for _, p := range parts {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

func trunc(s string, n int) string {
	if n <= 0 {
		return s
	}
	if len(s) <= n {
		return s
	}
	return s[:n-1] + "…"
}

func fatal(err error) {
	fmt.Fprintf(os.Stderr, "错误: %v\n", err)
	os.Exit(1)
}

func fatalMsg(msg string) {
	fmt.Fprintf(os.Stderr, "错误: %s\n", msg)
	os.Exit(1)
}

func isTerminal() bool {
	fi, err := os.Stdout.Stat()
	if err != nil {
		return false
	}
	return fi.Mode()&os.ModeCharDevice != 0
}

// ── cache 辅助 ─────────────────────────────────────────────────

func cacheStat(dir string) {
	fmt.Println("新闻缓存目录:", dir)
	fmt.Println()

	subdirs := []struct {
		name string
		desc string
	}{
		{"feed_cache", "Feed 响应缓存（TTL 按来源 6-30min）"},
		{"articles", "文章正文缓存（TTL 24h）"},
		{"llm", "LLM 翻译/摘要缓存（TTL 7 天）"},
		{"seen.json", "已读记录（7 天自动清理）"},
	}

	var totalSize int64
	for _, s := range subdirs {
		target := dir + "/" + s.name
		sz := dirSize(target)
		totalSize += sz
		if sz == 0 && s.name != "seen.json" {
			fmt.Printf("  %-15s （空）\n", s.name)
		} else if sz > 0 {
			files := countFiles(target)
			fmt.Printf("  %-15s %s  %d 文件  —  %s\n", s.name, formatBytes(sz), files, s.desc)
		}
	}
	fmt.Printf("\n总计: %s\n", formatBytes(totalSize))
}

// dirSize 递归计算目录/文件大小（不存在返回 0）。
func dirSize(path string) int64 {
	var total int64
	fi, err := os.Stat(path)
	if err != nil {
		return 0
	}
	if !fi.IsDir() {
		return fi.Size()
	}
	entries, err := os.ReadDir(path)
	if err != nil {
		return 0
	}
	for _, e := range entries {
		info, err := e.Info()
		if err != nil {
			continue
		}
		if e.IsDir() {
			total += dirSize(path + "/" + e.Name())
		} else {
			total += info.Size()
		}
	}
	return total
}

func countFiles(path string) int {
	fi, err := os.Stat(path)
	if err != nil {
		return 0
	}
	if !fi.IsDir() {
		return 1
	}
	entries, _ := os.ReadDir(path)
	return len(entries)
}

func formatBytes(n int64) string {
	const unit = 1024
	if n < unit {
		return fmt.Sprintf("%d B", n)
	}
	div, exp := int64(unit), 0
	for n2 := n / unit; n2 >= unit; n2 /= unit {
		div *= unit
		exp++
	}
	suffixes := []string{"KB", "MB", "GB"}
	return fmt.Sprintf("%.1f %s", float64(n)/float64(div), suffixes[exp])
}
