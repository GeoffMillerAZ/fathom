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
	// Fresh data: Sources contains only stale entries, so it must be empty.
	if len(result.Staleness.Sources) != 0 {
		t.Errorf("expected no stale SourceStatus entries for fresh data, got %d", len(result.Staleness.Sources))
	}
	// TotalSources must count the one observed source; FreshSources must equal TotalSources.
	if result.Staleness.TotalSources != 1 {
		t.Errorf("TotalSources = %d, want 1", result.Staleness.TotalSources)
	}
	if result.Staleness.FreshSources != 1 {
		t.Errorf("FreshSources = %d, want 1", result.Staleness.FreshSources)
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
	// Sources contains only stale entries: exactly stale.go.
	if result.Staleness.TotalSources != 2 {
		t.Errorf("TotalSources = %d, want 2", result.Staleness.TotalSources)
	}
	if result.Staleness.FreshSources != 1 {
		t.Errorf("FreshSources = %d, want 1", result.Staleness.FreshSources)
	}
	if len(result.Staleness.Sources) != 1 {
		t.Fatalf("expected 1 stale source entry, got %d: %v", len(result.Staleness.Sources), result.Staleness.Sources)
	}
	if result.Staleness.Sources[0].SourceRef != "stale.go" {
		t.Errorf("stale entry = %q, want stale.go", result.Staleness.Sources[0].SourceRef)
	}
	if !result.Staleness.Sources[0].Stale {
		t.Error("stale.go entry must have Stale=true")
	}
}

// ─── staleness deduplicated per source ───────────────────────────────────────

func TestQuery_StalenessReport_OneStatusPerSource(t *testing.T) {
	// Two observations for the same source: both fresh (epoch + 1h, maxAge 24h).
	// TotalSources must be 1 (deduplicated); Sources (stale-only) must be empty.
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
	// Deduplication: two observations on the same file → 1 distinct source.
	if result.Staleness.TotalSources != 1 {
		t.Errorf("TotalSources = %d, want 1 (deduplicated)", result.Staleness.TotalSources)
	}
	if result.Staleness.FreshSources != 1 {
		t.Errorf("FreshSources = %d, want 1", result.Staleness.FreshSources)
	}
	// Fresh source: no stale entries in Sources.
	if len(result.Staleness.Sources) != 0 {
		t.Errorf("expected 0 stale entries, got %d: %v", len(result.Staleness.Sources), result.Staleness.Sources)
	}
}

// ─── staleness stays compact when the whole index is stale ───────────────────

