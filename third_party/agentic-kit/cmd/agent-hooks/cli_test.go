package main

import (
	"os"
	"regexp"
	"strings"
	"testing"
	"time"
)

func TestRunDispatch(t *testing.T) {
	tests := []struct {
		args     []string
		wantCode int
		wantOut  string
	}{
		{nil, 2, ""},
		{[]string{"help"}, 0, "Usage:"},
		{[]string{"version"}, 0, "agent-hooks "},
		{[]string{"bogus"}, 2, ""},
		{[]string{"doctor", "--host", "cursor"}, 2, ""},
		{[]string{"policy"}, 2, ""},
		{[]string{"policy", "bogus"}, 2, ""},
		{[]string{"report", "--days", "0"}, 2, ""},
		{[]string{"print-hooks", "--host", "cursor"}, 2, ""},
	}
	for _, tt := range tests {
		ta := newTestApp(t)
		if code := ta.run(tt.args); code != tt.wantCode || !strings.Contains(ta.out.String(), tt.wantOut) {
			t.Errorf("run(%v) = %d, out %q; want %d containing %q", tt.args, code, ta.out.String(), tt.wantCode, tt.wantOut)
		}
	}
}

func TestBuildRevisionStampWins(t *testing.T) {
	old := buildRevision
	defer func() { buildRevision = old }()
	buildRevision = "v9.9.9-test"
	if got := buildVersion(); got != "v9.9.9-test" {
		t.Fatalf("buildVersion = %q, want the stamped revision", got)
	}
}

func TestPolicyCommands(t *testing.T) {
	ta := newTestApp(t)
	if code := ta.run([]string{"policy", "default"}); code != 0 || ta.out.String() != string(defaultPolicyJSON) {
		t.Fatalf("policy default: exit %d", code)
	}
	ta.out.Reset()
	if code := ta.run([]string{"policy", "check"}); code != 1 {
		t.Fatalf("policy check on an absent file: exit %d, want 1", code)
	}
	ta.writePolicy(t, codexPolicy)
	if code := ta.run([]string{"policy", "check"}); code != 0 || !strings.Contains(ta.out.String(), "is valid") {
		t.Fatalf("policy check on a valid file: exit %d, out %q", code, ta.out.String())
	}
	bad := ta.dir + "/bad.json"
	_ = os.WriteFile(bad, []byte(`{"version":1}`), 0o600)
	if code := ta.run([]string{"policy", "check", bad}); code != 1 || !strings.Contains(ta.errOut.String(), "is invalid") {
		t.Fatalf("policy check on an invalid file: exit %d, err %q", code, ta.errOut.String())
	}
}

func TestStateDirResolution(t *testing.T) {
	ta := newTestApp(t)
	delete(ta.vars, "AGENT_HOOKS_STATE")
	if got := ta.stateDir(); !strings.HasSuffix(got, "home/.local/state/agent-hooks") {
		t.Errorf("default state dir = %q", got)
	}
	ta.vars["XDG_STATE_HOME"] = "/xdg-state"
	if got := ta.stateDir(); got != "/xdg-state/agent-hooks" {
		t.Errorf("XDG state dir = %q", got)
	}
	ta.vars["CLAUDE_CONFIG_DIR"], ta.vars["CODEX_HOME"] = "/cc", "/cx"
	if ta.claudeHome() != "/cc" || ta.codexHome() != "/cx" {
		t.Errorf("host homes = %q, %q", ta.claudeHome(), ta.codexHome())
	}
}

func TestDoctorFlagsStaleLedgerAndFaults(t *testing.T) {
	ta := newTestApp(t)
	cfg := builtinPolicy().Ledger
	old := fixedNow.Add(-30 * 24 * time.Hour)
	ta.now = func() time.Time { return old }
	ta.appendLedger(cfg, ledgerRecord{Host: hostClaude, Kind: kindSpawn})
	ta.now = func() time.Time { return fixedNow }
	ta.run([]string{"doctor", "--host", hostClaude})
	if !regexp.MustCompile(`WARN\s+ledger\s+newest record is 720h0m0s old`).MatchString(ta.out.String()) {
		t.Fatalf("stale ledger not flagged:\n%s", ta.out.String())
	}

	ta.out.Reset()
	ta.appendLedger(cfg, ledgerRecord{Host: hostClaude, Kind: kindFault, Fault: "kaboom"})
	ta.run([]string{"doctor", "--host", hostClaude})
	if !regexp.MustCompile(`WARN\s+faults \(7d\)\s+1, latest: kaboom`).MatchString(ta.out.String()) {
		t.Fatalf("faults not flagged:\n%s", ta.out.String())
	}
}

func TestDoctorFailsOnUnwritableStateDir(t *testing.T) {
	ta := newTestApp(t)
	ta.vars["AGENT_HOOKS_STATE"] = "/dev/null/state"
	if code := ta.run([]string{"doctor", "--host", hostClaude}); code != 1 || !regexp.MustCompile(`FAIL\s+state dir`).MatchString(ta.out.String()) {
		t.Fatalf("exit %d:\n%s", code, ta.out.String())
	}
}
