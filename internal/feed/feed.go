// Package feed 实现 RSS 2.0 / Atom / RDF 三种主流 feed 格式的容错解析。
package feed

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"regexp"
	"strings"
	"time"
)

// Item 是统一后的新闻条目。
type Item struct {
	Title     string
	URL       string
	Published time.Time // 无法解析时为零值
	Summary   string
	SourceID  string
	Lang      string
}

// ── RSS 2.0 ───────────────────────────────────────────────────

type rssFeed struct {
	XMLName xml.Name   `xml:"rss"`
	Channel rssChannel `xml:"channel"`
}
type rssChannel struct {
	Items []rssItem `xml:"item"`
}
type rssItem struct {
	Title       string `xml:"title"`
	Link        string `xml:"link"`
	GUID        string `xml:"guid"`
	PubDate     string `xml:"pubDate"`
	Description string `xml:"description"`
	Content     string `xml:"encoded"`
}

// ── Atom ──────────────────────────────────────────────────────

type atomFeed struct {
	XMLName xml.Name   `xml:"feed"`
	Entries []atomItem `xml:"entry"`
}
type atomItem struct {
	Title     string     `xml:"title"`
	Links     []atomLink `xml:"link"`
	ID        string     `xml:"id"`
	Published string     `xml:"published"`
	Updated   string     `xml:"updated"`
	Summary   string     `xml:"summary"`
	Content   string     `xml:"content"`
}
type atomLink struct {
	Href string `xml:"href,attr"`
	Rel  string `xml:"rel,attr"`
}

// ── RDF (RSS 1.0) ─────────────────────────────────────────────

type rdfFeed struct {
	XMLName xml.Name  `xml:"RDF"`
	Items   []rdfItem `xml:"item"`
}
type rdfItem struct {
	Title       string `xml:"title"`
	Link        string `xml:"link"`
	Date        string `xml:"date"`
	Description string `xml:"description"`
}

// Parse 解析 feed 内容，自动识别 RSS 2.0 / Atom / RDF 格式。
// 解析失败或格式不支持时返回错误。
func Parse(data []byte) ([]Item, error) {
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) == 0 {
		return nil, fmt.Errorf("空内容")
	}
	head := string(trimmed[:min(400, len(trimmed))])

	switch {
	case strings.Contains(head, "<feed"):
		return parseAtom(trimmed)
	case strings.Contains(head, "<rss"):
		return parseRSS(trimmed)
	case strings.Contains(head, "<RDF") || strings.Contains(head, "rdf:RDF"):
		return parseRDF(trimmed)
	default:
		// 兜底：依次尝试全部解析器
		for _, fn := range []func([]byte) ([]Item, error){parseRSS, parseAtom, parseRDF} {
			items, err := fn(trimmed)
			if err == nil && len(items) > 0 {
				return items, nil
			}
		}
		return nil, fmt.Errorf("无法识别的 feed 格式")
	}
}

func parseRSS(data []byte) ([]Item, error) {
	var f rssFeed
	if err := xml.Unmarshal(data, &f); err != nil {
		return nil, err
	}
	items := make([]Item, 0, len(f.Channel.Items))
	for _, it := range f.Channel.Items {
		link := strings.TrimSpace(it.Link)
		if link == "" {
			link = strings.TrimSpace(it.GUID)
		}
		if link == "" || strings.TrimSpace(it.Title) == "" {
			continue
		}
		desc := it.Description
		if desc == "" {
			desc = it.Content
		}
		items = append(items, Item{
			Title:     xmlDecode(strings.TrimSpace(it.Title)),
			URL:       strings.TrimSpace(link),
			Published: parseTime(it.PubDate),
			Summary:   htmlToText(desc),
		})
	}
	if len(items) == 0 {
		return nil, fmt.Errorf("RSS 无条目")
	}
	return items, nil
}

func parseAtom(data []byte) ([]Item, error) {
	var f atomFeed
	if err := xml.Unmarshal(data, &f); err != nil {
		return nil, err
	}
	items := make([]Item, 0, len(f.Entries))
	for _, it := range f.Entries {
		link := ""
		for _, l := range it.Links {
			if l.Rel == "" || l.Rel == "alternate" {
				if l.Href != "" {
					link = l.Href
					break
				}
			}
		}
		if link == "" && strings.HasPrefix(it.ID, "http") {
			link = it.ID
		}
		if link == "" || strings.TrimSpace(it.Title) == "" {
			continue
		}
		pub := it.Published
		if pub == "" {
			pub = it.Updated
		}
		desc := it.Summary
		if desc == "" {
			desc = it.Content
		}
		items = append(items, Item{
			Title:     xmlDecode(strings.TrimSpace(it.Title)),
			URL:       strings.TrimSpace(link),
			Published: parseTime(pub),
			Summary:   htmlToText(desc),
		})
	}
	if len(items) == 0 {
		return nil, fmt.Errorf("Atom 无条目")
	}
	return items, nil
}

func parseRDF(data []byte) ([]Item, error) {
	var f rdfFeed
	if err := xml.Unmarshal(data, &f); err != nil {
		return nil, err
	}
	items := make([]Item, 0, len(f.Items))
	for _, it := range f.Items {
		if strings.TrimSpace(it.Link) == "" || strings.TrimSpace(it.Title) == "" {
			continue
		}
		items = append(items, Item{
			Title:     xmlDecode(strings.TrimSpace(it.Title)),
			URL:       strings.TrimSpace(it.Link),
			Published: parseTime(it.Date),
			Summary:   htmlToText(it.Description),
		})
	}
	if len(items) == 0 {
		return nil, fmt.Errorf("RDF 无条目")
	}
	return items, nil
}

