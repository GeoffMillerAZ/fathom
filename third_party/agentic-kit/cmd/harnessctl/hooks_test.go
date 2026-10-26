//go:build harness_maintenance

package main

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func hookTestNative(host string, input map[string]any) []byte {
	tool := "Agent"
	if host == "codex" {
		tool = "collaborationspawn_agent"
	}
	b, _ := json.Marshal(map[string]any{"hook_event_name": "PreToolUse", "session_id": "session", "tool_use_id": "call", "cwd": "/selected", "tool_name": tool, "transcript_path": nil, "tool_input": input})
	return b
}
func TestHookNormalizeStrictUsedAndForwardUnused(t *testing.T) {
	tests := []struct {
		name, host        string
		input             map[string]any
		operation         string
		fork, contextFork bool
	}{
		{"claude spawn", "claude", map[string]any{"subagent_type": "fork", "description": "critic", "prompt": "[deep] review"}, "spawn", true, false},
		{"claude resume", "claude", map[string]any{"resume": "old-agent", "prompt": "continue"}, "resume", false, false},
		{"codex fork", "codex", map[string]any{"model": "m", "reasoning_effort": "high", "fork_turns": "all", "message": "[critic] review", "items": []any{map[string]any{"text": "details"}}}, "spawn", false, true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, e := hookNormalize(hookTestNative(tc.host, tc.input), tc.host, "/selected")
			if e != nil || got.Subject.Operation != tc.operation || got.Subject.Fork != tc.fork || got.Subject.ContextFork != tc.contextFork || got.SessionID != "session" || !strings.Contains(got.Subject.Brief, "review") && tc.operation == "spawn" {
				t.Fatalf("%+v %v", got, e)
			}
		})
	}
	malformed := [][]byte{
		[]byte(`{"hook_event_name":"PreToolUse","hook_event_name":"PreToolUse"}`),
		hookTestNative("codex", map[string]any{"message": "x", "fork_context": "yes"}),
		hookTestNative("codex", map[string]any{"message": "x", "fork_context": true, "fork_turns": "none"}),
		hookTestNative("claude", map[string]any{"subagent_type": "fork", "resume": "id"}),
		hookTestNative("codex", map[string]any{"message": "x", "items": []any{map[string]any{"text": 7}}}),
	}
	for i, raw := range malformed {
		host := "codex"
		if i == 3 {
			host = "claude"
		}
		if _, e := hookNormalize(raw, host, "/selected"); e == nil {
			t.Fatalf("case %d accepted", i)
		}
	}
	raw := hookTestNative("codex", map[string]any{"message": "x"})
	var m map[string]any
	_ = json.Unmarshal(raw, &m)
	m["cwd"] = "/other"
	raw, _ = json.Marshal(m)
	if _, e := hookNormalize(raw, "codex", "/selected"); e == nil {
		t.Fatal("cwd mismatch accepted")
	}
}
func TestHookSamplingOnlyRateChangesSelection(t *testing.T) {
	key := strings.Repeat("a", 64)
	unit := hookSamplingUnit{Version: "harnessctl.advice-sample/1", Host: "codex", ProjectID: "p", ControlID: "advice", InstanceID: "instance", SessionID: "s", ToolUseID: "tool"}
	if hookAdviceSelected(key, unit, 0) || !hookAdviceSelected(key, unit, 1000000) {
		t.Fatal("endpoint")
	}
	one := hookAdviceSelected(key, unit, 420000)
	for i := 0; i < 10; i++ {
		if hookAdviceSelected(key, unit, 420000) != one {
			t.Fatal("unstable bucket")
		}
	}
}
func TestHookControllerOptionalAndReferenceUniqueness(t *testing.T) {
	base, e := filepath.EvalSymlinks(t.TempDir())
	if e != nil {
		t.Fatal(e)
	}
	path := filepath.Join(base, "profile")
	if e = os.WriteFile(path, []byte("fixture"), 0600); e != nil {
		t.Fatal(e)
	}
	c := maintenanceController{Projects: []maintenanceProject{{ID: "p"}}}
	if !hookControllerValid(c) {
		t.Fatal("legacy controller")
	}
	c.Hooks = []hookReference{{ProjectID: "p", Host: "codex", Path: path, SHA256: strings.Repeat("a", 64)}}
	if !hookControllerValid(c) {
		t.Fatal("valid reference")
	}
	c.Hooks = append(c.Hooks, c.Hooks[0])
	if hookControllerValid(c) {
		t.Fatal("duplicate selected ref")
	}
}
func TestHookProfileAndControlBindings(t *testing.T) {
	base, e := filepath.EvalSymlinks(t.TempDir())
	if e != nil {
		t.Fatal(e)
	}
	for _, name := range []string{"eval", "policy"} {
		if e = os.WriteFile(filepath.Join(base, name), []byte("fixture"), 0700); e != nil {
			t.Fatal(e)
		}
	}
	ctx := maintenanceContext{HostID: "host", ProjectID: "project", PrincipalID: "person", Stamp: maintenanceHostStamp{UTCMicros: time.Date(2026, 10, 24, 12, 0, 0, 0, time.UTC).UnixMicro()}}
	ref := hookReference{ProjectID: "project", Host: "codex"}
	p := hookProfile{Version: "harnessctl.spawn-profile/1", Profile: "spawn-economics/1", IssuedAt: "2026-10-24T11:00:00Z", ExpiresAt: "2026-10-24T13:00:00Z", Host: "codex", HostID: "host", ProjectID: "project", PrincipalID: "person", RequiredControlID: "required", AdvisoryControlID: "advice", SamplingKey: strings.Repeat("a", 64), Evaluator: hookEvaluator{Profile: "agent-hooks/spawn-eval/1", Executable: filepath.Join(base, "eval"), SHA256: strings.Repeat("b", 64), TimeoutMillis: 100}, Policy: hookPolicy{Path: filepath.Join(base, "policy"), SHA256: strings.Repeat("c", 64)}, MaxBoundaryMillis: 500}
	if !hookProfileValid(p, ref, ctx) {
		t.Fatal("valid profile")
	}
	raw, _ := json.Marshal(p)
	if !hookCueAccepts(t, "#HookProfile", raw) {
		t.Fatal("CUE rejected valid profile")
	}
	var profileFields map[string]any
	if json.Unmarshal(raw, &profileFields) != nil {
		t.Fatal("profile JSON")
	}
	for field := range profileFields {
		copy := map[string]any{}
		for k, v := range profileFields {
			if k != field {
				copy[k] = v
			}
		}
		missing, _ := json.Marshal(copy)
		var parsed hookProfile
		if strictJSON(missing, &parsed) == nil || hookCueAccepts(t, "#HookProfile", missing) {
			t.Fatalf("missing profile %s accepted", field)
		}
	}
	for _, parent := range []string{"evaluator", "policy"} {
		nested := profileFields[parent].(map[string]any)
		for field := range nested {
			copy := map[string]any{}
			for k, v := range nested {
				if k != field {
					copy[k] = v
				}
			}
			profileFields[parent] = copy
			missing, _ := json.Marshal(profileFields)
			var parsed hookProfile
			if strictJSON(missing, &parsed) == nil || hookCueAccepts(t, "#HookProfile", missing) {
				t.Fatalf("missing profile %s.%s accepted", parent, field)
			}
			profileFields[parent] = nested
		}
	}
	p.AdvisoryControlID = p.RequiredControlID
	raw, _ = json.Marshal(p)
	if hookProfileValid(p, ref, ctx) || hookCueAccepts(t, "#HookProfile", raw) {
		t.Fatal("same control accepted")
	}
	p.AdvisoryControlID = "advice"
	controls := []effectiveControl{{ID: "required", Event: "before-tool", Class: "mandatory", SamplePPM: 1000000, OnFault: "hold", MaxWorkMillis: 100, AllowMaintenance: false}, {ID: "advice", Event: "before-tool", Class: "advisory", SamplePPM: 0, OnFault: "hold", MaxWorkMillis: 100, AllowMaintenance: true}}
	if _, _, ok := hookControls(controls, p); !ok {
		t.Fatal("valid controls")
	}
	controls[0].SamplePPM = 999999
	if _, _, ok := hookControls(controls, p); ok {
		t.Fatal("mandatory sampled")
	}
}
func TestHookReportCUEAndDefaultNoIO(t *testing.T) {
	r := hookBaseReport()
	r.Code = "unsupported"
	if hookValidateReport(r) != nil {
		t.Fatal("base")
	}
	cue, e := exec.LookPath("cue")
	if e == nil {
		path := filepath.Join(t.TempDir(), "report.json")
		raw, _ := json.Marshal(r)
		if e = os.WriteFile(path, raw, 0600); e != nil {
			t.Fatal(e)
		}
		out, e := exec.Command(cue, "vet", "-c", "schema.cue", "maintenance_schema.cue", "hooks_schema.cue", path, "-d", "#HookReport").CombinedOutput()
		if e != nil {
			t.Fatalf("CUE: %v %s", e, out)
		}
	}
	var out bytes.Buffer
	base, e := filepath.EvalSymlinks(t.TempDir())
	if e != nil {
		t.Fatal(e)
	}
	controller := filepath.Join(base, "controller")
	input := filepath.Join(base, "input")
	for _, path := range []string{controller, input} {
		if e = os.WriteFile(path, []byte("unused"), 0600); e != nil {
			t.Fatal(e)
		}
	}
	code := runHooksWithDeps([]string{"evaluate", "--controller", controller, "--controller-sha256", strings.Repeat("a", 64), "--project", base, "--host", "codex", "--input", input}, &out, &bytes.Buffer{}, hookDeps{})
	if code != 2 || !bytes.Contains(out.Bytes(), []byte(`"code":"unsupported"`)) {
		t.Fatalf("code=%d output=%s", code, out.String())
	}
}

