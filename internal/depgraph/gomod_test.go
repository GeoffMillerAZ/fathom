package depgraph_test

import (
	"reflect"
	"sort"
	"testing"

	"github.com/geoffmilleraz/sextant/internal/depgraph"
)

// ─── ParseGoMod ───────────────────────────────────────────────────────────────

func TestParseGoMod_singleLineRequire(t *testing.T) {
	content := []byte(`module github.com/example/mymod

go 1.21

require github.com/foo/bar v1.2.3
require github.com/baz/qux v0.0.1
`)
	mod, reqs, err := depgraph.ParseGoMod(content)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if mod != "github.com/example/mymod" {
		t.Fatalf("wrong module: %q", mod)
	}
	if len(reqs) != 2 {
		t.Fatalf("expected 2 requires, got %v", reqs)
	}
	// order stable
	if reqs[0].Path != "github.com/foo/bar" || reqs[0].Version != "v1.2.3" {
		t.Fatalf("wrong req[0]: %+v", reqs[0])
	}
	if reqs[1].Path != "github.com/baz/qux" || reqs[1].Version != "v0.0.1" {
		t.Fatalf("wrong req[1]: %+v", reqs[1])
	}
}

func TestParseGoMod_blockRequire(t *testing.T) {
	content := []byte(`module github.com/example/service

go 1.22

require (
	github.com/alpha/one v1.0.0
	github.com/beta/two v2.3.4
	github.com/gamma/three v0.9.9
)
`)
	mod, reqs, err := depgraph.ParseGoMod(content)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if mod != "github.com/example/service" {
		t.Fatalf("wrong module: %q", mod)
	}
	if len(reqs) != 3 {
		t.Fatalf("expected 3 requires, got %+v", reqs)
	}
	paths := make([]string, len(reqs))
	for i, r := range reqs {
		paths[i] = r.Path
	}
	sort.Strings(paths)
	want := []string{"github.com/alpha/one", "github.com/beta/two", "github.com/gamma/three"}
	if !reflect.DeepEqual(paths, want) {
		t.Fatalf("paths: got %v want %v", paths, want)
	}
}

func TestParseGoMod_mixedRequires(t *testing.T) {
	content := []byte(`module github.com/example/mixed

go 1.23

require github.com/standalone/pkg v1.0.0

require (
	github.com/block/a v1.1.0
	github.com/block/b v1.2.0
)

require github.com/another/standalone v3.0.0
`)
	_, reqs, err := depgraph.ParseGoMod(content)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(reqs) != 4 {
		t.Fatalf("expected 4 requires, got %+v", reqs)
	}
}

func TestParseGoMod_withIndirectComment(t *testing.T) {
	content := []byte(`module github.com/example/indirect

go 1.21

require (
	github.com/direct/dep v1.0.0
	github.com/indirect/dep v0.1.0 // indirect
)
`)
	_, reqs, err := depgraph.ParseGoMod(content)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(reqs) != 2 {
		t.Fatalf("expected 2 requires (including indirect), got %+v", reqs)
	}
}

func TestParseGoMod_noRequires(t *testing.T) {
	content := []byte(`module github.com/example/empty

go 1.21
`)
	mod, reqs, err := depgraph.ParseGoMod(content)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if mod != "github.com/example/empty" {
		t.Fatalf("wrong module: %q", mod)
	}
	if len(reqs) != 0 {
		t.Fatalf("expected 0 requires, got %v", reqs)
	}
}

func TestParseGoMod_missingModule_returnsError(t *testing.T) {
	content := []byte(`go 1.21

require github.com/foo/bar v1.0.0
`)
	_, _, err := depgraph.ParseGoMod(content)
	if err == nil {
		t.Fatal("expected error for missing module declaration")
	}
}

func TestParseGoMod_versions(t *testing.T) {
	content := []byte(`module github.com/example/versions

go 1.21

require (
	github.com/a/b v0.0.0-20230101000000-abcdef123456
	github.com/c/d v2.0.0+incompatible
	github.com/e/f v1.2.3-beta.1
)
`)
	_, reqs, err := depgraph.ParseGoMod(content)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(reqs) != 3 {
		t.Fatalf("expected 3, got %+v", reqs)
	}
	if reqs[0].Version != "v0.0.0-20230101000000-abcdef123456" {
		t.Fatalf("wrong version: %q", reqs[0].Version)
	}
}

// ─── BuildModuleGraph ─────────────────────────────────────────────────────────

func TestBuildModuleGraph_basic(t *testing.T) {
	mods := map[string][]byte{
		"github.com/example/a": []byte(`module github.com/example/a
go 1.21
require github.com/example/b v1.0.0
`),
		"github.com/example/b": []byte(`module github.com/example/b
go 1.21
require github.com/example/c v1.0.0
`),
		"github.com/example/c": []byte(`module github.com/example/c
go 1.21
`),
	}
	g, err := depgraph.BuildModuleGraph(mods)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	deps := sortedStrings(g.Deps("github.com/example/a"))
	if !reflect.DeepEqual(deps, []string{"github.com/example/b"}) {
		t.Fatalf("got %v", deps)
	}
	// external dep (not in mods map) still appears as a node
	bDeps := sortedStrings(g.Deps("github.com/example/b"))
	if !reflect.DeepEqual(bDeps, []string{"github.com/example/c"}) {
		t.Fatalf("got %v", bDeps)
	}
}

func TestBuildModuleGraph_externalDepAsNode(t *testing.T) {
	mods := map[string][]byte{
		"github.com/example/mymod": []byte(`module github.com/example/mymod
go 1.21
require github.com/external/lib v2.0.0
`),
	}
	g, err := depgraph.BuildModuleGraph(mods)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// external lib should be a node
	nodes := sortedStrings(g.Nodes())
	found := false
	for _, n := range nodes {
		if n == "github.com/external/lib" {
			found = true
		}
	}
	if !found {
		t.Fatalf("external dep not registered as node; nodes=%v", nodes)
	}
}

func TestBuildModuleGraph_cycle(t *testing.T) {
	mods := map[string][]byte{
		"github.com/example/a": []byte(`module github.com/example/a
go 1.21
require github.com/example/b v1.0.0
`),
		"github.com/example/b": []byte(`module github.com/example/b
go 1.21
require github.com/example/a v1.0.0
`),
	}
	g, err := depgraph.BuildModuleGraph(mods)
	if err != nil {
		t.Fatalf("unexpected error from BuildModuleGraph (cycle detection is caller's job): %v", err)
	}
	if !g.HasCycle() {
		t.Fatal("expected cycle between a and b")
	}
}

func TestBuildModuleGraph_invalidGoMod_returnsError(t *testing.T) {
	mods := map[string][]byte{
		"bad": []byte(`go 1.21
require github.com/foo/bar v1.0.0
`), // no module declaration
	}
	_, err := depgraph.BuildModuleGraph(mods)
	if err == nil {
		t.Fatal("expected error for invalid go.mod")
	}
}
