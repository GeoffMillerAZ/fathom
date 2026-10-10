package fusion_test

// enrich_test.go exercises the optional LLM enrichment step in the fusion
// pipeline. It uses llmclient.MockClient for deterministic, network-free runs.

import (
	"context"
	"testing"

	"github.com/geoffmilleraz/agentic-kit/foundations/llmclient"
	"github.com/geoffmilleraz/fathom/internal/connector"
	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/fusion"
	"github.com/geoffmilleraz/fathom/internal/llm"
	"github.com/geoffmilleraz/fathom/internal/llmsec"
	"github.com/geoffmilleraz/fathom/internal/store"
)

// mockEnrichResponse is a canned LLM response that parseResponse handles cleanly.
const mockEnrichResponse = "Summary: A mock summary.\nTags: mock, test"

// newMockEnricher returns an Enricher backed by a MockClient that always
// returns mockEnrichResponse, with enough entries for any number of calls.
func newMockEnricher(entries int) *llm.Enricher {
	mockEntries := make([]llmclient.MockEntry, entries)
	for i := range mockEntries {
		mockEntries[i] = llmclient.MockEntryOK(mockEnrichResponse)
	}
	client := llmclient.NewMockClient(mockEntries...)
	redactor := llmsec.NewRedactor(llmsec.DefaultPosture())
	return llm.NewEnricher(client, redactor)
}

// TestEnrich_Disabled verifies that when Config.Enrich is false (the default)
// no knowledge cards are generated even if an enricher is wired.
func TestEnrich_Disabled(t *testing.T) {
	conn := connector.NewFauxConnector()
	st := store.NewMemoryStore()
	cfg := defaultCfg() // Enrich = false by default

	p := fusion.New(conn, st, cfg)
	p.WithEnricher(newMockEnricher(100))

	ctx := context.Background()
	if _, err := p.Ingest(ctx, paymentSvc); err != nil {
		t.Fatalf("Ingest: %v", err)
	}

	cards, err := st.KnowledgeCards(ctx, paymentSvc.ID)
	if err != nil {
		t.Fatalf("KnowledgeCards: %v", err)
	}
	if len(cards) != 0 {
		t.Errorf("expected 0 knowledge cards when Enrich=false, got %d", len(cards))
	}
}

// TestEnrich_NoEnricherWired verifies that setting Config.Enrich=true without
// wiring an enricher does not panic or error.
func TestEnrich_NoEnricherWired(t *testing.T) {
	conn := connector.NewFauxConnector()
	st := store.NewMemoryStore()
	cfg := defaultCfg()
	cfg.Enrich = true // enrichment enabled but no enricher wired

	p := fusion.New(conn, st, cfg)
	// Deliberately do NOT call p.WithEnricher(...)

	ctx := context.Background()
	if _, err := p.Ingest(ctx, paymentSvc); err != nil {
		t.Fatalf("Ingest: %v", err)
	}

	// No cards — enricher is nil so enrichment is silently skipped.
	cards, err := st.KnowledgeCards(ctx, paymentSvc.ID)
	if err != nil {
		t.Fatalf("KnowledgeCards: %v", err)
	}
	if len(cards) != 0 {
		t.Errorf("expected 0 cards (no enricher wired), got %d", len(cards))
	}
}

// TestEnrich_KnowledgeCardsGenerated verifies that enabling enrichment causes
// KnowledgeCards to be stored for extracted code symbols.
func TestEnrich_KnowledgeCardsGenerated(t *testing.T) {
	conn := connector.NewFauxConnector()
	st := store.NewMemoryStore()
	cfg := defaultCfg()
	cfg.Enrich = true

	p := fusion.New(conn, st, cfg)
	p.WithEnricher(newMockEnricher(100))

	ctx := context.Background()
	report, err := p.Ingest(ctx, paymentSvc)
	if err != nil {
		t.Fatalf("Ingest: %v", err)
	}
	if report.Symbols == 0 {
		t.Fatal("Ingest: no symbols extracted (fixture issue?)")
	}

	cards, err := st.KnowledgeCards(ctx, paymentSvc.ID)
	if err != nil {
		t.Fatalf("KnowledgeCards: %v", err)
	}
	if len(cards) == 0 {
		t.Fatal("expected knowledge cards to be stored, got 0")
	}

	// Every card must have the correct service ID and a non-empty summary.
	for _, c := range cards {
		if c.ServiceID != paymentSvc.ID {
			t.Errorf("card.ServiceID=%q, want %q", c.ServiceID, paymentSvc.ID)
		}
		if c.Symbol == "" {
			t.Error("card.Symbol is empty")
		}
		if c.Summary == "" {
			t.Errorf("card.Summary is empty for symbol %q", c.Symbol)
		}
	}
}

