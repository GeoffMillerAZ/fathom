package fusion

// resolve_regopkg_test.go — the rego-package strategy (LB-F132) on its own,
// through the file resolver (a package split across files resolves its
// members wherever they are defined), and through ingest: a .rego file
// reaches the Rego dialect (LB-F131) and its imports become stored
// file-resolved edges.

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/connector"
	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/extractor"
	"github.com/geoffmilleraz/fathom/internal/store"
)

// regoCorpus is a policy importing a package split across two files, a
// package by a rule of it, and a package that does not exist.
func regoCorpus() map[string]string {
	return map[string]string{
		"lib/ast/a.rego":       "package lib.ast\n\nimport rego.v1\n\nf(x) := x\n",
		"lib/ast/b.rego":       "package lib.ast\n\nimport rego.v1\n\ng(x) := y if y := x\n",
		"lib/config.rego":      "package lib.config\n\ncapabilities := {\"a\": 1}\n",
		"lib/config_more.rego": "package lib.config\n\nrules := {\"b\": 2}\n",
		"policy/p.rego": `package policy

import data.lib.ast
import data.lib.config
import data.lib.config.capabilities as caps
import data.lib.missing
import rego.v1

allow if {
	ast.f(input.x)
	ast.g(input.y)
}

deny contains "x" if config.rules.b

limits := caps

direct := data.lib.ast.f(1)
`,
	}
}