func hookCueAccepts(t *testing.T, definition string, raw []byte) bool {
	t.Helper()
	cue, e := exec.LookPath("cue")
	if e != nil {
		t.Skip("cue unavailable")
	}
	path := filepath.Join(t.TempDir(), "record.json")
	if e = os.WriteFile(path, raw, 0600); e != nil {
		t.Fatal(e)
	}
	_, e = exec.Command(cue, "vet", "-c", "schema.cue", "maintenance_schema.cue", "hooks_schema.cue", path, "-d", definition).CombinedOutput()
	return e == nil
}
func TestHookReportSchemaRelationships(t *testing.T) {
	base := hookBaseReport()
	valid := []hookReport{base}
	pass := base
	pass.Status = "complete"
	pass.Code = "ok"
	pass.RequiredDecision = "pass"
	pass.Codes = []string{"allowed"}
	pass.Class = "worker"
	pass.ClassSource = "default"
	pass.NativeStampAttempts = 3
	pass.NativeStampInvoked = 3
	pass.NativeStampCompleted = 3
	pass.EvaluatorAttempts = 1
	pass.EvaluatorInvoked = 1
	pass.EvaluatorCompleted = 1
	valid = append(valid, pass)
	selected := pass
	selected.AdvisorySelected = true
	selected.AdvisoryEvaluated = true
	selected.AdviceMatches = 2
	valid = append(valid, selected)
	deny := pass
	deny.Code = "spawn_denied"
	deny.RequiredDecision = "deny"
	deny.Codes = []string{"no_model", "fork_unmarked"}
	deny.Class = "critic"
	deny.ClassSource = "critic_marker"
	valid = append(valid, deny)
	resume := base
	resume.Status = "not-applicable"
	resume.Code = "resume_not_spawn"
	resume.RequiredDecision = "not-applicable"
	resume.Codes = []string{"resume_not_spawn"}
	resume.Class = "not-applicable"
	resume.ClassSource = "not-applicable"
	resume.NativeStampAttempts = 3
	resume.NativeStampInvoked = 3
	resume.NativeStampCompleted = 3
	resume.EvaluatorAttempts = 1
	resume.EvaluatorInvoked = 1
	resume.EvaluatorCompleted = 1
	valid = append(valid, resume)
	invalid := []hookReport{}
	bad := pass
	bad.AdvisorySelected = true
	invalid = append(invalid, bad)
	bad = deny
	bad.Codes = []string{"allowed"}
	invalid = append(invalid, bad)
	bad = deny
	bad.ClassSource = "default"
	invalid = append(invalid, bad)
	bad = base
	bad.Code = "ok"
	invalid = append(invalid, bad)
	for i, v := range valid {
		raw, _ := json.Marshal(v)
		if hookValidateReport(v) != nil || !hookCueAccepts(t, "#HookReport", raw) {
			t.Fatalf("valid %d rejected: %s", i, raw)
		}
	}
	for i, v := range invalid {
		raw, _ := json.Marshal(v)
		if hookValidateReport(v) == nil || hookCueAccepts(t, "#HookReport", raw) {
			t.Fatalf("invalid %d accepted: %s", i, raw)
		}
	}
}

