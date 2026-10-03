package fusion

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

// cueCorpus is a CUE module rooted below the repository root (deploy/): a
// package importing a sibling package, a vendored one (cue.mod/pkg), a
// generated one (cue.mod/gen, which an ingest never reads), the standard
// library and a registry module (both resolving to no file), a name declared
// in a sibling file and one declared in an ancestor directory's file of the
// same package, and a file outside the module.
func cueCorpus() map[string]string {
	return map[string]string{
		"deploy/cue.mod/module.cue":                  "module: \"example.com/app@v0\"\nlanguage: version: \"v0.16.0\"\n",
		"deploy/cue.mod/pkg/example.org/lib/lib.cue": "package lib\n\n#Lib: {name: string}\n",
		"deploy/cue.mod/gen/example.org/gen/gen.cue": "// Code generated cue get go. DO NOT EDIT.\n\npackage gen\n\n#Gen: {x: int}\n",
		"deploy/common.cue":                          "package svc\n\n#Common: {owner: string}\n",
		"deploy/schema/types.cue":                    "package schema\n\n#Service: {\n\tname: string\n\tport: int | *80\n}\n",
		"deploy/svc/helper.cue":                      "package svc\n\nhelper: {x: 1}\n",
		"deploy/svc/svc.cue": `package svc

import (
	"strings"
	"example.com/app/schema"
	"example.org/lib"
	"example.org/gen"
	"registry.example/x/remote"
)

web: schema.#Service & {name: strings.ToLower("W")}
l:      lib.#Lib
g:      gen.#Gen
own:    #Common
peer:   helper.x
far:    remote.#R
`,
		"tools/other.cue": "package other\n\nx: 1\n",
	}
}

func cueDocFiles(t *testing.T, sources map[string]string) []DocFile {
	t.Helper()
	var paths []string
	for p := range sources {
		paths = append(paths, p)
	}
	sort.Strings(paths)
	x := extractor.NewCUEExtractor()
	var files []DocFile
	for _, p := range paths {
		r, err := x.ExtractDoc("svc", p, []byte(sources[p]))
		if err != nil {
			t.Fatal(err)
		}
		files = append(files, DocFile{Path: p, Result: r})
	}
	return files
}

// TestResolveDocXrefs_CUE pins LB-Q-cue's cross-file references: an import
// names the files of the package it loads (inside the module by the module
// path, else in the vendored trees), a selector through it the declaration
// there, and a name a file does not declare its declaration in a sibling or
// ancestor file of the same package; the standard library and a registry
// module resolve to nothing.
func TestResolveDocXrefs_CUE(t *testing.T) {
	got := xrefBrief(ResolveDocXrefs(cueDocFiles(t, cueCorpus())))
	want := []string{
		"deploy/svc/svc.cue:example.com/app/schema -imports-> deploy/schema/types.cue:package/schema",
		"deploy/svc/svc.cue:example.org/gen -imports-> deploy/cue.mod/gen/example.org/gen/gen.cue:package/gen",
		"deploy/svc/svc.cue:example.org/lib -imports-> deploy/cue.mod/pkg/example.org/lib/lib.cue:package/lib",
		"deploy/svc/svc.cue:g -references-> deploy/cue.mod/gen/example.org/gen/gen.cue:#Gen",
		"deploy/svc/svc.cue:l -references-> deploy/cue.mod/pkg/example.org/lib/lib.cue:#Lib",
		"deploy/svc/svc.cue:own -references-> deploy/common.cue:#Common",
		"deploy/svc/svc.cue:peer -references-> deploy/svc/helper.cue:helper",
		"deploy/svc/svc.cue:peer -references-> deploy/svc/helper.cue:helper.x",
		"deploy/svc/svc.cue:web -references-> deploy/schema/types.cue:#Service",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got\n  %s\nwant\n  %s", strings.Join(got, "\n  "), strings.Join(want, "\n  "))
	}
}

// TestResolveDocXrefs_CUEPackagePath: a name the file declares, on a path of
// the package value, also names another file's declaration of that path (CUE
// unifies them), and a name declared only in the file names nothing else.
func TestResolveDocXrefs_CUEPackagePath(t *testing.T) {
	got := xrefBrief(ResolveDocXrefs(cueDocFiles(t, map[string]string{
		"cue.mod/module.cue": "module: \"example.com/m\"\n",
		"a.cue":              "package m\n\nnet: images: {app: \"x\"}\ndigest: net.config.host\nonly: 1\nuse: only\n",
		"b.cue":              "package m\n\nnet: config: {host: \"h\"}\n",
	})))
	want := []string{
		"a.cue:digest -references-> b.cue:net",
		"a.cue:digest -references-> b.cue:net.config",
		"a.cue:digest -references-> b.cue:net.config.host",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got\n  %s\nwant\n  %s", strings.Join(got, "\n  "), strings.Join(want, "\n  "))
	}
}

// TestResolveDocXrefs_CUEQualifiedImport: `import "path:pkg"` loads the files
// of package pkg in that directory, not another package written there.
func TestResolveDocXrefs_CUEQualifiedImport(t *testing.T) {
	got := xrefBrief(ResolveDocXrefs(cueDocFiles(t, map[string]string{
		"cue.mod/module.cue": "module: \"example.com/m\"\n",
		"deploy/k8s.cue":     "package k8s\n\n#D: {spec: {}}\n",
		"deploy/tool.cue":    "package other\n\n#D: {spec: {}}\n",
		"app/app.cue":        "package app\n\nimport \"example.com/m/deploy:k8s\"\n\nx: k8s.#D.spec\n",
	})))
	want := []string{
		"app/app.cue:example.com/m/deploy:k8s -imports-> deploy/k8s.cue:package/k8s",
		"app/app.cue:x -references-> deploy/k8s.cue:#D",
		"app/app.cue:x -references-> deploy/k8s.cue:#D.spec",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got\n  %s\nwant\n  %s", strings.Join(got, "\n  "), strings.Join(want, "\n  "))
	}
}

