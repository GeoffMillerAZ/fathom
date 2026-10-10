package intent_test

import (
	"sort"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/intent"
)

// newTree writes files (repo-relative path → body) under a fresh root.
func newTree(t *testing.T, files map[string]string) string {
	t.Helper()
	root := t.TempDir()
	writeTree(t, root, files)
	return root
}

const familyRouter = "# Project intent — router\n\nThe router.\n"

// The single-file corpus: one scope.md, one futures.md, one domain.md.
var singleFileCorpus = map[string]string{
	"docs/intent/README.md": familyRouter,
	"docs/intent/scope.md": `# Scope

Each goal carries a ` + "`Done when`" + ` test.

## Current goals

**G2. The A/B becomes admissible.** Done when: twenty units per arm.

**G9. The bench is honest, cheap to re-run, and the baseline
holds (ADR-031).** Done when: a stranger re-runs it.

## Current boundaries

- In: one machine. Revisit when: a second machine joins.

## Compatibility surface

- The CLI verbs.

## Non-goals

- **No hosted service.** Revisit when: an adopter asks in writing.
- **No web scale.** Revisit when: never.
`,
	"docs/intent/futures.md": `# Future directions

Things that might happen; each entry names its Promote when and Drop if conditions.

## F1. CGO-free binary

Promote when: the wazero engine matches the bindings. Drop if: never.

## F2. Public release

**Promote when.** A second user asks.
`,
	"docs/intent/domain.md": `# Domain

## Vocabulary

- **law N** — doctrine laws 1–15.

## Domain truths

- A claim cites its source.

## Assumptions

- One person, one machine. If false: revisit the daemon.
`,
}

// The same corpus split behind routers: every record byte-intact in a
// member, each router listing what it routes.
var familyCorpus = map[string]string{
	"docs/intent/README.md": familyRouter,
	"docs/intent/scope.md": `# Scope

Each goal carries a ` + "`Done when`" + ` test.

## Current goals

**G2. The A/B becomes admissible.** ` + "`scope/goals.md`" + `
**G9. The bench is honest, cheap to re-run, and the baseline holds (ADR-031).** ` + "`scope/goals.md`" + `

## Current boundaries

Held in ` + "`scope/rest.md`" + `.

## Compatibility surface

- The CLI verbs.

## Non-goals

Held in ` + "`scope/rest.md`" + `.
`,
	"docs/intent/scope/goals.md": `# Scope: current goals

**G2. The A/B becomes admissible.** Done when: twenty units per arm.

**G9. The bench is honest, cheap to re-run, and the baseline
holds (ADR-031).** Done when: a stranger re-runs it.
`,
	"docs/intent/scope/rest.md": `# Scope: boundaries and non-goals

## Current boundaries

- In: one machine. Revisit when: a second machine joins.

## Non-goals

- **No hosted service.** Revisit when: an adopter asks in writing.
- **No web scale.** Revisit when: never.
`,
	"docs/intent/futures.md": `# Future directions

Things that might happen; each entry names its Promote when and Drop if conditions.

| Id | Future | Member |
|---|---|---|
| F1 | CGO-free binary | ` + "`futures/f01-f02.md`" + ` |
| F2 | Public release | ` + "`futures/f01-f02.md`" + ` |
`,
	"docs/intent/futures/f01-f02.md": `# Future directions: F1 and F2

## F1. CGO-free binary

Promote when: the wazero engine matches the bindings. Drop if: never.

## F2. Public release

**Promote when.** A second user asks.
`,
	"docs/intent/domain.md": `# Domain

| Section | Member |
|---|---|
| Domain truths | ` + "`domain/truths.md`" + ` |
| Assumptions | ` + "`domain/assumptions.md`" + ` |

## Vocabulary

- **law N** — doctrine laws 1–15.
`,
	"docs/intent/domain/truths.md": `# Domain: truths

## Domain truths

- A claim cites its source.
`,
	"docs/intent/domain/assumptions.md": `# Domain: assumptions

## Assumptions

- One person, one machine. If false: revisit the daemon.
`,
}

// corpusShape is everything a reader of the corpus acts on, minus where it
// sits: the split moves lines, never identities.
func corpusShape(c intent.Corpus) []string {
	var out []string
	for _, g := range c.Goals {
		out = append(out, "goal "+g.ID+" "+g.Title)
	}
	var conds []string
	for _, x := range c.Conditions {
		conds = append(conds, "cond "+x.ID+" | "+x.Record+" | "+x.Label+" | "+x.Text)
	}
	sort.Strings(conds)
	out = append(out, conds...)
	for _, u := range c.Unparsed {
		out = append(out, "unparsed "+u.Reason)
	}
	var ex []string
	for _, e := range c.Excluded {
		ex = append(ex, "excluded "+e.Reason)
	}
	sort.Strings(ex)
	return append(out, ex...)
}

