package wiring

import (
	"strings"
	"testing"
)

// TestLocalZoneIgnoreRegionUndoRemovesOnlyTheRegion pins that undo of the
// local-zone region restores a hand-written .fathom/.gitignore byte for byte.
func TestLocalZoneIgnoreRegionUndoRemovesOnlyTheRegion(t *testing.T) {
	root := t.TempDir()
	hand := "# mine\nscratch/\n"
	write(t, root, ".fathom/.gitignore", hand)
	spec := Spec{Regions: []Region{localZoneIgnoreRegion()}}
	res, err := ApplyWiring(root, spec, ApplyOptions{Now: fixedClock()})
	if err != nil {
		t.Fatal(err)
	}
	got := read(t, root, ".fathom/.gitignore")
	if !strings.HasPrefix(got, hand) || !strings.Contains(got, "memories.jsonl") {
		t.Fatalf("apply result = %q", got)
	}
	if _, err := Undo(root, res.Receipt.ID, ApplyOptions{Now: fixedClock()}); err != nil {
		t.Fatal(err)
	}
	if got := read(t, root, ".fathom/.gitignore"); got != hand {
		t.Fatalf("after undo = %q, want %q", got, hand)
	}
}
