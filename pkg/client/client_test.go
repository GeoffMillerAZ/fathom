// Package client_test contains end-to-end tests for the public fathom client.
// Tests use FauxConnector + MemoryStore (or the SQLite-backed path via
// NewConstantClient) so they are network-free, secret-free, and deterministic.
package client_test

import (
	"context"
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/internal/connector"
	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/store"
	"github.com/geoffmilleraz/fathom/pkg/client"
)

// paymentSvc is the ServiceRef for the fauxtown payment-service fixture.
var paymentSvc = client.ServiceRef{
	ID:   "payment-service",
	Path: "payment-service",
}

// newFauxClient creates a Client backed by FauxConnector and a fresh
// MemoryStore.  It also returns the store so tests can seed ConcernID-tagged
// observations for QuerySymbols.
func newFauxClient(t *testing.T) (*client.Client, *store.MemoryStore) {
	t.Helper()
	conn := connector.NewFauxConnector()
	ms := store.NewMemoryStore()
	c := client.New(conn, ms)
	t.Cleanup(func() { _ = c.Close() })
	return c, ms
}

// seedConcernObs writes observations with a specific ConcernID into st.
// This is necessary because the fusion pipeline stores all observations with
// ConcernID "" and the query engine requires a non-empty ConcernID.
func seedConcernObs(t *testing.T, st *store.MemoryStore, serviceID, concernID string, obs []core.Observation) {
	t.Helper()
	ctx := context.Background()
	for i := range obs {
		obs[i].ServiceID = serviceID
		obs[i].ConcernID = concernID
	}
	if err := st.PutObservations(ctx, obs); err != nil {
		t.Fatalf("seedConcernObs: %v", err)
	}
}

// ─── Ingest ──────────────────────────────────────────────────────────────────

// TestIngest_PaymentService verifies that ingesting the fauxtown payment-service
// discovers and extracts the expected number of symbols.
func TestIngest_PaymentService(t *testing.T) {
	c, _ := newFauxClient(t)
	ctx := context.Background()

	report, err := c.Ingest(ctx, paymentSvc)
	if err != nil {
		t.Fatalf("Ingest: %v", err)
	}

	// payment-service has 2 .go files (payment.go + errors.go).
	if report.Discovered != 2 {
		t.Errorf("Discovered = %d, want 2", report.Discovered)
	}
	// First ingest: nothing skipped.
	if report.Skipped != 0 {
		t.Errorf("Skipped = %d, want 0 on first ingest", report.Skipped)
	}
	// payment.go exports 5 symbols (3 types/interface + 2 funcs); errors.go
	// has only unexported vars → 0 symbols.
	const wantSymbols = 5
	if report.Symbols != wantSymbols {
		t.Errorf("Symbols = %d, want %d", report.Symbols, wantSymbols)
	}
}

// TestIngest_HashGate verifies that re-ingesting unchanged files skips them.
func TestIngest_HashGate(t *testing.T) {
	c, _ := newFauxClient(t)
	ctx := context.Background()

	if _, err := c.Ingest(ctx, paymentSvc); err != nil {
		t.Fatalf("first Ingest: %v", err)
	}
	r2, err := c.Ingest(ctx, paymentSvc)
	if err != nil {
		t.Fatalf("second Ingest: %v", err)
	}
	if r2.Skipped != r2.Discovered {
		t.Errorf("second ingest: Skipped=%d Discovered=%d; want all skipped",
			r2.Skipped, r2.Discovered)
	}
	if r2.Symbols != 0 {
		t.Errorf("second ingest: Symbols=%d, want 0 (hash-gated)", r2.Symbols)
	}
}

// ─── IngestBatch ─────────────────────────────────────────────────────────────

// TestIngestBatch ingests both fauxtown services and checks that both reports
// are returned.
func TestIngestBatch(t *testing.T) {
	c, _ := newFauxClient(t)
	ctx := context.Background()

	svcs := []client.ServiceRef{
		{ID: "payment-service", Path: "payment-service"},
		{ID: "ledger-service", Path: "ledger-service"},
	}
	reports, err := c.IngestBatch(ctx, svcs)
	if err != nil {
		t.Fatalf("IngestBatch: %v", err)
	}
	if len(reports) != 2 {
		t.Fatalf("got %d reports, want 2", len(reports))
	}
	for i, r := range reports {
		if r.Discovered == 0 {
			t.Errorf("report[%d]: Discovered=0, expected at least 1 file", i)
		}
	}
}

