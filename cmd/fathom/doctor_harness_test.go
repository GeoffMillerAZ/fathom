package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/harness"
)

func TestNormalizeDoctorHarness(t *testing.T) {
	cases := map[string]string{"claude": "claude", "codex": "codex", "auto": "auto", "claude-code": "claude", "gemini-cli": "gemini-cli", "cursor": "cursor", "copilot": "copilot", "agents": "agents"}
	for in, want := range cases {
		got, ok := normalizeDoctorHarness(in)
		if !ok || got != want {
			t.Errorf("normalizeDoctorHarness(%q) = %q, %v; want %q", in, got, ok, want)
		}
	}
	if _, ok := normalizeDoctorHarness("bogus"); ok {
		t.Error("bogus must be rejected")
	}
	if doctorHarnessID("claude") != harness.ClaudeCode || doctorHarnessID("codex") != harness.Codex {
		t.Error("doctorHarnessID must invert the label")
	}
}

// TestResolveDoctorHarnessDetectsGemini pins HP-12: auto resolution runs
// through harness.Detect, so a descriptor the pre-seam switch never knew
// about is detected from its own markers.
func TestResolveDoctorHarnessDetectsGemini(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, ".gemini"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".gemini", "settings.json"), []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := resolveDoctorHarness("auto", dir); got != "gemini-cli" {
		t.Errorf("got %q, want gemini-cli", got)
	}
	if got := resolveDoctorHarness("claude-code", dir); got != "claude" {
		t.Errorf("explicit registry id: got %q, want claude", got)
	}
}

func TestCheckGatePosture(t *testing.T) {
	c := checkGatePosture("claude")
	if c.Name != "gate_posture" || c.Status != doctorPass || c.Reason != "advisory, fail-open (§5.3)" {
		t.Errorf("claude gate_posture = %+v", c)
	}
	for _, label := range []string{"codex", "gemini-cli", "agents"} {
		c := checkGatePosture(label)
		if c.Status != doctorPass || !strings.Contains(c.Reason, "fail mode") {
			t.Errorf("%s gate_posture = %+v", label, c)
		}
	}
	if c := checkGatePosture("bogus"); c.Status != doctorSkipped {
		t.Errorf("unknown label must be skipped: %+v", c)
	}
}

// TestCheckMCPRegistrationNonClaudeNeverGuessesPass pins HP-12: a found
// Codex entry is real evidence of a registration, not of a verified
// negotiation — it reads `skipped: unverified`; a harness with no reader
// reads skipped with the descriptor's own add line as the remedy.
func TestCheckMCPRegistrationNonClaudeNeverGuessesPass(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, ".codex"), 0o755); err != nil {
		t.Fatal(err)
	}
	cfg := "[mcp_servers.fathom]\nurl = \"http://127.0.0.1:7581/rpc/agent\"\n"
	if err := os.WriteFile(filepath.Join(dir, ".codex", "config.toml"), []byte(cfg), 0o644); err != nil {
		t.Fatal(err)
	}
	// HP-34: Codex is VERIFIED now (a real session negotiated /rpc/agent on
	// this workstation), so a found entry is a real PASS. The demotion
	// below it exists for the unverified state and must not outlive it —
	// an unverified-forever row makes the verified state unreachable.
	c := checkMCPRegistration(dir, "codex", "http://127.0.0.1:7581")
	if c.Status != doctorPass || !strings.Contains(c.Reason, "registered in") {
		t.Errorf("codex with an entry = %+v; want PASS naming the found entry", c)
	}
	// A missing entry is still a real FAIL — evidence, not a guess.
	if c := checkMCPRegistration(t.TempDir(), "codex", "http://127.0.0.1:7581"); c.Status == doctorPass {
		t.Errorf("codex with no config must never PASS: %+v", c)
	}

	g := checkMCPRegistration(dir, "gemini-cli", "http://127.0.0.1:7581")
	if g.Status != doctorSkipped || !strings.Contains(g.Reason, "unverified") || !strings.Contains(g.Remedy, "httpUrl") || !strings.Contains(g.Remedy, "http://127.0.0.1:7581/rpc/agent") {
		t.Errorf("gemini-cli = %+v", g)
	}
	a := checkMCPRegistration(dir, "agents", "http://127.0.0.1:7581")
	if a.Status != doctorSkipped || !strings.Contains(a.Remedy, "mcp add") {
		t.Errorf("agents = %+v", a)
	}
}

