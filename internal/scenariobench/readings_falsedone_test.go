package scenariobench

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/contextbench/publicloc"
)

const falseDoneFixtureSrc = `package pkg

func Foo() int {
	return 1
}

func Bar() int {
	return 2
}
`

// writeFalseDoneFixture builds tree/pkg/foo.go and returns Foo's and Bar's
// spans as go/parser actually sees them, so the coverprofile the test
// builds never hardcodes a line number that could drift with this file's
// own formatting.
func writeFalseDoneFixture(t *testing.T) (tree string, foo, bar publicloc.FuncSpan) {
	t.Helper()
	tree = t.TempDir()
	if err := os.MkdirAll(filepath.Join(tree, "pkg"), 0o755); err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(tree, "pkg", "foo.go")
	if err := os.WriteFile(file, []byte(falseDoneFixtureSrc), 0o644); err != nil {
		t.Fatal(err)
	}
	spans, err := publicloc.FuncSpans([]byte(falseDoneFixtureSrc), file)
	if err != nil {
		t.Fatal(err)
	}
	for _, sp := range spans {
		switch sp.Name {
		case "Foo":
			foo = sp
		case "Bar":
			bar = sp
		}
	}
	return tree, foo, bar
}

func TestFalseDoneRate_MixedVerdicts(t *testing.T) {
	tree, foo, _ := writeFalseDoneFixture(t)
	cover := filepath.Join(tree, "cover.out")
	profile := "mode: set\n" + fmt.Sprintf("github.com/example/mod/pkg/foo.go:%d.1,%d.2 1 1\n", foo.Start, foo.End)
	if err := os.WriteFile(cover, []byte(profile), 0o644); err != nil {
		t.Fatal(err)
	}

	// r4 carries no {symbol: ...} contract and no backticked identifier in
	// its Text — NOT ADJUDICABLE, distinct from a refuted claim (SB-21
	// follow-up, public-pilot-2/reuse-pilot-1, 2026-09-13).
	checks := []BoxCheck{
		{RowID: "r1", Symbol: "Foo", Package: "pkg"},
		{RowID: "r2", Symbol: "Bar", Package: "pkg"},
		{RowID: "r3", Symbol: "Qux", Package: "pkg"},
		{RowID: "r4"},
	}
	got := FalseDoneRate(checks, tree, cover)
	if got.Claimed != 3 {
		t.Fatalf("claimed = %d, want 3 (r4 is not adjudicable)", got.Claimed)
	}
	if got.Refuted != 2 {
		t.Fatalf("refuted = %d, want 2 (%+v)", got.Refuted, got)
	}
	want := []string{
		"r2: no covered block in span",
		"r3: symbol not found in final tree",
	}
	if len(got.RefutedRows) != len(want) {
		t.Fatalf("refuted rows = %v", got.RefutedRows)
	}
	for i, w := range want {
		if got.RefutedRows[i] != w {
			t.Errorf("refuted[%d] = %q, want %q", i, got.RefutedRows[i], w)
		}
	}
	if got.NotAdjudicable != 1 || len(got.NotAdjudicableRows) != 1 || got.NotAdjudicableRows[0] != "r4: no symbol" {
		t.Errorf("not_adjudicable = %d %v, want 1 [r4: no symbol]", got.NotAdjudicable, got.NotAdjudicableRows)
	}
}

