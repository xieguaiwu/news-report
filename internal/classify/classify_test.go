package classify

import "testing"

func TestClassifyEN(t *testing.T) {
	cases := []struct {
		title, summary string
		want           Category
	}{
		{"EU leaders agree on new sanctions package", "The European Council approved fresh sanctions against Russia", Politics},
		{"Fed holds interest rates steady as inflation cools", "The central bank kept rates at 4.25%", Economy},
		{"TSMC announces new semiconductor fab in Arizona", "Chipmaker plans massive fab capacity expansion", Industry},
		{"Germany wins football friendly 3-1", "The national team played well in the warm-up match", Other},
	}
	for _, c := range cases {
		got := Classify("en", c.title, c.summary)
		if got.Category != c.want {
			t.Errorf("Classify(%q, %q) = %s，期望 %s", c.title, c.summary, got.Category, c.want)
		}
	}
}

func TestClassifyDE(t *testing.T) {
	cases := []struct {
		title, summary string
		want           Category
	}{
		{"Bundestag berät über neue Sanktionen", "Die Regierungskoalition einigte sich auf einen Gesetzentwurf", Politics},
		{"EZB hält Leitzins stabil", "Die Inflation in der Eurozone sinkt langsam", Economy},
		{"Halbleiterhersteller baut neue Fabrik in Dresden", "Die Lieferkette für Chips wird gestärkt", Industry},
		{"Bayern gewinnt Fußballspiel", "Der Rekordmeister siegte deutlich", Other},
	}
	for _, c := range cases {
		got := Classify("de", c.title, c.summary)
		if got.Category != c.want {
			t.Errorf("Classify(%q, %q) = %s，期望 %s", c.title, c.summary, got.Category, c.want)
		}
	}
}

func TestClassifyFR(t *testing.T) {
	cases := []struct {
		title, summary string
		want           Category
	}{
		{"Le gouvernement présente une réforme fiscale", "L'Assemblée nationale débattra la loi la semaine prochaine", Politics},
		{"La BCE maintient ses taux directeurs", "L'inflation ralentit dans la zone euro", Economy},
		{"Usine de semi-conducteurs en France", "La production de puces doublera d'ici 2027", Industry},
		{"Le PSG remporte le match", "La star a marqué deux buts", Other},
	}
	for _, c := range cases {
		got := Classify("fr", c.title, c.summary)
		if got.Category != c.want {
			t.Errorf("Classify(%q, %q) = %s，期望 %s", c.title, c.summary, got.Category, c.want)
		}
	}
}

func TestClassifyAccentNormalization(t *testing.T) {
	// élection（带重音）应被 normalize 后匹配 election
	got := Classify("fr", "Élection présidentielle: le scrutin approche", "")
	if got.Category != Politics {
		t.Errorf("带重音法语应分类为 politics，实际 %s", got.Category)
	}
}

func TestClassifyNegativeDominates(t *testing.T) {
	// 标题含 economy 但整体是娱乐 → 应被压回 other
	got := Classify("en", "The Economy of Celebrity: how stars make millions", "A look inside Hollywood's money machine")
	if got.Category == Economy {
		t.Errorf("娱乐内容不应分类为 economy")
	}
}

func TestClassifyConfidence(t *testing.T) {
	got := Classify("en", "Fed raises interest rates", "The central bank increased rates again")
	if got.Category != Economy || got.Score < 3 {
		t.Errorf("置信度/得分异常: %+v", got)
	}
	if got.Confidence <= 0 || got.Confidence > 1 {
		t.Errorf("置信度应在 (0,1]: %v", got.Confidence)
	}
}

func TestClassifyUnknownLangFallsBackToEN(t *testing.T) {
	got := Classify("xx", "Central bank cuts rates", "")
	if got.Category != Economy {
		t.Errorf("未知语言应回退英文词表，实际 %s", got.Category)
	}
}
