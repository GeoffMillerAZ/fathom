package main

// claimcheck_hooktable_test.go — RO-38 (docs/plans/readouts.md): the
// SubagentStop row is in the one hook table only while `[hooks]
// claim_check` is on, and init, reconcile, doctor and plugin render all
// carry it from there.

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/internal/harness"
)

var subagentStopSpec = hookSpec{event: "SubagentStop", matcher: "", marker: "fathom stop-hook --event subagent-stop", sub: "stop-hook --event subagent-stop", timeout: 2}

func hasSubagentStop(specs []hookSpec) bool {
	for _, s := range specs {
		if s.event == "SubagentStop" {
			if s != subagentStopSpec {
				return false
			}
			return true
		}
	}
	return false
}

func TestHookSpecsForClaudeWiresSubagentStopOnlyUnderClaimCheck(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	off := t.TempDir()
	on := t.TempDir()
	writeRepoFile(t, on, ".fathom/config.toml", "[hooks]\ntest_run = true\nclaim_check = true\n")

	// The default table never carries it: every existing caller and golden
	// that passes no opt-in reads the table it read before.
	if hasSubagentStop(hookSpecsFor(harness.ClaudeCode)) || hasSubagentStop(fathomHookSpecs) {
		t.Fatal("the default Claude Code table carries SubagentStop with claim_check off")
	}
	if !hasSubagentStop(hookSpecsFor(harness.ClaudeCode, hookOptInClaimCheck)) {
		t.Fatalf("hookSpecsFor(claude-code, %s) lacks %+v", hookOptInClaimCheck, subagentStopSpec)
	}
	if got := len(hookSpecsFor(harness.ClaudeCode, hookOptInClaimCheck)); got != len(fathomHookSpecs)+1 {
		t.Errorf("opt-in table = %d rows, want the default %d plus one", got, len(fathomHookSpecs))
	}
	for _, id := range []string{harness.Codex, harness.GeminiCLI} {
		if hasSubagentStop(hookSpecsFor(id, hookOptInClaimCheck)) {
			t.Errorf("%s carries a SubagentStop row; it is unbound there", id)
		}
	}

	// The root's config decides.
	if hasSubagentStop(fathomHookSpecsAt(off)) {
		t.Error("fathomHookSpecsAt(off root) carries SubagentStop")
	}
	if !hasSubagentStop(fathomHookSpecsAt(on)) {
		t.Error("fathomHookSpecsAt(on root) lacks SubagentStop")
	}

	// init writes it only under the switch.
	targets, err := initTargetsFor(harness.ClaudeCode, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	initKeys := func(root string) bool {
		spec, _ := initWiringSpecFor(root, "svc", testBinPath, targets, hooksScopeAuto)
		for _, k := range spec.Keys {
			if len(k.Path) > 0 && k.Path[0] == "SubagentStop" && strings.Contains(k.Body, "stop-hook --event subagent-stop") && k.Timeout == 2 {
				return true
			}
		}
		return false
	}
	if initKeys(off) {
		t.Error("init plans a SubagentStop entry with claim_check off")
	}
	if !initKeys(on) {
		t.Error("init plans no SubagentStop entry with claim_check on")
	}

	// reconcile names it missing only under the switch.
	missing := func(root string) bool {
		for _, m := range analyzeWiring(root).Missing {
			if strings.HasPrefix(m, "SubagentStop[") {
				return true
			}
		}
		return false
	}
	if missing(off) {
		t.Error("reconcile expects SubagentStop with claim_check off")
	}
	if !missing(on) {
		t.Error("reconcile does not name the missing SubagentStop entry with claim_check on")
	}

	// doctor's hooks_installed expects it only under the switch: both roots
	// wire every default row, and only the claim_check root misses one.
	for _, root := range []string{off, on} {
		writeDefaultHookSettings(t, root)
	}
	if c := checkHooksInstalled(off, "claude"); c.Status != doctorPass {
		t.Errorf("doctor on the off root = %+v, want a pass with the default rows wired", c)
	}
	if c := checkHooksInstalled(on, "claude"); c.Status == doctorPass || !strings.Contains(c.Reason+c.Remedy, "SubagentStop") {
		t.Errorf("doctor on the claim_check root = %+v, want SubagentStop named missing", c)
	}

	// plugin render reads the user layer: a user-scope switch adds the row.
	if hasSubagentStop(hookSpecsFor(harness.ClaudeCode, hookOptInsAt("")...)) {
		t.Error("the user layer is empty, yet the plugin table carries SubagentStop")
	}
	if err := os.MkdirAll(filepath.Join(home, ".fathom"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeRepoFile(t, home, ".fathom/config.toml", "[hooks]\nclaim_check = true\n")
	if !hasSubagentStop(hookSpecsFor(harness.ClaudeCode, hookOptInsAt("")...)) {
		t.Error("a user-layer claim_check does not add the plugin's SubagentStop row")
	}
	if !hasSubagentStop(fathomHookSpecsAt(off)) {
		t.Error("a user-layer claim_check does not reach a root whose own config is silent")
	}
}

// writeDefaultHookSettings wires every default Claude Code row in root's
// project settings, by fathom's own marker.
func writeDefaultHookSettings(t *testing.T, root string) {
	t.Helper()
	hooks := map[string][]any{}
	for _, s := range fathomHookSpecs {
		hooks[s.event] = append(hooks[s.event], map[string]any{
			"matcher": s.matcher,
			"hooks":   []any{map[string]any{"type": "command", "command": "fathom " + s.sub, "timeout": s.timeout}},
		})
	}
	b, err := json.Marshal(map[string]any{"hooks": hooks})
	if err != nil {
		t.Fatal(err)
	}
	writeRepoFile(t, root, ".claude/settings.json", string(b))
}