// TestLivingDocFamilyParsesAsOneDocument (AG-35, K8): a living document
// split into docs/intent/<doc>.md plus docs/intent/<doc>/*.md reads as the
// single file did — the same goals in the same order, the same conditions
// under the same record ids (a record's id carries across a member boundary
// exactly as it carried across a section heading), the same exclusions, and
// no Unparsed entry for a router that lists rather than defines.
func TestLivingDocFamilyParsesAsOneDocument(t *testing.T) {
	single, err := intent.Load(newTree(t, singleFileCorpus))
	if err != nil {
		t.Fatal(err)
	}
	split, err := intent.Load(newTree(t, familyCorpus))
	if err != nil {
		t.Fatal(err)
	}
	want, got := corpusShape(single), corpusShape(split)
	if strings.Join(want, "\n") != strings.Join(got, "\n") {
		t.Fatalf("the split corpus reads differently from the single file\nsingle:\n  %s\nsplit:\n  %s",
			strings.Join(want, "\n  "), strings.Join(got, "\n  "))
	}
	if len(single.Goals) != 2 || len(single.Conditions) != 9 {
		t.Fatalf("fixture drifted: %d goals, %d conditions", len(single.Goals), len(single.Conditions))
	}
	if len(split.Unparsed) != 0 {
		t.Errorf("a router that lists its members raised Unparsed: %+v", split.Unparsed)
	}
	// Identities moved; their sites name the member, never the router's listing.
	for _, g := range split.Goals {
		if g.File != "docs/intent/scope/goals.md" {
			t.Errorf("goal %s parsed from %s, want the member that defines it", g.ID, g.File)
		}
	}
	for _, x := range split.Conditions {
		if x.File == "docs/intent/futures.md" || x.File == "docs/intent/scope.md" {
			t.Errorf("condition %s read from the router %s:%d", x.ID, x.File, x.Line)
		}
	}
	var fams []string
	for _, f := range split.Families {
		fams = append(fams, f.Name+"="+f.Router+":"+strings.Join(f.Members, ","))
	}
	wantFams := []string{
		"domain=docs/intent/domain.md:docs/intent/domain/assumptions.md,docs/intent/domain/truths.md",
		"futures=docs/intent/futures.md:docs/intent/futures/f01-f02.md",
		"scope=docs/intent/scope.md:docs/intent/scope/goals.md,docs/intent/scope/rest.md",
	}
	if strings.Join(fams, " ") != strings.Join(wantFams, " ") {
		t.Errorf("families = %v, want %v", fams, wantFams)
	}
	if len(single.Families) != 0 {
		t.Errorf("a single-file corpus has no families, got %+v", single.Families)
	}
}

// TestFamilyShapeIsCheckedAcrossTheFamily: the healthy-empty guard still
// fires when NO file of a family carries the shape — once, at the router —
// and a member directory is never mistaken for a nested intent router.
func TestFamilyShapeIsCheckedAcrossTheFamily(t *testing.T) {
	root := newTree(t, map[string]string{
		"docs/intent/README.md":          familyRouter,
		"docs/intent/futures.md":         "# Future directions\n\n| Id | Future | Member |\n|---|---|---|\n",
		"docs/intent/futures/README.md":  "# Futures members\n\nNothing here yet.\n",
		"docs/intent/futures/later.md":   "# Later\n\nprose only\n",
		"docs/intent/scope.md":           "# Scope\n\n## Current goals\n\n**G2. Listed.** `scope/goals.md`\n",
		"docs/intent/scope/goals.md":     "# Goals\n\nnone yet\n",
		"docs/intent/invariants.md":      "# Invariants\n\n| # | Invariant | Enforced by |\n|---|---|---|\n| I1 | x | `stated` |\n",
		"docs/intent/questions/notes.md": "# Notes\n",
	})
	c, err := intent.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, u := range c.Unparsed {
		got = append(got, u.File+": "+u.Reason)
	}
	sort.Strings(got)
	if len(got) != 2 || !strings.HasPrefix(got[0], "docs/intent/futures.md: no future") || !strings.HasPrefix(got[1], "docs/intent/scope.md: no goal") {
		t.Fatalf("want one shape finding per empty family, at its router; got %v", got)
	}
	if len(c.Goals) != 0 {
		t.Errorf("a router's listing line is not a goal: %+v", c.Goals)
	}
	if len(c.NestedRouters) != 0 {
		t.Errorf("a member directory's README is not a nested router: %v", c.NestedRouters)
	}
}

