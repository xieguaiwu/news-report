// Package classify 实现基于关键词的新闻分类：politics（国际政策）/ economy（经济金融）/ industry（产业发展）。
// 支持 en / de / fr 三种语言，词表按语言独立；带负面词表过滤娱乐/体育等噪音。
package classify

import (
	"strings"
	"unicode"
)

// init 预规范化全部词表（去重音），与 tokenize 的 normalize 保持一致。
func init() {
	for lang, ks := range sets {
		ks.politics = normList(ks.politics)
		ks.uspolitics = normList(ks.uspolitics)
		ks.economy = normList(ks.economy)
		ks.industry = normList(ks.industry)
		ks.negative = normList(ks.negative)
		ks.multiPolitics = normList(ks.multiPolitics)
		ks.multiUSPolitics = normList(ks.multiUSPolitics)
		ks.multiEconomy = normList(ks.multiEconomy)
		ks.multiIndustry = normList(ks.multiIndustry)
		ks.multiNegative = normList(ks.multiNegative)
		sets[lang] = ks
	}
}

func normList(list []string) []string {
	out := make([]string, len(list))
	for i, s := range list {
		out[i] = normalize(s)
	}
	return out
}

// Category 新闻分类。
type Category string

const (
	USPolitics Category = "uspolitics" // 美国本国政治
	Politics   Category = "politics"   // 国际政策
	Economy    Category = "economy"    // 经济金融
	Industry   Category = "industry"   // 产业发展
	Other      Category = "other"
)

// All 是全部专注分类（用于 CLI 校验与默认过滤）。
var All = []Category{USPolitics, Politics, Economy, Industry}

// keywordSet 一组分类词表。
type keywordSet struct {
	politics   []string // 单 token
	uspolitics []string
	economy    []string
	industry   []string
	negative   []string
	// 多词短语（直接对原文做子串匹配）
	multiPolitics   []string
	multiUSPolitics []string
	multiEconomy    []string
	multiIndustry   []string
	multiNegative   []string
}

