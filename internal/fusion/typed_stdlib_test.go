package fusion_test

// An adopter's report (2026-10-22): a Go 1.27 tree failed 12 of 20 typed loads with
// "math/rand/v2/rand.go: method must have no type parameters" because the
// released binary was built with Go 1.26, whose go/types rejects the generic
// methods Go 1.27's standard library uses. The typed pass must load the
// standard library of the toolchain that runs it without type errors.

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/connector"
	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/fusion"
	"github.com/geoffmilleraz/fathom/internal/store"
)

func ingestTypedFixture(t *testing.T, name string) (fusion.Report, []core.Edge) {
	t.Helper()
	dir, err := filepath.Abs(filepath.Join("testdata", name))
	if err != nil {
		t.Fatalf("abs: %v", err)
	}
	return ingestTypedDir(t, name, dir)
}

func ingestTypedDir(t *testing.T, name, dir string) (fusion.Report, []core.Edge) {
	t.Helper()
	ctx := context.Background()
	st := store.NewMemoryStore()
	pipe := fusion.New(connector.NewLocalConnector(), st, fusion.Config{})
	rep, err := pipe.Ingest(ctx, core.ServiceRef{ID: name, Path: dir})
	if err != nil {
		t.Fatalf("Ingest: %v", err)
	}
	typed, err := st.TypedEdges(ctx, name)
	if err != nil {
		t.Fatalf("TypedEdges: %v", err)
	}
	return rep, typed
}

// Whatever toolchain runs this, the standard library it ships must type-check
// in the typed pass. It cannot catch a binary built older than the go on PATH
// (go test puts its own go first); TestToolchainMatchesGoMod covers that.
func TestTypedPassLoadsToolchainStdlib(t *testing.T) {
	rep, typed := ingestTypedFixture(t, "stdlibrand")
	if rep.TypedPackagesFailed != 0 || rep.TypedFailed {
		t.Fatalf("typed pass failed on the toolchain's own standard library: %d package(s), %s", rep.TypedPackagesFailed, rep.TypedFailReason)
	}
	if len(typed) == 0 {
		t.Error("no typed edges: the fixture's roll -> rand.IntN call was not resolved")
	}
}
