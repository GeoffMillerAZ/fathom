package fusion

// resolve_cuemodule_test.go — the cue-module strategy (LB-F583) on its own,
// through the file resolver (a reference through an import binds the file of
// the package defining the name, and the import itself each file it loads),
// and the CUE extractor as the resolver's FileSpecExtractor.

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/extractor"
)

// cueModuleCorpus is a module (example.org/app, v0.9+ form) with a package
// split across files and directories, a second package beside it, files CUE
// leaves out of a package, vendored and generated dependencies, a nested
// module of the legacy form, and a file outside every module.
func cueModuleCorpus() map[string]string {
	return map[string]string{
		"cue.mod/module.cue":                       "module: \"example.org/app@v0\"\nlanguage: version: \"v0.9.0\"\n",
		"root.cue":                                 "package base\n\n#R: 1\n",
		"base/base.cue":                            "package base\n\n#A: {x: int}\n",
		"base/more.cue":                            "package base\n\n#B: string\n",
		"base/other.cue":                           "package other\n\n#O: 1\n",
		"base/_hidden.cue":                         "package base\n\n#H: 1\n",
		"base/.dot.cue":                            "package base\n\n#D: 1\n",
		"base/gen_tool.cue":                        "package base\n\ncommand: x: {}\n",
		"base/base_test.cue":                       "package base\n\n#T: 1\n",
		"base/sub/sub.cue":                         "package base\n\n#S: 1\n",
		"base/empty/note.cue":                      "package notes\n\nn: 1\n",
		"nopkg/data.cue":                           "a: 1\n",
		"kit/kit.cue":                              kitCUE,
		"kit/shared.cue":                           "package kit\n\n#Shared: 1\n",
		"cue.mod/pkg/example.org/vend/vend.cue":    "package vend\n\nimport \"example.org/app/base\"\n\n#V: base.#A\n",
		"cue.mod/gen/k8s.io/api/core/v1/types.cue": "package v1\n\n#Pod: {}\n",
		"cue.mod/usr/k8s.io/api/core/v1/extra.cue": "package v1\n\n#Pod: {spec: {}}\n",
		// decoys: a standard-library path is the cue binary's builtin and is
		// never looked up in cue.mod, so these bind nothing
		"cue.mod/pkg/strings/strings.cue":    "package strings\n\nToUpper: _\n",
		"cue.mod/gen/encoding/json/json.cue": "package json\n\n#J: 1\n",
		"nested/cue.mod/module.cue":          "module: \"example.org/nested\"\n",
		"nested/n/n.cue":                     "package n\n\n#N: 1\n",
		"nested/main.cue":                    "package main\n\nimport \"example.org/nested/n\"\n\nx: n.#N\n",
		"outside/cue.cue":                    "package outside\n",
	}
}

const kitCUE = `package kit

import (
	"strings"
	"example.org/app/base"
	o "example.org/app/base:other"
	"example.org/vend"
	"k8s.io/api/core/v1"
	"cue.dev/x/githubactions@v0"
)

#Kit: {
	a:   base.#A
	b:   base.#B
	r:   base.#R
	oo:  o.#O
	v:   vend.#V
	pod: v1.#Pod
	up:  strings.ToUpper("x")
	gh:  githubactions.#Step
}
`

// cueModuleEnv is the corpus' resolution universe, its package index built
// from the CUE extractor's own symbols.
func cueModuleEnv(t *testing.T) (*resolveEnv, []string, []core.SymbolNode) {
	t.Helper()
	corpus := cueModuleCorpus()
	paths := make([]string, 0, len(corpus))
	for p := range corpus {
		paths = append(paths, p)
	}
	sort.Strings(paths)
	var syms []core.SymbolNode
	x := extractor.NewCUEExtractor()
	for _, p := range paths {
		nodes, err := x.Extract("svc", p, []byte(corpus[p]))
		if err != nil {
			t.Fatal(err)
		}
		syms = append(syms, nodes...)
	}
	read := func(p string) ([]byte, bool) {
		b, ok := corpus[p]
		return []byte(b), ok
	}
	env := newResolveEnv("", paths, read)
	env.indexCUEPackages(syms)
	return env, paths, syms
}

