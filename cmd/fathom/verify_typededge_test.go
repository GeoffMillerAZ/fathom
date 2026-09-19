package main

import (
	"bytes"
	"path/filepath"
	"testing"
)

// TestCmdVerify_Store_DirectAmbiguousMethodTypedEdge is O40's end-to-end CLI
// closure proof. verifyAllAgainstStore (this package's verify.go) built its
// call graph with callgraph.Build(nodes) alone — never AddTypedEdges — so
// `fathom verify` fell back to the syntax-only resolver for every
// reachability check. That resolver refuses to guess whenever two receivers
// share a method name (internal/analysis/callgraph/graph.go's
// resolveAmbiguousCallees, byDotSuffix count > 1): a direct call on a
// concrete receiver (here *MemStore.PutClaims, sharing its name with
// DiskStore.PutClaims — the real repro is *SQLiteStore.EnsureIndexes vs
// MemoryStore.EnsureIndexes) was reported as unreached even though the
// typed pass had already resolved and stored the edge. This test ingests
// the real typedfixture module (go/packages extraction, not --mode faux)
// through the actual `ingest`+`verify` CLI path and requires edge_wired to
// pass for exactly that shape — it failed before verifyAllAgainstStore was
// changed to build with typed edges (callgraph.BuildBoth) and passes now.
func TestCmdVerify_Store_DirectAmbiguousMethodTypedEdge(t *testing.T) {
	fixtureDir, err := filepath.Abs(filepath.Join("..", "..", "internal", "fusion", "testdata", "typedfixture"))
	if err != nil {
		t.Fatalf("abs: %v", err)
	}

	storePath := filepath.Join(t.TempDir(), "test.sqlite")
	var ing bytes.Buffer
	code := run([]string{
		"ingest", "--service", "typedfixture",
		"--path", fixtureDir, "--store", storePath,
	}, &ing, &ing)
	if code != 0 {
		t.Fatalf("ingest failed (%d): %s", code, ing.String())
	}

	plan := "- [x] direct ambiguous method {symbol: MemStore.PutClaims, reachable_from: TestCallMemStoreDirect}\n"
	pf := writeTempPlan(t, plan)

	var out, errBuf bytes.Buffer
	code = run([]string{
		"verify", "--service", "typedfixture",
		"--plan", pf, "--store", storePath, "--strict",
	}, &out, &errBuf)
	if code != 0 {
		t.Fatalf("verify: exit %d (want 0 — TestCallMemStoreDirect must reach MemStore.PutClaims via the typed edge)\nstdout=%s\nstderr=%s",
			code, out.String(), errBuf.String())
	}
}
