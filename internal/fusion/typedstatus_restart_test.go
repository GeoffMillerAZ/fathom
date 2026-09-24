package fusion_test

// typedstatus_restart_test.go — DG-10 (fathom diagnostics sweep): the
// daemon restarts roughly twice a day (O108 mitigation), and before this a
// typed-edge status lived only in fusion.Pipeline's in-memory map — most of
// the fleet's typed-edge health was unobservable through
// list_services/doctor at any given moment purely from restart timing,
// independent of whether anything was actually broken.
//
// This lives in internal/fusion, not internal/store: internal/store must
// stay pure Go (ADR-020/025, CLAUDE.md — `CGO_ENABLED=0 go test
// ./internal/store/...` is a gate), and any test file physically inside
// internal/store — even one in the external store_test package — is
// compiled into that same build; internal/fusion pulls in the cgo-built
// tree-sitter extractors, so a store-directory test importing it would
// break that purity gate. internal/fusion already imports internal/store
// freely (no such constraint runs the other way), so this is that
// dependency's natural, one-way home.

import (
	"context"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/connector"
	"github.com/geoffmilleraz/fathom/internal/fusion"
	"github.com/geoffmilleraz/fathom/internal/store"
)

// TestSQLiteStore_TypedEdgeStatusSurvivesRestart: a status written through
// one store handle, then read through a BRAND NEW Pipeline built on a
// freshly reopened handle to the SAME on-disk file (exactly what a
// restarted daemon does), must still answer known=true — Pipeline's own
// in-memory typedEdgeStatus map starts empty on the fresh Pipeline, so this
// proves the store fallback, not the in-process cache
// TestTypedEdgeStatus_RecordedOnService (typedenv_test.go) already covers.
func TestSQLiteStore_TypedEdgeStatusSurvivesRestart(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "typed-edge-status.sqlite")

	s1, err := store.NewSQLiteStore(path)
	if err != nil {
		t.Fatalf("NewSQLiteStore: %v", err)
	}
	want := store.TypedEdgeStatusRecord{
		PackagesSeen:           5,
		Packages:               5,
		EdgesTotal:             11,
		Fallback:               "GOWORK=off",
		DiscoveryGaps:          1,
		DiscoveryGapFiles:      []string{"adapter.go"},
		EdgesDroppedOutOfScope: 2,
	}
	if err := s1.SetTypedEdgeStatus(context.Background(), "svc", want); err != nil {
		t.Fatalf("SetTypedEdgeStatus: %v", err)
	}
	if err := s1.Close(); err != nil {
		t.Fatalf("close s1: %v", err)
	}

	s2, err := store.NewSQLiteStore(path)
	if err != nil {
		t.Fatalf("reopen NewSQLiteStore: %v", err)
	}
	t.Cleanup(func() { _ = s2.Close() })
	p2 := fusion.New(connector.NewLocalConnector(), s2, fusion.Config{Concurrency: 2})

	got, known := p2.TypedEdgeStatus("svc")
	if !known {
		t.Fatal("TypedEdgeStatus known=false for a fresh Pipeline over a store that already has a persisted status — DG-10's restart fallback did not fire")
	}
	if got.Packages != want.Packages {
		t.Errorf("Packages = %d, want %d", got.Packages, want.Packages)
	}
	if got.EdgesTotal != want.EdgesTotal {
		t.Errorf("EdgesTotal = %d, want %d", got.EdgesTotal, want.EdgesTotal)
	}
	if got.DiscoveryGaps != want.DiscoveryGaps {
		t.Errorf("DiscoveryGaps = %d, want %d", got.DiscoveryGaps, want.DiscoveryGaps)
	}
	if !reflect.DeepEqual(got.DiscoveryGapFiles, want.DiscoveryGapFiles) {
		t.Errorf("DiscoveryGapFiles = %v, want %v", got.DiscoveryGapFiles, want.DiscoveryGapFiles)
	}
	if !got.Persisted {
		t.Error("Persisted = false, want true — this status was NOT recorded by this process, it fell back to the store")
	}
	if got.RecordedAt.IsZero() {
		t.Error("RecordedAt is zero, want the watermark from the persisted record")
	}
}
