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

// postSeamClaudeHookAdditions are the rows vision 26 §4 added AFTER the
// pre-seam eight (PC-23 InstructionsLoaded, PC-24 ConfigChange), the one
// deliberate change to the Claude Code table since the seam. They are
// appended, never interleaved, so the pre-seam prefix stays byte-identical.
var postSeamClaudeHookAdditions = []struct {
	event, matcher, marker, sub string
	timeout                     int
}{
	{"InstructionsLoaded", "", "fathom session-hook --event instructions-loaded", "session-hook --event instructions-loaded", 5},
	{"ConfigChange", "", "fathom config audit --hook", "config audit --hook", 10},
}

// waveThreeClaudeHookAdditions are HP-26..HP-31, appended after the
// vision 26 §4 pair for the same reason that pair was appended after the
// pre-seam eight: a repo wired before this change gains exactly these
// seven entries on its next init/reconcile and loses nothing.
//
// DELIBERATE GOLDEN CHANGE (2026-09-12): this table and
// cmd/fathom/hookspecs_test.go's twin were updated together, because the
// wiring an adopter carries genuinely changed. Nothing above this literal
// moved a byte, and TestLookupClaudeCodeReproducesHookTable still pins the
// pre-seam prefix to the byte.
var waveThreeClaudeHookAdditions = []struct {
	event, matcher, marker, sub string
	timeout                     int
}{
	{"SubagentStart", "", "fathom session-hook --event subagent-start", "session-hook --event subagent-start", 5},
	{"PostToolUseFailure", `mcp__fathom__.*|mcp__plugin_[^_]*_fathom__.*`, "fathom session-hook --event tool-failure", "session-hook --event tool-failure", 5},
	{"FileChanged", "CLAUDE.md|AGENTS.md|concerns.cue|config.toml", "fathom config audit --hook --event file-changed", "config audit --hook --event file-changed", 10},
	{"CwdChanged", "", "fathom session-hook --event cwd-changed", "session-hook --event cwd-changed", 5},
	{"DirectoryAdded", "", "fathom session-hook --event cwd-changed", "session-hook --event cwd-changed", 5},
	{"Setup", "", "fathom doctor --brief", "doctor --brief", 15},
}

// promptHookClaudeHookAddition is O57 items 4/5's two rows, appended after
// waveThreeClaudeHookAdditions for the same reason that group was
// appended after the vision 26 §4 pair: nothing above it moved a byte.
var promptHookClaudeHookAddition = []struct {
	event, matcher, marker, sub string
	timeout                     int
}{
	{"UserPromptSubmit", "", "fathom prompt-hook", "prompt-hook", 10},
	{"PostToolUse", "Bash", "fathom edit-hook", "edit-hook", 5},
}

// testRunClaudeHookAddition is TR-2's two rows, appended after the O57
// pair (docs/plans/test-run-hook.md): the test-run hook on PostToolUse(Bash)
// and PostToolUseFailure(Bash). DELIBERATE GOLDEN CHANGE (2026-09-29):
// cmd/fathom/hookspecs_test.go's testRunHookSpecAddition is the twin.
var testRunClaudeHookAddition = []struct {
	event, matcher, marker, sub string
	timeout                     int
}{
	{"PostToolUse", "Bash", "fathom session-hook --event test-run", "session-hook --event test-run", 5},
	{"PostToolUseFailure", "Bash", "fathom session-hook --event failed-test-run", "session-hook --event failed-test-run", 5},
}

