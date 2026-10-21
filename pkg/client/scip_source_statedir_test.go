package client_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/projectdir"
	"github.com/geoffmilleraz/fathom/internal/statedir"
	"github.com/geoffmilleraz/fathom/internal/store"
	"github.com/geoffmilleraz/fathom/pkg/client"
)

func copySCIPSourcePair(t *testing.T, from, to string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(to), 0700); err != nil {
		t.Fatal(err)
	}
	for _, suffix := range []string{"", ".receipt.json"} {
		raw, err := os.ReadFile(from + suffix)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(to+suffix, raw, 0600); err != nil {
			t.Fatal(err)
		}
	}
}

func hasSCIPSourceNamespace(t *testing.T, st store.Store, prefix string) bool {
	t.Helper()
	packages, err := st.ListTypedEdgePackages(context.Background(), scipO116Service)
	if err != nil {
		t.Fatal(err)
	}
	for _, pkg := range packages {
		if strings.HasPrefix(pkg, prefix) {
			return true
		}
	}
	return false
}

func TestSCIPSourceStateDirectorySelectionAndRefusal(t *testing.T) {
	t.Setenv(statedir.EnvOverride, filepath.Join(t.TempDir(), "state"))
	c, st, root, original := scipSourceFixture(t)
	legacy := filepath.Join(root, ".fathom", "scip", "native.scip")
	fresh := statedir.LocalPath(root, projectdir.SCIP, "native.scip")
	copySCIPSourcePair(t, original, legacy)
	if err := c.IngestService(context.Background(), scipO116Service); err != nil {
		t.Fatal(err)
	}
	legacyPrefix := "scip:.fathom/scip/native.scip::"
	statePrefix := "scip:native.scip::"
	if !hasSCIPSourceNamespace(t, st, legacyPrefix) {
		t.Fatal("unshadowed legacy index was not imported")
	}
	copySCIPSourcePair(t, original, fresh)
	// The remembered legacy bytes are now invalid. They must not be read as
	// an active source once upstream state-directory precedence shadows them.
	if err := os.WriteFile(legacy, []byte("superseded legacy bytes"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := c.IngestService(context.Background(), scipO116Service); err != nil {
		t.Fatalf("valid selected state index refused: %v", err)
	}
	if hasSCIPSourceNamespace(t, st, legacyPrefix) || !hasSCIPSourceNamespace(t, st, statePrefix) {
		t.Fatal("shadowed legacy rows survived or selected state index was not imported")
	}
	// A valid legacy copy cannot rescue a corrupt selected state index.
	copySCIPSourcePair(t, original, legacy)
	if err := os.WriteFile(fresh, []byte("failed preferred attempt"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := c.IngestService(context.Background(), scipO116Service); !errors.Is(err, client.ErrSCIPSourceReceipt) {
		t.Fatalf("preferred failure fell back instead of refusing: %v", err)
	}
	if hasSCIPSourceNamespace(t, st, legacyPrefix) || hasSCIPSourceNamespace(t, st, statePrefix) {
		t.Fatal("preferred refusal retained selected or shadowed typed rows")
	}
	if !hasSCIPSourceNamespace(t, st, "scip:index.scip::") {
		t.Fatal("state/legacy refusal removed the unrelated root index")
	}
}
