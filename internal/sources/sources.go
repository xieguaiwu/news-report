// Package sources 维护内置的权威欧美新闻源注册表。
//
// 广度/深度取舍策略：
//   - wire       —— 全球通讯社（路透/美联社），最快最广，权重 1.0
//   - legacy     —— 老牌大报（BBC/卫报/纽时/世界报/FAZ…），深度报道，权重 0.9
//   - specialist —— 机构与智库（IMF/美联储/布鲁金斯/EIA…），政策与经济深度，权重 1.0
//
// 语言覆盖：en / de / fr。
// 每个源可配置多个候选 feed（按顺序尝试），失败的源只告警不致命；
// Optional=true 表示该 feed 地址不确定性较高（可能已停用），失败时静默降级。
package sources

import (
	"sort"
	"strings"
)

// Tier 来源层级。
type Tier string

const (
	TierWire       Tier = "wire"
	TierLegacy     Tier = "legacy"
	TierSpecialist Tier = "specialist"
)

// Source 描述一个新闻来源。
type Source struct {
	ID        string   `json:"id"`
	Name      string   `json:"name"`
	Lang      string   `json:"lang"` // en / de / fr
	Tier      Tier     `json:"tier"`
	Weight    float64  `json:"weight"`
	Feeds     []string `json:"feeds,omitempty"` // 候选 feed URL，按顺序尝试
	Scrape    *Scrape  `json:"scrape,omitempty"`
	Optional  bool     `json:"optional,omitempty"` // 失效时静默（不占用告警名额）
	Enabled   bool     `json:"enabled"`
	UserAgent string   `json:"user_agent,omitempty"` // 站点显式白名单的 UA（如 Feedfetcher-Google）
}

// Scrape 描述无 feed 时的 HTML 兜底抓取规则。
type Scrape struct {
	URL         string `json:"url"`
	Selector    string `json:"selector,omitempty"` // goquery 选择器；空 = 启发式
	LinkPattern string `json:"link_pattern,omitempty"`
	MaxItems    int    `json:"max_items,omitempty"`
}

// weight 返回来源权重（按层级）。
func weightFor(t Tier) float64 {
	switch t {
	case TierWire:
		return 1.0
	case TierSpecialist:
		return 1.0
	default:
		return 0.9
	}
}

func src(id, name, lang string, tier Tier, optional bool, feeds ...string) Source {
	return Source{
		ID: id, Name: name, Lang: lang, Tier: tier,
		Weight: weightFor(tier), Feeds: feeds, Optional: optional, Enabled: true,
	}
}

