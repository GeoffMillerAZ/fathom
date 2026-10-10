package query

import (
	"testing"

	"github.com/geoffmilleraz/fathom/internal/core"
)

// symbol_qualifier_test.go — O770 / SR-77: a qualified query for an ordinary
// bare-stored name matches only when the qualifier ends the symbol's package
// or file stem path (the module-qualifier rule), never on the bare suffix alone.
func TestSymbolMatchesQualifierMustNameTheSymbolsFile(t *testing.T) {
	o := core.Observation{Symbol: "get_debug_flag", Package: "flask", SourceRef: "src/flask/helpers.py"}
	for q, want := range map[string]bool{
		"get_debug_flag":                   true,
		"helpers.get_debug_flag":           true,
		"flask.get_debug_flag":             true,
		"flask.helpers.get_debug_flag":     true,
		"src/flask/helpers.get_debug_flag": true,
		"zzz.get_debug_flag":               false,
		"lpers.get_debug_flag":             false,
	} {
		if got := symbolMatches(o, q); got != want {
			t.Errorf("symbolMatches(%q) = %v, want %v", q, got, want)
		}
	}
}
