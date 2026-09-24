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
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/internal/dlog"
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

// setTypedEdgeStatusErrorStore wraps a real store.Store and forces
// SetTypedEdgeStatus to fail, so noteTypedEdgeStatus's error-handling path
// can be exercised without needing a genuinely broken on-disk store.
type setTypedEdgeStatusErrorStore struct {
	store.Store
	err error
}

func (s *setTypedEdgeStatusErrorStore) SetTypedEdgeStatus(ctx context.Context, service string, rec store.TypedEdgeStatusRecord) error {
	return s.err
}

// TestNoteTypedEdgeStatus_LogsStoreWriteFailure is the finding [6]
// regression test: noteTypedEdgeStatus used to discard SetTypedEdgeStatus's
// error with a bare `_ = ...`, violating the repo's never-silent doctrine
// (docs/vision/11-dial-to-eleven.md law 3, applied to this exact subsystem
// three lines away via p.diagWarn for its OTHER failure branches). A DG-10
// persistence write failing for any real operational reason (DB locked,
// disk full, the recurring TCC wedge on /Volumes this repo's own CLAUDE.md
// documents) must produce a warning naming the service and the error, not
// silence.
func TestNoteTypedEdgeStatus_LogsStoreWriteFailure(t *testing.T) {
	ctx := context.Background()
	wantErr := errors.New("boom: disk full")
	p := &Pipeline{st: &setTypedEdgeStatusErrorStore{Store: store.NewMemoryStore(), err: wantErr}}

	var buf strings.Builder
	p.SetLogger(dlog.New(&buf, dlog.LevelWarn, time.Now))

	p.noteTypedEdgeStatus(ctx, "svc", TypedEdgeStatus{Packages: 1})

	out := buf.String()
	if !strings.Contains(out, "svc") {
		t.Errorf("expected the warning to name the service %q, got %q", "svc", out)
	}
	if !strings.Contains(out, "boom: disk full") {
		t.Errorf("expected the warning to carry the store's error, got %q", out)
	}
}
