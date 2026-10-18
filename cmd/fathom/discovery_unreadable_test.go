package main

// An unreadable Bash command (RO-4) is a discovery beat of kind
// "unreadable": counted and named, never read as a discovery action, so
// no share's denominator treats a command fathom could not read as a
// known action (I38: unknown is neither a healthy zero nor a known value).

import (
	"strings"
	"testing"

	heartbeat "github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/telemetry/heartbeat"
)

func TestDiscoveryShareExcludesUnreadableCommands(t *testing.T) {
	since := mustTime(t, "2026-07-21T00:00:00Z")
	until := mustTime(t, "2026-07-28T00:00:00Z")
	at := mustTime(t, "2026-07-23T10:00:00Z")
	entries := []adoptionCallEntry{
		{TS: "2026-07-23T10:00:00Z", Tool: "find_symbol", Surface: "agent"},
		{TS: "2026-07-23T10:01:00Z", Tool: "references", Surface: "agent"},
	}

	t.Run("named in the kinds, out of the denominator", func(t *testing.T) {
		beats := []heartbeat.Beat{
			{Channel: "discovery", Kind: "shell-search", At: at},
			{Channel: "discovery", Kind: "shell-read", At: at},
			{Channel: "discovery", Kind: discoveryKindUnreadable, At: at},
		}
		r := buildResearchReport(researchReportInputs{since: since, until: until, now: until, beats: beats, hooksLogRead: true, entries: entries, callLogRead: true})
		if r.DiscoveryActionsTotal != 2 {
			t.Errorf("DiscoveryActionsTotal = %d, want 2 (the unreadable command is no known action)", r.DiscoveryActionsTotal)
		}
		if r.DiscoveryActionsByKind[discoveryKindUnreadable] != 1 {
			t.Errorf("DiscoveryActionsByKind = %v, want the unreadable command named once", r.DiscoveryActionsByKind)
		}
		if r.DiscoverySharePercent == nil || *r.DiscoverySharePercent != 50 {
			t.Fatalf("DiscoverySharePercent = %v, want 50 (2 index calls over 2 calls + 2 known actions)", r.DiscoverySharePercent)
		}
	})

	t.Run("only unreadable beats: not measurable, and the note names them", func(t *testing.T) {
		beats := []heartbeat.Beat{{Channel: "discovery", Kind: discoveryKindUnreadable, At: at}}
		r := buildResearchReport(researchReportInputs{since: since, until: until, now: until, beats: beats, hooksLogRead: true, entries: entries, callLogRead: true})
		if r.DiscoverySharePercent != nil {
			t.Fatalf("DiscoverySharePercent = %v, want nil: no known action, no denominator", *r.DiscoverySharePercent)
		}
		if !strings.Contains(r.DiscoveryShareDenominatorNote, "not measurable") || !strings.Contains(r.DiscoveryShareDenominatorNote, "1 unreadable") {
			t.Fatalf("DiscoveryShareDenominatorNote = %q, want not measurable naming the 1 unreadable command", r.DiscoveryShareDenominatorNote)
		}
	})

	t.Run("the session ledger counts known actions only", func(t *testing.T) {
		const sid = "s-unreadable"
		meta := map[string]string{"session": sid}
		beats := []heartbeat.Beat{
			{Channel: "discovery", Kind: "shell-search", At: at, Meta: meta},
			{Channel: "discovery", Kind: discoveryKindUnreadable, At: at, Meta: meta},
		}
		if _, n, _ := sumSessionBeats(beats, sid); n != 1 {
			t.Fatalf("discoveryActions = %d, want 1", n)
		}
	})
}
