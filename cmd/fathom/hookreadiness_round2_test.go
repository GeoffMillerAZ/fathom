package main

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/internal/harness"
	"github.com/geoffmilleraz/fathom/internal/semantic"
)

func TestHealthTransitionsDoNotRewriteSameStateOrDay(t *testing.T) {
	root, now := readinessEnv(t)
	cap := harness.CapObligationsPostEdit
	if err := updateHookHealth(root, harness.ClaudeCode, cap, true, "", now); err != nil {
		t.Fatal(err)
	}
	path := hookHealthPath(root, harness.ClaudeCode)
	sentinel := now.Add(-24 * time.Hour)
	if err := os.Chtimes(path, sentinel, sentinel); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	for i := 1; i <= hookDedupeMaxEntries+1; i++ {
		if err := updateHookHealth(root, harness.ClaudeCode, cap, true, "", now.Add(time.Duration(i)*time.Second)); err != nil {
			t.Fatal(err)
		}
	}
	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if !info.ModTime().Equal(sentinel) {
		t.Fatal("same state/day rewrote the file")
	}
	if !bytes.Equal(before, after) {
		t.Fatalf("same state/day changed stamp: %s", after)
	}
	var state struct {
		Channels map[string]map[string]json.RawMessage `json:"channels"`
	}
	if err := json.Unmarshal(after, &state); err != nil {
		t.Fatal(err)
	}
	stamp := state.Channels[string(cap)]
	if len(stamp) != 3 || stamp["last_state"] == nil || stamp["changed_at"] == nil || stamp["last_emit_at"] == nil {
		t.Fatalf("stamp must contain only transition state: %s", after)
	}
}

func TestStampErrorsNeverChangeReadinessSkip(t *testing.T) {
	root, now := readinessEnv(t)
	path := hookHealthPath(root, harness.ClaudeCode)
	if err := os.MkdirAll(path, 0700); err != nil {
		t.Fatal(err)
	}
	if channelReady(harness.CapObligationsPostEdit, readinessInput{root: root, harnessID: harness.ClaudeCode, now: now, instructionsKnown: true, uncompiled: 1}, "edit-hook", "s") {
		t.Fatal("stamp error changed no-instructions skip")
	}
	rows := readBeats(t, beatLogPath(), "edit-hook")
	if len(rows) != 1 || rows[0].Meta["reason"] != "not-ready:no-instructions" {
		t.Fatalf("skip row missing: %+v", rows)
	}
}

func TestObserveOnlyHooksNeverCreateHealthStamps(t *testing.T) {
	root, _ := readinessEnv(t)
	isolateHookRun(t)
	t.Setenv("CLAUDE_PROJECT_DIR", root)
	for _, tool := range []string{"Read", "Grep", "Glob", "Bash"} {
		args := []string{"--url", "http://daemon.invalid"}
		if tool != "Bash" {
			args = append(args, "--pre")
		}
		input := map[string]string{"file_path": filepath.Join(root, "file.go")}
		if tool == "Bash" {
			input = map[string]string{"command": "ls"}
		}
		body := mustJSON(map[string]any{"session_id": "s", "cwd": root, "tool_name": tool, "tool_input": input})
		restore := pipeStdin(t, body)
		code := cmdEditHook(args, io.Discard, io.Discard)
		restore()
		if code != 0 {
			t.Fatal(code)
		}
	}
	paths, _ := filepath.Glob(filepath.Join(os.Getenv("FATHOM_HOME"), "hook-dedupe", "health-*"))
	if len(paths) > 0 {
		t.Fatalf("observe-only calls created stamps: %v", paths)
	}
}

