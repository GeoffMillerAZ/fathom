package intent

import (
	"strings"
	"testing"
)

func surfaceCorpus() Corpus {
	return Corpus{
		Root: "/repo",
		Surfaces: []Surface{
			{Globs: []string{"internal/store/**"}, Tests: []string{"TestStoreFormatV0"}},
			{Globs: []string{"internal/mcp/**"}},                                   // no named test
			{Globs: []string{"cmd/fathom/*hook*.go"}, Tests: []string{"TestHook"}}, // file-scoped only
		},
	}
}

// EP-38: the finding is a CLAIMED row on a sensitive surface that pins no
// named test — never a row proving itself with a test the router did not
// happen to list.
func TestSurfaceRowsUncoveredListedNotGated(t *testing.T) {
	rows := []PlanRow{
		{Plan: "docs/plans/a.md", ID: "A-1", Package: "internal/store", Claimed: true},
		{Plan: "docs/plans/a.md", ID: "A-2", Package: "internal/store", Claimed: true, ReachableFrom: []string{"TestSomethingElse"}},
		{Plan: "docs/plans/a.md", ID: "A-3", Package: "internal/store", Claimed: false},
		{Plan: "docs/plans/a.md", ID: "A-4", Package: "internal/store", Claimed: true, SkipTest: true, SkipReason: "doc-only row"},
		{Plan: "docs/plans/a.md", ID: "A-5", Package: "internal/store", Claimed: true, SkipTest: true},
		{Plan: "docs/plans/a.md", ID: "A-6", Package: "internal/core", Claimed: true},
		{Plan: "docs/plans/a.md", ID: "A-7", Claimed: true},
		{Plan: "docs/plans/a.md", ID: "A-8", Package: "internal/mcp", Claimed: true},
	}

	claims, notes := SurfaceRowsUncovered(surfaceCorpus(), rows)

	got := map[string]Claim{}
	for _, c := range claims {
		if c.Kind != "U12" {
			t.Fatalf("wrong kind %q", c.Kind)
		}
		got[c.Ref] = c
	}
	want := []string{"docs/plans/a.md A-1", "docs/plans/a.md A-5", "docs/plans/a.md A-8"}
	for _, w := range want {
		if _, ok := got[w]; !ok {
			t.Fatalf("expected a finding for %s; got %v", w, keys(got))
		}
	}
	if len(claims) != len(want) {
		t.Fatalf("want exactly %v, got %v", want, keys(got))
	}

	// A row proving itself another way (A-2) is CORRECT, not a finding — the
	// spec's original join fired on every healthy row, which is why it was
	// narrowed.
	if _, ok := got["docs/plans/a.md A-2"]; ok {
		t.Fatal("a row that names its own test is not a gap")
	}
	// A `skip_test` with a written reason is an authored decision (A-4);
	// one without a reason (A-5) is an omission.
	if _, ok := got["docs/plans/a.md A-4"]; ok {
		t.Fatal("an authored skip with a reason is not a gap")
	}
	if !strings.Contains(got["docs/plans/a.md A-5"].Detail, "skips its test with no reason") {
		t.Fatalf("A-5 must say WHY it fired, got %q", got["docs/plans/a.md A-5"].Detail)
	}

	// The remedy names the surface's tests when it has them, and says
	// plainly when it does not — never invents one.
	if !strings.Contains(got["docs/plans/a.md A-1"].Remedy, "TestStoreFormatV0") {
		t.Fatalf("remedy must name the surface's test, got %q", got["docs/plans/a.md A-1"].Remedy)
	}
	if !strings.Contains(got["docs/plans/a.md A-8"].Remedy, "names no Test*") {
		t.Fatalf("a surface with no named test must say so, got %q", got["docs/plans/a.md A-8"].Remedy)
	}

	// Every surface it could not evaluate is NAMED (law 3).
	joined := strings.Join(notes, "\n")
	for _, want := range []string{"cmd/fathom/*hook*.go", "file-scoped", "name no Test*"} {
		if !strings.Contains(joined, want) {
			t.Fatalf("notes must name %q; got:\n%s", want, joined)
		}
	}
}

// Deterministic: same input, same order, every run.
func TestSurfaceRowsUncoveredDeterministic(t *testing.T) {
	rows := []PlanRow{
		{Plan: "p.md", ID: "Z", Package: "internal/store", Claimed: true},
		{Plan: "p.md", ID: "A", Package: "internal/store", Claimed: true},
		{Plan: "p.md", ID: "M", Package: "internal/mcp", Claimed: true},
	}
	first, firstNotes := SurfaceRowsUncovered(surfaceCorpus(), rows)
	for i := 0; i < 8; i++ {
		got, notes := SurfaceRowsUncovered(surfaceCorpus(), rows)
		for j := range got {
			if got[j].Ref != first[j].Ref {
				t.Fatalf("order drifted at %d: %q vs %q", j, got[j].Ref, first[j].Ref)
			}
		}
		if strings.Join(notes, "|") != strings.Join(firstNotes, "|") {
			t.Fatal("note order drifted")
		}
	}
	if first[0].Ref != "p.md A" {
		t.Fatalf("claims sort by ref; got %q first", first[0].Ref)
	}
}

func keys(m map[string]Claim) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
