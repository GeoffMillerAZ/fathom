package scenariobench

import "testing"

// tokenCounts mirrors contextbench's digit-boundary rule: "CLAUDE.md#L10"
// never counts inside "CLAUDE.md#L100".
func TestTokenCounts_LineCitationDigitBoundary(t *testing.T) {
	cases := []struct {
		text string
		want bool
	}{
		{"see CLAUDE.md#L100 for the rule", false},
		{"see CLAUDE.md#L10 for the rule", true},
		{"see CLAUDE.md#L10-L12 for the rule", true},
		{"CLAUDE.md#L101, then CLAUDE.md#L10.", true},
	}
	for _, c := range cases {
		if got := tokenCounts(c.text, "CLAUDE.md#L10"); got != c.want {
			t.Errorf("tokenCounts(%q) = %v, want %v", c.text, got, c.want)
		}
	}
}
