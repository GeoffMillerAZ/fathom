package main

import (
	"strings"
	"testing"
	"time"

	heartbeat "github.com/geoffmilleraz/agentic-kit/foundations/telemetry/heartbeat"
	"github.com/geoffmilleraz/fathom/internal/contextbench"
)

func mustParseTime(t *testing.T, s string) time.Time {
	t.Helper()
	tm, err := time.Parse(time.RFC3339, s)
	if err != nil {
		t.Fatalf("parse time %q: %v", s, err)
	}
	return tm
}

// TestPercentile_SmallN is critic Finding 5's own regression test: the
// prior floor-index formula (int(p*float64(n-1))) returned sorted[0] — the
// MINIMUM, not the 95th percentile — for any 2-element slice at p=0.95, a
// flattering bias in a credibility artifact. Nearest-rank
// (ceil(p*n)-1, clamped) must return the maximum here.
func TestPercentile_SmallN(t *testing.T) {
	two := []float64{10, 20}
	if got := percentile(two, 0.95); got != 20 {
		t.Errorf("percentile({10,20}, 0.95) = %v, want 20 (the prior floor-index bug returned 10, the minimum)", got)
	}
	if got := percentile(two, 0.50); got != 10 {
		t.Errorf("percentile({10,20}, 0.50) = %v, want 10 (nearest-rank: rank=ceil(0.5*2)=1, 1-indexed)", got)
	}

	one := []float64{42}
	if got := percentile(one, 0.50); got != 42 {
		t.Errorf("percentile({42}, 0.50) = %v, want 42", got)
	}
	if got := percentile(one, 0.95); got != 42 {
		t.Errorf("percentile({42}, 0.95) = %v, want 42", got)
	}

	ten := []float64{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}
	if got := percentile(ten, 0.50); got != 5 {
		t.Errorf("percentile(1..10, 0.50) = %v, want 5 (rank=ceil(5)=5, 1-indexed -> value 5)", got)
	}
	if got := percentile(ten, 0.95); got != 10 {
		t.Errorf("percentile(1..10, 0.95) = %v, want 10 (rank=ceil(9.5)=10, 1-indexed -> value 10)", got)
	}
}

// ─── buildResearchReport: the pure, deterministic core ─────────────────────

func TestBuildResearchReport_Golden(t *testing.T) {
	since := mustParseTime(t, "2026-07-21T00:00:00Z")
	until := mustParseTime(t, "2026-07-28T00:00:00Z")

	entries := []adoptionCallEntry{
		{TS: "2026-07-23T10:00:00Z", Tool: "find_symbol", Surface: "agent", Service: "fathom", LatencyMS: 10},
		{TS: "2026-07-23T10:01:00Z", Tool: "find_symbol", Surface: "agent", Service: "fathom", LatencyMS: 20},
		{TS: "2026-07-23T10:02:00Z", Tool: "staleness_report", Surface: "agent", Service: "fathom", LatencyMS: 5},
		{
			TS: "2026-07-23T10:03:00Z", Tool: "verify_plan", Surface: "", Service: "fathom", LatencyMS: 100,
			VerdictOutcome: map[string]int{"claimed": 3, "verified": 2, "failing": 1},
		},
		{TS: "2026-07-23T10:04:00Z", Tool: "verify_plan", Surface: "", Service: "other-service", LatencyMS: 50},
		// Outside the window entirely — must not be counted.
		{TS: "2026-07-01T00:00:00Z", Tool: "find_symbol", Surface: "agent", Service: "fathom", LatencyMS: 999},
	}

	beats := []heartbeat.Beat{
		{Channel: "edit-hook", Kind: "seen", At: mustParseTime(t, "2026-07-23T10:00:00Z")},
		{Channel: "edit-hook", Kind: "seen", At: mustParseTime(t, "2026-07-23T10:01:00Z")},
		{Channel: "edit-hook", Kind: "answered", At: mustParseTime(t, "2026-07-23T10:00:05Z"), Meta: map[string]string{"emitted": "true"}},
		{Channel: "edit-hook", Kind: "answered", At: mustParseTime(t, "2026-07-23T10:01:05Z"), Meta: map[string]string{"emitted": "false"}},
		// Outside window.
		{Channel: "edit-hook", Kind: "seen", At: mustParseTime(t, "2026-06-01T00:00:00Z")},
	}

	obsContent := observationsHeader + "\n" +
		formatObservationEntry(1, "confusing error", "find_symbol", "", "", "") +
		"\n" + formatObservationEntryWithStatus(2, "slow query", "query_symbols", "resolved")

	r := buildResearchReport(researchReportInputs{
		service: "fathom",
		since:   since,
		until:   until,
		now:     until,

		callLogPath: "workspace/logs/mcp-calls.jsonl",
		callLogRead: true,
		entries:     entries,

		hooksLogPath: "/home/x/.fathom/hooks.jsonl",
		hooksLogRead: true,
		beats:        beats,

		observationsPath: ".fathom/observations.md",
		obsRead:          true,
		obsContent:       obsContent,
	})

	// Service scoping: only "fathom"-service rows count (4 of the 6 fixture
	// rows — the "other-service" row and the out-of-window row are excluded).
	if r.TotalCallsInWindow != 4 {
		t.Errorf("TotalCallsInWindow = %d, want 4", r.TotalCallsInWindow)
	}

	if r.StalenessReportCalls != 1 {
		t.Errorf("StalenessReportCalls = %d, want 1", r.StalenessReportCalls)
	}
	if r.StalenessNote != "" {
		t.Errorf("StalenessNote = %q, want empty (log was read)", r.StalenessNote)
	}

	if r.VerifyClaimed != 3 || r.VerifyVerified != 2 || r.VerifyFailing != 1 {
		t.Errorf("verify totals = %d/%d/%d, want 3/2/1", r.VerifyClaimed, r.VerifyVerified, r.VerifyFailing)
	}
	if r.VerifyInstrumentedCalls != 1 {
		t.Errorf("VerifyInstrumentedCalls = %d, want 1", r.VerifyInstrumentedCalls)
	}

	// Injection coverage is per-user/unscoped by design — both beats in
	// range count regardless of --service.
	if r.InjectionSeen != 2 || r.InjectionAnswered != 2 {
		t.Errorf("injection seen/answered = %d/%d, want 2/2", r.InjectionSeen, r.InjectionAnswered)
	}
	if r.ObligationsServed != 1 {
		t.Errorf("ObligationsServed = %d, want 1 (only the emitted=true answer)", r.ObligationsServed)
	}
	if r.InjectionCoverageRate == nil || *r.InjectionCoverageRate != 1.0 {
		t.Errorf("InjectionCoverageRate = %v, want 1.0", r.InjectionCoverageRate)
	}
	if !strings.Contains(r.InjectionScopeNote, "per-user") {
		t.Errorf("InjectionScopeNote = %q, want it to say per-user", r.InjectionScopeNote)
	}

	// Honest not-measurable notes are ALWAYS present, non-empty.
	for name, note := range map[string]string{
		"ObligationsEvictedNote": r.ObligationsEvictedNote,
		"ReAnchorNote":           r.ReAnchorNote,
		"DiscoveryShareNote":     r.DiscoveryShareNote,
		"ObservationsWindowNote": r.ObservationsWindowNote,
	} {
		if !strings.HasPrefix(note, "not measurable") && !strings.Contains(note, "not windowed") {
			t.Errorf("%s = %q, want an honest not-measurable/not-windowed note", name, note)
		}
	}

	if r.TotalObservations == nil || *r.TotalObservations != 2 {
		t.Errorf("TotalObservations = %v, want 2", r.TotalObservations)
	}
	if r.OpenObservations == nil || *r.OpenObservations != 1 {
		t.Errorf("OpenObservations = %v, want 1 (one open, one resolved)", r.OpenObservations)
	}
}