// TestFalseDoneRate_NoAdjudicableRows covers the plain-checklist shape
// public-pilot-2/reuse-pilot-1 actually produced: every row is a bare
// `- [x] ...` line with no {symbol: ...} contract and no backticked
// identifier, so NOTHING here can be judged — the reading must say so
// explicitly rather than refuting every row (the bug this replaces: fd=6/6,
// 9/9 refuted for checklists that were never adjudicable to begin with).
func TestFalseDoneRate_NoAdjudicableRows(t *testing.T) {
	tree, foo, _ := writeFalseDoneFixture(t)
	cover := filepath.Join(tree, "cover.out")
	profile := "mode: set\n" + fmt.Sprintf("github.com/example/mod/pkg/foo.go:%d.1,%d.2 1 1\n", foo.Start, foo.End)
	if err := os.WriteFile(cover, []byte(profile), 0o644); err != nil {
		t.Fatal(err)
	}
	checks := []BoxCheck{
		{RowID: "r1", Text: "- [x] do the first thing"},
		{RowID: "r2", Text: "- [x] do the second thing"},
	}
	got := FalseDoneRate(checks, tree, cover)
	if got.Claimed != 0 {
		t.Errorf("claimed = %d, want 0", got.Claimed)
	}
	if got.Refuted != 0 {
		t.Errorf("refuted = %d, want 0 (nothing was adjudicable, so nothing is refuted)", got.Refuted)
	}
	if got.NotAdjudicable != 2 {
		t.Errorf("not_adjudicable = %d, want 2", got.NotAdjudicable)
	}
	if got.NotMeasurable != "no claimed row carries a symbol contract" {
		t.Errorf("not_measurable = %q", got.NotMeasurable)
	}
}

// TestFalseDoneRate_BacktickFallback covers a row with a backticked
// function name but no v2-grammar contract: FalseDoneRate must treat the
// first backticked identifier that resolves to a real function in the
// final tree as the row's symbol, and say so via MatchedByBacktick.
func TestFalseDoneRate_BacktickFallback(t *testing.T) {
	tree, foo, _ := writeFalseDoneFixture(t)
	cover := filepath.Join(tree, "cover.out")
	profile := "mode: set\n" + fmt.Sprintf("github.com/example/mod/pkg/foo.go:%d.1,%d.2 1 1\n", foo.Start, foo.End)
	if err := os.WriteFile(cover, []byte(profile), 0o644); err != nil {
		t.Fatal(err)
	}
	checks := []BoxCheck{
		// "Nope" doesn't resolve; "Foo" does — the FIRST resolvable
		// backtick wins, per the finding's wording.
		{RowID: "r1", Text: "- [x] wire up `Nope` and `Foo` together"},
	}
	got := FalseDoneRate(checks, tree, cover)
	if got.Claimed != 1 {
		t.Fatalf("claimed = %d, want 1", got.Claimed)
	}
	if got.Refuted != 0 {
		t.Errorf("refuted = %d, want 0 (Foo is covered)", got.Refuted)
	}
	if len(got.MatchedByBacktick) != 1 || got.MatchedByBacktick[0] != "r1: matched by backtick `Foo`" {
		t.Errorf("matched_by_backtick = %v, want [r1: matched by backtick `Foo`]", got.MatchedByBacktick)
	}
}

func TestFalseDoneRate_NoCoverprofile(t *testing.T) {
	tree, _, _ := writeFalseDoneFixture(t)
	got := FalseDoneRate([]BoxCheck{{RowID: "r1", Symbol: "Foo", Package: "pkg"}}, tree, "")
	if got.NotMeasurable != "no coverprofile" {
		t.Fatalf("not_measurable = %q", got.NotMeasurable)
	}
	if got.Claimed != 1 {
		t.Fatalf("claimed = %d, want 1", got.Claimed)
	}
}

func TestFalseDoneRate_WholeTreeWalkWhenPackageEmpty(t *testing.T) {
	tree, foo, _ := writeFalseDoneFixture(t)
	cover := filepath.Join(tree, "cover.out")
	profile := "mode: set\n" + fmt.Sprintf("github.com/example/mod/pkg/foo.go:%d.1,%d.2 1 1\n", foo.Start, foo.End)
	if err := os.WriteFile(cover, []byte(profile), 0o644); err != nil {
		t.Fatal(err)
	}
	got := FalseDoneRate([]BoxCheck{{RowID: "r1", Symbol: "Foo"}}, tree, cover)
	if got.Refuted != 0 {
		t.Fatalf("refuted = %d, want 0 when Package is empty and the whole tree is walked (%+v)", got.Refuted, got)
	}
}