func TestCheckHooksNonClaudeReasons(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	g := checkHooksInstalled(t.TempDir(), "gemini-cli")
	if g.Status != doctorSkipped || !strings.Contains(g.Reason, "harness_hooks") || !strings.Contains(g.Reason, "Gemini CLI") {
		t.Errorf("gemini-cli hooks_installed = %+v", g)
	}
	cu := checkHooksInstalled(t.TempDir(), "cursor")
	if cu.Status != doctorSkipped || !strings.Contains(cu.Reason, "harness_hooks") || !strings.Contains(cu.Reason, "Cursor") {
		t.Errorf("cursor hooks_installed = %+v", cu)
	}
	a := checkHooksFiring(nil, "http://127.0.0.1:1", false, t.TempDir(), "agents")
	if a.Status != doctorSkipped || !strings.Contains(a.Reason, "no hook mechanism") {
		t.Errorf("agents hooks_firing = %+v", a)
	}
	// Codex is wired AND verified (HP-34), so its rows are real verdicts,
	// not pointers at harness_hooks: an unwired repo FAILs, with a remedy
	// that does not prescribe `init` (init cannot write that file).
	c := checkHooksInstalled(t.TempDir(), "codex")
	if c.Status != doctorFail || strings.Contains(c.Reason, "Codex has no") || !strings.Contains(c.Remedy, "fathom harness hooks") {
		t.Errorf("codex hooks_installed = %+v", c)
	}
}

func TestCheckSkillsInstalledAgentsShape(t *testing.T) {
	dir := t.TempDir()
	if err := installSkills(dir, "agents", &bytes.Buffer{}); err != nil {
		t.Fatal(err)
	}
	c := checkSkillsInstalled(dir, "gemini-cli")
	if c.Status != doctorPass || !strings.Contains(c.Reason, "unverified") {
		t.Errorf("gemini-cli skills via .agents/skills = %+v", c)
	}
	if m := checkSkillsInstalled(t.TempDir(), "agents"); m.Status != doctorFail || !strings.Contains(m.Remedy, "--skills=agents") {
		t.Errorf("agents with nothing installed = %+v", m)
	}
}

// TestCmdDoctorAcceptsRegistryID drives doctor with a registry id the
// pre-seam validation rejected: it runs (never exit 2), labels the report
// with the id, and carries the gate_posture row.
func TestCmdDoctorAcceptsRegistryID(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	dir := t.TempDir()
	var out, errb bytes.Buffer
	code := cmdDoctor([]string{"--path", dir, "--service", "svc", "--url", "http://127.0.0.1:1", "--harness", "gemini-cli", "--json"}, &out, &errb)
	if code == 2 {
		t.Fatalf("doctor refused a registry id: %s", errb.String())
	}
	var report struct {
		Harness string `json:"harness"`
		Checks  []struct {
			Name   string `json:"name"`
			Status string `json:"status"`
		} `json:"checks"`
	}
	if err := json.Unmarshal(out.Bytes(), &report); err != nil {
		t.Fatalf("--json: %v\n%s", err, out.String())
	}
	if report.Harness != "gemini-cli" {
		t.Errorf("harness = %q", report.Harness)
	}
	names := map[string]string{}
	for _, c := range report.Checks {
		names[c.Name] = c.Status
	}
	if _, ok := names["gate_posture"]; !ok {
		t.Errorf("gate_posture row missing: %v", names)
	}
	if names["mcp_registration"] == "pass" {
		t.Errorf("mcp_registration must never PASS on an unverified harness: %v", names)
	}
}