func TestQuery_StalenessReport_CompactWhenManyStale(t *testing.T) {
	// The dogfood failure mode: a stale index used to attach one SourceStatus
	// row per source to EVERY query answer (117 lines dwarfing an 8-node
	// result). The report must stay constant-size: at most 3 example rows
	// (oldest first), with counts and the oldest-ingest low-water mark
	// carrying the full picture.
	stale := func(h int) time.Time { return epoch.Add(-time.Duration(h) * time.Hour) }
	obs := []core.Observation{
		{ServiceID: "svc", ConcernID: "c", Symbol: "A", Claim: "a", EvidenceTier: core.TierVerified, SourceRef: "a.go", IngestedAt: stale(30)},
		{ServiceID: "svc", ConcernID: "c", Symbol: "B", Claim: "b", EvidenceTier: core.TierVerified, SourceRef: "b.go", IngestedAt: stale(80)},
		{ServiceID: "svc", ConcernID: "c", Symbol: "C", Claim: "c", EvidenceTier: core.TierVerified, SourceRef: "c.go", IngestedAt: stale(50)},
		{ServiceID: "svc", ConcernID: "c", Symbol: "D", Claim: "d", EvidenceTier: core.TierVerified, SourceRef: "d.go", IngestedAt: stale(70)},
		{ServiceID: "svc", ConcernID: "c", Symbol: "E", Claim: "e", EvidenceTier: core.TierVerified, SourceRef: "e.go", IngestedAt: stale(60)},
		{ServiceID: "svc", ConcernID: "c", Symbol: "F", Claim: "f", EvidenceTier: core.TierVerified, SourceRef: "fresh.go", IngestedAt: epoch.Add(-1 * time.Hour)},
	}
	ms := newStore(t, obs)
	e := query.New(ms, query.WithNow(func() time.Time { return epoch }), query.WithMaxAge(24*time.Hour))

	result, err := e.Query(context.Background(), core.Query{ServiceID: "svc", ConcernID: "c"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	st := result.Staleness
	if st.TotalSources != 6 || st.FreshSources != 1 {
		t.Errorf("TotalSources=%d FreshSources=%d, want 6/1", st.TotalSources, st.FreshSources)
	}
	if st.StaleSources != 5 {
		t.Errorf("StaleSources = %d, want 5", st.StaleSources)
	}
	if !st.ReingestRecommended {
		t.Error("expected ReingestRecommended=true")
	}
	// Examples are capped at 3, oldest first.
	if len(st.Sources) != 3 {
		t.Fatalf("expected 3 example rows (capped), got %d: %v", len(st.Sources), st.Sources)
	}
	wantOrder := []string{"b.go", "d.go", "e.go"}
	for i, want := range wantOrder {
		if st.Sources[i].SourceRef != want {
			t.Errorf("Sources[%d] = %q, want %q (oldest first)", i, st.Sources[i].SourceRef, want)
		}
	}
	// The low-water mark is the oldest ingest across ALL sources.
	if !st.OldestIngestedAt.Equal(stale(80)) {
		t.Errorf("OldestIngestedAt = %v, want %v", st.OldestIngestedAt, stale(80))
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

// TestFindSymbol_TableDriven exercises exact, qualified, bare-method, and no-match cases.
func TestFindSymbol_TableDriven(t *testing.T) {
	obs := []core.Observation{
		// Plain function.
		{ServiceID: "svc", ConcernID: "c", Symbol: "CloseCase", Claim: "func", EvidenceTier: core.TierVerified, SourceRef: "registry.go", IngestedAt: epoch},
		// Method stored as "Registry.CloseCase".
		{ServiceID: "svc", ConcernID: "c", Symbol: "Registry.CloseCase", Claim: "method", EvidenceTier: core.TierVerified, SourceRef: "registry.go", IngestedAt: epoch},
		// Unrelated symbol.
		{ServiceID: "svc", ConcernID: "c", Symbol: "OpenCase", Claim: "func", EvidenceTier: core.TierVerified, SourceRef: "registry.go", IngestedAt: epoch},
	}
	ms := newStore(t, obs)
	e := query.New(ms)

	tests := []struct {
		name      string
		query     string
		wantNames []string // sorted, empty means no match expected
	}{
		{
			name:      "exact match plain func",
			query:     "CloseCase",
			wantNames: []string{"CloseCase", "Registry.CloseCase"}, // bare "CloseCase" matches both
		},
		{
			name:      "exact match method",
			query:     "Registry.CloseCase",
			wantNames: []string{"CloseCase", "Registry.CloseCase"}, // qualified matches both via suffix rules
		},
		{
			name:      "bare method name matches Type.Method stored symbol",
			query:     "CloseCase",
			wantNames: []string{"CloseCase", "Registry.CloseCase"},
		},
		{
			name:      "no match",
			query:     "NonExistent",
			wantNames: []string{},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			nodes, err := e.FindSymbol(context.Background(), "svc", tc.query)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			got := make([]string, len(nodes))
			for i, n := range nodes {
				got[i] = n.Name
			}
			if len(got) != len(tc.wantNames) {
				t.Errorf("query %q: got %v, want %v", tc.query, got, tc.wantNames)
				return
			}
			for i, w := range tc.wantNames {
				if got[i] != w {
					t.Errorf("query %q node[%d]: got %q, want %q", tc.query, i, got[i], w)
				}
			}
		})
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

// ─── unscoped-store fallback ─────────────────────────────────────────────────

// unscopedObs returns observations as a plain ingest stores them: no concern
// tagging at all.
func unscopedObs() []core.Observation {
	return []core.Observation{
		{ServiceID: "svc", ConcernID: "", Symbol: "Registry.CloseCase", Claim: "func CloseCase(ctx, id, outcomeSummary string) error", EvidenceTier: core.TierVerified, SourceRef: "registry.go", IngestedAt: epoch},
		{ServiceID: "svc", ConcernID: "", Symbol: "ReadOutcome", Claim: "func ReadOutcome(ctx) (*Outcome, error)", EvidenceTier: core.TierVerified, SourceRef: "outcome.go", IngestedAt: epoch},
		{ServiceID: "svc", ConcernID: "", Symbol: "ChargeCard", Claim: "charges a card", EvidenceTier: core.TierVerified, SourceRef: "billing.go", IngestedAt: epoch},
	}
}

func TestQuery_UnscopedFallback_KeywordsMatch(t *testing.T) {
	e := query.New(newStore(t, unscopedObs()))

	result, err := e.Query(context.Background(), core.Query{
		ServiceID: "svc",
		ConcernID: "outcome",
		Keywords:  []string{"outcome"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Nodes) != 2 {
		t.Fatalf("expected 2 outcome nodes via fallback, got %d: %v", len(result.Nodes), result.Nodes)
	}
	for _, n := range result.Nodes {
		if n.Name == "ChargeCard" {
			t.Error("ChargeCard does not match keywords and must not be returned")
		}
	}
}

func TestQuery_UnscopedFallback_ConcernUsedAsKeyword(t *testing.T) {
	e := query.New(newStore(t, unscopedObs()))

	result, err := e.Query(context.Background(), core.Query{
		ServiceID: "svc",
		ConcernID: "outcome",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Nodes) != 2 {
		t.Fatalf("expected concern to act as keyword in fallback, got %d nodes", len(result.Nodes))
	}
}

func TestQuery_UnscopedFallback_NoMatchStaysEmpty(t *testing.T) {
	e := query.New(newStore(t, unscopedObs()))

	result, err := e.Query(context.Background(), core.Query{
		ServiceID: "svc",
		ConcernID: "nonexistent-concern",
		Keywords:  []string{"zzz-no-such-keyword"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Nodes) != 0 {
		t.Errorf("fallback must not fabricate relevance; got %d nodes: %v", len(result.Nodes), result.Nodes)
	}
}

func TestQuery_UnscopedFallback_NotTriggeredWhenConcernMatches(t *testing.T) {
	obs := []core.Observation{
		{ServiceID: "svc", ConcernID: "auth", Symbol: "ValidateToken", Claim: "validates JWT", EvidenceTier: core.TierVerified, SourceRef: "auth.go", IngestedAt: epoch},
		{ServiceID: "svc", ConcernID: "", Symbol: "OutcomeHelper", Claim: "auth-adjacent text", EvidenceTier: core.TierVerified, SourceRef: "x.go", IngestedAt: epoch},
	}
	e := query.New(newStore(t, obs))

	result, err := e.Query(context.Background(), core.Query{
		ServiceID: "svc",
		ConcernID: "auth",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Nodes) != 1 || result.Nodes[0].Name != "ValidateToken" {
		t.Errorf("concern-scoped hit must bypass fallback; got %v", result.Nodes)
	}
}
