package main

// research_report_phasep_test.go — instruments A (payload cost), B
// (discovery share), and E (ident-cache hit rate) in `fathom research
// report` (docs/design/production-tokenomics.md).

import (
	"strings"
	"testing"
	"time"

	heartbeat "github.com/geoffmilleraz/agentic-kit/foundations/telemetry/heartbeat"
)

func mustTime(t *testing.T, s string) time.Time {
	t.Helper()
	tm, err := time.Parse(time.RFC3339, s)
	if err != nil {
		t.Fatal(err)
	}
	return tm
}

// ─── instrument A: what fathom SPENT ───────────────────────────────────────

func TestBuildResearchReport_PayloadCost_PerChannelTotals(t *testing.T) {
	since := mustTime(t, "2026-07-21T00:00:00Z")
	until := mustTime(t, "2026-07-28T00:00:00Z")
	beats := []heartbeat.Beat{
		{Channel: "payload-cost", Kind: "session-brief", At: mustTime(t, "2026-07-23T10:00:00Z"), Meta: map[string]string{"bytes": "400", "tokens_estimate": "100", "rules": "0"}},
		{Channel: "payload-cost", Kind: "post-edit", At: mustTime(t, "2026-07-23T10:05:00Z"), Meta: map[string]string{"bytes": "200", "tokens_estimate": "50", "rules": "2"}},
		{Channel: "payload-cost", Kind: "post-edit", At: mustTime(t, "2026-07-23T10:10:00Z"), Meta: map[string]string{"bytes": "80", "tokens_estimate": "20", "rules": "1"}},
		{Channel: "payload-cost", Kind: "pre-edit-guard", At: mustTime(t, "2026-07-23T10:15:00Z"), Meta: map[string]string{"bytes": "40", "tokens_estimate": "10", "rules": "1"}},
		{Channel: "payload-cost", Kind: "module-rules", At: mustTime(t, "2026-07-23T10:16:00Z"), Meta: map[string]string{"bytes": "60", "tokens_estimate": "15", "rules": "1"}},
		// out of window — must be excluded
		{Channel: "payload-cost", Kind: "post-edit", At: mustTime(t, "2026-08-01T00:00:00Z"), Meta: map[string]string{"bytes": "999", "tokens_estimate": "999"}},
	}
	r := buildResearchReport(researchReportInputs{since: since, until: until, now: until, beats: beats, hooksLogRead: true})

	if got := r.PayloadCostByChannel["session-brief"]; got.Beats != 1 || got.TotalBytes != 400 || got.TotalTokensEstimate != 100 {
		t.Errorf("session-brief = %+v", got)
	}
	postEdit := r.PayloadCostByChannel["post-edit"]
	if postEdit.Beats != 2 || postEdit.TotalBytes != 280 || postEdit.TotalTokensEstimate != 70 {
		t.Errorf("post-edit = %+v", postEdit)
	}
	if got := r.PayloadCostByChannel["pre-edit-guard"]; got.Beats != 1 || got.TotalTokensEstimate != 10 {
		t.Errorf("pre-edit-guard = %+v", got)
	}
	if got := r.PayloadCostByChannel["module-rules"]; got.Beats != 1 || got.TotalTokensEstimate != 15 {
		t.Errorf("module-rules = %+v", got)
	}
	if r.PayloadCostPerEditP50Estimate == nil || r.PayloadCostPerEditP95Estimate == nil {
		t.Fatal("expected per-edit p50/p95 to be computed from post-edit + pre-edit-guard beats")
	}
	if !strings.Contains(r.PayloadCostNote, "what fathom SPENT") {
		t.Errorf("PayloadCostNote = %q, want it to name the section", r.PayloadCostNote)
	}
	// law 12: the note must explicitly disclaim a saving, not merely omit
	// the word — "none of it implies a saving" is the required shape.
	if !strings.Contains(r.PayloadCostNote, "law 12") {
		t.Errorf("PayloadCostNote = %q — must cite law 12 (docs/design/production-tokenomics.md §2)", r.PayloadCostNote)
	}
}

