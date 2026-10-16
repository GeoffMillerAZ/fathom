// Package readcoverage is fathom's rendering adapter over the kit's read
// manifest — and nothing more.
//
// The accounting is NOT here. It lives in agentic-kit's
// foundations/telemetry/read-manifest, which the kit generalized from
// fathom's own `internal/readset` (ADR-027, FB-10): a Set records
// `read | absent | unreadable(errno) | skipped(category)` per path,
// first-outcome-wins, and only `unreadable` makes a manifest incomplete.
// Fathom deleted its copy onto that package rather than keep two shapes of
// the same idea — a second implementation of "what could I not see" is
// exactly the drift that makes blindness invisible again.
//
// What stays fathom's is the WORDING of a count over an incomplete
// manifest, because ADR-027 and the CHANGELOG pin the exact string a
// reader is trained on — `≥ 85 (1 source(s) unreadable: <path>: <errno>)`
// — and it carries a fathom-side noun ("invariant(s)", "unproven
// claim(s)") the kit's own LowerBound has no business knowing. Every count
// derived from an incomplete manifest goes through RenderCount; no caller
// formats one itself. That is how "0 invariants" got printed for a
// repository with 37.
package readcoverage

import (
	"fmt"
	"strings"

	readmanifest "github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/telemetry/read-manifest"
)

// Note renders the one-line explanation of what a manifest could not see —
// every failed path with the operating system's own wording — or "" when
// it saw everything. Unbounded on purpose: it is the full accounting.
func Note(m readmanifest.Manifest) string {
	bad := m.Unreadable()
	if len(bad) == 0 {
		return ""
	}
	parts := make([]string, 0, len(bad))
	for _, e := range bad {
		parts = append(parts, fmt.Sprintf("%s: %s", e.Path, e.Outcome.Detail))
	}
	return fmt.Sprintf("%d source(s) unreadable: %s", len(bad), strings.Join(parts, "; "))
}

// RenderCount is how a count derived from a manifest must be written: a
// bare number only when coverage is complete, and a NAMED lower bound
// otherwise.
func RenderCount(m readmanifest.Manifest, n int, noun string) string {
	if m.Complete() {
		return fmt.Sprintf("%d %s", n, noun)
	}
	return fmt.Sprintf("≥ %d %s (%s)", n, noun, Note(m))
}
