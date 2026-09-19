package main

import (
	"bytes"
	"context"
	"path/filepath"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/store"
	"github.com/geoffmilleraz/fathom/pkg/client"
)

// TestCmdVerify_Store_TypedEdgeWiresReachability (INC-21): `fathom verify
// --store` builds the SAME call graph the daemon's verify_plan builds — the
// syntax edges plus the typed pass's edges — so a plan row whose reachability
// runs only through a typed edge (a method value, an interface dispatch)
// verifies identically on both surfaces. Before the fix the CLI built the
// syntax-only graph and reported "does not reach" for rows the daemon proved.
func TestCmdVerify_Store_TypedEdgeWiresReachability(t *testing.T) {
	storePath := filepath.Join(t.TempDir(), "test.sqlite")
	var ing bytes.Buffer
	if code := run([]string{
		"ingest", "--service", "payment-service",
		"--path", fauxPaymentPath(),
		"--store", storePath, "--mode", "faux",
	}, &ing, &ing); code != 0 {
		t.Fatalf("ingest failed (%d): %s", code, ing.String())
	}

	// validateCharge never calls Charge in source, so the syntax graph alone
	// cannot wire this row.
	pf := writeTempPlan(t, "- [x] Charge reachable from validateCharge {symbol: Charge, reachable_from: validateCharge, skip_test: true, skip: \"test fixture\"}\n")
	verifyArgs := []string{"verify", "--service", "payment-service", "--plan", pf, "--store", storePath, "--quiet-pass"}

	var out, errBuf bytes.Buffer
	if code := run(verifyArgs, &out, &errBuf); code != 1 {
		t.Fatalf("want exit 1 before any typed edge exists, got %d\nstdout=%s\nstderr=%s", code, out.String(), errBuf.String())
	}

	// Record the edge the typed pass would emit for Charge referenced as a
	// value inside validateCharge.
	ctx := context.Background()
	c, err := client.NewConstantClient(client.ModeLocal, storePath)
	if err != nil {
		t.Fatal(err)
	}
	nodes, err := c.ListAllSymbols(ctx, "payment-service")
	c.Close()
	if err != nil {
		t.Fatal(err)
	}
	var fromKey, toKey string
	for _, n := range nodes {
		switch n.Name {
		case "validateCharge":
			fromKey = n.Key()
		case "Charge":
			toKey = n.Key()
		}
	}
	if fromKey == "" || toKey == "" {
		t.Fatalf("fixture symbols missing: validateCharge=%q Charge=%q", fromKey, toKey)
	}
	s, err := store.NewSQLiteStore(storePath)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.PutTypedEdges(ctx, "payment-service", "payment-service", []core.Edge{{
		FromService: "payment-service", FromSymbol: fromKey, ToSymbol: toKey, Kind: core.EdgeFuncValue,
	}}); err != nil {
		t.Fatal(err)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}

	out.Reset()
	errBuf.Reset()
	if code := run(verifyArgs, &out, &errBuf); code != 0 {
		t.Fatalf("want exit 0 once the typed edge wires the row, got %d\nstdout=%s\nstderr=%s", code, out.String(), errBuf.String())
	}
}
