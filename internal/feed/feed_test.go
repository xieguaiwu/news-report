package feed

import (
	"strings"
	"testing"
	"time"
)

func TestParseRSS(t *testing.T) {
	xml := `<?xml version="1.0"?>
<rss version="2.0" xmlns:dc="http://purl.org/dc/elements/1.1/">
<channel><title>Test</title>
<item>
  <title>EU reaches new trade deal</title>
  <link>https://example.com/eu-trade</link>
  <guid>https://example.com/eu-trade</guid>
  <pubDate>Mon, 01 Aug 2026 10:00:00 GMT</pubDate>
  <description>&lt;p&gt;The EU and partners signed a &amp;amp; new agreement.&lt;/p&gt;</description>
</item>
<item>
  <title>Central bank keeps rates on hold</title>
  <link>https://example.com/rates</link>
  <pubDate>Sun, 31 Jul 2026 22:30:00 +0200</pubDate>
  <description>Markets reacted calmly.</description>
</item>
</channel></rss>`
	items, err := Parse([]byte(xml))
	if err != nil {
		t.Fatalf("Parse RSS: %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("期望 2 条，实际 %d", len(items))
	}
	it := items[0]
	if it.Title != "EU reaches new trade deal" {
		t.Errorf("标题错误: %q", it.Title)
	}
	if it.URL != "https://example.com/eu-trade" {
		t.Errorf("URL 错误: %q", it.URL)
	}
	want := time.Date(2026, 8, 1, 10, 0, 0, 0, time.UTC)
	if !it.Published.Equal(want) {
		t.Errorf("时间错误: %v，期望 %v", it.Published, want)
	}
	if !strings.Contains(it.Summary, "&") || strings.Contains(it.Summary, "<p>") {
		t.Errorf("摘要清洗错误: %q", it.Summary)
	}
	if !items[1].Published.Equal(time.Date(2026, 7, 31, 20, 30, 0, 0, time.UTC)) {
		t.Errorf("时区偏移解析错误: %v", items[1].Published)
	}
}

func TestParseAtom(t *testing.T) {
	xml := `<?xml version="1.0" encoding="utf-8"?>
<feed xmlns="http://www.w3.org/2005/Atom">
  <title>Test Feed</title>
  <entry>
    <title>Semiconductor export controls tighten</title>
    <link rel="alternate" href="https://example.com/chips"/>
    <id>https://example.com/chips</id>
    <published>2026-08-01T08:15:00Z</published>
    <summary>New restrictions on chip exports.</summary>
  </entry>
  <entry>
    <title>No link entry should be skipped</title>
    <id>https://example.com/nolink</id>
    <updated>2026-07-30T12:00:00Z</updated>
  </entry>
</feed>`
	items, err := Parse([]byte(xml))
	if err != nil {
		t.Fatalf("Parse Atom: %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("期望 2 条，实际 %d", len(items))
	}
	if items[0].URL != "https://example.com/chips" {
		t.Errorf("Atom link 错误: %q", items[0].URL)
	}
	want := time.Date(2026, 8, 1, 8, 15, 0, 0, time.UTC)
	if !items[0].Published.Equal(want) {
		t.Errorf("published 解析错误: %v", items[0].Published)
	}
	if items[1].URL != "https://example.com/nolink" {
		t.Errorf("无 link 时应回退到 id: %q", items[1].URL)
	}
}

func TestParseRDF(t *testing.T) {
	xml := `<?xml version="1.0"?>
<rdf:RDF xmlns:rdf="http://www.w3.org/1999/02/22-rdf-syntax-ns#"
         xmlns="http://purl.org/rss/1.0/"
         xmlns:dc="http://purl.org/dc/elements/1.1/">
  <item rdf:about="https://example.com/germany">
    <title>Berlin announces energy package</title>
    <link>https://example.com/germany</link>
    <dc:date>2026-08-01T06:00:00Z</dc:date>
    <description>Details of the package.</description>
  </item>
</rdf:RDF>`
	items, err := Parse([]byte(xml))
	if err != nil {
		t.Fatalf("Parse RDF: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("期望 1 条，实际 %d", len(items))
	}
	if items[0].Title != "Berlin announces energy package" {
		t.Errorf("RDF 标题错误: %q", items[0].Title)
	}
}

func TestParseDateVariants(t *testing.T) {
	cases := []struct {
		in   string
		want time.Time
	}{
		{"Mon, 02 Jan 2006 15:04:05 GMT", time.Date(2006, 1, 2, 15, 4, 5, 0, time.UTC)},
		{"Mon, 02 Jan 2006 15:04:05 -0700", time.Date(2006, 1, 2, 15, 4, 5, 0, time.FixedZone("", -7*3600))},
		{"2 Jan 2006 15:04:05 +0000", time.Date(2006, 1, 2, 15, 4, 5, 0, time.UTC)},
		{"2006-01-02T15:04:05Z", time.Date(2006, 1, 2, 15, 4, 5, 0, time.UTC)},
		{"2006-01-02 15:04:05", time.Date(2006, 1, 2, 15, 4, 5, 0, time.UTC)},
		{"2006-01-02", time.Date(2006, 1, 2, 0, 0, 0, 0, time.UTC)},
		{"Tue, 01 Aug 2026 09:00:00 CEST", time.Date(2026, 8, 1, 7, 0, 0, 0, time.UTC)},  // CEST=UTC+2 → 07:00 UTC
		{"Fri, 31 Jul 2026 18:00:00 JST", time.Date(2026, 7, 31, 9, 0, 0, 0, time.UTC)},  // JST=UTC+9
		{"Mon, 01 Aug 2026 12:00:00 WEST", time.Date(2026, 8, 1, 12, 0, 0, 0, time.UTC)}, // 未收录缩写 → 按 UTC
		{"2026/8/3 09:30", time.Date(2026, 8, 3, 9, 30, 0, 0, time.UTC)},                 // 中文/东亚格式
		{"2026年8月3日 09:30", time.Date(2026, 8, 3, 9, 30, 0, 0, time.UTC)},
		{"2026-8-3 09:30:00", time.Date(2026, 8, 3, 9, 30, 0, 0, time.UTC)},
		{"", time.Time{}},
		{"not a date", time.Time{}},
	}
	for _, c := range cases {
		got := parseTime(c.in)
		if c.want.IsZero() {
			if !got.IsZero() {
				t.Errorf("parseTime(%q) 期望零值，实际 %v", c.in, got)
			}
			continue
		}
		if !got.Equal(c.want) {
			t.Errorf("parseTime(%q) = %v，期望 %v", c.in, got, c.want)
		}
	}
}

func TestParseBOM(t *testing.T) {
	// 部分台湾 feed 以 UTF-8 BOM 开头
	xml := "\xef\xbb\xbf<?xml version=\"1.0\"?><rss version=\"2.0\"><channel>" +
		"<item><title>測試新聞</title><link>https://example.com/1</link></item>" +
		"</channel></rss>"
	items, err := Parse([]byte(xml))
	if err != nil {
		t.Fatalf("BOM feed 解析失败: %v", err)
	}
	if len(items) != 1 || items[0].Title != "測試新聞" {
		t.Errorf("BOM 剥离失败: %+v", items)
	}
}

func TestParseSourceElement(t *testing.T) {
	xml := `<rss version="2.0"><channel><item>
<title>EU sanctions - Reuters</title>
<link>https://news.google.com/rss/articles/x</link>
<source url="https://www.reuters.com">Reuters</source>
</item></channel></rss>`
	items, err := Parse([]byte(xml))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if items[0].SourceName != "Reuters" || items[0].SourceURL != "https://www.reuters.com" {
		t.Errorf("source 元素解析失败: %q / %q", items[0].SourceName, items[0].SourceURL)
	}
}

func TestParseGarbage(t *testing.T) {
	if _, err := Parse([]byte("<html><body>not a feed</body></html>")); err == nil {
		t.Error("垃圾输入应报错")
	}
	if _, err := Parse([]byte("")); err == nil {
		t.Error("空输入应报错")
	}
	if _, err := Parse([]byte("\xef\xbb\xbf")); err == nil {
		t.Error("仅 BOM 应报错")
	}
}

func TestParseXMLNamespaceCDATA(t *testing.T) {

	xml := `<rss version="2.0"><channel><item>
<title><![CDATA[France unveils <strong>industrial</strong> plan]]></title>
<link>https://example.com/fr</link>
<description><![CDATA[<p>First &amp; foremost</p>]]></description>
</item></channel></rss>`
	items, err := Parse([]byte(xml))
	if err != nil {
		t.Fatalf("Parse CDATA: %v", err)
	}
	if items[0].Title != "France unveils <strong>industrial</strong> plan" {
		t.Errorf("CDATA 标题应保留字面内容: %q", items[0].Title)
	}
	if strings.Contains(items[0].Summary, "<p>") {
		t.Errorf("CDATA 摘要应去标签: %q", items[0].Summary)
	}
}