var sets = map[string]keywordSet{
	"en": {
		uspolitics: []string{
			"congress", "congressional", "senate", "senator", "senators",
			"governor", "governors", "midterms", "midterm", "primary", "primaries",
			"republican", "republicans", "democrat", "democrats", "gop", "capitol",
			"impeachment", "impeach", "federal", "whitehouse", "biden", "trump",
			"harris", "vance", "houseofrepresentatives",
		},
		politics: []string{
			"policy", "policies", "government", "parliament", "election", "elections",
			"summit", "sanctions", "sanction", "diplomacy", "diplomatic", "minister",
			"ministry", "nato", "eu", "unitednations", "treaty", "tariff", "tariffs",
			"geopolitical", "geopolitics", "president", "prime", "chancellor",
			"lawmaker", "lawmakers", "legislation", "bill",
			"foreign", "ambassador", "referendum", "coalition", "cabinet",
			"europeancommission", "bilateral", "multilateral", "embargo",
			"defense", "defence", "military", "ceasefire", "peace", "negotiation",
			"negotiations", "g7", "g20", "apec", "who", "imf", "worldbank",
			"unsecuritycouncil", "refugee", "asylum", "migration", "border",
			"war", "conflict", "airstrike", "airstrikes", "invasion", "missile",
			"missiles", "shelling", "disarmament", "protests", "protest", "strikes",
		},
		economy: []string{
			"economy", "economic", "gdp", "inflation", "interestrate", "centralbank",
			"fed", "federalreserve", "ecb", "bankofengland", "monetary", "fiscal",
			"deficit", "debt", "recession", "growth", "unemployment", "labor",
			"labour", "market", "markets", "stock", "stocks", "bond", "bonds",
			"currency", "exchange", "trade", "exports", "imports", "bank",
			"banks", "banking", "treasury", "yield", "yields", "investment",
			"investor", "investors", "financial", "finance", "budget", "tax",
			"taxes", "taxation", "stimulus", "subsidy", "subsidies", "austerity",
			"housing", "mortgage", "forecast", "outlook", "recovery", "slowdown",
		},
		industry: []string{
			"industry", "industrial", "manufacturing", "manufacturer", "factory",
			"semiconductor", "semiconductors", "chip", "chips", "supplychain",
			"energy", "oil", "gas", "electricity", "renewable", "solar", "wind",
			"nuclear", "electricvehicle", "evs", "automotive", "automaker",
			"automakers", "steel", "aluminum", "aluminium", "battery", "batteries",
			"lithium", "rareearth", "pharma", "pharmaceutical", "biotech",
			"aerospace", "defenseindustry", "shipbuilding", "chemical",
			"chemicals", "infrastructure", "supply", "suppliers", "exportcontrols",
			"artificialintelligence", "ai", "data", "technology", "tech",
			"telecom", "5g", "greenenergy", "decarbonization", "carbon",
			"hydrogen", "mining", "commodities", "commodity", "production",
			"output", "capacity", "supply", "shortage", "tariff",
		},
		negative: []string{
			"sport", "football", "soccer", "tennis", "basketball", "cricket",
			"olympics", "worldcup", "celebrity", "entertainment", "hollywood",
			"movie", "movies", "film", "music", "concert", "fashion", "wedding",
			"recipe", "weather", "forecast", "horoscope", "puzzle", "quiz",
			"gossip", "royal", "baby", "pets", "cooking", "travel",
		},
		multiPolitics: []string{
			"prime minister", "foreign policy", "foreign minister", "defense minister",
			"defence minister", "secretary of state", "security council",
			"trade war", "trade deal", "peace talks", "cold war", "soft power",
			"government shutdown", "executive order", "election campaign",
			"voting", "ballot", "diplomatic relations", "political crisis",
		},
		multiUSPolitics: []string{
			"white house", "supreme court", "attorney general", "justice department",
			"midterm elections", "primary elections", "state legislature",
			"house of representatives", "governor race", "senate race",
			"congressional hearing", "election denier",
		},
		multiEconomy: []string{
			"central bank", "interest rate", "interest rates", "gross domestic product",
			"consumer prices", "inflation rate", "economic growth", "fiscal policy",
			"monetary policy", "quantitative easing", "balance of trade",
			"trade deficit", "trade surplus", "stock market", "stock markets",
			"wall street", "bond market", "sovereign debt", "government debt",
			"public debt", "budget deficit", "gdp growth", "jobless claims",
			"labor market", "labour market", "housing market", "real estate",
		},
		multiIndustry: []string{
			"supply chain", "supply chains", "electric vehicle", "electric vehicles",
			"rare earth", "rare earths", "export controls", "industrial policy",
			"artificial intelligence", "energy transition", "clean energy",
			"renewable energy", "fossil fuels", "natural gas", "crude oil",
			"oil prices", "carbon emissions", "green technology", "chips act",
			"semiconductor industry", "smart manufacturing", "industrial production",
			"factory output", "capacity utilization",
		},
		multiNegative: []string{
			"sports news", "game results", "movie review", "music review",
			"celebrity news", "tv ratings", "football match", "weather forecast",
		},
	},
	"de": {
		uspolitics: []string{
			"senat", "kongress", "trump", "biden", "gouverneur",
			"republikaner", "demokraten", "wahlkampf", "präsidentschaftswahl",
		},
		politics: []string{
			"politik", "regierung", "parlament", "wahl", "wahlen", "gipfel",
			"sanktionen", "sanktion", "diplomatie", "diplomatisch", "minister",
			"ministerium", "nato", "eu", "un", "vertrag", "verträge", "zoll",
			"zölle", "geopolitik", "geopolitisch", "präsident", "kanzler",
			"kanzlerin", "bundestag", "abgeordnete", "gesetz", "gesetze",
			"koalition", "kabinett", "außenminister", "außenpolitik",
			"botschafter", "volksentscheid", "referendum", "sicherheitsrat",
			"verteidigung", "militär", "waffenruhe", "frieden", "verhandlung",
			"verhandlungen", "g7", "g20", "welthandelsorganisation",
			"flüchtlinge", "asyl", "migration", "grenze", "europäischekommission",
			"krieg", "konflikt", "angriff", "angriffe", "offensive", "raketen",
			"luftangriff", "waffen", "abrüstung", "protest", "proteste",
		},
		economy: []string{
			"wirtschaft", "wirtschaftlich", "bip", "inflation", "zinsen",
			"zins", "zentralbank", "notenbank", "ezb", "geldpolitik", "fiskal",
			"defizit", "schulden", "rezession", "wachstum", "arbeitslosigkeit",
			"arbeitsmarkt", "markt", "märkte", "aktien", "anleihen", "devisen",
			"handel", "export", "exporte", "import", "importe", "bank", "banken",
			"treasury", "rendite", "renditen", "investition", "investor",
			"investoren", "finanz", "finanzen", "haushalt", "steuer", "steuern",
			"konjunktur", "prognose", "erholung", "abschwächung", "staatsschulden",
		},
		industry: []string{
			"industrie", "industriell", "produktion", "herstellung", "fabrik",
			"halbleiter", "chip", "chips", "lieferkette", "lieferketten", "energie",
			"öl", "gas", "strom", "erneuerbare", "solar", "wind", "atomkraft",
			"elektroauto", "elektroautos", "automobil", "autohersteller", "stahl",
			"aluminium", "batterie", "batterien", "lithium", "selteneerden",
			"pharma", "biotech", "luftfahrt", "raumschiff", "schiffbau", "chemie",
			"infrastruktur", "zulieferer", "exportkontrollen", "künstlicheintelligenz",
			"technologie", "technik", "telekommunikation", "5g", "grüneenergie",
			"dekarbonisierung", "kohlenstoff", "wasserstoff", "bergbau", "rohstoffe",
			"produktionskapazität", "kapazität", "engpass", "verknappung", "krise",
		},
		negative: []string{
			"sport", "fußball", "tennis", "basketball", "olympia", "weltmeisterschaft",
			"prominente", "unterhaltung", "film", "kino", "musik", "konzert",
			"mode", "hochzeit", "rezept", "wetter", "wettervorhersage", "horoskop",
			"rätsel", "quiz", "klatsch", "royals", "baby", "haustiere",
		},
		multiPolitics: []string{
			"außenministerium", "innere sicherheit", "sicherheitspolitik",
			"handelskrieg", "handelsabkommen", "friedensgespräche", "europäische union",
			"vereinte nationen", "sicherheitsrat", "geplante gesetzesänderung",
			"regierungskrise", "koalitionsverhandlungen",
		},
		multiUSPolitics: []string{
			"weißes haus", "weisses haus", "us-kongress", "us-senat",
			"oberster gerichtshof", "justizministerium", "mittelfristige wahlen",
			"us-präsident", "us-präsidenten", "us-regierung", "us-wahl", "us-wahlen",
		},
		multiEconomy: []string{
			"europäische zentralbank", "leitzins", "leitzinsen", "bruttoinlandsprodukt",
			"verbraucherpreise", "inflationstate", "wirtschaftswachstum",
			"fiskalpolitik", "geldpolitik", "quantitative lockerung", "handelsbilanz",
			"haushaltsdefizit", "aktienmarkt", "börse", "anleihemarkt",
			"staatsverschuldung", "arbeitslosenquote", "konjunkturprognose",
			"währungspolitik", "währungskurs", "währungsreserven", "währungsunion",
		},
		multiIndustry: []string{
			"lieferkette", "elektrofahrzeug", "elektrofahrzeuge", "seltene erden",
			"exportkontrollen", "industriepolitik", "künstliche intelligenz",
			"energiewende", "erneuerbare energien", "fossile brennstoffe",
			"erdgas", "rohöl", "ölpreise", "co2-emissionen", "grüne technologien",
			"halbleiterindustrie", "industrieproduktion", "produktionsausfälle",
		},
		multiNegative: []string{
			"sportnachrichten", "spielergebnisse", "filmkritik", "musikkritik",
			"prominenten-news", "tv-quoten", "wettervorhersage",
		},
	},
	"fr": {
		uspolitics: []string{
			"congrès", "sénat", "trump", "biden", "gouverneur",
			"républicains", "démocrates", "midterms",
		},
		politics: []string{
			"politique", "politiques", "gouvernement", "parlement", "élection",
			"élections", "election", "sommets", "sommet", "sanctions", "sanction",
			"diplomatie", "diplomatique", "ministre", "ministère", "otan", "ue",
			"onu", "traité", "tarifs", "douaniers", "géopolitique", "président",
			"premier", "chancelier", "assemblée", "sénat", "député", "députés",
			"loi", "lois", "étranger", "ambassadeur", "référendum", "coalition",
			"cabinet", "gouvernemental", "g7", "g20", "conseildesécurité",
			"défense", "militaire", "cessez-le-feu", "paix", "négociation",
			"négociations", "migration", "frontière", "asile", "réfugiés",
			"guerre", "conflit", "bombardement", "bombardements", "frappe",
			"frappes", "offensive", "trêve", "armes", "missiles",
			"manifestation", "protestation",
		},
		economy: []string{
			"économie", "economie", "économique", "pib", "inflation", "taux",
			"banquecentrale", "bce", "politiquemonetaire", "budgetaire",
			"déficit", "dette", "récession", "croissance", "chômage", "emploi",
			"marché", "marchés", "bourse", "actions", "obligations", "devise",
			"commerce", "exportations", "importations", "banque", "banques",
			"trésor", "rendement", "investissement", "investisseurs", "finance",
			"finances", "budget", "impôt", "impôts", "fiscalité", "relance",
			"prévision", "reprise", "ralentissement",
		},
		industry: []string{
			"industrie", "industriel", "fabrication", "manufacture", "usine",
			"semiconducteurs", "semi-conducteurs", "puce", "puces", "chaîned'approvisionnement",
			"énergie", "energie", "pétrole", "gaz", "électricité", "renouvelable",
			"solaire", "éolien", "nucléaire", "voitureélectrique", "véhiculesélectriques",
			"automobile", "constructeurs", "acier", "aluminium", "batterie",
			"batteries", "lithium", "terresrares", "pharma", "pharmaceutique",
			"biotech", "aérospatial", "chantiernaval", "chimie", "infrastructure",
			"fournisseurs", "contrôlesàl'exportation", "intelligenceartificielle",
			"technologie", "tech", "télécom", "5g", "énergieverte", "décarbonation",
			"carbone", "hydrogène", "mines", "matièrespremières", "production",
			"capacité", "pénurie", "approvisionnement",
		},
		negative: []string{
			"sport", "football", "tennis", "basketball", "jeuxolympiques", "coupedumonde",
			"célébrité", "divertissement", "cinéma", "film", "musique", "concert",
			"mode", "mariage", "recette", "météo", "horoscope", "mots-croisés",
			"quiz", "potins", "royauté", "bébé", "animaux",
		},
		multiPolitics: []string{
			"premier ministre", "ministre des affaires étrangères", "politique étrangère",
			"conseil de sécurité", "guerre commerciale", "accord commercial",
			"négociations de paix", "union européenne", "nations unies",
			"commission européenne", "crise politique",
		},
		multiUSPolitics: []string{
			"maison blanche", "congrès américain", "sénat américain",
			"cour suprême", "ministère de la justice", "élections de mi-mandat",
		},
		multiEconomy: []string{
			"banque centrale", "taux d'intérêt", "taux d'intérêts", "produit intérieur brut",
			"prix à la consommation", "croissance économique", "politique budgétaire",
			"politique monétaire", "assouplissement quantitatif", "balance commerciale",
			"déficit commercial", "marché boursier", "dette souveraine", "dette publique",
			"déficit budgétaire", "taux de chômage", "marché du travail",
		},
		multiIndustry: []string{
			"chaîne d'approvisionnement", "chaînes d'approvisionnement",
			"voiture électrique", "voitures électriques", "véhicule électrique",
			"terres rares", "contrôles à l'exportation", "politique industrielle",
			"intelligence artificielle", "transition énergétique", "énergie propre",
			"énergies renouvelables", "combustibles fossiles", "gaz naturel",
			"pétrole brut", "prix du pétrole", "émissions de carbone",
			"industrie des semi-conducteurs", "production industrielle",
		},
		multiNegative: []string{
			"actualités sportives", "résultats de match", "critique de film",
			"critique musicale", "actualité people", "audiences tv", "prévisions météo",
		},
	},

	// zh（繁体为主，含简体变体）：中文无空格，按子串匹配
	"zh": {
		uspolitics: []string{
			"白宮", "白宫", "參議院", "参议院", "眾議院", "众议院", "最高法院",
			"聯邦", "联邦", "州長", "州长", "共和黨", "共和党", "民主黨", "民主党",
			"彈劾", "弹劾", "期中選舉", "中期选举", "初選", "初选", "川普", "特朗普", "拜登",
			"州議會", "州议会", "國會山", "国会山",
		},
		politics: []string{
			"外交", "國際", "国际", "政策", "政府", "總統", "总统", "國會", "国会",
			"議會", "议会", "選舉", "选举", "制裁", "峰會", "峰会", "條約", "条约",
			"關稅", "关税", "部長", "部长", "聯合國", "联合国", "北約", "北约", "歐盟", "欧盟",
			"大使", "談判", "谈判", "停火", "戰爭", "战争", "衝突", "冲突", "軍事", "军事",
			"移民", "難民", "难民", "邊境", "边境", "國防", "国防", "協議", "协议",
			"公投", "內閣", "内阁", "貿易戰", "贸易战", "地緣政治", "地缘政治", "武器",
			"飛彈", "导弹", "空襲", "空袭", "情報", "情报", "大使館", "使馆",
		},
		economy: []string{
			"經濟", "经济", "財經", "财经", "通膨", "通貨膨脹", "通货膨胀", "利率",
			"央行", "聯準會", "美联储", "貨幣", "货币", "財政", "财政", "赤字", "債務",
			"债务", "衰退", "成長", "增长", "失業", "失业", "就業", "就业", "市場", "市场",
			"股市", "債市", "汇率", "匯率", "貿易", "贸易", "出口", "進口", "进口", "銀行",
			"银行", "預算", "预算", "稅", "税", "投資", "投资", "金融", "復甦", "复苏",
			"放緩", "放缓", "升息", "降息", "國債", "国债", "房市", "物價", "物价",
			"薪資", "薪资", "薪資成長", "稅收", "税收",
		},
		industry: []string{
			"產業", "产业", "工業", "工业", "製造", "制造", "半導體", "半导体", "晶片",
			"芯片", "供應鏈", "供应链", "能源", "石油", "天然氣", "天然气", "電力", "电力",
			"再生能源", "可再生能源", "太陽能", "太阳能", "風電", "风电", "核電", "核电",
			"電動車", "电动车", "汽車", "汽车", "鋼鐵", "钢铁", "鋁", "铝", "電池", "电池",
			"鋰", "锂", "稀土", "製藥", "制药", "生技", "航太", "航天", "造船", "化學",
			"化学", "基礎設施", "基础设施", "人工智慧", "人工智能", "科技", "電信", "电信",
			"5G", "綠能", "绿能", "減碳", "减碳", "碳", "氫能", "氢能", "採礦", "采矿",
			"原物料", "原材料", "產能", "产能", "缺貨", "短缺", "晶圓", "晶圆",
			"台積電", "台积电", "輝達", "辉达", "面板", "鋼材", "钢材", "石化",
			"油價", "油价", "原油", "頁岩", "页岩", "煉油", "炼油",
		},
		negative: []string{
			"體育", "体育", "足球", "棒球", "籃球", "篮球", "網球", "网球", "高爾夫", "高尔夫",
			"奧運", "奥运", "世界盃", "世界杯", "娛樂", "娱乐", "電影", "电影", "影劇", "影剧",
			"音樂", "音乐", "演唱會", "演唱会", "時尚", "时尚", "婚禮", "婚礼", "食譜", "食谱",
			"天氣", "天气", "星座", "謎題", "谜题", "測驗", "测验", "八卦", "寵物", "宠物",
			"旅遊", "旅游", "明星", "藝人", "艺人", "綜藝", "综艺", "偶像劇",
		},
		multiPolitics: []string{
			"兩岸關係", "两岸关系", "雙邊關係", "外交政策", "聯合國安理會", "聯合國大會",
			"和平談判", "和平協議", "貿易談判", "政黨輪替", "內閣改組", "外交使節",
		},
		multiUSPolitics: []string{
			"美國國會", "美国国会", "美國參議院", "美國眾議院", "國務卿", "国务卿",
			"司法部", "聯邦調查局", "聯邦最高法院", "總統大選", "美國總統",
		},
		multiEconomy: []string{
			"中央銀行", "中央银行", "利率決策", "貨幣政策", "財政政策", "量化寬鬆",
			"貿易逆差", "貿易順差", "貿易赤字", "國民生產總值", "生產毛額", "消費者物價",
			"經濟成長", "經濟衰退", "失業率", "就業市場", "房地產市場", "公債殖利率",
		},
		multiIndustry: []string{
			"供應鏈重整", "產業政策", "工業生產", "先進製程", "晶圓代工", "電動車電池",
			"綠色能源", "再生能源", "碳排放", "能源轉型", "離岸風電", "半導體產業",
		},
		multiNegative: []string{
			"體育新聞", "體育賽事", "賽事報導", "電影評論", "音樂評論", "娛樂新聞",
			"綜藝節目", "偶像劇", "旅遊攻略", "天氣預報",
		},
	},
}

