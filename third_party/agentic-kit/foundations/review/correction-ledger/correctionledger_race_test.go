package correctionledger_test

// correctionledger_race_test.go — Concurrency safety tests.
// These tests must pass cleanly under `go test -race`. Before the sync.RWMutex
// was added to Ledger, running them with -race would report a data race on the
// events slice and knownIDs map.

import (
	"fmt"
	"sync"
	"testing"
	"time"

	cl "github.com/geoffmilleraz/agentic-kit/foundations/review/correction-ledger"
)

// TestLedger_ConcurrentOpenFinding verifies that concurrent calls to
// OpenFinding do not produce a data race on the internal events slice or
// knownIDs map. Each goroutine opens a unique finding so there are no
// duplicate-ID collisions; the test simply exercises the shared append and
// map-write paths simultaneously.
func TestLedger_ConcurrentOpenFinding(t *testing.T) {
	const goroutines = 50

	l := cl.New()
	var wg sync.WaitGroup
	wg.Add(goroutines)

	for i := range goroutines {
		i := i
		go func() {
			defer wg.Done()
			id := fmt.Sprintf("concurrent-open-%d", i)
			_, err := l.OpenFinding(id, "concurrent open test")
			if err != nil {
				// Duplicate-ID errors are impossible here because each i is unique.
				t.Errorf("goroutine %d: OpenFinding(%q): unexpected error: %v", i, id, err)
			}
		}()
	}
	wg.Wait()

	// All goroutines succeeded; verify the ledger event count matches.
	events := l.Events()
	if len(events) != goroutines {
		t.Errorf("expected %d events, got %d", goroutines, len(events))
	}
}

// TestLedger_ConcurrentOpenFindingAndRecordCorrection verifies that
// concurrent OpenFinding and RecordCorrection calls do not produce a data race.
// One goroutine pre-opens a finding; then many goroutines simultaneously record
// corrections against it while other goroutines open new findings.
func TestLedger_ConcurrentOpenFindingAndRecordCorrection(t *testing.T) {
	const (
		openers    = 20
		correctors = 20
	)

	l := cl.New()

	// Pre-open the finding that correctors will reference; must happen before
	// the goroutine fan-out to guarantee the finding exists.
	anchorID := "anchor-finding"
	if _, err := l.OpenFinding(anchorID, "anchor for corrections"); err != nil {
		t.Fatalf("pre-open anchor finding: %v", err)
	}

	var wg sync.WaitGroup
	wg.Add(openers + correctors)

	// Goroutines that open new findings.
	for i := range openers {
		i := i
		go func() {
			defer wg.Done()
			id := fmt.Sprintf("race-open-%d", i)
			_, err := l.OpenFinding(id, "opener goroutine")
			if err != nil {
				t.Errorf("opener %d: %v", i, err)
			}
		}()
	}

	// Goroutines that record corrections against the pre-opened anchor finding.
	for i := range correctors {
		i := i
		go func() {
			defer wg.Done()
			c := cl.Correction{
				FindingID:   anchorID,
				Target:      cl.CorrectionTarget{StableTarget: fmt.Sprintf("doc/s/%d.v1", i)},
				Original:    "old",
				Corrected:   "new",
				CorrectedBy: "tester",
				Reason:      "race test",
				Confidence:  3,
				CorrectedAt: time.Now().UTC(),
			}
			if err := l.RecordCorrection(c); err != nil {
				t.Errorf("corrector %d: %v", i, err)
			}
		}()
	}

	wg.Wait()

	// Sanity: total events = 1 (anchor open) + openers (opens) + correctors (corrections).
	wantEvents := 1 + openers + correctors
	events := l.Events()
	if len(events) != wantEvents {
		t.Errorf("expected %d events, got %d", wantEvents, len(events))
	}
}