// ─── QuerySymbols ─────────────────────────────────────────────────────────────

// TestQuerySymbols_ConcernAndKeyword ingests payment-service, seeds
// ConcernID-tagged observations, then queries by concern + keyword.
func TestQuerySymbols_ConcernAndKeyword(t *testing.T) {
	c, ms := newFauxClient(t)
	ctx := context.Background()

	// Ingest first so fingerprints + unscoped observations are present.
	if _, err := c.Ingest(ctx, paymentSvc); err != nil {
		t.Fatalf("Ingest: %v", err)
	}

	now := time.Now().UTC()
	// Seed concern-scoped observations matching the ingested symbols.
	seedConcernObs(t, ms, paymentSvc.ID, "payments", []core.Observation{
		{
			SourceRef:    "payment-service/payment.go",
			SourceKind:   core.KindGoFunc,
			EvidenceTier: core.TierVerified,
			Symbol:       "Charge",
			Claim:        "func Charge(ctx context.Context, req ChargeRequest) (ChargeResult, error)",
			IngestedAt:   now,
		},
		{
			SourceRef:    "payment-service/payment.go",
			SourceKind:   core.KindGoFunc,
			EvidenceTier: core.TierVerified,
			Symbol:       "validateCharge",
			Claim:        "func validateCharge(req ChargeRequest) error",
			IngestedAt:   now,
		},
		{
			SourceRef:    "payment-service/payment.go",
			SourceKind:   core.KindGoType,
			EvidenceTier: core.TierVerified,
			Symbol:       "ChargeRequest",
			Claim:        "type ChargeRequest struct",
			IngestedAt:   now,
		},
	})

	result, err := c.QuerySymbols(ctx, client.Query{
		ServiceID: paymentSvc.ID,
		ConcernID: "payments",
		Keywords:  []string{"Charge"},
		TopN:      10,
	})
	if err != nil {
		t.Fatalf("QuerySymbols: %v", err)
	}
	if len(result.Nodes) == 0 {
		t.Fatal("QuerySymbols returned no nodes; expected at least one")
	}

	// The first node should have the highest keyword overlap — "Charge" appears
	// in both symbol name and claim.
	found := false
	for _, n := range result.Nodes {
		if n.Name == "Charge" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected symbol 'Charge' in results; got %v", symbolNames(result.Nodes))
	}
}

// TestQuerySymbols_KindFilter verifies that the Kinds filter restricts results.
func TestQuerySymbols_KindFilter(t *testing.T) {
	c, ms := newFauxClient(t)
	ctx := context.Background()

	if _, err := c.Ingest(ctx, paymentSvc); err != nil {
		t.Fatalf("Ingest: %v", err)
	}

	now := time.Now().UTC()
	seedConcernObs(t, ms, paymentSvc.ID, "types", []core.Observation{
		{SourceRef: "payment-service/payment.go", SourceKind: core.KindGoType, EvidenceTier: core.TierVerified, Symbol: "ChargeRequest", Claim: "type ChargeRequest struct", IngestedAt: now},
		{SourceRef: "payment-service/payment.go", SourceKind: core.KindGoFunc, EvidenceTier: core.TierVerified, Symbol: "Charge", Claim: "func Charge(...)", IngestedAt: now},
	})

	result, err := c.QuerySymbols(ctx, client.Query{
		ServiceID: paymentSvc.ID,
		ConcernID: "types",
		Kinds:     []client.SourceKind{core.KindGoType},
	})
	if err != nil {
		t.Fatalf("QuerySymbols with kind filter: %v", err)
	}
	for _, n := range result.Nodes {
		if n.Kind != core.KindGoType {
			t.Errorf("kind filter leaked non-GoType node: %v", n)
		}
	}
	if len(result.Nodes) == 0 {
		t.Error("expected at least one GoType node")
	}
}

// ─── FindCallers ─────────────────────────────────────────────────────────────

// TestFindCallers_ValidateCharge verifies that after ingesting payment-service,
// FindCallers("validateCharge") returns Charge (the only caller).
func TestFindCallers_ValidateCharge(t *testing.T) {
	c, _ := newFauxClient(t)
	ctx := context.Background()

	if _, err := c.Ingest(ctx, paymentSvc); err != nil {
		t.Fatalf("Ingest: %v", err)
	}

	// validateCharge is called only by Charge.  The call edge is stored as the
	// full SymbolKey "payment-service::payment.validateCharge".
	callers, err := c.FindCallers(ctx, paymentSvc.ID, "validateCharge")
	if err != nil {
		t.Fatalf("FindCallers: %v", err)
	}
	if len(callers) == 0 {
		t.Fatal("FindCallers returned no callers; expected Charge to call validateCharge")
	}
	found := false
	for _, caller := range callers {
		if caller.Name == "Charge" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected 'Charge' in callers; got %v", symbolNames(callers))
	}
}

