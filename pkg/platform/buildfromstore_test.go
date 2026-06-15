package platform_test

import (
	"context"
	"reflect"
	"testing"

	"github.com/geoffmilleraz/sextant/internal/core"
	"github.com/geoffmilleraz/sextant/internal/store"
	"github.com/geoffmilleraz/sextant/pkg/platform"
)

// TestBuildFromStore_MultiLanguage proves the store-backed, language-agnostic
// path end-to-end: observations carrying Package + Calls (as the fusion
// pipeline writes them for ANY language) are read back from the store and
// joined into cross-repo edges — no filesystem walk, no Go parser.
func TestBuildFromStore_MultiLanguage(t *testing.T) {
	ctx := context.Background()
	st := store.NewMemoryStore()

	// A Python "lib" service defines helpers.validate; a Python "api" service
	// calls it. Plus a Ruby pair to underline language-agnosticism.
	put(t, st, []core.Observation{
		{ServiceID: "py-lib", SourceRef: "helpers.py", Symbol: "validate", Package: "helpers", EvidenceTier: core.TierVerified},
		{ServiceID: "py-api", SourceRef: "app.py", Symbol: "handler", Package: "app", Calls: []string{"helpers.validate"}, EvidenceTier: core.TierVerified},
		{ServiceID: "rb-core", SourceRef: "money.rb", Symbol: "convert", Package: "Money", EvidenceTier: core.TierVerified},
		{ServiceID: "rb-web", SourceRef: "checkout.rb", Symbol: "pay", Package: "Checkout", Calls: []string{"Money.convert"}, EvidenceTier: core.TierVerified},
	})

	g, err := platform.BuildFromStore(ctx, st, nil) // nil → every service in the store
	if err != nil {
		t.Fatalf("BuildFromStore: %v", err)
	}

	callers := platform.PlatformCallers(g, "validate")
	if len(callers) != 1 || callers[0].FromService != "py-api" || callers[0].ToService != "py-lib" {
		t.Fatalf("PlatformCallers(validate) = %#v, want one py-api→py-lib edge", callers)
	}
	if br := platform.CrossRepoBlastRadius(g, "py-lib"); !reflect.DeepEqual(br, []string{"py-api"}) {
		t.Fatalf("CrossRepoBlastRadius(py-lib) = %v, want [py-api]", br)
	}
	if br := platform.CrossRepoBlastRadius(g, "rb-core"); !reflect.DeepEqual(br, []string{"rb-web"}) {
		t.Fatalf("CrossRepoBlastRadius(rb-core) = %v, want [rb-web]", br)
	}
}

// TestBuildFromStore_ExplicitServices restricts the graph to a subset of
// services and rejects a nil store.
func TestBuildFromStore_ExplicitServices(t *testing.T) {
	ctx := context.Background()
	st := store.NewMemoryStore()
	put(t, st, []core.Observation{
		{ServiceID: "a", SourceRef: "a.py", Symbol: "f", Package: "pa", EvidenceTier: core.TierVerified},
		{ServiceID: "b", SourceRef: "b.py", Symbol: "g", Package: "pb", Calls: []string{"pa.f"}, EvidenceTier: core.TierVerified},
	})

	g, err := platform.BuildFromStore(ctx, st, []string{"a", "b"})
	if err != nil {
		t.Fatalf("BuildFromStore: %v", err)
	}
	if got := len(platform.PlatformCallers(g, "f")); got != 1 {
		t.Fatalf("expected 1 caller of f, got %d", got)
	}

	if _, err := platform.BuildFromStore(ctx, nil, nil); err == nil {
		t.Fatal("expected error for nil store")
	}
}

func put(t *testing.T, st store.Store, obs []core.Observation) {
	t.Helper()
	if err := st.PutObservations(context.Background(), obs); err != nil {
		t.Fatalf("PutObservations: %v", err)
	}
}
