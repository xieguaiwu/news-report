// news-report — 欧美权威媒体新闻聚合器（英/德/法，专注国际政策/经济/产业）。
//
// 用法：
//
//	news-report                    抓取并输出终端报告（默认最近 24h）
//	news-report read <url>         深度阅读：抓取网页并提取正文
//	news-report sources            列出消息源
//	news-report sources --live     实测消息源可用性
//	news-report init               生成默认配置文件
//	news-report version            版本信息
package main

import (
	"context"
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
	"news-report/internal/output"
	"news-report/internal/report"
	"news-report/internal/sources"
)

func main() {
	if len(os.Args) < 2 {
		runReport(os.Args[1:])
		return
	}
	switch os.Args[1] {
	case "read":
		runRead(os.Args[2:])
	case "sources":
		runSources(os.Args[2:])
	case "init":
		runInit(os.Args[2:])
	case "version", "-v", "--version":
		fmt.Printf("news-report v%s\n", config.Version)
	case "help", "-h", "--help":
		usage()
	default:
		runReport(os.Args[1:])
	}
}

func usage() {
	fmt.Print(`news-report — 欧美权威媒体新闻聚合器（EN/DE/FR · 政治/经济/产业）

用法:
  news-report [flags]               抓取并生成报告（默认终端输出）
  news-report read <url> [flags]    深度阅读：抓取网页并提取正文
  news-report sources [--live]      列出消息源；--live 实测可用性
  news-report init                  生成默认配置文件 (~/.config/news-report/config.yaml)

报告 flags:
  --lang en,de,fr        语言过滤（默认全部）
  --cat politics,economy,industry   分类过滤（默认全部）
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
  --lang en|de|fr        Accept-Language（默认 en）
  --max-chars N          截断正文长度（0 = 不截断）
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
	_ = fs.Parse(args)
	if fs.NArg() < 1 {
		fatalMsg("用法: news-report read <url> [--lang en|de|fr] [--max-chars N]")
	}
	url := fs.Arg(0)
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
