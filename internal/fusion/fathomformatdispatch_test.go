package fusion

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/connector"
	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/store"
)

const fmtPlanSrc = "# P\n\n- [ ] a row {id: P-1, symbol: Alpha, package: internal/alpha}\n"

// TestFathomFormatDispatch (LB-F219): a project's .fathom/config.toml and a
// plan under docs/plans/ are claimed; README.md and every other Markdown or
// TOML file are not.
func TestFathomFormatDispatch(t *testing.T) {
	for _, tc := range []struct {
		path, src string
		want      ToolKind
	}{
		{".fathom/config.toml", "[daemon]\nauto_refresh = true\n", ToolFathomConfig},
		{"sub/.fathom/config.toml", "default_profile = \"x\"\n", ToolFathomConfig},
		{".fathom/other.toml", "[daemon]\n", ""},
		{"config.toml", "[daemon]\n", ""},
		{".cargo/config.toml", "[build]\n", ""},
		{".fathom/concerns.cue", "concerns: a: {want: \"w\"}\n", ToolFathomConcerns},
		{"sub/.fathom/concerns.cue", "concerns: {}\n", ToolFathomConcerns},
		{"concerns.cue", "concerns: {}\n", ""},
		{".fathom/layers/ui/layer.cue", "name: \"ui\"\n", ToolFathomLayerPacks},
		{".fathom/rules/arch.cue", "rules: []\n", ToolFathomLayerPacks},
		{".fathom/rules/sub/arch.cue", "rules: []\n", ""},
		{".fathom/guard.cue", "guards: []\n", ""},
		{"docs/plans/alpha.md", fmtPlanSrc, ToolFathomPlanRows},
		{"docs/plans/README.md", "# Plans\n\nRows read `- [ ] title {id: X-1}`.\n", ""},
		{"docs/plans/notes.md", "# Notes\n\n- [ ] a checklist\n", ""},
		{"docs/plans/archive/old.md", fmtPlanSrc, ""},
		{"docs/other.md", fmtPlanSrc, ""},
	} {
		e, tk, ok := fathomFormatDispatch(tc.path, []byte(tc.src))
		if (tc.want != "") != ok || tk != tc.want || (ok && e == nil) {
			t.Errorf("%s: dispatch = %v, %q, %v; want %q", tc.path, e != nil, tk, ok, tc.want)
		}
	}
	// LB-Q-markdown: every Markdown file is a doc candidate now (the native markdown
	// dialect reads its links); the plan dialect still claims docs/plans/*.md first.
	if !isDocCandidate("docs/plans/x.md") || !isDocCandidate("docs/x.md") || !isDocCandidate("README.md") || !isDocCandidate("site/p.mdx") || isDocCandidate("docs/x.txt") {
		t.Error("isDocCandidate: .md and .mdx are candidates, other text is not")
	}
}

func fmtCorpus() map[string]string {
	return map[string]string{
		".fathom/config.toml": "default_profile = \"example-main\"\n\n[profiles.example-main]\nprovider = \"anthropic\"\n\n[tests]\nresults_dir = \"reports\"\n",
		"docs/plans/alpha.md": "# Alpha\n\n" +
			"- [x] anchored by symbol and package {id: AL-1, symbol: Alpha, package: internal/alpha, reachable_from: TestAlpha}\n" +
			"- [ ] needs a row of another plan and one of its own {id: AL-2, symbol: Twin, needs: \"AL-1,beta.md#BE-1,beta.md#BE-9\"}\n" +
			"- [ ] a method anchor {id: AL-3, symbol: Method, package: internal/alpha}\n" +
			"- [ ] a missing symbol {id: AL-4, symbol: NoSuchThing}\n" +
			"- [ ] a receiver-qualified anchor {id: AL-5, symbol: (T).Method, package: internal/alpha}\n",
		"docs/plans/beta.md":           "# Beta\n\n- [x] the other plan {id: BE-1, symbol: Beta, package: pkg/beta}\n",
		"docs/plans/README.md":         "# Plans\n\nA row is `- [ ] title {id: X-1}`.\n",
		"internal/alpha/alpha.go":      "package alpha\n\nfunc Alpha() int { return 1 }\n\n// Twin is also defined in pkg/beta, so a bare anchor is ambiguous.\nfunc Twin() int { return 2 }\n\ntype T struct{}\n\nfunc (T) Method() int { return 3 }\n",
		"internal/alpha/alpha_test.go": "package alpha\n\nimport \"testing\"\n\nfunc TestAlpha(t *testing.T) { _ = Alpha() }\n",
		"pkg/beta/beta.go":             "package beta\n\nfunc Beta() int { return 1 }\n\nfunc Twin() int { return 2 }\n",
	}
}

