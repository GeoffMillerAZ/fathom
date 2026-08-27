package main

// capabilityenforce_test.go — the obligation the first cut of the registry
// was missing: a switch must SWITCH something.
//
// The registry shipped with resolution read only by its two reporting
// surfaces, so `capabilities set guards off` recorded intent, the table
// reported "switched off by this repository", and guards kept firing. This
// file makes that state impossible to reach again: every Switchable
// capability must have a case here proving observed behaviour changes, and
// every non-switchable one must refuse `set` rather than accept it.

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/capability"
)

// TestEverySwitchableCapabilityHasAnEnforcementCase is the registry-level
// guard: adding Switchable: true obliges you to add a case below. Without
// this, the next capability re-introduces the original defect silently.
func TestEverySwitchableCapabilityHasAnEnforcementCase(t *testing.T) {
	covered := map[string]bool{
		"obligations":   true, // TestSwitchOffStopsObligationInjection
		"guards":        true, // TestSwitchOffStopsGuards
		"verify-gate":   true, // TestSwitchOffSkipsVerifyButKeepsLedger
		"intent-corpus": true, // TestSwitchOffSilencesIntentRowsNotTheVerb
	}
	for _, c := range capability.Default().All() {
		if c.Switchable && !covered[c.ID] {
			t.Errorf("%q is Switchable but has no enforcement test — a switch that switches nothing "+
				"is the defect this file exists to prevent", c.ID)
		}
		if !c.Switchable && strings.TrimSpace(c.NotSwitchableBecause) == "" {
			t.Errorf("%q is not switchable and does not say why; a reader needs to know what DOES control it", c.ID)
		}
	}
}

// enforceRepo builds a repo with guards and rules present, plus a
// capabilities file setting id off.
func enforceRepo(t *testing.T, id string, off bool) string {
	t.Helper()
	root := t.TempDir()
	mk := func(rel, body string) {
		p := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	mk("main.go", "package main\n")
	mk(".fathom/guard.cue", "guards: [{scopes: [\"**/*.go\"], note: \"be careful here\"}]\n")
	mk(".fathom/rules/r.cue", "rules: []\n")
	if off {
		mk(".fathom/capabilities.cue", "capabilities: {\n\t"+id+": on: false\n}\n")
	}
	return root
}

func TestSwitchOffStopsGuards(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("FATHOM_CHANNELS", "")
	orig := guardStampDir
	guardStampDir = filepath.Join(t.TempDir(), "stamps")
	t.Cleanup(func() { guardStampDir = orig })

	// One harness, run twice: identical repo and daemon, the only
	// difference being the capabilities file. Anything that differs
	// between the two runs is attributable to the switch alone.
	run := func(t *testing.T, switchOff bool) (string, int32) {
		root := t.TempDir()
		writeGuardCue(t, root, `guards: [{scopes: ["internal/**"], note: "GUARDNOTE", max_rules: 4}]`)
		if switchOff {
			if err := os.WriteFile(filepath.Join(root, ".fathom", "capabilities.cue"),
				[]byte("capabilities: {\n\tguards: on: false\n}\n"), 0o600); err != nil {
				t.Fatal(err)
			}
		}
		file := filepath.Join(root, "internal", "store", "db.go")

		var hits int32
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			atomic.AddInt32(&hits, 1)
			payload := map[string]any{
				"service": strings.ToLower(filepath.Base(root)),
				"file":    "internal/store/db.go",
				"rules": []any{map[string]any{
					"rule": "RULETEXT", "src": "docs/spec.md#L1-L2", "tier": "verified",
				}},
			}
			_ = json.NewEncoder(w).Encode(map[string]any{
				"jsonrpc": "2.0", "id": 1,
				"result": map[string]any{"content": []any{
					map[string]any{"type": "text", "text": mustJSON(payload)},
				}},
			})
		}))
		defer srv.Close()

		hookStdin(t, fmt.Sprintf(`{"tool_name":"Edit","tool_input":{"file_path":%q},"cwd":%q,"session_id":"s1"}`, file, root))
		var out, errBuf bytes.Buffer
		if code := runWithStdin([]string{"edit-hook", "--pre", "--url=" + srv.URL}, nil, &out, &errBuf); code != 0 {
			t.Fatalf("exit %d: %s", code, errBuf.String())
		}
		return out.String(), atomic.LoadInt32(&hits)
	}

	onOut, onHits := run(t, false)
	if !strings.Contains(onOut, "GUARDNOTE") {
		t.Fatalf("baseline broken — the guard must fire when the capability is on:\n%s", onOut)
	}
	if onHits == 0 {
		t.Fatal("baseline broken — a matched guard fetches rules from the daemon")
	}

	offOut, offHits := run(t, true)
	if strings.Contains(offOut, "GUARDNOTE") || strings.Contains(offOut, "RULETEXT") {
		t.Errorf("`capabilities set guards off` did not stop the guard firing:\n%s", offOut)
	}
	if offOut != "" {
		t.Errorf("an off capability must be silent, got:\n%s", offOut)
	}
	if offHits != 0 {
		t.Errorf("an off capability made %d daemon call(s) — the gate must precede the traffic", offHits)
	}
}