func TestFoldedReadoutRegistersTestRunReadiness(t *testing.T) {
	root, _ := readinessEnv(t)
	if err := os.WriteFile(filepath.Join(os.Getenv("FATHOM_HOME"), "config.toml"), []byte("[readouts]\nmode = 'rich'\n[hooks]\ntest_run = true\n"), 0600); err != nil {
		t.Fatal(err)
	}
	hooks := map[string][]any{}
	for _, b := range hookSpecsFor(harness.ClaudeCode, hookOptInsAt(root)...) {
		hooks[b.event] = append(hooks[b.event], map[string]any{"matcher": b.matcher, "hooks": []any{map[string]any{"type": "command", "command": b.marker}}})
	}
	path := filepath.Join(root, ".claude", "settings.json")
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(mustJSON(map[string]any{"hooks": hooks})), 0600); err != nil {
		t.Fatal(err)
	}
	if r := readinessInstalled(root, harness.ClaudeCode, harness.CapTestRunLine); !r.ready {
		t.Fatalf("folded test-run not registered: %+v", r)
	}
}

func TestPostEditReadinessFiltersHeadingsWithoutDroppingOtherPayloads(t *testing.T) {
	for _, kind := range []string{"compiled", "raw", "decision", "finding", "spec", "stale"} {
		t.Run(kind, func(t *testing.T) {
			root, _ := readinessEnv(t)
			file := filepath.Join(root, "file.go")
			if err := os.WriteFile(file, []byte("package example\n// implements [US-042]\n"), 0600); err != nil {
				t.Fatal(err)
			}
			rules := []semantic.Obligation{{Rule: "Working conventions", Tier: "raw"}}
			payload := map[string]any{"rules": rules, "layer_coverage": map[string]int{"sections_total": 3, "sections_compiled": 0}}
			want := ""
			switch kind {
			case "compiled":
				rules = append(rules, semantic.Obligation{Rule: "Keep output sorted.", Tier: "doc-claimed"})
				want = "Keep output sorted."
			case "raw":
				rules = append(rules, semantic.Obligation{Rule: "Never lose an instruction.", Tier: "raw"})
				want = "Never lose an instruction."
			case "decision":
				rules = append(rules, semantic.Obligation{Rule: "**Decision** Always keep deterministic evidence.", Tier: "raw"})
				want = "**Decision** Always keep deterministic evidence."
			case "finding":
				rules = append(rules, semantic.Obligation{Rule: "Check the edit.", Kind: "finding", Src: "file.go#L1-L2", RuleID: "example", Producer: "review"})
				want = "Check the edit."
			case "spec":
				payload["spec_refs"] = map[string]any{"US-042": []specRefLoc{{File: "docs/spec.md", Line: 2}}}
				want = "implements [US-042]"
			case "stale":
				payload["index_stale"] = true
				want = "index is behind"
			}
			payload["rules"] = rules
			readinessHTTP(t, payload)
			text, _, ok := renderPostEdit("http://daemon.invalid", root, file, "s", "package example")
			if !ok || !strings.Contains(text, want) || strings.Contains(text, "Working conventions") {
				t.Fatalf("post-edit %s: %q", kind, text)
			}
			if !fileRecentlyEdited("s", "file.go", time.Now()) {
				t.Fatal("edit was not recorded")
			}
		})
	}
}

func TestReadinessEmptyCorpusIsNotMissingInstructions(t *testing.T) {
	readinessEnv(t)
	readinessHTTP(t, map[string]any{"rules": []any{}, "layer_coverage": map[string]int{"sections_total": 0, "sections_compiled": 0}})
	var got readinessResult
	_, _, _, ok, err := fetchObligations("http://daemon.invalid", "example", "file.go", 0, time.Second, "", func(r readinessResult) { got = r })
	if !ok || err != nil || !got.ready {
		t.Fatalf("empty corpus is not no-instructions: %+v %v", got, err)
	}
}

