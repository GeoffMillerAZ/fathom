// citation.go adds heading-anchored citations (item 3, wave N8) beside the
// line-anchored form a guards-corpus rule.source has always used
// ("ADR.md#L823"). A line citation breaks on every ADR.md insertion above
// it — guards-m1's ADR-025 citation needed re-deriving twice already
// (wave N7's "guards-m1's ADR-025 citation follows the section after
// ADR-011 and ADR-036 grew the file"). A heading citation
// ("ADR.md#ADR-025", the record's own "## ADR-025 …" heading) names the
// SECTION, never a line inside it, so it resolves correctly regardless of
// how many lines ADR.md gains above or inside that record — resolved
// FRESH, at load time, off whatever the file holds right now, never
// cached or baked into the corpus.
package scenariobench

import (
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"
)

// citationRe parses a "path#Lnn" (line) or "path#ADR-nnn" (ADR record
// heading) citation. The heading form is deliberately scoped to ADR.md's
// own "ADR-NNN" id convention — the shape this item was asked to support
// — not a generic heading-anchor syntax for every possible id convention
// this repo's other governing documents use (docs/intent/invariants.md's
// "| I31 |" table rows are a different citation shape entirely, checked
// by TestGuardsCorpusDetectorsExist's own marker logic, untouched here).
var citationRe = regexp.MustCompile(`^(.+)#(?:L(\d+)|(ADR-\d+))$`)

// ResolveCitationLine resolves source — "path#Lnn" or "path#ADR-nnn" —
// against the tree rooted at root into a concrete (path, 1-based line)
// pair. A line citation returns its own number unchanged (no filesystem
// read needed). A heading citation reads root/path and returns the line
// of the FIRST "## " line containing the heading id verbatim (ADR.md's
// own "## ADR-025 — Title" convention) — resolved fresh on every call, so
// an ADR.md insertion above that heading never invalidates the citation
// the way it would a hardcoded line number.
func ResolveCitationLine(root, source string) (path string, line int, err error) {
	m := citationRe.FindStringSubmatch(source)
	if m == nil {
		return "", 0, fmt.Errorf("citation %q is not \"path#Lnn\" or \"path#ADR-nnn\"", source)
	}
	path = m[1]
	if m[2] != "" { // "Lnn" form: the citation already names its own line.
		n, err := strconv.Atoi(m[2])
		if err != nil {
			return "", 0, fmt.Errorf("citation %q: %w", source, err)
		}
		return path, n, nil
	}
	headingID := m[3] // "ADR-nnn" form.
	data, err := os.ReadFile(root + "/" + path)
	if err != nil {
		return "", 0, fmt.Errorf("citation %q: reading %s: %w", source, path, err)
	}
	for i, l := range strings.Split(string(data), "\n") {
		if strings.HasPrefix(l, "## ") && strings.Contains(l, headingID) {
			return path, i + 1, nil
		}
	}
	return "", 0, fmt.Errorf("citation %q: no \"## %s\" heading in %s", source, headingID, path)
}
