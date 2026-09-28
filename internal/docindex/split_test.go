package docindex

import "testing"

// TestSplitDocUnitsHeadingPaths pins KR-2's core contract: a heading path
// per unit built from every still-open ancestor, a leaf heading (one with
// its own body text) becomes a unit, and a purely structural heading (only
// immediate sub-headings, no body of its own) does not — plus 1-based,
// inclusive line ranges for every emitted unit.
func TestSplitDocUnitsHeadingPaths(t *testing.T) {
	content := `# Title

intro paragraph, belongs to the document root

## Section A

### Subsection A1

body text for A1

## Section B

body text for B
`
	units := SplitDocUnits(DocKindOther, content)

	want := []struct {
		path      string
		lineStart int
		lineEnd   int
	}{
		{"Title", 1, 4},
		{"Title > Section A > Subsection A1", 7, 10},
		{"Title > Section B", 11, 13},
	}
	if len(units) != len(want) {
		t.Fatalf("SplitDocUnits = %d units, want %d: %+v", len(units), len(want), units)
	}
	for i, w := range want {
		u := units[i]
		if u.HeadingPath != w.path {
			t.Errorf("unit %d HeadingPath = %q, want %q", i, u.HeadingPath, w.path)
		}
		if u.LineStart != w.lineStart || u.LineEnd != w.lineEnd {
			t.Errorf("unit %d lines = [%d,%d], want [%d,%d]", i, u.LineStart, u.LineEnd, w.lineStart, w.lineEnd)
		}
		if u.Kind != KindSection {
			t.Errorf("unit %d Kind = %q, want %q", i, u.Kind, KindSection)
		}
	}
	// Section A itself (structural only) must not appear as its own unit.
	for _, u := range units {
		if u.HeadingPath == "Title > Section A" {
			t.Fatalf("structural-only heading emitted as its own unit: %+v", u)
		}
	}
}

// TestSplitDocUnitsADRBoldLabels pins the ADR rule (§3): a `## ADR-NNN`
// section splits further at bold paragraph labels, each becoming its own
// unit under "<ADR heading> > <Label>".
func TestSplitDocUnitsADRBoldLabels(t *testing.T) {
	content := `# Architecture Decision Records

## ADR-001 — Three-layer architecture

**Status.** Accepted.

**Decision.** Strictly separate three layers.

Some continuation text for the decision.

**Why.** Each layer is independently testable.

## ADR-002 — Something else

**Status.** Accepted.
`
	units := SplitDocUnits(DocKindADR, content)
	if len(units) != 4 {
		t.Fatalf("SplitDocUnits(adr) = %d units, want 4: %+v", len(units), units)
	}
	wantPaths := []string{
		"Architecture Decision Records > ADR-001 — Three-layer architecture > Status",
		"Architecture Decision Records > ADR-001 — Three-layer architecture > Decision",
		"Architecture Decision Records > ADR-001 — Three-layer architecture > Why",
		"Architecture Decision Records > ADR-002 — Something else > Status",
	}
	for i, p := range wantPaths {
		if units[i].HeadingPath != p {
			t.Errorf("unit %d HeadingPath = %q, want %q", i, units[i].HeadingPath, p)
		}
		if units[i].Kind != KindADRSection {
			t.Errorf("unit %d Kind = %q, want %q", i, units[i].Kind, KindADRSection)
		}
	}
	if got := units[1].Body; got == "" || len(got) < len("**Decision.**") {
		t.Errorf("Decision unit body too short: %q", got)
	}
	// The continuation paragraph belongs to Decision, not a new unit.
	for _, u := range units {
		if u.HeadingPath == "Architecture Decision Records > ADR-001 — Three-layer architecture > Some continuation text for the decision." {
			t.Fatalf("continuation text split into its own unit: %+v", u)
		}
	}
}

