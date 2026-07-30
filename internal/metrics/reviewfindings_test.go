package metrics

// reviewfindings_test.go — O20's fixture-pinned regression test (docs/
// design/findings-as-evidence.md honesty rule 5: "no count ships without a
// test pinning it to a fixture of known size"). The fixture below has a
// KNOWN size: 50 review-call snapshots, each reporting 253 active findings
// (arch_conformance + spec_coverage combined) at a distinct, increasing
// timestamp. The pre-fix formula (sum every snapshot) would report
// 50*253=12650 — reproducing O20's exact failure class (a call-log window
// with many review runs inflates a state metric into an event total).
// LatestReviewActiveFindings must report exactly 253: the LAST snapshot's
// count, not the sum.

import (
	"testing"
	"time"
)

func mkSnapshot(t time.Time, arch, spec int) ReviewCallSnapshot {
	return ReviewCallSnapshot{At: t, Outcome: map[string]int{"arch_conformance": arch, "spec_coverage": spec}}
}

func TestLatestReviewActiveFindings_FixtureOfKnownSize_ReportsLatestSnapshotNotSum(t *testing.T) {
	base := time.Date(2026, 7, 23, 0, 0, 0, 0, time.UTC)
	var snapshots []ReviewCallSnapshot
	// 50 pushes over a week, each re-reporting the same ~253 active
	// findings (a realistic O20-shaped fixture: findings barely change
	// push to push, but the CALL COUNT does).
	for i := 0; i < 50; i++ {
		snapshots = append(snapshots, mkSnapshot(base.Add(time.Duration(i)*3*time.Hour), 200, 53))
	}
	count, sampleSize, ok := LatestReviewActiveFindings(snapshots)
	if !ok {
		t.Fatal("expected ok=true for a non-empty fixture")
	}
	if sampleSize != 50 {
		t.Errorf("sampleSize = %d, want 50 (the fixture's known review-call count)", sampleSize)
	}
	const want = 253 // 200 + 53, the LAST snapshot's own sum
	if count != want {
		t.Errorf("count = %d, want %d (the latest snapshot's active-finding count)", count, want)
	}
	const buggySum = 50 * 253
	if count == buggySum {
		t.Fatalf("count reproduces the pre-fix O20 bug (summed across all %d calls = %d) instead of reporting the latest snapshot", sampleSize, buggySum)
	}
}

func TestLatestReviewActiveFindings_LatestSnapshotWinsRegardlessOfInputOrder(t *testing.T) {
	early := mkSnapshot(time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC), 10, 5)
	late := mkSnapshot(time.Date(2026, 7, 20, 0, 0, 0, 0, time.UTC), 1, 1)
	// Deliberately out of chronological order in the slice — the function
	// must find the max timestamp, not trust positional order.
	count, sampleSize, ok := LatestReviewActiveFindings([]ReviewCallSnapshot{late, early})
	if !ok || sampleSize != 2 {
		t.Fatalf("ok=%v sampleSize=%d, want ok=true sampleSize=2", ok, sampleSize)
	}
	if count != 2 {
		t.Errorf("count = %d, want 2 (the chronologically later snapshot's sum, 1+1)", count)
	}
}

func TestLatestReviewActiveFindings_EmptyInput_NotOK(t *testing.T) {
	count, sampleSize, ok := LatestReviewActiveFindings(nil)
	if ok {
		t.Fatal("expected ok=false for empty input — a caller must render an honest note, never a silent zero")
	}
	if count != 0 || sampleSize != 0 {
		t.Errorf("count=%d sampleSize=%d, want 0,0 on the not-ok path", count, sampleSize)
	}
}

func TestLatestReviewActiveFindings_SingleCall_ReportsItsOwnSum(t *testing.T) {
	only := mkSnapshot(time.Date(2026, 7, 30, 0, 0, 0, 0, time.UTC), 3, 4)
	count, sampleSize, ok := LatestReviewActiveFindings([]ReviewCallSnapshot{only})
	if !ok || sampleSize != 1 || count != 7 {
		t.Errorf("got count=%d sampleSize=%d ok=%v, want 7,1,true", count, sampleSize, ok)
	}
}