// Defaults 返回内置来源注册表（按语言与层级排序）。
func Defaults() []Source {
	list := []Source{
		// ── EN · 全球通讯社 (wire) ──────────────────────────────
		src("reuters-world", "Reuters World", "en", TierWire, true,
			"https://www.reuters.com/arc/outboundfeeds/world/?outputType=xml"),
		src("reuters-business", "Reuters Business", "en", TierWire, true,
			"https://www.reuters.com/arc/outboundfeeds/business/?outputType=xml"),
		src("reuters-markets", "Reuters Markets", "en", TierWire, true,
			"https://www.reuters.com/arc/outboundfeeds/markets/?outputType=xml"),
		src("ap-news", "AP News", "en", TierWire, true,
			"https://apnews.com/feed"),
		// ── EN · 老牌媒体 (legacy) ─────────────────────────────
		src("bbc-world", "BBC World", "en", TierLegacy, false,
			"https://feeds.bbci.co.uk/news/world/rss.xml"),
		src("bbc-business", "BBC Business", "en", TierLegacy, false,
			"https://feeds.bbci.co.uk/news/business/rss.xml"),
		src("bbc-tech", "BBC Technology", "en", TierLegacy, false,
			"https://feeds.bbci.co.uk/news/technology/rss.xml"),
		src("guardian-world", "The Guardian World", "en", TierLegacy, false,
			"https://www.theguardian.com/world/rss"),
		src("guardian-business", "The Guardian Business", "en", TierLegacy, false,
			"https://www.theguardian.com/business/rss"),
		src("guardian-tech", "The Guardian Technology", "en", TierLegacy, false,
			"https://www.theguardian.com/technology/rss"),
		src("nyt-world", "NYT World", "en", TierLegacy, false,
			"https://rss.nytimes.com/services/xml/rss/nyt/World.xml"),
		src("nyt-business", "NYT Business", "en", TierLegacy, false,
			"https://rss.nytimes.com/services/xml/rss/nyt/Business.xml"),
		src("nyt-economy", "NYT Economy", "en", TierLegacy, false,
			"https://rss.nytimes.com/services/xml/rss/nyt/Economy.xml"),
		src("nyt-tech", "NYT Technology", "en", TierLegacy, false,
			"https://rss.nytimes.com/services/xml/rss/nyt/Technology.xml"),
		src("wsj-world", "WSJ World", "en", TierLegacy, false,
			"https://feeds.a.dj.com/rss/RSSWorldNews.xml"),
		src("wsj-markets", "WSJ Markets", "en", TierLegacy, false,
			"https://feeds.a.dj.com/rss/RSSMarketsMain.xml"),
		src("wsj-tech", "WSJ Tech & Business", "en", TierLegacy, true,
			"https://feeds.a.dj.com/rss/WSJcomTech-Business.xml"), // 可能 403（与其他 WSJ feed 不一致）
		src("wapo-world", "Washington Post World", "en", TierLegacy, false,
			"https://feeds.washingtonpost.com/rss/world"),
		src("wapo-business", "Washington Post Business", "en", TierLegacy, false,
			"https://feeds.washingtonpost.com/rss/business"),
		src("economist-finance", "The Economist Finance", "en", TierLegacy, true,
			"https://www.economist.com/finance-and-economics/rss.xml"),
		// ── EN · 政策/产业深度 (specialist) ────────────────────
		src("politico-eu", "Politico Europe", "en", TierSpecialist, true,
			"https://www.politico.eu/feed/"), // Cloudflare 反爬，可能 403（代理 IP 信誉），属预期失败
		src("euractiv", "EURACTIV", "en", TierSpecialist, true,
			"https://www.euractiv.com/feed/"), // Cloudflare 反爬，可能 403
		src("thehill", "The Hill", "en", TierSpecialist, false,
			"https://thehill.com/feed/"),
		{
			// 2026 起 RSS 已下线（302→首页），依赖 scrape 兑底
			ID: "brookings", Name: "Brookings Institution", Lang: "en", Tier: TierSpecialist,
			Weight: weightFor(TierSpecialist), Optional: true, Enabled: true,
			Feeds: []string{"https://www.brookings.edu/feed/"},
			Scrape: &Scrape{URL: "https://www.brookings.edu/", MaxItems: 20,
				LinkPattern: `^https://www\.brookings\.edu/(articles|research|commentary|podcast-episode)/`},
		},
		src("cfr", "Council on Foreign Relations", "en", TierSpecialist, true,
			"https://www.cfr.org/rss.xml",
			"https://www.cfr.org/rss/"),
		src("piie", "Peterson Institute (PIIE)", "en", TierSpecialist, true,
			"https://www.piie.com/rss.xml"),
		src("csis", "CSIS", "en", TierSpecialist, true,
			"https://www.csis.org/rss.xml"),
		src("chathamhouse", "Chatham House", "en", TierSpecialist, true,
			"https://www.chathamhouse.org/rss.xml"),
		{
			// 站点 robots 对 /feed 显式白名单 Feedfetcher-Google（Disallow: */news/ 对普通 UA 生效）
			ID: "un-news", Name: "UN News", Lang: "en", Tier: TierSpecialist,
			Weight: weightFor(TierSpecialist), Optional: false, Enabled: true,
			UserAgent: "Feedfetcher-Google/1.0",
			Feeds:     []string{"https://news.un.org/feed/subscribe/en/news/all/rss.xml"},
		},
		src("eia", "US EIA Press", "en", TierSpecialist, true,
			"https://www.eia.gov/rss/press_rss.xml"), // 注意：EIA robots.txt 自身禁止 /rss，默认会被 robots 拦截，--no-robots 可绕过
		src("imf", "IMF", "en", TierSpecialist, true,
			"https://www.imf.org/external/np/News/feed.aspx",
			"https://www.imf.org/en/News/RSS"),
		src("fed", "US Federal Reserve", "en", TierSpecialist, false,
			"https://www.federalreserve.gov/feeds/press_all.xml"),
		src("ecb", "ECB Press", "en", TierSpecialist, true,
			"https://www.ecb.europa.eu/rss/press.xml",
			"https://www.ecb.europa.eu/rss/press_releases.xml"),

		// ── DE · 老牌媒体 (legacy) ─────────────────────────────
		src("dw-en", "Deutsche Welle (EN)", "en", TierLegacy, false,
			"https://rss.dw.com/rdf/rss-en-all",
			"https://rss.dw.com/rdf/rss-en-world"),
		src("dw-de", "Deutsche Welle (DE)", "de", TierLegacy, false,
			"https://rss.dw.com/rdf/rss-de-all"),
		src("tagesschau", "Tagesschau", "de", TierLegacy, false,
			"https://www.tagesschau.de/xml/rss2/"),
		src("spiegel-tops", "Der Spiegel Top", "de", TierLegacy, false,
			"https://www.spiegel.de/schlagzeilen/tops/index.rss"),
		src("spiegel-wirtschaft", "Der Spiegel Wirtschaft", "de", TierLegacy, true,
			"https://www.spiegel.de/wirtschaft/index.rss"),
		src("zeit-all", "Die Zeit", "de", TierLegacy, false,
			"https://newsfeed.zeit.de/all"),
		src("zeit-politik", "Die Zeit Politik", "de", TierLegacy, false,
			"https://newsfeed.zeit.de/politik/index"),
		src("zeit-wirtschaft", "Die Zeit Wirtschaft", "de", TierLegacy, false,
			"https://newsfeed.zeit.de/wirtschaft/index"),
		src("faz-politik", "FAZ Politik", "de", TierLegacy, false,
			"https://www.faz.net/rss/aktuell/politik/"),
		src("faz-wirtschaft", "FAZ Wirtschaft", "de", TierLegacy, false,
			"https://www.faz.net/rss/aktuell/wirtschaft/"),
		src("handelsblatt", "Handelsblatt", "de", TierLegacy, false,
			"https://www.handelsblatt.com/contentexport/feed/schlagzeilen"),
		src("handelsblatt-wirtschaft", "Handelsblatt Wirtschaft", "de", TierLegacy, true,
			"https://www.handelsblatt.com/contentexport/feed/wirtschaft",
			"https://www.handelsblatt.com/contentexport/feed/unternehmen"),
		src("sz-top", "Süddeutsche Top", "de", TierLegacy, false,
			"https://rss.sueddeutsche.de/rss/Topthemen"),
		src("sz-wirtschaft", "Süddeutsche Wirtschaft", "de", TierLegacy, false,
			"https://rss.sueddeutsche.de/rss/Wirtschaft"),

		// ── FR · 老牌媒体 (legacy) ─────────────────────────────
		src("lemonde-international", "Le Monde International", "fr", TierLegacy, false,
			"https://www.lemonde.fr/international/rss_full.xml"),
		src("lemonde-economie", "Le Monde Économie", "fr", TierLegacy, false,
			"https://www.lemonde.fr/economie/rss_full.xml"),
		src("lemonde-entreprises", "Le Monde Entreprises", "fr", TierLegacy, false,
			"https://www.lemonde.fr/entreprises/rss_full.xml"),
		src("figaro-international", "Le Figaro International", "fr", TierLegacy, false,
			"https://www.lefigaro.fr/rss/figaro_international.xml"),
		src("figaro-economie", "Le Figaro Économie", "fr", TierLegacy, false,
			"https://www.lefigaro.fr/rss/figaro_economie.xml"),
		src("france24-fr", "France 24 (FR)", "fr", TierLegacy, false,
			"https://www.france24.com/fr/rss"),
		src("france24-en", "France 24 (EN)", "en", TierLegacy, false,
			"https://www.france24.com/en/rss"),
		src("rfi-fr", "RFI", "fr", TierLegacy, true,
			"https://www.rfi.fr/fr/rss",
			"https://www.rfi.fr/fr/rss/une"),
		src("lesechos", "Les Échos", "fr", TierLegacy, true,
			"https://www.lesechos.fr/rss.xml"),
		src("lepoint", "Le Point", "fr", TierLegacy, true,
			"https://www.lepoint.fr/rss.xml"),
		src("franceinfo", "franceinfo", "fr", TierLegacy, true,
			"https://www.francetvinfo.fr/rss/"),
	}
	sort.SliceStable(list, func(i, j int) bool {
		if list[i].Lang != list[j].Lang {
			return list[i].Lang < list[j].Lang
		}
		return list[i].ID < list[j].ID
	})
	return list
}

