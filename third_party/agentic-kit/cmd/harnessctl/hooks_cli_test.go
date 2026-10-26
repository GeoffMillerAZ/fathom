//go:build harness_maintenance

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/third_party/agentic-kit/internal/hookeval"
)

type hookPeerFixture struct {
	decision    string
	wrongExit   bool
	unknown     bool
	broken      bool
	prefix      bool
	extraLF     bool
	trailing    bool
	closed      bool
	intactValue bool
	closeErr    bool
	calls       int
	seen        []hookeval.Request
	during      func()
}

func (p *hookPeerFixture) Evaluate(_ context.Context, raw []byte, _ time.Duration) ([]byte, bool, bool, int, error) {
	p.calls++
	req, e := hookeval.DecodeRequest(raw)
	if e != nil {
		return nil, true, true, 2, e
	}
	p.seen = append(p.seen, req)
	if p.during != nil {
		p.during()
	}
	result := hookeval.Result{Version: hookeval.ResultVersion, RequestID: req.RequestID, ContextSHA256: req.ContextSHA256, PolicySHA256: req.PolicySHA256, NoActionAuthority: true, RequiredDecision: p.decision, Codes: []string{"allowed"}, Class: "worker", ClassSource: "default", AdvisoryEvaluated: req.AdvisorySelected, AdviceMatches: 0}
	status := 0
	if req.Subject.Operation == "resume" {
		result.RequiredDecision = "not-applicable"
		result.Codes = []string{"resume_not_spawn"}
		result.Class = "not-applicable"
		result.ClassSource = "not-applicable"
		result.AdvisoryEvaluated = false
	}
	if p.decision == "deny" {
		result.Codes = []string{"no_model"}
		result.AdvisoryEvaluated = false
		status = 1
	}
	if p.unknown {
		result.RequiredDecision = "unknown"
		result.Codes = []string{"policy_invalid"}
		result.Class = "unknown"
		result.ClassSource = "unknown"
		result.AdvisoryEvaluated = false
		status = 2
	}
	b, e := hookeval.MarshalResult(result)
	if e != nil {
		return nil, true, true, status, e
	}
	if p.broken {
		b = []byte(`{"secret":"raw brief"}`)
	}
	b = append(b, '\n')
	if p.prefix {
		b = append([]byte("\n"), b...)
	}
	if p.extraLF {
		b = append(b, '\n')
	}
	if p.trailing {
		b = append(b, []byte(`{}`)...)
	}
	if p.wrongExit {
		status = 1 - status
	}
	return b, true, true, status, nil
}
func (p *hookPeerFixture) intact() bool { return p.intactValue }
func (p *hookPeerFixture) Close() error {
	p.closed = true
	if p.closeErr {
		return errors.New("private cleanup failed")
	}
	return nil
}

type hookReceiverFixture struct {
	base       cliFixture
	profile    hookProfile
	policyPath string
	state      *hookMemoryStore
	stamp      maintenanceHostStamp
	peer       *hookPeerFixture
	input      []byte
}