// TestIntentFamilyIDsRouteOnce (AG-36): zero citation breaks, as a check.
// `fathom intent audit` names, per family, an id a member defines that the
// router does not list, an id the router lists that no member defines (or
// routes to the wrong member), and an id defined twice.
func TestIntentFamilyIDsRouteOnce(t *testing.T) {
	root := newTree(t, map[string]string{
		"docs/intent/README.md": familyRouter,
		"docs/intent/futures.md": "# Future directions\n\n| Id | Future | Member |\n|---|---|---|\n" +
			"| F1 | One | `futures/a.md` |\n" +
			"| F2 | Two | `futures/a.md` |\n" +
			"| F3 | Three | `futures/b.md` |\n" +
			"| F4 | Four | `futures/a.md` |\n",
		"docs/intent/futures/a.md": "# A\n\n## F1. One\n\nbody\n\n## F3. Three\n\nbody\n",
		"docs/intent/futures/b.md": "# B\n\n## F3. Three again\n\nbody\n\n## F4. Four\n\nbody\n\n## F5. Five\n\nbody\n",
		"docs/intent/scope.md": "# Scope\n\n## Current goals\n\n**G2. Two.** `scope/g.md`\n\n" +
			"**G7. Seven, kept in the router.** Done when: it ships.\n",
		"docs/intent/scope/g.md": "# Goals\n\n**G2. Two.** Done when: it lands.\n",
		"docs/intent/domain.md": "# Domain\n\n| Section | Member |\n|---|---|\n| Platform facts | `domain/facts.md` |\n\n" +
			"## Vocabulary\n\n- **term** — a word.\n",
		"docs/intent/domain/facts.md":  "## Platform facts\n\n- one\n",
		"docs/intent/domain/truths.md": "## Domain truths\n\n- two\n",
	})
	c, err := intent.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	a := intent.RunAudit(intent.AuditInput{Corpus: c})
	var got []string
	for _, cl := range a.Unproven {
		if cl.Kind != "U15" {
			continue
		}
		if cl.Remedy == "" || cl.Evidence == "" {
			t.Errorf("claim %s carries no remedy or evidence: %+v", cl.Ref, cl)
		}
		got = append(got, cl.Ref+" | "+cl.Detail)
	}
	want := []string{
		"docs/intent/domain.md Domain truths | defined in a member",
		"docs/intent/futures.md F2 | routed",
		"docs/intent/futures.md F3 | defined 2 times",
		"docs/intent/futures.md F4 | routed",
		"docs/intent/futures.md F5 | defined in a member",
	}
	if len(got) != len(want) {
		t.Fatalf("U15 claims = %d, want %d:\n  %s", len(got), len(want), strings.Join(got, "\n  "))
	}
	for i := range want {
		parts := strings.SplitN(want[i], " | ", 2)
		if !strings.HasPrefix(got[i], parts[0]+" | ") || !strings.Contains(got[i], parts[1]) {
			t.Errorf("claim %d = %q, want ref %q naming %q", i, got[i], parts[0], parts[1])
		}
	}
	for _, g := range got {
		if strings.Contains(g, " G2 ") || strings.Contains(g, " G7 ") || strings.Contains(g, "Platform facts") || strings.Contains(g, "Vocabulary") {
			t.Errorf("a routed or router-held record was claimed: %s", g)
		}
	}
	if !strings.Contains(got[3], "docs/intent/futures/b.md") {
		t.Errorf("a wrong-member route names where the record really is: %s", got[3])
	}
	if a.ByKind["U15"] != len(want) {
		t.Errorf("ByKind[U15] = %d", a.ByKind["U15"])
	}
}

// TestRepoIntentFamiliesRouteEveryID holds this repository's own split
// documents to zero citation breaks: every F and G id and every domain
// section is defined once and listed in its router.
func TestRepoIntentFamiliesRouteEveryID(t *testing.T) {
	c, err := intent.Load(repoRoot(t))
	if err != nil {
		t.Fatal(err)
	}
	for _, cl := range intent.FamilyRouteBreaks(c) {
		t.Errorf("%s: %s (%s)", cl.Ref, cl.Detail, cl.Evidence)
	}
}

// TestIntentAuditScansWholeFamily: a dead citation in a member that holds
// no condition (domain.md's platform facts) is still U11 — the single file
// was scanned because it held conditions somewhere, and the split must not
// make its sections invisible.
func TestIntentAuditScansWholeFamily(t *testing.T) {
	root := newTree(t, map[string]string{
		"docs/intent/README.md": familyRouter,
		"docs/intent/domain.md": "# Domain\n\n| Section | Member |\n|---|---|\n| Platform facts | `domain/facts.md` |\n\n" +
			"## Vocabulary\n\n- **term** — a word.\n",
		"docs/intent/domain/facts.md": "## Platform facts\n\n- A fact. Evidence: `docs/intent/gone.md`.\n",
	})
	c, err := intent.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	a := intent.RunAudit(intent.AuditInput{Corpus: c})
	found := false
	for _, cl := range a.Unproven {
		if cl.Kind == "U11" && strings.HasPrefix(cl.Ref, "docs/intent/domain/facts.md:3") {
			found = true
		}
	}
	if !found {
		t.Errorf("the dead citation in a condition-free member was not reported: %+v", a.Unproven)
	}
}
