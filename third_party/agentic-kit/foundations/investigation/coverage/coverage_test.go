package coverage_test

import (
	"testing"
	"time"

	"github.com/geoffmilleraz/agentic-kit/foundations/investigation/coverage"
)

// validCoverage returns a well-formed SourceCoverage for use in tests.
func validCoverage() coverage.SourceCoverage {
	return coverage.SourceCoverage{
		SourceID:       "src-emails",
		RecordsKnown:   100,
		RecordsScanned: 60,
		RecordsFailed:  5,
		RecordsPending: 35,
		LastSync:       time.Date(2026, 6, 8, 12, 0, 0, 0, time.UTC),
		Cursor:         "page=3",
	}
}

// ---------------------------------------------------------------------------
// Validate
// ---------------------------------------------------------------------------

func TestValidateHappyPath(t *testing.T) {
	if err := validCoverage().Validate(); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}

func TestValidateEmptySourceID(t *testing.T) {
	c := validCoverage()
	c.SourceID = ""
	if err := c.Validate(); err == nil {
		t.Fatal("expected error for empty SourceID, got nil")
	}
}

func TestValidateNegativeCounts(t *testing.T) {
	cases := []struct {
		name  string
		mutFn func(*coverage.SourceCoverage)
	}{
		{"negative-Known", func(c *coverage.SourceCoverage) { c.RecordsKnown = -1 }},
		{"negative-Scanned", func(c *coverage.SourceCoverage) { c.RecordsScanned = -1 }},
		{"negative-Failed", func(c *coverage.SourceCoverage) { c.RecordsFailed = -1 }},
		{"negative-Pending", func(c *coverage.SourceCoverage) { c.RecordsPending = -1 }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := validCoverage()
			tc.mutFn(&c)
			if err := c.Validate(); err == nil {
				t.Errorf("%s: expected validation error, got nil", tc.name)
			}
		})
	}
}

func TestValidateInvariantViolation(t *testing.T) {
	// Scanned + Failed + Pending > Known must be rejected.
	c := coverage.SourceCoverage{
		SourceID:       "src-x",
		RecordsKnown:   10,
		RecordsScanned: 7,
		RecordsFailed:  2,
		RecordsPending: 3, // 7+2+3=12 > 10
	}
	if err := c.Validate(); err == nil {
		t.Fatal("expected error for Scanned+Failed+Pending > Known, got nil")
	}
}

func TestValidateExactlyAtBoundary(t *testing.T) {
	// Scanned + Failed + Pending == Known is valid.
	c := coverage.SourceCoverage{
		SourceID:       "src-y",
		RecordsKnown:   10,
		RecordsScanned: 5,
		RecordsFailed:  2,
		RecordsPending: 3, // 5+2+3=10 == 10
	}
	if err := c.Validate(); err != nil {
		t.Fatalf("expected no error at boundary, got %v", err)
	}
}

func TestValidateZeroKnown(t *testing.T) {
	// All zeros is valid (source advertises nothing yet).
	c := coverage.SourceCoverage{
		SourceID:       "src-empty",
		RecordsKnown:   0,
		RecordsScanned: 0,
		RecordsFailed:  0,
		RecordsPending: 0,
	}
	if err := c.Validate(); err != nil {
		t.Fatalf("all-zeros should be valid, got %v", err)
	}
}

// ---------------------------------------------------------------------------
// Ratio
// ---------------------------------------------------------------------------

func TestRatioNormal(t *testing.T) {
	c := coverage.SourceCoverage{
		SourceID:       "src-r",
		RecordsKnown:   200,
		RecordsScanned: 100,
	}
	if got := c.Ratio(); got != 0.5 {
		t.Errorf("Ratio: got %v, want 0.5", got)
	}
}

func TestRatioKnownZero(t *testing.T) {
	c := coverage.SourceCoverage{SourceID: "src-r"}
	if got := c.Ratio(); got != 0 {
		t.Errorf("Ratio with Known=0: got %v, want 0", got)
	}
}

func TestRatioFull(t *testing.T) {
	c := coverage.SourceCoverage{
		SourceID:       "src-r",
		RecordsKnown:   50,
		RecordsScanned: 50,
	}
	if got := c.Ratio(); got != 1.0 {
		t.Errorf("Ratio fully scanned: got %v, want 1.0", got)
	}
}