// TestBuildResearchReport_CallLogUnreadable proves the honest
// absent-vs-zero distinction: an unreadable call log produces a Staleness
// note, not a silent zero, while injection coverage (a different source)
// remains fully computed.
func TestBuildResearchReport_CallLogUnreadable(t *testing.T) {
	since := mustParseTime(t, "2026-07-21T00:00:00Z")
	until := mustParseTime(t, "2026-07-28T00:00:00Z")

	r := buildResearchReport(researchReportInputs{
		since: since, until: until, now: until,
		callLogPath: "workspace/logs/mcp-calls.jsonl",
		callLogRead: false,
	})

	if r.CallLogRead {
		t.Error("CallLogRead = true, want false")
	}
	if r.TotalCallsInWindow != 0 {
		t.Errorf("TotalCallsInWindow = %d, want 0", r.TotalCallsInWindow)
	}
	if r.StalenessNote == "" || !strings.Contains(r.StalenessNote, "absent/unreadable") {
		t.Errorf("StalenessNote = %q, want it to say absent/unreadable", r.StalenessNote)
	}
	if r.LatencyNote == "" {
		t.Error("LatencyNote should be set when there are zero calls in window")
	}
}

// TestBuildResearchReport_BenchArms proves context-bench results feed
// through, sorted by arm name, with an honest note about the un-windowed
// comparison.
func TestBuildResearchReport_BenchArms(t *testing.T) {
	since := mustParseTime(t, "2026-07-21T00:00:00Z")
	until := mustParseTime(t, "2026-07-28T00:00:00Z")

	rr := &contextbench.RunResults{
		GeneratedAt: "2026-07-24T00:00:00Z",
		Summary: map[string]contextbench.Aggregate{
			"grep":   {Arm: "grep", TaskCount: 13, SuccessCount: 11, SuccessRate: 0.846, TotalTokens: 114400, MeanTokens: 8800},
			"fathom": {Arm: "fathom", TaskCount: 13, SuccessCount: 13, SuccessRate: 1.0, TotalTokens: 65000, MeanTokens: 5000},
		},
	}

	r := buildResearchReport(researchReportInputs{
		since: since, until: until, now: until,
		benchPath:    "workspace/artifacts/context-bench/results-1.json",
		benchResults: rr,
	})

	if len(r.BenchArms) != 2 {
		t.Fatalf("BenchArms = %+v, want 2 arms", r.BenchArms)
	}
	if r.BenchArms[0].Arm != "fathom" || r.BenchArms[1].Arm != "grep" {
		t.Errorf("BenchArms not sorted by name: %+v", r.BenchArms)
	}
	if !strings.Contains(r.BenchNote, "2026-07-24") {
		t.Errorf("BenchNote = %q, want it to mention the bench's own generated_at", r.BenchNote)
	}
}

// formatObservationEntryWithStatus is a test helper that writes an entry
// and then overwrites its Status line — observe.go's own formatter always
// writes "open", so tests that need a "resolved" entry patch it here rather
// than hand-writing the whole entry shape.
func formatObservationEntryWithStatus(id int, symptom, tool, status string) string {
	entry := formatObservationEntry(id, symptom, tool, "", "", "")
	return strings.Replace(entry, "- **Status:** open", "- **Status:** "+status, 1)
}
