package lang_test

import (
	"context"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/extractor"
	"github.com/geoffmilleraz/fathom/pkg/lang"
)

const dialectsDir = "../../internal/extractor/dialects"

// fixture is one dialect fixture file under its dialect's testdata directory,
// named by its path inside that directory.
type fixture struct {
	dir, rel string
	src      []byte
}

// dialectFixtures reads every fixture of every built-in dialect (a file under
// dialects/<id>/testdata that is not a golden), sorted.
func dialectFixtures(t *testing.T) []fixture {
	t.Helper()
	var out []fixture
	err := filepath.WalkDir(dialectsDir, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.Contains(p, "/testdata/") || strings.HasSuffix(p, ".golden.json") {
			return err
		}
		b, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		parts := strings.SplitN(strings.TrimPrefix(p, dialectsDir+"/"), "/testdata/", 2)
		out = append(out, fixture{dir: parts[0], rel: parts[1], src: b})
		return nil
	})
	if err != nil || len(out) < 300 {
		t.Fatalf("read %d dialect fixtures: %v", len(out), err)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].dir+"/"+out[i].rel < out[j].dir+"/"+out[j].rel })
	return out
}

// ownSamples are the dialects whose fixtures are not named the way a
// repository names the file (the dialect claims Cargo.toml, a fixture is
// basic.toml), so each gets a sample under the path that claims it.
var ownSamples = map[string]struct{ path, src string }{
	"cargo":            {"Cargo.toml", "[package]\nname = \"x\"\nversion = \"0.1.0\"\n\n[dependencies]\nserde = \"1\"\n"},
	"circleci":         {".circleci/config.yml", "version: 2.1\njobs:\n  build:\n    docker:\n      - image: cimg/base:2020.01\n    steps:\n      - checkout\nworkflows:\n  main:\n    jobs:\n      - build\n"},
	"compile-commands": {"compile_commands.json", `[{"directory":"/b","command":"cc -c a.c","file":"a.c"}]`},
	"composer":         {"composer.json", `{"name":"a/b","require":{"php":">=8.0"}}`},
	"git-hooks-config": {".pre-commit-config.yaml", "repos:\n- repo: https://example.com/hooks\n  rev: v1\n  hooks:\n  - id: lint\n"},
	"github-actions":   {".github/workflows/ci.yml", "name: ci\non: push\njobs:\n  a:\n    runs-on: ubuntu-latest\n    steps:\n      - run: echo\n"},
	"gitlab-ci":        {".gitlab-ci.yml", "stages:\n  - build\nbuild-job:\n  stage: build\n  script:\n    - make\n"},
	"go":               {"a/a.go", "package a\n\nfunc F() { G() }\n\nfunc G() {}\n"},
	"go-mod":           {"go.mod", "module example.com/m\n\ngo 1.22\n\nrequire example.com/x v1.0.0\n"},
	"kustomize":        {"kustomization.yaml", "resources:\n- deploy.yaml\n"},
	"maven":            {"pom.xml", `<project><modelVersion>4.0.0</modelVersion><groupId>g</groupId><artifactId>a</artifactId><version>1</version></project>`},
	"meson":            {"meson.build", "project('p', 'c')\nexecutable('e', 'e.c')\n"},
	"package-json":     {"package.json", `{"name":"web","version":"1.0.0","scripts":{"build":"tsc"}}`},
	"python-packaging": {"pyproject.toml", "[project]\nname = \"x\"\nversion = \"1\"\n"},
	"unity-asmdef":     {"Assets/Core.asmdef", `{"name":"Core","references":[]}`},
}

