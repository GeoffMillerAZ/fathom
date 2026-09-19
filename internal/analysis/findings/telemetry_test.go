package findings

import (
	"testing"
	"time"

	heartbeat "github.com/geoffmilleraz/agentic-kit/foundations/telemetry/heartbeat"

	"github.com/geoffmilleraz/fathom/internal/core"
)

func mkBeat(channel, kind string, at time.Time, meta map[string]string) heartbeat.Beat {
	return heartbeat.Beat{Channel: channel, Kind: kind, At: at, Meta: meta}
}

func TestRuleTelemetry_ServedCountsAnsweredEmittedBeatsMatchingScope(t *testing.T) {
	t0 := time.Date(2026, 9, 1, 12, 0, 0, 0, time.UTC)
	claims := []core.SemanticClaim{
		{ClaimID: "c1", Layer: "obligations", Kind: "obligation", Scope: []string{"internal/foo/**"}, Status: core.ClaimFresh},
	}
	beats := []heartbeat.Beat{
		mkBeat("edit-hook", "answered", t0, map[string]string{"emitted": "true", "file": "internal/foo/bar.go"}),
		mkBeat("edit-hook", "answered", t0.Add(time.Minute), map[string]string{"emitted": "false", "file": "internal/foo/bar.go"}),    // not emitted: ignored
		mkBeat("edit-hook", "answered", t0.Add(2*time.Minute), map[string]string{"emitted": "true", "file": "internal/other/baz.go"}), // out of scope: ignored
	}
	got := RuleTelemetry(claims, nil, beats)
	tel := got["c1"]
	if tel.Served != 1 {
		t.Errorf("Served = %d, want 1", tel.Served)
	}
	if !tel.LastFired.Equal(t0) {
		t.Errorf("LastFired = %v, want %v", tel.LastFired, t0)
	}
}

func TestRuleTelemetry_TouchedCountsFindingTouchedBeatsMatchingScope(t *testing.T) {
	t0 := time.Date(2026, 9, 1, 12, 0, 0, 0, time.UTC)
	claims := []core.SemanticClaim{
		{ClaimID: "f1", Layer: "findings", Kind: "finding", Scope: []string{"a.go"}, Status: core.ClaimFresh},
	}
	beats := []heartbeat.Beat{
		mkBeat("edit-hook", "finding_touched", t0, map[string]string{"file": "a.go", "shown": "1"}),
		mkBeat("edit-hook", "finding_touched", t0.Add(time.Hour), map[string]string{"file": "b.go", "shown": "1"}), // different file
	}
	got := RuleTelemetry(claims, nil, beats)
	tel := got["f1"]
	if tel.Touched != 1 {
		t.Errorf("Touched = %d, want 1", tel.Touched)
	}
}

func TestRuleTelemetry_DisputedFromRefuteEvents_ExactByClaimID(t *testing.T) {
	claims := []core.SemanticClaim{
		{ClaimID: "c1", Scope: []string{"a.go"}, Status: core.ClaimRefuted},
		{ClaimID: "c2", Scope: []string{"a.go"}, Status: core.ClaimFresh},
	}
	events := []core.ClaimEvent{
		{ClaimID: "c1", Verdict: "refuted"},
		{ClaimID: "c1", Verdict: "refuted"}, // a second refutation: counts again (append-only log)
		{ClaimID: "c2", Verdict: "corroborated"},
		{ClaimID: "unknown", Verdict: "refuted"}, // not in claims: ignored, never a panic
	}
	got := RuleTelemetry(claims, events, nil)
	if got["c1"].Disputed != 2 {
		t.Errorf("c1.Disputed = %d, want 2", got["c1"].Disputed)
	}
	if got["c2"].Disputed != 0 {
		t.Errorf("c2.Disputed = %d, want 0 (corroborate is not a dispute)", got["c2"].Disputed)
	}
}

