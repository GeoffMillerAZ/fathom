package certify

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/store"
)

const testResolveService = "svc"

// seedResolveFixture builds a service whose corpus root (on disk, under
// t.TempDir()) carries a minimal docs/intent/ router + invariants.md +
// incidents.md, and whose store carries: an obligations-layer claim (for
// kind "rule"), a spec-referencing symbol (for kind "spec"), and an
// interface symbol (for kind "port").
func seedResolveFixture(t *testing.T) (context.Context, store.Store) {
	t.Helper()
	ctx := context.Background()
	st := store.NewMemoryStore()
	t.Cleanup(func() { _ = st.Close() })

	root := t.TempDir()
	mustWrite(t, filepath.Join(root, "docs", "intent", "README.md"), "# Router\n\ndocs/intent/invariants.md is the living doc.\n")
	mustWrite(t, filepath.Join(root, "docs", "intent", "invariants.md"), "# Invariants\n\n| # | Statement | Enforced by |\n|---|---|---|\n| I1 | Widgets never negative | test (`TestWidgetCount`) |\n")
	mustWrite(t, filepath.Join(root, "docs", "intent", "incidents.md"), "# Incidents\n\n## Open\n\n### INC-1\n\nSomething happened.\n\nOutcome: pending\n")
	// billing/routes.go: a real on-disk file registered in the store below
	// (CV-22, resolveBehaviour) — its route:GET /v1/checkout registration
	// is the inventory item "behaviour is declared-only"/
	// TestResolveTarget_BehaviourInventoryKey resolve against.
	mustWrite(t, filepath.Join(root, "billing", "routes.go"), "package billing\n\nimport \"net/http\"\n\nfunc register(mux *http.ServeMux) {\n\tmux.HandleFunc(\"GET /v1/checkout\", handleCheckout)\n}\n\nfunc handleCheckout(w http.ResponseWriter, r *http.Request) {}\n")

	if err := st.SetServiceMeta(ctx, store.ServiceMeta{ServiceID: testResolveService, Path: root, IndexedCommit: "deadbeef"}); err != nil {
		t.Fatalf("SetServiceMeta: %v", err)
	}

	if err := st.PutObservations(ctx, []core.Observation{
		{
			ServiceID: testResolveService, SourceRef: "billing/spec.go", SourceKind: core.KindGoFunc,
			EvidenceTier: core.TierVerified, Symbol: "Refund", Package: "billing",
			Claim: "func Refund() // [US-12]", IngestedAt: time.Now(),
		},
		{
			ServiceID: testResolveService, SourceRef: "billing/ports.go", SourceKind: core.KindGoInterface,
			EvidenceTier: core.TierVerified, Symbol: "Ledger", Package: "billing",
			Claim: "type Ledger interface", IngestedAt: time.Now(),
		},
		{
			ServiceID: testResolveService, SourceRef: "billing/routes.go", SourceKind: core.KindGoFunc,
			EvidenceTier: core.TierVerified, Symbol: "register", Package: "billing",
			Claim: "func register(mux *http.ServeMux)", IngestedAt: time.Now(),
		},
	}); err != nil {
		t.Fatalf("seed PutObservations: %v", err)
	}

	claim := core.SemanticClaim{
		ClaimID: "rule-claim-1", ServiceID: testResolveService, Layer: "obligations", Kind: "obligation",
		Subject: "CLAUDE.md#deploy", Rule: "deploy = task deploy", Scope: []string{"cmd/fathom/deploy.go"},
		Tier: core.TierDocClaimed, Status: core.ClaimFresh,
	}
	if err := st.PutClaims(ctx, []core.SemanticClaim{claim}, nil); err != nil {
		t.Fatalf("seed PutClaims: %v", err)
	}

	return ctx, st
}

