package metrics

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	heartbeat "github.com/geoffmilleraz/agentic-kit/foundations/telemetry/heartbeat"
)

// writeArmStateForTest points FATHOM_ARM_STATE at a fresh OFF-arm ledger
// set at setAt with sessionsRemaining left, cleaned up automatically.
func writeArmStateForTest(t *testing.T, setAt time.Time, sessionsRemaining int) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "arm.json")
	raw, err := json.Marshal(map[string]any{
		"arm":                "off",
		"set_at":             setAt.UTC().Format(time.RFC3339),
		"sessions_remaining": sessionsRemaining,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, raw, 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("FATHOM_ARM_STATE", path)
}

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

// TestAbsenceNotesAreArmAware pins IT-2: a "not measurable" note whose
// cause could be an OFF measurement arm silencing hook channels must name
// the arm — never the misleading "guard hook is not installed" wording a
// genuinely absent hook gets — whenever an OFF arm actually covers the
// window. Without an arm on file, wording is unchanged.
func TestAbsenceNotesAreArmAware(t *testing.T) {
	since, until := channelsWindow()

	t.Run("with an OFF arm covering the window, notes name the arm", func(t *testing.T) {
		writeArmStateForTest(t, since.Add(time.Hour), 4)

		report := AggregateChannels(nil, since, until)
		if !strings.Contains(report.GuardCoverageNote, "OFF measurement arm active") {
			t.Errorf("GuardCoverageNote = %q, want it to name the arm, not a missing hook", report.GuardCoverageNote)
		}
		if strings.Contains(report.GuardCoverageNote, "guard hook is not installed") {
			t.Errorf("GuardCoverageNote still blames a missing hook under an active arm: %q", report.GuardCoverageNote)
		}
		if !strings.Contains(report.GuardCoverageNote, "4 session(s) remaining") {
			t.Errorf("GuardCoverageNote must name the session budget: %q", report.GuardCoverageNote)
		}
		if !strings.Contains(report.ModuleServingNote, "OFF measurement arm active") {
			t.Errorf("ModuleServingNote = %q, want it to name the arm", report.ModuleServingNote)
		}
	})

	t.Run("without an arm on file, wording is unchanged", func(t *testing.T) {
		t.Setenv("FATHOM_ARM_STATE", filepath.Join(t.TempDir(), "absent-arm.json"))

		report := AggregateChannels(nil, since, until)
		if !strings.Contains(report.GuardCoverageNote, "guard hook is not installed") {
			t.Errorf("GuardCoverageNote = %q, want the original missing-hook wording with no arm on file", report.GuardCoverageNote)
		}
		if strings.Contains(report.GuardCoverageNote, "OFF measurement arm") {
			t.Errorf("GuardCoverageNote must not claim an arm that isn't there: %q", report.GuardCoverageNote)
		}
	})
}
