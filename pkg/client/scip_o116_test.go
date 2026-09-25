package client_test

// scip_o116_test.go — the OPEN residual of O116 (.fathom/observations.md):
// O116 itself fixed a package whose corrected re-import still names it but
// now computes to zero edges. It explicitly left open a DIFFERENT shape: a
// package a PREVIOUS `fathom ingest -scip` import wrote typed edges for,
// that is ABSENT ENTIRELY from a later import (its source was deleted or
// renamed, or the producer stopped emitting it) — never named at all, so
// the per-package store.PutTypedEdges loop in pkg/client.Client.ImportSCIP
// never sees it and its stale rows survive forever.

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"google.golang.org/protobuf/proto"

	scippb "github.com/geoffmilleraz/fathom/third_party/scip"

	"github.com/geoffmilleraz/fathom/internal/connector"
	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/extractor"
	"github.com/geoffmilleraz/fathom/internal/scip"
	"github.com/geoffmilleraz/fathom/internal/store"
	"github.com/geoffmilleraz/fathom/pkg/client"
)

// scipO116Service is the fixed service ID every test in this file uses.
const scipO116Service = "o116-vanished-svc"

// scipO116TSFixtureSources are BYTE-FOR-BYTE the same three files
// cmd/fathom/ingest_scip_test.go's writeTSFixture uses, which
// internal/scip.BuildTSFixtureIndex's occurrences are hand-built to match
// line-for-line (internal/scip/fixture.go's own doc comment) — reusing them
// here means the real TypeScript extractor mints core.SymbolNode values at
// exactly the positions BuildTSFixtureIndex's Range/EnclosingRange values
// expect, with no separate fixture to keep in sync.
var scipO116TSFixtureSources = map[string]string{
	"math.ts":    "export function add(a: number, b: number): number {\n    return a + b;\n}\n",
	"greeter.ts": "import { add } from \"./math\";\n\nexport class Greeter {\n    greet(name: string): number {\n        console.log(name);\n        return add(1, 2) + name.length;\n    }\n}\n",
	"main.ts":    "import { Greeter } from \"./greeter\";\n\nexport function run(): number {\n    const g = new Greeter();\n    return g.greet(\"hi\");\n}\n",
}

// seedSCIPO116Fixture runs the REAL TypeScript extractor over the fixture
// sources and writes the resulting symbols as Observations — the same
// SymbolNode→Observation shape a real ingest's syntax pass persists
// (core.Observation.ToSymbolNode is its documented inverse) — so
// Client.ImportSCIP's own ListAllSymbols call has the service's symbol
// table to position-match SCIP definitions against (O115).
func seedSCIPO116Fixture(t *testing.T, ctx context.Context, st store.Store) {
	t.Helper()
	e, err := extractor.NewTSExtractor("javascript")
	if err != nil {
		t.Fatalf("NewTSExtractor: %v", err)
	}
	var obs []core.Observation
	for name, src := range scipO116TSFixtureSources {
		nodes, err := e.Extract(scipO116Service, name, []byte(src))
		if err != nil {
			t.Fatalf("extract %s: %v", name, err)
		}
		for _, n := range nodes {
			obs = append(obs, core.Observation{
				ServiceID:    n.Service,
				SourceRef:    n.FilePath,
				SourceKind:   n.Kind,
				EvidenceTier: n.Tier,
				Symbol:       n.Name,
				Package:      n.Package,
				Claim:        n.Signature,
				Calls:        n.Calls,
				Complexity:   n.Complexity,
				Line:         n.Line,
				EndLine:      n.EndLine,
			})
		}
	}
	if err := st.PutObservations(ctx, obs); err != nil {
		t.Fatalf("PutObservations: %v", err)
	}
}

// scipIndexWithoutGreeter returns BuildTSFixtureIndex with greeter.ts's
// document dropped ENTIRELY — not merely stripped of its reference
// occurrences (that is O116's already-fixed "drops to zero edges" shape,
// TestImportIndex_O116_ReimportClearsPackageThatDropsToZeroEdges in
// internal/scip). Dropping the whole document means package "greeter" (its
// class Greeter and Greeter.greet) has NO mapped definition in this index
// at all, so it is absent from Import.Packages entirely — the vanished-
// package shape this file's tests target.
func scipIndexWithoutGreeter() *scippb.Index {
	idx := scip.BuildTSFixtureIndex()
	var docs []*scippb.Document
	for _, d := range idx.Documents {
		if d.RelativePath != "greeter.ts" {
			docs = append(docs, d)
		}
	}
	idx.Documents = docs
	return idx
}

