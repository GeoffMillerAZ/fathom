package fusion

import (
	"os"
	"path/filepath"
	"reflect"
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

	allowed := map[string]bool{
		"r.go": true, "kits/a/x.go": true, "kits/a/inner/y.go": true, "kits/a/sub/z.go": true,
		"pkg/testdata/m/w.go": true, "_tools/t.go": true,
	}
	got := typedModules(dir, allowed)
	want := []goModule{
		{rel: ".", path: "example.com/r"},
		{rel: "kits/a", path: "example.com/r/kits/a", expected: 2},
		{rel: "kits/a/sub", path: "example.com/r/kits/a/sub", expected: 1},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("typedModules(allowed) = %+v\nwant %+v", got, want)
	}

	// Whole-module scope walks the tree, still skipping testdata and "_".
	var rels []string
	for _, m := range typedModules(dir, nil) {
		rels = append(rels, m.rel)
	}
	if want := []string{".", "kits/a", "kits/a/sub", "kits/ignored"}; !reflect.DeepEqual(rels, want) {
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
