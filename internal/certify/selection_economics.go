package certify

// selection_economics.go — selectionTimeSaved (TQ-16, docs/design/
// test-intelligence.md §3 T4: "the estimated CI time saved from the last
// attested run durations, recorded for economics"). Pure over a
// caller-supplied durations map, exactly like SelectCertifyingFirst
// itself (selection.go) — the store read that would gather each test's
// own last attested run duration (a future extension of
// store.TestCaseRunsForSymbol/TestCaseRun, which does not yet carry
// duration_ms even though test_cases stores it — ADR-051 decision 1's
// join rule means this function is a pure view over that fact, never a
// new store) stays the caller's job.

import "sort"

// TimeSaved is selectionTimeSaved's answer, in milliseconds.
type TimeSaved struct {
	SelectedMs int64
	SkippedMs  int64
	TotalMs    int64
	// Percent is SkippedMs/TotalMs*100 — the share of CI time this
	// selection avoided running. 0 when TotalMs is 0 (nothing to save
	// against) rather than a division fabricating a rate.
	Percent float64
	// Missing names a selected or skipped test absent from durations
	// (never attested, or attested with no duration on record) — it
	// contributes 0 to both sums rather than a guessed average, and is
	// named here so the estimate's own coverage is honest.
	Missing []string
}

// selectionTimeSaved sums sel's Selected (sel.Tests) and Skipped test
// durations from durations (test key -> last attested run duration, ms).
func selectionTimeSaved(sel Selection, durations map[string]int64) TimeSaved {
	var out TimeSaved
	var missing []string
	for _, t := range sel.Tests {
		if d, ok := durations[t]; ok {
			out.SelectedMs += d
		} else {
			missing = append(missing, t)
		}
	}
	for _, t := range sel.Skipped {
		if d, ok := durations[t]; ok {
			out.SkippedMs += d
		} else {
			missing = append(missing, t)
		}
	}
	out.TotalMs = out.SelectedMs + out.SkippedMs
	if out.TotalMs > 0 {
		out.Percent = float64(out.SkippedMs) / float64(out.TotalMs) * 100
	}
	sort.Strings(missing)
	out.Missing = missing
	return out
}
