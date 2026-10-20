package readout

import "testing"

func TestUnplacedIdentsSkipsPlacedAndRepeatedRoots(t *testing.T) {
	recs := CompileErrors(`other/x.go:5:2: undefined: Bar
other/y.go:7:9: undefined: Bar
cart/cart.go:10:20: not enough arguments in call to money.Round
cart/cart.go:11:2: "fmt" imported and not used
`, nil)
	got := UnplacedIdents(recs, []CompileDef{{Name: "money.Round", File: "money/money.go", Line: 3}})
	if len(got) != 1 || got[0] != "Bar" {
		t.Errorf("UnplacedIdents = %q, want [Bar]", got)
	}
}
