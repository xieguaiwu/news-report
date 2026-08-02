package rank

import (
	"testing"
	"time"
)

func TestScoreFreshness(t *testing.T) {
	halflife := 12 * time.Hour
	newScore := Score(1.0, 0, true, time.Hour, halflife)
	oldScore := Score(1.0, 0, true, 48*time.Hour, halflife)
	if newScore <= oldScore {
		t.Errorf("更新鲜的新闻得分应更高: new=%v old=%v", newScore, oldScore)
	}
}

func TestScoreWeight(t *testing.T) {
	halflife := 12 * time.Hour
	high := Score(1.0, 0, false, time.Hour, halflife)
	low := Score(0.5, 0, false, time.Hour, halflife)
	if high <= low {
		t.Errorf("更高权重的来源得分应更高: high=%v low=%v", high, low)
	}
}

func TestScoreCategorizedBonus(t *testing.T) {
	halflife := 12 * time.Hour
	cat := Score(0.9, 0, true, time.Hour, halflife)
	uncat := Score(0.9, 0, false, time.Hour, halflife)
	if cat <= uncat {
		t.Errorf("专注分类应有加成: cat=%v uncat=%v", cat, uncat)
	}
}

func TestScoreRelevanceCap(t *testing.T) {
	halflife := 12 * time.Hour
	low := Score(0.9, 0, false, time.Hour, halflife)
	high := Score(0.9, 100, false, time.Hour, halflife)
	if high < low {
		t.Errorf("关键词得分不应为负影响")
	}
	// 相关性有上限，不应爆炸
	if high > 1.5 {
		t.Errorf("得分异常偏高: %v", high)
	}
}

func TestAgeLabel(t *testing.T) {
	cases := []struct {
		d    time.Duration
		want string
	}{
		{30 * time.Second, "now"},
		{35 * time.Minute, "35m"},
		{2 * time.Hour, "2h"},
		{50 * time.Hour, "2d"},
	}
	for _, c := range cases {
		if got := AgeLabel(c.d); got != c.want {
			t.Errorf("AgeLabel(%v) = %q，期望 %q", c.d, got, c.want)
		}
	}
}
