package metrics

import (
	"testing"
	"time"

	heartbeat "github.com/geoffmilleraz/agentic-kit/foundations/telemetry/heartbeat"
)

func channelsWindow() (time.Time, time.Time) {
	return time.Unix(0, 0).UTC(), time.Unix(1<<32, 0).UTC()
}

func beat(channel, kind string, at time.Time, meta map[string]string) heartbeat.Beat {
	return heartbeat.Beat{Channel: channel, Kind: kind, At: at, Meta: meta}
}

func TestAggregateChannels_InjectionCoverage(t *testing.T) {
	since, until := channelsWindow()
	at := time.Unix(100, 0).UTC()
	beats := []heartbeat.Beat{
		beat("edit-hook", "seen", at, nil),
		beat("edit-hook", "seen", at, nil),
		beat("edit-hook", "answered", at, map[string]string{"emitted": "true"}),
	}
	report := AggregateChannels(beats, since, until)
	if report.InjectionSeen != 2 || report.InjectionAnswered != 1 || report.ObligationsServed != 1 {
		t.Fatalf("report = %+v, want seen=2 answered=1 served=1", report)
	}
	if report.InjectionCoverageRate == nil || *report.InjectionCoverageRate != 0.5 {
		t.Fatalf("InjectionCoverageRate = %v, want 0.5", report.InjectionCoverageRate)
	}
}

func TestAggregateChannels_GuardPreServed_DedupSuppressed(t *testing.T) {
	since, until := channelsWindow()
	at := time.Unix(100, 0).UTC()
	beats := []heartbeat.Beat{
		beat("edit-hook", "seen", at, map[string]string{"phase": "pre"}),
		beat("edit-hook", "answered", at, map[string]string{"phase": "pre", "emitted": "true"}),
		beat("edit-hook", "dedup-suppressed", at, map[string]string{"count": "3"}),
	}
	report := AggregateChannels(beats, since, until)
	if report.GuardPreSeen != 1 || report.GuardPreServed != 1 {
		t.Fatalf("report = %+v, want pre-seen=1 pre-served=1", report)
	}
	if report.GuardDedupSuppressed != 3 {
		t.Fatalf("GuardDedupSuppressed = %d, want 3", report.GuardDedupSuppressed)
	}
	// Pre-channel beats must never leak into the post/legacy counters.
	if report.InjectionSeen != 0 || report.InjectionAnswered != 0 {
		t.Errorf("post counters polluted by pre beats: seen=%d answered=%d", report.InjectionSeen, report.InjectionAnswered)
	}
}

func TestAggregateChannels_ModuleRulesServed(t *testing.T) {
	since, until := channelsWindow()
	at := time.Unix(100, 0).UTC()
	beats := []heartbeat.Beat{
		beat("edit-hook", "module-served", at, map[string]string{"module": "platform-standards", "rules": "4"}),
		beat("edit-hook", "module-served", at, map[string]string{"module": "platform-standards", "rules": "2"}),
	}
	report := AggregateChannels(beats, since, until)
	if report.ModuleRulesServed["platform-standards"] != 6 {
		t.Fatalf("ModuleRulesServed = %+v, want platform-standards=6", report.ModuleRulesServed)
	}
	if report.ModuleServingNote != "" {
		t.Errorf("ModuleServingNote = %q, want empty when a module served", report.ModuleServingNote)
	}
}

// TestAggregateChannels_NeverMeasurable_NotesVerbatim pins the exact
// phase-P note strings — carried through verbatim per design §6 rule 2.
func TestAggregateChannels_NeverMeasurable_NotesVerbatim(t *testing.T) {
	since, until := channelsWindow()
	report := AggregateChannels(nil, since, until)
	wantObligationsEvicted := "not measurable: no eviction event is logged anywhere in fathom today " +
		"(the edit-hook beat log records seen/answered/error only)"
	if report.ObligationsEvictedNote != wantObligationsEvicted {
		t.Errorf("ObligationsEvictedNote = %q, want verbatim %q", report.ObligationsEvictedNote, wantObligationsEvicted)
	}
	if report.ModuleServingNote == "" {
		t.Error("ModuleServingNote is empty on zero beats, want an honest note")
	}
	if report.GuardCoverageNote == "" {
		t.Error("GuardCoverageNote is empty on zero beats, want an honest note")
	}
	if report.InjectionCoverageNote == "" {
		t.Error("InjectionCoverageNote is empty on zero seen, want an honest zero-denominator note")
	}
}

func TestAggregateChannels_EmptyInput_HonestZeros(t *testing.T) {
	since, until := channelsWindow()
	report := AggregateChannels(nil, since, until)
	if report.InjectionSeen != 0 || report.InjectionAnswered != 0 || report.ObligationsServed != 0 {
		t.Fatalf("report = %+v, want all zero counts", report)
	}
	if report.InjectionCoverageRate != nil {
		t.Errorf("InjectionCoverageRate = %v, want nil on zero seen", report.InjectionCoverageRate)
	}
}

func TestAggregateChannels_WindowFiltersOutOfRangeBeats(t *testing.T) {
	since := time.Unix(100, 0).UTC()
	until := time.Unix(200, 0).UTC()
	beats := []heartbeat.Beat{
		beat("edit-hook", "seen", time.Unix(50, 0).UTC(), nil),  // before
		beat("edit-hook", "seen", time.Unix(150, 0).UTC(), nil), // in window
		beat("edit-hook", "seen", time.Unix(250, 0).UTC(), nil), // after
	}
	report := AggregateChannels(beats, since, until)
	if report.InjectionSeen != 1 {
		t.Fatalf("InjectionSeen = %d, want 1", report.InjectionSeen)
	}
}

func TestAggregateChannels_OtherChannelsIgnored(t *testing.T) {
	since, until := channelsWindow()
	at := time.Unix(100, 0).UTC()
	beats := []heartbeat.Beat{
		beat("payload-cost", "post-edit", at, nil),
		beat("discovery", "grep", at, nil),
	}
	report := AggregateChannels(beats, since, until)
	if report.InjectionSeen != 0 || report.ObligationsServed != 0 {
		t.Fatalf("report = %+v, want zero (non-edit-hook channels ignored)", report)
	}
}
