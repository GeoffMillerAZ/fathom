// Package policy provides runtime policy enforcement for agentic tool dispatch.
// It wraps foundations/environment/tool-execution-policy and exposes a single
// Enforcer that evaluates a named tool against a named environment and returns
// a Decision.
//
// Enforcement is deny-by-default and deterministic: given the same PolicySet
// and the same (tool, env) pair, Enforcer.Check always returns the same
// Decision.
package policy

import (
	toolexecutionpolicy "github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/environment/tool-execution-policy"
)

// Decision captures the outcome of an Enforcer.Check call.
// It mirrors toolexecutionpolicy.Decision so callers don't need to import the
// foundation package directly.
type Decision struct {
	// Allowed is true when the policy permits the tool to run.
	Allowed bool

	// Reason is a human-readable explanation of the decision.
	Reason string
}

// fromFoundationDecision converts a toolexecutionpolicy.Decision to a policy
// Decision.
func fromFoundationDecision(d toolexecutionpolicy.Decision) Decision {
	return Decision{Allowed: d.Allowed, Reason: d.Reason}
}
