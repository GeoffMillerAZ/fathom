package main

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// uicomponents_test.go keeps docs/ui-components.md honest about the one thing a
// consumer cannot discover by reading a signature: which TIER a package is, and
// therefore what has to be set up for it to render correctly (ADR-0017 §1).
//
// The L3a/L3b split existed undocumented for a year. ui/viz emits zero class=
// and drops into a page with no Tailwind and no build; ui/topology emits literal
// utility strings nine times and renders unstyled without an @source glob. A
// package that enters the kit without declaring which it is hands that discovery
// to whoever adopts it, at render time.

// tierCell matches a tier declaration in the package table: L0, L1, L2, L3a,
// L3b or L4, in any combination ("L1 + L2", "L3b + L4").
var tierCell = regexp.MustCompile(`\bL(?:0|1|2|3a|3b|4)\b`)

// TestUIComponentsDocCoversEveryPackage — every directory under appkit/ui/ has a
// row in docs/ui-components.md carrying a tier. Walking the filesystem rather
// than a list is the point: a new package cannot be added to the kit tierless.
func TestUIComponentsDocCoversEveryPackage(t *testing.T) {
	const doc = "../../docs/ui-components.md"
	body, err := os.ReadFile(doc)
	if err != nil {
		t.Fatalf("read %s: %v", doc, err)
	}

	// Index the rows by the package name in their first cell.
	// A package may appear in more than one table (the tier table plus its own
	// section); any row carrying a tier satisfies the rule.
	rows := map[string][]string{}
	for _, line := range strings.Split(string(body), "\n") {
		s := strings.TrimSpace(line)
		if !strings.HasPrefix(s, "| `") {
			continue
		}
		name, rest, ok := strings.Cut(strings.TrimPrefix(s, "| `"), "`")
		if !ok {
			continue
		}
		rows[name] = append(rows[name], rest)
	}

	entries, err := os.ReadDir("../../appkit/ui")
	if err != nil {
		t.Fatalf("read appkit/ui: %v", err)
	}
	seen := 0
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		pkg := e.Name()
		if strings.HasPrefix(pkg, ".") || strings.HasPrefix(pkg, "_") {
			continue
		}
		seen++
		found := rows[pkg]
		if len(found) == 0 {
			t.Errorf("appkit/ui/%s has no row in docs/ui-components.md — every package declares a tier (ADR-0017 §1), "+
				"because the tier IS the answer to what a consumer must set up", pkg)
			continue
		}
		tiered := false
		for _, row := range found {
			// The tier is the second cell.
			_, after, _ := strings.Cut(row, "|")
			tierField, _, _ := strings.Cut(after, "|")
			if tierCell.MatchString(tierField) {
				tiered = true
			}
		}
		if !tiered {
			t.Errorf("appkit/ui/%s appears in docs/ui-components.md but no row declares a tier; want one of L0/L1/L2/L3a/L3b/L4", pkg)
		}
	}
	if seen == 0 {
		t.Fatal("found no packages under appkit/ui — the walk is checking nothing")
	}

	// The two tiers a reader most needs distinguished must be spelled out, not
	// merely used in a table cell.
	for _, want := range []string{"L3a", "L3b", "no Tailwind", "@source"} {
		if !strings.Contains(string(body), want) {
			t.Errorf("docs/ui-components.md does not explain %q", want)
		}
	}

	// Guard the guard: if the table's package column were renamed, every lookup
	// above would miss and this would still pass with zero rows matched.
	matched := 0
	for pkg := range rows {
		if _, err := os.Stat(filepath.Join("../../appkit/ui", pkg)); err == nil {
			matched++
		}
	}
	if matched < seen {
		t.Errorf("only %d of %d packages matched a doc row", matched, seen)
	}
}
