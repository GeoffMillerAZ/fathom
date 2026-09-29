package extractor

import (
	"testing"

	"github.com/geoffmilleraz/fathom/internal/core"
)

// TestDefaultBuildExcludes pins EW-15's entry rule: a file is excluded only
// when no default build (any known GOOS/GOARCH, with or without cgo, no
// custom tags) compiles it — never because of the host running the ingest.
func TestDefaultBuildExcludes(t *testing.T) {
	for _, c := range []struct {
		name, src string
		want      bool
	}{
		{"no constraint", "package main\n", false},
		{"ignore", "//go:build ignore\n\npackage main\n", true},
		{"tools", "// Code comment.\n\n//go:build tools\n\npackage tools\n", true},
		{"custom tag", "//go:build integration\n\npackage main\n", true},
		{"legacy ignore", "// +build ignore\n\npackage main\n", true},
		{"one platform", "//go:build linux\n\npackage main\n", false},
		{"not a platform", "//go:build !linux\n\npackage main\n", false},
		{"unix", "//go:build unix && cgo\n\npackage main\n", false},
		{"ios implies darwin", "//go:build darwin && arm64\n\npackage main\n", false},
		{"release tag", "//go:build go1.21\n\npackage main\n", false},
		{"custom or platform", "//go:build integration || windows\n\npackage main\n", false},
		{"unsatisfiable", "//go:build linux && !linux\n\npackage main\n", true},
		{"go:build wins over +build", "//go:build linux\n// +build ignore\n\npackage main\n", false},
		{"after the package clause", "package main\n\n//go:build ignore\n", false},
		{"inside a block comment", "/*\n//go:build ignore\n*/\npackage main\n", false},
	} {
		if got := defaultBuildExcludes([]byte(c.src)); got != c.want {
			t.Errorf("%s: defaultBuildExcludes = %v, want %v", c.name, got, c.want)
		}
	}
}

// TestGoExtractBlankSinkAndBuildExcluded (EW-15): a bare reference assigned
// to the blank identifier is not recorded as a call, a call in the same
// position still is, and every symbol of a file no default build compiles
// is marked core.ToolKindGoBuildExcluded.
func TestGoExtractBlankSinkAndBuildExcluded(t *testing.T) {
	src := []byte(`package main

func kept() int  { return 1 }
func blank() int { return 2 }
func called() int { return 3 }

func main() {
	_ = blank
	var _ = (blank)
	_ = called()
	_ = kept
	use(kept)
}

func use(f func() int) {}
`)
	nodes, err := NewGoExtractor().Extract("svc", "cmd/app/main.go", src)
	if err != nil {
		t.Fatal(err)
	}
	var calls []string
	for _, n := range nodes {
		if n.Name == "main" {
			calls = n.Calls
		}
		if n.ToolKind != "" {
			t.Errorf("%s: ToolKind %q on a file every build compiles", n.Name, n.ToolKind)
		}
	}
	has := map[string]bool{}
	for _, c := range calls {
		has[c] = true
	}
	if has["svc::main.blank"] {
		t.Errorf("main's calls %v record a blank-sink reference as a call", calls)
	}
	if !has["svc::main.called"] || !has["svc::main.kept"] {
		t.Errorf("main's calls %v lost a real call or a real value reference", calls)
	}

	gen, err := NewGoExtractor().Extract("svc", "tools/gen/gen.go", []byte("//go:build ignore\n\npackage main\n\nfunc main() {}\n"))
	if err != nil {
		t.Fatal(err)
	}
	if len(gen) == 0 || gen[0].ToolKind != core.ToolKindGoBuildExcluded {
		t.Errorf("a //go:build ignore file's symbols = %+v, want ToolKind %q", gen, core.ToolKindGoBuildExcluded)
	}
}
