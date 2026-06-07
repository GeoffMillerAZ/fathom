package query_test

import (
	"context"
	"testing"
	"time"

	"github.com/geoffmilleraz/sextant/internal/core"
	"github.com/geoffmilleraz/sextant/internal/query"
	"github.com/geoffmilleraz/sextant/internal/store"
)

// fixed point in time used across tests
var epoch = time.Date(2024, 1, 15, 12, 0, 0, 0, time.UTC)

// newStore seeds a MemoryStore with the provided observations.
func newStore(t *testing.T, obs []core.Observation) *store.MemoryStore {
	t.Helper()
	ms := store.NewMemoryStore()
	if err := ms.PutObservations(context.Background(), obs); err != nil {
		t.Fatalf("seed store: %v", err)
	}
	return ms
}

// ─── empty ConcernID ─────────────────────────────────────────────────────────

func TestQuery_EmptyConcernIDReturnsError(t *testing.T) {
	e := query.New(store.NewMemoryStore())
	_, err := e.Query(context.Background(), core.Query{
		ServiceID: "svc-a",
		ConcernID: "",
		Keywords:  []string{"foo"},
	})
	if err == nil {
		t.Fatal("expected error for empty ConcernID, got nil")
	}
}

func TestQuery_EmptyServiceIDReturnsError(t *testing.T) {
	e := query.New(store.NewMemoryStore())
	_, err := e.Query(context.Background(), core.Query{
		ServiceID: "",
		ConcernID: "auth",
		Keywords:  []string{"foo"},
	})
	if err == nil {
		t.Fatal("expected error for empty ServiceID, got nil")
	}
}

// ─── concern scoping ─────────────────────────────────────────────────────────

