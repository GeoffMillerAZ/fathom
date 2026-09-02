package core

import "testing"

// TestSymbolNodeKeyCarriesPackagePath is PC-32 (docs/plans/proof-credibility.md,
// O21): two `package main` binaries with a same-named function must never
// share a key — the key is qualified by the package's DIRECTORY, not only its
// declared name, so callgraph.Build's node map cannot collapse them
// last-writer-wins.
func TestSymbolNodeKeyCarriesPackagePath(t *testing.T) {
	a := SymbolNode{Service: "svc", Package: "main", Name: "run", FilePath: "cmd/a/main.go"}
	b := SymbolNode{Service: "svc", Package: "main", Name: "run", FilePath: "cmd/b/main.go"}
	if a.Key() == b.Key() {
		t.Fatalf("two package-main run funcs collapsed to one key %q", a.Key())
	}
	if got, want := a.Key(), "svc::cmd/a/main.run"; got != want {
		t.Errorf("Key() = %q, want %q", got, want)
	}
	if got, want := a.PackagePath(), "cmd/a/main"; got != want {
		t.Errorf("PackagePath() = %q, want %q", got, want)
	}

	// A method node's Package is its receiver type; the directory still
	// qualifies it.
	m := SymbolNode{Service: "svc", Package: "Recv", Name: "Recv.Do", FilePath: "internal/x/recv.go"}
	if got, want := m.Key(), "svc::internal/x/Recv.Recv.Do"; got != want {
		t.Errorf("method Key() = %q, want %q", got, want)
	}

	// Root-level and path-less nodes keep the historical "svc::pkg.Name"
	// form: nothing to qualify by.
	root := SymbolNode{Service: "svc", Package: "pkg", Name: "F", FilePath: "f.go"}
	if got, want := root.Key(), "svc::pkg.F"; got != want {
		t.Errorf("root Key() = %q, want %q", got, want)
	}
	bare := SymbolNode{Service: "svc", Package: "pkg", Name: "F"}
	if got, want := bare.Key(), "svc::pkg.F"; got != want {
		t.Errorf("path-less Key() = %q, want %q", got, want)
	}
	// A node without a package is not qualified either: the bare form is
	// the only address such symbols have ever had.
	nopkg := SymbolNode{Service: "svc", Name: "deploy", FilePath: "k8s/app.yaml"}
	if got, want := nopkg.Key(), "svc::deploy"; got != want {
		t.Errorf("package-less Key() = %q, want %q", got, want)
	}
}

func TestQualifiedPackage(t *testing.T) {
	cases := []struct{ file, pkg, want string }{
		{"cmd/a/main.go", "main", "cmd/a/main"},
		{"main.go", "main", "main"},
		{"", "main", "main"},
		{"cmd/a/main.go", "", ""},
		{"/abs/cmd/a/main.go", "main", "abs/cmd/a/main"},
		{"internal/fusion/x_test.go", "fusion_test", "internal/fusion/fusion_test"},
	}
	for _, c := range cases {
		if got := QualifiedPackage(c.file, c.pkg); got != c.want {
			t.Errorf("QualifiedPackage(%q, %q) = %q, want %q", c.file, c.pkg, got, c.want)
		}
	}
}