func mustWrite(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func TestResolveTarget_EveryKindAndUnknown(t *testing.T) {
	ctx, st := seedResolveFixture(t)

	t.Run("behaviour resolves to a real inventory key, Symbols stays empty", func(t *testing.T) {
		got, err := ResolveTarget(ctx, st, testResolveService, "behaviour", "route:GET /v1/checkout")
		if err != nil {
			t.Fatalf("ResolveTarget: %v", err)
		}
		if !got.Resolved || len(got.Symbols) != 0 {
			t.Fatalf("behaviour route:GET /v1/checkout = %+v, want Resolved=true, Symbols empty", got)
		}
	})

	t.Run("behaviour unknown key does not resolve", func(t *testing.T) {
		got, err := ResolveTarget(ctx, st, testResolveService, "behaviour", "checkout-happy-path")
		if err != nil {
			t.Fatalf("ResolveTarget: %v", err)
		}
		if got.Resolved || got.Reason != "target does not resolve" {
			t.Fatalf("behaviour checkout-happy-path = %+v, want unresolved with the standard reason", got)
		}
	})

	t.Run("invariant known", func(t *testing.T) {
		got, err := ResolveTarget(ctx, st, testResolveService, "invariant", "I1")
		if err != nil {
			t.Fatalf("ResolveTarget: %v", err)
		}
		if !got.Resolved {
			t.Fatalf("invariant I1 = %+v, want Resolved=true", got)
		}
		if len(got.Files) != 1 || got.Files[0] != "docs/intent/invariants.md" {
			t.Errorf("invariant I1 Files = %v, want [docs/intent/invariants.md]", got.Files)
		}
	})

	t.Run("invariant unknown", func(t *testing.T) {
		got, err := ResolveTarget(ctx, st, testResolveService, "invariant", "I999")
		if err != nil {
			t.Fatalf("ResolveTarget: %v", err)
		}
		if got.Resolved || got.Reason != "target does not resolve" {
			t.Fatalf("invariant I999 = %+v, want unresolved with the standard reason", got)
		}
	})

	t.Run("incident known", func(t *testing.T) {
		got, err := ResolveTarget(ctx, st, testResolveService, "incident", "INC-1")
		if err != nil {
			t.Fatalf("ResolveTarget: %v", err)
		}
		if !got.Resolved {
			t.Fatalf("incident INC-1 = %+v, want Resolved=true", got)
		}
	})

	t.Run("incident unknown", func(t *testing.T) {
		got, err := ResolveTarget(ctx, st, testResolveService, "incident", "INC-999")
		if err != nil {
			t.Fatalf("ResolveTarget: %v", err)
		}
		if got.Resolved || got.Reason != "target does not resolve" {
			t.Fatalf("incident INC-999 = %+v, want unresolved with the standard reason", got)
		}
	})

	t.Run("spec known", func(t *testing.T) {
		got, err := ResolveTarget(ctx, st, testResolveService, "spec", "US-12")
		if err != nil {
			t.Fatalf("ResolveTarget: %v", err)
		}
		if !got.Resolved || len(got.Symbols) != 1 {
			t.Fatalf("spec US-12 = %+v, want Resolved=true with one symbol", got)
		}
	})

	t.Run("spec unknown", func(t *testing.T) {
		got, err := ResolveTarget(ctx, st, testResolveService, "spec", "US-999")
		if err != nil {
			t.Fatalf("ResolveTarget: %v", err)
		}
		if got.Resolved {
			t.Fatalf("spec US-999 = %+v, want unresolved", got)
		}
	})

	t.Run("rule known", func(t *testing.T) {
		got, err := ResolveTarget(ctx, st, testResolveService, "rule", "CLAUDE.md#deploy")
		if err != nil {
			t.Fatalf("ResolveTarget: %v", err)
		}
		if !got.Resolved {
			t.Fatalf("rule CLAUDE.md#deploy = %+v, want Resolved=true", got)
		}
	})

	t.Run("rule unknown", func(t *testing.T) {
		got, err := ResolveTarget(ctx, st, testResolveService, "rule", "CLAUDE.md#nope")
		if err != nil {
			t.Fatalf("ResolveTarget: %v", err)
		}
		if got.Resolved {
			t.Fatalf("rule CLAUDE.md#nope = %+v, want unresolved", got)
		}
	})

	t.Run("port known", func(t *testing.T) {
		got, err := ResolveTarget(ctx, st, testResolveService, "port", "billing/ports.Ledger")
		if err != nil {
			t.Fatalf("ResolveTarget: %v", err)
		}
		if !got.Resolved || len(got.Symbols) == 0 {
			t.Fatalf("port billing/ports.Ledger = %+v, want Resolved=true with at least the interface symbol", got)
		}
	})

	t.Run("port unknown", func(t *testing.T) {
		got, err := ResolveTarget(ctx, st, testResolveService, "port", "billing/ports.Nope")
		if err != nil {
			t.Fatalf("ResolveTarget: %v", err)
		}
		if got.Resolved {
			t.Fatalf("port billing/ports.Nope = %+v, want unresolved", got)
		}
	})

	t.Run("unknown kind", func(t *testing.T) {
		got, err := ResolveTarget(ctx, st, testResolveService, "bogus", "whatever")
		if err != nil {
			t.Fatalf("ResolveTarget: %v", err)
		}
		if got.Resolved || got.Reason != "target does not resolve" {
			t.Fatalf("unknown kind = %+v, want unresolved with the standard reason", got)
		}
	})
}

// TestResolveTarget_BehaviourInventoryKey pins CV-22 (ADR-044 §2.4)
// end to end: a `certifies: behaviour <key>` target resolves exactly when
// <key> names a real capability-inventory item at HEAD, and a changed
// registration (a route that no longer exists) flips a previously
// resolving target back to unresolved — the deterministic re-derivation a
// characterization/golden test needs to certify something checkable.
func TestResolveTarget_BehaviourInventoryKey(t *testing.T) {
	ctx, st := seedResolveFixture(t)

	got, err := ResolveTarget(ctx, st, testResolveService, "behaviour", "route:GET /v1/checkout")
	if err != nil {
		t.Fatalf("ResolveTarget: %v", err)
	}
	if !got.Resolved {
		t.Fatalf("route:GET /v1/checkout = %+v, want Resolved=true", got)
	}
	if len(got.Symbols) != 0 {
		t.Errorf("Symbols = %v, want empty (design doc §1: behaviour's tier never exceeds doc-claimed)", got.Symbols)
	}

	unknown, err := ResolveTarget(ctx, st, testResolveService, "behaviour", "route:DELETE /v1/checkout")
	if err != nil {
		t.Fatalf("ResolveTarget: %v", err)
	}
	if unknown.Resolved {
		t.Fatalf("route:DELETE /v1/checkout = %+v, want unresolved — no such registration exists", unknown)
	}
	if unknown.Reason != "target does not resolve" {
		t.Errorf("Reason = %q, want the standard %q", unknown.Reason, "target does not resolve")
	}
}
