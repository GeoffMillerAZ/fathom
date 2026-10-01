package extractor

// githooksdialect_test.go — goldens for the git-hooks-config doc-engine dialect
// (LB-Q-git-hooks-config, no incumbent, design §13.2 steps 10/12/14): lefthook's
// hooks and their commands, scripts and jobs, pre-commit's configured hooks and a
// hook repository's manifest. The same golden shape as the taskfile dialect's
// (symbols: name/kind/package/line/end_line/signature/calls/tier). A fixture named
// `<case>__<basename>` is read as if it sat at repo/<basename>, since the entity
// specs select by the file's basename (a lefthook.yml and a .pre-commit-config.yaml
// read differently).

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
)

type githooksGoldenSymbol struct {
	Name      string   `json:"name"`
	Kind      string   `json:"kind"`
	Package   string   `json:"package"`
	Line      int      `json:"line"`
	EndLine   int      `json:"end_line"`
	Signature string   `json:"signature,omitempty"`
	Calls     []string `json:"calls,omitempty"`
	Tier      string   `json:"tier"`
}

type githooksGolden struct {
	Symbols []githooksGoldenSymbol `json:"symbols"`
}

func loadGitHooksExtractor(t *testing.T) *DocExtractor {
	t.Helper()
	x, err := NewDocDialectExtractor("git-hooks-config")
	if err != nil {
		t.Fatalf("NewDocDialectExtractor(git-hooks-config): %v", err)
	}
	return x
}

// githooksFixtureRel is the path a fixture is read at: repo/ and the basename
// after the fixture's "__".
func githooksFixtureRel(t *testing.T, fixture string) string {
	t.Helper()
	_, base, ok := strings.Cut(filepath.Base(fixture), "__")
	if !ok || base == "" {
		t.Fatalf("%s: a fixture is named <case>__<basename>", fixture)
	}
	return "repo/" + base
}

func githooksFixtures(t *testing.T) []string {
	t.Helper()
	dir := filepath.Join("dialects", "git-hooks-config", "testdata")
	ents, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read %s: %v", dir, err)
	}
	var out []string
	for _, e := range ents {
		if e.IsDir() || strings.HasSuffix(e.Name(), ".golden.json") {
			continue
		}
		out = append(out, filepath.Join(dir, e.Name()))
	}
	sort.Strings(out)
	if len(out) == 0 {
		t.Fatalf("%s: no fixtures", dir)
	}
	return out
}

// TestGitHooksDialectGoldens: every testdata fixture's hooks, commands,
// scripts, jobs and pre-commit hooks against a hand-written golden.
func TestGitHooksDialectGoldens(t *testing.T) {
	x := loadGitHooksExtractor(t)
	for _, fx := range githooksFixtures(t) {
		t.Run(filepath.Base(fx), func(t *testing.T) {
			rel := githooksFixtureRel(t, fx)
			src, err := os.ReadFile(fx)
			if err != nil {
				t.Fatal(err)
			}
			res, err := x.ExtractDoc("svc", rel, src)
			if err != nil {
				t.Fatalf("ExtractDoc: %v", err)
			}
			got := githooksGolden{Symbols: []githooksGoldenSymbol{}}
			for _, n := range res.Nodes {
				if n.Service != "svc" || n.FilePath != rel {
					t.Fatalf("%s: plumbing changed: service %q file %q", fx, n.Service, n.FilePath)
				}
				got.Symbols = append(got.Symbols, githooksGoldenSymbol{
					Name: n.Name, Kind: string(n.Kind), Package: n.Package,
					Line: n.Line, EndLine: n.EndLine, Signature: n.Signature,
					Calls: n.Calls, Tier: tierName(n.Tier),
				})
			}
			if len(res.CrossFile) != 0 {
				t.Errorf("%s: cross_file %+v; the dialect declares none (L1)", fx, res.CrossFile)
			}
			gp := fx + ".golden.json"
			raw, err := os.ReadFile(gp)
			if err != nil {
				t.Fatalf("read golden: %v", err)
			}
			var want githooksGolden
			if err := json.Unmarshal(raw, &want); err != nil {
				t.Fatalf("%s: %v", gp, err)
			}
			if want.Symbols == nil {
				want.Symbols = []githooksGoldenSymbol{}
			}
			if !reflect.DeepEqual(got.Symbols, want.Symbols) {
				t.Errorf("%s: symbols\n got: %+v\nwant: %+v", fx, got.Symbols, want.Symbols)
			}
		})
	}
}

// TestGitHooksDialectReadsByBasename: the entity specs are per format, so a
// lefthook config's shape in a .pre-commit-config.yaml (or the reverse) yields
// nothing, and every lefthook spelling reads the same.
func TestGitHooksDialectReadsByBasename(t *testing.T) {
	x := loadGitHooksExtractor(t)
	lefthook := "pre-commit:\n  commands:\n    lint:\n      run: x\n"
	precommit := "repos:\n  - repo: local\n    hooks:\n      - id: lint\n        entry: x\n        language: system\n"
	for _, c := range []struct {
		rel, src string
		want     []string
	}{
		{"lefthook.yml", lefthook, []string{"LefthookHook:pre-commit", "LefthookCommand:lint"}},
		{"a/b/.lefthook.yaml", lefthook, []string{"LefthookHook:pre-commit", "LefthookCommand:lint"}},
		{".lefthook-local.yml", lefthook, []string{"LefthookHook:pre-commit", "LefthookCommand:lint"}},
		{".pre-commit-config.yaml", lefthook, nil},
		{".pre-commit-config.yaml", precommit, []string{"PreCommitHook:lint"}},
		{"lefthook.yml", precommit, nil},
		{".pre-commit-hooks.yaml", precommit, nil},
	} {
		res, err := x.ExtractDoc("svc", c.rel, []byte(c.src))
		if err != nil {
			t.Fatal(err)
		}
		var got []string
		for _, n := range res.Nodes {
			got = append(got, string(n.Kind)+":"+n.Name)
		}
		if !reflect.DeepEqual(got, c.want) {
			t.Errorf("%s: got %q, want %q", c.rel, got, c.want)
		}
	}
}