// TestFindCallers_NoCallers verifies that a symbol with no callers returns an
// empty slice without an error.
func TestFindCallers_NoCallers(t *testing.T) {
	c, _ := newFauxClient(t)
	ctx := context.Background()

	if _, err := c.Ingest(ctx, paymentSvc); err != nil {
		t.Fatalf("Ingest: %v", err)
	}

	callers, err := c.FindCallers(ctx, paymentSvc.ID, "NonExistentSymbol")
	if err != nil {
		t.Fatalf("FindCallers: %v", err)
	}
	if len(callers) != 0 {
		t.Errorf("expected no callers, got %v", symbolNames(callers))
	}
}

// ─── FindSymbol ──────────────────────────────────────────────────────────────

// TestFindSymbol_Charge verifies that FindSymbol returns the Charge function.
func TestFindSymbol_Charge(t *testing.T) {
	c, _ := newFauxClient(t)
	ctx := context.Background()

	if _, err := c.Ingest(ctx, paymentSvc); err != nil {
		t.Fatalf("Ingest: %v", err)
	}

	nodes, err := c.FindSymbol(ctx, paymentSvc.ID, "Charge")
	if err != nil {
		t.Fatalf("FindSymbol: %v", err)
	}
	if len(nodes) == 0 {
		t.Fatal("FindSymbol('Charge') returned no nodes")
	}
	for _, n := range nodes {
		if n.Name != "Charge" {
			t.Errorf("unexpected symbol %q in FindSymbol('Charge') results", n.Name)
		}
	}
}

// TestFindSymbol_NotFound verifies that FindSymbol returns an empty slice for
// unknown symbols.
func TestFindSymbol_NotFound(t *testing.T) {
	c, _ := newFauxClient(t)
	ctx := context.Background()

	if _, err := c.Ingest(ctx, paymentSvc); err != nil {
		t.Fatalf("Ingest: %v", err)
	}

	nodes, err := c.FindSymbol(ctx, paymentSvc.ID, "DoesNotExist")
	if err != nil {
		t.Fatalf("FindSymbol: %v", err)
	}
	if len(nodes) != 0 {
		t.Errorf("expected no nodes, got %v", symbolNames(nodes))
	}
}

// ─── BatchQuery ───────────────────────────────────────────────────────────────

