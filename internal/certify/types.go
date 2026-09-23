// Package certify implements the certifications layer (vision 33 /
// ADR-043 stage 1, docs/design/test-certifications.md §3): declared
// per-test and per-suite `certifies` facts, compiled deterministically —
// no model — into one core.SemanticClaim per (target, test), tiered and
// stated exactly like the findings layer's own deterministic bridge
// (internal/findings/external, internal/analysis/review/claims.go): "the
// certifications layer is the second deterministic claim layer after
// findings" (ADR-043 consequences).
//
// fathom asserts a fact; it never judges adequacy (vision 33 §3). Only
// State == Live ever satisfies a gate.
package certify

// Layer/LayerVersion/Kind/PromptVersion/TierProfile are the
// certifications layer's claim identity (design doc §3) — the SAME
// content-addressed-claim shape review's findings bridge uses
// (internal/analysis/review/claims.go's ClaimsLayer/ClaimsKind/…), a
// second instance of the pattern rather than a new mechanism.
const (
	Layer         = "certifications"
	LayerVersion  = "v1"
	Kind          = "certification"
	PromptVersion = "certify/v1"
	TierProfile   = "engineering"
)

// State is the certifications layer's own state ladder (design doc §3) —
// a fact served BESIDE Tier, never folded into it. Only StateLive
// satisfies a gate.
type State string

const (
	// StateLive: the test reaches the target and its last attested run
	// passed, with no flake in the last three attested runs.
	StateLive State = "live"
	// StateFlaky: reach + a passing last run, but a flake appears in the
	// last three attested runs.
	StateFlaky State = "flaky"
	// StateBroken: reach, but the last attested run failed.
	StateBroken State = "broken"
	// StateUnproven: reach, but no attested run has ever been ingested
	// for this test.
	StateUnproven State = "unproven"
	// StateUnreachable: declared, but the test reaches nothing the
	// target names (or the test symbol itself is not in the index).
	StateUnreachable State = "unreachable"
)

// Valid reports whether s is a known State.
func (s State) Valid() bool {
	switch s {
	case StateLive, StateFlaky, StateBroken, StateUnproven, StateUnreachable:
		return true
	}
	return false
}

// Satisfies reports whether s alone is enough to satisfy a gate (design
// doc §3: "Only live satisfies a gate").
func (s State) Satisfies() bool { return s == StateLive }

// Target is what a certification's kind+id resolves to (design doc §1):
// the symbols a test must reach to certify it, or the reason it could not
// resolve at all.
type Target struct {
	Kind string
	ID   string

	// Symbols is the sorted set of canonical symbol keys the target
	// names. Empty for kind "behaviour" (declared only — the caller caps
	// its tier at doc-claimed, design doc §1) and for kind "invariant"/
	// "incident": the row/record itself is what resolves for those two
	// kinds, and neither one binds to a specific code symbol the way
	// spec/rule/port do (see ResolveTarget's own doc comment) — a
	// certificate for one of these two kinds is checked for the target's
	// EXISTENCE only, and reach is vacuously satisfied once the target
	// itself resolves.
	Symbols []string
	// Files is the sorted set of repo-relative files backing Symbols,
	// plus (for invariant/incident) the doc file the target's own record
	// lives in — Scope's second half (design doc §3).
	Files []string
	// Resolved is false when id names nothing the kind's own rule can
	// find (design doc §1: "or the certificate is contradicted").
	Resolved bool
	// Reason explains an unresolved target.
	Reason string
}

// Report is CompileCertifications'/SweepCertifications' return: what each
// wrote, never a silent no-op.
type Report struct {
	Declared      int // declared (target, test) entries considered
	ClaimsWritten int
	// Skipped names an entry this compile could not turn into a claim at
	// all (an ambiguous or absent test symbol) — never silently dropped.
	Skipped []string
}
