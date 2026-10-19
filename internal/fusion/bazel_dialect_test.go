package fusion_test

// bazel_dialect_test.go — LB-Q-bazel (design §6, §13): load("//pkg:f.bzl", "sym") and load(":f.bzl", alias = "sym")
// bind the symbols they name to the .bzl file, through the file-symbol strategy's Bazel labels, so a target (or
// a def) that calls the loaded macro or rule resolves to the file that defines it. A load of another repository
// (@rules_cc//...) and a native rule resolve to nothing. ingest dispatches BUILD, MODULE.bazel, WORKSPACE and
// *.bzl through the tags-engine loop (the manifest claims them by basename and extension).

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/connector"
	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/fusion"
	"github.com/geoffmilleraz/fathom/internal/store"
)

func TestResolveFileEdges_BazelLoads(t *testing.T) {
	dir, err := filepath.Abs(filepath.Join("testdata", "bazelfixture"))
	if err != nil {
		t.Fatal(err)
	}
	paths := []string{"BUILD", "local.bzl", "tools/defs.bzl", "tools/helpers.bzl"}
	files, symbols := loadFileResolveFixture(t, dir, "bazel", paths)
	edges, report := fusion.ResolveFileEdges(context.Background(), "bazel", dir, files, symbols)
	if report.Skipped {
		t.Fatalf("skipped: %s", report.SkipReason)
	}
	want := []string{
		"BUILD|aliased -> tools/defs.bzl|other_macro", // renamed = "other_macro": the alias binds the exported name
		"BUILD|core -> BUILD|plain",                   // ":plain": a label in the same package, same file
		"BUILD|gen -> BUILD|core",                     // deps = [":core"]
		"BUILD|gen -> tools/defs.bzl|my_macro",        // load("//tools:defs.bzl"): a label from the repository root
		"BUILD|plain -> local.bzl|local_rule",         // load(":local.bzl"): a label in the importer's package
		"tools/defs.bzl|my_macro -> tools/helpers.bzl|join",
		"tools/defs.bzl|other_macro -> tools/helpers.bzl|join",
	}
	got := namedEdges(edges, symbols)
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("edges:\n  %s\nwant:\n  %s", strings.Join(got, "\n  "), strings.Join(want, "\n  "))
	}
}

func TestIngest_BazelDialectIsDispatched(t *testing.T) {
	dir := t.TempDir()
	for rel, src := range map[string]string{
		"BUILD.bazel":    "cc_library(name = \"lib\", srcs = [\"l.cc\"])\n",
		"pkg/BUILD":      "load(\"//tools:defs.bzl\", \"m\")\nm(name = \"app\", deps = [\"//:lib\"])\n",
		"tools/defs.bzl": "def m(name, deps = []):\n    native.cc_binary(name = name, deps = deps)\n",
		"MODULE.bazel":   "module(name = \"demo\")\nbazel_dep(name = \"rules_cc\", version = \"0.0.17\")\n",
		"WORKSPACE":      "workspace(name = \"demo\")\n",
		"notes/BUILD.md": "cc_library(name = \"ignored\")\n",
	} {
		p := filepath.Join(dir, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(src), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	for _, p := range []string{"BUILD", "a/b/BUILD.bazel", "x.bzl", "MODULE.bazel", "WORKSPACE", "WORKSPACE.bazel", "WORKSPACE.bzlmod"} {
		if got := connector.LanguageForPath(p); got != "bazel" {
			t.Errorf("LanguageForPath(%q) = %q, want bazel", p, got)
		}
	}
	st := store.NewMemoryStore()
	svc := core.ServiceRef{ID: "bazel-svc", Path: dir}
	p := fusion.New(connector.NewLocalConnector(), st, defaultCfg())
	t.Cleanup(p.Close)
	if _, err := p.Ingest(context.Background(), svc); err != nil {
		t.Fatalf("Ingest: %v", err)
	}
	obs, err := st.QueryObservations(context.Background(), svc.ID, "")
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, o := range obs {
		got = append(got, o.SourceRef+"|"+string(o.SourceKind)+"|"+o.Symbol)
	}
	slices.Sort(got)
	want := []string{
		"BUILD.bazel|BazelTarget|lib",
		"MODULE.bazel|BazelDependency|demo",
		"MODULE.bazel|BazelDependency|rules_cc",
		"WORKSPACE|BazelDependency|demo",
		"pkg/BUILD|BazelTarget|app",
		"tools/defs.bzl|BazelFunction|m",
	}
	slices.Sort(want)
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("observations:\n  %s\nwant:\n  %s", strings.Join(got, "\n  "), strings.Join(want, "\n  "))
	}
}
