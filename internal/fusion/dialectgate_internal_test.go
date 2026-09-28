package fusion

import (
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/connector"
)

// TestGateDialectFile_SniffAndAmbiguous: the gate's verdicts for the two
// classification outcomes no shipped manifest exercises yet (no dialect
// with a sniff is registered): a sniffed file is counted and extracted, an
// ambiguous one excluded, a minified sniffed one excluded and still
// counted as sniffed.
func TestGateDialectFile_SniffAndAmbiguous(t *testing.T) {
	rules := []connector.DialectRule{
		{ID: "perl", Sniff: &connector.DialectSniffRule{Extensions: []string{".pl"}, Pattern: `\buse\s+strict\b`, Priority: 0}},
		{ID: "prolog", Sniff: &connector.DialectSniffRule{Extensions: []string{".pl"}, Pattern: `:-`, Priority: 1}},
	}
	cases := []struct {
		name, content string
		want          dialectVerdict
	}{
		{"sniffed", "use strict;\n", dialectVerdict{sniffed: true}},
		{"ambiguous", "hello\n", dialectVerdict{excluded: ExcludedAmbiguous}},
		{"sniffed minified", "use strict;" + strings.Repeat("x", 900), dialectVerdict{excluded: ExcludedSkippedGenerated, sniffed: true}},
	}
	for _, c := range cases {
		if got := gateDialectFileWith(rules, "a.pl", []byte(c.content)); got != c.want {
			t.Errorf("%s: got %+v, want %+v", c.name, got, c.want)
		}
	}
	// A file no dialect owns is never gated, minified or not.
	if got := gateDialectFileWith(rules, "a.txt", []byte(strings.Repeat("x", 900))); got != (dialectVerdict{}) {
		t.Errorf("unowned file gated: %+v", got)
	}
}