// F3: Dialects lists what Extract can read. Each listed dialect reads a sample
// of its own (a fixture, or a sample under the path that claims it) and the
// file is not skipped.
func TestEveryListedDialectReadsASampleOfItsOwn(t *testing.T) {
	r := newReader(t)
	fixtures := dialectFixtures(t)
	for _, d := range r.Dialects() {
		var read bool
		var tried []string
		if s, ok := ownSamples[d.ID]; ok {
			got := extract(t, r, s.path, s.src)
			read = got.Dialect == d.ID && got.Skipped == ""
			tried = append(tried, s.path+" -> "+got.Dialect+"/"+got.Skipped)
		} else {
			for _, f := range fixtures {
				if f.dir != d.ID {
					continue
				}
				got, err := r.Extract(context.Background(), lang.File{Path: f.rel, Src: f.src})
				if err != nil {
					t.Fatal(err)
				}
				if read = got.Dialect == d.ID && got.Skipped == ""; read {
					break
				}
				tried = append(tried, f.rel+" -> "+got.Dialect+"/"+got.Skipped)
			}
		}
		if !read {
			t.Errorf("Dialects lists %s (level %s) but Extract reads none of its samples: %v", d.ID, d.Level, tried)
		}
	}
}

// A dialect the product refuses, or only the product reads, is not listed, and
// a sample of its own is skipped or read by another reader.
func TestDialectsOmitsWhatTheDoorCannotRead(t *testing.T) {
	listed := map[string]bool{}
	for _, d := range newReader(t).Dialects() {
		listed[d.ID] = true
	}
	for _, id := range []string{"swift", "tcl", "vimscript", "fathom-config", "fathom-plan-rows", "fathom-concerns", "fathom-layer-packs"} {
		if listed[id] {
			t.Errorf("Dialects lists %s, which Extract cannot read", id)
		}
	}
	if !listed["go"] || !listed["terraform"] || !listed["python"] {
		t.Errorf("Dialects lost a readable dialect: %v", listed)
	}
}

// F4 and F5: every symbol of every dialect fixture carries a Canonical value,
// but for the kinds extractor.HasNoCanonical lists with a reason, and every
// reference names an edge and a ToKey that is a symbol of the file or empty.
func TestFixturesHoldTheResultContract(t *testing.T) {
	r := newReader(t)
	empty := map[string]bool{}
	scan := func(path string, src []byte) {
		got, err := r.Extract(context.Background(), lang.File{Path: path, Src: src})
		if err != nil {
			t.Fatal(err)
		}
		keys := map[string]bool{}
		for _, s := range got.Symbols {
			keys[s.Key] = true
			if s.Canonical == "" {
				empty[s.Kind] = true
			}
		}
		for _, ref := range got.References {
			if ref.Edge == "" || ref.Line < 0 {
				t.Errorf("%s: reference %+v has no edge or a negative line", path, ref)
			}
			if ref.ToKey != "" && !keys[ref.ToKey] {
				t.Errorf("%s: reference %+v names a ToKey that is no symbol's Key", path, ref)
			}
			if ref.ToKey != "" && ref.Strategy != "" {
				t.Errorf("%s: cross-file reference %+v carries a ToKey", path, ref)
			}
		}
	}
	for _, f := range dialectFixtures(t) {
		scan(f.rel, f.src)
	}
	for _, s := range ownSamples {
		scan(s.path, []byte(s.src))
	}
	scan("data.json", []byte(`{"a": 1}`))
	scan("data.toml", []byte("a = 1\n[t]\nb = 2\n"))
	scan("a/a.go", []byte("package a\n\ntype T struct{ F int }\ntype I interface{ M() }\nconst C = 1\nvar V = 2\nfunc (T) M() {}\nfunc F() {}\n"))
	for kind := range empty {
		if _, ok := extractor.HasNoCanonical(kind); !ok {
			t.Errorf("kind %s reads an empty Canonical and is not on the no-canonical list", kind)
		}
	}
	for _, want := range []string{"GoFunc", "GoType", "GoField", "GoConst", "GoMethod", "GoVar", "GoInterface", "JSONKey", "TOMLKey", "TOMLTable"} {
		got := map[string]string{}
		for _, src := range []struct{ p, s string }{
			{"a/a.go", "package a\n\ntype T struct{ F int }\ntype I interface{ M() }\nconst C = 1\nvar V = 2\nfunc (T) M() {}\nfunc F() {}\n"},
			{"d.json", `{"a": 1}`}, {"d.toml", "a = 1\n[t]\nb = 2\n"},
		} {
			for _, s := range extract(t, r, src.p, src.s).Symbols {
				got[s.Kind] = s.Canonical
			}
		}
		if got[want] == "" {
			t.Errorf("kind %s reads Canonical %q", want, got[want])
		}
	}
}