func TestHookReportRequiredFieldsParity(t *testing.T) {
	base := hookBaseReport()
	base.Code = "unsupported"
	pass := base
	pass.Status = "complete"
	pass.Code = "ok"
	pass.RequiredDecision = "pass"
	pass.Codes = []string{"allowed"}
	pass.Class = "worker"
	pass.ClassSource = "default"
	pass.NativeStampAttempts = 3
	pass.NativeStampInvoked = 3
	pass.NativeStampCompleted = 3
	pass.EvaluatorAttempts = 1
	pass.EvaluatorInvoked = 1
	pass.EvaluatorCompleted = 1
	for _, report := range []hookReport{base, pass} {
		raw, _ := json.Marshal(report)
		var fields map[string]any
		_ = json.Unmarshal(raw, &fields)
		for field := range fields {
			copy := map[string]any{}
			for k, v := range fields {
				if k != field {
					copy[k] = v
				}
			}
			missing, _ := json.Marshal(copy)
			var decoded hookReport
			if strictJSON(missing, &decoded) == nil || hookCueAccepts(t, "#HookReport", missing) {
				t.Fatalf("omitted %s accepted for %s", field, report.Status)
			}
		}
	}
}
func TestHookControllerReferenceRequiredParity(t *testing.T) {
	f := newHookReceiverFixture(t)
	raw, e := os.ReadFile(f.base.controllerPath)
	if e != nil {
		t.Fatal(e)
	}
	var doc map[string]any
	if json.Unmarshal(raw, &doc) != nil {
		t.Fatal("controller")
	}
	if !hookCueAccepts(t, "#MaintenanceController", raw) {
		t.Fatal("valid controller CUE")
	}
	original := doc["hooks"]
	delete(doc, "hooks")
	without, _ := json.Marshal(doc)
	var c maintenanceController
	if strictJSON(without, &c) != nil || !hookControllerValid(c) || !hookCueAccepts(t, "#MaintenanceController", without) {
		t.Fatal("legacy controller rejected")
	}
	doc["hooks"] = []any{}
	empty, _ := json.Marshal(doc)
	if hookCueAccepts(t, "#MaintenanceController", empty) {
		t.Fatal("empty hooks CUE accepted")
	}
	doc["hooks"] = original
	refs := original.([]any)
	ref := refs[0].(map[string]any)
	for field := range ref {
		copy := map[string]any{}
		for k, v := range ref {
			if k != field {
				copy[k] = v
			}
		}
		doc["hooks"] = []any{copy}
		broken, _ := json.Marshal(doc)
		var parsed maintenanceController
		if strictJSON(broken, &parsed) == nil || hookCueAccepts(t, "#MaintenanceController", broken) {
			t.Fatalf("missing hook %s accepted", field)
		}
	}
}

