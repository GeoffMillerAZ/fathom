package extractor

import (
	"slices"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/extractor/tsengine"
)

// TestDialectGrammarsInLock (LB-F12, I55): every blob and entry point a
// dialect manifest names is described by grammars.lock.json, so no manifest
// can reach a grammar the lock does not pin.
func TestDialectGrammarsInLock(t *testing.T) {
	lock, err := tsengine.LoadGrammarLock()
	if err != nil {
		t.Fatal(err)
	}
	ds, err := LoadDialects()
	if err != nil {
		t.Fatal(err)
	}
	for _, d := range ds {
		if d.Grammar == nil {
			continue
		}
		row, ok := lock.Row(d.Grammar.Blob)
		if !ok {
			t.Errorf("%s: blob %q has no lock row", d.ID, d.Grammar.Blob)
			continue
		}
		eps := []string{d.Grammar.EntryPoint}
		for _, v := range d.Variants {
			eps = append(eps, v.EntryPoint)
		}
		for _, ep := range eps {
			if !slices.Contains(row.EntryPoints, ep) {
				t.Errorf("%s: entry point %q is not in the %s lock row %v", d.ID, ep, row.Blob, row.EntryPoints)
			}
		}
	}
}
