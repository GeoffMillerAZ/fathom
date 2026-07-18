package policy

import (
	"fmt"

	toolexecutionpolicy "github.com/geoffmilleraz/agentic-kit/foundations/environment/tool-execution-policy"
)

// Enforcer evaluates tool-execution policy at runtime.
//
// Construct one with NewEnforcer. All methods are safe for concurrent use
// after construction.
type Enforcer struct {
	ps *toolexecutionpolicy.PolicySet
}

// NewEnforcer builds an Enforcer from the supplied ToolPolicy rules.
// Rules are validated; an error is returned if any rule is structurally
// invalid. The Enforcer is immutable after construction.
func NewEnforcer(rules []toolexecutionpolicy.ToolPolicy) (*Enforcer, error) {
	ps, err := toolexecutionpolicy.NewPolicySet(rules)
	if err != nil {
		return nil, fmt.Errorf("policy.NewEnforcer: %w", err)
	}
	return &Enforcer{ps: ps}, nil
}

// Check evaluates whether tool is permitted to run in the named env.
//
//   - tool and env must be non-empty; a non-nil error is returned otherwise.
//   - Deny-by-default: if no explicit allow rule matches, Allowed=false.
//   - Deterministic: identical inputs always yield identical output.
func (e *Enforcer) Check(tool, env string) (Decision, error) {
	d, err := e.ps.Decide(tool, env)
	if err != nil {
		return Decision{}, fmt.Errorf("policy.Enforcer.Check: %w", err)
	}
	return fromFoundationDecision(d), nil
}