// TestEnrich_DocSummaryGenerated verifies that enriching a service with a
// Markdown file produces a DocSummary.
func TestEnrich_DocSummaryGenerated(t *testing.T) {
	conn := connector.NewFauxConnector()
	st := store.NewMemoryStore()
	cfg := defaultCfg()
	cfg.Enrich = true

	p := fusion.New(conn, st, cfg)
	// polyglot-service has README.md (3 headings) and several code files.
	// Provide enough mock entries for all symbols + 1 doc summary call.
	p.WithEnricher(newMockEnricher(200))

	ctx := context.Background()
	if _, err := p.Ingest(ctx, polyglotSvc); err != nil {
		t.Fatalf("Ingest: %v", err)
	}

	summaries, err := st.DocSummaries(ctx, polyglotSvc.ID)
	if err != nil {
		t.Fatalf("DocSummaries: %v", err)
	}
	if len(summaries) == 0 {
		t.Fatal("expected at least one DocSummary for README.md, got 0")
	}

	// At least one summary must reference the README.md source ref.
	found := false
	for _, s := range summaries {
		if s.ServiceID != polyglotSvc.ID {
			t.Errorf("summary.ServiceID=%q, want %q", s.ServiceID, polyglotSvc.ID)
		}
		if s.SourceRef != "" && s.Summary != "" {
			found = true
		}
	}
	if !found {
		t.Error("no DocSummary with non-empty SourceRef and Summary found")
	}
}

// TestEnrich_HashGateSkipsUnchanged verifies that running Ingest twice only
// enriches once — the second run's hash-gate causes all files to be skipped,
// so the enricher is not called again.
func TestEnrich_HashGateSkipsUnchanged(t *testing.T) {
	conn := connector.NewFauxConnector()
	st := store.NewMemoryStore()
	cfg := defaultCfg()
	cfg.Enrich = true

	// Track how many times the mock enricher is called by using a fresh mock
	// with a call-count we can check afterwards.
	mockEntries := make([]llmclient.MockEntry, 200)
	for i := range mockEntries {
		mockEntries[i] = llmclient.MockEntryOK(mockEnrichResponse)
	}
	client := llmclient.NewMockClient(mockEntries...)
	redactor := llmsec.NewRedactor(llmsec.DefaultPosture())
	enricher := llm.NewEnricher(client, redactor)

	p := fusion.New(conn, st, cfg)
	p.WithEnricher(enricher)

	ctx := context.Background()

	// First ingest: files are new → enrichment runs.
	if _, err := p.Ingest(ctx, paymentSvc); err != nil {
		t.Fatalf("first Ingest: %v", err)
	}
	callsAfterFirst := client.CallCount()
	if callsAfterFirst == 0 {
		t.Error("expected enricher to be called on first ingest")
	}

	// Second ingest: fingerprints match → all files skipped → no enrichment.
	if _, err := p.Ingest(ctx, paymentSvc); err != nil {
		t.Fatalf("second Ingest: %v", err)
	}
	callsAfterSecond := client.CallCount()
	if callsAfterSecond != callsAfterFirst {
		t.Errorf("enricher was called %d times on second ingest (unchanged files), expected 0 new calls",
			callsAfterSecond-callsAfterFirst)
	}
}

// TestEnrich_Deterministic verifies that two independent enriched ingests of the
// same service produce identical knowledge cards.
func TestEnrich_Deterministic(t *testing.T) {
	ctx := context.Background()

	run := func() []store.KnowledgeCard {
		conn := connector.NewFauxConnector()
		st := store.NewMemoryStore()
		cfg := defaultCfg()
		cfg.Enrich = true
		p := fusion.New(conn, st, cfg)
		p.WithEnricher(newMockEnricher(100))
		if _, err := p.Ingest(ctx, paymentSvc); err != nil {
			t.Fatalf("Ingest: %v", err)
		}
		cards, err := st.KnowledgeCards(ctx, paymentSvc.ID)
		if err != nil {
			t.Fatalf("KnowledgeCards: %v", err)
		}
		return cards
	}

	cards1 := run()
	cards2 := run()

	if len(cards1) != len(cards2) {
		t.Fatalf("non-deterministic: run1=%d cards, run2=%d cards", len(cards1), len(cards2))
	}
	for i := range cards1 {
		if cards1[i].Symbol != cards2[i].Symbol {
			t.Errorf("card[%d].Symbol: run1=%q run2=%q", i, cards1[i].Symbol, cards2[i].Symbol)
		}
		if cards1[i].Summary != cards2[i].Summary {
			t.Errorf("card[%d].Summary: run1=%q run2=%q", i, cards1[i].Summary, cards2[i].Summary)
		}
	}
}

// TestEnrich_ExistingTestsUnaffected verifies that the standard (non-enriching)
// pipeline behaves identically to the pre-enrichment implementation.
func TestEnrich_ExistingTestsUnaffected(t *testing.T) {
	conn := connector.NewFauxConnector()
	st := store.NewMemoryStore()
	p := fusion.New(conn, st, defaultCfg()) // Enrich=false, no enricher

	ctx := context.Background()
	report, err := p.Ingest(ctx, paymentSvc)
	if err != nil {
		t.Fatalf("Ingest: %v", err)
	}

	const wantSymbols = 14 // payment.go 5 + their 7 members (K17) + errors.go 2 unexported vars (LB-F60)
	if report.Symbols != wantSymbols {
		t.Errorf("Symbols=%d, want %d", report.Symbols, wantSymbols)
	}

	obs, err := st.QueryObservations(ctx, paymentSvc.ID, "")
	if err != nil {
		t.Fatalf("QueryObservations: %v", err)
	}
	if len(obs) != wantSymbols {
		t.Errorf("stored %d observations, want %d", len(obs), wantSymbols)
	}

	for _, o := range obs {
		if o.EvidenceTier != core.TierVerified {
			t.Errorf("obs %q: EvidenceTier=%q, want verified", o.Symbol, o.EvidenceTier)
		}
	}
}
