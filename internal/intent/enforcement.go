package intent

// enforcement.go — ExtractInvariantEnforcementTests: a single-file,
// content-only reader of invariants.md's enforcement column, for a caller
// that has ONLY that one file's bytes at ONE point in a git history (a
// `git show <ref>:docs/intent/invariants.md` read) rather than a full
// Corpus built by Load's directory walk over a live docs/intent/ tree.
//
// ADR-044 §1.2 says a test is guarded when, AT BASE, it is named in
// invariants.md's enforcement column — among other reasons. Load cannot
// itself read historical content (it globs the working filesystem), so a
// caller judging guardedness strictly at base (internal/conserve's
// integrity plane) needs this file's enforcement-column facts alone,
// exactly the discipline internal/analysis/integrity.
// ExtractScopeEvidenceTests already gives scope.md's Compatibility-surface
// section. parseInvariants itself already operates on content alone (it is
// Load's own per-file parser); this exposes that fact without requiring
// docs/intent/'s other files or a real directory.
func ExtractInvariantEnforcementTests(body []byte) []string {
	c := &Corpus{}
	c.parseInvariants("docs/intent/invariants.md", string(body), func(string) bool { return false })
	seen := map[string]bool{}
	for _, inv := range c.Invariants {
		for _, t := range inv.Tests {
			seen[t] = true
		}
	}
	if len(seen) == 0 {
		return nil
	}
	return sortedKeys(seen)
}
