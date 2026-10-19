package decide_test

// hexagonal_test.go — KS-8 and KS-9 (docs/plans/decision-port.md section J,
// ADR-073 decisions 2 and 3, docs/design/decision-models.md section 2): the
// port is hexagonal. Every provider decide.Open can build is an adapter
// package with a decisiontest.RunConformance run of its own (KS-8), and no
// package outside internal/decide and the kit mirror imports an adapter
// package (KS-9). Both read the source tree with go/parser: the providers
// come from internal/decide's own switches on cfg.Provider and the adapters
// from its own imports, never from a hand list that could drift. Each check
// is a function of a tree's root, so a temp tree proves it red.

import (
	"bufio"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"testing"
)

// kitDecisionDir is the kit mirror's decision package, relative to the
// module root.
const kitDecisionDir = "third_party/agentic-kit/foundations/decision"

// repoRoot is this repository's root, two levels above this file.
func repoRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	return filepath.Join(filepath.Dir(file), "..", "..")
}

// modulePath reads the module line of root's go.mod.
func modulePath(root string) (string, error) {
	f, err := os.Open(filepath.Join(root, "go.mod"))
	if err != nil {
		return "", err
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		if mod, ok := strings.CutPrefix(strings.TrimSpace(sc.Text()), "module "); ok {
			return strings.TrimSpace(mod), nil
		}
	}
	return "", fmt.Errorf("%s/go.mod has no module line", root)
}

// parseDir parses every .go file in dir (tests too when withTests), sorted
// by name.
func parseDir(dir string, mode parser.Mode, withTests bool) ([]*ast.File, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	fset := token.NewFileSet()
	var files []*ast.File
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".go") || (!withTests && strings.HasSuffix(name, "_test.go")) {
			continue
		}
		f, err := parser.ParseFile(fset, filepath.Join(dir, name), nil, mode)
		if err != nil {
			return nil, err
		}
		files = append(files, f)
	}
	return files, nil
}

// importAliases maps each local name f gives an import to its path.
func importAliases(f *ast.File) map[string]string {
	aliases := map[string]string{}
	for _, imp := range f.Imports {
		p, err := strconv.Unquote(imp.Path.Value)
		if err != nil {
			continue
		}
		name := path.Base(p)
		if imp.Name != nil {
			name = imp.Name.Name
		}
		aliases[name] = p
	}
	return aliases
}

// openableProviders is every provider internal/decide's non-test code can
// build: the string cases of each switch on cfg.Provider, less "" and
// "none" (decision.None, the kit's own), sorted. adapters is every kit
// adapter package that code imports (a subpackage of the kit's decision
// package other than decisiontest), sorted.
func openableProviders(root string) (providers, adapters []string, err error) {
	mod, err := modulePath(root)
	if err != nil {
		return nil, nil, err
	}
	files, err := parseDir(filepath.Join(root, "internal", "decide"), 0, false)
	if err != nil {
		return nil, nil, err
	}
	kit := mod + "/" + kitDecisionDir + "/"
	for _, f := range files {
		for _, p := range importAliases(f) {
			if sub, ok := strings.CutPrefix(p, kit); ok && sub != "decisiontest" && !strings.Contains(sub, "/") && !slices.Contains(adapters, p) {
				adapters = append(adapters, p)
			}
		}
		ast.Inspect(f, func(n ast.Node) bool {
			sw, ok := n.(*ast.SwitchStmt)
			if !ok {
				return true
			}
			sel, ok := sw.Tag.(*ast.SelectorExpr)
			if !ok || sel.Sel.Name != "Provider" {
				return true
			}
			for _, st := range sw.Body.List {
				for _, e := range st.(*ast.CaseClause).List {
					lit, ok := e.(*ast.BasicLit)
					if !ok || lit.Kind != token.STRING {
						continue
					}
					v, _ := strconv.Unquote(lit.Value)
					if v != "" && v != "none" && !slices.Contains(providers, v) {
						providers = append(providers, v)
					}
				}
			}
			return true
		})
	}
	slices.Sort(providers)
	slices.Sort(adapters)
	return providers, adapters, nil
}

