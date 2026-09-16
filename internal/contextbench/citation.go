// citation.go adds a heading-anchored TruthAtHead anchor kind (item 3,
// wave N8): AnchorHeading resolves an ADR record's own "## ADR-nnn …"
// heading to wherever it CURRENTLY sits, instead of pinning a hardcoded
// line number that the next ADR.md insertion above it breaks — the same
// fix internal/scenariobench.ResolveCitationLine makes for a guards-corpus
// rule.source citation, duplicated here rather than imported: this
// package's own TruthAnchor validates by (File, Needle) pairs, not a
// parsed "path#anchor" string, so the two resolvers' inputs differ even
// though the underlying primitive — find the "## ADR-nnn" heading's line
// — is the same search.
package contextbench

import (
	"fmt"
	"strings"
)

// resolveHeadingLine returns the 1-based line number of the FIRST line in
// body that begins with "## " and contains headingID (e.g. "ADR-025") —
// resolved fresh against whatever body holds right now, never cached.
// Errors when no such heading exists.
func resolveHeadingLine(body, headingID string) (int, error) {
	for i, l := range strings.Split(body, "\n") {
		if strings.HasPrefix(l, "## ") && strings.Contains(l, headingID) {
			return i + 1, nil
		}
	}
	return 0, fmt.Errorf("no \"## \" heading containing %q", headingID)
}