// TestClaudeHookTableTestRunAddition pins TR-2's rows: after the O57 pair,
// both inject (Claude Code documents additionalContext on PostToolUse and
// PostToolUseFailure), both the test-run capability, the failure row reads
// the `error` key TR-V1 found (not `tool_error`), and neither marker is a
// substring of the other or of any marker already in its matcher group.
func TestClaudeHookTableTestRunAddition(t *testing.T) {
	d, _ := Lookup(ClaudeCode)
	base := len(preSeamClaudeHookTable) + len(postSeamClaudeHookAdditions) + len(waveThreeClaudeHookAdditions) + len(promptHookClaudeHookAddition)
	for i, want := range testRunClaudeHookAddition {
		got := d.Hooks[base+i]
		if got.Harness != want.event || got.Matcher != want.matcher || got.Marker != want.marker || got.Verb != want.sub || got.TimeoutSec != want.timeout {
			t.Errorf("addition %d = {%s %q %q %q %d}, want {%s %q %q %q %d}", i,
				got.Harness, got.Matcher, got.Marker, got.Verb, got.TimeoutSec,
				want.event, want.matcher, want.marker, want.sub, want.timeout)
		}
		if got.Vehicle != VehicleHookInject || got.Capability != CapTestRunLine {
			t.Errorf("%s: vehicle %s capability %s, want hook-inject and %s", got.Harness, got.Vehicle, got.Capability, CapTestRunLine)
		}
		for _, other := range d.Hooks {
			if other.Harness == got.Harness && other.Matcher == got.Matcher && other.Marker != got.Marker &&
				(strings.Contains(other.Marker, got.Marker) || strings.Contains(got.Marker, other.Marker)) {
				t.Errorf("marker %q and %q share a matcher group and one contains the other", got.Marker, other.Marker)
			}
		}
	}
	if got := d.Hooks[base+1]; got.Input["error"] != "error" || got.Event != EventToolFailure {
		t.Errorf("failed-test-run row = %+v, want the tool-failure event reading `error`", got)
	}
	if got := d.Hooks[base]; got.Input["tool_response.stdout"] != "tool_response.stdout" || got.Event != EventPostTool {
		t.Errorf("test-run row = %+v, want the post-tool event reading tool_response.stdout", got)
	}
	if b, ok := d.Binding(EventToolFailure); !ok || b.Capability != CapToolFailureAdvice || b.Input["error"] != "error" {
		t.Errorf("HP-27's row must stay the event's first binding and read `error`: %+v", b)
	}
	if d.Vehicles[CapTestRunLine] != VehicleHookInject {
		t.Errorf("claude-code test-run-line = %s", d.Vehicles[CapTestRunLine])
	}
}

