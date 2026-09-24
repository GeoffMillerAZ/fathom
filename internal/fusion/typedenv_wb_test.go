package fusion

// typedenv_wb_test.go — white-box (package fusion, not fusion_test), the
// same convention typedpass_serial_test.go documents: DG-9 pins
// noteTypedEdgeStatus's own field-copying behavior directly, since driving
// a real connector-discovery gap end-to-end through Pipeline.Ingest would
// need a throwaway fixture copy with its own .fathomignore — this is the
// narrower, faster proof that the three fields actually travel from a
// TypedEdgeReport-shaped TypedEdgeStatus onto the per-service record
// list_services/staleness_report/doctor read.

import (
	"context"
	"reflect"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/store"
)

// TestNoteTypedEdgeStatus_CarriesDiscoveryGaps: before DG-9,
// noteTypedEdgeStatus's TypedEdgeStatus{...} literals at every call site in
// typedcalls.go named PackagesSeen/Packages/EdgesTotal/Failed/Reason/
// Fallback but never DiscoveryGaps/DiscoveryGapFiles/EdgesDroppedOutOfScope
// — EnrichTypedEdges computed, deduped and printed those to stderr every
// run (docs/intent/invariants.md I24's documented gap), but no
// list_services/staleness_report/doctor caller could ever see them.
func TestNoteTypedEdgeStatus_CarriesDiscoveryGaps(t *testing.T) {
	ctx := context.Background()
	p := &Pipeline{st: store.NewMemoryStore()}

	want := TypedEdgeStatus{
		PackagesSeen:           2,
		Packages:               1,
		EdgesTotal:             3,
		Failed:                 true,
		Reason:                 "1/2 package(s) failed typed loading",
		DiscoveryGaps:          1,
		DiscoveryGapFiles:      []string{"adapter.go"},
		EdgesDroppedOutOfScope: 4,
	}
	p.noteTypedEdgeStatus(ctx, "svc", want)

	got, known := p.TypedEdgeStatus("svc")
	if !known {
		t.Fatal("TypedEdgeStatus reported unknown right after noteTypedEdgeStatus")
	}
	if got.DiscoveryGaps != want.DiscoveryGaps {
		t.Errorf("DiscoveryGaps = %d, want %d", got.DiscoveryGaps, want.DiscoveryGaps)
	}
	if !reflect.DeepEqual(got.DiscoveryGapFiles, want.DiscoveryGapFiles) {
		t.Errorf("DiscoveryGapFiles = %v, want %v", got.DiscoveryGapFiles, want.DiscoveryGapFiles)
	}
	if got.EdgesDroppedOutOfScope != want.EdgesDroppedOutOfScope {
		t.Errorf("EdgesDroppedOutOfScope = %d, want %d", got.EdgesDroppedOutOfScope, want.EdgesDroppedOutOfScope)
	}
}
