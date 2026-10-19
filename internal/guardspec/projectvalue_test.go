package guardspec_test

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/bindspec"
	"github.com/geoffmilleraz/fathom/internal/capability"
	"github.com/geoffmilleraz/fathom/internal/custodian"
	"github.com/geoffmilleraz/fathom/internal/guardspec"
	"github.com/geoffmilleraz/fathom/internal/projectdir"
)

// projectValueCorpora is this repository, every fauxtown service (none
// carries a .fathom directory, so each is the absent case) and three
// synthetic layouts: all-commented scaffolds, files that fail their readers,
// and files that do not parse.
func projectValueCorpora(t *testing.T) []string {
	t.Helper()
	roots := []string{filepath.Join("..", "..")}
	fauxtown, err := filepath.Glob(filepath.Join("..", "connector", "testdata", "fauxtown", "*"))
	if err != nil {
		t.Fatal(err)
	}
	for _, d := range fauxtown {
		if st, serr := os.Stat(d); serr == nil && st.IsDir() {
			roots = append(roots, d)
		}
	}
	if len(roots) < 5 {
		t.Fatalf("fauxtown corpus missing: %v", roots)
	}
	layouts := []map[projectdir.File]string{
		{
			projectdir.Guard: "// scaffold\n", projectdir.Modules: "// scaffold\n",
			projectdir.Capabilities: "// scaffold\n", projectdir.Concerns: "// scaffold\n",
		},
		{
			projectdir.Guard:        "guards: [{note: \"governs nothing\"}]\nlayer_rules: [{id: \"Bad\"}]\n",
			projectdir.Modules:      "bindings: [{module: \"Bad Name\"}]\n",
			projectdir.Capabilities: "capabilities: {nope: {on: true}}\nbudgets?: {list: 3}\n",
			projectdir.Concerns:     "concerns: {c: {want: \"x\"}}\nextra: 1\n",
		},
		{
			projectdir.Guard: "guards: [\n", projectdir.Modules: "bindings: [\n",
			projectdir.Capabilities: "capabilities: {\n", projectdir.Concerns: "concerns: {\n",
		},
	}
	for _, files := range layouts {
		root := t.TempDir()
		if err := os.MkdirAll(projectdir.Under(root), 0o755); err != nil {
			t.Fatal(err)
		}
		for f, body := range files {
			if err := os.WriteFile(f.Path(root), []byte(body), 0o644); err != nil {
				t.Fatal(err)
			}
		}
		roots = append(roots, root)
	}
	return roots
}

func errText(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

func sameReading(t *testing.T, root, reader string, byPath, byProject any, pathErr, projectErr error) {
	t.Helper()
	if !reflect.DeepEqual(byPath, byProject) || errText(pathErr) != errText(projectErr) {
		t.Errorf("%s: %s differs\n by path:    %#v err %q\n by project: %#v err %q", root, reader, byPath, errText(pathErr), byProject, errText(projectErr))
	}
}

// FD-5 (docs/plans/fathom-dir.md): guardspec, bindspec, capability and
// custodian read their keys from projectdir.Load's value with no second
// parse, and each reading is identical, value and error text, to the path
// reader's on this repository, the fauxtown fixtures and the synthetic
// scaffold, reader-error and syntax-error layouts.
func TestLoadGuardsThroughProjectValueByteIdentical(t *testing.T) {
	reg := capability.Default()
	for _, root := range projectValueCorpora(t) {
		p, err := projectdir.Load(root)
		if err != nil {
			t.Fatalf("%s: Load: %v", root, err)
		}

		g1, e1 := guardspec.LoadGuards(projectdir.Guard.Path(root))
		g2, e2 := guardspec.GuardsFromProject(p)
		sameReading(t, root, "guards", g1, g2, e1, e2)

		l1, e1 := guardspec.LoadLayerRules(projectdir.Guard.Path(root))
		l2, e2 := guardspec.LayerRulesFromProject(p)
		sameReading(t, root, "layer_rules", l1, l2, e1, e2)

		b1, e1 := bindspec.LoadBindings(projectdir.Modules.Path(root))
		b2, e2 := bindspec.BindingsFromProject(p)
		sameReading(t, root, "bindings", b1, b2, e1, e2)

		s1, e1 := reg.LoadSettings(projectdir.Capabilities.Path(root), capability.ScopeProject)
		s2, e2 := reg.SettingsFromProject(p)
		sameReading(t, root, "capabilities", s1, s2, e1, e2)

		u1, e1 := capability.LoadBudgets(projectdir.Capabilities.Path(root), capability.ScopeProject)
		u2, e2 := capability.BudgetsFromProject(p)
		sameReading(t, root, "budgets", u1, u2, e1, e2)

		c1, e1 := custodian.LoadScopeFile(projectdir.Concerns.Path(root), custodian.ScopeProject)
		c2, e2 := custodian.ScopeFileFromProject(p)
		sameReading(t, root, "concerns", c1, c2, e1, e2)
	}
	// This repository's guards are real: an empty reading on both sides
	// would pass the comparison and prove nothing.
	p, err := projectdir.Load(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	if g, err := guardspec.GuardsFromProject(p); err != nil || len(g.Guards) == 0 {
		t.Errorf("this repository's guards through the project value: %d guards, err %v", len(g.Guards), err)
	}

	// No second parse: every reading comes from the loaded value, so
	// rewriting each file on disk after Load changes none of them. A reader
	// that went back to its file would read the syntax error instead.
	root := t.TempDir()
	if err := os.MkdirAll(projectdir.Under(root), 0o755); err != nil {
		t.Fatal(err)
	}
	files := []projectdir.File{projectdir.Guard, projectdir.Modules, projectdir.Capabilities, projectdir.Concerns}
	for _, f := range files {
		data, err := os.ReadFile(f.Path(filepath.Join("..", "..")))
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(f.Path(root), data, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	p, err = projectdir.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	readAll := func() []any {
		g, ge := guardspec.GuardsFromProject(p)
		l, le := guardspec.LayerRulesFromProject(p)
		b, be := bindspec.BindingsFromProject(p)
		s, se := reg.SettingsFromProject(p)
		u, ue := capability.BudgetsFromProject(p)
		c, ce := custodian.ScopeFileFromProject(p)
		return []any{g, errText(ge), l, errText(le), b, errText(be), s, errText(se), u, errText(ue), c, errText(ce)}
	}
	before := readAll()
	for _, f := range files {
		if err := os.WriteFile(f.Path(root), []byte("rewritten: [\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := guardspec.LoadGuards(projectdir.Guard.Path(root)); err == nil {
		t.Fatal("the rewritten guard.cue still parses: the probe proves nothing")
	}
	if after := readAll(); !reflect.DeepEqual(before, after) {
		t.Errorf("a reading changed after the files changed on disk: a reader parsed its file a second time\n before: %#v\n after:  %#v", before, after)
	}
}