func writeSCIPIndex(t *testing.T, path string, idx *scippb.Index) {
	t.Helper()
	data, err := proto.Marshal(idx)
	if err != nil {
		t.Fatalf("marshal scip index: %v", err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

// scipO116Backends is the "on BOTH store.NewMemoryStore() and
// store.NewSQLiteStore(\"\")" backend matrix every table-driven test below
// runs against — the same two-backend discipline
// internal/store/contract_test.go and internal/scip's own O116 regression
// test already hold PutTypedEdges/ListTypedEdgePackages/
// DeleteTypedEdgePackage to.
func scipO116Backends(t *testing.T) []struct {
	name string
	st   store.Store
} {
	t.Helper()
	sqliteStore, err := store.NewSQLiteStore("")
	if err != nil {
		t.Fatalf("NewSQLiteStore: %v", err)
	}
	t.Cleanup(func() { _ = sqliteStore.Close() })
	return []struct {
		name string
		st   store.Store
	}{
		{"memory", store.NewMemoryStore()},
		{"sqlite", sqliteStore},
	}
}

// TestImportSCIP_O116_VanishedPackageClearsStaleEdges is the residual's own
// regression test: import a SCIP index naming packages "greeter" (2 edges)
// and "math"/"main", then re-import the SAME index PATH with greeter.ts's
// document removed entirely — package "greeter" never named at all, not
// even with zero edges — and assert every stored edge FROM a greeter
// symbol is gone. Before the fix in pkg/client.Client.ImportSCIP, the
// second call's imp.Packages never lists "greeter" at all, so the
// per-package PutTypedEdges loop never touches it and the first import's
// rows survive forever.
func TestImportSCIP_O116_VanishedPackageClearsStaleEdges(t *testing.T) {
	for _, b := range scipO116Backends(t) {
		t.Run(b.name, func(t *testing.T) {
			ctx := context.Background()
			seedSCIPO116Fixture(t, ctx, b.st)

			conn := connector.NewFauxConnector()
			c := client.New(conn, b.st)
			t.Cleanup(func() { _ = c.Close() })

			idxPath := filepath.Join(t.TempDir(), "index.scip")
			writeSCIPIndex(t, idxPath, scip.BuildTSFixtureIndex())

			full, err := c.ImportSCIP(ctx, scipO116Service, idxPath)
			if err != nil {
				t.Fatalf("ImportSCIP(full): %v", err)
			}
			greeterGreetKey := core.SymbolKey(scipO116Service, "greeter", "Greeter.greet")
			before, err := b.st.TypedEdges(ctx, scipO116Service)
			if err != nil {
				t.Fatalf("TypedEdges: %v", err)
			}
			var beforeCount int
			for _, e := range before {
				if e.FromSymbol == greeterGreetKey {
					beforeCount++
				}
			}
			if beforeCount == 0 {
				t.Fatalf("test setup: no edges FROM %q after full import (want > 0); full.Packages=%+v", greeterGreetKey, full.Packages)
			}

			// Re-import the SAME path, greeter.ts's document gone entirely.
			writeSCIPIndex(t, idxPath, scipIndexWithoutGreeter())
			corrected, err := c.ImportSCIP(ctx, scipO116Service, idxPath)
			if err != nil {
				t.Fatalf("ImportSCIP(corrected): %v", err)
			}
			for _, pkg := range corrected.Packages {
				if pkg.Package == "greeter" {
					t.Fatalf(`test setup: "greeter" still present in corrected Import.Packages — want entirely absent (this test targets the vanished-package shape, not O116's own zero-edges-but-listed shape)`)
				}
			}

			after, err := b.st.TypedEdges(ctx, scipO116Service)
			if err != nil {
				t.Fatalf("TypedEdges after corrected reimport: %v", err)
			}
			for _, e := range after {
				if e.FromSymbol == greeterGreetKey {
					t.Errorf("stale edge from %q survived a reimport whose SCIP index no longer names package %q at all (O116 residual): %+v", greeterGreetKey, "greeter", e)
				}
			}
		})
	}
}

// TestImportSCIP_O116_GoTypedPassPackageSurvivesSCIPReimport proves the
// vanished-package reconcile is scoped to the SCIP index's OWN packages —
// never to a package the Go typed pass (internal/fusion.EnrichTypedEdges)
// wrote for the same service under a bare (unprefixed) pkg_path. Without
// this scoping, "clear every stored package this SCIP import does not
// name" would also delete Go's typed edges the instant a service mixed a
// Go package with SCIP-imported ones, since a Go pkg_path is never named
// by ANY SCIP import either.
func TestImportSCIP_O116_GoTypedPassPackageSurvivesSCIPReimport(t *testing.T) {
	for _, b := range scipO116Backends(t) {
		t.Run(b.name, func(t *testing.T) {
			ctx := context.Background()
			seedSCIPO116Fixture(t, ctx, b.st)

			// Simulate the Go typed pass having already written edges for a
			// Go package of the SAME service, under its real (bare, never
			// "scip:"-prefixed) pkg_path convention
			// (internal/fusion/typedcalls.go's own st.PutTypedEdges(ctx,
			// serviceID, pkg.ID, edges) call).
			goPkgPath := "github.com/example/o116vanished/gopkg"
			goEdges := []core.Edge{{
				FromService: scipO116Service,
				FromFile:    "gopkg/foo.go",
				FromSymbol:  core.SymbolKey(scipO116Service, "gopkg", "Foo"),
				ToSymbol:    core.SymbolKey(scipO116Service, "gopkg", "Bar"),
				Kind:        core.EdgeDirect,
			}}
			if err := b.st.PutTypedEdges(ctx, scipO116Service, goPkgPath, goEdges); err != nil {
				t.Fatalf("PutTypedEdges(go pkg): %v", err)
			}

			conn := connector.NewFauxConnector()
			c := client.New(conn, b.st)
			t.Cleanup(func() { _ = c.Close() })

			idxPath := filepath.Join(t.TempDir(), "index.scip")
			// A SCIP index that never mentions goPkgPath at all — the exact
			// condition that must NOT cause the Go package to be reconciled
			// away.
			writeSCIPIndex(t, idxPath, scipIndexWithoutGreeter())
			if _, err := c.ImportSCIP(ctx, scipO116Service, idxPath); err != nil {
				t.Fatalf("ImportSCIP: %v", err)
			}

			after, err := b.st.TypedEdges(ctx, scipO116Service)
			if err != nil {
				t.Fatalf("TypedEdges: %v", err)
			}
			var found bool
			for _, e := range after {
				if e.FromSymbol == goEdges[0].FromSymbol && e.ToSymbol == goEdges[0].ToSymbol {
					found = true
				}
			}
			if !found {
				t.Errorf("Go typed-pass edge for package %q was deleted by a SCIP reimport that never named it", goPkgPath)
			}
			pkgs, err := b.st.ListTypedEdgePackages(ctx, scipO116Service)
			if err != nil {
				t.Fatalf("ListTypedEdgePackages: %v", err)
			}
			var stillListed bool
			for _, p := range pkgs {
				if p == goPkgPath {
					stillListed = true
				}
			}
			if !stillListed {
				t.Errorf("package %q no longer listed by ListTypedEdgePackages after a SCIP reimport that never named it", goPkgPath)
			}
		})
	}
}

// TestImportSCIP_O116_SameIndexFromAnotherPathReconciles: the namespace is
// the index's stable identity, not its path as given, so re-importing the
// corrected index from a DIFFERENT temp directory (a fresh --scip location
// each run) still clears the vanished package instead of opening a second
// namespace beside the first.
func TestImportSCIP_O116_SameIndexFromAnotherPathReconciles(t *testing.T) {
	for _, b := range scipO116Backends(t) {
		t.Run(b.name, func(t *testing.T) {
			ctx := context.Background()
			seedSCIPO116Fixture(t, ctx, b.st)
			c := client.New(connector.NewFauxConnector(), b.st)
			t.Cleanup(func() { _ = c.Close() })

			first := filepath.Join(t.TempDir(), "index.scip")
			writeSCIPIndex(t, first, scip.BuildTSFixtureIndex())
			if _, err := c.ImportSCIP(ctx, scipO116Service, first); err != nil {
				t.Fatalf("ImportSCIP(first): %v", err)
			}
			second := filepath.Join(t.TempDir(), "index.scip")
			writeSCIPIndex(t, second, scipIndexWithoutGreeter())
			if _, err := c.ImportSCIP(ctx, scipO116Service, second); err != nil {
				t.Fatalf("ImportSCIP(second): %v", err)
			}
			greeterGreetKey := core.SymbolKey(scipO116Service, "greeter", "Greeter.greet")
			after, err := b.st.TypedEdges(ctx, scipO116Service)
			if err != nil {
				t.Fatalf("TypedEdges: %v", err)
			}
			for _, e := range after {
				if e.FromSymbol == greeterGreetKey {
					t.Errorf("edge from %q survived a re-import of the same index from another path: %+v", greeterGreetKey, e)
				}
			}
		})
	}
}