func newHookReceiverFixture(t *testing.T) hookReceiverFixture {
	return newHookReceiverFixtureRate(t, 0)
}
func newHookReceiverFixtureRate(t *testing.T, rate int) hookReceiverFixture {
	t.Helper()
	f := maintenanceCLIFixture(t)
	f.f.reg.Controls[0].OnFault = "hold"
	f.f.reg.Controls[0].DefaultSamplePPM = rate
	f.f.reg.Controls[0].AllowMaintenance = true
	f.f.reg.Controls = append(f.f.reg.Controls, control{ID: "required", Event: "before-tool", Class: "mandatory", MinSamplePPM: 1000000, MaxSamplePPM: 1000000, DefaultSamplePPM: 1000000, MaxWorkMillis: 1000, OnFault: "hold", AllowMaintenance: false, TunableBy: []string{}})
	f.f.reg.Projects[0].ControlIDs = append(f.f.reg.Projects[0].ControlIDs, "required")
	f.f.reg.Projects[0].Layers[0].ControlIDs = append(f.f.reg.Projects[0].Layers[0].ControlIDs, "required")
	f.f.layer.Controls[0].MinSamplePPM = 0
	f.f.layer.Controls[0].MaxSamplePPM = 1000000
	f.f.layer.Controls[0].AllowMaintenance = true
	f.f.layer.Controls = append(f.f.layer.Controls, constraint{ID: "required", MinSamplePPM: 1000000, MaxSamplePPM: 1000000, MaxWorkMillis: 1000, OnFault: "hold", AllowMaintenance: false, TunableBy: []string{}})
	f.f.layer.Preferences[0].SamplePPM = rate
	f.f.save(t)
	base := filepath.Dir(f.controllerPath)
	policyPath := filepath.Join(base, "spawn-policy.json")
	policy := []byte(`{"version":"test-policy"}`)
	if e := os.WriteFile(policyPath, policy, 0600); e != nil {
		t.Fatal(e)
	}
	evaluatorPath := filepath.Join(base, "spawn-evaluator")
	if e := os.WriteFile(evaluatorPath, []byte("test-peer"), 0700); e != nil {
		t.Fatal(e)
	}
	profile := hookProfile{Version: "harnessctl.spawn-profile/1", Profile: "spawn-economics/1", IssuedAt: "2026-10-24T11:00:00Z", ExpiresAt: "2026-10-24T13:00:00Z", Host: "codex", HostID: "host", ProjectID: "project", PrincipalID: "operator", RequiredControlID: "required", AdvisoryControlID: "review", SamplingKey: strings.Repeat("a", 64), Evaluator: hookEvaluator{Profile: "agent-hooks/spawn-eval/1", Executable: evaluatorPath, SHA256: hookDigest([]byte("test-peer")), TimeoutMillis: 1000}, Policy: hookPolicy{Path: policyPath, SHA256: hookDigest(policy)}, MaxBoundaryMillis: 5000}
	profileRaw, _ := json.Marshal(profile)
	profilePath := filepath.Join(base, "spawn-profile.json")
	if e := os.WriteFile(profilePath, profileRaw, 0600); e != nil {
		t.Fatal(e)
	}
	controllerRaw, e := os.ReadFile(f.controllerPath)
	if e != nil {
		t.Fatal(e)
	}
	var c maintenanceController
	if strictJSON(controllerRaw, &c) != nil {
		t.Fatal("controller decode")
	}
	c.Hooks = []hookReference{{ProjectID: "project", Host: "codex", Path: profilePath, SHA256: hookDigest(profileRaw)}}
	controllerRaw, _ = json.Marshal(c)
	if e := os.WriteFile(f.controllerPath, controllerRaw, 0600); e != nil {
		t.Fatal(e)
	}
	f.controllerHash = hookDigest(controllerRaw)
	stamp := f.native.stamp
	initReport, code, _ := cliCall(t, f, "init")
	if code != 0 {
		t.Fatalf("init code %d %+v", code, initReport)
	}
	disk, e := openMaintenanceFileStore(f.stateRoot, nil)
	if e != nil {
		t.Fatal(e)
	}
	state, e := disk.Load()
	if e != nil || state == nil {
		t.Fatalf("state: %v", e)
	}
	if e := disk.Close(); e != nil {
		t.Fatal(e)
	}
	input, _ := json.Marshal(map[string]any{"hook_event_name": "PreToolUse", "session_id": "session", "tool_use_id": "call", "cwd": f.projectPath, "tool_name": "collaborationspawn_agent", "tool_input": map[string]any{"message": "worker task", "model": "m", "reasoning_effort": "high"}})
	return hookReceiverFixture{base: f, profile: profile, policyPath: policyPath, state: &hookMemoryStore{state: state}, stamp: stamp, peer: &hookPeerFixture{decision: "pass", intactValue: true}, input: input}
}
func (f *hookReceiverFixture) deps() hookDeps {
	maintenance := f.base.deps()
	maintenance.newNative = func(maintenanceNative) (maintenanceNativePort, error) {
		return &cliNative{stamp: f.stamp, key: f.base.key, status: "signed"}, nil
	}
	maintenance.newStore = func(string, func(string) error) (maintenanceStorePort, error) { return f.state, nil }
	return hookDeps{maintenance: maintenance, stdin: bytes.NewReader(f.input), now: func() time.Time { return maintenanceStampTime(f.stamp) }, prepare: func(hookEvaluator, string) (hookChildPort, error) { return f.peer, nil }}
}
func (f *hookReceiverFixture) args(op string, t *testing.T) []string {
	a := []string{op, "--controller", f.base.controllerPath, "--controller-sha256", f.base.controllerHash, "--project", f.base.projectPath, "--host", "codex"}
	if op == "evaluate" {
		path := filepath.Join(t.TempDir(), "event.json")
		if e := os.WriteFile(path, f.input, 0600); e != nil {
			t.Fatal(e)
		}
		a = append(a, "--input", path)
	}
	return a
}
func (f *hookReceiverFixture) call(t *testing.T, op string) (hookReport, int, string, string) {
	t.Helper()
	var out, errOut bytes.Buffer
	code := runHooksWithDeps(f.args(op, t), &out, &errOut, f.deps())
	var report hookReport
	if op == "evaluate" {
		if strictJSON(out.Bytes(), &report) != nil {
			t.Fatalf("report: %q", out.String())
		}
	}
	return report, code, out.String(), errOut.String()
}
func TestHookReceiverCurrentPassDenyAndNativeSilence(t *testing.T) {
	f := newHookReceiverFixture(t)
	r, code, out, _ := f.call(t, "evaluate")
	if code != 0 || r.Code != "ok" || r.RequiredDecision != "pass" || r.EvaluatorCompleted != 1 || r.NativeStampCompleted != 3 || r.AdvisorySelected || !f.peer.closed {
		t.Fatalf("pass: %d %+v", code, r)
	}
	if strings.Contains(out, f.base.projectPath) || strings.Contains(out, "worker task") || strings.Contains(out, f.profile.SamplingKey) {
		t.Fatal("private data leaked")
	}
	f.peer = &hookPeerFixture{decision: "deny", intactValue: true}
	r, code, _, _ = f.call(t, "evaluate")
	if code != 1 || r.Code != "spawn_denied" || r.RequiredDecision != "deny" || f.peer.calls != 1 {
		t.Fatalf("deny: %d %+v", code, r)
	}
	f.peer = &hookPeerFixture{decision: "deny", intactValue: true}
	_, code, out, stderr := f.call(t, "run")
	if code != 2 || out != "" || stderr != "harnessctl spawn check blocked\n" || len(f.peer.seen) != 1 || f.peer.seen[0].AdvisorySelected {
		t.Fatalf("native: %d %q %q", code, out, stderr)
	}
	for i := 0; i < 3; i++ {
		f.peer = &hookPeerFixture{decision: "deny", intactValue: true}
		r, code, _, _ = f.call(t, "evaluate")
		if code != 1 || r.RequiredDecision != "deny" {
			t.Fatalf("repeat %d: %+v", i, r)
		}
	}
}
func TestHookReceiverNativeNeverSamplesAdvice(t *testing.T) {
	f := newHookReceiverFixtureRate(t, 1000000)
	r, code, _, _ := f.call(t, "evaluate")
	if code != 0 || !r.AdvisorySelected || !r.AdvisoryEvaluated || !f.peer.seen[0].AdvisorySelected {
		t.Fatalf("operator sample: %d %+v", code, r)
	}
	f.peer = &hookPeerFixture{decision: "pass", intactValue: true}
	_, code, out, stderr := f.call(t, "run")
	if code != 0 || out != "" || stderr != "" || f.peer.seen[0].AdvisorySelected {
		t.Fatalf("native sample: %d %q %q", code, out, stderr)
	}
}
func TestHookReceiverMaintenanceRateZeroKeepsRequiredDenial(t *testing.T) {
	f := newHookReceiverFixture(t)
	candidate := maintenanceCandidate{Version: "harnessctl.advisory-candidate/1", Purpose: "maintenance", ApproverID: "human", DurationSeconds: 30, Reason: "Short diagnostic", Controls: []maintenanceCandidateControl{{ID: "review", SamplePPM: 0}}}
	raw, _ := json.Marshal(candidate)
	if e := os.WriteFile(f.base.candidatePath, raw, 0600); e != nil {
		t.Fatal(e)
	}
	f.base.native = &cliNative{stamp: f.stamp, key: f.base.key, status: "signed"}
	if r, code, _ := cliCall(t, f.base, "prepare"); code != 0 {
		t.Fatalf("prepare: %d %+v", code, r)
	}
	f.base.native = &cliNative{stamp: f.stamp, key: f.base.key, status: "signed"}
	if r, code, _ := cliCall(t, f.base, "enter"); code != 0 {
		t.Fatalf("enter: %d %+v", code, r)
	}
	disk, e := openMaintenanceFileStore(f.base.stateRoot, nil)
	if e != nil {
		t.Fatal(e)
	}
	state, e := disk.Load()
	if e != nil || state == nil {
		t.Fatalf("load: %v", e)
	}
	if e = disk.Close(); e != nil {
		t.Fatal(e)
	}
	f.state.state = state
	f.peer = &hookPeerFixture{decision: "deny", intactValue: true}
	r, code, _, _ := f.call(t, "evaluate")
	if code != 1 || r.RequiredDecision != "deny" || r.AdvisorySelected || f.peer.calls != 1 {
		t.Fatalf("lease denial: %d %+v", code, r)
	}
}
func TestHookReceiverPeerFaultsAndPostChildDrift(t *testing.T) {
	cases := []struct {
		name      string
		configure func(*hookReceiverFixture)
		want      string
	}{
		{"unknown", func(f *hookReceiverFixture) { f.peer.unknown = true }, "evaluator_incomplete"},
		{"wrong exit", func(f *hookReceiverFixture) { f.peer.wrongExit = true }, "evaluator_incomplete"},
		{"malformed", func(f *hookReceiverFixture) { f.peer.broken = true }, "evaluator_incomplete"},
		{"leading newline", func(f *hookReceiverFixture) { f.peer.prefix = true }, "evaluator_incomplete"},
		{"extra newline", func(f *hookReceiverFixture) { f.peer.extraLF = true }, "evaluator_incomplete"},
		{"trailing JSON", func(f *hookReceiverFixture) { f.peer.trailing = true }, "evaluator_incomplete"},
		{"state drift", func(f *hookReceiverFixture) { f.peer.during = func() { f.state.state.Sequence++ } }, "binding_mismatch"},
		{"policy drift", func(f *hookReceiverFixture) {
			f.peer.during = func() { _ = os.WriteFile(f.policyPath, []byte(`{"changed":true}`), 0600) }
		}, "binding_mismatch"},
		{"evaluator drift", func(f *hookReceiverFixture) { f.peer.during = func() { f.peer.intactValue = false } }, "binding_mismatch"},
		{"cleanup fault", func(f *hookReceiverFixture) { f.peer.closeErr = true }, "evaluator_incomplete"},
		{"clock write fault", func(f *hookReceiverFixture) {
			f.stamp.UTCMicros += 1e6
			f.stamp.ContinuousNanos += 1e9
			f.state.failCommit = true
		}, "storage_incomplete"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := newHookReceiverFixture(t)
			tc.configure(&f)
			r, code, out, _ := f.call(t, "evaluate")
			if code != 2 || r.Status != "hold" || r.Code != tc.want || r.RequiredDecision != "" || len(r.Codes) != 0 || strings.Contains(out, "worker task") {
				t.Fatalf("%d %+v %s", code, r, out)
			}
		})
	}
}
