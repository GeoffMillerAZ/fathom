package harness

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

// preSeamClaudeHookTable is cmd/fathom/init.go's fathomHookSpecs as it
// stood before the seam (HEAD 3f0c3b1): event, matcher, marker, sub,
// timeout, in order. HP-1's proof: the registry entry reproduces it
// event for event. Do not edit this literal to make the test pass — a
// change here is a change to what every wired repository carries.
var preSeamClaudeHookTable = []struct {
	event, matcher, marker, sub string
	timeout                     int
}{
	{"SessionStart", "startup|resume|clear|compact", "fathom session-hook", "session-hook", 10},
	{"SessionEnd", "", "fathom session-end-hook", "session-end-hook", 10},
	{"PreCompact", "manual|auto", "fathom pre-compact-hook", "pre-compact-hook", 10},
	{"PostToolUse", "Edit|Write|MultiEdit", "fathom edit-hook", "edit-hook", 5},
	{"PreToolUse", "Edit|Write|MultiEdit", "fathom edit-hook --pre", "edit-hook --pre", 5},
	{"PreToolUse", "Bash", "fathom edit-hook --pre", "edit-hook --pre", 5},
	{"PreToolUse", "Grep|Glob|Read", "fathom edit-hook --pre", "edit-hook --pre", 5},
	{"Stop", "", "fathom stop-hook", "stop-hook", 10},
}

func TestLookupClaudeCodeReproducesHookTable(t *testing.T) {
	d, ok := Lookup(ClaudeCode)
	if !ok {
		t.Fatal("claude-code missing from the registry")
	}
	if len(d.Hooks) != len(preSeamClaudeHookTable) {
		t.Fatalf("claude-code bindings = %d, want %d", len(d.Hooks), len(preSeamClaudeHookTable))
	}
	for i, want := range preSeamClaudeHookTable {
		got := d.Hooks[i]
		if got.Harness != want.event || got.Matcher != want.matcher || got.Marker != want.marker || got.Verb != want.sub || got.TimeoutSec != want.timeout {
			t.Errorf("binding %d = {%s %q %q %q %d}, want {%s %q %q %q %d}", i,
				got.Harness, got.Matcher, got.Marker, got.Verb, got.TimeoutSec,
				want.event, want.matcher, want.marker, want.sub, want.timeout)
		}
	}
	if d.Unverified() {
		t.Error("claude-code must carry a real verified_against, not \"unverified\"")
	}
	if !d.HooksWired() {
		t.Error("claude-code must have its codec wired")
	}
}

func TestRegistryOrderAndLookup(t *testing.T) {
	want := []string{ClaudeCode, Codex, GeminiCLI, "cursor", "copilot", Agents}
	if got := IDs(); !reflect.DeepEqual(got, want) {
		t.Fatalf("IDs() = %v, want %v (registry order is priority, not alphabetical)", got, want)
	}
	if got := IDs(); !reflect.DeepEqual(got, IDs()) {
		t.Error("IDs() is not deterministic")
	}
	for _, id := range want {
		d, ok := Lookup(id)
		if !ok || d.ID != id {
			t.Errorf("Lookup(%q) = %+v, %v", id, d.ID, ok)
		}
	}
	if _, ok := Lookup("nope"); ok {
		t.Error("Lookup of an unknown id must fail")
	}
	if len(All()) != len(want) {
		t.Errorf("All() = %d descriptors, want %d", len(All()), len(want))
	}
}

// TestEveryDescriptorIsHonestByConstruction pins vision 25 §2.2 rule 5:
// every capability has a declared vehicle, no descriptor without a codec
// claims a hook vehicle, no descriptor without a transcript claims the
// session ledger, and only Claude Code is verified.
func TestEveryDescriptorIsHonestByConstruction(t *testing.T) {
	for _, d := range All() {
		for _, c := range Capabilities {
			if _, ok := d.Vehicles[c]; !ok {
				t.Errorf("%s: no vehicle declared for %s", d.ID, c)
			}
		}
		if len(d.Vehicles) != len(Capabilities) {
			t.Errorf("%s: %d vehicles, want exactly %d", d.ID, len(d.Vehicles), len(Capabilities))
		}
		if !d.HooksWired() {
			for c, v := range d.Vehicles {
				if isHookVehicle(v) {
					t.Errorf("%s: %s claims %s but no codec is wired", d.ID, c, v)
				}
			}
		}
		if d.Transcript == "" && d.Vehicles[CapSessionLedger] != VehicleAbsent {
			t.Errorf("%s: no transcript but session-ledger is %s (ADR-029 §6)", d.ID, d.Vehicles[CapSessionLedger])
		}
		if d.ID != ClaudeCode && !d.Unverified() {
			t.Errorf("%s: must ship unverified until a real session verifies it", d.ID)
		}
		if len(d.StanzaFiles()) == 0 {
			t.Errorf("%s: declares no stanza file", d.ID)
		}
		if d.MCP.AddCommand == "" || !strings.Contains(d.MCPAddLine("http://h:1"), "http://h:1") {
			t.Errorf("%s: MCP add line does not carry the url: %q", d.ID, d.MCPAddLine("http://h:1"))
		}
		if d.Codec != "" && d.CodecLanded == "" {
			t.Errorf("%s: codec %q has no landing date — the demotion clock cannot start", d.ID, d.Codec)
		}
	}
}

