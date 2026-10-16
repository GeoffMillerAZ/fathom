package extractiongaplog_test

import (
	"bytes"
	"testing"
	"time"

	gaplog "github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/evidence/extraction-gap-log"
	improvementflywheel "github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/orchestration/improvement-flywheel"
)

// ---------------------------------------------------------------------------
// Fixed clock.
// ---------------------------------------------------------------------------

var fixedNow = time.Date(2026, 7, 10, 9, 0, 0, 0, time.UTC)

func fixedClock() time.Time { return fixedNow }

// ---------------------------------------------------------------------------
// GapRecord validation.
// ---------------------------------------------------------------------------

func TestGapRecordValidate(t *testing.T) {
	t.Parallel()

	valid := gaplog.GapRecord{
		ConcernID:     "c-001",
		ScopeSearched: "**/*.go",
		Outcome:       gaplog.OutcomeFound,
		OccurredAt:    "2026-07-10T09:00:00Z",
	}
	if err := valid.Validate(); err != nil {
		t.Errorf("valid record: unexpected error: %v", err)
	}

	cases := []struct {
		name    string
		rec     gaplog.GapRecord
		wantErr bool
	}{
		{"empty concern_id", gaplog.GapRecord{ScopeSearched: "x", Outcome: gaplog.OutcomeFound, OccurredAt: "2026-01-01T00:00:00Z"}, true},
		{"empty occurred_at", gaplog.GapRecord{ConcernID: "c", ScopeSearched: "x", Outcome: gaplog.OutcomeFound}, true},
		{"invalid outcome", gaplog.GapRecord{ConcernID: "c", ScopeSearched: "x", Outcome: "bad", OccurredAt: "2026-01-01T00:00:00Z"}, true},
		{"absent_proven valid", gaplog.GapRecord{ConcernID: "c", ScopeSearched: "x", Outcome: gaplog.OutcomeAbsentProven, OccurredAt: "2026-01-01T00:00:00Z"}, false},
		{"unresolved valid", gaplog.GapRecord{ConcernID: "c", ScopeSearched: "x", Outcome: gaplog.OutcomeUnresolved, OccurredAt: "2026-01-01T00:00:00Z"}, false},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			err := tc.rec.Validate()
			if (err != nil) != tc.wantErr {
				t.Errorf("Validate() error=%v, wantErr=%v", err, tc.wantErr)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// GapLog append + load round-trip.
// ---------------------------------------------------------------------------

func TestGapLog_AppendAndLoad(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	gl, err := gaplog.NewGapLog(&buf, fixedClock)
	if err != nil {
		t.Fatal(err)
	}

	recs := []gaplog.GapRecord{
		{ConcernID: "c-001", ScopeSearched: "**/*.go", Outcome: gaplog.OutcomeFound, FoundRefs: []string{"file://main.go#L1-L2"}, OccurredAt: "2026-07-10T09:00:00Z"},
		{ConcernID: "c-002", ScopeSearched: "docs/**", Outcome: gaplog.OutcomeUnresolved, OccurredAt: "2026-07-10T10:00:00Z"},
	}
	for _, r := range recs {
		if err := gl.Append(r); err != nil {
			t.Fatalf("Append: %v", err)
		}
	}

	loaded, err := gaplog.Load(&buf, false)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(loaded) != 2 {
		t.Errorf("Load() = %d records, want 2", len(loaded))
	}
	if loaded[0].ConcernID != "c-001" || loaded[1].ConcernID != "c-002" {
		t.Errorf("unexpected records: %+v", loaded)
	}
}

// ---------------------------------------------------------------------------
// Injected clock: OccurredAt set from clock when empty.
// ---------------------------------------------------------------------------

func TestGapLog_ClockInjection(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	gl, _ := gaplog.NewGapLog(&buf, fixedClock)

	// Append without OccurredAt — should be set from clock.
	rec := gaplog.GapRecord{
		ConcernID:     "c-clock",
		ScopeSearched: "**/*.go",
		Outcome:       gaplog.OutcomeUnresolved,
	}
	if err := gl.Append(rec); err != nil {
		t.Fatalf("Append: %v", err)
	}

	loaded, _ := gaplog.Load(&buf, false)
	if len(loaded) != 1 {
		t.Fatalf("expected 1 record, got %d", len(loaded))
	}
	want := "2026-07-10T09:00:00Z"
	if loaded[0].OccurredAt != want {
		t.Errorf("OccurredAt = %q, want %q", loaded[0].OccurredAt, want)
	}
}

// ---------------------------------------------------------------------------
// NewGapLog: nil clock rejected.
// ---------------------------------------------------------------------------

func TestNewGapLog_NilClockRejected(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	if _, err := gaplog.NewGapLog(&buf, nil); err == nil {
		t.Error("nil clock should be rejected")
	}
}

// ---------------------------------------------------------------------------
// Projection: HitRate.
// ---------------------------------------------------------------------------

func TestHitRate(t *testing.T) {
	t.Parallel()
	recs := []gaplog.GapRecord{
		{ConcernID: "c-001", ScopeSearched: "x", Outcome: gaplog.OutcomeFound, OccurredAt: "2026-07-10T09:00:00Z"},
		{ConcernID: "c-001", ScopeSearched: "x", Outcome: gaplog.OutcomeUnresolved, OccurredAt: "2026-07-10T10:00:00Z"},
		{ConcernID: "c-002", ScopeSearched: "y", Outcome: gaplog.OutcomeUnresolved, OccurredAt: "2026-07-10T11:00:00Z"},
	}
	// c-001 had 2 gaps out of 4 runs → hit_rate = 0.5
	// c-002 had 1 gap out of 2 runs → hit_rate = 0.5
	totalRuns := map[string]int{"c-001": 4, "c-002": 2}
	rates := gaplog.HitRate(recs, totalRuns)

	if got := rates["c-001"]; got != 0.5 {
		t.Errorf("c-001 hit rate = %g, want 0.5", got)
	}
	if got := rates["c-002"]; got != 0.5 {
		t.Errorf("c-002 hit rate = %g, want 0.5", got)
	}

	// Unknown concern with no total runs → 0.
	recs2 := []gaplog.GapRecord{
		{ConcernID: "c-new", ScopeSearched: "z", Outcome: gaplog.OutcomeFound, OccurredAt: "2026-07-10T09:00:00Z"},
	}
	rates2 := gaplog.HitRate(recs2, map[string]int{})
	if rates2["c-new"] != 0.0 {
		t.Errorf("c-new with no total runs should have 0.0 hit rate, got %g", rates2["c-new"])
	}
}

// ---------------------------------------------------------------------------
// Projection: WarmList.
// ---------------------------------------------------------------------------

func TestWarmList(t *testing.T) {
	t.Parallel()
	recs := []gaplog.GapRecord{
		// c-001 missed twice, found once with refs.
		{ConcernID: "c-001", ScopeSearched: "x", Outcome: gaplog.OutcomeUnresolved, OccurredAt: "2026-07-10T09:00:00Z"},
		{ConcernID: "c-001", ScopeSearched: "x", Outcome: gaplog.OutcomeFound, FoundRefs: []string{"file://main.go#L1-L1"}, OccurredAt: "2026-07-10T10:00:00Z"},
		// c-002 missed once, no found refs → should not appear in WarmList.
		{ConcernID: "c-002", ScopeSearched: "y", Outcome: gaplog.OutcomeUnresolved, OccurredAt: "2026-07-10T11:00:00Z"},
		// c-003 missed three times, found once.
		{ConcernID: "c-003", ScopeSearched: "z", Outcome: gaplog.OutcomeFound, FoundRefs: []string{"file://a.go#L5-L5", "file://b.go#L2-L2"}, OccurredAt: "2026-07-10T12:00:00Z"},
		{ConcernID: "c-003", ScopeSearched: "z", Outcome: gaplog.OutcomeUnresolved, OccurredAt: "2026-07-10T13:00:00Z"},
		{ConcernID: "c-003", ScopeSearched: "z", Outcome: gaplog.OutcomeUnresolved, OccurredAt: "2026-07-10T14:00:00Z"},
	}

	warm := gaplog.WarmList(recs)

	// Only c-001 and c-003 have found refs.
	if len(warm) != 2 {
		t.Fatalf("WarmList() = %d entries, want 2: %+v", len(warm), warm)
	}
	// c-003 should be first (3 misses > 2 misses).
	if warm[0].ConcernID != "c-003" {
		t.Errorf("WarmList[0] = %q, want c-003", warm[0].ConcernID)
	}
	if warm[1].ConcernID != "c-001" {
		t.Errorf("WarmList[1] = %q, want c-001", warm[1].ConcernID)
	}

	// Honest caveat: warm entries are Tier-1 signal only.
	// Verify they have refs (which is what would be cached).
	if len(warm[0].Refs) == 0 {
		t.Error("WarmList entry should have refs")
	}
}

// ---------------------------------------------------------------------------
// Projection: Signals.
// ---------------------------------------------------------------------------

func TestSignals_EmitsFlywheelSignals(t *testing.T) {
	t.Parallel()
	recs := []gaplog.GapRecord{
		{ConcernID: "c-001", ScopeSearched: "x", Outcome: gaplog.OutcomeUnresolved, OccurredAt: "2026-07-10T09:00:00Z"},
		{ConcernID: "c-001", ScopeSearched: "x", Outcome: gaplog.OutcomeUnresolved, OccurredAt: "2026-07-10T10:00:00Z"},
		{ConcernID: "c-002", ScopeSearched: "y", Outcome: gaplog.OutcomeFound, OccurredAt: "2026-07-10T11:00:00Z"}, // only 1 gap → no signal
	}

	signals := gaplog.Signals(recs)

	if len(signals) != 1 {
		t.Fatalf("Signals() = %d, want 1", len(signals))
	}
	if signals[0].Key != "c-001" {
		t.Errorf("signal Key = %q, want c-001", signals[0].Key)
	}
	if signals[0].Type != improvementflywheel.PatternOperatorFriction {
		t.Errorf("signal Type = %q, want operator_friction", signals[0].Type)
	}
}

func TestSignals_NoSignalsUnderThreshold(t *testing.T) {
	t.Parallel()
	recs := []gaplog.GapRecord{
		{ConcernID: "c-only-once", ScopeSearched: "x", Outcome: gaplog.OutcomeUnresolved, OccurredAt: "2026-07-10T09:00:00Z"},
	}
	if signals := gaplog.Signals(recs); len(signals) != 0 {
		t.Errorf("single gap should not emit signal, got %d", len(signals))
	}
}

func TestSignals_EmptyRecs(t *testing.T) {
	t.Parallel()
	if signals := gaplog.Signals(nil); len(signals) != 0 {
		t.Errorf("nil recs should give empty signals, got %d", len(signals))
	}
}

// ---------------------------------------------------------------------------
// Signals are deterministically sorted by concern ID.
// ---------------------------------------------------------------------------

func TestSignals_Deterministic(t *testing.T) {
	t.Parallel()
	// Three concerns, each with >=2 gaps.
	recs := []gaplog.GapRecord{
		{ConcernID: "z-concern", ScopeSearched: "x", Outcome: gaplog.OutcomeUnresolved, OccurredAt: "2026-07-10T09:00:00Z"},
		{ConcernID: "z-concern", ScopeSearched: "x", Outcome: gaplog.OutcomeUnresolved, OccurredAt: "2026-07-10T10:00:00Z"},
		{ConcernID: "a-concern", ScopeSearched: "y", Outcome: gaplog.OutcomeUnresolved, OccurredAt: "2026-07-10T11:00:00Z"},
		{ConcernID: "a-concern", ScopeSearched: "y", Outcome: gaplog.OutcomeUnresolved, OccurredAt: "2026-07-10T12:00:00Z"},
		{ConcernID: "m-concern", ScopeSearched: "z", Outcome: gaplog.OutcomeUnresolved, OccurredAt: "2026-07-10T13:00:00Z"},
		{ConcernID: "m-concern", ScopeSearched: "z", Outcome: gaplog.OutcomeUnresolved, OccurredAt: "2026-07-10T14:00:00Z"},
	}

	s1 := gaplog.Signals(recs)
	s2 := gaplog.Signals(recs)

	if len(s1) != len(s2) {
		t.Fatalf("non-deterministic length: %d vs %d", len(s1), len(s2))
	}
	for i := range s1 {
		if s1[i].Key != s2[i].Key {
			t.Errorf("non-deterministic at %d: %q vs %q", i, s1[i].Key, s2[i].Key)
		}
	}
	// Should be alphabetical: a, m, z.
	if len(s1) == 3 {
		if s1[0].Key != "a-concern" || s1[1].Key != "m-concern" || s1[2].Key != "z-concern" {
			t.Errorf("signals not sorted: %v", func() []string {
				var ks []string
				for _, s := range s1 {
					ks = append(ks, s.Key)
				}
				return ks
			}())
		}
	}
}
