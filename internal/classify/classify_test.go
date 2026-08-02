package classify

import "testing"

func TestClassifyUSPolitics(t *testing.T) {
	cases := []struct {
		title, summary string
		want           Category
	}{
		{"Senate passes sweeping tax bill", "The House will vote next week on the legislation", USPolitics},
		{"Trump fires FBI director amid probe", "White House confirms the decision", USPolitics},
		{"Supreme Court hears case on federal agency power", "Justices appeared skeptical", USPolitics},
		{"California governor signs new budget", "The state faces a deficit", USPolitics},
		// 国际政治不应被抢走
		{"EU leaders agree on new sanctions package", "The European Council approved fresh sanctions", Politics},
		{"Germany and France seek joint defense policy", "Foreign ministers met in Paris", Politics},
	}
	for _, c := range cases {
		got := Classify("en", c.title, c.summary)
		if got.Category != c.want {
			t.Errorf("Classify(%q) = %s，期望 %s (score=%d)", c.title, got.Category, c.want, got.Score)
		}
	}
}

func TestClassifyUSPoliticsDE(t *testing.T) {
	got := Classify("de", "Trump kündigt neue Zölle an", "Der US-Präsident sprach im Weißen Haus")
	if got.Category != USPolitics {
		t.Errorf("德语特朗普新闻应分类 uspolitics，实际 %s", got.Category)
	}
}

func TestClassifyUSPoliticsFR(t *testing.T) {
	got := Classify("fr", "Le Sénat américain adopte un texte budgétaire", "Trump menace de veto")
	if got.Category != USPolitics {
		t.Errorf("法语美国参议院新闻应分类 uspolitics，实际 %s", got.Category)
	}
}

func TestClassifyZH(t *testing.T) {
	cases := []struct {
		title, summary string
		want           Category
	}{
		// 繁体（台湾媒体）
		{"川普暫緩對伊朗發動新攻擊 與沙烏地王儲通話", "白宮表示正在等待回應", USPolitics},
		{"美國參議院通過重大預算法案", "共和黨與民主黨達成協議", USPolitics},
		{"菲律賓海成台海衝突關鍵戰場", "專家警告軍事對峙風險升高", Politics},
		{"歐盟對俄羅斯實施新一輪制裁", "27國外交部長達成共識", Politics},
		{"台灣對美出口大爆發 專家示警關稅風險", "貿易順差創新高", Economy},
		{"央行宣布升息 抑制通膨", "利率決策委員會一致通過", Economy},
		{"台積電先進製程產能滿載", "半導體供應鏈持續擴張", Industry},
		{"OPEC+決議增產 國際油價走跌", "能源市場供應增加", Industry},
		{"中華隊奪得世界盃冠軍", "決賽精彩萬分", Other},
	}
	for _, c := range cases {
		got := Classify("zh", c.title, c.summary)
		if got.Category != c.want {
			t.Errorf("ClassifyZH(%q) = %s，期望 %s", c.title, got.Category, c.want)
		}
	}
}

func TestClassifyZHSimplified(t *testing.T) {
	// 简体变体也应能匹配（词表繁简双向）
	got := Classify("zh", "美国国会通过新制裁法案", "参议院表决通过")
	if got.Category != USPolitics {
		t.Errorf("简体美国政治标题应分类 uspolitics，实际 %s", got.Category)
	}
	got = Classify("zh", "新能源汽车产量创新高", "电动车出口增长")
	if got.Category != Industry {
		t.Errorf("简体产业标题应分类 industry，实际 %s", got.Category)
	}
}

func TestClassifyZHNegative(t *testing.T) {
	got := Classify("zh", "球星結婚 演藝圈好友齊聚", "婚禮眾星雲集 娛樂新聞")
	if got.Category == Economy || got.Category == Politics {
		t.Errorf("娱乐新闻不应分类为经济/政治: %s", got.Category)
	}
}