// TestSplitDocUnitsPlanRows pins the plan rule (§3): every checklist row
// carrying an {id: ...} annotation becomes its own KindPlanRow unit, pulled
// out of its enclosing section's prose so it is never indexed twice.
func TestSplitDocUnitsPlanRows(t *testing.T) {
	content := `# Knowledge retrieval: the plan

## Wave 2: the index, ranking, links and drift

- [ ] the store schema {id: KR-1, symbol: PutDocUnits, package: internal/store}
- [ ] the splitter {id: KR-2, symbol: SplitDocUnits, package: internal/docindex}

## Refused

- [-] Embeddings {id: KR-X1, disposition: refused, reason: "no"}
`
	units := SplitDocUnits(DocKindPlan, content)

	var rows []Unit
	var sections []Unit
	for _, u := range units {
		if u.Kind == KindPlanRow {
			rows = append(rows, u)
		} else {
			sections = append(sections, u)
		}
	}
	if len(rows) != 3 {
		t.Fatalf("plan rows = %d, want 3: %+v", len(rows), units)
	}
	wantIDs := []string{"KR-1", "KR-2", "KR-X1"}
	for i, id := range wantIDs {
		if rows[i].RowID != id {
			t.Errorf("row %d RowID = %q, want %q", i, rows[i].RowID, id)
		}
	}
	if rows[0].HeadingPath != "Knowledge retrieval: the plan > Wave 2: the index, ranking, links and drift > KR-1" {
		t.Errorf("row 0 HeadingPath = %q", rows[0].HeadingPath)
	}
	if rows[2].HeadingPath != "Knowledge retrieval: the plan > Refused > KR-X1" {
		t.Errorf("row 2 HeadingPath = %q", rows[2].HeadingPath)
	}
	// Row lines are excluded from the enclosing section's own prose unit —
	// a wave heading with ONLY row lines beneath it produces no section
	// unit at all (nothing left to index once the rows are pulled out).
	for _, s := range sections {
		if s.HeadingPath == "Knowledge retrieval: the plan > Wave 2: the index, ranking, links and drift" {
			t.Fatalf("wave section retained row text after exclusion: %+v", s)
		}
	}
}

// TestSplitDocUnitsIntentRecords pins the intent rule (§3): "a `##` section
// or a numbered record" — a heading with a plain-prose body becomes one
// unit, while a heading whose body is bold-numbered records (the shape
// docs/intent/scope.md's goals and ADR.md's labels both use) splits at
// each record.
func TestSplitDocUnitsIntentRecords(t *testing.T) {
	content := `# Scope

## Current goals

**G2. The A/B becomes admissible.** Done when: fathom flywheel reports enough units.

**G3. The continuity-thread bet reaches its sample.** Done when: 20 units.

## Plain section

just prose, no numbered records here
`
	units := SplitDocUnits(DocKindIntent, content)
	var paths []string
	for _, u := range units {
		paths = append(paths, u.HeadingPath)
		if u.Kind != KindIntentSection {
			t.Errorf("unit %q Kind = %q, want %q", u.HeadingPath, u.Kind, KindIntentSection)
		}
	}
	wantPaths := []string{
		"Scope > Current goals > G2. The A/B becomes admissible",
		"Scope > Current goals > G3. The continuity-thread bet reaches its sample",
		"Scope > Plain section",
	}
	if len(paths) != len(wantPaths) {
		t.Fatalf("SplitDocUnits(intent) paths = %v, want %v", paths, wantPaths)
	}
	for i, p := range wantPaths {
		if paths[i] != p {
			t.Errorf("unit %d path = %q, want %q", i, paths[i], p)
		}
	}
}

// TestSplitDocUnitsEmpty pins that an empty or heading-free document yields
// no units rather than panicking or emitting an empty-body unit.
func TestSplitDocUnitsEmpty(t *testing.T) {
	if got := SplitDocUnits(DocKindOther, ""); len(got) != 0 {
		t.Fatalf("SplitDocUnits(\"\") = %+v, want none", got)
	}
	if got := SplitDocUnits(DocKindOther, "\n\n\n"); len(got) != 0 {
		t.Fatalf("SplitDocUnits(blank) = %+v, want none", got)
	}
}
