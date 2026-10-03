package extractor

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/core"
)

func planFixture(t *testing.T, name string) []byte {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("dialects", "fathom-plan-rows", "testdata", name))
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func TestPlanRowsEntitiesAndReferences(t *testing.T) {
	x := NewPlanRowsExtractor()
	res, err := x.ExtractDoc("svc", "docs/plans/alpha.md", planFixture(t, "alpha.md"))
	if err != nil {
		t.Fatal(err)
	}
	rows := map[string]core.SymbolNode{}
	heads := 0
	for _, n := range res.Nodes {
		switch n.Kind {
		case KindPlanRow:
			rows[n.Name] = n
		case core.KindDocChunk:
			heads++
		}
	}
	if heads != 2 {
		t.Errorf("headings kept = %d, want 2 (the Markdown extractor is composed)", heads)
	}
	if len(rows) != 4 {
		t.Fatalf("rows = %v, want AL-1..AL-4 (the row without an id names nothing)", rows)
	}
	if r := rows["AL-1"]; r.Line != 7 || r.EndLine != 7 || r.Tier != core.TierPreScan {
		t.Errorf("AL-1 = line %d-%d tier %v", r.Line, r.EndLine, r.Tier)
	}
	// An in-file reference names a row the same plan declares: AL-9 is no row,
	// the duplicate collapses, alpha.md#AL-1 is this plan's own AL-1, and
	// beta.md#... is another plan's.
	wantCalls := map[string][]string{"AL-2": {"AL-1"}, "AL-3": {"AL-2"}, "AL-4": {"AL-1"}}
	for name, want := range wantCalls {
		if !reflect.DeepEqual(rows[name].Calls, want) {
			t.Errorf("%s calls = %v, want %v", name, rows[name].Calls, want)
		}
	}
	if len(rows["AL-1"].Calls) != 0 {
		t.Errorf("AL-1 calls = %v", rows["AL-1"].Calls)
	}
	var got []string
	for _, c := range res.CrossFile {
		got = append(got, c.From+" "+c.Edge+" "+c.Strategy+" "+c.Raw+" -> "+c.Target)
	}
	want := []string{
		"AL-1 plan_package repo-path internal/alpha -> internal/alpha",
		"AL-1 plan_reachable_from plan-symbol TestAlpha -> internal/alpha",
		"AL-1 plan_symbol plan-symbol Alpha -> internal/alpha",
		"AL-2 needs plan-row beta.md#BE-1 -> docs/plans/beta.md",
		"AL-4 needs plan-row beta.md#BE-2 -> docs/plans/beta.md",
		"AL-4 plan_symbol plan-symbol Four -> ",
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("cross-file =\n%q\nwant\n%q", got, want)
	}
}

func TestPlanRowsWrappedRowSpan(t *testing.T) {
	res, err := NewPlanRowsExtractor().ExtractDoc("svc", "docs/plans/beta.md", planFixture(t, "beta.md"))
	if err != nil {
		t.Fatal(err)
	}
	for _, n := range res.Nodes {
		if n.Name == "BE-2" && (n.Line != 4 || n.EndLine != 5) {
			t.Errorf("BE-2 = lines %d-%d, want 4-5", n.Line, n.EndLine)
		}
	}
}

func TestLooksLikePlan(t *testing.T) {
	if !LooksLikePlan(planFixture(t, "alpha.md")) {
		t.Error("a file of rows with ids is a plan")
	}
	for name, src := range map[string]string{
		"readme":        "# Plans\n\nWrite rows as `- [ ] title {id: X-1}` in a plan.\n",
		"no id":         "- [ ] a checklist\n- [x] another {symbol: Foo}\n",
		"empty":         "",
		"positional id": "- [ ] item {skip_test: true}\n",
	} {
		if LooksLikePlan([]byte(src)) {
			t.Errorf("%s: not a plan", name)
		}
	}
}