func TestBuildResearchReport_PayloadCost_NeverFired_HonestNote(t *testing.T) {
	since := mustTime(t, "2026-07-21T00:00:00Z")
	until := mustTime(t, "2026-07-28T00:00:00Z")
	r := buildResearchReport(researchReportInputs{since: since, until: until, now: until})
	if len(r.PayloadCostByChannel) != 0 {
		t.Errorf("PayloadCostByChannel = %v, want empty", r.PayloadCostByChannel)
	}
	if !strings.HasPrefix(r.PayloadCostNote, "not measurable") {
		t.Errorf("PayloadCostNote = %q, want an honest not-measurable note", r.PayloadCostNote)
	}
}

// The report title MUST be exactly "what fathom SPENT" per the mission's
// literal requirement — pinned directly against the renderer's output.
func TestRenderResearchReportText_TitleIsExactlyWhatFathomSpent(t *testing.T) {
	var sb strings.Builder
	renderResearchReportText(&sb, ResearchReport{
		PayloadCostNote:    "not measurable: none",
		LatencyNote:        "not measurable: no calls in window",
		DiscoveryShareNote: "correlational placeholder",
		IdentCacheNote:     "not measurable: none",
	})
	if !strings.Contains(sb.String(), "\nwhat fathom SPENT:\n") {
		t.Errorf("output does not contain the exact section title:\n%s", sb.String())
	}
}

// ─── instrument B: discovery share ─────────────────────────────────────────

func TestBuildResearchReport_DiscoveryShare(t *testing.T) {
	since := mustTime(t, "2026-07-21T00:00:00Z")
	until := mustTime(t, "2026-07-28T00:00:00Z")
	entries := []adoptionCallEntry{
		{TS: "2026-07-23T10:00:00Z", Tool: "find_symbol", Surface: "agent"},
		{TS: "2026-07-23T10:01:00Z", Tool: "references", Surface: "agent"},
		{TS: "2026-07-23T10:02:00Z", Tool: "list_services", Surface: "agent"}, // plumbing — must NOT count
		{TS: "2026-07-23T10:03:00Z", Tool: "obligations", Surface: "agent"},   // governance — must NOT count
	}
	beats := []heartbeat.Beat{
		{Channel: "discovery", Kind: "grep", At: mustTime(t, "2026-07-23T10:04:00Z")},
		{Channel: "discovery", Kind: "grep", At: mustTime(t, "2026-07-23T10:05:00Z")},
		{Channel: "discovery", Kind: "glob", At: mustTime(t, "2026-07-23T10:06:00Z")},
		{Channel: "discovery", Kind: "read", At: mustTime(t, "2026-07-23T10:07:00Z")},
		{Channel: "discovery", Kind: "shell-search", At: mustTime(t, "2026-07-23T10:08:00Z")},
	}
	r := buildResearchReport(researchReportInputs{since: since, until: until, now: until, entries: entries, callLogRead: true, beats: beats, hooksLogRead: true})

	if r.DiscoveryIndexToolCalls != 2 {
		t.Fatalf("DiscoveryIndexToolCalls = %d, want 2 (find_symbol + references only)", r.DiscoveryIndexToolCalls)
	}
	if r.DiscoveryActionsTotal != 5 {
		t.Fatalf("DiscoveryActionsTotal = %d, want 5", r.DiscoveryActionsTotal)
	}
	if r.DiscoveryActionsByKind["read"] != 1 {
		t.Errorf("read must be its own bucket: %v", r.DiscoveryActionsByKind)
	}
	if r.DiscoveryActionsByKind["grep"] != 2 {
		t.Errorf("grep = %v, want 2", r.DiscoveryActionsByKind)
	}
	wantShare := 2.0 / 7.0 * 100
	if r.DiscoverySharePercent == nil || *r.DiscoverySharePercent < wantShare-0.01 || *r.DiscoverySharePercent > wantShare+0.01 {
		t.Errorf("DiscoverySharePercent = %v, want ~%.2f", r.DiscoverySharePercent, wantShare)
	}
}

