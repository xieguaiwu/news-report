package s2t

import "testing"

func TestToSimplified(t *testing.T) {
	tests := []struct{ in, want string }{
		{"國際", "国际"},
		{"經濟", "经济"},
		{"選舉", "选举"},
		{"臺灣", "台湾"},
		{"hello世界", "hello世界"},
		{"", ""},
		{"ABC", "ABC"},
	}
	for _, tt := range tests {
		got := ToSimplified(tt.in)
		if got != tt.want {
			t.Errorf("ToSimplified(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestToTraditional(t *testing.T) {
	tests := []struct{ in, want string }{
		{"国际", "國際"},
		{"经济", "經濟"},
		{"选举", "選舉"},
		{"台湾", "臺灣"},
		{"hello世界", "hello世界"},
		{"", ""},
	}
	for _, tt := range tests {
		got := ToTraditional(tt.in)
		if got != tt.want {
			t.Errorf("ToTraditional(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestNormalize(t *testing.T) {
	// Normalize should unify both to simplified
	if got := Normalize("國際經濟"); got != "国际经济" {
		t.Errorf("Normalize(國際經濟) = %q, want 国际经济", got)
	}
	if got := Normalize("国际经济"); got != "国际经济" {
		t.Errorf("Normalize(国际经济) = %q, want 国际经济", got)
	}
}