// ---------------------------------------------------------------------------
// KnownUnknowns
// ---------------------------------------------------------------------------

func TestKnownUnknowns(t *testing.T) {
	cases := []struct {
		name   string
		c      coverage.SourceCoverage
		wantKU int
	}{
		{
			name: "partial",
			c: coverage.SourceCoverage{
				SourceID:       "src-ku",
				RecordsKnown:   100,
				RecordsScanned: 60,
				RecordsFailed:  10,
				RecordsPending: 30,
			},
			wantKU: 30, // 100 - 60 - 10
		},
		{
			name: "fully-scanned",
			c: coverage.SourceCoverage{
				SourceID:       "src-full",
				RecordsKnown:   50,
				RecordsScanned: 50,
				RecordsFailed:  0,
				RecordsPending: 0,
			},
			wantKU: 0,
		},
		{
			name: "none-scanned",
			c: coverage.SourceCoverage{
				SourceID:       "src-none",
				RecordsKnown:   10,
				RecordsScanned: 0,
				RecordsFailed:  0,
				RecordsPending: 10,
			},
			wantKU: 10,
		},
		{
			name: "zero-known",
			c: coverage.SourceCoverage{
				SourceID: "src-zero",
			},
			wantKU: 0,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.c.KnownUnknowns(); got != tc.wantKU {
				t.Errorf("KnownUnknowns: got %d, want %d", got, tc.wantKU)
			}
		})
	}
}

func TestKnownUnknownsClampedAtZero(t *testing.T) {
	// Direct construction bypasses Validate; Scanned+Failed > Known must not
	// return a negative "known unknowns".
	c := coverage.SourceCoverage{
		SourceID:       "src-overflow",
		RecordsKnown:   3,
		RecordsScanned: 2,
		RecordsFailed:  2, // 2+2=4 > 3 — invalid but constructible directly
	}
	if got := c.KnownUnknowns(); got != 0 {
		t.Errorf("KnownUnknowns with Scanned+Failed>Known: got %d, want 0", got)
	}
}

// ---------------------------------------------------------------------------
// Complete
// ---------------------------------------------------------------------------

