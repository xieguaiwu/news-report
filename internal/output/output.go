// Package output 渲染报告：终端（彩色）/ Markdown / JSON。
package output

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"

	"news-report/internal/classify"
	"news-report/internal/report"
)

const (
	cReset  = "\x1b[0m"
	cBold   = "\x1b[1m"
	cDim    = "\x1b[2m"
	cRed    = "\x1b[31m"
	cGreen  = "\x1b[32m"
	cYellow = "\x1b[33m"
	cBlue   = "\x1b[34m"
	cCyan   = "\x1b[36m"
	cGray   = "\x1b[90m"
)

var catColor = map[classify.Category]string{
	classify.Politics:  cBlue,
	classify.Economy:   cGreen,
	classify.Industry:  cYellow,
	classify.EduPolicy: cCyan,
	classify.Other:     cGray,
}

var catIcon = map[classify.Category]string{
	classify.Politics:  "🏛",
	classify.Economy:   "📈",
	classify.Industry:  "🏭",
	classify.EduPolicy: "🎓",
	classify.Other:     "•",
}

// Terminal 输出彩色终端报告。
func Terminal(w io.Writer, r *report.Report, color bool) {
	ok, fail := 0, 0
	for _, s := range r.SourceStats {
		if s.OK {
			ok++
		} else if s.Err != "" {
			fail++
		}
	}
	title := r.Config.ReportTitle
	if title == "" {
		title = "News Report"
	}
	b := bold(title, color) + "  " + dim(r.Generated.Format("2006-01-02 15:04"), color)
	fmt.Fprintf(w, "%s | 来源 %d OK / %d 失败 | 抓取 %d → 去重 %d → 新 %d | 耗时 %s\n\n",
		b, ok, fail, r.RawCount, r.DupRemoved, len(r.Items), r.Duration.Round(time.Millisecond))

	last := classify.Other
	idx := map[classify.Category]int{}
	for _, it := range r.Items {
		if it.Category != last {
			last = it.Category
			icon := catIcon[it.Category]
			head := fmt.Sprintf("%s %s", icon, strings.ToUpper(string(it.Category)))
			fmt.Fprintf(w, "\n%s\n%s\n", bold(head, color), dim(strings.Repeat("─", 56), color))
		}
		idx[it.Category]++
		n := idx[it.Category]
		cat := ""
		if color {
			cat = catColor[it.Category]
		}
		lang := strings.ToUpper(it.Lang)
		line := fmt.Sprintf("%2d. %s%s%s — %s · %s · %s",
			n, cat, it.Title, cResetIf(color, cat), it.SourceName, it.AgeLabel, lang)
		fmt.Fprintln(w, line)
		if color {
			fmt.Fprintf(w, "    %s%s%s\n", cDim, it.URL, cReset)
		} else {
			fmt.Fprintf(w, "    %s\n", it.URL)
		}
		if it.Body != "" {
			fmt.Fprintf(w, "    %s[全文已抓取 · 摘要]%s\n", cDim, cReset)
		}
	}
	fmt.Fprintf(w, "\n%s\n", dim(strings.Repeat("─", 56), color))
	for _, s := range r.SourceStats {
		if s.OK {
			continue
		}
		if s.Err != "" {
			if color {
				fmt.Fprintf(w, "%s✗ %s%s %s: %s\n", cRed, cReset, s.ID, s.Err, "")
			} else {
				fmt.Fprintf(w, "✗ %s %s: %s\n", s.ID, s.Err, "")
			}
		}
	}
}

// Markdown 输出 Markdown 报告（适合存档与分享）。
func Markdown(w io.Writer, r *report.Report) {
	ok, fail := 0, 0
	for _, s := range r.SourceStats {
		if s.OK {
			ok++
		} else if s.Err != "" {
			fail++
		}
	}
	fmt.Fprintf(w, "# %s\n\n", r.Config.ReportTitle)
	fmt.Fprintf(w, "> 生成时间：%s ｜ 来源：%d OK / %d 失败 ｜ 抓取 %d 条 → 去重后 %d 条 ｜ 耗时 %s\n\n",
		r.Generated.Format("2006-01-02 15:04:05 -0700"), ok, fail,
		r.RawCount, len(r.Items), r.Duration.Round(time.Millisecond))

	last := classify.Other
	idx := map[classify.Category]int{}
	for _, it := range r.Items {
		if it.Category != last {
			last = it.Category
			fmt.Fprintf(w, "\n## %s %s\n\n", catIcon[it.Category], strings.ToUpper(string(it.Category)))
		}
		idx[it.Category]++
		fmt.Fprintf(w, "**%d. %s**  \n", idx[it.Category], it.Title)
		fmt.Fprintf(w, "`%s` · `%s` · `%s` · %s ago  \n",
			it.SourceName, strings.ToUpper(it.Lang), it.Tier, it.AgeLabel)
		if s := strings.TrimSpace(it.Summary); s != "" {
			fmt.Fprintf(w, "%s  \n", truncate(s, 220))
		}
		if it.Body != "" {
			fmt.Fprintf(w, "\n<details><summary>全文摘要（%d 字）</summary>\n\n%s\n\n</details>\n",
				len(it.Body), truncate(it.Body, 1200))
		}
		fmt.Fprintf(w, "[原文链接](%s)\n\n", it.URL)
	}

	fmt.Fprintf(w, "\n---\n\n## 来源统计\n\n| 来源 | 语言 | 层级 | 抓取条数 | 状态 |\n|---|---|---|---|---|\n")
	for _, s := range r.SourceStats {
		status := "✓"
		if !s.OK {
			status = "✗ " + s.Err
		} else if s.UsedScrape {
			status = "✓ (scrape)"
		}
		fmt.Fprintf(w, "| %s | %s | %s | %d | %s |\n", s.Name, strings.ToUpper(s.Lang), s.Tier, s.Items, status)
	}
	fmt.Fprintf(w, "\n*由 news-report v%s 自动生成*\n", version())
}

// JSON 输出结构化报告。
func JSON(w io.Writer, r *report.Report) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(r)
}

func bold(s string, color bool) string {
	if color {
		return cBold + s + cReset
	}
	return s
}

func dim(s string, color bool) string {
	if color {
		return cDim + s + cReset
	}
	return s
}

func cResetIf(color bool, c string) string {
	if color {
		return cReset
	}
	return ""
}

func truncate(s string, n int) string {
	if n <= 0 {
		return s
	}
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n]) + "…"
}

func version() string { return "0.4.0" }
