package store

import (
	"context"
	"sort"
	"testing"
	"time"

	"github.com/geoffmilleraz/sextant/internal/core"
)

// storeContract runs the full Store interface contract against the supplied
// Store. It is called from both duckdb_test.go and memory_test.go so both
// implementations are verified against the same expectations.
func storeContract(t *testing.T, s Store) {
	t.Helper()
	ctx := context.Background()

	// ------------------------------------------------------------------ //
	// Round-trip: PutObservations + QueryObservations (service scope)
	// ------------------------------------------------------------------ //
	t.Run("put_and_query_all", func(t *testing.T) {
		obs := []core.Observation{
			{
				ServiceID:    "svc-a",
				ConcernID:    "auth",
				SourceRef:    "internal/auth/token.go",
				SourceKind:   core.KindGoFunc,
				EvidenceTier: core.TierVerified,
				Symbol:       "ValidateToken",
				Claim:        "func ValidateToken(ctx context.Context, tok string) error",
				ContentHash:  "abc123",
				Calls:        []string{"db.Query"},
				IngestedAt:   time.Now().UTC().Truncate(time.Second),
			},
			{
				ServiceID:    "svc-a",
				ConcernID:    "",
				SourceRef:    "internal/auth/token.go",
				SourceKind:   core.KindGoFunc,
				EvidenceTier: core.TierVerified,
				Symbol:       "IssueToken",
				Claim:        "func IssueToken(ctx context.Context, sub string) (string, error)",
				ContentHash:  "abc123",
				Calls:        nil,
				IngestedAt:   time.Now().UTC().Truncate(time.Second),
			},
		}
		if err := s.PutObservations(ctx, obs); err != nil {
			t.Fatalf("PutObservations: %v", err)
		}
		got, err := s.QueryObservations(ctx, "svc-a", "")
		if err != nil {
			t.Fatalf("QueryObservations: %v", err)
		}
		if len(got) < 2 {
			t.Fatalf("expected at least 2 observations, got %d", len(got))
		}
		symbols := symbolSet(got)
		if !symbols["ValidateToken"] || !symbols["IssueToken"] {
			t.Fatalf("missing symbols; got %v", symbols)
		}
	})

	// ------------------------------------------------------------------ //
	// Concern-scoped filtering
	// ------------------------------------------------------------------ //
	t.Run("concern_scoping", func(t *testing.T) {
		// Put observations tagged with two different concerns.
		obs := []core.Observation{
			makeObs("svc-scope", "billing", "billing/invoice.go", "ProcessInvoice"),
			makeObs("svc-scope", "auth", "auth/login.go", "LoginUser"),
			makeObs("svc-scope", "", "shared/util.go", "FormatDate"),
		}
		if err := s.PutObservations(ctx, obs); err != nil {
			t.Fatalf("PutObservations: %v", err)
		}

		billingObs, err := s.QueryObservations(ctx, "svc-scope", "billing")
		if err != nil {
			t.Fatalf("QueryObservations billing: %v", err)
		}
		for _, o := range billingObs {
			if o.ConcernID != "billing" {
				t.Errorf("expected concern=billing, got %q", o.ConcernID)
			}
		}
		found := symbolSet(billingObs)
		if !found["ProcessInvoice"] {
			t.Errorf("ProcessInvoice not in billing results")
		}
		if found["LoginUser"] {
			t.Errorf("LoginUser should not appear in billing results")
		}

		allObs, err := s.QueryObservations(ctx, "svc-scope", "")
		if err != nil {
			t.Fatalf("QueryObservations all: %v", err)
		}
		allSymbols := symbolSet(allObs)
		for _, sym := range []string{"ProcessInvoice", "LoginUser", "FormatDate"} {
			if !allSymbols[sym] {
				t.Errorf("symbol %q missing from unscoped query", sym)
			}
		}
	})

	// ------------------------------------------------------------------ //
	// Fingerprint set / get / update
	// ------------------------------------------------------------------ //
	t.Run("fingerprint_set_get", func(t *testing.T) {
		svc, ref := "svc-fp", "pkg/foo/bar.go"

		// Not found before setting.
		_, found, err := s.Fingerprint(ctx, svc, ref)
		if err != nil {
			t.Fatalf("Fingerprint: %v", err)
		}
		if found {
			t.Fatal("expected not found before SetFingerprint")
		}

		if err := s.SetFingerprint(ctx, svc, ref, "hash-v1"); err != nil {
			t.Fatalf("SetFingerprint: %v", err)
		}
		hash, found, err := s.Fingerprint(ctx, svc, ref)
		if err != nil {
			t.Fatalf("Fingerprint after set: %v", err)
		}
		if !found {
			t.Fatal("expected found after SetFingerprint")
		}
		if hash != "hash-v1" {
			t.Fatalf("hash mismatch: got %q want %q", hash, "hash-v1")
		}

		// Update idempotency.
		if err := s.SetFingerprint(ctx, svc, ref, "hash-v2"); err != nil {
			t.Fatalf("SetFingerprint update: %v", err)
		}
		hash2, _, _ := s.Fingerprint(ctx, svc, ref)
		if hash2 != "hash-v2" {
			t.Fatalf("update: got %q want hash-v2", hash2)
		}
	})

	// ------------------------------------------------------------------ //
	// PurgeSource removes only the target file's observations
	// ------------------------------------------------------------------ //
	t.Run("purge_source", func(t *testing.T) {
		obs := []core.Observation{
			makeObs("svc-purge", "x", "pkg/a/a.go", "FuncA"),
			makeObs("svc-purge", "x", "pkg/b/b.go", "FuncB"),
			makeObs("svc-purge", "x", "pkg/b/b.go", "FuncB2"),
		}
		if err := s.PutObservations(ctx, obs); err != nil {
			t.Fatalf("PutObservations: %v", err)
		}

		if err := s.PurgeSource(ctx, "svc-purge", "pkg/b/b.go"); err != nil {
			t.Fatalf("PurgeSource: %v", err)
		}

		remaining, err := s.QueryObservations(ctx, "svc-purge", "")
		if err != nil {
			t.Fatalf("QueryObservations after purge: %v", err)
		}
		syms := symbolSet(remaining)
		if !syms["FuncA"] {
			t.Error("FuncA should remain after purging pkg/b/b.go")
		}
		if syms["FuncB"] || syms["FuncB2"] {
			t.Error("FuncB / FuncB2 should be gone after purge")
		}
	})

	// ------------------------------------------------------------------ //
	// Complexity round-trips through Put + Query
	// ------------------------------------------------------------------ //
	t.Run("complexity_roundtrip", func(t *testing.T) {
		obs := []core.Observation{
			{
				ServiceID:    "svc-cx",
				ConcernID:    "perf",
				SourceRef:    "pkg/hot/hot.go",
				SourceKind:   core.KindGoFunc,
				EvidenceTier: core.TierVerified,
				Symbol:       "HotPath",
				Claim:        "func HotPath(n int) int",
				ContentHash:  "cx123",
				Calls:        nil,
				Complexity:   7,
				IngestedAt:   time.Now().UTC().Truncate(time.Second),
			},
			{
				ServiceID:    "svc-cx",
				ConcernID:    "perf",
				SourceRef:    "pkg/hot/hot.go",
				SourceKind:   core.KindGoFunc,
				EvidenceTier: core.TierVerified,
				Symbol:       "SimpleOp",
				Claim:        "func SimpleOp()",
				ContentHash:  "cx123",
				Calls:        nil,
				Complexity:   1,
				IngestedAt:   time.Now().UTC().Truncate(time.Second),
			},
		}
		if err := s.PutObservations(ctx, obs); err != nil {
			t.Fatalf("PutObservations: %v", err)
		}
		got, err := s.QueryObservations(ctx, "svc-cx", "perf")
		if err != nil {
			t.Fatalf("QueryObservations: %v", err)
		}
		bySymbol := make(map[string]core.Observation, len(got))
		for _, o := range got {
			bySymbol[o.Symbol] = o
		}
		if o, ok := bySymbol["HotPath"]; !ok {
			t.Error("HotPath missing from results")
		} else if o.Complexity != 7 {
			t.Errorf("HotPath.Complexity = %d, want 7", o.Complexity)
		}
		if o, ok := bySymbol["SimpleOp"]; !ok {
			t.Error("SimpleOp missing from results")
		} else if o.Complexity != 1 {
			t.Errorf("SimpleOp.Complexity = %d, want 1", o.Complexity)
		}
	})

	// ------------------------------------------------------------------ //
	// Nil / empty Calls slice round-trips safely
	// ------------------------------------------------------------------ //
	t.Run("nil_calls", func(t *testing.T) {
		obs := []core.Observation{
			{
				ServiceID:    "svc-calls",
				ConcernID:    "test",
				SourceRef:    "x.go",
				SourceKind:   core.KindGoFunc,
				EvidenceTier: core.TierVerified,
				Symbol:       "NoCalls",
				Claim:        "func NoCalls()",
				ContentHash:  "xyz",
				Calls:        nil,
				IngestedAt:   time.Now().UTC(),
			},
		}
		if err := s.PutObservations(ctx, obs); err != nil {
			t.Fatalf("PutObservations nil calls: %v", err)
		}
		got, err := s.QueryObservations(ctx, "svc-calls", "test")
		if err != nil {
			t.Fatalf("QueryObservations: %v", err)
		}
		if len(got) == 0 {
			t.Fatal("expected at least one result")
		}
		// Calls should be non-nil (empty slice) or nil — not a crash.
		_ = got[0].Calls
	})

	// ------------------------------------------------------------------ //
	// KnowledgeCard round-trip + per-service scoping
	// ------------------------------------------------------------------ //
	t.Run("knowledge_cards_roundtrip", func(t *testing.T) {
		cards := []KnowledgeCard{
			{
				ServiceID: "svc-kc",
				Symbol:    "AuthMiddleware",
				File:      "internal/auth/middleware.go",
				Summary:   "Validates JWT tokens on every request.",
				Tags:      []string{"auth", "middleware", "jwt"},
			},
			{
				ServiceID: "svc-kc",
				Symbol:    "ParseConfig",
				File:      "internal/config/config.go",
				Summary:   "Parses YAML configuration into a Config struct.",
				Tags:      []string{"config", "yaml"},
			},
		}
		if err := s.PutKnowledgeCards(ctx, cards); err != nil {
			t.Fatalf("PutKnowledgeCards: %v", err)
		}

		got, err := s.KnowledgeCards(ctx, "svc-kc")
		if err != nil {
			t.Fatalf("KnowledgeCards: %v", err)
		}
		if len(got) != 2 {
			t.Fatalf("expected 2 knowledge cards, got %d", len(got))
		}
		// Verify deterministic order (by symbol: AuthMiddleware < ParseConfig).
		if got[0].Symbol != "AuthMiddleware" || got[1].Symbol != "ParseConfig" {
			t.Errorf("unexpected order: %q, %q", got[0].Symbol, got[1].Symbol)
		}
		// Verify field round-trip.
		if got[0].File != "internal/auth/middleware.go" {
			t.Errorf("File mismatch: got %q", got[0].File)
		}
		if got[0].Summary != "Validates JWT tokens on every request." {
			t.Errorf("Summary mismatch: got %q", got[0].Summary)
		}
		if len(got[0].Tags) != 3 || got[0].Tags[0] != "auth" {
			t.Errorf("Tags mismatch: got %v", got[0].Tags)
		}
	})

	t.Run("knowledge_cards_upsert", func(t *testing.T) {
		initial := []KnowledgeCard{
			{ServiceID: "svc-kc-up", Symbol: "Foo", File: "foo.go", Summary: "old", Tags: []string{"x"}},
		}
		if err := s.PutKnowledgeCards(ctx, initial); err != nil {
			t.Fatalf("PutKnowledgeCards initial: %v", err)
		}
		updated := []KnowledgeCard{
			{ServiceID: "svc-kc-up", Symbol: "Foo", File: "foo.go", Summary: "new", Tags: []string{"y", "z"}},
		}
		if err := s.PutKnowledgeCards(ctx, updated); err != nil {
			t.Fatalf("PutKnowledgeCards update: %v", err)
		}
		got, err := s.KnowledgeCards(ctx, "svc-kc-up")
		if err != nil {
			t.Fatalf("KnowledgeCards after upsert: %v", err)
		}
		if len(got) != 1 {
			t.Fatalf("expected 1 card after upsert, got %d", len(got))
		}
		if got[0].Summary != "new" {
			t.Errorf("upsert did not update Summary: got %q", got[0].Summary)
		}
		if len(got[0].Tags) != 2 {
			t.Errorf("upsert did not update Tags: got %v", got[0].Tags)
		}
	})

	t.Run("knowledge_cards_service_scoping", func(t *testing.T) {
		cards := []KnowledgeCard{
			{ServiceID: "svc-kc-a", Symbol: "Alpha", File: "a.go", Summary: "alpha summary", Tags: []string{"alpha"}},
			{ServiceID: "svc-kc-b", Symbol: "Beta", File: "b.go", Summary: "beta summary", Tags: []string{"beta"}},
		}
		if err := s.PutKnowledgeCards(ctx, cards); err != nil {
			t.Fatalf("PutKnowledgeCards scoping: %v", err)
		}
		gotA, err := s.KnowledgeCards(ctx, "svc-kc-a")
		if err != nil {
			t.Fatalf("KnowledgeCards svc-kc-a: %v", err)
		}
		for _, c := range gotA {
			if c.ServiceID != "svc-kc-a" {
				t.Errorf("service scoping leak: got serviceID %q in svc-kc-a results", c.ServiceID)
			}
		}
		syms := make(map[string]bool)
		for _, c := range gotA {
			syms[c.Symbol] = true
		}
		if !syms["Alpha"] {
			t.Error("Alpha missing from svc-kc-a results")
		}
		if syms["Beta"] {
			t.Error("Beta should not appear in svc-kc-a results")
		}
	})

	// ------------------------------------------------------------------ //
	// DocSummary round-trip + per-service scoping
	// ------------------------------------------------------------------ //
	t.Run("doc_summaries_roundtrip", func(t *testing.T) {
		docs := []DocSummary{
			{
				ServiceID: "svc-ds",
				SourceRef: "docs/api.md",
				Title:     "API Reference",
				Summary:   "Complete reference for the public REST API.",
			},
			{
				ServiceID: "svc-ds",
				SourceRef: "docs/auth.md",
				Title:     "Authentication Guide",
				Summary:   "Explains OAuth2 and API key authentication flows.",
			},
		}
		if err := s.PutDocSummaries(ctx, docs); err != nil {
			t.Fatalf("PutDocSummaries: %v", err)
		}

		got, err := s.DocSummaries(ctx, "svc-ds")
		if err != nil {
			t.Fatalf("DocSummaries: %v", err)
		}
		if len(got) != 2 {
			t.Fatalf("expected 2 doc summaries, got %d", len(got))
		}
		// Verify deterministic order (by source_ref: docs/api.md < docs/auth.md).
		if got[0].SourceRef != "docs/api.md" || got[1].SourceRef != "docs/auth.md" {
			t.Errorf("unexpected order: %q, %q", got[0].SourceRef, got[1].SourceRef)
		}
		if got[0].Title != "API Reference" {
			t.Errorf("Title mismatch: got %q", got[0].Title)
		}
		if got[1].Summary != "Explains OAuth2 and API key authentication flows." {
			t.Errorf("Summary mismatch: got %q", got[1].Summary)
		}
	})

	t.Run("doc_summaries_upsert", func(t *testing.T) {
		initial := []DocSummary{
			{ServiceID: "svc-ds-up", SourceRef: "README.md", Title: "Old Title", Summary: "old summary"},
		}
		if err := s.PutDocSummaries(ctx, initial); err != nil {
			t.Fatalf("PutDocSummaries initial: %v", err)
		}
		updated := []DocSummary{
			{ServiceID: "svc-ds-up", SourceRef: "README.md", Title: "New Title", Summary: "new summary"},
		}
		if err := s.PutDocSummaries(ctx, updated); err != nil {
			t.Fatalf("PutDocSummaries update: %v", err)
		}
		got, err := s.DocSummaries(ctx, "svc-ds-up")
		if err != nil {
			t.Fatalf("DocSummaries after upsert: %v", err)
		}
		if len(got) != 1 {
			t.Fatalf("expected 1 doc summary after upsert, got %d", len(got))
		}
		if got[0].Title != "New Title" {
			t.Errorf("upsert did not update Title: got %q", got[0].Title)
		}
		if got[0].Summary != "new summary" {
			t.Errorf("upsert did not update Summary: got %q", got[0].Summary)
		}
	})

	t.Run("doc_summaries_service_scoping", func(t *testing.T) {
		docs := []DocSummary{
			{ServiceID: "svc-ds-a", SourceRef: "a.md", Title: "A Doc", Summary: "a summary"},
			{ServiceID: "svc-ds-b", SourceRef: "b.md", Title: "B Doc", Summary: "b summary"},
		}
		if err := s.PutDocSummaries(ctx, docs); err != nil {
			t.Fatalf("PutDocSummaries scoping: %v", err)
		}
		gotA, err := s.DocSummaries(ctx, "svc-ds-a")
		if err != nil {
			t.Fatalf("DocSummaries svc-ds-a: %v", err)
		}
		for _, d := range gotA {
			if d.ServiceID != "svc-ds-a" {
				t.Errorf("service scoping leak: got serviceID %q in svc-ds-a results", d.ServiceID)
			}
		}
		refs := make(map[string]bool)
		for _, d := range gotA {
			refs[d.SourceRef] = true
		}
		if !refs["a.md"] {
			t.Error("a.md missing from svc-ds-a results")
		}
		if refs["b.md"] {
			t.Error("b.md should not appear in svc-ds-a results")
		}
	})

	// ------------------------------------------------------------------ //
	// ListServiceIDs returns distinct, sorted service IDs
	// ------------------------------------------------------------------ //
	t.Run("list_service_ids", func(t *testing.T) {
		obs := []core.Observation{
			makeObs("svc-list-a", "c1", "a.go", "FuncA1"),
			makeObs("svc-list-a", "c1", "a.go", "FuncA2"),
			makeObs("svc-list-b", "c1", "b.go", "FuncB1"),
		}
		if err := s.PutObservations(ctx, obs); err != nil {
			t.Fatalf("PutObservations: %v", err)
		}
		ids, err := s.ListServiceIDs(ctx)
		if err != nil {
			t.Fatalf("ListServiceIDs: %v", err)
		}
		// The store may have other service IDs from prior sub-tests; just check
		// that svc-list-a and svc-list-b are both present and in sorted order.
		idSet := make(map[string]bool, len(ids))
		for _, id := range ids {
			idSet[id] = true
		}
		if !idSet["svc-list-a"] {
			t.Error("svc-list-a missing from ListServiceIDs")
		}
		if !idSet["svc-list-b"] {
			t.Error("svc-list-b missing from ListServiceIDs")
		}
		// Verify sorted order within the returned IDs.
		for i := 1; i < len(ids); i++ {
			if ids[i] < ids[i-1] {
				t.Errorf("ListServiceIDs not sorted at index %d: %q > %q", i, ids[i-1], ids[i])
			}
		}
	})
}

// -------------------------------------------------------------------------
// Helpers
// -------------------------------------------------------------------------

func makeObs(serviceID, concernID, sourceRef, symbol string) core.Observation {
	return core.Observation{
		ServiceID:    serviceID,
		ConcernID:    concernID,
		SourceRef:    sourceRef,
		SourceKind:   core.KindGoFunc,
		EvidenceTier: core.TierVerified,
		Symbol:       symbol,
		Claim:        "func " + symbol + "()",
		ContentHash:  "hash-" + symbol,
		Calls:        []string{},
		IngestedAt:   time.Now().UTC(),
	}
}

func symbolSet(obs []core.Observation) map[string]bool {
	m := make(map[string]bool, len(obs))
	for _, o := range obs {
		m[o.Symbol] = true
	}
	return m
}

// sortedSymbols returns sorted symbol names for deterministic test output.
func sortedSymbols(obs []core.Observation) []string {
	ss := make([]string, 0, len(obs))
	for _, o := range obs {
		ss = append(ss, o.Symbol)
	}
	sort.Strings(ss)
	return ss
}