// TestIngest_FathomFormats (LB-F219..LB-F222): one ingest of a scratch
// repository holds the config's sections and keys, the plans' rows, the
// reference a key makes to a profile, a row's needs on another plan's row,
// and a row's symbol anchor to the one code symbol it names (an ambiguous
// bare name and an absent one resolve to nothing).
func TestIngest_FathomFormats(t *testing.T) {
	dir := t.TempDir()
	for rel, src := range fmtCorpus() {
		p := filepath.Join(dir, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(src), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	st := store.NewMemoryStore()
	p := New(connector.NewLocalConnector(), st, Config{})
	t.Cleanup(p.Close)
	ctx := context.Background()
	if _, err := p.Ingest(ctx, core.ServiceRef{ID: "ff", Path: dir}); err != nil {
		t.Fatal(err)
	}
	obs, err := st.QueryObservations(ctx, "ff", "")
	if err != nil {
		t.Fatal(err)
	}
	kinds := map[string]string{}
	tools := map[string]string{}
	var ownKinds []string
	for _, o := range obs {
		kinds[o.SourceRef+"|"+o.Symbol] = string(o.SourceKind)
		tools[o.SourceRef] = string(o.ToolKind)
		if o.SourceRef == "docs/plans/README.md" {
			ownKinds = append(ownKinds, string(o.SourceKind))
		}
	}
	for key, want := range map[string]string{
		".fathom/config.toml|default_profile":   "FathomConfigSetting",
		".fathom/config.toml|example-main":      "FathomConfigProfile",
		".fathom/config.toml|tests":             "FathomConfigSection",
		".fathom/config.toml|tests.results_dir": "FathomConfigKey",
		"docs/plans/alpha.md|AL-2":              "PlanRow",
		"docs/plans/beta.md|BE-1":               "PlanRow",
	} {
		if kinds[key] != want {
			t.Errorf("%s = %q, want %q", key, kinds[key], want)
		}
	}
	if tools[".fathom/config.toml"] != string(ToolFathomConfig) || tools["docs/plans/alpha.md"] != string(ToolFathomPlanRows) {
		t.Errorf("tool kinds = %v", tools)
	}
	for _, k := range ownKinds {
		if k == "PlanRow" {
			t.Error("README.md is not a plan and carries no row")
		}
	}
	all, _ := st.TypedEdges(ctx, "ff")
	var edges []string
	for _, e := range all {
		if e.Kind == core.EdgeFileResolved && (e.FromFile == "docs/plans/alpha.md" || e.FromFile == ".fathom/config.toml") {
			edges = append(edges, e.FromSymbol+"->"+e.ToSymbol)
		}
	}
	sort.Strings(edges)
	want := []string{
		"ff::docs/plans/alpha.AL-1->ff::internal/alpha/alpha.Alpha",     // symbol, in its package
		"ff::docs/plans/alpha.AL-1->ff::internal/alpha/alpha.TestAlpha", // reachable_from
		"ff::docs/plans/alpha.AL-2->ff::docs/plans/beta.BE-1",           // needs: beta.md#BE-1; Twin is ambiguous, beta.md#BE-9 is no row
		"ff::docs/plans/alpha.AL-3->ff::internal/alpha/T.T.Method",      // a method anchor
		"ff::docs/plans/alpha.AL-5->ff::internal/alpha/T.T.Method",      // (T).Method, the gate\'s receiver form
	}
	if !reflect.DeepEqual(edges, want) {
		t.Errorf("stored edges\n got: %q\nwant: %q", edges, want)
	}
}

// TestReingestFile_FathomPlan (LB-F219): the one-file refresh reads a plan
// through the dispatch an ingest does, so editing a row keeps the rows (they
// would otherwise fall back to headings only).
func TestReingestFile_FathomPlan(t *testing.T) {
	dir := t.TempDir()
	for rel, src := range fmtCorpus() {
		p := filepath.Join(dir, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(src), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	st := store.NewMemoryStore()
	p := New(connector.NewLocalConnector(), st, Config{})
	t.Cleanup(p.Close)
	ctx := context.Background()
	if _, err := p.Ingest(ctx, core.ServiceRef{ID: "ff", Path: dir}); err != nil {
		t.Fatal(err)
	}
	rel := "docs/plans/beta.md"
	edited := "# Beta\n\n- [x] the other plan, edited {id: BE-1, symbol: Beta, package: pkg/beta}\n- [ ] a new row {id: BE-2, symbol: Beta}\n"
	if err := os.WriteFile(filepath.Join(dir, filepath.FromSlash(rel)), []byte(edited), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := p.ReingestFile(ctx, st, "ff", dir, rel); err != nil {
		t.Fatal(err)
	}
	obs, err := st.QueryObservations(ctx, "ff", "")
	if err != nil {
		t.Fatal(err)
	}
	var rows []string
	for _, o := range obs {
		if o.SourceRef == rel && o.SourceKind == "PlanRow" {
			rows = append(rows, o.Symbol)
		}
	}
	sort.Strings(rows)
	if !reflect.DeepEqual(rows, []string{"BE-1", "BE-2"}) {
		t.Errorf("rows after the refresh = %v, want [BE-1 BE-2]", rows)
	}
}