// Result 是分类结果。
type Result struct {
	Category   Category
	Score      int // 命中关键词次数（不含负面）
	Confidence float64
	NegHits    int
}

// tokenize 按 Unicode 字母切分 token（跨语言安全，变音符号视为字母的一部分）。
func tokenize(s string) map[string]int {
	tokens := map[string]int{}
	var sb strings.Builder
	flush := func() {
		if sb.Len() > 0 {
			tokens[sb.String()]++
			sb.Reset()
		}
	}
	for _, r := range strings.ToLower(s) {
		if unicode.IsLetter(r) {
			sb.WriteRune(r)
		} else {
			flush()
		}
	}
	flush()
	return tokens
}

// Classify 对标题+摘要进行分类，返回分类与置信度。
func Classify(lang, title, summary string) Result {
	text := strings.ToLower(title + " " + summary)
	// 规范化：去重音（é→e 等），提升法语匹配率
	text = normalize(text)

	ks, ok := sets[lang]
	if !ok {
		ks = sets["en"]
	}

	tokens := tokenize(text)
	textLower := strings.ToLower(text)

	var pol, us, eco, ind, neg int
	for tok, n := range tokens {
		if contains(ks.politics, tok) {
			pol += n
		}
		if contains(ks.uspolitics, tok) {
			us += n * 2 // US 机构/人物词是强信号（senate/congress/trump 几乎不出现于他国语境）
		}
		if contains(ks.economy, tok) {
			eco += n
		}
		if contains(ks.industry, tok) {
			ind += n
		}
		if contains(ks.negative, tok) {
			neg += n
		}
	}
	// 中文（zh）：无空格语言，token 化会失败，改用子串匹配（词表均为 2+ 字短语）
	if lang == "zh" {
		pol, us, eco, ind, neg = 0, 0, 0, 0, 0
		for _, kw := range ks.politics {
			if strings.Contains(textLower, kw) {
				pol++
			}
		}
		for _, kw := range ks.uspolitics {
			if strings.Contains(textLower, kw) {
				us += 2
			}
		}
		for _, kw := range ks.economy {
			if strings.Contains(textLower, kw) {
				eco++
			}
		}
		for _, kw := range ks.industry {
			if strings.Contains(textLower, kw) {
				ind++
			}
		}
		for _, kw := range ks.negative {
			if strings.Contains(textLower, kw) {
				neg++
			}
		}
	}
	// 多词短语
	for _, p := range ks.multiPolitics {
		if strings.Contains(text, p) {
			pol += 2
		}
	}
	for _, p := range ks.multiUSPolitics {
		if strings.Contains(text, p) {
			us += 4
		}
	}
	for _, p := range ks.multiEconomy {
		if strings.Contains(text, p) {
			eco += 2
		}
	}
	for _, p := range ks.multiIndustry {
		if strings.Contains(text, p) {
			ind += 2
		}
	}
	for _, p := range ks.multiNegative {
		if strings.Contains(text, p) {
			neg += 2
		}
	}

	best := Other
	bestScore := 0
	for _, c := range []struct {
		cat Category
		sc  int
	}{
		// uspolitics 优先：US 专属词命中更多时才归类（避免抢走一般国际政治）
		{USPolitics, us}, {Politics, pol}, {Economy, eco}, {Industry, ind},
	} {
		if c.sc > bestScore {
			best, bestScore = c.cat, c.sc
		}
	}
	total := pol + eco + ind + us
	conf := 0.0
	if total > 0 {
		conf = float64(bestScore) / float64(total)
	}
	// 负面命中强压：即使有分类词，娱乐/体育类内容也压回 other
	if neg > 0 && neg >= bestScore {
		best, bestScore = Other, 0
	}
	return Result{Category: best, Score: bestScore, Confidence: conf, NegHits: neg}
}

// normalize 去掉重音符号（用于跨写法匹配）。
var accentMap = map[rune]string{
	'é': "e", 'è': "e", 'ê': "e", 'ë': "e",
	'à': "a", 'â': "a", 'ä': "a", 'á': "a", 'ã': "a", 'å': "a",
	'î': "i", 'ï': "i", 'í': "i", 'ì': "i",
	'ô': "o", 'ö': "o", 'ó': "o", 'ò': "o", 'õ': "o",
	'û': "u", 'ü': "u", 'ú': "u", 'ù': "u",
	'ç': "c",
	'ß': "ss",
	'œ': "oe", 'æ': "ae",
}

func normalize(s string) string {
	var sb strings.Builder
	for _, r := range s {
		if repl, ok := accentMap[r]; ok {
			sb.WriteString(repl)
		} else {
			sb.WriteRune(r)
		}
	}
	return sb.String()
}

func contains(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}
