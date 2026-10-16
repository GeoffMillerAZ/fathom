package toolexecutionpolicy_test

import (
	"strings"
	"testing"

	toolexecutionpolicy "github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/environment/tool-execution-policy"
)

// ── ToolPolicy.Validate ───────────────────────────────────────────────────────

func TestToolPolicy_Validate_valid(t *testing.T) {
	cases := []toolexecutionpolicy.ToolPolicy{
		{Tool: "web_search", Allow: true, Environments: []string{"dev", "staging"}},
		{Tool: "file_delete", Allow: false, Environments: []string{"prod"}},
		{Tool: "code_runner", Allow: true, Environments: []string{}}, // empty list is valid
		{Tool: "tool_x", Allow: false, Environments: nil},            // nil list is valid
	}
	for _, p := range cases {
		if err := p.Validate(); err != nil {
			t.Errorf("Validate(%+v): unexpected error: %v", p, err)
		}
	}
}

func TestToolPolicy_Validate_invalid(t *testing.T) {
	cases := []struct {
		name    string
		policy  toolexecutionpolicy.ToolPolicy
		wantErr string
	}{
		{
			"empty Tool",
			toolexecutionpolicy.ToolPolicy{Tool: "", Allow: true, Environments: []string{"dev"}},
			"Tool must be non-empty",
		},
		{
			"whitespace Tool",
			toolexecutionpolicy.ToolPolicy{Tool: "   ", Allow: true, Environments: []string{"dev"}},
			"Tool must be non-empty",
		},
		{
			"empty env name in list",
			toolexecutionpolicy.ToolPolicy{Tool: "t", Allow: true, Environments: []string{"dev", ""}},
			"Environments[1] must be non-empty",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.policy.Validate()
			if err == nil {
				t.Fatal("expected a validation error, got nil")
			}
			if !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("error %q does not contain %q", err.Error(), tc.wantErr)
			}
		})
	}
}

// ── NewPolicySet ──────────────────────────────────────────────────────────────

func TestNewPolicySet_valid(t *testing.T) {
	policies := []toolexecutionpolicy.ToolPolicy{
		{Tool: "web_search", Allow: true, Environments: []string{"dev"}},
	}
	ps, err := toolexecutionpolicy.NewPolicySet(policies)
	if err != nil {
		t.Fatalf("NewPolicySet: unexpected error: %v", err)
	}
	if ps == nil {
		t.Fatal("NewPolicySet: returned nil PolicySet")
	}
}

func TestNewPolicySet_invalid(t *testing.T) {
	policies := []toolexecutionpolicy.ToolPolicy{
		{Tool: "", Allow: true, Environments: []string{"dev"}}, // invalid
	}
	_, err := toolexecutionpolicy.NewPolicySet(policies)
	if err == nil {
		t.Fatal("expected error for invalid policy")
	}
}

// ── PolicySet.Decide ──────────────────────────────────────────────────────────

func TestDecide_denyByDefault(t *testing.T) {
	ps, _ := toolexecutionpolicy.NewPolicySet(nil)

	dec, err := ps.Decide("any_tool", "dev")
	if err != nil {
		t.Fatalf("Decide: unexpected error: %v", err)
	}
	if dec.Allowed {
		t.Error("expected Allowed=false (deny-by-default), got true")
	}
	if !strings.Contains(dec.Reason, "deny-by-default") {
		t.Errorf("Reason %q should mention deny-by-default", dec.Reason)
	}
}

func TestDecide_explicitAllow(t *testing.T) {
	ps, _ := toolexecutionpolicy.NewPolicySet([]toolexecutionpolicy.ToolPolicy{
		{Tool: "web_search", Allow: true, Environments: []string{"dev", "staging"}},
	})

	dec, err := ps.Decide("web_search", "dev")
	if err != nil {
		t.Fatalf("Decide: %v", err)
	}
	if !dec.Allowed {
		t.Errorf("expected Allowed=true, got false; reason: %s", dec.Reason)
	}
}

func TestDecide_explicitAllowOtherEnv(t *testing.T) {
	ps, _ := toolexecutionpolicy.NewPolicySet([]toolexecutionpolicy.ToolPolicy{
		{Tool: "web_search", Allow: true, Environments: []string{"dev"}},
	})

	// Same tool, but environment not in the allow list → deny-by-default.
	dec, err := ps.Decide("web_search", "prod")
	if err != nil {
		t.Fatalf("Decide: %v", err)
	}
	if dec.Allowed {
		t.Errorf("expected Allowed=false for env not in allow list, got true")
	}
}

func TestDecide_explicitDeny(t *testing.T) {
	ps, _ := toolexecutionpolicy.NewPolicySet([]toolexecutionpolicy.ToolPolicy{
		{Tool: "file_delete", Allow: false, Environments: []string{"prod"}},
	})

	dec, err := ps.Decide("file_delete", "prod")
	if err != nil {
		t.Fatalf("Decide: %v", err)
	}
	if dec.Allowed {
		t.Error("expected Allowed=false from explicit deny rule, got true")
	}
	if !strings.Contains(dec.Reason, "explicit deny") {
		t.Errorf("Reason %q should mention explicit deny", dec.Reason)
	}
}

func TestDecide_firstMatchWins(t *testing.T) {
	// Allow rule listed first, deny second — allow should win.
	ps, _ := toolexecutionpolicy.NewPolicySet([]toolexecutionpolicy.ToolPolicy{
		{Tool: "web_search", Allow: true, Environments: []string{"dev"}},
		{Tool: "web_search", Allow: false, Environments: []string{"dev"}},
	})

	dec, err := ps.Decide("web_search", "dev")
	if err != nil {
		t.Fatalf("Decide: %v", err)
	}
	if !dec.Allowed {
		t.Errorf("expected first rule (allow) to win, got Allowed=false; reason: %s", dec.Reason)
	}
}

func TestDecide_emptyTool(t *testing.T) {
	ps, _ := toolexecutionpolicy.NewPolicySet(nil)
	_, err := ps.Decide("", "dev")
	if err == nil {
		t.Fatal("expected error for empty tool")
	}
}

func TestDecide_emptyEnv(t *testing.T) {
	ps, _ := toolexecutionpolicy.NewPolicySet(nil)
	_, err := ps.Decide("web_search", "")
	if err == nil {
		t.Fatal("expected error for empty env")
	}
}

func TestDecide_deterministic(t *testing.T) {
	ps, _ := toolexecutionpolicy.NewPolicySet([]toolexecutionpolicy.ToolPolicy{
		{Tool: "analyzer", Allow: true, Environments: []string{"staging"}},
	})

	// Call Decide multiple times; result must be stable.
	for i := 0; i < 20; i++ {
		dec, err := ps.Decide("analyzer", "staging")
		if err != nil {
			t.Fatalf("iteration %d: Decide error: %v", i, err)
		}
		if !dec.Allowed {
			t.Errorf("iteration %d: expected Allowed=true", i)
		}
	}
}