func TestRuleTelemetry_EmptyBeats_HonestZeros(t *testing.T) {
	claims := []core.SemanticClaim{{ClaimID: "c1", Scope: []string{"a.go"}, Status: core.ClaimFresh}}
	got := RuleTelemetry(claims, nil, nil)
	tel, ok := got["c1"]
	if !ok {
		t.Fatal("c1 missing from telemetry map — every input claim must get an entry")
	}
	if tel.Served != 0 || tel.Touched != 0 || tel.Disputed != 0 || !tel.LastFired.IsZero() {
		t.Errorf("got %+v, want all-zero", tel)
	}
}

func TestSummarizeTelemetry_NeverFired30dAndTopFired(t *testing.T) {
	now := time.Date(2026, 9, 19, 0, 0, 0, 0, time.UTC)
	claims := []core.SemanticClaim{
		{ClaimID: "hot", Status: core.ClaimFresh},
		{ClaimID: "quiet", Status: core.ClaimFresh},
		{ClaimID: "stale-fire", Status: core.ClaimFresh},
		{ClaimID: "not-served", Status: core.ClaimSuperseded}, // excluded entirely
	}
	telemetry := map[string]Telemetry{
		"hot":        {Served: 5, Touched: 2, LastFired: now.Add(-time.Hour)},
		"stale-fire": {Served: 1, LastFired: now.Add(-40 * 24 * time.Hour)}, // fired, but >30d ago
		// "quiet" has no entry at all (zero value on lookup).
	}
	neverFired, top := summarizeTelemetry(claims, telemetry, now)
	if neverFired != 2 {
		t.Errorf("neverFired = %d, want 2 (quiet + stale-fire)", neverFired)
	}
	if len(top) != 2 {
		t.Fatalf("len(top) = %d, want 2 (hot, stale-fire — quiet never fired at all)", len(top))
	}
	if top[0].ClaimID != "hot" {
		t.Errorf("top[0] = %q, want %q (highest served+touched first)", top[0].ClaimID, "hot")
	}
}

func TestSummarizeTelemetry_TopFiredCap(t *testing.T) {
	now := time.Date(2026, 9, 19, 0, 0, 0, 0, time.UTC)
	var claims []core.SemanticClaim
	telemetry := map[string]Telemetry{}
	for i := 0; i < topFiredCap+5; i++ {
		id := "c" + string(rune('a'+i))
		claims = append(claims, core.SemanticClaim{ClaimID: id, Status: core.ClaimFresh})
		telemetry[id] = Telemetry{Served: 1, LastFired: now}
	}
	_, top := summarizeTelemetry(claims, telemetry, now)
	if len(top) != topFiredCap {
		t.Errorf("len(top) = %d, want cap %d", len(top), topFiredCap)
	}
}

func TestBuildStatusWithTelemetry_InjectedClock(t *testing.T) {
	now := time.Date(2026, 9, 19, 0, 0, 0, 0, time.UTC)
	claims := []core.SemanticClaim{
		{ClaimID: "c1", Layer: "findings", Kind: "finding", Scope: []string{"a.go"}, Status: core.ClaimFresh, Class: core.FindingClassSecurity},
	}
	beats := []heartbeat.Beat{
		mkBeat("edit-hook", "finding_touched", now.Add(-time.Hour), map[string]string{"file": "a.go"}),
	}
	r := BuildStatusWithTelemetry(claims, nil, beats, now)
	if r.RulesNeverFired30d != 0 {
		t.Errorf("RulesNeverFired30d = %d, want 0 (c1 fired within the window)", r.RulesNeverFired30d)
	}
	if len(r.TopFired) != 1 || r.TopFired[0].ClaimID != "c1" {
		t.Errorf("TopFired = %+v, want [c1]", r.TopFired)
	}
	// Base BuildStatus fields still populate identically.
	if r.DistinctClasses[0] != core.FindingClassSecurity {
		t.Errorf("DistinctClasses = %v, want [security]", r.DistinctClasses)
	}
}