func TestBuildResearchReport_DiscoveryShare_ReadNeverFoldedIntoGrep(t *testing.T) {
	since := mustTime(t, "2026-07-21T00:00:00Z")
	until := mustTime(t, "2026-07-28T00:00:00Z")
	beats := []heartbeat.Beat{
		{Channel: "discovery", Kind: "read", At: mustTime(t, "2026-07-23T10:00:00Z")},
	}
	r := buildResearchReport(researchReportInputs{since: since, until: until, now: until, beats: beats, hooksLogRead: true})
	if r.DiscoveryActionsByKind["grep"] != 0 {
		t.Errorf("read leaked into grep: %v", r.DiscoveryActionsByKind)
	}
	if r.DiscoveryActionsByKind["read"] != 1 {
		t.Errorf("read = %v, want 1 in its own bucket", r.DiscoveryActionsByKind)
	}
}

func TestBuildResearchReport_DiscoveryShareNote_AlwaysCorrelational(t *testing.T) {
	r := buildResearchReport(researchReportInputs{now: time.Now()})
	if !strings.Contains(r.DiscoveryShareNote, "CORRELATIONAL") {
		t.Errorf("DiscoveryShareNote = %q, want it to say CORRELATIONAL even with zero traffic", r.DiscoveryShareNote)
	}
}

// ─── instrument E: ident-cache hit rate ────────────────────────────────────

func TestBuildResearchReport_IdentCacheHitRate(t *testing.T) {
	since := mustTime(t, "2026-07-21T00:00:00Z")
	until := mustTime(t, "2026-07-28T00:00:00Z")
	beats := []heartbeat.Beat{
		{Channel: "ident-cache", Kind: "hit", At: mustTime(t, "2026-07-23T10:00:00Z")},
		{Channel: "ident-cache", Kind: "hit", At: mustTime(t, "2026-07-23T10:01:00Z")},
		{Channel: "ident-cache", Kind: "hit", At: mustTime(t, "2026-07-23T10:02:00Z")},
		{Channel: "ident-cache", Kind: "miss", At: mustTime(t, "2026-07-23T10:03:00Z")},
	}
	r := buildResearchReport(researchReportInputs{since: since, until: until, now: until, beats: beats, hooksLogRead: true})
	if r.IdentCacheHits != 3 || r.IdentCacheMisses != 1 {
		t.Fatalf("hits/misses = %d/%d, want 3/1", r.IdentCacheHits, r.IdentCacheMisses)
	}
	if r.IdentCacheHitRate == nil || *r.IdentCacheHitRate != 0.75 {
		t.Fatalf("IdentCacheHitRate = %v, want 0.75", r.IdentCacheHitRate)
	}
	if !strings.Contains(r.IdentCacheNote, "latency") {
		t.Errorf("IdentCacheNote = %q, want it labeled latency/load", r.IdentCacheNote)
	}
	if strings.Contains(strings.ToLower(r.IdentCacheNote), "token") == false {
		// note must explicitly disclaim being a token signal
		t.Errorf("IdentCacheNote = %q, want it to disclaim being a token figure", r.IdentCacheNote)
	}
}

func TestBuildResearchReport_IdentCache_NeverFired_HonestNote(t *testing.T) {
	r := buildResearchReport(researchReportInputs{now: time.Now()})
	if r.IdentCacheHitRate != nil {
		t.Errorf("IdentCacheHitRate = %v, want nil", r.IdentCacheHitRate)
	}
	if !strings.HasPrefix(r.IdentCacheNote, "not measurable") {
		t.Errorf("IdentCacheNote = %q, want an honest not-measurable note", r.IdentCacheNote)
	}
}
