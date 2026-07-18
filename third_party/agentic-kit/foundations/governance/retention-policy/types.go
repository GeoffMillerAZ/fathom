// Package retentionpolicy provides pure, WASM-safe evaluation of data-retention
// rules. It decides which namespaces are due for a retention action; callers
// execute the actions.
//
// Design:
//   - stdlib only. No file I/O, no goroutines, no network. Safe for
//     GOOS=wasip1 / GOOS=js compilation.
//   - Injected clock (now func() time.Time). Never calls time.Now() directly.
//   - CUE schema (schema.cue) is the authoritative shape; these Go types mirror
//     it. Drift is a bug.
//   - Decision only: retentionpolicy.Due returns the list of actions that are
//     due given a caller-supplied inventory. The caller owns I/O and execution.
//
// Concurrency: all functions in this package are pure (no shared state); they
// are safe for concurrent use without synchronization.
package retentionpolicy

import "time"

// ActionKind is the type of retention action to perform when a policy fires.
type ActionKind string

const (
	// ActionShred instructs the caller to perform crypto-erasure (shred the
	// namespace key). This is the primary action for PII in append-only stores.
	ActionShred ActionKind = "shred"

	// ActionFlag instructs the caller to raise a human-review flag. Use when
	// automated shredding is unsafe or requires approval.
	ActionFlag ActionKind = "flag"

	// ActionReport instructs the caller to emit a retention audit report. Use
	// for compliance reporting without immediate data modification.
	ActionReport ActionKind = "report"
)

// Validate returns an error if k is not a recognised ActionKind.
func (k ActionKind) Validate() error {
	switch k {
	case ActionShred, ActionFlag, ActionReport:
		return nil
	default:
		return errorf("unknown ActionKind %q; must be shred|flag|report", k)
	}
}

// Policy is a single retention rule. It binds a namespace pattern and data
// class to a maximum age and a prescribed action.
//
// NamespacePrefix and Class are used to match inventory entries:
//   - NamespacePrefix: if non-empty, matches namespaces that have this prefix.
//   - Class: if non-empty, matches inventory entries with this exact class.
//
// A blank NamespacePrefix matches all namespaces. A blank Class matches all
// classes. An entry must satisfy both constraints to be eligible (logical AND).
type Policy struct {
	// Name is a human-readable label for this policy (e.g. "pii-7-year").
	// Must be non-empty.
	Name string

	// NamespacePrefix, if non-empty, restricts the policy to namespaces with
	// this prefix. An empty string matches all namespaces.
	NamespacePrefix string

	// Class, if non-empty, restricts the policy to inventory entries of this
	// exact class. An empty string matches all classes.
	Class string

	// MaxAge is the maximum allowed age for data in matching namespaces. Data
	// older than MaxAge at evaluation time is subject to Action.
	MaxAge time.Duration

	// Action is the retention action to perform when a matching entry exceeds
	// MaxAge.
	Action ActionKind
}

// Validate returns an error if the Policy violates any invariant.
func (p Policy) Validate() error {
	if p.Name == "" {
		return errorf("Policy.Name must not be empty")
	}
	if p.MaxAge <= 0 {
		return errorf("Policy.MaxAge must be > 0 (policy %q)", p.Name)
	}
	return p.Action.Validate()
}

// NamespaceEntry is a caller-supplied fact describing one namespace whose data
// may be subject to retention evaluation. The caller owns the inventory — this
// package never performs I/O to discover namespaces.
type NamespaceEntry struct {
	// Namespace is the namespace identifier. Must be non-empty.
	Namespace string

	// Class is the data sensitivity class stored in this namespace (e.g. "pii").
	// May be empty if unknown or mixed; in that case only policies with a blank
	// Class will match.
	Class string

	// CreatedAt is the time the namespace (or the data stored in it) was created.
	// Due evaluation computes age as now.Sub(CreatedAt).
	CreatedAt time.Time
}

// DueAction is one actionable output from Due. It pairs an inventory entry with
// the first matching Policy and the computed age that triggered it.
type DueAction struct {
	// Entry is the inventory entry that triggered this action.
	Entry NamespaceEntry

	// Policy is the first Policy (in evaluation order) that matched the entry and
	// found it over-age.
	Policy Policy

	// Age is the computed age of the entry at evaluation time (now - CreatedAt).
	Age time.Duration

	// Kind is the prescribed action (same as Policy.Action, surfaced here for
	// caller convenience).
	Kind ActionKind
}