// runsConformance reports whether a _test.go file in dir calls
// decisiontest.RunConformance (under whatever name it imports it).
func runsConformance(dir, decisiontestPath string) (bool, error) {
	files, err := parseDir(dir, 0, true)
	if err != nil {
		return false, err
	}
	found := false
	for _, f := range files {
		var local []string
		for name, p := range importAliases(f) {
			if p == decisiontestPath {
				local = append(local, name)
			}
		}
		ast.Inspect(f, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			sel, ok := call.Fun.(*ast.SelectorExpr)
			if !ok || sel.Sel.Name != "RunConformance" {
				return true
			}
			if x, ok := sel.X.(*ast.Ident); ok && slices.Contains(local, x.Name) {
				found = true
			}
			return true
		})
	}
	return found, nil
}

// conformanceGaps lists, sorted, every provider decide.Open can build that
// has no adapter package of its own name or whose adapter package runs no
// decisiontest.RunConformance, every adapter decide imports that no provider
// names, and the kit's decision package itself when it runs none (provider
// none, decision.None). The providers and adapters it judged come back too.
func conformanceGaps(root string) (gaps, providers, adapters []string, err error) {
	providers, adapters, err = openableProviders(root)
	if err != nil {
		return nil, nil, nil, err
	}
	mod, err := modulePath(root)
	if err != nil {
		return nil, nil, nil, err
	}
	dtest := mod + "/" + kitDecisionDir + "/decisiontest"
	check := func(label, importPath string) error {
		ok, err := runsConformance(filepath.Join(root, filepath.FromSlash(strings.TrimPrefix(importPath, mod+"/"))), dtest)
		if err != nil {
			return err
		}
		if !ok {
			gaps = append(gaps, fmt.Sprintf("%s: %s has no decisiontest.RunConformance run in its _test.go files", label, importPath))
		}
		return nil
	}
	named := map[string]bool{}
	for _, p := range providers {
		i := slices.IndexFunc(adapters, func(a string) bool { return path.Base(a) == p })
		if i < 0 {
			gaps = append(gaps, fmt.Sprintf("provider %s: decide.Open can build it, but internal/decide imports no kit adapter package named %s", p, p))
			continue
		}
		named[adapters[i]] = true
		if err := check("provider "+p, adapters[i]); err != nil {
			return nil, nil, nil, err
		}
	}
	for _, a := range adapters {
		if !named[a] {
			gaps = append(gaps, fmt.Sprintf("adapter %s: internal/decide imports it, but no case of a switch on cfg.Provider names %s", a, path.Base(a)))
		}
	}
	if err := check("provider none", mod+"/"+kitDecisionDir); err != nil {
		return nil, nil, nil, err
	}
	slices.Sort(gaps)
	return gaps, providers, adapters, nil
}

// adapterImportViolations lists, sorted, every Go file (tests included)
// outside internal/decide and third_party (the kit mirror) that imports one
// of forbidden. testdata directories, hidden ones and the gitignored
// workspace/, ref/ and tmp/ are not the tree's packages and are skipped.
func adapterImportViolations(root string, forbidden []string) ([]string, error) {
	var out []string
	fset := token.NewFileSet()
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(root, p)
		rel = filepath.ToSlash(rel)
		if d.IsDir() {
			name := d.Name()
			switch {
			case rel == ".":
				return nil
			case strings.HasPrefix(name, "."), name == "testdata", name == "node_modules",
				rel == "third_party", rel == "workspace", rel == "ref", rel == "tmp", rel == "internal/decide":
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(rel, ".go") {
			return nil
		}
		f, err := parser.ParseFile(fset, p, nil, parser.ImportsOnly)
		if err != nil {
			return err
		}
		for _, imp := range f.Imports {
			ip, _ := strconv.Unquote(imp.Path.Value)
			if slices.Contains(forbidden, ip) {
				out = append(out, rel+" imports "+ip)
			}
		}
		return nil
	})
	slices.Sort(out)
	return out, err
}