// TestBatchQuery runs two queries in one call and validates both results.
func TestBatchQuery(t *testing.T) {
	c, ms := newFauxClient(t)
	ctx := context.Background()

	if _, err := c.Ingest(ctx, paymentSvc); err != nil {
		t.Fatalf("Ingest: %v", err)
	}

	now := time.Now().UTC()
	seedConcernObs(t, ms, paymentSvc.ID, "concern-a", []core.Observation{
		{SourceRef: "payment-service/payment.go", SourceKind: core.KindGoFunc, EvidenceTier: core.TierVerified, Symbol: "Charge", Claim: "func Charge", IngestedAt: now},
	})
	seedConcernObs(t, ms, paymentSvc.ID, "concern-b", []core.Observation{
		{SourceRef: "payment-service/payment.go", SourceKind: core.KindGoType, EvidenceTier: core.TierVerified, Symbol: "ChargeRequest", Claim: "type ChargeRequest struct", IngestedAt: now},
	})

	qs := []client.Query{
		{ServiceID: paymentSvc.ID, ConcernID: "concern-a", Keywords: []string{"Charge"}},
		{ServiceID: paymentSvc.ID, ConcernID: "concern-b", Keywords: []string{"ChargeRequest"}},
	}
	results, err := c.BatchQuery(ctx, qs)
	if err != nil {
		t.Fatalf("BatchQuery: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("BatchQuery: got %d results, want 2", len(results))
	}
	if len(results[0].Nodes) == 0 {
		t.Error("BatchQuery[0]: expected at least one node for concern-a / Charge")
	}
	if len(results[1].Nodes) == 0 {
		t.Error("BatchQuery[1]: expected at least one node for concern-b / ChargeRequest")
	}
}

// TestBatchQuery_ErrorAborts verifies that an invalid query causes BatchQuery
// to return an error and the partial results collected so far.
func TestBatchQuery_ErrorAborts(t *testing.T) {
	c, ms := newFauxClient(t)
	ctx := context.Background()

	if _, err := c.Ingest(ctx, paymentSvc); err != nil {
		t.Fatalf("Ingest: %v", err)
	}

	now := time.Now().UTC()
	seedConcernObs(t, ms, paymentSvc.ID, "valid-concern", []core.Observation{
		{SourceRef: "payment-service/payment.go", SourceKind: core.KindGoFunc, EvidenceTier: core.TierVerified, Symbol: "Charge", Claim: "func Charge", IngestedAt: now},
	})

	qs := []client.Query{
		{ServiceID: paymentSvc.ID, ConcernID: "valid-concern"},
		// second query has empty ConcernID — engine will reject it.
		{ServiceID: paymentSvc.ID, ConcernID: ""},
	}
	results, err := c.BatchQuery(ctx, qs)
	if err == nil {
		t.Fatal("BatchQuery: expected error for empty ConcernID, got nil")
	}
	// Partial results from the first (valid) query should be returned.
	if len(results) != 1 {
		t.Errorf("BatchQuery partial results: got %d, want 1", len(results))
	}
}

// ─── Determinism ─────────────────────────────────────────────────────────────

// TestDeterminism verifies that running Ingest + FindCallers twice on the same
// fixture produces identical results.
func TestDeterminism(t *testing.T) {
	ctx := context.Background()

	run := func() []string {
		conn := connector.NewFauxConnector()
		ms := store.NewMemoryStore()
		c := client.New(conn, ms)
		defer c.Close()

		if _, err := c.Ingest(ctx, paymentSvc); err != nil {
			t.Fatalf("Ingest: %v", err)
		}
		nodes, err := c.FindCallers(ctx, paymentSvc.ID, "validateCharge")
		if err != nil {
			t.Fatalf("FindCallers: %v", err)
		}
		return symbolNames(nodes)
	}

	first := run()
	second := run()

	if len(first) != len(second) {
		t.Fatalf("non-deterministic result length: %d vs %d", len(first), len(second))
	}
	for i := range first {
		if first[i] != second[i] {
			t.Errorf("non-deterministic at [%d]: %q vs %q", i, first[i], second[i])
		}
	}
}

// ─── NewConstantClient (SQLite-backed) ───────────────────────────────────────

// TestNewConstantClient_FauxInMemory proves the SQLite-backed path works
// end-to-end using ModeFaux + in-memory store ("").
func TestNewConstantClient_FauxInMemory(t *testing.T) {
	ctx := context.Background()

	c, err := client.NewConstantClient(client.ModeFaux, "")
	if err != nil {
		t.Fatalf("NewConstantClient(ModeFaux, \"\"): %v", err)
	}
	defer func() {
		if closeErr := c.Close(); closeErr != nil {
			t.Errorf("Close: %v", closeErr)
		}
	}()

	report, err := c.Ingest(ctx, paymentSvc)
	if err != nil {
		t.Fatalf("Ingest via NewConstantClient: %v", err)
	}
	if report.Discovered == 0 {
		t.Error("expected at least one file discovered")
	}
	if report.Symbols == 0 {
		t.Error("expected at least one symbol extracted")
	}

	// FindCallers must work end-to-end through the SQLite store.
	callers, err := c.FindCallers(ctx, paymentSvc.ID, "validateCharge")
	if err != nil {
		t.Fatalf("FindCallers via NewConstantClient: %v", err)
	}
	if len(callers) == 0 {
		t.Error("FindCallers returned no callers; expected Charge")
	}
}

// TestNewConstantClient_GitHubNotConfigured verifies that ModeGitHub returns a
// clear "not configured" error.
func TestNewConstantClient_GitHubNotConfigured(t *testing.T) {
	_, err := client.NewConstantClient(client.ModeGitHub, "")
	if err == nil {
		t.Fatal("expected error for ModeGitHub, got nil")
	}
}

// ─── helpers ─────────────────────────────────────────────────────────────────

// symbolNames extracts the Name field from a slice of SymbolNodes.
func symbolNames(nodes []client.SymbolNode) []string {
	names := make([]string, len(nodes))
	for i, n := range nodes {
		names[i] = n.Name
	}
	return names
}