// TestIngest_CUEModuleReachesTheCUEDialect is LB-Q-cue's product reach: an
// ingest extracts every .cue file through the CUE dialect (its package entity,
// its fields wherever written) and stores the cross-file references that
// resolve as doc-xref edges, an import to the package it loads among them; a
// generated file is never read (the dialect gate), so nothing resolves into
// it.
func TestIngest_CUEModuleReachesTheCUEDialect(t *testing.T) {
	dir := t.TempDir()
	writeCorpus(t, dir, cueCorpus())
	st := store.NewMemoryStore()
	p := New(connector.NewLocalConnector(), st, Config{})
	t.Cleanup(p.Close)
	ctx := context.Background()
	rep, err := p.Ingest(ctx, core.ServiceRef{ID: "cm", Path: dir})
	if err != nil {
		t.Fatal(err)
	}
	obs, err := st.QueryObservations(ctx, "cm", "")
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]string{}
	for _, o := range obs {
		got[o.SourceRef+"|"+o.Symbol] = string(o.SourceKind)
		if strings.Contains(o.SourceRef, "cue.mod/gen/") {
			t.Errorf("%s|%s: a generated file was extracted", o.SourceRef, o.Symbol)
		}
	}
	for key, kind := range map[string]string{
		"deploy/svc/svc.cue|package/svc":                  "CUEPackage",
		"deploy/svc/svc.cue|example.com/app/schema":       "CUEImport",
		"deploy/svc/svc.cue|web":                          "CUEField",
		"deploy/schema/types.cue|#Service.port":           "CUEField",
		"deploy/cue.mod/pkg/example.org/lib/lib.cue|#Lib": "CUEDefinition",
	} {
		if got[key] != kind {
			t.Errorf("%s = %q, want %q", key, got[key], kind)
		}
	}
	all, err := st.TypedEdges(ctx, "cm")
	if err != nil {
		t.Fatal(err)
	}
	var edges []string
	for _, e := range all {
		if e.Kind == core.EdgeFileResolved && strings.HasSuffix(e.FromFile, ".cue") {
			edges = append(edges, e.FromFile+"|"+e.FromSymbol+"->"+e.ToSymbol)
		}
	}
	sort.Strings(edges)
	wantEdges := []string{
		"deploy/svc/svc.cue|cm::deploy/svc/svc.example.com/app/schema->cm::deploy/schema/schema.package/schema",
		"deploy/svc/svc.cue|cm::deploy/svc/svc.example.org/lib->cm::deploy/cue.mod/pkg/example.org/lib/lib.package/lib",
		"deploy/svc/svc.cue|cm::deploy/svc/svc.l->cm::deploy/cue.mod/pkg/example.org/lib/lib.#Lib",
		"deploy/svc/svc.cue|cm::deploy/svc/svc.own->cm::deploy/svc.#Common",
		"deploy/svc/svc.cue|cm::deploy/svc/svc.peer->cm::deploy/svc/svc.helper",
		"deploy/svc/svc.cue|cm::deploy/svc/svc.peer->cm::deploy/svc/svc.helper.x",
		"deploy/svc/svc.cue|cm::deploy/svc/svc.web->cm::deploy/schema/schema.#Service",
	}
	if !reflect.DeepEqual(edges, wantEdges) {
		t.Fatalf("stored doc-xref edges\n got: %s\nwant: %s", strings.Join(edges, "\n      "), strings.Join(wantEdges, "\n      "))
	}
	if rep.DocXrefEdges != len(wantEdges) {
		t.Errorf("DocXrefEdges = %d, want %d", rep.DocXrefEdges, len(wantEdges))
	}
}

// TestIngest_ExtractorEpoch_CUEForcesReExtraction pins wave fx's bump: a .cue
// file fingerprinted under epoch 49 carried the old entities (no package
// entity, no field inside a conjunction's struct), so the bump to 50 must
// re-read it (the seed is salted with "49", the true prior epoch, so the test
// fails if the bump reverts).
func TestIngest_ExtractorEpoch_CUEForcesReExtraction(t *testing.T) {
	dir := t.TempDir()
	src := []byte("package p\n\nweb: #W & {port: 80}\n")
	if err := os.WriteFile(filepath.Join(dir, "p.cue"), src, 0o644); err != nil {
		t.Fatal(err)
	}
	st := store.NewMemoryStore()
	ctx := context.Background()
	svc := core.ServiceRef{ID: "epoch-cue", Path: dir}
	old := sha256.Sum256(append(append([]byte{}, src...), []byte("\x00fathom-extractor-epoch:49")...))
	if err := st.SetFingerprint(ctx, svc.ID, "p.cue", hex.EncodeToString(old[:])); err != nil {
		t.Fatal(err)
	}
	p := New(connector.NewLocalConnector(), st, Config{Concurrency: 1})
	t.Cleanup(p.Close)
	rep, err := p.Ingest(ctx, svc)
	if err != nil {
		t.Fatal(err)
	}
	if rep.Skipped != 0 {
		t.Fatalf("%d file(s) skipped: an epoch-49 fingerprint was honored, ExtractorEpoch was not bumped for .cue", rep.Skipped)
	}
	obs, err := st.QueryObservations(ctx, svc.ID, "")
	if err != nil {
		t.Fatal(err)
	}
	for _, o := range obs {
		if o.SourceRef == "p.cue" && o.Symbol == "web.port" {
			return
		}
	}
	t.Fatalf("re-extraction did not read p.cue's conjunction struct: %+v", obs)
}
