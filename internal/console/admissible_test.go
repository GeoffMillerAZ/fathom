package console

import (
	"strings"
	"testing"
)

// PE-17 (docs/design/progress-and-economics.md §5, §6): fathom-on versus
// off is shown only when G2's admissibility test passes for the compared
// units (docs/intent/scope.md G2: ≥20 arm-homogeneous units per arm,
// MI-5) — otherwise the panel names why it is withheld, naming the short
// arm rather than a blanket refusal.
func TestOnOffWithheldUnlessAdmissible(t *testing.T) {
	cases := []struct {
		name           string
		on, off        int
		wantAdmissible bool
	}{
		{"both short", 5, 3, false},
		{"on short only", 19, 20, false},
		{"off short only", 25, 19, false},
		{"exactly at threshold", 20, 20, true},
		{"well above threshold", 41, 33, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := admissibleComparison(tc.on, tc.off)
			if c.Admissible != tc.wantAdmissible {
				t.Fatalf("Admissible = %v, want %v (on=%d off=%d): %+v", c.Admissible, tc.wantAdmissible, tc.on, tc.off, c)
			}
			if c.Admissible && c.Withheld != "" {
				t.Fatalf("an admissible comparison must carry no Withheld reason: %+v", c)
			}
			if !c.Admissible && c.Withheld == "" {
				t.Fatalf("a withheld comparison must name why (I38 — never a silent blank): %+v", c)
			}
			if c.OnUnits != tc.on || c.OffUnits != tc.off {
				t.Fatalf("unit counts not carried through: %+v", c)
			}
		})
	}

	// The withheld reason names the specific short arm, not just "no".
	on := admissibleComparison(3, 25)
	if on.Admissible {
		t.Fatal("3 ON-arm units must not be admissible")
	}
	if !containsAll(on.Withheld, "3", "20") {
		t.Fatalf("withheld reason should cite the ON count and the threshold: %q", on.Withheld)
	}
	off := admissibleComparison(25, 3)
	if off.Admissible {
		t.Fatal("3 OFF-arm units must not be admissible")
	}
	if !containsAll(off.Withheld, "3", "20") {
		t.Fatalf("withheld reason should cite the OFF count and the threshold: %q", off.Withheld)
	}
}

func containsAll(s string, subs ...string) bool {
	for _, sub := range subs {
		if !strings.Contains(s, sub) {
			return false
		}
	}
	return true
}