// TestResolveCUEModuleBindsPackageFiles: an import path names the files of
// the package the cue binary loads for it, by the module's own rules.
func TestResolveCUEModuleBindsPackageFiles(t *testing.T) {
	env, _, _ := cueModuleEnv(t)
	baseFiles := []string{"base/base.cue", "base/more.cue", "root.cue"}
	for _, c := range []struct {
		importer, module string
		want             []string
	}{
		// in the module: the package directory's files of the package (the
		// path's last element), and those of its ancestors up to the module
		// root; never a _, ., _test.cue or _tool.cue file
		{"kit/kit.cue", "example.org/app/base", baseFiles},
		{"kit/kit.cue", "example.org/app/base@v0", baseFiles},
		{"kit/kit.cue", "example.org/app/base:base", baseFiles},
		{"kit/kit.cue", "example.org/app/base:other", []string{"base/other.cue"}},
		{"kit/kit.cue", "example.org/app/base/sub:base", []string{"base/base.cue", "base/more.cue", "base/sub/sub.cue", "root.cue"}},
		{"kit/kit.cue", "example.org/app/kit", []string{"kit/kit.cue", "kit/shared.cue"}},
		// a directory with no file of the package walks no ancestor
		{"kit/kit.cue", "example.org/app/base/empty", nil},
		{"kit/kit.cue", "example.org/app/missing", nil},
		{"kit/kit.cue", "example.org/app/nopkg", nil},
		// outside the module: cue.mod/pkg, cue.mod/gen and cue.mod/usr, read as one
		{"kit/kit.cue", "example.org/vend", []string{"cue.mod/pkg/example.org/vend/vend.cue"}},
		{"kit/kit.cue", "k8s.io/api/core/v1", []string{"cue.mod/gen/k8s.io/api/core/v1/types.cue", "cue.mod/usr/k8s.io/api/core/v1/extra.cue"}},
		// a file under cue.mod belongs to the module holding it
		{"cue.mod/pkg/example.org/vend/vend.cue", "example.org/app/base", baseFiles},
		// the standard library and a registry dependency bind nothing
		{"kit/kit.cue", "strings", nil},
		{"kit/kit.cue", "encoding/json", nil},
		{"kit/kit.cue", "cue.dev/x/githubactions@v0", nil},
		// the nearest module: a nested one of the legacy form
		{"nested/main.cue", "example.org/nested/n", []string{"nested/n/n.cue"}},
		{"kit/kit.cue", "example.org/nested/n", nil},
		// .:name is the package name instantiated at the importer's own
		// directory: what a file re-exports (its package's other files)
		{"base/more.cue", ".:base", baseFiles},
		{"base/sub/sub.cue", ".:base", []string{"base/base.cue", "base/more.cue", "base/sub/sub.cue", "root.cue"}},
		{"cue.mod/usr/k8s.io/api/core/v1/extra.cue", ".:v1", []string{"cue.mod/gen/k8s.io/api/core/v1/types.cue", "cue.mod/usr/k8s.io/api/core/v1/extra.cue"}},
	} {
		if got := resolveByStrategy("cue-module", extractor.ImportParams{}, env, c.importer, c.module); !reflect.DeepEqual(got, c.want) {
			t.Errorf("%s from %s resolved to %q, want %q", c.module, c.importer, got, c.want)
		}
	}
}

// TestResolveFileEdges_CUEModule: through the file resolver, an import is an
// edge from the import entity to the package entity of each file it loads,
// and a reference through the import binds the file of the package defining
// the name; the standard library, a registry dependency and the package's
// own files bind nothing.
func TestResolveFileEdges_CUEModule(t *testing.T) {
	dir := t.TempDir()
	corpus := cueModuleCorpus()
	var files []core.SourceFile
	var syms []core.SymbolNode
	x := extractor.NewCUEExtractor()
	paths := make([]string, 0, len(corpus))
	for p := range corpus {
		paths = append(paths, p)
	}
	sort.Strings(paths)
	for _, rel := range paths {
		src := []byte(corpus[rel])
		p := filepath.Join(dir, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, src, 0o644); err != nil {
			t.Fatal(err)
		}
		files = append(files, core.SourceFile{Path: rel, Content: src})
		nodes, err := x.Extract("svc", rel, src)
		if err != nil {
			t.Fatal(err)
		}
		syms = append(syms, nodes...)
	}
	edges, rep := ResolveFileEdges(context.Background(), "cue", dir, files, syms)
	if rep.Skipped {
		t.Fatalf("skipped: %s", rep.SkipReason)
	}
	name := map[string]string{}
	for _, s := range syms {
		name[s.Key()] = s.Name
	}
	// Two files of one package in one directory share their package
	// entity's key (core.SymbolNode.Key: directory and package), so an
	// import's edges are one per directory of the package.
	got := map[string]bool{}
	for _, e := range edges {
		if e.FromFile != "kit/kit.cue" {
			continue
		}
		got[name[e.FromSymbol]+" -> "+strings.TrimPrefix(e.ToSymbol, "svc::")] = true
	}
	for _, want := range []string{
		"example.org/app/base -> base/base.package/base",
		"example.org/app/base -> base.package/base", // root.cue
		"example.org/app/base:other -> base/other.package/other",
		"example.org/vend -> cue.mod/pkg/example.org/vend/vend.package/vend",
		"k8s.io/api/core/v1 -> cue.mod/gen/k8s.io/api/core/v1/v1.package/v1",
		"k8s.io/api/core/v1 -> cue.mod/usr/k8s.io/api/core/v1/v1.package/v1",
		"#Kit.a -> base/base.#A",
		"#Kit.b -> base/base.#B", // more.cue's: the key is the directory's
		"#Kit.r -> base.#R",
		"#Kit.oo -> base/other.#O",
		"#Kit.v -> cue.mod/pkg/example.org/vend/vend.#V",
		"#Kit.pod -> cue.mod/gen/k8s.io/api/core/v1/v1.#Pod", // the bound file's own declaration (usr's unifies with it)
	} {
		if !got[want] {
			t.Errorf("missing edge %s", want)
		}
	}
	if len(got) != 12 {
		t.Errorf("%d edges from kit/kit.cue, want 12: %v", len(got), got)
	}
	// the definition binds the file defining it, wherever the package splits
	byFile := map[string]bool{}
	for _, e := range edges {
		if e.FromFile == "kit/kit.cue" {
			for _, s := range syms {
				if s.Key() == e.ToSymbol && strings.HasPrefix(s.Name, "#") {
					byFile[s.FilePath] = true
				}
			}
		}
	}
	for _, f := range []string{"base/base.cue", "base/more.cue", "root.cue", "base/other.cue"} {
		if !byFile[f] {
			t.Errorf("no reference edge reaches %s", f)
		}
	}
}

