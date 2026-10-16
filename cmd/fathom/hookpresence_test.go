package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/wiring"
)

// testRunHookMarker is the PostToolUse(Bash) test-run row's marker (TR-2).
const testRunHookMarker = "fathom session-hook --event test-run"

// settingsWithout renders every fathom hook row (the template, spelled
// "fathom …") minus the rows whose command contains one of drop.
func settingsWithout(t *testing.T, drop ...string) []byte {
	t.Helper()
	merged, _, _, err := mergeHookSettings(nil, "fathom")
	if err != nil {
		t.Fatal(err)
	}
	var root map[string]any
	if err := json.Unmarshal(merged, &root); err != nil {
		t.Fatal(err)
	}
	hooks := root["hooks"].(map[string]any)
	for ev, v := range hooks {
		var groups []any
		for _, g := range v.([]any) {
			gm := g.(map[string]any)
			var kept []any
			for _, h := range gm["hooks"].([]any) {
				cmd, _ := h.(map[string]any)["command"].(string)
				gone := false
				for _, d := range drop {
					if strings.Contains(cmd, d) {
						gone = true
					}
				}
				if !gone {
					kept = append(kept, h)
				}
			}
			if len(kept) > 0 {
				gm["hooks"] = kept
				groups = append(groups, gm)
			}
		}
		if len(groups) == 0 {
			delete(hooks, ev)
			continue
		}
		hooks[ev] = groups
	}
	out, err := json.MarshalIndent(root, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	return out
}

func writeSettings(t *testing.T, dir string, body []byte) string {
	t.Helper()
	p := filepath.Join(dir, ".claude", "settings.json")
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, body, 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

// TestHookRowSharingAMatcherGroupIsCheckedByItsOwnMarker pins FA-7
// (O175): two template rows share the PostToolUse(Bash) group (edit-hook,
// O57, and the test-run hook, TR-2), and each is present only when its OWN
// marker is wired in that group at some scope. The (event, matcher) key
// alone let edit-hook's entry stand for the test-run row: doctor's
// hooks_installed read PASS, reconcile's dry-run named nothing missing and
// its apply left the row unwired with a "not fathom's template spelling"
// note, and init skipped it.
func TestHookRowSharingAMatcherGroupIsCheckedByItsOwnMarker(t *testing.T) {
	const missingLine = "PostToolUse[Bash] → fathom session-hook --event test-run"

	t.Run("project scope wires edit-hook in the group, not test-run", func(t *testing.T) {
		t.Setenv("HOME", t.TempDir()) // user scope empty
		root := t.TempDir()
		settings := writeSettings(t, root, settingsWithout(t, testRunHookMarker))

		got := checkHooksInstalled(root, "claude")
		if got.Status != doctorFail || !strings.Contains(got.Reason, "session-hook --event test-run [PostToolUse, matcher Bash]") {
			t.Errorf("doctor hooks_installed = %+v, want a fail naming the test-run row", got)
		}
		if strings.Contains(got.Reason, "edit-hook [PostToolUse") {
			t.Errorf("doctor named the wired edit-hook row missing: %s", got.Reason)
		}
		if d := analyzeWiring(root); len(d.Missing) != 1 || d.Missing[0] != missingLine {
			t.Errorf("reconcile missing = %q, want exactly %q", d.Missing, missingLine)
		}
		spec, _ := initWiringSpec(root, "svc", "fathom")
		if n := keysWithMarker(spec.Keys, testRunHookMarker); n != 1 {
			t.Errorf("init plans %d test-run rows, want 1", n)
		}

		var stdout, stderr strings.Builder
		if code := cmdFleetReconcile([]string{"--repo", root}, &stdout, &stderr); code != 0 {
			t.Fatalf("dry-run exit %d: %s\n%s", code, stdout.String(), stderr.String())
		}
		if !strings.Contains(stdout.String(), "missing: "+missingLine) {
			t.Errorf("dry-run does not name the row:\n%s", stdout.String())
		}
		stdout.Reset()
		if code := cmdFleetReconcile([]string{"--apply", "--repo", root}, &stdout, &stderr); code != 0 {
			t.Fatalf("apply exit %d: %s\n%s", code, stdout.String(), stderr.String())
		}
		if strings.Contains(stdout.String(), "not fathom's template spelling") {
			t.Errorf("apply mistook edit-hook for the test-run row:\n%s", stdout.String())
		}
		data, _ := os.ReadFile(settings)
		var parsed struct {
			Hooks map[string][]struct {
				Matcher string `json:"matcher"`
				Hooks   []struct {
					Command string `json:"command"`
				} `json:"hooks"`
			} `json:"hooks"`
		}
		if err := json.Unmarshal(data, &parsed); err != nil {
			t.Fatal(err)
		}
		var bashGroups [][]string
		for _, g := range parsed.Hooks["PostToolUse"] {
			if g.Matcher == "Bash" {
				var cmds []string
				for _, h := range g.Hooks {
					cmds = append(cmds, h.Command)
				}
				bashGroups = append(bashGroups, cmds)
			}
		}
		if len(bashGroups) != 1 || len(bashGroups[0]) != 2 || bashGroups[0][0] != hookCommand("fathom", "edit-hook") || !strings.HasSuffix(bashGroups[0][1], testRunHookMarker) {
			t.Errorf("PostToolUse(Bash) after apply = %q, want one group: edit-hook then the test-run row", bashGroups)
		}
		if got := checkHooksInstalled(root, "claude"); got.Status != doctorPass {
			t.Errorf("doctor after apply = %+v, want pass", got)
		}
		if d := analyzeWiring(root); len(d.Missing) != 0 {
			t.Errorf("reconcile after apply still misses %q", d.Missing)
		}
	})

	t.Run("user scope wires edit-hook in the group, not test-run", func(t *testing.T) {
		home := t.TempDir()
		t.Setenv("HOME", home)
		writeSettings(t, home, []byte(`{"hooks":{"PostToolUse":[{"matcher":"Bash","hooks":[{"type":"command","command":"fathom edit-hook","timeout":5}]}]}}`))
		root := t.TempDir()
		writeSettings(t, root, settingsWithout(t, testRunHookMarker, "fathom edit-hook\""))
		// The project settings carry every other row but no PostToolUse(Bash)
		// group; user scope serves edit-hook there and nothing serves test-run.
		body, _ := os.ReadFile(filepath.Join(root, ".claude", "settings.json"))
		if strings.Contains(string(body), testRunHookMarker) {
			t.Fatalf("fixture still wires the test-run row:\n%s", body)
		}

		got := checkHooksInstalled(root, "claude")
		if got.Status != doctorFail || !strings.Contains(got.Reason, "session-hook --event test-run [PostToolUse, matcher Bash]") {
			t.Errorf("doctor hooks_installed = %+v, want a fail naming the test-run row", got)
		}
		keys, _ := shimHookKeys(root)
		if n := keysWithMarker(keys, testRunHookMarker); n != 1 {
			t.Errorf("reconcile plans %d test-run rows at project scope, want 1 (user scope serves only edit-hook)", n)
		}
		spec, notes := initWiringSpec(root, "svc", "fathom")
		if n := keysWithMarker(spec.Keys, testRunHookMarker); n != 1 {
			t.Errorf("init plans %d test-run rows, want 1; notes %q", n, notes)
		}
	})
}

func keysWithMarker(keys []wiring.Key, marker string) int {
	n := 0
	for _, k := range keys {
		if k.Marker == marker {
			n++
		}
	}
	return n
}
