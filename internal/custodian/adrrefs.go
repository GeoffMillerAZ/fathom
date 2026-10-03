package custodian

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/geoffmilleraz/fathom/internal/fsfault"
)

// adrrefs.go — EP-14, the kind that would have caught INC-14.
//
// On 2026-08-26 a design document declared itself "adopted (ADR-026)" and
// three intent documents cited that record while ADR.md had no such
// section. A design panel caught it; nothing mechanical would have. The
// citation is the cheapest possible check — a regex and a set — and it is
// exactly the "instructed means true" shape the custodian already runs for
// agent configuration.
//
// Scope is every document that CITES decisions: docs/design/*.md status
// lines, docs/intent/**, CHANGELOG.md, and docs/plans/*.md. A citation to a
// record that does not exist is a violation; a repo with no decision
// records at all is unable-to-check, never a violation.

var (
	adrCiteRe   = regexp.MustCompile(`\bADR-(\d+)\b`)
	adrRecordRe = regexp.MustCompile(`(?m)^##\s+ADR-(\d+)\b`)
	adrFileRe   = regexp.MustCompile(`(?i)^(?:adr-)?(\d+)[-_.]`)
)

// checkADRReferencesResolve implements KindADRReferencesResolve.
func checkADRReferencesResolve(root string, c Concern) Finding {
	f := Finding{Concern: c, Status: StatusHolds}

	records, home := knownADRIDs(root)
	if len(records) == 0 {
		f.Status = StatusUnable
		f.Detail = "no decision records found (looked for ADR.md sections, docs/adr/*.md and docs/intent/decisions/*.md) — nothing to resolve citations against"
		return f
	}

	// Only citations written in THIS repo's id shape are checked. A repo
	// numbering ADR-001..ADR-026 does not write ADR-0070: that four-digit
	// form is a SIBLING repo's record (this repo's plans cite
	// platform-engineering's ADR-0070 by name), and reporting it as missing
	// would be the tool claiming another repo's tree is broken — the same
	// overreach F18 refuses.
	widths := map[int]bool{}
	for id := range records {
		widths[len(strings.TrimPrefix(id, "ADR-"))] = true
	}

	var violations []string
	var foreign int
	var unreadable []string
	checked := 0
	for _, rel := range adrCitingFiles(root) {
		body, err := fsfault.ReadFile(filepath.Join(root, rel))
		if err != nil {
			// ADR-027: a citing file this check could not open is an
			// instrument failure, not a cleaner repo. Skipping it silently
			// shrinks `checked` and hides whatever it cites.
			unreadable = append(unreadable, rel)
			continue
		}
		checked++
		for i, ln := range strings.Split(string(body), "\n") {
			for _, m := range adrCiteRe.FindAllStringSubmatch(ln, -1) {
				id := "ADR-" + m[1]
				if records[id] {
					continue
				}
				if !widths[len(m[1])] {
					foreign++
					continue
				}
				violations = append(violations, fmt.Sprintf("%s:%d → %s", rel, i+1, id))
				f.Evidence = append(f.Evidence, fmt.Sprintf("%s:%d → %s not in %s", rel, i+1, id, home))
			}
		}
	}
	sort.Strings(violations)

	blind := ""
	if n := len(unreadable); n > 0 {
		sort.Strings(unreadable)
		blind = fmt.Sprintf("; %d file(s) could NOT BE READ, so their citations were not checked: %s",
			n, strings.Join(unreadable, ", "))
	}

	if len(violations) > 0 {
		f.Status = StatusViolated
		f.Detail = fmt.Sprintf("%d citation(s) name a decision record that does not exist: %s",
			len(violations), strings.Join(violations, "; ")) + blind
		return f
	}
	if blind != "" {
		// Not violated — nothing it could read was wrong — but not a clean
		// pass either: the check cannot claim coverage it did not have.
		f.Status = StatusUnable
		f.Detail = fmt.Sprintf("every ADR citation across the %d readable file(s) resolves against %s (%d record(s))%s",
			checked, home, len(records), blind)
		return f
	}
	f.Detail = fmt.Sprintf("every ADR citation across %d file(s) resolves against %s (%d record(s))",
		checked, home, len(records))
	if foreign > 0 {
		f.Detail += fmt.Sprintf("; %d citation(s) in another repo's id shape were not checked", foreign)
	}
	return f
}

// knownADRIDs collects the ids that actually exist, from either record
// shape, and names where it found them.
func knownADRIDs(root string) (map[string]bool, string) {
	out := map[string]bool{}
	if body, err := os.ReadFile(filepath.Join(root, "ADR.md")); err == nil {
		for _, m := range adrRecordRe.FindAllStringSubmatch(string(body), -1) {
			out["ADR-"+m[1]] = true
		}
		if len(out) > 0 {
			return out, "ADR.md"
		}
	}
	for _, dir := range []string{filepath.Join("docs", "adr"), filepath.Join("docs", "intent", "decisions")} {
		entries, _ := filepath.Glob(filepath.Join(root, dir, "*.md"))
		for _, e := range entries {
			base := filepath.Base(e)
			if strings.EqualFold(base, "README.md") {
				continue
			}
			if m := adrFileRe.FindStringSubmatch(base); m != nil {
				out["ADR-"+strings.TrimLeft(m[1], "0")] = true
				out["ADR-"+m[1]] = true
			}
		}
		if len(out) > 0 {
			return out, dir
		}
	}
	return out, ""
}

// adrCitingFiles lists the documents whose ADR citations are checked.
func adrCitingFiles(root string) []string {
	var out []string
	add := func(pattern string) {
		matches, _ := filepath.Glob(filepath.Join(root, pattern))
		for _, m := range matches {
			if r, err := filepath.Rel(root, m); err == nil {
				out = append(out, filepath.ToSlash(r))
			}
		}
	}
	add(filepath.Join("docs", "design", "*.md"))
	add(filepath.Join("docs", "intent", "*.md"))
	add(filepath.Join("docs", "intent", "*", "*.md"))
	add(filepath.Join("docs", "plans", "*.md"))
	add("CHANGELOG.md")
	sort.Strings(out)
	return out
}
