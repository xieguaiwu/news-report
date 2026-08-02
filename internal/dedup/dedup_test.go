package dedup

import "testing"

func TestNormalizeTitle(t *testing.T) {
	cases := []struct{ in, want string }{
		{"EU Leaders Agree On New Sanctions!", "eu leaders agree on new sanctions"},
		{"EU   Leaders — Agree (On) [New] Sanctions", "eu leaders agree on new sanctions"},
		{"Zurück in die Zukunft: Ölpreis steigt", "zurück in die zukunft ölpreis steigt"},
		{"  ", ""},
	}
	for _, c := range cases {
		if got := NormalizeTitle(c.in); got != c.want {
			t.Errorf("NormalizeTitle(%q) = %q，期望 %q", c.in, got, c.want)
		}
	}
}

func TestIsDuplicate(t *testing.T) {
	cases := []struct {
		a, b   string
		aL, bL string
		want   bool
	}{
		{"EU leaders agree on new sanctions package", "EU leaders agree on new sanctions package", "en", "en", true},
		{"EU leaders agree on new sanctions package", "EU Leaders Agree On New Sanctions Package!", "en", "en", true},
		{"EU leaders agree on new sanctions package", "EU leaders agree on sanctions package details", "en", "en", true},
		{"EU leaders agree on new sanctions package", "Paris fashion week opens with new shows", "en", "en", false},
		{"EU leaders agree on new sanctions package", "Die EU einigt sich auf neue Sanktionen", "en", "de", false},
		{"", "anything", "en", "en", false},
	}
	for _, c := range cases {
		if got := IsDuplicate(c.a, c.b, c.aL, c.bL); got != c.want {
			t.Errorf("IsDuplicate(%q, %q) = %v，期望 %v", c.a, c.b, got, c.want)
		}
	}
}

func TestJaccard(t *testing.T) {
	a := []string{"eu", "leaders", "agree", "sanctions"}
	b := []string{"eu", "leaders", "agree", "sanctions", "package"}
	if got := Jaccard(a, b); got != 0.8 {
		t.Errorf("Jaccard 期望 0.8，实际 %v", got)
	}
	if got := Jaccard(a, []string{}); got != 0 {
		t.Errorf("空集 Jaccard 应为 0，实际 %v", got)
	}
}
