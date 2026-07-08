package store

import (
	"context"
	"sort"
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/internal/core"
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
	// Package round-trips (regression: it used to be silently dropped, which
	// broke SymbolKey reconstruction to "svc::.Name" and cross-repo matching)
	// ------------------------------------------------------------------ //
	t.Run("package_round_trips", func(t *testing.T) {
		obs := []core.Observation{
			{
				ServiceID:    "svc-pkg",
				SourceRef:    "internal/auth/token.go",
				SourceKind:   core.KindGoFunc,
				EvidenceTier: core.TierVerified,
				Symbol:       "ValidateToken",
				Package:      "auth",
				Claim:        "func ValidateToken()",
				Calls:        []string{"db.Query"},
				IngestedAt:   time.Now().UTC().Truncate(time.Second),
			},
		}
		if err := s.PutObservations(ctx, obs); err != nil {
			t.Fatalf("PutObservations: %v", err)
		}
		got, err := s.QueryObservations(ctx, "svc-pkg", "")
		if err != nil {
			t.Fatalf("QueryObservations: %v", err)
		}
		var found bool
		for _, o := range got {
			if o.Symbol == "ValidateToken" {
				found = true
				if o.Package != "auth" {
					t.Fatalf("Package not round-tripped: got %q want %q", o.Package, "auth")
				}
			}
		}
		if !found {
			t.Fatalf("ValidateToken observation not returned")
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
	// PurgeService + CountAbsoluteSourceRefs (ADR-017 self-heal support)
	// ------------------------------------------------------------------ //
	t.Run("purge_service_and_count_absolute_refs", func(t *testing.T) {
		obs := []core.Observation{
			makeObs("svc-heal", "x", "/abs/machine/pkg/a.go", "OldA"),
			makeObs("svc-heal", "x", "pkg/b.go", "NewB"),
			makeObs("svc-other", "x", "/abs/machine/pkg/z.go", "OtherZ"),
		}
		if err := s.PutObservations(ctx, obs); err != nil {
			t.Fatalf("PutObservations: %v", err)
		}
		for _, fp := range []struct{ svc, ref string }{
			{"svc-heal", "/abs/machine/pkg/a.go"},
			{"svc-heal", "pkg/b.go"},
			{"svc-other", "/abs/machine/pkg/z.go"},
		} {
			if err := s.SetFingerprint(ctx, fp.svc, fp.ref, "h"); err != nil {
				t.Fatalf("SetFingerprint: %v", err)
			}
		}

		n, err := s.CountAbsoluteSourceRefs(ctx, "svc-heal")
		if err != nil {
			t.Fatalf("CountAbsoluteSourceRefs: %v", err)
		}
		if n != 1 {
			t.Errorf("CountAbsoluteSourceRefs(svc-heal) = %d, want 1", n)
		}

		if err := s.PurgeService(ctx, "svc-heal"); err != nil {
			t.Fatalf("PurgeService: %v", err)
		}
		remaining, err := s.QueryObservations(ctx, "svc-heal", "")
		if err != nil {
			t.Fatalf("QueryObservations after purge: %v", err)
		}
		if len(remaining) != 0 {
			t.Errorf("svc-heal observations after PurgeService = %d, want 0", len(remaining))
		}
		if _, found, _ := s.Fingerprint(ctx, "svc-heal", "pkg/b.go"); found {
			t.Error("svc-heal fingerprint survived PurgeService")
		}
		n, err = s.CountAbsoluteSourceRefs(ctx, "svc-heal")
		if err != nil || n != 0 {
			t.Errorf("CountAbsoluteSourceRefs after purge = %d, %v; want 0, nil", n, err)
		}

		// Other services are untouched.
		other, err := s.QueryObservations(ctx, "svc-other", "")
		if err != nil || len(other) != 1 {
			t.Errorf("svc-other observations = %d, %v; want 1, nil", len(other), err)
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
	// Line field round-trips through Put + Query
	// ------------------------------------------------------------------ //
	t.Run("line_roundtrip", func(t *testing.T) {
		obs := []core.Observation{
			{
				ServiceID:    "svc-line",
				ConcernID:    "nav",
				SourceRef:    "pkg/nav/nav.go",
				SourceKind:   core.KindGoFunc,
				EvidenceTier: core.TierVerified,
				Symbol:       "Navigate",
				Claim:        "func Navigate(ctx context.Context) error",
				ContentHash:  "ln123",
				Calls:        nil,
				Line:         42,
				IngestedAt:   time.Now().UTC().Truncate(time.Second),
			},
			{
				ServiceID:    "svc-line",
				ConcernID:    "nav",
				SourceRef:    "pkg/nav/nav.go",
				SourceKind:   core.KindGoFunc,
				EvidenceTier: core.TierVerified,
				Symbol:       "NoLine",
				Claim:        "func NoLine()",
				ContentHash:  "ln123",
				Calls:        nil,
				Line:         0, // unknown — must round-trip as 0
				IngestedAt:   time.Now().UTC().Truncate(time.Second),
			},
		}
		if err := s.PutObservations(ctx, obs); err != nil {
			t.Fatalf("PutObservations: %v", err)
		}
		got, err := s.QueryObservations(ctx, "svc-line", "nav")
		if err != nil {
			t.Fatalf("QueryObservations: %v", err)
		}
		bySymbol := make(map[string]core.Observation, len(got))
		for _, o := range got {
			bySymbol[o.Symbol] = o
		}
		if o, ok := bySymbol["Navigate"]; !ok {
			t.Error("Navigate missing from results")
		} else if o.Line != 42 {
			t.Errorf("Navigate.Line = %d, want 42", o.Line)
		}
		if o, ok := bySymbol["NoLine"]; !ok {
			t.Error("NoLine missing from results")
		} else if o.Line != 0 {
			t.Errorf("NoLine.Line = %d, want 0", o.Line)
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

	// ------------------------------------------------------------------ //
	// ServiceMeta round-trip + upsert
	// ------------------------------------------------------------------ //
	t.Run("service_meta_roundtrip", func(t *testing.T) {
		meta := ServiceMeta{
			ServiceID:     "svc-meta",
			Path:          "/code/svc",
			IndexedCommit: "abc1234",
			IndexedAt:     1234567890000000000,
		}
		if err := s.SetServiceMeta(ctx, meta); err != nil {
			t.Fatalf("SetServiceMeta: %v", err)
		}
		got, found, err := s.GetServiceMeta(ctx, "svc-meta")
		if err != nil {
			t.Fatalf("GetServiceMeta: %v", err)
		}
		if !found {
			t.Fatal("expected found after SetServiceMeta")
		}
		if got.ServiceID != meta.ServiceID {
			t.Errorf("ServiceID = %q, want %q", got.ServiceID, meta.ServiceID)
		}
		if got.Path != meta.Path {
			t.Errorf("Path = %q, want %q", got.Path, meta.Path)
		}
		if got.IndexedCommit != meta.IndexedCommit {
			t.Errorf("IndexedCommit = %q, want %q", got.IndexedCommit, meta.IndexedCommit)
		}
		if got.IndexedAt != meta.IndexedAt {
			t.Errorf("IndexedAt = %d, want %d", got.IndexedAt, meta.IndexedAt)
		}
	})

	t.Run("service_meta_not_found", func(t *testing.T) {
		_, found, err := s.GetServiceMeta(ctx, "svc-does-not-exist-xyz")
		if err != nil {
			t.Fatalf("GetServiceMeta: %v", err)
		}
		if found {
			t.Error("expected not found for unknown service")
		}
	})

	t.Run("service_meta_upsert", func(t *testing.T) {
		initial := ServiceMeta{
			ServiceID:     "svc-meta-up",
			Path:          "/old/path",
			IndexedCommit: "aaa",
			IndexedAt:     100,
		}
		if err := s.SetServiceMeta(ctx, initial); err != nil {
			t.Fatalf("SetServiceMeta initial: %v", err)
		}
		updated := ServiceMeta{
			ServiceID:     "svc-meta-up",
			Path:          "/new/path",
			IndexedCommit: "bbb",
			IndexedAt:     200,
		}
		if err := s.SetServiceMeta(ctx, updated); err != nil {
			t.Fatalf("SetServiceMeta update: %v", err)
		}
		got, found, err := s.GetServiceMeta(ctx, "svc-meta-up")
		if err != nil {
			t.Fatalf("GetServiceMeta after upsert: %v", err)
		}
		if !found {
			t.Fatal("expected found after upsert")
		}
		if got.IndexedCommit != "bbb" {
			t.Errorf("IndexedCommit = %q, want bbb", got.IndexedCommit)
		}
		if got.Path != "/new/path" {
			t.Errorf("Path = %q, want /new/path", got.Path)
		}
	})

	t.Run("service_meta_empty_commit", func(t *testing.T) {
		meta := ServiceMeta{
			ServiceID:     "svc-meta-empty",
			Path:          "/some/path",
			IndexedCommit: "", // no git repo
			IndexedAt:     999,
		}
		if err := s.SetServiceMeta(ctx, meta); err != nil {
			t.Fatalf("SetServiceMeta empty commit: %v", err)
		}
		got, found, err := s.GetServiceMeta(ctx, "svc-meta-empty")
		if err != nil {
			t.Fatalf("GetServiceMeta: %v", err)
		}
		if !found {
			t.Fatal("expected found")
		}
		if got.IndexedCommit != "" {
			t.Errorf("IndexedCommit = %q, want empty", got.IndexedCommit)
		}
	})

	// ------------------------------------------------------------------ //
	// Commit round-trip + replace-all + newest-first ordering
	// ------------------------------------------------------------------ //
	t.Run("commits_roundtrip", func(t *testing.T) {
		svc := "svc-commits"
		commits := []Commit{
			{ServiceID: svc, Hash: "aaa", Author: "Alice", Date: 300, Message: "third M18.2", Files: []string{"a.go"}, SpecIDs: []string{"M18.2"}},
			{ServiceID: svc, Hash: "bbb", Author: "Bob", Date: 100, Message: "first", Files: []string{"b.go", "c.go"}, SpecIDs: nil},
			{ServiceID: svc, Hash: "ccc", Author: "Carol", Date: 200, Message: "second", Files: []string{"d.go"}, SpecIDs: nil},
		}
		if err := s.SetCommits(ctx, svc, commits); err != nil {
			t.Fatalf("SetCommits: %v", err)
		}
		got, err := s.ListCommits(ctx, svc)
		if err != nil {
			t.Fatalf("ListCommits: %v", err)
		}
		if len(got) != 3 {
			t.Fatalf("expected 3 commits, got %d", len(got))
		}
		// Newest-first: dates 300, 200, 100
		if got[0].Hash != "aaa" || got[1].Hash != "ccc" || got[2].Hash != "bbb" {
			t.Errorf("order wrong: %v %v %v", got[0].Hash, got[1].Hash, got[2].Hash)
		}
		if got[0].Author != "Alice" {
			t.Errorf("Author = %q, want Alice", got[0].Author)
		}
		if len(got[0].SpecIDs) != 1 || got[0].SpecIDs[0] != "M18.2" {
			t.Errorf("SpecIDs = %v, want [M18.2]", got[0].SpecIDs)
		}
		if len(got[2].Files) != 2 {
			t.Errorf("Files = %v, want 2", got[2].Files)
		}
	})

	t.Run("commits_replace_all", func(t *testing.T) {
		svc := "svc-commits-replace"
		first := []Commit{
			{ServiceID: svc, Hash: "old1", Date: 100, Message: "old", Files: nil, SpecIDs: nil},
			{ServiceID: svc, Hash: "old2", Date: 200, Message: "older", Files: nil, SpecIDs: nil},
		}
		if err := s.SetCommits(ctx, svc, first); err != nil {
			t.Fatalf("SetCommits first: %v", err)
		}
		second := []Commit{
			{ServiceID: svc, Hash: "new1", Date: 500, Message: "new", Files: []string{"x.go"}, SpecIDs: nil},
		}
		if err := s.SetCommits(ctx, svc, second); err != nil {
			t.Fatalf("SetCommits second: %v", err)
		}
		got, err := s.ListCommits(ctx, svc)
		if err != nil {
			t.Fatalf("ListCommits: %v", err)
		}
		if len(got) != 1 {
			t.Fatalf("replace-all: expected 1 commit, got %d", len(got))
		}
		if got[0].Hash != "new1" {
			t.Errorf("replace-all: hash = %q, want new1", got[0].Hash)
		}
	})

	t.Run("commits_empty_service", func(t *testing.T) {
		got, err := s.ListCommits(ctx, "svc-no-such-commits-xyz")
		if err != nil {
			t.Fatalf("ListCommits empty: %v", err)
		}
		if len(got) != 0 {
			t.Errorf("expected empty for unknown service, got %d", len(got))
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
