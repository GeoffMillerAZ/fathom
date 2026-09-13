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

	checks := []BoxCheck{
		{RowID: "r1", Symbol: "Foo", Package: "pkg"},
		{RowID: "r2", Symbol: "Bar", Package: "pkg"},
		{RowID: "r3", Symbol: "Qux", Package: "pkg"},
		{RowID: "r4"},
	}
	got := FalseDoneRate(checks, tree, cover)
	if got.Claimed != 4 {
		t.Fatalf("claimed = %d, want 4", got.Claimed)
	}
	if got.Refuted != 3 {
		t.Fatalf("refuted = %d, want 3 (%+v)", got.Refuted, got)
	}
	want := []string{
		"r2: no covered block in span",
		"r3: symbol not found in final tree",
		"r4: no symbol",
	}
	if len(got.RefutedRows) != len(want) {
		t.Fatalf("refuted rows = %v", got.RefutedRows)
	}
	for i, w := range want {
		if got.RefutedRows[i] != w {
			t.Errorf("refuted[%d] = %q, want %q", i, got.RefutedRows[i], w)
		}
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
