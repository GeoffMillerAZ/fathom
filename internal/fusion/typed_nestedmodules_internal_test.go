package fusion

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestTypedModulesFindsNestedModulesTheConnectorKeeps(t *testing.T) {
	dir := t.TempDir()
	write := func(rel, content string) {
		t.Helper()
		p := filepath.Join(dir, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("go.mod", "module example.com/r\n\ngo 1.21\n")
	write("r.go", "package r\n")
	write("kits/a/go.mod", "module example.com/r/kits/a\n\ngo 1.21\n")
	write("kits/a/x.go", "package a\n")
	write("kits/a/inner/y.go", "package inner\n")
	write("kits/a/sub/go.mod", "module example.com/r/kits/a/sub\n\ngo 1.21\n")
	write("kits/a/sub/z.go", "package sub\n")
	write("kits/ignored/go.mod", "module example.com/r/kits/ignored\n\ngo 1.21\n")
	write("kits/ignored/i.go", "package ignored\n")
	write("pkg/testdata/m/go.mod", "module example.com/fixture\n\ngo 1.21\n")
	write("pkg/testdata/m/w.go", "package m\n")
	write("_tools/go.mod", "module example.com/tools\n\ngo 1.21\n")
	write("_tools/t.go", "package tools\n")
	write("kits/v/go.mod", "module example.com/r/kits/v/v2\n\ngo 1.21\n")
	write("kits/v/v.go", "package v\n")
	write("oracles/o/go.mod", "module example.org/oracles/o\n\ngo 1.21\n")
	write("oracles/o/o.go", "package main\n")

	allowed := map[string]bool{
		"r.go": true, "kits/a/x.go": true, "kits/a/inner/y.go": true, "kits/a/sub/z.go": true,
		"pkg/testdata/m/w.go": true, "_tools/t.go": true, "kits/v/v.go": true, "oracles/o/o.go": true,
	}
	got, foreign := typedModules(dir, allowed)
	want := []goModule{
		{rel: ".", path: "example.com/r"},
		{rel: "kits/a", path: "example.com/r/kits/a", expected: 2},
		{rel: "kits/a/sub", path: "example.com/r/kits/a/sub", expected: 1},
		{rel: "kits/v", path: "example.com/r/kits/v/v2", expected: 1},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("typedModules(allowed) = %+v\nwant %+v", got, want)
	}
	// A module whose path is not the repository's own is disclosed, not loaded.
	if wantF := []goModule{{rel: "oracles/o", path: "example.org/oracles/o", expected: 1, skip: skipNotOwn}}; !reflect.DeepEqual(foreign, wantF) {
		t.Errorf("typedModules(allowed) foreign = %+v\nwant %+v", foreign, wantF)
	}

	// Whole-module scope walks the tree, still skipping testdata and "_".
	var rels []string
	all, _ := typedModules(dir, nil)
	for _, m := range all {
		rels = append(rels, m.rel)
	}
	if want := []string{".", "kits/a", "kits/a/sub", "kits/ignored", "kits/v"}; !reflect.DeepEqual(rels, want) {
		t.Errorf("typedModules(nil) = %v, want %v", rels, want)
	}

	if !hasTypedGoModule(dir, allowed) {
		t.Error("hasTypedGoModule: false for a root module")
	}
	if err := os.Remove(filepath.Join(dir, "go.mod")); err != nil {
		t.Fatal(err)
	}
	if !hasTypedGoModule(dir, allowed) {
		t.Error("hasTypedGoModule: false for a repository whose modules are all nested")
	}
	// With no root module, every nested module is the repository's own.
	if mods, foreign := typedModules(dir, allowed); len(foreign) != 0 || len(mods) != 4 {
		t.Errorf("no root go.mod: mods %+v foreign %+v, want all four nested modules loaded", mods, foreign)
	}
	if hasTypedGoModule(dir, map[string]bool{"r.go": true}) {
		t.Error("hasTypedGoModule: true with no module governing an allowed file")
	}
}

func TestTypedStoreKeysKeepModulesApart(t *testing.T) {
	root := goModule{rel: "."}
	a := goModule{rel: "kits/a"}
	ab := goModule{rel: "kits/a/b"}
	if got := root.typedStoreKey("example.com/r/x"); got != "example.com/r/x" {
		t.Errorf("root key %q: the root module keeps the bare package ID", got)
	}
	if got := a.typedStoreKey("example.com/cli [example.com/cli.test]"); got != "gomod:kits/a//example.com/cli [example.com/cli.test]" {
		t.Errorf("nested key %q", got)
	}
	if isNamespacedTypedKey(a.typedStoreKey("p")) {
		t.Error("a nested module's key reads as another pass's namespace: the Go reconcile would never purge it")
	}
	// A failed module protects its own rows, never a sibling's whose
	// directory it prefixes.
	if !protectedTypedKey(a.typedStoreKey("p"), []string{a.storeKeyPrefix()}, false) {
		t.Error("kits/a failed: its row is not protected")
	}
	if protectedTypedKey(ab.typedStoreKey("p"), []string{a.storeKeyPrefix()}, false) {
		t.Error("kits/a failed: kits/a/b's row is protected too")
	}
	if !protectedTypedKey("example.com/r/x", nil, true) || protectedTypedKey(a.typedStoreKey("p"), nil, true) {
		t.Error("root failed: must protect exactly the un-prefixed keys")
	}
}

func TestOwnNestedModuleFollowsGoSubdirectoryRule(t *testing.T) {
	for _, tc := range []struct {
		root, path, rel string
		own             bool
	}{
		{"github.com/o/r", "github.com/o/r/kits/a", "kits/a", true},
		{"github.com/o/r", "github.com/o/r/kits/a/v3", "kits/a", true},
		{"github.com/o/r/v2", "github.com/o/r/tools", "tools", true},
		{"github.com/o/r", "github.com/o/r/kits/b", "kits/a", false},
		{"github.com/o/r", "fathom.bench/oracles/cue", "calibration/oracles/cue", false},
		{"github.com/o/r", "github.com/spf13/cobra", "kits/a/third_party/cobra", false},
		{"github.com/o/r", "", "kits/a", false},
	} {
		if got := ownNestedModule(tc.root, goModule{rel: tc.rel, path: tc.path}); got != tc.own {
			t.Errorf("ownNestedModule(%q, %q at %s) = %v, want %v", tc.root, tc.path, tc.rel, got, tc.own)
		}
	}
	six := []goModule{{rel: "a"}, {rel: "b"}, {rel: "c"}, {rel: "d"}, {rel: "e"}, {rel: "f"}}
	if line := foreignModulesLine("s", skippedModuleNames(six)); !strings.Contains(line, "6 nested Go module(s)") || !strings.Contains(line, "and 1 more") {
		t.Errorf("foreignModulesLine = %q", line)
	}
	if foreignModulesLine("s", nil) != "" {
		t.Error("foreignModulesLine: a line with nothing to disclose")
	}
}

// TestTypedModulesOwnModuleRule is EW-62 (K26 condition b): the own-module
// rule keeps the common shapes' modules. A committed go.work's use list
// declares own modules, whatever their module path; with no root go.mod
// every nested module outside Go's ignored directories and the vendored
// trees (custodian.VendoredPath) is own; every module skipped is named.
func TestTypedModulesOwnModuleRule(t *testing.T) {
	dir := t.TempDir()
	write := func(rel, content string) {
		t.Helper()
		p := filepath.Join(dir, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("go.mod", "module example.com/r\n\ngo 1.22\n")
	write("r.go", "package r\n")
	write("go.work", "go 1.22\n\nuse (\n\t.\n\t./tools/gen\n\t../sibling\n\t./missing\n)\n\nuse ./kits/c\n")
	write("tools/gen/go.mod", "module example.org/gen\n\ngo 1.22\n")
	write("tools/gen/g.go", "package main\n")
	write("kits/c/go.mod", "module example.com/r/kits/c\n\ngo 1.22\n")
	write("kits/c/c.go", "package c\n")
	write("oracles/o/go.mod", "module example.net/oracle\n\ngo 1.22\n")
	write("oracles/o/o.go", "package main\n")
	rels := func(ms []goModule) []string {
		out := []string{}
		for _, m := range ms {
			out = append(out, m.rel)
		}
		return out
	}
	goFiles := map[string]bool{"r.go": true, "tools/gen/g.go": true, "kits/c/c.go": true, "oracles/o/o.go": true}
	withWork := map[string]bool{"go.work": true}
	for k := range goFiles {
		withWork[k] = true
	}

	// The connector discovered the go.work: its in-repo use list is own.
	mods, foreign := typedModules(dir, withWork)
	if got, want := rels(mods), []string{".", "kits/c", "tools/gen"}; !reflect.DeepEqual(got, want) {
		t.Errorf("committed go.work: loaded %v, want %v", got, want)
	}
	if got, want := rels(foreign), []string{"oracles/o"}; !reflect.DeepEqual(got, want) {
		t.Errorf("committed go.work: skipped %v, want %v", got, want)
	}
	// Whole-module scope reads a go.work on disk.
	if mods, _ := typedModules(dir, nil); !reflect.DeepEqual(rels(mods), []string{".", "kits/c", "tools/gen"}) {
		t.Errorf("nil allowed: loaded %v, want the go.work's use list honoured", rels(mods))
	}
	// A go.work the connector did not discover (a developer's ignored one)
	// declares nothing: only the module-path rule stands.
	if mods, foreign := typedModules(dir, goFiles); !reflect.DeepEqual(rels(mods), []string{".", "kits/c"}) || !reflect.DeepEqual(rels(foreign), []string{"oracles/o", "tools/gen"}) {
		t.Errorf("undiscovered go.work: loaded %v skipped %v", rels(mods), rels(foreign))
	}
	// A go.work is no Go file the root load expects.
	if err := zeroPackagesDisclosed(dir, map[string]bool{"go.work": true}, 0); err != nil {
		t.Errorf("zeroPackagesDisclosed counted go.work as a Go file: %v", err)
	}

	// No root go.mod: every nested module outside Go's ignored directories
	// and the vendored trees is own; the vendored ones are named.
	nr := t.TempDir()
	dir = nr
	write("svc/api/go.mod", "module example.com/api\n\ngo 1.22\n")
	write("svc/api/a.go", "package api\n")
	write("third_party/fork/go.mod", "module github.com/upstream/fork\n\ngo 1.22\n")
	write("third_party/fork/f.go", "package fork\n")
	write("kits/k/third_party/cli/go.mod", "module example.com/cli\n\ngo 1.22\n")
	write("kits/k/third_party/cli/c.go", "package cli\n")
	write("node_modules/x/go.mod", "module example.com/x\n\ngo 1.22\n")
	write("node_modules/x/x.go", "package x\n")
	write("_scratch/go.mod", "module example.com/scratch\n\ngo 1.22\n")
	write("_scratch/s.go", "package scratch\n")
	allowed := map[string]bool{"svc/api/a.go": true, "third_party/fork/f.go": true, "kits/k/third_party/cli/c.go": true, "node_modules/x/x.go": true, "_scratch/s.go": true}
	for _, scope := range []map[string]bool{allowed, nil} {
		mods, foreign := typedModules(nr, scope)
		if got, want := rels(mods), []string{"svc/api"}; !reflect.DeepEqual(got, want) {
			t.Errorf("no root go.mod (allowed nil=%v): loaded %v, want %v", scope == nil, got, want)
		}
		if got, want := rels(foreign), []string{"kits/k/third_party/cli", "node_modules/x", "third_party/fork"}; !reflect.DeepEqual(got, want) {
			t.Errorf("no root go.mod (allowed nil=%v): skipped %v, want %v", scope == nil, got, want)
		}
		for _, m := range foreign {
			if m.skip != skipVendored {
				t.Errorf("%s skipped as %q, want %q", m.rel, m.skip, skipVendored)
			}
		}
	}

	// The one line names every skipped module and why.
	line := foreignModulesLine("s", skippedModuleNames([]goModule{
		{rel: "oracles/o", skip: skipNotOwn},
		{rel: "third_party/fork", skip: skipVendored},
	}))
	for _, want := range []string{"2 nested Go module(s) not loaded", "oracles/o/go.mod (module path not the repository's)", "third_party/fork/go.mod (vendored)"} {
		if !strings.Contains(line, want) || strings.Count(line, "\n") != 1 {
			t.Errorf("foreignModulesLine = %q, want one line containing %q", line, want)
		}
	}
}
