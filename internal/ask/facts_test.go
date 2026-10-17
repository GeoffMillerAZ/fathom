package ask

import (
	"context"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// fixtureSpan is one hand-labelled function in testdata/facts: its span and
// the labels its "want:" line gives each fact.
type fixtureSpan struct {
	span Span
	want map[string]string
}

var factLabels = []string{"unchecked", "wrapped", "executes_only", "locked", "threaded"}

func label(f Fact) string {
	switch {
	case !f.Applies:
		return "-"
	case f.Holds:
		return "yes"
	default:
		return "no"
	}
}

func factsByLabel(f Facts) map[string]string {
	return map[string]string{
		"unchecked":     label(f.UncheckedError),
		"wrapped":       label(f.Wrapped),
		"executes_only": label(f.ExecutesOnly),
		"locked":        label(f.LockedWrite),
		"threaded":      label(f.ThreadedContext),
	}
}

// fixtureSpans reads every "want:" line in dir's Go files (not
// recursively), parsing with go/parser only to find each labelled
// function's lines: the facts themselves come from Compute.
func fixtureSpans(t *testing.T, dir string) []fixtureSpan {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	var out []fixtureSpan
	fset := token.NewFileSet()
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".go") {
			continue
		}
		f, err := parser.ParseFile(fset, filepath.Join(dir, e.Name()), nil, parser.ParseComments)
		if err != nil {
			t.Fatal(err)
		}
		for _, d := range f.Decls {
			fd, ok := d.(*ast.FuncDecl)
			if !ok || fd.Doc == nil {
				continue
			}
			var want map[string]string
			for _, c := range fd.Doc.List {
				line, ok := strings.CutPrefix(c.Text, "// want:")
				if !ok {
					continue
				}
				want = map[string]string{}
				for _, kv := range strings.Fields(line) {
					k, v, _ := strings.Cut(kv, "=")
					want[k] = v
				}
			}
			if want == nil {
				continue
			}
			if len(want) != len(factLabels) {
				t.Fatalf("%s %s: a want line must label all five facts, got %v", e.Name(), fd.Name.Name, want)
			}
			out = append(out, fixtureSpan{
				span: Span{Repo: "fixture", File: e.Name(), Name: funcName(fd),
					Start: fset.Position(fd.Pos()).Line, End: fset.Position(fd.End()).Line},
				want: want,
			})
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].span.File != out[j].span.File {
			return out[i].span.File < out[j].span.File
		}
		return out[i].span.Start < out[j].span.Start
	})
	return out
}

// TestSpanFactsMatchHandLabeledFixtures is EXP-2's proof: every
// hand-labelled function in testdata/facts, tricky negatives included,
// computes exactly the labels its want line gives each of the five facts.
func TestSpanFactsMatchHandLabeledFixtures(t *testing.T) {
	dir := filepath.Join("testdata", "facts")
	fixtures := fixtureSpans(t, dir)
	if len(fixtures) < 40 {
		t.Fatalf("only %d labelled fixtures; the fixture set has shrunk", len(fixtures))
	}
	spans := make([]Span, len(fixtures))
	for i, f := range fixtures {
		spans[i] = f.span
	}
	got, err := Compute(context.Background(), dir, spans, nil)
	if err != nil {
		t.Fatal(err)
	}
	perFact := map[string]map[string]int{}
	for i, c := range got {
		fx := fixtures[i]
		if c.Excluded != "" {
			t.Errorf("%s %s: excluded (%s), want computed", fx.span.File, fx.span.Name, c.Excluded)
			continue
		}
		have := factsByLabel(c.Facts)
		for _, k := range factLabels {
			if have[k] != fx.want[k] {
				t.Errorf("%s %s: %s = %s, want %s", fx.span.File, fx.span.Name, k, have[k], fx.want[k])
			}
			if perFact[k] == nil {
				perFact[k] = map[string]int{}
			}
			perFact[k][fx.want[k]]++
		}
		if !strings.HasPrefix(c.Source, "func ") || !strings.HasSuffix(c.Source, "}") {
			t.Errorf("%s %s: source is not the function text: %q", fx.span.File, fx.span.Name, c.Source)
		}
		if c.Facts.LockedWrite.Applies != (c.Guard != "") {
			t.Errorf("%s %s: guard declaration %q, want one exactly when LockedWrite applies", fx.span.File, fx.span.Name, c.Guard)
		}
	}
	// Every fact is pinned both ways: at least two spans where it holds,
	// two where it does not, and two where it does not apply.
	for _, k := range factLabels {
		for _, v := range []string{"yes", "no", "-"} {
			if perFact[k][v] < 2 {
				t.Errorf("fact %s has %d fixtures labelled %s, want at least 2", k, perFact[k][v], v)
			}
		}
	}
}

func TestComputeExcludesWhatItCannotTypeCheck(t *testing.T) {
	dir := filepath.Join("testdata", "facts")
	spans := []Span{
		{Repo: "fixture", File: "broken/broken.go", Name: "Fine", Start: 8, End: 10},
		{Repo: "fixture", File: "broken/broken.go", Name: "Bad", Start: 13, End: 15},
		{Repo: "fixture", File: "missing/none.go", Name: "Gone", Start: 1, End: 2},
		{Repo: "fixture", File: "errs.go", Name: "noSuchFunc", Start: 1, End: 2},
	}
	got, err := Compute(context.Background(), dir, spans, nil)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{ExcludedLoad, ExcludedLoad, ExcludedNotLoaded, ExcludedNoDecl}
	for i, c := range got {
		if c.Excluded != want[i] {
			t.Errorf("%s %s: excluded %q, want %q", c.Span.File, c.Span.Name, c.Excluded, want[i])
		}
		if c.Facts != (Facts{}) || c.Source != "" {
			t.Errorf("%s %s: an excluded span carries facts or source: %+v", c.Span.File, c.Span.Name, c)
		}
	}
}

func TestFormatVerbs(t *testing.T) {
	cases := []struct {
		in   string
		want string
		ok   bool
	}{
		{"open: %w", "w", true},
		{"100%% %d %q: %w", "dqw", true},
		{"%+v %-5s %08.3f %#x", "vsfx", true},
		{"%*d", "", false},
		{"%[1]v", "", false},
		{"%.*f", "", false},
		{"trailing %", "", false},
	}
	for _, c := range cases {
		got, ok := formatVerbs(c.in)
		if ok != c.ok || string(got) != c.want {
			t.Errorf("formatVerbs(%q) = %q, %v; want %q, %v", c.in, string(got), ok, c.want, c.ok)
		}
	}
}