// TestCUEExtractorIsFileSpecExtractor: the CUE dialect's own reader parses
// its imports for the file resolver (one whole-package import per
// declaration, named by the package's identifier the extractor's references
// use) and re-exports its own package.
func TestCUEExtractorIsFileSpecExtractor(t *testing.T) {
	e, err := extractor.NewFileSpecExtractor("cue")
	if err != nil {
		t.Fatal(err)
	}
	defer e.Close()
	if e.Name() != "cue" {
		t.Fatalf("Name = %q", e.Name())
	}
	fs, err := e.ExtractFileSpecs("kit/kit.cue", []byte(kitCUE))
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, s := range fs.Imports {
		got = append(got, s.LocalName+"="+s.Module)
		if s.Kind != extractor.ImportWhole {
			t.Errorf("%s: kind %v, want a whole import", s.Module, s.Kind)
		}
	}
	want := []string{"strings=strings", "base=example.org/app/base", "other=example.org/app/base:other", "vend=example.org/vend", "v1=k8s.io/api/core/v1", "githubactions=cue.dev/x/githubactions@v0"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("imports %q, want %q", got, want)
	}
	if len(fs.Exports) != 1 || fs.Exports[0].Module != ".:kit" || fs.Exports[0].Line != 1 {
		t.Errorf("exports %+v, want the file's own package .:kit at line 1", fs.Exports)
	}
	spans, err := e.ExtractImportSpans("kit/kit.cue", []byte(kitCUE))
	if err != nil {
		t.Fatal(err)
	}
	if len(spans) != len(want) || spans[1].StartLine != 5 || spans[1].EndLine != 5 {
		t.Errorf("spans %+v", spans)
	}
	langs, err := extractor.FileResolveLangs()
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, l := range langs {
		if l.Lang == "cue" {
			found = l.Strategy == "cue-module" && reflect.DeepEqual(l.Extensions, []string{".cue"})
		}
	}
	if !found {
		t.Error("the file resolver does not serve cue by the cue-module strategy")
	}
}

// TestResolveFileEdges_CUEWidePackage: every file of a package re-exports
// the package, so a lookup through an import reads each file's own
// definitions once (a flat re-export), never the re-exports of re-exports: a
// package of forty files resolves at once (it walked up to 40^8 paths
// before, past any test deadline).
func TestResolveFileEdges_CUEWidePackage(t *testing.T) {
	const n = 40
	files := []core.SourceFile{
		{Path: "cue.mod/module.cue", Content: []byte("module: \"example.org/w@v0\"\nlanguage: version: \"v0.9.0\"\n")},
		{Path: "app/app.cue", Content: []byte("package app\n\nimport \"example.org/w/wide\"\n\nx: wide.#D39\ny: wide.#Missing\n")},
	}
	for i := 0; i < n; i++ {
		files = append(files, core.SourceFile{Path: fmt.Sprintf("wide/f%02d.cue", i), Content: []byte(fmt.Sprintf("package wide\n\n#D%d: %d\n", i, i))})
	}
	x := extractor.NewCUEExtractor()
	var syms []core.SymbolNode
	for _, f := range files {
		nodes, err := x.Extract("svc", f.Path, f.Content)
		if err != nil {
			t.Fatal(err)
		}
		syms = append(syms, nodes...)
	}
	edges, _ := ResolveFileEdges(context.Background(), "cue", "", files, syms) // exponential before: the test's deadline
	found := false
	for _, e := range edges {
		if e.ToSymbol == "svc::wide/wide.#D39" {
			found = true
		}
		if strings.Contains(e.ToSymbol, "#Missing") {
			t.Errorf("an undefined name bound: %+v", e)
		}
	}
	if !found {
		t.Error("x: wide.#D39 bound no edge to the file defining it")
	}
}
