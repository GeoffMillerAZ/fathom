package fathomformat

import (
	"slices"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/langroute"
)

// The language library does not list or read the dialects this package claims.
// langroute names them in ProductOnlyDialects, and this package's tool kinds
// are the other half of the same set.
func TestProductOnlyDialectsAreTheFormatsThisPackageClaims(t *testing.T) {
	claimed := []string{string(ToolFathomConfig), string(ToolFathomPlanRows), string(ToolFathomConcerns), string(ToolFathomLayerPacks)}
	slices.Sort(claimed)
	if got := langroute.ProductOnlyDialects(); !slices.Equal(got, claimed) {
		t.Fatalf("langroute.ProductOnlyDialects = %v, this package claims %v", got, claimed)
	}
	for _, id := range claimed {
		if langroute.DoorReadable(id) {
			t.Errorf("%s is claimed by the product's format dispatch and still readable by the door", id)
		}
	}
}