func TestComplete(t *testing.T) {
	cases := []struct {
		name     string
		c        coverage.SourceCoverage
		wantDone bool
	}{
		{
			name: "complete",
			c: coverage.SourceCoverage{
				SourceID:       "src-done",
				RecordsKnown:   5,
				RecordsScanned: 5,
			},
			wantDone: true,
		},
		{
			name: "pending-remains",
			c: coverage.SourceCoverage{
				SourceID:       "src-pending",
				RecordsKnown:   5,
				RecordsScanned: 4,
				RecordsPending: 1,
			},
			wantDone: false,
		},
		{
			name: "failed-remains",
			c: coverage.SourceCoverage{
				SourceID:       "src-failed",
				RecordsKnown:   5,
				RecordsScanned: 4,
				RecordsFailed:  1,
			},
			wantDone: false,
		},
		{
			name: "known-zero",
			c: coverage.SourceCoverage{
				SourceID: "src-empty",
			},
			wantDone: false,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.c.Complete(); got != tc.wantDone {
				t.Errorf("Complete: got %v, want %v", got, tc.wantDone)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Tracker Set / Get / All
// ---------------------------------------------------------------------------

func TestTrackerSetGetRoundtrip(t *testing.T) {
	tr := coverage.NewTracker()
	c := validCoverage()
	if err := tr.Set(c); err != nil {
		t.Fatalf("Set: %v", err)
	}
	got, ok := tr.Get(c.SourceID)
	if !ok {
		t.Fatal("Get: not found after Set")
	}
	if got.SourceID != c.SourceID || got.RecordsKnown != c.RecordsKnown {
		t.Errorf("Get returned unexpected value: %+v", got)
	}
}

func TestTrackerSetRejectsInvalid(t *testing.T) {
	tr := coverage.NewTracker()
	bad := coverage.SourceCoverage{} // empty SourceID
	if err := tr.Set(bad); err == nil {
		t.Fatal("Set with invalid coverage should return error")
	}
}

func TestTrackerGetMissing(t *testing.T) {
	tr := coverage.NewTracker()
	_, ok := tr.Get("nonexistent")
	if ok {
		t.Fatal("Get on empty tracker should return false")
	}
}

func TestTrackerAllSorted(t *testing.T) {
	tr := coverage.NewTracker()
	sources := []coverage.SourceCoverage{
		{SourceID: "src-c", RecordsKnown: 10, RecordsScanned: 10},
		{SourceID: "src-a", RecordsKnown: 5, RecordsScanned: 5},
		{SourceID: "src-b", RecordsKnown: 20, RecordsScanned: 20},
	}
	for _, s := range sources {
		if err := tr.Set(s); err != nil {
			t.Fatalf("Set(%s): %v", s.SourceID, err)
		}
	}
	all := tr.All()
	if len(all) != 3 {
		t.Fatalf("All: got %d entries, want 3", len(all))
	}
	wantOrder := []string{"src-a", "src-b", "src-c"}
	for i, want := range wantOrder {
		if all[i].SourceID != want {
			t.Errorf("All[%d].SourceID = %s, want %s", i, all[i].SourceID, want)
		}
	}
}

func TestTrackerAllEmpty(t *testing.T) {
	tr := coverage.NewTracker()
	if got := tr.All(); len(got) != 0 {
		t.Errorf("All on empty tracker: got %d entries, want 0", len(got))
	}
}

// ---------------------------------------------------------------------------
// Overall aggregation
// ---------------------------------------------------------------------------

func TestOverallAggregation(t *testing.T) {
	tr := coverage.NewTracker()
	sources := []coverage.SourceCoverage{
		{SourceID: "src-1", RecordsKnown: 100, RecordsScanned: 80, RecordsFailed: 5, RecordsPending: 15},
		{SourceID: "src-2", RecordsKnown: 200, RecordsScanned: 150, RecordsFailed: 10, RecordsPending: 40},
	}
	for _, s := range sources {
		if err := tr.Set(s); err != nil {
			t.Fatalf("Set: %v", err)
		}
	}
	o := tr.Overall()
	if o.Known != 300 {
		t.Errorf("Overall.Known: got %d, want 300", o.Known)
	}
	if o.Scanned != 230 {
		t.Errorf("Overall.Scanned: got %d, want 230", o.Scanned)
	}
	if o.Failed != 15 {
		t.Errorf("Overall.Failed: got %d, want 15", o.Failed)
	}
	if o.Pending != 55 {
		t.Errorf("Overall.Pending: got %d, want 55", o.Pending)
	}
	wantRatio := 230.0 / 300.0
	if o.Ratio != wantRatio {
		t.Errorf("Overall.Ratio: got %v, want %v", o.Ratio, wantRatio)
	}
	if o.Complete {
		t.Error("Overall.Complete: got true, want false (sources have pending/failed)")
	}
}

func TestOverallCompleteAllDone(t *testing.T) {
	tr := coverage.NewTracker()
	sources := []coverage.SourceCoverage{
		{SourceID: "src-a", RecordsKnown: 10, RecordsScanned: 10},
		{SourceID: "src-b", RecordsKnown: 20, RecordsScanned: 20},
	}
	for _, s := range sources {
		if err := tr.Set(s); err != nil {
			t.Fatalf("Set: %v", err)
		}
	}
	o := tr.Overall()
	if !o.Complete {
		t.Error("Overall.Complete: got false, want true (all sources complete)")
	}
	if o.Ratio != 1.0 {
		t.Errorf("Overall.Ratio: got %v, want 1.0", o.Ratio)
	}
}

func TestOverallEmptyTracker(t *testing.T) {
	tr := coverage.NewTracker()
	o := tr.Overall()
	if o.Known != 0 || o.Scanned != 0 || o.Ratio != 0 {
		t.Errorf("Overall on empty tracker: got %+v", o)
	}
	if o.Complete {
		t.Error("Overall.Complete on empty tracker: got true, want false")
	}
}

func TestOverallKnownZeroRatio(t *testing.T) {
	tr := coverage.NewTracker()
	if err := tr.Set(coverage.SourceCoverage{SourceID: "src-new", RecordsKnown: 0}); err != nil {
		t.Fatalf("Set: %v", err)
	}
	o := tr.Overall()
	if o.Ratio != 0 {
		t.Errorf("Overall.Ratio with Known=0: got %v, want 0", o.Ratio)
	}
}