func TestSwitchOffStopsObligationInjection(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("FATHOM_CHANNELS", "")
	hermeticObsEnv(t)

	root := enforceRepo(t, "obligations", true)
	var hits atomic.Int64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{}`))
	}))
	defer srv.Close()

	env := map[string]any{
		"tool_name":  "Edit",
		"tool_input": map[string]string{"file_path": filepath.Join(root, "main.go")},
		"cwd":        root,
	}
	raw, _ := json.Marshal(env)
	defer pipeStdin(t, string(raw))()

	var out, errb bytes.Buffer
	if code := cmdEditHook([]string{"--url", srv.URL}, &out, &errb); code != 0 {
		t.Fatalf("exit %d: %s", code, errb.String())
	}
	if out.Len() != 0 {
		t.Errorf("obligations switched off must inject nothing:\n%s", out.String())
	}
	if n := hits.Load(); n != 0 {
		t.Errorf("obligations switched off made %d daemon call(s) — the gate must precede the traffic", n)
	}
}

func TestSwitchOffSkipsVerifyButKeepsLedger(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("FATHOM_CHANNELS", "")
	dir := hermeticObsEnv(t)
	ledger := filepath.Join(dir, "ledger.jsonl")

	root := enforceRepo(t, "verify-gate", true)
	var hits atomic.Int64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{}`))
	}))
	defer srv.Close()

	env := map[string]any{
		"stop_hook_active": false,
		"cwd":              root,
		"session_id":       "cap-off",
		"transcript_path":  filepath.Join(t.TempDir(), "none.jsonl"),
	}
	raw, _ := json.Marshal(env)
	defer pipeStdin(t, string(raw))()

	var out, errb bytes.Buffer
	if code := cmdStopHook([]string{"--url", srv.URL}, &out, &errb); code != 0 {
		t.Fatalf("exit %d: %s", code, errb.String())
	}
	if n := hits.Load(); n != 0 {
		t.Errorf("the verify gate ran despite being switched off (%d daemon call(s))", n)
	}
	// Switching off an assistive gate is not a reason to stop MEASURING.
	data, err := os.ReadFile(ledger)
	if err != nil || len(strings.TrimSpace(string(data))) == 0 {
		t.Fatalf("the ledger row must still be written: %v", err)
	}
	var row struct {
		VerifyCatches *int `json:"verify_catches"`
	}
	if err := json.Unmarshal([]byte(strings.Split(strings.TrimSpace(string(data)), "\n")[0]), &row); err != nil {
		t.Fatal(err)
	}
	if row.VerifyCatches != nil {
		t.Errorf("verify_catches = %v, want nil — the gate did not run, so its result is unknown (law 3)", *row.VerifyCatches)
	}
}

