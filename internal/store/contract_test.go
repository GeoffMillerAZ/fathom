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
	// DeleteFingerprint removes only the target entry; missing is a no-op
	// ------------------------------------------------------------------ //
	t.Run("delete_fingerprint", func(t *testing.T) {
		svc := "svc-fp-del"
		if err := s.SetFingerprint(ctx, svc, "a.go", "hash-a"); err != nil {
			t.Fatalf("SetFingerprint a: %v", err)
		}
		if err := s.SetFingerprint(ctx, svc, "b.go", "hash-b"); err != nil {
			t.Fatalf("SetFingerprint b: %v", err)
		}

		if err := s.DeleteFingerprint(ctx, svc, "a.go"); err != nil {
			t.Fatalf("DeleteFingerprint: %v", err)
		}
		if _, found, err := s.Fingerprint(ctx, svc, "a.go"); err != nil {
			t.Fatalf("Fingerprint a after delete: %v", err)
		} else if found {
			t.Error("a.go fingerprint still present after DeleteFingerprint")
		}
		if _, found, err := s.Fingerprint(ctx, svc, "b.go"); err != nil {
			t.Fatalf("Fingerprint b: %v", err)
		} else if !found {
			t.Error("b.go fingerprint removed — DeleteFingerprint must only remove the target")
		}

		// Deleting a missing fingerprint is a no-op, not an error.
		if err := s.DeleteFingerprint(ctx, svc, "a.go"); err != nil {
			t.Errorf("DeleteFingerprint on missing entry: %v, want nil", err)
		}
	})

	// ------------------------------------------------------------------ //
	// ListSourceRefs returns the sorted union of observation + fingerprint refs
	// ------------------------------------------------------------------ //
	t.Run("list_source_refs", func(t *testing.T) {
		svc := "svc-refs"
		obs := []core.Observation{
			makeObs(svc, "", "pkg/obs-only.go", "ObsOnly"),
			makeObs(svc, "", "pkg/both.go", "Both"),
			makeObs(svc, "", "pkg/both.go", "Both2"), // duplicate ref must dedupe
		}
		if err := s.PutObservations(ctx, obs); err != nil {
			t.Fatalf("PutObservations: %v", err)
		}
		if err := s.SetFingerprint(ctx, svc, "pkg/both.go", "h1"); err != nil {
			t.Fatalf("SetFingerprint both: %v", err)
		}
		// Fingerprint-only ref (a file with no registered extractor).
		if err := s.SetFingerprint(ctx, svc, "pkg/fp-only.txt", "h2"); err != nil {
			t.Fatalf("SetFingerprint fp-only: %v", err)
		}
		// Another service's rows must not leak in.
		if err := s.SetFingerprint(ctx, "svc-refs-other", "other.go", "h3"); err != nil {
			t.Fatalf("SetFingerprint other: %v", err)
		}

		got, err := s.ListSourceRefs(ctx, svc)
		if err != nil {
			t.Fatalf("ListSourceRefs: %v", err)
		}
		want := []string{"pkg/both.go", "pkg/fp-only.txt", "pkg/obs-only.go"}
		if len(got) != len(want) {
			t.Fatalf("ListSourceRefs = %v, want %v", got, want)
		}
		for i := range want {
			if got[i] != want[i] {
				t.Errorf("ListSourceRefs[%d] = %q, want %q", i, got[i], want[i])
			}
		}

		empty, err := s.ListSourceRefs(ctx, "svc-refs-none-xyz")
		if err != nil {
			t.Fatalf("ListSourceRefs empty: %v", err)
		}
		if len(empty) != 0 {
			t.Errorf("expected no refs for unknown service, got %v", empty)
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

	// ------------------------------------------------------------------ //
	// SemanticClaims: round-trip, upsert, layer filter, link replace
	// ------------------------------------------------------------------ //
	t.Run("claims_roundtrip", func(t *testing.T) {
		compiledAt := time.Unix(1751900000, 123456789).UTC()
		claims := []core.SemanticClaim{
			{
				ClaimID:             "claim-rt",
				ServiceID:           "svc-claims-rt",
				Layer:               "obligations",
				LayerVersion:        "v1",
				Kind:                "obligation",
				Subject:             "CLAUDE.md#deterministic-excellence-gate",
				Rule:                "All outputs must be sorted.",
				Scope:               []string{"internal/**", "pkg/**"},
				Tier:                core.TierDocClaimed,
				TierProfile:         "engineering",
				Status:              core.ClaimFresh,
				Model:               "claude-opus-4-5",
				PromptVersion:       "obligations/v1",
				ReproKey:            "abc123",
				InputDigest:         "sha256-input",
				CompiledAtWatermark: "deadbeef",
				CompiledAt:          compiledAt,
			},
		}
		links := []core.ClaimLink{
			{ClaimID: "claim-rt", Rel: core.RelCites, TargetRef: "CLAUDE.md#L10-L20", TargetHash: "hash1"},
			{ClaimID: "claim-rt", Rel: core.RelScopes, TargetRef: "internal/**", TargetHash: ""},
		}
		if err := s.PutClaims(ctx, claims, links); err != nil {
			t.Fatalf("PutClaims: %v", err)
		}
		got, err := s.ClaimsForService(ctx, "svc-claims-rt", "")
		if err != nil {
			t.Fatalf("ClaimsForService: %v", err)
		}
		if len(got) != 1 {
			t.Fatalf("expected 1 claim, got %d", len(got))
		}
		c := got[0]
		if c.ClaimID != "claim-rt" {
			t.Errorf("ClaimID = %q, want claim-rt", c.ClaimID)
		}
		if c.Rule != "All outputs must be sorted." {
			t.Errorf("Rule = %q", c.Rule)
		}
		if len(c.Scope) != 2 || c.Scope[0] != "internal/**" || c.Scope[1] != "pkg/**" {
			t.Errorf("Scope = %v, want [internal/** pkg/**]", c.Scope)
		}
		if c.Tier != core.TierDocClaimed {
			t.Errorf("Tier = %q, want %q", c.Tier, core.TierDocClaimed)
		}
		if c.Status != core.ClaimFresh {
			t.Errorf("Status = %q, want fresh", c.Status)
		}
		if c.Model != "claude-opus-4-5" {
			t.Errorf("Model = %q", c.Model)
		}
		if c.PromptVersion != "obligations/v1" {
			t.Errorf("PromptVersion = %q", c.PromptVersion)
		}
		if c.InputDigest != "sha256-input" {
			t.Errorf("InputDigest = %q", c.InputDigest)
		}
		if c.CompiledAtWatermark != "deadbeef" {
			t.Errorf("CompiledAtWatermark = %q", c.CompiledAtWatermark)
		}
		if !c.CompiledAt.Equal(compiledAt) {
			t.Errorf("CompiledAt = %v, want %v", c.CompiledAt, compiledAt)
		}
		// Links round-trip.
		gotLinks, err := s.ClaimLinks(ctx, "svc-claims-rt", []string{"claim-rt"})
		if err != nil {
			t.Fatalf("ClaimLinks: %v", err)
		}
		if len(gotLinks) != 2 {
			t.Fatalf("expected 2 links, got %d", len(gotLinks))
		}
		// Order: (claim_id, rel, target_ref) — "cites" < "scopes"
		if gotLinks[0].Rel != core.RelCites || gotLinks[1].Rel != core.RelScopes {
			t.Errorf("link order: got %q, %q; want cites, scopes", gotLinks[0].Rel, gotLinks[1].Rel)
		}
		if gotLinks[0].TargetHash != "hash1" {
			t.Errorf("TargetHash = %q, want hash1", gotLinks[0].TargetHash)
		}
	})

	t.Run("claims_upsert_replaces_mutable_fields", func(t *testing.T) {
		svc := "svc-claims-up"
		orig := []core.SemanticClaim{
			{
				ClaimID: "c-up", ServiceID: svc, Layer: "obligations", Kind: "obligation",
				Subject: "S", Rule: "R-old", Status: core.ClaimFresh,
				Tier: core.TierDocClaimed, CompiledAt: time.Unix(1, 0).UTC(),
			},
		}
		if err := s.PutClaims(ctx, orig, nil); err != nil {
			t.Fatalf("PutClaims original: %v", err)
		}
		updated := []core.SemanticClaim{
			{
				ClaimID: "c-up", ServiceID: svc, Layer: "obligations", Kind: "obligation",
				Subject: "S", Rule: "R-new", Status: core.ClaimStale,
				Tier: core.TierDocClaimed, CompiledAt: time.Unix(2, 0).UTC(),
			},
		}
		if err := s.PutClaims(ctx, updated, nil); err != nil {
			t.Fatalf("PutClaims updated: %v", err)
		}
		got, err := s.ClaimsForService(ctx, svc, "")
		if err != nil {
			t.Fatalf("ClaimsForService: %v", err)
		}
		if len(got) != 1 {
			t.Fatalf("expected 1 claim after upsert, got %d", len(got))
		}
		if got[0].Rule != "R-new" {
			t.Errorf("Rule not updated: got %q", got[0].Rule)
		}
		if got[0].Status != core.ClaimStale {
			t.Errorf("Status not updated: got %q", got[0].Status)
		}
	})

	t.Run("claims_layer_filter_and_order", func(t *testing.T) {
		svc := "svc-claims-layers"
		claims := []core.SemanticClaim{
			{
				ClaimID: "cl-b", ServiceID: svc, Layer: "obligations", Kind: "obligation",
				Subject: "B", Status: core.ClaimFresh, Tier: core.TierDocClaimed, CompiledAt: time.Unix(1, 0).UTC(),
			},
			{
				ClaimID: "cl-a", ServiceID: svc, Layer: "findings", Kind: "finding",
				Subject: "A", Status: core.ClaimFresh, Tier: core.TierDocClaimed, CompiledAt: time.Unix(1, 0).UTC(),
			},
		}
		if err := s.PutClaims(ctx, claims, nil); err != nil {
			t.Fatalf("PutClaims: %v", err)
		}
		all, err := s.ClaimsForService(ctx, svc, "")
		if err != nil {
			t.Fatalf("ClaimsForService all: %v", err)
		}
		if len(all) != 2 {
			t.Fatalf("expected 2 claims for all layers, got %d", len(all))
		}
		// Deterministic order: by (layer, subject, claim_id) — "findings" < "obligations"
		if all[0].Layer != "findings" || all[1].Layer != "obligations" {
			t.Errorf("order: got layers %q, %q; want findings, obligations", all[0].Layer, all[1].Layer)
		}
		obls, err := s.ClaimsForService(ctx, svc, "obligations")
		if err != nil {
			t.Fatalf("ClaimsForService obligations: %v", err)
		}
		if len(obls) != 1 || obls[0].ClaimID != "cl-b" {
			t.Errorf("layer filter: got %v", obls)
		}
	})

	t.Run("claims_link_replace_semantics", func(t *testing.T) {
		svc := "svc-claims-links"
		claim := []core.SemanticClaim{
			{
				ClaimID: "cl-lr", ServiceID: svc, Layer: "obligations", Kind: "obligation",
				Subject: "S", Status: core.ClaimFresh, Tier: core.TierDocClaimed, CompiledAt: time.Unix(1, 0).UTC(),
			},
		}
		// First put: 2 links.
		if err := s.PutClaims(ctx, claim, []core.ClaimLink{
			{ClaimID: "cl-lr", Rel: core.RelCites, TargetRef: "a.md#L1-L2", TargetHash: "h1"},
			{ClaimID: "cl-lr", Rel: core.RelCites, TargetRef: "b.md#L1-L2", TargetHash: "h2"},
		}); err != nil {
			t.Fatalf("PutClaims 2 links: %v", err)
		}
		got, err := s.ClaimLinks(ctx, svc, []string{"cl-lr"})
		if err != nil {
			t.Fatalf("ClaimLinks after 2: %v", err)
		}
		if len(got) != 2 {
			t.Fatalf("expected 2 links, got %d", len(got))
		}
		// Second put: 1 link — old 2 must be gone.
		if err := s.PutClaims(ctx, claim, []core.ClaimLink{
			{ClaimID: "cl-lr", Rel: core.RelCites, TargetRef: "a.md#L1-L2", TargetHash: "h1"},
		}); err != nil {
			t.Fatalf("PutClaims 1 link: %v", err)
		}
		got, err = s.ClaimLinks(ctx, svc, []string{"cl-lr"})
		if err != nil {
			t.Fatalf("ClaimLinks after replace: %v", err)
		}
		if len(got) != 1 {
			t.Errorf("link replace: expected exactly 1 link, got %d", len(got))
		}
	})

	t.Run("claims_set_status", func(t *testing.T) {
		svc := "svc-claim-status"
		c := core.SemanticClaim{
			ClaimID: "cst-1", ServiceID: svc, Layer: "obligations", Kind: "obligation",
			Subject: "S", Status: core.ClaimFresh, Tier: core.TierDocClaimed, CompiledAt: time.Unix(1, 0).UTC(),
		}
		if err := s.PutClaims(ctx, []core.SemanticClaim{c}, nil); err != nil {
			t.Fatalf("PutClaims: %v", err)
		}
		if err := s.SetClaimStatus(ctx, svc, "cst-1", core.ClaimStale); err != nil {
			t.Fatalf("SetClaimStatus: %v", err)
		}
		got, err := s.ClaimsForService(ctx, svc, "")
		if err != nil {
			t.Fatalf("ClaimsForService: %v", err)
		}
		if len(got) != 1 || got[0].Status != core.ClaimStale {
			t.Errorf("status not flipped: got %v", got)
		}
		// Idempotent on absent claim — must not error.
		if err := s.SetClaimStatus(ctx, svc, "no-such-claim-xyz", core.ClaimRefuted); err != nil {
			t.Errorf("SetClaimStatus absent: expected no error, got %v", err)
		}
	})

	// ------------------------------------------------------------------ //
	// PutClaims refutation durability (FT-4, docs/plans/findings-tracer.md):
	// an upsert must not resurrect a refuted claim to the compiler's fresh
	// status — suppression is refutation with a note, and it must survive
	// every future recompile of the same claim_id.
	// ------------------------------------------------------------------ //
	t.Run("claims_refute_durability", func(t *testing.T) {
		svc := "svc-claim-durability"
		base := core.SemanticClaim{
			ServiceID: svc, Layer: "findings", Kind: "finding",
			Subject: "findings/wa-001.md#remediation", Rule: "Add retry with backoff.",
			Scope: []string{"mod/**"}, Tier: core.TierSMEStated, TierProfile: "engineering",
			Status: core.ClaimFresh, CompiledAt: time.Unix(1, 0).UTC(),
		}
		refuted := base
		refuted.ClaimID = "cl-refute-target"
		other := base
		other.ClaimID = "cl-refute-other"
		other.Subject = "findings/wa-002.md#remediation"

		// Initial compile: both claims land fresh.
		if err := s.PutClaims(ctx, []core.SemanticClaim{refuted, other}, nil); err != nil {
			t.Fatalf("PutClaims initial: %v", err)
		}

		// A human refutes the first claim (fathom claim refute).
		if err := s.SetClaimStatus(ctx, svc, refuted.ClaimID, core.ClaimRefuted); err != nil {
			t.Fatalf("SetClaimStatus refute: %v", err)
		}

		// A recompile re-emits BOTH claims unchanged, with the compiler's
		// default fresh status (as Compile always produces for a newly
		// verified draft) — this must not resurrect the refuted one.
		recompiled := refuted
		recompiled.Status = core.ClaimFresh
		recompiledOther := other
		recompiledOther.Status = core.ClaimFresh
		if err := s.PutClaims(ctx, []core.SemanticClaim{recompiled, recompiledOther}, nil); err != nil {
			t.Fatalf("PutClaims recompile: %v", err)
		}

		got, err := s.ClaimsForService(ctx, svc, "")
		if err != nil {
			t.Fatalf("ClaimsForService: %v", err)
		}
		byID := make(map[string]core.SemanticClaim, len(got))
		for _, c := range got {
			byID[c.ClaimID] = c
		}
		if c, ok := byID[refuted.ClaimID]; !ok || c.Status != core.ClaimRefuted {
			t.Errorf("refuted claim status after recompile = %+v, want status=refuted (durability)", c)
		}
		if c, ok := byID[other.ClaimID]; !ok || c.Status != core.ClaimFresh {
			t.Errorf("different claim_id status = %+v, want fresh (unaffected by the other claim's refutation)", c)
		}
	})

	t.Run("claim_events_append_and_order", func(t *testing.T) {
		t1 := time.Unix(1000, 0).UTC()
		t2 := time.Unix(2000, 0).UTC()
		evs := []core.ClaimEvent{
			{ClaimID: "evt-claim", Verdict: "corroborated", Author: "alice", Note: "looks good", At: t2},
			{ClaimID: "evt-claim", Verdict: "refuted", Author: "bob", Note: "wrong", At: t1},
		}
		for _, ev := range evs {
			if err := s.AppendClaimEvent(ctx, ev); err != nil {
				t.Fatalf("AppendClaimEvent: %v", err)
			}
		}
		got, err := s.ClaimEvents(ctx, "evt-claim")
		if err != nil {
			t.Fatalf("ClaimEvents: %v", err)
		}
		if len(got) != 2 {
			t.Fatalf("expected 2 events, got %d", len(got))
		}
		// Order by at asc: t1 (bob/refuted) first.
		if !got[0].At.Equal(t1) || got[0].Author != "bob" {
			t.Errorf("event order wrong: first = %+v", got[0])
		}
		if !got[1].At.Equal(t2) || got[1].Author != "alice" {
			t.Errorf("event order wrong: second = %+v", got[1])
		}
		// Unknown claim returns empty, no error.
		other, err := s.ClaimEvents(ctx, "no-such-claim-xyz")
		if err != nil {
			t.Fatalf("ClaimEvents unknown: %v", err)
		}
		if len(other) != 0 {
			t.Errorf("expected empty for unknown claim, got %d", len(other))
		}
	})

	t.Run("compile_runs_roundtrip_and_order", func(t *testing.T) {
		svc := "svc-cr"
		t1 := time.Unix(1000, 0).UTC()
		t2 := time.Unix(2000, 0).UTC()
		runs := []core.CompileRun{
			{
				RunID: "run-b", ServiceID: svc, Layer: "obligations",
				CorpusWatermark: "abc", Model: "claude-sonnet-4-5", PromptVersion: "obligations/v1",
				DryRun: false, TokensIn: 100, TokensOut: 50, CostUSD: 0.001, DurationMS: 200,
				SectionsTotal: 5, SectionsCompiled: 3, ClaimsEmitted: 2, ClaimsRejected: 1,
				RanAt: t1,
				// M1-7: a mid-run budget stop must round-trip too.
				BudgetStopped: true, RefusalReason: "token total 700 exceeds layer \"obligations\" budget max_tokens_per_run=600",
			},
			{
				RunID: "run-a", ServiceID: svc, Layer: "obligations",
				CorpusWatermark: "def", Model: "claude-sonnet-4-5", PromptVersion: "obligations/v1",
				DryRun: true, TokensIn: 0, TokensOut: 0, CostUSD: 0, DurationMS: 10,
				SectionsTotal: 5, SectionsCompiled: 0, ClaimsEmitted: 0, ClaimsRejected: 0,
				RanAt: t2,
				// M1-7: a pre-flight refusal must round-trip too.
				Refused: true, RefusalReason: "estimated cost $1.5000 exceeds layer \"obligations\" budget max_usd_per_run=$0.0001",
			},
		}
		for _, r := range runs {
			if err := s.RecordCompileRun(ctx, r); err != nil {
				t.Fatalf("RecordCompileRun: %v", err)
			}
		}
		got, err := s.CompileRuns(ctx, svc, "")
		if err != nil {
			t.Fatalf("CompileRuns: %v", err)
		}
		if len(got) != 2 {
			t.Fatalf("expected 2 runs, got %d", len(got))
		}
		// Newest-first: t2 (run-a) before t1 (run-b).
		if got[0].RunID != "run-a" || got[1].RunID != "run-b" {
			t.Errorf("order wrong: got %q, %q; want run-a, run-b", got[0].RunID, got[1].RunID)
		}
		if !got[0].DryRun {
			t.Errorf("DryRun = false, want true")
		}
		if got[1].TokensIn != 100 {
			t.Errorf("TokensIn = %d, want 100", got[1].TokensIn)
		}
		if got[1].CostUSD != 0.001 {
			t.Errorf("CostUSD = %v, want 0.001", got[1].CostUSD)
		}
		if !got[0].RanAt.Equal(t2) {
			t.Errorf("RanAt = %v, want %v", got[0].RanAt, t2)
		}
		// M1-7: Refused / RefusalReason / BudgetStopped round-trip.
		if !got[0].Refused || got[0].RefusalReason == "" {
			t.Errorf("run-a (newest) Refused/RefusalReason = %v/%q, want true/non-empty", got[0].Refused, got[0].RefusalReason)
		}
		if got[0].BudgetStopped {
			t.Errorf("run-a BudgetStopped = true, want false")
		}
		if !got[1].BudgetStopped || got[1].RefusalReason == "" {
			t.Errorf("run-b (oldest) BudgetStopped/RefusalReason = %v/%q, want true/non-empty", got[1].BudgetStopped, got[1].RefusalReason)
		}
		if got[1].Refused {
			t.Errorf("run-b Refused = true, want false")
		}
		// Layer filter: "findings" returns none.
		none, err := s.CompileRuns(ctx, svc, "findings")
		if err != nil {
			t.Fatalf("CompileRuns findings: %v", err)
		}
		if len(none) != 0 {
			t.Errorf("expected 0 for findings layer, got %d", len(none))
		}
	})

	// ------------------------------------------------------------------ //
	// Typed edges (ADR-021 §3): package-granularity replace, service scoping,
	// Kind round-trip, fingerprint cache, and PurgeService cleanup.
	// ------------------------------------------------------------------ //
	t.Run("typed_edges_roundtrip_and_replace", func(t *testing.T) {
		svc := "svc-typed"
		edgesA := []core.Edge{
			{FromService: svc, FromFile: "pkg/a.go", FromSymbol: svc + "::pkg.Caller", ToSymbol: svc + "::pkg.Direct", Kind: core.EdgeDirect},
			{FromService: svc, FromFile: "pkg/a.go", FromSymbol: svc + "::pkg.Caller", ToSymbol: svc + "::pkg.ImplA", Kind: core.EdgeIfaceDispatch},
			{FromService: svc, FromFile: "pkg/a.go", FromSymbol: svc + "::pkg.Caller", ToSymbol: svc + "::pkg.ImplB", Kind: core.EdgeIfaceDispatch},
		}
		if err := s.PutTypedEdges(ctx, svc, "example.com/pkg", edgesA); err != nil {
			t.Fatalf("PutTypedEdges: %v", err)
		}
		// A second package's edges must coexist (service-wide TypedEdges
		// returns the union) and must not be touched by a re-put of the
		// first package.
		edgesB := []core.Edge{
			{FromService: svc, FromFile: "pkg2/b.go", FromSymbol: svc + "::pkg2.Other", ToSymbol: svc + "::pkg2.Callee", Kind: core.EdgeUnresolved},
		}
		if err := s.PutTypedEdges(ctx, svc, "example.com/pkg2", edgesB); err != nil {
			t.Fatalf("PutTypedEdges pkg2: %v", err)
		}

		got, err := s.TypedEdges(ctx, svc)
		if err != nil {
			t.Fatalf("TypedEdges: %v", err)
		}
		if len(got) != 4 {
			t.Fatalf("TypedEdges count = %d, want 4: %+v", len(got), got)
		}
		// Deterministic order: (from_symbol, to_symbol, kind).
		for i := 1; i < len(got); i++ {
			if got[i-1].FromSymbol > got[i].FromSymbol {
				t.Fatalf("TypedEdges not sorted by FromSymbol at %d: %+v", i, got)
			}
		}
		var directCount, dispatchCount, unresolvedCount int
		for _, e := range got {
			switch e.Kind {
			case core.EdgeDirect:
				directCount++
			case core.EdgeIfaceDispatch:
				dispatchCount++
			case core.EdgeUnresolved:
				unresolvedCount++
			default:
				t.Errorf("unexpected Kind %q on edge %+v", e.Kind, e)
			}
		}
		if directCount != 1 || dispatchCount != 2 || unresolvedCount != 1 {
			t.Errorf("kind counts = direct:%d dispatch:%d unresolved:%d, want 1/2/1", directCount, dispatchCount, unresolvedCount)
		}

		// Replace semantics: re-putting pkg with a smaller edge set drops the
		// old rows for THAT package only — pkg2's edges survive untouched.
		edgesAReplaced := []core.Edge{
			{FromService: svc, FromFile: "pkg/a.go", FromSymbol: svc + "::pkg.Caller", ToSymbol: svc + "::pkg.OnlyOne", Kind: core.EdgeDirect},
		}
		if err := s.PutTypedEdges(ctx, svc, "example.com/pkg", edgesAReplaced); err != nil {
			t.Fatalf("PutTypedEdges replace: %v", err)
		}
		got, err = s.TypedEdges(ctx, svc)
		if err != nil {
			t.Fatalf("TypedEdges after replace: %v", err)
		}
		if len(got) != 2 {
			t.Fatalf("TypedEdges after replace = %d, want 2 (1 replaced + pkg2's 1): %+v", len(got), got)
		}

		// Empty edges is a valid call that still clears the package's rows.
		if err := s.PutTypedEdges(ctx, svc, "example.com/pkg", nil); err != nil {
			t.Fatalf("PutTypedEdges empty: %v", err)
		}
		got, err = s.TypedEdges(ctx, svc)
		if err != nil {
			t.Fatalf("TypedEdges after empty replace: %v", err)
		}
		if len(got) != 1 {
			t.Fatalf("TypedEdges after clearing pkg = %d, want 1 (pkg2 only): %+v", len(got), got)
		}
		if got[0].FromSymbol != svc+"::pkg2.Other" {
			t.Errorf("surviving edge = %+v, want pkg2's edge", got[0])
		}

		// A service with no typed edges at all returns nil, no error.
		none, err := s.TypedEdges(ctx, "svc-typed-empty")
		if err != nil {
			t.Fatalf("TypedEdges empty service: %v", err)
		}
		if len(none) != 0 {
			t.Errorf("TypedEdges(svc-typed-empty) = %d, want 0", len(none))
		}
	})

	t.Run("typed_edge_package_fingerprint", func(t *testing.T) {
		svc := "svc-typed-fp"
		if _, found, err := s.TypedEdgePackageFingerprint(ctx, svc, "example.com/pkg"); err != nil || found {
			t.Fatalf("TypedEdgePackageFingerprint before set = found:%v err:%v, want false/nil", found, err)
		}
		if err := s.SetTypedEdgePackageFingerprint(ctx, svc, "example.com/pkg", "hash-1"); err != nil {
			t.Fatalf("SetTypedEdgePackageFingerprint: %v", err)
		}
		hash, found, err := s.TypedEdgePackageFingerprint(ctx, svc, "example.com/pkg")
		if err != nil {
			t.Fatalf("TypedEdgePackageFingerprint: %v", err)
		}
		if !found || hash != "hash-1" {
			t.Errorf("TypedEdgePackageFingerprint = (%q, %v), want (hash-1, true)", hash, found)
		}
		// Replace semantics: setting again overwrites, does not duplicate.
		if err := s.SetTypedEdgePackageFingerprint(ctx, svc, "example.com/pkg", "hash-2"); err != nil {
			t.Fatalf("SetTypedEdgePackageFingerprint replace: %v", err)
		}
		hash, found, err = s.TypedEdgePackageFingerprint(ctx, svc, "example.com/pkg")
		if err != nil || !found || hash != "hash-2" {
			t.Errorf("TypedEdgePackageFingerprint after replace = (%q, %v, %v), want (hash-2, true, nil)", hash, found, err)
		}
	})

	t.Run("purge_service_clears_typed_edges", func(t *testing.T) {
		svc := "svc-typed-purge"
		edges := []core.Edge{
			{FromService: svc, FromFile: "pkg/a.go", FromSymbol: svc + "::pkg.Caller", ToSymbol: svc + "::pkg.Callee", Kind: core.EdgeDirect},
		}
		if err := s.PutTypedEdges(ctx, svc, "example.com/pkg", edges); err != nil {
			t.Fatalf("PutTypedEdges: %v", err)
		}
		if err := s.SetTypedEdgePackageFingerprint(ctx, svc, "example.com/pkg", "hash-1"); err != nil {
			t.Fatalf("SetTypedEdgePackageFingerprint: %v", err)
		}
		if err := s.PurgeService(ctx, svc); err != nil {
			t.Fatalf("PurgeService: %v", err)
		}
		got, err := s.TypedEdges(ctx, svc)
		if err != nil {
			t.Fatalf("TypedEdges after PurgeService: %v", err)
		}
		if len(got) != 0 {
			t.Errorf("TypedEdges after PurgeService = %d, want 0", len(got))
		}
		if _, found, _ := s.TypedEdgePackageFingerprint(ctx, svc, "example.com/pkg"); found {
			t.Error("typed-edge package fingerprint survived PurgeService")
		}
	})

	t.Run("claim_links_empty_ids", func(t *testing.T) {
		got, err := s.ClaimLinks(ctx, "any-svc", nil)
		if err != nil {
			t.Fatalf("ClaimLinks nil ids: %v", err)
		}
		if len(got) != 0 {
			t.Errorf("expected empty for nil claimIDs, got %d", len(got))
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