func TestLookupClaudeCodeReproducesHookTable(t *testing.T) {
	d, ok := Lookup(ClaudeCode)
	if !ok {
		t.Fatal("claude-code missing from the registry")
	}
	want := len(preSeamClaudeHookTable) + len(postSeamClaudeHookAdditions) + len(waveThreeClaudeHookAdditions) + len(promptHookClaudeHookAddition) + len(testRunClaudeHookAddition)
	if len(d.Hooks) != want {
		t.Fatalf("claude-code bindings = %d, want %d (%d pre-seam + %d vision 26 §4 + %d wave 3 + %d O57 + %d TR-2)", len(d.Hooks), want,
			len(preSeamClaudeHookTable), len(postSeamClaudeHookAdditions), len(waveThreeClaudeHookAdditions), len(promptHookClaudeHookAddition), len(testRunClaudeHookAddition))
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

// TestClaudeHookTableAdditions pins the two vision 26 §4 rows to their
// position (after the pre-seam eight) and their contract: no matcher on
// either (InstructionsLoaded matches on load_reason, which fathom records
// rather than filters; ConfigChange matches on source, likewise), both
// silent vehicles, and the two new events decode a top-level file_path.
func TestClaudeHookTableAdditions(t *testing.T) {
	d, _ := Lookup(ClaudeCode)
	base := len(preSeamClaudeHookTable)
	for i, want := range postSeamClaudeHookAdditions {
		got := d.Hooks[base+i]
		if got.Harness != want.event || got.Matcher != want.matcher || got.Marker != want.marker || got.Verb != want.sub || got.TimeoutSec != want.timeout {
			t.Errorf("addition %d = {%s %q %q %q %d}, want {%s %q %q %q %d}", i,
				got.Harness, got.Matcher, got.Marker, got.Verb, got.TimeoutSec,
				want.event, want.matcher, want.marker, want.sub, want.timeout)
		}
		if got.Vehicle != VehicleHookSilent {
			t.Errorf("%s: vehicle = %s, want hook-silent (neither event can inject)", got.Harness, got.Vehicle)
		}
		if len(got.Output) != 0 {
			t.Errorf("%s: declares output %v but the harness discards it", got.Harness, got.Output)
		}
	}
	if b, ok := d.Binding(EventInstructionsLoaded); !ok || b.Capability != CapStanzaLoaded || b.Input["file_path"] != "file_path" || b.Input["memory_type"] != "memory_type" || b.Input["load_reason"] != "load_reason" {
		t.Errorf("instructions-loaded binding = %+v", b)
	}
	if b, ok := d.Binding(EventConfigChange); !ok || b.Capability != CapConfigAudit || b.Input["source"] != "source" || b.Input["file_path"] != "file_path" {
		t.Errorf("config-change binding = %+v", b)
	}
	if d.Vehicles[CapClaimGate] != VehicleHookInject {
		t.Errorf("claim-gate = %s, want hook-inject (PC-22 rides the PostToolUse edit hook)", d.Vehicles[CapClaimGate])
	}
	codec := ClaudeCodec()
	in, err := codec.DecodeInput(EventInstructionsLoaded, []byte(`{"session_id":"s1","cwd":"/r","hook_event_name":"InstructionsLoaded","file_path":"/r/CLAUDE.md","memory_type":"Project","load_reason":"session_start"}`))
	if err != nil || in.FilePath != "/r/CLAUDE.md" || in.MemoryType != "Project" || in.LoadReason != "session_start" || in.SessionID != "s1" {
		t.Errorf("InstructionsLoaded decode = %+v, %v", in, err)
	}
	in, err = codec.DecodeInput(EventConfigChange, []byte(`{"session_id":"s1","cwd":"/r","hook_event_name":"ConfigChange","source":"project_settings","file_path":"/r/.claude/settings.json"}`))
	if err != nil || in.FilePath != "/r/.claude/settings.json" || in.Source != "project_settings" {
		t.Errorf("ConfigChange decode = %+v, %v", in, err)
	}
	// A tool event never reads the top-level file_path (undocumented there).
	in, _ = codec.DecodeInput(EventPostTool, []byte(`{"tool_name":"Edit","file_path":"/r/top.md","tool_input":{"file_path":"/r/tool.md"}}`))
	if in.FilePath != "/r/tool.md" {
		t.Errorf("PostToolUse file_path = %q, want tool_input.file_path", in.FilePath)
	}
}

// TestClaudeHookTablePromptAddition pins O57 items 4/5's one row: appended
// after wave 3, no matcher (UserPromptSubmit fires on every prompt),
// inject vehicle (Claude Code documents additionalContext on this event —
// claudeContextEvents), and the prompt field actually decodes.
func TestClaudeHookTablePromptAddition(t *testing.T) {
	d, _ := Lookup(ClaudeCode)
	base := len(preSeamClaudeHookTable) + len(postSeamClaudeHookAdditions) + len(waveThreeClaudeHookAdditions)
	for i, want := range promptHookClaudeHookAddition {
		got := d.Hooks[base+i]
		if got.Harness != want.event || got.Matcher != want.matcher || got.Marker != want.marker || got.Verb != want.sub || got.TimeoutSec != want.timeout {
			t.Errorf("addition %d = {%s %q %q %q %d}, want {%s %q %q %q %d}", i,
				got.Harness, got.Matcher, got.Marker, got.Verb, got.TimeoutSec,
				want.event, want.matcher, want.marker, want.sub, want.timeout)
		}
		if got.Vehicle != VehicleHookInject {
			t.Errorf("%s: vehicle = %s, want hook-inject (the harness documents additionalContext here)", got.Harness, got.Vehicle)
		}
	}
	b, ok := d.Binding(EventUserPromptSubmit)
	if !ok || b.Capability != CapPromptObligations || b.Input["prompt"] != "prompt" {
		t.Fatalf("user-prompt-submit binding = %+v, %v", b, ok)
	}
	codec := ClaudeCodec()
	in, err := codec.DecodeInput(EventUserPromptSubmit, []byte(`{"session_id":"s1","cwd":"/r","hook_event_name":"UserPromptSubmit","prompt":"refactor internal/store/backend.go"}`))
	if err != nil || in.Prompt != "refactor internal/store/backend.go" || in.SessionID != "s1" {
		t.Errorf("UserPromptSubmit decode = %+v, %v", in, err)
	}
	// The catalog row this addition wires must actually say so — a
	// forgotten catalog flip is exactly the shape TestEventCatalogMatchesBindings
	// exists to catch, but the row's own text (the ONE place an adopter
	// reads why) is worth pinning directly too.
	found := false
	for _, e := range claudeEventCatalog {
		if e.Event == "UserPromptSubmit" {
			found = true
			if !e.Wired {
				t.Error("UserPromptSubmit catalog row must say Wired: true now that a binding exists")
			}
		}
	}
	if !found {
		t.Error("UserPromptSubmit missing from claudeEventCatalog entirely")
	}
}

// TestClaudeHookTableBashWriteAddition pins O57 item 4/5(b)'s row: a
// SECOND PostToolUse matcher group (Bash), the same edit-hook verb and
// marker the Edit|Write|MultiEdit group already carries (matcher groups
// share a marker exactly like PreToolUse's three already do), inject
// vehicle, and the tool_input.command field actually decoding.
func TestClaudeHookTableBashWriteAddition(t *testing.T) {
	d, _ := Lookup(ClaudeCode)
	base := len(preSeamClaudeHookTable) + len(postSeamClaudeHookAdditions) + len(waveThreeClaudeHookAdditions) + 1 // + the prompt-hook row above it
	want := promptHookClaudeHookAddition[1]
	got := d.Hooks[base]
	if got.Harness != want.event || got.Matcher != want.matcher || got.Marker != want.marker || got.Verb != want.sub || got.TimeoutSec != want.timeout {
		t.Errorf("addition = {%s %q %q %q %d}, want {%s %q %q %q %d}",
			got.Harness, got.Matcher, got.Marker, got.Verb, got.TimeoutSec,
			want.event, want.matcher, want.marker, want.sub, want.timeout)
	}
	if got.Vehicle != VehicleHookInject {
		t.Errorf("vehicle = %s, want hook-inject", got.Vehicle)
	}
	if got.Capability != CapObligationsPostShell {
		t.Errorf("capability = %s, want %s", got.Capability, CapObligationsPostShell)
	}
	codec := ClaudeCodec()
	in, err := codec.DecodeInput(EventPostTool, []byte(`{"tool_name":"Bash","tool_input":{"command":"echo hi > internal/store/backend.go"}}`))
	if err != nil || in.Command != "echo hi > internal/store/backend.go" || in.Tool != ToolShell {
		t.Errorf("Bash PostToolUse decode = %+v, %v", in, err)
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
		// A descriptor may only be verified by a real session on this
		// machine. Claude Code and (since HP-34) Codex are; everything else
		// must still ship unverified, because "unverified" is the default
		// that keeps an unmeasured claim out of every generated table.
		if d.ID != ClaudeCode && d.ID != Codex && !d.Unverified() {
			t.Errorf("%s: must ship unverified until a real session verifies it", d.ID)
		}
		if !d.Unverified() && !strings.Contains(d.VerifiedAgainst, "20") {
			t.Errorf("%s: verified_against %q must name the date it was verified", d.ID, d.VerifiedAgainst)
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
		for _, id := range []string{GeminiCLI, "cursor", "copilot", Agents} {
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
		// PC-22 made claim-gate a hook vehicle on Claude Code; the absent
		// rendering is pinned on the pull tier, where it is still absent.
		if !strings.Contains(table, "| `claim-gate` | `hook-inject` |") {
			t.Errorf("claim-gate must ride the edit hook on claude-code (vision 26 §4):\n%s", table)
		}
		if codexTable := RenderCapabilityTable(Codex, now); !strings.Contains(codexTable, "| `claim-gate` | `absent` |") {
			t.Errorf("an absent vehicle must print as absent:\n%s", codexTable)
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

// TestWaveThreeClaudeHookAdditions pins HP-26..HP-31 to their position
// (after the vision 26 §4 pair) and to the one contract that matters most
// here: ONLY the two events whose reference documents additionalContext
// may declare an injecting vehicle. Claude Code 2.1.270 documents
// additionalContext on SubagentStart and PostToolUseFailure and NOT on
// FileChanged, WorktreeCreate, CwdChanged, DirectoryAdded or Setup — so a
// future edit that quietly promotes one of those to hook-inject (and with
// it, a stanza sentence promising a channel the harness does not have)
// fails here instead of in an adopter's session.
func TestWaveThreeClaudeHookAdditions(t *testing.T) {
	d, _ := Lookup(ClaudeCode)
	base := len(preSeamClaudeHookTable) + len(postSeamClaudeHookAdditions)
	canInject := map[string]bool{"SubagentStart": true, "PostToolUseFailure": true}
	for i, want := range waveThreeClaudeHookAdditions {
		got := d.Hooks[base+i]
		if got.Harness != want.event || got.Matcher != want.matcher || got.Marker != want.marker || got.Verb != want.sub || got.TimeoutSec != want.timeout {
			t.Errorf("wave-3 addition %d = {%s %q %q %q %d}, want {%s %q %q %q %d}", i,
				got.Harness, got.Matcher, got.Marker, got.Verb, got.TimeoutSec,
				want.event, want.matcher, want.marker, want.sub, want.timeout)
		}
		if canInject[got.Harness] {
			if got.Vehicle != VehicleHookInject || len(got.Output) == 0 {
				t.Errorf("%s documents additionalContext: vehicle = %s, output = %v — want hook-inject with an output map", got.Harness, got.Vehicle, got.Output)
			}
			continue
		}
		if got.Vehicle != VehicleHookSilent {
			t.Errorf("%s documents NO additionalContext: vehicle = %s, want hook-silent", got.Harness, got.Vehicle)
		}
		if len(got.Output) != 0 {
			t.Errorf("%s declares output %v but the reference documents no additionalContext on it", got.Harness, got.Output)
		}
	}
	// The matcher is the plugin-scoped-name half of HP-27: a fathom tool
	// reached through a plugin is still a fathom tool.
	b, ok := d.Binding(EventToolFailure)
	if !ok || !strings.Contains(b.Matcher, "mcp__plugin_") || !strings.Contains(b.Matcher, "mcp__fathom__") {
		t.Errorf("tool-failure matcher = %q, want both the plain and the plugin-scoped fathom tool names", b.Matcher)
	}
}

// TestWaveThreeClaudeCodecDecodes pins the field maps to real decoding:
// each wave-3 event's own stdin fields land in the envelope the verbs
// read, and no event reads a field the reference does not show on it.
func TestWaveThreeClaudeCodecDecodes(t *testing.T) {
	c := ClaudeCodec()
	in, err := c.DecodeInput(EventSubagentStart, []byte(`{"session_id":"s1","cwd":"/r","agent_id":"a1","agent_type":"explorer"}`))
	if err != nil || in.AgentType != "explorer" || in.Source != "a1" {
		t.Errorf("SubagentStart = %+v, %v", in, err)
	}
	in, err = c.DecodeInput(EventToolFailure, []byte(`{"tool_name":"mcp__fathom__find_symbol","tool_error":"daemon refused","tool_use_id":"t1"}`))
	if err != nil || in.ToolName != "mcp__fathom__find_symbol" || in.ToolError != "daemon refused" {
		t.Errorf("PostToolUseFailure = %+v, %v", in, err)
	}
	in, err = c.DecodeInput(EventWorktreeCreate, []byte(`{"session_id":"s1","worktree_path":"/r/wt"}`))
	if err != nil || in.Path != "/r/wt" {
		t.Errorf("WorktreeCreate = %+v, %v", in, err)
	}
	in, err = c.DecodeInput(EventCwdChanged, []byte(`{"cwd":"/r/new","previous_cwd":"/r/old"}`))
	if err != nil || in.Path != "/r/new" || in.Reason != "/r/old" {
		t.Errorf("CwdChanged = %+v, %v", in, err)
	}
	in, err = c.DecodeInput(EventCwdChanged, []byte(`{"cwd":"/r","directory_path":"/other","how_added":"slash_command"}`))
	if err != nil || in.Path != "/other" || in.Source != "slash_command" {
		t.Errorf("DirectoryAdded = %+v, %v", in, err)
	}
	in, err = c.DecodeInput(EventFileChanged, []byte(`{"file_path":"/r/CLAUDE.md"}`))
	if err != nil || in.FilePath != "/r/CLAUDE.md" {
		t.Errorf("FileChanged = %+v, %v", in, err)
	}
	in, err = c.DecodeInput(EventSetup, []byte(`{"triggered_by":"init"}`))
	if err != nil || in.Trigger != "init" {
		t.Errorf("Setup = %+v, %v", in, err)
	}
	// A tool event still never reads the top-level file_path.
	in, _ = c.DecodeInput(EventPostTool, []byte(`{"tool_name":"Edit","file_path":"/r/top.md","tool_input":{"file_path":"/r/tool.md"}}`))
	if in.FilePath != "/r/tool.md" {
		t.Errorf("PostToolUse file_path = %q, want tool_input.file_path", in.FilePath)
	}
}

// TestEventCatalogMatchesBindings is HP-32's honesty proof. The catalog
// is what doctor's hooks_coverage row PRINTS, so a Wired flag that
// disagrees with the bindings is a lie told to every adopter who runs
// doctor. Both directions are checked, and every unwired row must carry a
// reason: "not wired" with no reason is the shape of an oversight, and an
// oversight rendered as a decision is exactly the dishonesty ADR-029 §5
// exists to prevent.
func TestEventCatalogMatchesBindings(t *testing.T) {
	for _, d := range All() {
		if len(d.Events) == 0 {
			continue // not catalogued; doctor reports that as SKIPPED, never as full coverage
		}
		wired := map[string]bool{}
		for _, n := range d.WiredEventNames() {
			wired[n] = true
		}
		inCatalog := map[string]bool{}
		for _, e := range d.Events {
			if inCatalog[e.Event] {
				t.Errorf("%s: duplicate catalog row %q", d.ID, e.Event)
			}
			inCatalog[e.Event] = true
			if e.Wired != wired[e.Event] {
				t.Errorf("%s: catalog says %s wired=%t, bindings say %t", d.ID, e.Event, e.Wired, wired[e.Event])
			}
			if !e.Wired && strings.TrimSpace(e.Reason) == "" {
				t.Errorf("%s: unwired event %s carries no reason", d.ID, e.Event)
			}
			if e.Wired && e.Reason != "" {
				t.Errorf("%s: wired event %s carries an unwired reason %q", d.ID, e.Event, e.Reason)
			}
		}
		for n := range wired {
			if !inCatalog[n] {
				t.Errorf("%s: binds %s but the event catalog has no row for it", d.ID, n)
			}
		}
	}
	// The two catalogued surfaces, pinned to their documented sizes so a
	// harness release that grows one is noticed rather than absorbed.
	for id, want := range map[string]int{ClaudeCode: 33, Codex: 12} {
		d, _ := Lookup(id)
		if len(d.Events) != want {
			t.Errorf("%s catalogue = %d events, want %d (update the catalogue AND verified_against together)", id, len(d.Events), want)
		}
	}
}
