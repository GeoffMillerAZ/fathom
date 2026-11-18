// Package depgraph provides dependency graphs over caller-supplied units,
// declared edges, transitive dependents, and canonical input hashing.
//
// Materialize and InputKey use content digests and preserve line endings.
// Empty directories are invisible. The exec bit is semantic, and a symlink
// hashes its target path. Callers supply repository-relative, slash-separated,
// NFC paths.
//
// This package and its subpackages are outside the compatibility surface
// until the consumer switch (DEP-P1b) lands. Until then, InputKey stays
// byte-compatible with the source engine.
package depgraph

import (
	"context"
	"io/fs"
)

// Fidelity marks evidence as exact or approximate. Approximate evidence
// demotes the trust of results derived from it.
type Fidelity int

const (
	Exact Fidelity = iota
	Approximate
)

func (f Fidelity) String() string {
	if f == Approximate {
		return "approximate"
	}
	return "exact"
}

// Mode flag bits on a SourceRef. Only these two survive
// canonicalization: everything else about a file's mode is noise that
// would thrash keys across checkouts.
const (
	ModeExec    uint32 = 1 << 0 // the exec bit is semantic
	ModeSymlink uint32 = 1 << 1 // digest is sha256 of the target path string
)

// SourceRef is one canonicalized file in a unit's source closure.
// Path is repo-relative, slash-separated, NFC-normalized. Digest is
// sha256 of the file's bytes — or of the symlink's target path string
// when ModeSymlink is set. Neither mtime nor size ever appears here.
type SourceRef struct {
	Path   string
	Mode   uint32
	Digest []byte
}

// UnitRef names a unit across ecosystems, for cross-ecosystem edges.
type UnitRef struct {
	Ecosystem string
	Name      string
}

// Unit is one keyable node of an ecosystem's graph — a Go package, a
// CUE instance, a task-graph node, a glob-mapped component. Unit is not
// a directory: the resolver, not the caller, decides a unit's files
// (a _test.go file belongs only to the test unit, so editing a test
// never re-proves the build).
type Unit struct {
	Ecosystem string
	Name      string
	Sources   []SourceRef
	// Deps are same-ecosystem dependencies by unit name.
	Deps []string
	// LockDigests pins the unit's external dependency evidence
	// (go.sum digest, cue.mod vendored digests), name → digest.
	LockDigests map[string]string
}

// Edge is a cross-ecosystem dependency — a Go unit embedding a CUE
// instance, a timoni module consuming an image digest. Declared today,
// detected later (declare first, detect second): a missing edge is
// under-keying by another name.
type Edge struct {
	From UnitRef
	To   UnitRef
}

// EdgeDecl is a declared cross-ecosystem edge as it appears in plan
// data, before resolution against real units.
type EdgeDecl struct {
	From UnitRef
	To   UnitRef
}

// Graph is the DepGraph result: units, edges and lock digests.
type Graph struct {
	Units []Unit
	Edges []Edge
	// LockDigests aggregates every unit's lock evidence, keyed
	// "<ecosystem>/<name>" per lock entry name.
	LockDigests map[string]string
}

// Profile carries the caller's resolution context. Dir is the on-disk
// location of the tree passed as an fs.FS, for resolvers that need it.
// Affected maps components to path globs for approximate resolution.
type Profile struct {
	Dir      string
	Affected map[string][]string
}

// Resolver materializes an ecosystem's units. Implementations must return
// the same units in the same order for identical inputs.
type Resolver interface {
	// Ecosystem names the resolver's ecosystem.
	Ecosystem() string
	// Units materializes the ecosystem's units from the tree.
	Units(ctx context.Context, root fs.FS, p Profile) ([]Unit, error)
	// Fidelity is the honesty mark; Approximate demotes trust.
	Fidelity() Fidelity
}

// Key is a lowercase-hex sha256 canonical input hash.
type Key string

// KeyInputs is everything InputKey hashes, and nothing it doesn't.
// The deliberate exclusions are the contract's other half: no wall
// clock, no branch, no commit SHA, no store URL, no tier, no semver —
// the caller cannot smuggle them in because there is no field to put
// them in.
type KeyInputs struct {
	// Domain separates key families. Changing it rekeys all inputs in
	// the family.
	Domain string
	// Epoch is bumped only when derivation or render semantics
	// change — never on a routine release.
	Epoch    uint32
	Task     string
	Platform string
	Sources  []SourceRef
	// Deps are upstream KEYS, not content — a leaf edit rekeys each
	// downstream exactly once.
	Deps                []string
	ToolchainLockDigest string
	CueLang             string
	// ToolchainPackages is name → resolved store hash, hashed sorted.
	ToolchainPackages map[string]string
	// Env is DECLARED env only.
	Env  map[string]string
	Args []string
	// SecretNames are the declared secret names — names enter the
	// key, values never (a rotation must not re-key the world; a
	// rename should).
	SecretNames []string
}