type hookMemoryStore struct {
	state      *maintenanceState
	commits    int
	failCommit bool
}

func (s *hookMemoryStore) Load() (*maintenanceState, error) { return maintenanceClone(s.state), nil }
func (s *hookMemoryStore) Commit(next maintenanceState, _ string) error {
	if s.failCommit {
		return errMaintenanceStoreIncomplete
	}
	s.state = maintenanceClone(&next)
	s.commits++
	return nil
}
func (s *hookMemoryStore) Recover(maintenanceState) error { return errMaintenanceStoreInvalid }
func (s *hookMemoryStore) Close() error                   { return nil }
func TestHookStateCheckCommitsRollbackLock(t *testing.T) {
	ctx, _ := engineContext(t)
	store := &hookMemoryStore{state: engineInit(t, ctx)}
	r := hookBaseReport()
	state, controls, code := hookStateCheck(&r, store, ctx)
	if code != "" || state == nil || len(controls) == 0 || store.commits != 0 {
		t.Fatalf("baseline: %s %+v", code, r)
	}
	ctx.Stamp.UTCMicros--
	state, controls, code = hookStateCheck(&r, store, ctx)
	if code != "clock_invalid" || state != nil || controls != nil || store.state.Phase != "locked" || store.commits != 1 || r.StateWritesAttempted != 1 || r.StateWritesCommitted != 1 {
		t.Fatalf("rollback: %s %+v %+v", code, r, store.state)
	}
}
