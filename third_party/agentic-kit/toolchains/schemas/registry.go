package schemas

// Registry lists the canonical CUE schema package paths that agentic-kit
// governs.  Tooling uses this list to:
//   - Run `cue vet` over every schema surface in CI.
//   - Detect drift between Go types and their CUE counterparts.
//   - Enumerate all governed modules for documentation generation.
//
// Entries are module-relative import paths (relative to the CUE module root at
// the repository root).  They are kept in alphabetical order.
var Registry = []string{
	"toolchains/materialization/workspace-query-builder",
	"toolchains/schemas",
}

// KnownNodeKinds returns the complete set of valid NodeKind values.
// Callers that need to validate user-supplied node kind strings should use
// this rather than a hard-coded switch.
func KnownNodeKinds() []NodeKind {
	return []NodeKind{
		NodeKindAgent,
		NodeKindEvidence,
		NodeKindFinding,
		NodeKindService,
	}
}