func TestHealthBreakerAndUnusedMapsAreAbsentFromPrimer(t *testing.T) {
	root, now := readinessEnv(t)
	if err := updateHookHealth(root, harness.ClaudeCode, harness.CapObligationsPostEdit, false, "breaker_open", now); err != nil {
		t.Fatal(err)
	}
	if got := inactiveFeaturesLine(root, harness.ClaudeCode, now); got != "" {
		t.Fatalf("breaker or unused map in primer: %q", got)
	}
}

func TestHealthInvalidContentsResetToFixedSize(t *testing.T) {
	root, now := readinessEnv(t)
	path := hookHealthPath(root, harness.ClaudeCode)
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	for _, raw := range []string{"bad json", `{"version":1,"channels":{}}`, strings.Repeat("x", hookDedupeMaxBytes+1), `{"version":2,"channels":{"obligations-post-edit":{"last_state":"ready"}}}`} {
		if err := os.WriteFile(path, []byte(raw), 0600); err != nil {
			t.Fatal(err)
		}
		if err := updateHookHealth(root, harness.ClaudeCode, harness.CapObligationsPostEdit, true, "", now); err != nil {
			t.Fatal(err)
		}
		after, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if len(after) > 512 || strings.Contains(string(after), "events") {
			t.Fatalf("state did not reset: %s", after)
		}
		state, err := readHookHealth(root, harness.ClaudeCode, now)
		if err != nil || state.Channels[harness.CapObligationsPostEdit].LastState != "ready" {
			t.Fatalf("state=%+v err=%v", state, err)
		}
	}
}

func TestOptInReadinessCoversEveryDescriptorKey(t *testing.T) {
	seams := hookOptInCapabilities()
	if len(seams) != len(hookOptInKeys()) {
		t.Fatalf("seam keys=%v descriptor keys=%v", seams, hookOptInKeys())
	}
	for _, d := range harness.All() {
		for _, b := range d.Hooks {
			if b.OptIn == "" {
				continue
			}
			found := false
			for _, cap := range seams[b.OptIn] {
				if cap == b.Capability {
					found = true
				}
			}
			if !found || hookReadinessRegistry()[b.Capability].check == nil {
				t.Errorf("%s %s missing %s", d.ID, b.OptIn, b.Capability)
			}
		}
	}
}

func TestHealthUnknownFieldsResetInsteadOfRetainingHistory(t *testing.T) {
	root, now := readinessEnv(t)
	cap := harness.CapObligationsPostEdit
	if err := updateHookHealth(root, harness.ClaudeCode, cap, true, "", now); err != nil {
		t.Fatal(err)
	}
	path := hookHealthPath(root, harness.ClaudeCode)
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	raw = bytes.Replace(raw, []byte(`"last_state"`), []byte(`"events":[{}],"last_state"`), 1)
	if err := os.WriteFile(path, raw, 0600); err != nil {
		t.Fatal(err)
	}
	if err := updateHookHealth(root, harness.ClaudeCode, cap, true, "", now); err != nil {
		t.Fatal(err)
	}
	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(after, []byte(`"events"`)) {
		t.Fatal("invalid stamp retained an event array")
	}
}

func TestPostEditKnownSectionsWithoutRulesLogNoInstructions(t *testing.T) {
	root, _ := readinessEnv(t)
	readinessHTTP(t, map[string]any{"rules": []any{}, "layer_coverage": map[string]int{"sections_total": 3, "sections_compiled": 0}})
	text, _, ok := renderPostEdit("http://daemon.invalid", root, filepath.Join(root, "file.go"), "s", "")
	if ok || text != "" {
		t.Fatalf("empty advice emitted: %q", text)
	}
	if !fileRecentlyEdited("s", "file.go", time.Now()) {
		t.Fatal("silent edit was not recorded")
	}
	rows := readBeats(t, beatLogPath(), "edit-hook")
	if len(rows) != 1 || rows[0].Kind != "skipped" || rows[0].Meta["reason"] != "not-ready:no-instructions" {
		t.Fatalf("known sections lost readiness skip: %+v", rows)
	}
}
