package client

import (
	"errors"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/docindex"
)

// TestDocIndexResultTypesAreAliases pins that the doc-index result types
// a Client method returns are nameable from outside this module: each is
// an alias of the internal type (assignable both ways with no conversion,
// which a distinct defined type would refuse), and the unavailable-index
// sentinel is the one value the internal package returns.
func TestDocIndexResultTypesAreAliases(t *testing.T) {
	var ranked RankedDoc = docindex.RankedDoc{Path: "docs/a.md", Score: 1}
	var internalRanked docindex.RankedDoc = ranked
	if internalRanked.Path != "docs/a.md" {
		t.Fatalf("RankedDoc round trip lost a field: %+v", internalRanked)
	}

	var drifted DriftedDocLink = docindex.DriftedDocLink{Path: "docs/a.md", Target: "x.go"}
	var internalDrifted docindex.DriftedDocLink = drifted
	if internalDrifted.Target != "x.go" {
		t.Fatalf("DriftedDocLink round trip lost a field: %+v", internalDrifted)
	}

	if !errors.Is(docindex.ErrDocIndexUnavailable, ErrDocIndexUnavailable) {
		t.Fatal("client.ErrDocIndexUnavailable is not the internal sentinel")
	}
}