// GoogleNewsFeeds 返回谷歌新闻聚合源（可选广度补充，默认关闭）。
// 每个分类 2 个查询 × 3 种语言，覆盖政策/经济/产业热词。
func GoogleNewsFeeds() []Source {
	queries := map[string][]string{
		"politics": {
			"international policy diplomacy OR sanctions OR summit",
			"EU politics OR government OR parliament",
		},
		"economy": {
			"global economy OR central bank OR inflation",
			"international trade OR tariffs OR fiscal policy",
		},
		"industry": {
			"semiconductors OR supply chain OR manufacturing",
			"energy transition OR electric vehicles OR industrial policy",
		},
	}
	langParam := map[string]struct{ hl, gl string }{
		"en": {"en-US", "US"},
		"de": {"de-DE", "DE"},
		"fr": {"fr-FR", "FR"},
	}
	var list []Source
	for cat, qs := range queries {
		for lang, p := range langParam {
			for i, q := range qs {
				url := "https://news.google.com/rss/search?q=" +
					strings.ReplaceAll(q, " ", "+") +
					"&hl=" + p.hl + "&gl=" + p.gl + "&ceid=" + p.gl + ":" + p.hl
				list = append(list, Source{
					ID:       "gnews-" + cat + "-" + lang + "-" + itoa(i),
					Name:     "Google News " + cat + " (" + lang + ")",
					Lang:     lang,
					Tier:     TierSpecialist,
					Weight:   0.75,
					Feeds:    []string{url},
					Optional: true,
					Enabled:  true,
				})
			}
		}
	}
	return list
}

func itoa(i int) string {
	return string(rune('1' + i))
}
