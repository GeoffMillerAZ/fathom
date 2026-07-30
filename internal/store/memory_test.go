package store

import (
	"context"
	"reflect"
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/internal/core"
)

func TestMemoryStoreContract(t *testing.T) {
	s := NewMemoryStore()
	t.Cleanup(func() { _ = s.Close() })
	storeContract(t, s)
}

// TestMemoryStoreImplementsStore is a compile-time check that *MemoryStore satisfies
// the Store interface.
func TestMemoryStoreImplementsStore(t *testing.T) {
	var _ Store = (*MemoryStore)(nil)
}

// TestMemoryStore_RecordCompileRun_RoundTripsRejectionReasons (Q-3): struct-
// copy semantics already carry the new field, asserted directly rather than
// left to inspection.
func TestMemoryStore_RecordCompileRun_RoundTripsRejectionReasons(t *testing.T) {
	s := NewMemoryStore()
	t.Cleanup(func() { _ = s.Close() })
	ctx := context.Background()

	want := []core.RejectionBreakdown{{Cause: "over-broad", Count: 1, Samples: []string{"docs/a.md#x: scope \"**\" is over-broad"}}}
	if err := s.RecordCompileRun(ctx, core.CompileRun{
		RunID: "run-1", ServiceID: "svc", Layer: "obligations",
		RanAt: time.Unix(1, 0).UTC(), RejectionReasons: want,
	}); err != nil {
		t.Fatalf("RecordCompileRun: %v", err)
	}
	got, err := s.CompileRuns(ctx, "svc", "")
	if err != nil {
		t.Fatalf("CompileRuns: %v", err)
	}
	if len(got) != 1 || len(got[0].RejectionReasons) != 1 || !reflect.DeepEqual(got[0].RejectionReasons[0], want[0]) {
		t.Fatalf("got %+v, want RejectionReasons %+v", got, want)
	}
}

// TestMemoryStore_ClaimEventsSince_FiltersByWindow (Q-6).
func TestMemoryStore_ClaimEventsSince_FiltersByWindow(t *testing.T) {
	s := NewMemoryStore()
	t.Cleanup(func() { _ = s.Close() })
	ctx := context.Background()

	events := []core.ClaimEvent{
		{ClaimID: "c1", Verdict: "superseded", Author: "review", At: time.Unix(10, 0).UTC()},
		{ClaimID: "c2", Verdict: "refuted", Author: "human", At: time.Unix(20, 0).UTC()},
		{ClaimID: "c3", Verdict: "superseded", Author: "review", At: time.Unix(30, 0).UTC()},
	}
	for _, ev := range events {
		if err := s.AppendClaimEvent(ctx, ev); err != nil {
			t.Fatalf("AppendClaimEvent: %v", err)
		}
	}
	got, err := s.ClaimEventsSince(ctx, time.Unix(15, 0).UTC(), time.Unix(25, 0).UTC())
	if err != nil {
		t.Fatalf("ClaimEventsSince: %v", err)
	}
	if len(got) != 1 || got[0].ClaimID != "c2" {
		t.Fatalf("got %+v, want only c2 (in [15,25])", got)
	}
}