// xmlDecode 处理 XML 实体（&amp; 等已被 encoding/xml 解码，这里再兜底 CDATA 场景）。
func xmlDecode(s string) string {
	return strings.TrimSpace(s)
}

// ── 时间解析 ──────────────────────────────────────────────────

var timeLayouts = []string{
	time.RFC1123Z, time.RFC1123, time.RFC822Z, time.RFC822,
	time.RFC3339, time.RFC3339Nano,
	"Mon, 02 Jan 2006 15:04:05 -0700",
	"Mon, 2 Jan 2006 15:04:05 -0700",
	"Mon, 2 Jan 2006 15:04:05 MST",
	"02 Jan 2006 15:04:05 -0700",
	"2 Jan 2006 15:04:05 -0700",
	"2 Jan 2006 15:04:05 MST",
	"2006-01-02T15:04:05Z07:00",
	"2006-01-02T15:04:05",
	"2006-01-02 15:04:05",
	"2006-01-02",
	"2006/01/02 15:04:05",
	"Mon Jan 2 15:04:05 2006",
}

// 未知时区缩写 → 偏移映射（Go 的 time 包只识别少量缩写，其余按此表纠正）。
// 只收录无歧义常见缩写；歧义的（如 IST）不收录。
var tzOffsets = map[string]int{
	"CET": 1 * 3600, "CEST": 2 * 3600,
	"BST": 1 * 3600,
	"EET": 2 * 3600, "EEST": 3 * 3600,
	"MSK": 3 * 3600,
	"GST": 4 * 3600,
	"PKT": 5 * 3600,
	"WIB": 7 * 3600, "SGT": 8 * 3600, "HKT": 8 * 3600, "CST-CHINA": 8 * 3600,
	"JST": 9 * 3600, "KST": 9 * 3600,
	"AEST": 10 * 3600, "AEDT": 11 * 3600,
	"EST": -5 * 3600, "EDT": -4 * 3600,
	"CST": -6 * 3600, "CDT": -5 * 3600,
	"MST": -7 * 3600, "MDT": -6 * 3600,
	"PST": -8 * 3600, "PDT": -7 * 3600,
	"AKST": -9 * 3600, "AKDT": -8 * 3600,
	"HST": -10 * 3600,
}

var tzAbbrevRe = regexp.MustCompile(`\s+([A-Z]{2,5})$`)

// 无时区的布局（配合 ParseInLocation 使用）
var noZoneLayouts = []string{
	"Mon, 02 Jan 2006 15:04:05",
	"Mon, 2 Jan 2006 15:04:05",
	"02 Jan 2006 15:04:05",
	"2 Jan 2006 15:04:05",
	"2006-01-02T15:04:05",
	"2006-01-02 15:04:05",
	"2006/01/02 15:04:05",
	"Mon Jan 2 15:04:05 2006",
	"2006-01-02",
}

// parseTime 尝试多种布局解析时间；全部失败返回零值。
// 顺序：已知缩写映射（纠正 Go 把未知缩写当 UTC 的偏差）→ 通用布局 → 失败。
func parseTime(s string) time.Time {
	s = strings.TrimSpace(s)
	if s == "" {
		return time.Time{}
	}
	// 已知时区缩写（如 CEST=+2）：先查表，避免 Go 以偏移 0 解析
	if m := tzAbbrevRe.FindStringSubmatch(s); m != nil {
		if off, ok := tzOffsets[m[1]]; ok {
			noTZ := tzAbbrevRe.ReplaceAllString(s, "")
			zone := time.FixedZone(m[1], off)
			for _, layout := range noZoneLayouts {
				if t, err := time.ParseInLocation(layout, noTZ, zone); err == nil {
					return t
				}
			}
		}
	}
	for _, layout := range timeLayouts {
		if t, err := time.Parse(layout, s); err == nil {
			return t
		}
	}
	return time.Time{}
}

// ── HTML 摘要清洗 ─────────────────────────────────────────────

var (
	tagRe    = regexp.MustCompile(`<[^>]+>`)
	spaceRe  = regexp.MustCompile(`\s+`)
	entityRe = regexp.MustCompile(`&(nbsp|#160|#xa0);`)
	scriptRe = regexp.MustCompile(`(?is)<script[^>]*>.*?</script>`)
	styleRe  = regexp.MustCompile(`(?is)<style[^>]*>.*?</style>`)
)

func htmlToText(s string) string {
	if s == "" {
		return ""
	}
	s = scriptRe.ReplaceAllString(s, " ")
	s = styleRe.ReplaceAllString(s, " ")
	s = tagRe.ReplaceAllString(s, " ")
	s = entityRe.ReplaceAllString(s, " ")
	s = strings.ReplaceAll(s, "&amp;", "&")
	s = strings.ReplaceAll(s, "&quot;", `"`)
	s = strings.ReplaceAll(s, "&#39;", "'")
	s = strings.ReplaceAll(s, "&apos;", "'")
	s = strings.ReplaceAll(s, "&lt;", "<")
	s = strings.ReplaceAll(s, "&gt;", ">")
	s = spaceRe.ReplaceAllString(s, " ")
	return strings.TrimSpace(s)
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