// writeTree writes files (slash paths relative to root) under a fresh root.
func writeTree(t *testing.T, files map[string]string) string {
	t.Helper()
	root := t.TempDir()
	for rel, body := range files {
		p := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

const fixtureModule = "example.test/fixture"

func TestEveryOpenableProviderHasAConformanceRun(t *testing.T) {
	gaps, providers, adapters, err := conformanceGaps(repoRoot(t))
	if err != nil {
		t.Fatal(err)
	}
	if len(providers) == 0 || len(adapters) == 0 {
		t.Fatalf("found providers %v and adapters %v: the derivation read nothing, so it proves nothing", providers, adapters)
	}
	for _, gap := range gaps {
		t.Error(gap)
	}
	t.Logf("providers decide.Open can build: %v (and none, the kit's own); adapters: %v", providers, adapters)

	t.Run("a provider whose adapter runs no conformance is a gap", func(t *testing.T) {
		kit := fixtureModule + "/" + kitDecisionDir
		conforms := "package x_test\n\nimport \"" + kit + "/decisiontest\"\n\nfunc TestConformance(t *testing.T) { decisiontest.RunConformance(t, decisiontest.Harness{}) }\n"
		root := writeTree(t, map[string]string{
			"go.mod": "module " + fixtureModule + "\n",
			"internal/decide/decide.go": "package decide\n\nimport (\n\t\"" + kit + "/llm\"\n\t\"" + kit + "/gemini\"\n\t\"" + kit + "/orphan\"\n)\n\n" +
				"func build(cfg struct{ Provider string }) {\n\tswitch cfg.Provider {\n\tcase \"\", \"none\":\n\tcase \"llm\":\n\t\t_ = llm.New\n\tcase \"gemini\":\n\t\t_ = gemini.New\n\tcase \"bare\":\n\t}\n\t_ = orphan.New\n}\n",
			"internal/decide/decide_test.go":               "package decide\n\nfunc init() { switch x := struct{ Provider string }{}; x.Provider { case \"testonly\": } }\n",
			kitDecisionDir + "/conformance_test.go":        conforms,
			kitDecisionDir + "/llm/conformance_test.go":    conforms,
			kitDecisionDir + "/gemini/gemini_test.go":      "package gemini_test\n\nfunc TestNothing(t *testing.T) {}\n",
			kitDecisionDir + "/orphan/conformance_test.go": conforms,
		})
		gaps, providers, _, err := conformanceGaps(root)
		if err != nil {
			t.Fatal(err)
		}
		if want := []string{"bare", "gemini", "llm"}; !slices.Equal(providers, want) {
			t.Errorf("providers = %v, want %v (a test file's switch never counts)", providers, want)
		}
		joined := strings.Join(gaps, "\n")
		for _, want := range []string{"provider gemini: " + kit + "/gemini has no decisiontest.RunConformance run", "provider bare: decide.Open can build it, but internal/decide imports no kit adapter package named bare", "adapter " + kit + "/orphan"} {
			if !strings.Contains(joined, want) {
				t.Errorf("gaps do not name %q:\n%s", want, joined)
			}
		}
		if strings.Contains(joined, "provider llm") || strings.Contains(joined, "provider none") {
			t.Errorf("a conforming adapter was named a gap:\n%s", joined)
		}
	})
}

func TestOnlyDecideImportsProviderAdapters(t *testing.T) {
	root := repoRoot(t)
	mod, err := modulePath(root)
	if err != nil {
		t.Fatal(err)
	}
	_, adapters, err := openableProviders(root)
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"llm", "typesafe", "openai"} {
		if p := mod + "/" + kitDecisionDir + "/" + name; !slices.Contains(adapters, p) {
			t.Fatalf("the derived adapters %v do not hold %s: the guard would not cover an adapter ADR-073 names", adapters, p)
		}
	}
	violations, err := adapterImportViolations(root, adapters)
	if err != nil {
		t.Fatal(err)
	}
	for _, v := range violations {
		t.Errorf("%s: only internal/decide imports a provider adapter package (ADR-073 decision 2); ask through decide.Open's port instead", v)
	}

	t.Run("an import outside internal/decide is named, tests included", func(t *testing.T) {
		ts := fixtureModule + "/" + kitDecisionDir + "/typesafe"
		imports := "package p\n\nimport _ \"" + ts + "\"\n"
		root := writeTree(t, map[string]string{
			"go.mod":                           "module " + fixtureModule + "\n",
			"internal/decide/remote.go":        imports,
			"pkg/client/decision_test.go":      "package p_test\n\nimport _ \"" + ts + "\"\n",
			"cmd/tool/main.go":                 "package main\n\nimport _ \"" + fixtureModule + "/" + kitDecisionDir + "\"\n",
			"internal/x/testdata/fixture.go":   imports,
			"third_party/agentic-kit/x/x.go":   imports,
			"workspace/tmp/scratch/scratch.go": imports,
			"internal/y/y.go":                  imports,
			"internal/decide/sub/inside.go":    imports,
		})
		got, err := adapterImportViolations(root, []string{ts})
		if err != nil {
			t.Fatal(err)
		}
		want := []string{"pkg/client/decision_test.go imports " + ts, "internal/y/y.go imports " + ts}
		slices.Sort(want)
		if !slices.Equal(got, want) {
			t.Errorf("violations = %v, want %v", got, want)
		}
	})
}