// TestSetRefusesStatusOnlyCapabilities pins the other half: a capability
// with no switch must refuse `set` rather than write a file that changes
// nothing.
func TestSetRefusesStatusOnlyCapabilities(t *testing.T) {
	repo := t.TempDir()
	t.Setenv("HOME", t.TempDir())
	for _, c := range capability.Default().All() {
		if c.Switchable {
			continue
		}
		t.Run(c.ID, func(t *testing.T) {
			var out, errb bytes.Buffer
			if code := cmdCapabilities([]string{"set", "--repo", repo, c.ID, "off"}, &out, &errb); code == 0 {
				t.Fatalf("%q has no switch; `set` must refuse rather than record intent it cannot honour", c.ID)
			}
			if !strings.Contains(errb.String(), "not a switch") {
				t.Errorf("the refusal must say what it is: %s", errb.String())
			}
		})
	}
	if _, err := os.Stat(filepath.Join(repo, ".fathom", "capabilities.cue")); err == nil {
		t.Error("a refused set must not have written a config file")
	}
}

// TestEnforcementFailsOpenOnBrokenConfig pins the direction of failure: a
// malformed config must never DISARM a safety feature. It is reported
// loudly elsewhere (doctor FAILs, `capabilities` prints it); it does not
// get to switch guards off by accident.
func TestEnforcementFailsOpenOnBrokenConfig(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".fathom"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".fathom", "capabilities.cue"),
		[]byte("capabilities: {\n\tguards: on?: false\n}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if capabilityOffAt(root, "guards") {
		t.Error("a config file that does not load must not disarm a guard — fail open, and report loudly elsewhere")
	}
}

// TestSwitchOffSilencesIntentRowsNotTheVerb (EP-10) is intent-corpus's
// enforcement case. `off` must change observed behaviour, and the behaviour
// it changes is the REPORTING channel: doctor stops narrating the corpus.
// The verb keeps answering — switching off a report is not the same as
// making the tool refuse a direct question, and conflating the two is how a
// switch becomes a gag.
func TestSwitchOffSilencesIntentRowsNotTheVerb(t *testing.T) {
	root := t.TempDir()
	writeCorpusFixture(t, root)
	t.Setenv("FATHOM_INTENT_LEDGER", filepath.Join(root, "l.jsonl"))
	t.Setenv("FATHOM_VERIFY_LEDGER", filepath.Join(root, "v.jsonl"))

	// On (the default when a router exists): the row narrates the corpus.
	on := checkIntent(root)
	if !strings.Contains(on.Reason, "condition(s)") {
		t.Fatalf("with the capability on, the row must narrate the corpus: %q", on.Reason)
	}

	// Off: the row says so and stops reading.
	if err := os.MkdirAll(filepath.Join(root, ".fathom"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".fathom", "capabilities.cue"),
		[]byte("capabilities: {\n\t\"intent-corpus\": on: false\n}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	off := checkIntent(root)
	if !strings.Contains(off.Reason, "switched off") {
		t.Errorf("with the capability off, the row must say so: %q", off.Reason)
	}
	if strings.Contains(off.Reason, "condition(s)") {
		t.Errorf("a switched-off capability must stop narrating: %q", off.Reason)
	}
	if offClaims := checkUnprovenClaims(root, "http://127.0.0.1:1", "svc"); !strings.Contains(offClaims.Reason, "switched off") {
		t.Errorf("unproven_claims must honour the same switch: %q", offClaims.Reason)
	}

	// The VERB still answers — off silences a report, never a question.
	var out, errBuf bytes.Buffer
	if code := cmdIntent([]string{"audit", "--root", root, "--url", "http://127.0.0.1:1", "--service", "svc"}, &out, &errBuf); code != 0 {
		t.Fatalf("the verb must keep working when the capability is off: exit %d %s", code, errBuf.String())
	}
	if !strings.Contains(out.String(), "conditions:") {
		t.Errorf("the verb must still answer on demand:\n%s", out.String())
	}
}
