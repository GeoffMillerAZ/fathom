package policy_test

import (
	"strings"
	"testing"

	toolexecutionpolicy "github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/environment/tool-execution-policy"
	"github.com/geoffmilleraz/fathom/third_party/agentic-kit/runtimes/policy"
)

// ── NewEnforcer ───────────────────────────────────────────────────────────────

func TestNewEnforcer_EmptyRules(t *testing.T) {
	e, err := policy.NewEnforcer(nil)
	if err != nil {
		t.Fatalf("NewEnforcer(nil): unexpected error: %v", err)
	}
	if e == nil {
		t.Fatal("NewEnforcer returned nil Enforcer")
	}
}

func TestNewEnforcer_InvalidRule(t *testing.T) {
	rules := []toolexecutionpolicy.ToolPolicy{
		{Tool: "", Allow: true, Environments: []string{"dev"}}, // invalid: empty Tool
	}
	_, err := policy.NewEnforcer(rules)
	if err == nil {
		t.Fatal("expected error for invalid rule, got nil")
	}
}

// ── Enforcer.Check — deny-by-default ─────────────────────────────────────────

func TestCheck_DenyByDefault(t *testing.T) {
	e, err := policy.NewEnforcer(nil)
	if err != nil {
		t.Fatalf("NewEnforcer: %v", err)
	}
	d, err := e.Check("any-tool", "prod")
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	if d.Allowed {
		t.Error("expected Allowed=false (deny-by-default), got true")
	}
	if !strings.Contains(d.Reason, "deny-by-default") {
		t.Errorf("Reason %q should mention deny-by-default", d.Reason)
	}
}

// ── Enforcer.Check — explicit allow ──────────────────────────────────────────

func TestCheck_ExplicitAllow(t *testing.T) {
	rules := []toolexecutionpolicy.ToolPolicy{
		{Tool: "web_search", Allow: true, Environments: []string{"dev", "staging"}},
	}
	e, _ := policy.NewEnforcer(rules)

	cases := []struct {
		tool    string
		env     string
		allowed bool
	}{
		{"web_search", "dev", true},
		{"web_search", "staging", true},
		{"web_search", "prod", false}, // not in allow list
		{"other_tool", "dev", false},  // not in policy at all
	}
	for _, tc := range cases {
		d, err := e.Check(tc.tool, tc.env)
		if err != nil {
			t.Fatalf("Check(%q,%q): %v", tc.tool, tc.env, err)
		}
		if d.Allowed != tc.allowed {
			t.Errorf("Check(%q,%q): got Allowed=%v, want %v; reason: %s",
				tc.tool, tc.env, d.Allowed, tc.allowed, d.Reason)
		}
	}
}

// ── Enforcer.Check — explicit deny ───────────────────────────────────────────

func TestCheck_ExplicitDeny(t *testing.T) {
	rules := []toolexecutionpolicy.ToolPolicy{
		{Tool: "file_delete", Allow: false, Environments: []string{"prod"}},
	}
	e, _ := policy.NewEnforcer(rules)

	d, err := e.Check("file_delete", "prod")
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	if d.Allowed {
		t.Error("expected Allowed=false from explicit deny, got true")
	}
	if !strings.Contains(d.Reason, "explicit deny") {
		t.Errorf("Reason %q should mention explicit deny", d.Reason)
	}
}

// ── Enforcer.Check — input validation ────────────────────────────────────────

func TestCheck_EmptyTool(t *testing.T) {
	e, _ := policy.NewEnforcer(nil)
	_, err := e.Check("", "dev")
	if err == nil {
		t.Fatal("expected error for empty tool, got nil")
	}
}

func TestCheck_EmptyEnv(t *testing.T) {
	e, _ := policy.NewEnforcer(nil)
	_, err := e.Check("my-tool", "")
	if err == nil {
		t.Fatal("expected error for empty env, got nil")
	}
}

// ── Determinism ───────────────────────────────────────────────────────────────

func TestCheck_Deterministic(t *testing.T) {
	rules := []toolexecutionpolicy.ToolPolicy{
		{Tool: "analyzer", Allow: true, Environments: []string{"staging"}},
	}
	e, _ := policy.NewEnforcer(rules)

	var prev *policy.Decision
	for i := 0; i < 20; i++ {
		d, err := e.Check("analyzer", "staging")
		if err != nil {
			t.Fatalf("iteration %d: Check error: %v", i, err)
		}
		if prev != nil && (d.Allowed != prev.Allowed || d.Reason != prev.Reason) {
			t.Errorf("iteration %d: non-deterministic result: %+v vs %+v", i, d, *prev)
		}
		copy := d
		prev = &copy
	}
}

// ── First-match wins ──────────────────────────────────────────────────────────

func TestCheck_FirstMatchWins(t *testing.T) {
	rules := []toolexecutionpolicy.ToolPolicy{
		{Tool: "web_search", Allow: true, Environments: []string{"dev"}},
		{Tool: "web_search", Allow: false, Environments: []string{"dev"}},
	}
	e, _ := policy.NewEnforcer(rules)

	d, err := e.Check("web_search", "dev")
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	if !d.Allowed {
		t.Errorf("first rule (allow) should win; Reason: %s", d.Reason)
	}
}
