// Package toolexecutionpolicy governs which tools an agent may call in a given
// runtime environment. The policy is deny-by-default: a tool is allowed only
// when an explicit allow rule matches both the tool name and the environment.
// Decisions are deterministic — given the same PolicySet and inputs, Decide
// always returns the same Decision.
//
// CUE is authoritative for the shape of ToolPolicy; Go is authoritative for
// PolicySet.Decide behaviour.
//
// Concurrency contract: PolicySet is immutable after construction and safe for
// concurrent use; Decide reads only the fixed policy slice and has no mutable
// state.
package toolexecutionpolicy

import (
	"errors"
	"fmt"
	"strings"
)

// ToolPolicy is a single policy rule that either allows or denies a named tool
// in one or more named environments. It mirrors #ToolPolicy in schema.cue.
type ToolPolicy struct {
	// Tool is the name of the tool this rule applies to. Must be non-empty.
	Tool string `json:"Tool"`

	// Allow specifies whether this rule permits (true) or explicitly denies
	// (false) tool execution when it matches.
	Allow bool `json:"Allow"`

	// Environments is the list of environment names to which this rule applies.
	// Each entry must be non-empty. An empty slice means the rule matches no
	// environment and is effectively inert.
	Environments []string `json:"Environments"`
}

// Validate enforces the same constraints that #ToolPolicy encodes in
// schema.cue. It returns a non-nil error when any constraint is violated.
func (p ToolPolicy) Validate() error {
	var errs []string

	if strings.TrimSpace(p.Tool) == "" {
		errs = append(errs, "Tool must be non-empty")
	}
	for i, env := range p.Environments {
		if strings.TrimSpace(env) == "" {
			errs = append(errs, fmt.Sprintf("Environments[%d] must be non-empty", i))
		}
	}

	if len(errs) > 0 {
		return errors.New("ToolPolicy: " + strings.Join(errs, "; "))
	}
	return nil
}

// Decision is the outcome of a PolicySet.Decide call.
type Decision struct {
	// Allowed is true when the tool is permitted to execute.
	Allowed bool

	// Reason is a human-readable explanation of why the decision was made.
	Reason string
}

// PolicySet is an ordered collection of ToolPolicy rules evaluated in
// declaration order. The first matching allow rule wins; deny-by-default
// applies if no allow rule matches.
type PolicySet struct {
	policies []ToolPolicy
}

// NewPolicySet constructs a PolicySet from the given rules, validating each
// rule. It returns an error if any rule is invalid.
func NewPolicySet(policies []ToolPolicy) (*PolicySet, error) {
	for i, p := range policies {
		if err := p.Validate(); err != nil {
			return nil, fmt.Errorf("NewPolicySet: policy[%d]: %w", i, err)
		}
	}
	// Defensive copy to prevent external mutation.
	cp := make([]ToolPolicy, len(policies))
	copy(cp, policies)
	return &PolicySet{policies: cp}, nil
}

// Decide evaluates tool against the PolicySet for the named environment.
// Rules are checked in declaration order; the first matching allow rule
// causes an Allowed=true decision. If no allow rule matches the combination,
// the call is denied (deny-by-default).
//
// tool and env must be non-empty; Decide returns an error otherwise.
func (ps *PolicySet) Decide(tool, env string) (Decision, error) {
	if strings.TrimSpace(tool) == "" {
		return Decision{}, errors.New("Decide: tool must be non-empty")
	}
	if strings.TrimSpace(env) == "" {
		return Decision{}, errors.New("Decide: env must be non-empty")
	}

	for _, p := range ps.policies {
		if p.Tool != tool {
			continue
		}
		for _, e := range p.Environments {
			if e != env {
				continue
			}
			// Rule matches: honour the Allow flag.
			if p.Allow {
				return Decision{
					Allowed: true,
					Reason:  fmt.Sprintf("explicit allow rule matched: tool=%q env=%q", tool, env),
				}, nil
			}
			// Explicit deny.
			return Decision{
				Allowed: false,
				Reason:  fmt.Sprintf("explicit deny rule matched: tool=%q env=%q", tool, env),
			}, nil
		}
	}

	// No rule matched — deny by default.
	return Decision{
		Allowed: false,
		Reason:  fmt.Sprintf("deny-by-default: no allow rule for tool=%q env=%q", tool, env),
	}, nil
}
