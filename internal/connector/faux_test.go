package connector_test

// faux_test.go — tests for FauxConnector.
// FauxConnector is self-rooted at testdata/fauxtown/ so tests need only
// supply logical service IDs (matching the subdirectory names).

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/connector"
	"github.com/geoffmilleraz/fathom/internal/core"
)

func TestFauxConnector_Discover_PaymentService(t *testing.T) {
	fc := connector.NewFauxConnector()
	ctx := context.Background()

	svc := core.ServiceRef{ID: "payment-service", Path: "payment-service"}
	refs, err := fc.Discover(ctx, svc)
	if err != nil {
		t.Fatalf("Discover: %v", err)
	}
	if len(refs) == 0 {
		t.Fatal("expected at least one SourceRef, got none")
	}

	// Service field must be propagated.
	for _, r := range refs {
		if r.Service != "payment-service" {
			t.Errorf("SourceRef.Service = %q, want %q", r.Service, "payment-service")
		}
	}
}

func TestFauxConnector_Discover_LedgerService(t *testing.T) {
	fc := connector.NewFauxConnector()
	ctx := context.Background()

	svc := core.ServiceRef{ID: "ledger-service", Path: "ledger-service"}
	refs, err := fc.Discover(ctx, svc)
	if err != nil {
		t.Fatalf("Discover: %v", err)
	}
	if len(refs) == 0 {
		t.Fatal("expected at least one SourceRef, got none")
	}
}

func TestFauxConnector_Discover_Sorted(t *testing.T) {
	fc := connector.NewFauxConnector()
	ctx := context.Background()

	svc := core.ServiceRef{ID: "ledger-service", Path: "ledger-service"}
	refs, err := fc.Discover(ctx, svc)
	if err != nil {
		t.Fatalf("Discover: %v", err)
	}

	for i := 1; i < len(refs); i++ {
		if refs[i].Path < refs[i-1].Path {
			t.Errorf("refs not sorted: [%d]=%q < [%d]=%q", i, refs[i].Path, i-1, refs[i-1].Path)
		}
	}
}

func TestFauxConnector_Discover_Deterministic(t *testing.T) {
	fc := connector.NewFauxConnector()
	ctx := context.Background()

	svc := core.ServiceRef{ID: "payment-service", Path: "payment-service"}

	first, err := fc.Discover(ctx, svc)
	if err != nil {
		t.Fatalf("first Discover: %v", err)
	}
	second, err := fc.Discover(ctx, svc)
	if err != nil {
		t.Fatalf("second Discover: %v", err)
	}

	if len(first) != len(second) {
		t.Fatalf("non-deterministic result length: %d vs %d", len(first), len(second))
	}
	for i := range first {
		if first[i] != second[i] {
			t.Errorf("non-deterministic at [%d]: %+v vs %+v", i, first[i], second[i])
		}
	}
}

func TestFauxConnector_Fetch(t *testing.T) {
	fc := connector.NewFauxConnector()
	ctx := context.Background()

	svc := core.ServiceRef{ID: "payment-service", Path: "payment-service"}
	refs, err := fc.Discover(ctx, svc)
	if err != nil {
		t.Fatalf("Discover: %v", err)
	}

	for _, r := range refs {
		data, err := fc.Fetch(ctx, r)
		if err != nil {
			t.Errorf("Fetch(%q): %v", r.Path, err)
			continue
		}
		if len(data) == 0 {
			t.Errorf("Fetch(%q): empty bytes", r.Path)
		}
	}
}

func TestFauxConnector_Fetch_KnownContent(t *testing.T) {
	fc := connector.NewFauxConnector()
	ctx := context.Background()

	// Build a SourceRef for payment.go directly (path is relative to fauxtown root).
	ref := core.SourceRef{
		Service: "payment-service",
		Path:    filepath.Join("payment-service", "payment.go"),
		Kind:    core.KindGoFunc,
	}
	data, err := fc.Fetch(ctx, ref)
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	// The file starts with the package declaration.
	const wantPrefix = "// Package payment"
	if len(data) < len(wantPrefix) || string(data[:len(wantPrefix)]) != wantPrefix {
		t.Errorf("unexpected file prefix: got %q", string(data[:min(len(data), 40)]))
	}
}

func TestFauxConnector_KindGoFunc_ForGoFiles(t *testing.T) {
	fc := connector.NewFauxConnector()
	ctx := context.Background()

	svc := core.ServiceRef{ID: "ledger-service", Path: "ledger-service"}
	refs, err := fc.Discover(ctx, svc)
	if err != nil {
		t.Fatalf("Discover: %v", err)
	}

	for _, r := range refs {
		if filepath.Ext(r.Path) == ".go" {
			if r.Kind != core.KindGoFunc {
				t.Errorf("%q: Kind = %q, want KindGoFunc", r.Path, r.Kind)
			}
		}
	}
}

// TestFauxConnector_Discover_SkipsClaudeWorktrees verifies the FauxConnector
// walker shares the skipDir defaults: the worktree-service fixture carries a
// stale copy under .claude/worktrees/agent-0001/ that must never be
// discovered (agent worktrees are tool state, same class as .idea/.vscode).
func TestFauxConnector_Discover_SkipsClaudeWorktrees(t *testing.T) {
	fc := connector.NewFauxConnector()
	ctx := context.Background()

	svc := core.ServiceRef{ID: "worktree-service", Path: "worktree-service"}
	refs, err := fc.Discover(ctx, svc)
	if err != nil {
		t.Fatalf("Discover: %v", err)
	}

	for _, r := range refs {
		if strings.Contains(filepath.ToSlash(r.Path), "/.claude/") {
			t.Errorf("discovered ref under .claude/: %q", r.Path)
		}
	}
	if len(refs) != 1 {
		t.Fatalf("Discover returned %d refs, want 1 (main.go only): %+v", len(refs), refs)
	}
	if filepath.Base(refs[0].Path) != "main.go" {
		t.Errorf("discovered %q, want main.go", refs[0].Path)
	}
}

func TestFauxConnector_SatisfiesInterface(t *testing.T) {
	fc := connector.NewFauxConnector()
	svc := core.ServiceRef{ID: "payment-service", Path: "payment-service"}
	wantPaths := []string{
		filepath.Join("payment-service", "errors.go"),
		filepath.Join("payment-service", "payment.go"),
	}
	connectorContract(t, fc, svc, wantPaths)
}

// min is a helper for Go <1.21 compat; on Go 1.26 the builtin exists but
// spelling it out keeps the file self-contained.
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