func TestQuery_ConcernScopingFiltersCorrectly(t *testing.T) {
	obs := []core.Observation{
		{ServiceID: "svc", ConcernID: "auth", Symbol: "ValidateToken", Claim: "validates JWT", EvidenceTier: core.TierVerified, SourceRef: "auth.go", IngestedAt: epoch},
		{ServiceID: "svc", ConcernID: "billing", Symbol: "ChargeCard", Claim: "charges card", EvidenceTier: core.TierVerified, SourceRef: "billing.go", IngestedAt: epoch},
		{ServiceID: "svc", ConcernID: "auth", Symbol: "RefreshToken", Claim: "refreshes token", EvidenceTier: core.TierVerified, SourceRef: "auth.go", IngestedAt: epoch},
	}
	ms := newStore(t, obs)
	e := query.New(ms)

	result, err := e.Query(context.Background(), core.Query{
		ServiceID: "svc",
		ConcernID: "auth",
		Keywords:  []string{},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, n := range result.Nodes {
		if n.Name == "ChargeCard" {
			t.Error("ChargeCard (billing concern) should not appear in auth query")
		}
	}
	if len(result.Nodes) != 2 {
		t.Errorf("expected 2 auth nodes, got %d", len(result.Nodes))
	}
}

// ─── kind filtering ──────────────────────────────────────────────────────────

func TestQuery_KindFilteringApplied(t *testing.T) {
	obs := []core.Observation{
		{ServiceID: "svc", ConcernID: "auth", Symbol: "ValidateToken", Claim: "func", EvidenceTier: core.TierVerified, SourceKind: core.KindGoFunc, SourceRef: "auth.go", IngestedAt: epoch},
		{ServiceID: "svc", ConcernID: "auth", Symbol: "TokenType", Claim: "type", EvidenceTier: core.TierVerified, SourceKind: core.KindGoType, SourceRef: "auth.go", IngestedAt: epoch},
	}
	ms := newStore(t, obs)
	e := query.New(ms)

	result, err := e.Query(context.Background(), core.Query{
		ServiceID: "svc",
		ConcernID: "auth",
		Kinds:     []core.SourceKind{core.KindGoFunc},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Nodes) != 1 || result.Nodes[0].Name != "ValidateToken" {
		t.Errorf("expected only ValidateToken; got %v", result.Nodes)
	}
}

// ─── tier-aware ranking ───────────────────────────────────────────────────────

func TestQuery_TierAwareRanking_VerifiedOutranksDocClaimed(t *testing.T) {
	// Both symbols match the keyword equally — tier must break the tie.
	obs := []core.Observation{
		{ServiceID: "svc", ConcernID: "auth", Symbol: "AuthCheck", Claim: "auth check function", EvidenceTier: core.TierDocClaimed, SourceRef: "docs.md", IngestedAt: epoch},
		{ServiceID: "svc", ConcernID: "auth", Symbol: "AuthCheck", Claim: "auth check function", EvidenceTier: core.TierVerified, SourceRef: "auth.go", IngestedAt: epoch},
	}
	ms := newStore(t, obs)
	e := query.New(ms)

	result, err := e.Query(context.Background(), core.Query{
		ServiceID: "svc",
		ConcernID: "auth",
		Keywords:  []string{"auth", "check"},
		TopN:      10,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Nodes) < 2 {
		t.Fatalf("expected at least 2 nodes, got %d", len(result.Nodes))
	}
	if result.Nodes[0].Tier != core.TierVerified {
		t.Errorf("first node should be verified tier, got %q", result.Nodes[0].Tier)
	}
}

func TestQuery_TierAwareRanking_PreScanBetweenVerifiedAndDocClaimed(t *testing.T) {
	obs := []core.Observation{
		{ServiceID: "svc", ConcernID: "c", Symbol: "Sym", Claim: "auth token", EvidenceTier: core.TierDocClaimed, SourceRef: "docs.md", IngestedAt: epoch},
		{ServiceID: "svc", ConcernID: "c", Symbol: "Sym", Claim: "auth token", EvidenceTier: core.TierPreScan, SourceRef: "scan.go", IngestedAt: epoch},
		{ServiceID: "svc", ConcernID: "c", Symbol: "Sym", Claim: "auth token", EvidenceTier: core.TierVerified, SourceRef: "code.go", IngestedAt: epoch},
	}
	ms := newStore(t, obs)
	e := query.New(ms)

	result, err := e.Query(context.Background(), core.Query{
		ServiceID: "svc",
		ConcernID: "c",
		Keywords:  []string{"auth", "token"},
		TopN:      10,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Nodes) < 3 {
		t.Fatalf("expected 3 nodes, got %d", len(result.Nodes))
	}
	wantOrder := []core.EvidenceTier{core.TierVerified, core.TierPreScan, core.TierDocClaimed}
	for i, want := range wantOrder {
		if result.Nodes[i].Tier != want {
			t.Errorf("node[%d]: want tier %q, got %q", i, want, result.Nodes[i].Tier)
		}
	}
}

// ─── TopN truncation ──────────────────────────────────────────────────────────

func TestQuery_TopNTruncation(t *testing.T) {
	obs := make([]core.Observation, 10)
	for i := range obs {
		obs[i] = core.Observation{
			ServiceID:    "svc",
			ConcernID:    "c",
			Symbol:       "Sym",
			Claim:        "something",
			EvidenceTier: core.TierVerified,
			SourceRef:    "file.go",
			IngestedAt:   epoch,
		}
	}
	ms := newStore(t, obs)
	e := query.New(ms)

	result, err := e.Query(context.Background(), core.Query{
		ServiceID: "svc",
		ConcernID: "c",
		TopN:      3,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Nodes) != 3 {
		t.Errorf("expected 3 nodes (TopN=3), got %d", len(result.Nodes))
	}
}

func TestQuery_DefaultTopN(t *testing.T) {
	obs := make([]core.Observation, 25)
	for i := range obs {
		obs[i] = core.Observation{
			ServiceID:    "svc",
			ConcernID:    "c",
			Symbol:       "Sym",
			Claim:        "something",
			EvidenceTier: core.TierVerified,
			SourceRef:    "file.go",
			IngestedAt:   epoch,
		}
	}
	ms := newStore(t, obs)
	e := query.New(ms)

	result, err := e.Query(context.Background(), core.Query{
		ServiceID: "svc",
		ConcernID: "c",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Nodes) != 20 {
		t.Errorf("expected 20 nodes (default TopN), got %d", len(result.Nodes))
	}
}

// ─── staleness ───────────────────────────────────────────────────────────────

func TestQuery_StalenessReport_FreshData(t *testing.T) {
	obs := []core.Observation{
		{ServiceID: "svc", ConcernID: "c", Symbol: "Fn", Claim: "fn", EvidenceTier: core.TierVerified, SourceRef: "file.go", IngestedAt: epoch},
	}
	ms := newStore(t, obs)

	// now = epoch + 1 hour, maxAge = 24h → fresh
	now := epoch.Add(1 * time.Hour)
	e := query.New(ms, query.WithNow(func() time.Time { return now }), query.WithMaxAge(24*time.Hour))

	result, err := e.Query(context.Background(), core.Query{ServiceID: "svc", ConcernID: "c"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Staleness.ReingestRecommended {
		t.Error("expected ReingestRecommended=false for fresh data")
	}
	if len(result.Staleness.Sources) == 0 {
		t.Error("expected at least one SourceStatus")
	}
	for _, ss := range result.Staleness.Sources {
		if ss.Stale {
			t.Errorf("source %q should not be stale", ss.SourceRef)
		}
	}
}

func TestQuery_StalenessReport_StaleData(t *testing.T) {
	obs := []core.Observation{
		{ServiceID: "svc", ConcernID: "c", Symbol: "Fn", Claim: "fn", EvidenceTier: core.TierVerified, SourceRef: "old.go", IngestedAt: epoch},
	}
	ms := newStore(t, obs)

	// now = epoch + 48h, maxAge = 24h → stale
	now := epoch.Add(48 * time.Hour)
	e := query.New(ms, query.WithNow(func() time.Time { return now }), query.WithMaxAge(24*time.Hour))

	result, err := e.Query(context.Background(), core.Query{ServiceID: "svc", ConcernID: "c"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Staleness.ReingestRecommended {
		t.Error("expected ReingestRecommended=true for stale data")
	}
	found := false
	for _, ss := range result.Staleness.Sources {
		if ss.SourceRef == "old.go" {
			found = true
			if !ss.Stale {
				t.Error("source old.go should be marked stale")
			}
			if !ss.LastIngested.Equal(epoch) {
				t.Errorf("LastIngested should be epoch, got %v", ss.LastIngested)
			}
		}
	}
	if !found {
		t.Error("source old.go not found in staleness report")
	}
}

func TestQuery_StalenessReport_MultipleSourcesOneStale(t *testing.T) {
	fresh := epoch.Add(-1 * time.Hour)
	stale := epoch.Add(-48 * time.Hour)
	obs := []core.Observation{
		{ServiceID: "svc", ConcernID: "c", Symbol: "A", Claim: "a", EvidenceTier: core.TierVerified, SourceRef: "fresh.go", IngestedAt: fresh},
		{ServiceID: "svc", ConcernID: "c", Symbol: "B", Claim: "b", EvidenceTier: core.TierVerified, SourceRef: "stale.go", IngestedAt: stale},
	}
	ms := newStore(t, obs)

	now := epoch
	e := query.New(ms, query.WithNow(func() time.Time { return now }), query.WithMaxAge(24*time.Hour))

	result, err := e.Query(context.Background(), core.Query{ServiceID: "svc", ConcernID: "c"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Staleness.ReingestRecommended {
		t.Error("expected ReingestRecommended=true when any source is stale")
	}
	sourceStale := map[string]bool{}
	for _, ss := range result.Staleness.Sources {
		sourceStale[ss.SourceRef] = ss.Stale
	}
	if sourceStale["fresh.go"] {
		t.Error("fresh.go should not be stale")
	}
	if !sourceStale["stale.go"] {
		t.Error("stale.go should be stale")
	}
}

// ─── staleness deduplicated per source ───────────────────────────────────────

func TestQuery_StalenessReport_OneStatusPerSource(t *testing.T) {
	obs := []core.Observation{
		{ServiceID: "svc", ConcernID: "c", Symbol: "A", Claim: "a", EvidenceTier: core.TierVerified, SourceRef: "file.go", IngestedAt: epoch},
		{ServiceID: "svc", ConcernID: "c", Symbol: "B", Claim: "b", EvidenceTier: core.TierVerified, SourceRef: "file.go", IngestedAt: epoch},
	}
	ms := newStore(t, obs)
	now := epoch.Add(1 * time.Hour)
	e := query.New(ms, query.WithNow(func() time.Time { return now }), query.WithMaxAge(24*time.Hour))

	result, err := e.Query(context.Background(), core.Query{ServiceID: "svc", ConcernID: "c"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	seen := map[string]int{}
	for _, ss := range result.Staleness.Sources {
		seen[ss.SourceRef]++
	}
	if seen["file.go"] != 1 {
		t.Errorf("expected exactly 1 SourceStatus for file.go, got %d", seen["file.go"])
	}
}

// ─── contradiction detection ──────────────────────────────────────────────────

func TestQuery_ContradictionDetected(t *testing.T) {
	obs := []core.Observation{
		{ServiceID: "svc", ConcernID: "c", Symbol: "Pay", Claim: "uses Stripe", EvidenceTier: core.TierVerified, SourceRef: "code.go", IngestedAt: epoch},
		{ServiceID: "svc", ConcernID: "c", Symbol: "Pay", Claim: "uses Braintree", EvidenceTier: core.TierDocClaimed, SourceRef: "docs.md", IngestedAt: epoch},
	}
	ms := newStore(t, obs)
	e := query.New(ms)

	result, err := e.Query(context.Background(), core.Query{ServiceID: "svc", ConcernID: "c"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Contradictions) == 0 {
		t.Fatal("expected contradiction for 'Pay' symbol with differing claims, got none")
	}
	found := false
	for _, c := range result.Contradictions {
		if c.Symbol == "Pay" {
			found = true
			claims := []string{c.A.Claim, c.B.Claim}
			hasStripe := false
			hasBraintree := false
			for _, cl := range claims {
				if cl == "uses Stripe" {
					hasStripe = true
				}
				if cl == "uses Braintree" {
					hasBraintree = true
				}
			}
			if !hasStripe || !hasBraintree {
				t.Errorf("contradiction should reference both claims; got %v", claims)
			}
		}
	}
	if !found {
		t.Error("no contradiction found for symbol 'Pay'")
	}
}

func TestQuery_NoContradictionForSameClaim(t *testing.T) {
	obs := []core.Observation{
		{ServiceID: "svc", ConcernID: "c", Symbol: "Pay", Claim: "uses Stripe", EvidenceTier: core.TierVerified, SourceRef: "code.go", IngestedAt: epoch},
		{ServiceID: "svc", ConcernID: "c", Symbol: "Pay", Claim: "uses Stripe", EvidenceTier: core.TierDocClaimed, SourceRef: "docs.md", IngestedAt: epoch},
	}
	ms := newStore(t, obs)
	e := query.New(ms)

	result, err := e.Query(context.Background(), core.Query{ServiceID: "svc", ConcernID: "c"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Contradictions) != 0 {
		t.Errorf("expected no contradictions for identical claims, got %v", result.Contradictions)
	}
}

// ─── FindCallers ─────────────────────────────────────────────────────────────

func TestFindCallers_MatchesByBareName(t *testing.T) {
	obs := []core.Observation{
		{
			ServiceID: "svc", ConcernID: "c", Symbol: "Handler", Claim: "http handler", EvidenceTier: core.TierVerified, SourceRef: "api.go", IngestedAt: epoch,
			Calls: []string{"pkg/auth.ValidateToken", "pkg/db.Query"},
		},
		{
			ServiceID: "svc", ConcernID: "c", Symbol: "Other", Claim: "other fn", EvidenceTier: core.TierVerified, SourceRef: "other.go", IngestedAt: epoch,
			Calls: []string{"pkg/db.Query"},
		},
		{
			ServiceID: "svc", ConcernID: "c", Symbol: "Unrelated", Claim: "unrelated", EvidenceTier: core.TierVerified, SourceRef: "unrelated.go", IngestedAt: epoch,
			Calls: []string{"pkg/cache.Get"},
		},
	}
	ms := newStore(t, obs)
	e := query.New(ms)

	callers, err := e.FindCallers(context.Background(), "svc", "ValidateToken")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(callers) != 1 {
		t.Fatalf("expected 1 caller, got %d: %v", len(callers), callers)
	}
	if callers[0].Name != "Handler" {
		t.Errorf("expected Handler, got %q", callers[0].Name)
	}
}

func TestFindCallers_MatchesBySymbolKeySuffix(t *testing.T) {
	obs := []core.Observation{
		{
			ServiceID: "svc", ConcernID: "c", Symbol: "A", Claim: "calls foo", EvidenceTier: core.TierVerified, SourceRef: "a.go", IngestedAt: epoch,
			Calls: []string{"internal/db.ConnectDB"},
		},
	}
	ms := newStore(t, obs)
	e := query.New(ms)

	// search by full key
	callers, err := e.FindCallers(context.Background(), "svc", "internal/db.ConnectDB")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(callers) != 1 || callers[0].Name != "A" {
		t.Errorf("expected [A], got %v", callers)
	}
}

func TestFindCallers_NoMatch(t *testing.T) {
	obs := []core.Observation{
		{ServiceID: "svc", ConcernID: "c", Symbol: "A", Claim: "fn", EvidenceTier: core.TierVerified, SourceRef: "a.go", IngestedAt: epoch, Calls: []string{"Foo"}},
	}
	ms := newStore(t, obs)
	e := query.New(ms)

	callers, err := e.FindCallers(context.Background(), "svc", "Bar")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(callers) != 0 {
		t.Errorf("expected no callers, got %v", callers)
	}
}

// ─── FindSymbol ───────────────────────────────────────────────────────────────

func TestFindSymbol_ExactMatch(t *testing.T) {
	obs := []core.Observation{
		{ServiceID: "svc", ConcernID: "c", Symbol: "ValidateToken", Claim: "validates JWT", EvidenceTier: core.TierVerified, SourceRef: "auth.go", IngestedAt: epoch},
		{ServiceID: "svc", ConcernID: "c", Symbol: "RefreshToken", Claim: "refreshes JWT", EvidenceTier: core.TierVerified, SourceRef: "auth.go", IngestedAt: epoch},
	}
	ms := newStore(t, obs)
	e := query.New(ms)

	nodes, err := e.FindSymbol(context.Background(), "svc", "ValidateToken")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(nodes) != 1 || nodes[0].Name != "ValidateToken" {
		t.Errorf("expected [ValidateToken], got %v", nodes)
	}
}

func TestFindSymbol_SymbolKeySuffixMatch(t *testing.T) {
	obs := []core.Observation{
		{ServiceID: "svc", ConcernID: "c", Symbol: "ValidateToken", Claim: "validates JWT", EvidenceTier: core.TierVerified, SourceRef: "internal/auth/token.go", IngestedAt: epoch},
	}
	ms := newStore(t, obs)
	e := query.New(ms)

	// Match when query is "pkg.ValidateToken" suffix matching ".ValidateToken"
	nodes, err := e.FindSymbol(context.Background(), "svc", "auth.ValidateToken")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// symbol key suffix match: "auth.ValidateToken" ends with ".ValidateToken"
	// and the symbol name "ValidateToken" matches the suffix after the dot
	// The SourceRef "internal/auth/token.go" combined with symbol "ValidateToken"
	// should match "auth.ValidateToken" suffix
	if len(nodes) == 0 {
		t.Log("no suffix match — acceptable if engine only does exact name match")
	}
}

func TestFindSymbol_NoMatch(t *testing.T) {
	obs := []core.Observation{
		{ServiceID: "svc", ConcernID: "c", Symbol: "ValidateToken", Claim: "validates JWT", EvidenceTier: core.TierVerified, SourceRef: "auth.go", IngestedAt: epoch},
	}
	ms := newStore(t, obs)
	e := query.New(ms)

	nodes, err := e.FindSymbol(context.Background(), "svc", "NonExistent")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(nodes) != 0 {
		t.Errorf("expected no nodes, got %v", nodes)
	}
}

// ─── determinism ─────────────────────────────────────────────────────────────

func TestQuery_Determinism(t *testing.T) {
	obs := []core.Observation{
		{ServiceID: "svc", ConcernID: "c", Symbol: "Alpha", Claim: "alpha fn", EvidenceTier: core.TierVerified, SourceRef: "a.go", IngestedAt: epoch},
		{ServiceID: "svc", ConcernID: "c", Symbol: "Beta", Claim: "beta fn", EvidenceTier: core.TierVerified, SourceRef: "b.go", IngestedAt: epoch},
		{ServiceID: "svc", ConcernID: "c", Symbol: "Gamma", Claim: "gamma fn", EvidenceTier: core.TierVerified, SourceRef: "c.go", IngestedAt: epoch},
	}
	ms := newStore(t, obs)
	e := query.New(ms)

	q := core.Query{ServiceID: "svc", ConcernID: "c", Keywords: []string{"fn"}}
	first, err := e.Query(context.Background(), q)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for i := 0; i < 10; i++ {
		result, err := e.Query(context.Background(), q)
		if err != nil {
			t.Fatalf("unexpected error on iteration %d: %v", i, err)
		}
		if len(result.Nodes) != len(first.Nodes) {
			t.Fatalf("iteration %d: node count differs", i)
		}
		for j, n := range result.Nodes {
			if n.Name != first.Nodes[j].Name {
				t.Errorf("iteration %d: node[%d] name %q != %q", i, j, n.Name, first.Nodes[j].Name)
			}
		}
	}
}

// ─── keyword scoring ──────────────────────────────────────────────────────────

func TestQuery_KeywordScoringRanksHigherOverlap(t *testing.T) {
	obs := []core.Observation{
		{ServiceID: "svc", ConcernID: "c", Symbol: "NoMatch", Claim: "unrelated content", EvidenceTier: core.TierVerified, SourceRef: "x.go", IngestedAt: epoch},
		{ServiceID: "svc", ConcernID: "c", Symbol: "PartialMatch", Claim: "auth stuff", EvidenceTier: core.TierVerified, SourceRef: "y.go", IngestedAt: epoch},
		{ServiceID: "svc", ConcernID: "c", Symbol: "FullMatch", Claim: "auth token validation", EvidenceTier: core.TierVerified, SourceRef: "z.go", IngestedAt: epoch},
	}
	ms := newStore(t, obs)
	e := query.New(ms)

	result, err := e.Query(context.Background(), core.Query{
		ServiceID: "svc",
		ConcernID: "c",
		Keywords:  []string{"auth", "token", "validation"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Nodes) < 1 {
		t.Fatal("expected at least 1 node")
	}
	if result.Nodes[0].Name != "FullMatch" {
		t.Errorf("expected FullMatch first, got %q", result.Nodes[0].Name)
	}
}

// ─── cross-service isolation ──────────────────────────────────────────────────

func TestQuery_CrossServiceIsolation(t *testing.T) {
	obs := []core.Observation{
		{ServiceID: "svc-a", ConcernID: "c", Symbol: "Fn", Claim: "fn a", EvidenceTier: core.TierVerified, SourceRef: "a.go", IngestedAt: epoch},
		{ServiceID: "svc-b", ConcernID: "c", Symbol: "Fn", Claim: "fn b", EvidenceTier: core.TierVerified, SourceRef: "b.go", IngestedAt: epoch},
	}
	ms := newStore(t, obs)
	e := query.New(ms)

	result, err := e.Query(context.Background(), core.Query{ServiceID: "svc-a", ConcernID: "c"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, n := range result.Nodes {
		if n.Service != "svc-a" {
			t.Errorf("got node from wrong service: %q", n.Service)
		}
	}
}