func TestDetect(t *testing.T) {
	touch := func(t *testing.T, path string) {
		t.Helper()
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte("{}"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	t.Run("nothing", func(t *testing.T) {
		if got := Detect(t.TempDir(), t.TempDir()); got != nil {
			t.Errorf("Detect on empty dirs = %v, want nil", got)
		}
	})
	t.Run("claude from the project", func(t *testing.T) {
		root := t.TempDir()
		touch(t, filepath.Join(root, ".claude", "settings.json"))
		if got := Detect(root, t.TempDir()); !reflect.DeepEqual(got, []string{ClaudeCode}) {
			t.Errorf("got %v", got)
		}
	})
	t.Run("codex from home", func(t *testing.T) {
		home := t.TempDir()
		touch(t, filepath.Join(home, ".codex", "config.toml"))
		if got := Detect(t.TempDir(), home); !reflect.DeepEqual(got, []string{Codex}) {
			t.Errorf("got %v", got)
		}
	})
	t.Run("gemini from the project", func(t *testing.T) {
		root := t.TempDir()
		touch(t, filepath.Join(root, ".gemini", "settings.json"))
		if got := Detect(root, t.TempDir()); !reflect.DeepEqual(got, []string{GeminiCLI}) {
			t.Errorf("got %v", got)
		}
	})
	t.Run("several, in registry order", func(t *testing.T) {
		root, home := t.TempDir(), t.TempDir()
		touch(t, filepath.Join(root, ".gemini", "settings.json"))
		touch(t, filepath.Join(root, ".codex", "config.toml"))
		touch(t, filepath.Join(home, ".claude.json"))
		want := []string{ClaudeCode, Codex, GeminiCLI}
		if got := Detect(root, home); !reflect.DeepEqual(got, want) {
			t.Errorf("got %v, want %v", got, want)
		}
	})
	t.Run("agents is never detected", func(t *testing.T) {
		root := t.TempDir()
		touch(t, filepath.Join(root, "AGENTS.md"))
		if got := Detect(root, t.TempDir()); got != nil {
			t.Errorf("got %v, want nil", got)
		}
	})
	t.Run("empty home never matches a home marker", func(t *testing.T) {
		if got := Detect(t.TempDir(), ""); got != nil {
			t.Errorf("got %v", got)
		}
	})
}

func TestClassifyTool(t *testing.T) {
	cases := []struct {
		id, tool string
		want     ToolClass
	}{
		{ClaudeCode, "Edit", ToolFileEdit},
		{ClaudeCode, "Write", ToolFileEdit},
		{ClaudeCode, "MultiEdit", ToolFileEdit},
		{ClaudeCode, "Bash", ToolShell},
		{ClaudeCode, "Read", ToolDiscoveryRead},
		{ClaudeCode, "Grep", ToolDiscoveryGrep},
		{ClaudeCode, "Glob", ToolDiscoveryGlob},
		{ClaudeCode, "NotebookEdit", ToolOther},
		{ClaudeCode, "", ToolOther},
		{Codex, "apply_patch", ToolFileEdit},
		{Codex, "Bash", ToolShell},
		{Codex, "Read", ToolOther},
		{GeminiCLI, "write_file", ToolFileEdit},
		{GeminiCLI, "replace", ToolFileEdit},
		{Agents, "Edit", ToolOther},
		{"nope", "Edit", ToolOther},
	}
	for _, tc := range cases {
		if got := ClassifyTool(tc.id, tc.tool); got != tc.want {
			t.Errorf("ClassifyTool(%q, %q) = %s, want %s", tc.id, tc.tool, got, tc.want)
		}
	}
}

func TestDemotionDue(t *testing.T) {
	landed := time.Date(2026, 9, 2, 0, 0, 0, 0, time.UTC)
	unverified := Descriptor{ID: "x", Codec: "x", CodecLanded: "2026-09-02", VerifiedAgainst: "unverified", Tier: "A"}

	if due, _ := DemotionDue(unverified, landed.Add(30*24*time.Hour)); due {
		t.Error("30 days in: not due")
	}
	if due, _ := DemotionDue(unverified, landed.Add(60*24*time.Hour)); due {
		t.Error("exactly 60 days: not yet due")
	}
	due, reason := DemotionDue(unverified, landed.Add(61*24*time.Hour))
	if !due || !strings.Contains(reason, "tier D") || !strings.Contains(reason, "61 days") {
		t.Errorf("61 days: due=%v reason=%q", due, reason)
	}
	if EffectiveTier(unverified, landed.Add(61*24*time.Hour)) != "D" || EffectiveTier(unverified, landed) != "A" {
		t.Error("EffectiveTier must follow the clock")
	}

	verified := unverified
	verified.VerifiedAgainst = "a real session"
	if due, _ := DemotionDue(verified, landed.Add(400*24*time.Hour)); due {
		t.Error("a verified descriptor never demotes")
	}
	dataOnly := unverified
	dataOnly.Codec, dataOnly.CodecLanded = "", ""
	if due, _ := DemotionDue(dataOnly, landed.Add(400*24*time.Hour)); due {
		t.Error("a data-only descriptor has no clock")
	}
	broken := unverified
	broken.CodecLanded = "yesterday"
	if due, reason := DemotionDue(broken, landed); !due || !strings.Contains(reason, "unparsable") {
		t.Errorf("an unreadable clock must be due: %v %q", due, reason)
	}

	// The shipped registry: nothing is due on the landing day (the wave-2
	// codecs landed 2026-09-02), and Claude Code can never be.
	for _, d := range All() {
		if due, reason := DemotionDue(d, landed); due {
			t.Errorf("%s demoted at founding: %s", d.ID, reason)
		}
	}
}

func TestCapabilityRowsAndTable(t *testing.T) {
	now := time.Date(2026, 9, 2, 0, 0, 0, 0, time.UTC)

	t.Run("unverified descriptor: every row says so", func(t *testing.T) {
		for _, id := range []string{Codex, GeminiCLI, "cursor", "copilot", Agents} {
			d, _ := Lookup(id)
			table := RenderCapabilityTable(id, now)
			lines := strings.Split(strings.TrimSpace(table), "\n")
			if len(lines) != 2+len(Capabilities) {
				t.Fatalf("%s: %d lines, want header+separator+%d rows:\n%s", id, len(lines), len(Capabilities), table)
			}
			for _, line := range lines[2:] {
				if !strings.Contains(line, "unverified on this machine") {
					t.Errorf("%s row lacks the honesty marker: %s", id, line)
				}
				if !d.HooksWired() && strings.Contains(line, "hook-") {
					t.Errorf("%s: a hook vehicle without a codec: %s", id, line)
				}
				if d.HooksWired() && strings.Contains(line, "not wired") {
					t.Errorf("%s: a wired codec labelled not wired: %s", id, line)
				}
			}
		}
	})
	t.Run("claude-code: verified, no unverified row", func(t *testing.T) {
		table := RenderCapabilityTable(ClaudeCode, now)
		if strings.Contains(table, "unverified") {
			t.Errorf("claude-code table carries an unverified row:\n%s", table)
		}
		if !strings.Contains(table, "| `claim-gate` | `absent` |") {
			t.Errorf("an absent vehicle must print as absent:\n%s", table)
		}
	})
	t.Run("unknown id renders a line, never silence", func(t *testing.T) {
		if got := RenderCapabilityTable("nope", now); !strings.Contains(got, "not in the registry") {
			t.Errorf("got %q", got)
		}
	})
	t.Run("demoted descriptor drops its hook vehicles", func(t *testing.T) {
		d := Descriptor{
			ID: "x", Codec: "x", CodecLanded: "2026-01-01", VerifiedAgainst: "unverified", Tier: "A",
			Vehicles: map[Capability]Vehicle{CapPrime: VehicleHookInject, CapStanza: VehicleStanza},
		}
		rows := CapabilityRows(d, now)
		if rows[0].Capability != CapPrime || rows[0].Vehicle != VehicleAbsent || !strings.Contains(rows[0].Status, "tier D") {
			t.Errorf("prime row after demotion = %+v", rows[0])
		}
		var stanza CapabilityRow
		for _, r := range rows {
			if r.Capability == CapStanza {
				stanza = r
			}
		}
		if stanza.Vehicle != VehicleStanza {
			t.Errorf("stanza survives demotion, got %+v", stanza)
		}
	})
	t.Run("hook vehicle without a codec is labelled not wired", func(t *testing.T) {
		d := Descriptor{
			ID: "x", VerifiedAgainst: "unverified", Tier: "A",
			Vehicles: map[Capability]Vehicle{CapPrime: VehicleHookInject},
		}
		rows := CapabilityRows(d, now)
		if !strings.Contains(rows[0].Status, "hooks: not wired") {
			t.Errorf("got %+v", rows[0])
		}
	})
	t.Run("deterministic", func(t *testing.T) {
		if RenderCapabilityTable(Codex, now) != RenderCapabilityTable(Codex, now) {
			t.Error("RenderCapabilityTable is not deterministic")
		}
	})
}

func TestExportJSONIsDeterministic(t *testing.T) {
	a, err := ExportJSON()
	if err != nil {
		t.Fatal(err)
	}
	b, _ := ExportJSON()
	if string(a) != string(b) {
		t.Error("ExportJSON is not deterministic")
	}
	for _, id := range IDs() {
		if !strings.Contains(string(a), `"id": "`+id+`"`) {
			t.Errorf("export lacks %s", id)
		}
	}
}