func writeRegoCorpus(t *testing.T) (string, []core.SourceFile, []core.SymbolNode) {
	t.Helper()
	dir := t.TempDir()
	var files []core.SourceFile
	var syms []core.SymbolNode
	x := extractor.NewRegoExtractor()
	paths := make([]string, 0, len(regoCorpus()))
	for rel := range regoCorpus() {
		paths = append(paths, rel)
	}
	sort.Strings(paths)
	for _, rel := range paths {
		src := []byte(regoCorpus()[rel])
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
	return dir, files, syms
}

// TestResolveRegoPackage: an import path names the files declaring its
// package, else the files of its parent package defining a rule of its last
// segment; input, rego.v1, future and a bare data name no file.
func TestResolveRegoPackage(t *testing.T) {
	corpus := regoCorpus()
	var paths []string
	for p := range corpus {
		paths = append(paths, p)
	}
	read := func(p string) ([]byte, bool) {
		b, ok := corpus[p]
		return []byte(b), ok
	}
	env := newResolveEnv("", paths, read)
	for module, want := range map[string][]string{
		"data.lib.ast":                 {"lib/ast/a.rego", "lib/ast/b.rego"},
		"data.lib.ast.f":               {"lib/ast/a.rego"},
		"data.lib.config":              {"lib/config.rego", "lib/config_more.rego"},
		"data.lib.config.capabilities": {"lib/config.rego"},
		"data.lib.config.rules":        {"lib/config_more.rego"},
		"data.lib.missing":             nil,
		"data.lib.config.nope":         nil,
		"data":                         nil,
		"input.request":                nil,
		"rego.v1":                      nil,
		"future.keywords.in":           nil,
	} {
		if got := resolveByStrategy("rego-package", extractor.ImportParams{}, env, "policy/p.rego", module); !reflect.DeepEqual(got, want) {
			t.Errorf("%s resolved to %q, want %q", module, got, want)
		}
	}
}

// TestResolveFileEdges_RegoPackageSpansFiles: a call through an import of a
// package split across files resolves to the file defining the member, a
// reference through it too, an import of a rule binds the rule, and a call
// spelled in full through data (no import) resolves nothing.
func TestResolveFileEdges_RegoPackageSpansFiles(t *testing.T) {
	dir, files, syms := writeRegoCorpus(t)
	edges, rep := ResolveFileEdges(context.Background(), "rego", dir, files, syms)
	if rep.Skipped {
		t.Fatalf("skipped: %s", rep.SkipReason)
	}
	var got []string
	for _, e := range edges {
		got = append(got, e.FromFile+" "+e.FromSymbol[strings.LastIndex(e.FromSymbol, ".")+1:]+" -> "+e.ToSymbol)
	}
	sort.Strings(got)
	want := []string{
		"policy/p.rego allow -> svc::lib/ast/lib.ast.f",
		"policy/p.rego allow -> svc::lib/ast/lib.ast.g",
		"policy/p.rego deny -> svc::lib/lib.config.rules",
		"policy/p.rego limits -> svc::lib/lib.config.capabilities",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("file-resolved edges:\n%s\nwant:\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}

	ris, irep := ResolveImportSpecs(context.Background(), "rego", dir, files, syms)
	if irep.Skipped {
		t.Fatalf("skipped: %s", irep.SkipReason)
	}
	targets := map[string][]string{}
	for _, ri := range ris {
		targets[ri.Spec.Module+" as "+ri.Spec.LocalName] = ri.Targets
	}
	wantTargets := map[string][]string{
		"data.lib.ast as ast":                  {"lib/ast/a.rego", "lib/ast/b.rego"},
		"data.lib.config as config":            {"lib/config.rego", "lib/config_more.rego"},
		"data.lib.config.capabilities as caps": {"lib/config.rego"},
		"data.lib.missing as missing":          nil,
	}
	if !reflect.DeepEqual(targets, wantTargets) {
		t.Fatalf("import targets = %q, want %q", targets, wantTargets)
	}
}

// TestIngest_RegoReachesTheRegoDialect: discovery and dispatch read .rego
// files through the Rego dialect (no extractor was registered for them), the
// entities reach the store, the file-resolved pass stores the import edges,
// and ExtractForBench dispatches alike.
func TestIngest_RegoReachesTheRegoDialect(t *testing.T) {
	dir, _, _ := writeRegoCorpus(t)
	st := store.NewMemoryStore()
	p := New(connector.NewLocalConnector(), st, Config{Concurrency: 1})
	t.Cleanup(p.Close)
	ctx := context.Background()
	svc := core.ServiceRef{ID: "svc", Path: dir}
	if _, err := p.Ingest(ctx, svc); err != nil {
		t.Fatal(err)
	}
	obs, err := st.QueryObservations(ctx, svc.ID, "")
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]core.SourceKind{}
	for _, o := range obs {
		got[o.SourceRef+"#"+o.Symbol] = o.SourceKind
	}
	for key, kind := range map[string]core.SourceKind{
		"policy/p.rego#policy":         "RegoPackage",
		"policy/p.rego#allow":          "RegoRule",
		"policy/p.rego#deny":           "RegoRule",
		"lib/ast/a.rego#f":             "RegoFunction",
		"lib/ast/b.rego#g":             "RegoFunction",
		"lib/config.rego#capabilities": "RegoRule",
	} {
		if got[key] != kind {
			t.Errorf("%s: kind %q, want %q", key, got[key], kind)
		}
	}
	edges, err := st.TypedEdges(ctx, svc.ID)
	if err != nil {
		t.Fatal(err)
	}
	var resolved []string
	for _, e := range edges {
		if e.Kind == core.EdgeFileResolved {
			resolved = append(resolved, e.FromFile+" -> "+e.ToSymbol)
		}
	}
	sort.Strings(resolved)
	want := []string{
		"policy/p.rego -> svc::lib/ast/lib.ast.f",
		"policy/p.rego -> svc::lib/ast/lib.ast.g",
		"policy/p.rego -> svc::lib/lib.config.capabilities",
		"policy/p.rego -> svc::lib/lib.config.rules",
	}
	if !reflect.DeepEqual(resolved, want) {
		t.Fatalf("stored file-resolved edges = %q, want %q", resolved, want)
	}

	src := []byte(regoCorpus()["lib/ast/a.rego"])
	be, err := p.ExtractForBench(svc.ID, nil, "lib/ast/a.rego", src)
	if err != nil || be.Skipped != "" || len(be.Nodes) != 2 || be.Nodes[1].Kind != "RegoFunction" {
		t.Fatalf("ExtractForBench(.rego) = %+v, %v", be, err)
	}

	// ReingestFile decides as ingest does: an edited .rego file is read again
	// through the Rego dialect.
	edited := "package lib.ast\n\nimport rego.v1\n\nh(x) := x\n"
	if err := os.WriteFile(filepath.Join(dir, "lib", "ast", "a.rego"), []byte(edited), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := p.ReingestFile(ctx, st, svc.ID, dir, "lib/ast/a.rego"); err != nil {
		t.Fatal(err)
	}
	obs, err = st.QueryObservations(ctx, svc.ID, "")
	if err != nil {
		t.Fatal(err)
	}
	var sawH bool
	for _, o := range obs {
		if o.SourceRef == "lib/ast/a.rego" && o.Symbol == "h" && o.SourceKind == "RegoFunction" {
			sawH = true
		}
	}
	if !sawH {
		t.Fatal("ReingestFile did not read the edited .rego file through the Rego dialect")
	}
}

// TestIngest_ExtractorEpoch_RegoForcesReExtraction pins wave pc's bump: a .rego
// file fingerprinted under epoch 45 had no extractor, so the bump to 46 must
// re-read it through the Rego dialect (the seed is salted with "45", the true
// prior epoch: wave cx took 45 first, so the test fails if wave pc's bump reverts).
func TestIngest_ExtractorEpoch_RegoForcesReExtraction(t *testing.T) {
	dir := t.TempDir()
	src := []byte("package p\n\nallow if input.ok\n")
	if err := os.WriteFile(filepath.Join(dir, "p.rego"), src, 0o644); err != nil {
		t.Fatal(err)
	}
	st := store.NewMemoryStore()
	ctx := context.Background()
	svc := core.ServiceRef{ID: "epoch-rego", Path: dir}
	old := sha256.Sum256(append(append([]byte{}, src...), []byte("\x00fathom-extractor-epoch:45")...))
	if err := st.SetFingerprint(ctx, svc.ID, "p.rego", hex.EncodeToString(old[:])); err != nil {
		t.Fatal(err)
	}
	p := New(connector.NewLocalConnector(), st, Config{Concurrency: 1})
	t.Cleanup(p.Close)
	rep, err := p.Ingest(ctx, svc)
	if err != nil {
		t.Fatal(err)
	}
	if rep.Skipped != 0 {
		t.Fatalf("%d file(s) skipped: an epoch-45 fingerprint was honored, ExtractorEpoch was not bumped for .rego", rep.Skipped)
	}
	obs, err := st.QueryObservations(ctx, svc.ID, "")
	if err != nil {
		t.Fatal(err)
	}
	for _, o := range obs {
		if o.SourceRef == "p.rego" && o.SourceKind == "RegoRule" && o.Symbol == "allow" {
			return
		}
	}
	t.Fatalf("re-extraction did not read p.rego through the Rego dialect: %+v", obs)
}
