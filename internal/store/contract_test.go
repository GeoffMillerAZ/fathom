package store

import (
	"context"
	"errors"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/internal/core"
)

// storeContract runs the full Store interface contract against the supplied
// Store. It is called from both sqlite_test.go and memory_test.go so both
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
	// ToolKind + Generated round-trip (ADR-021 §2 / critic F1). These two
	// fields were once write-only against the persistent backends: the
	// memory fake (whole-struct append) round-tripped them while sqlite and
	// duckdb dropped them at the column list — the exact fake-vs-real
	// divergence class this shared contract suite exists to prevent, so the
	// assertion lives HERE, not in a backend-specific test.
	// ------------------------------------------------------------------ //
	t.Run("tool_kind_and_generated_round_trip", func(t *testing.T) {
		obs := []core.Observation{
			{
				ServiceID:    "svc-toolkind",
				SourceRef:    "crds/widget.yaml",
				SourceKind:   core.KindK8sWorkload,
				EvidenceTier: core.TierVerified,
				Symbol:       "CustomResourceDefinition/widgets.example.com",
				Claim:        "apiextensions.k8s.io/v1",
				ToolKind:     "k8s-crd",
				IngestedAt:   time.Now().UTC().Truncate(time.Second),
			},
			{
				ServiceID:    "svc-toolkind",
				SourceRef:    "gen/foo.pb.go",
				SourceKind:   core.KindGoFunc,
				EvidenceTier: core.TierVerified,
				Symbol:       "GeneratedFunc",
				Generated:    true,
				IngestedAt:   time.Now().UTC().Truncate(time.Second),
			},
			{
				ServiceID:    "svc-toolkind",
				SourceRef:    "main.go",
				SourceKind:   core.KindGoFunc,
				EvidenceTier: core.TierVerified,
				Symbol:       "PlainFunc",
				IngestedAt:   time.Now().UTC().Truncate(time.Second),
			},
		}
		if err := s.PutObservations(ctx, obs); err != nil {
			t.Fatalf("PutObservations: %v", err)
		}
		got, err := s.QueryObservations(ctx, "svc-toolkind", "")
		if err != nil {
			t.Fatalf("QueryObservations: %v", err)
		}
		bySymbol := make(map[string]core.Observation, len(got))
		for _, o := range got {
			bySymbol[o.Symbol] = o
		}
		crd, ok := bySymbol["CustomResourceDefinition/widgets.example.com"]
		if !ok {
			t.Fatal("CRD observation not returned")
		}
		if crd.ToolKind != "k8s-crd" {
			t.Errorf("ToolKind not round-tripped: got %q, want %q", crd.ToolKind, "k8s-crd")
		}
		if crd.Generated {
			t.Error("CRD observation Generated: got true, want false")
		}
		gen, ok := bySymbol["GeneratedFunc"]
		if !ok {
			t.Fatal("generated observation not returned")
		}
		if !gen.Generated {
			t.Error("Generated not round-tripped: got false, want true")
		}
		plain, ok := bySymbol["PlainFunc"]
		if !ok {
			t.Fatal("plain observation not returned")
		}
		if plain.ToolKind != "" || plain.Generated {
			t.Errorf("plain observation should have zero-value ToolKind/Generated, got (%q, %v)", plain.ToolKind, plain.Generated)
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
	// ClaimServiceIDs returns distinct, sorted service IDs from
	// semantic_claims — the claims-only counterpart to ListServiceIDs
	// (docs/plans/gke-module-host.md's module-host federation layer,
	// critic finding 4: a claims-only, findings-style corpus with NO
	// observations rows at all must still be discoverable).
	// ------------------------------------------------------------------ //
	t.Run("claim_service_ids", func(t *testing.T) {
		claims := []core.SemanticClaim{
			{ClaimID: "claim-svc-ids-1", ServiceID: "svc-claimonly-a", Layer: "obligations", Kind: "obligation", Rule: "rule a", Status: core.ClaimFresh},
			{ClaimID: "claim-svc-ids-2", ServiceID: "svc-claimonly-b", Layer: "obligations", Kind: "obligation", Rule: "rule b", Status: core.ClaimFresh},
		}
		if err := s.PutClaims(ctx, claims, nil); err != nil {
			t.Fatalf("PutClaims: %v", err)
		}
		ids, err := s.ClaimServiceIDs(ctx)
		if err != nil {
			t.Fatalf("ClaimServiceIDs: %v", err)
		}
		idSet := make(map[string]bool, len(ids))
		for _, id := range ids {
			idSet[id] = true
		}
		if !idSet["svc-claimonly-a"] {
			t.Error("svc-claimonly-a missing from ClaimServiceIDs")
		}
		if !idSet["svc-claimonly-b"] {
			t.Error("svc-claimonly-b missing from ClaimServiceIDs")
		}
		// svc-claimonly-a/b never appear in ListServiceIDs (no observations
		// were ever written for them) — the whole point of this method.
		obsIDs, err := s.ListServiceIDs(ctx)
		if err != nil {
			t.Fatalf("ListServiceIDs: %v", err)
		}
		for _, id := range obsIDs {
			if id == "svc-claimonly-a" || id == "svc-claimonly-b" {
				t.Errorf("claims-only service %q unexpectedly present in ListServiceIDs (observations-derived)", id)
			}
		}
		for i := 1; i < len(ids); i++ {
			if ids[i] < ids[i-1] {
				t.Errorf("ClaimServiceIDs not sorted at index %d: %q > %q", i, ids[i-1], ids[i])
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

	// E2-1: the commit message column must round-trip a FULL multi-line
	// commit body (subject + blank line + prose + blank line + a
	// Co-Authored-By trailer) byte-for-byte, in every backend — proving no
	// backend silently truncates at the first newline (or the first NUL a
	// naive C-string-shaped column would) now that parseGitLog widens
	// Message beyond the subject line.
	t.Run("commits_multiline_message_roundtrip", func(t *testing.T) {
		svc := "svc-commits-multiline"
		body := "Fix auth flow\n\nLonger explanation of the change,\nacross two lines.\n\nCo-Authored-By: Claude Fable 5 <noreply@anthropic.com>"
		commits := []Commit{
			{ServiceID: svc, Hash: "full-body-1", Author: "Alice", Date: 100, Message: body, Files: []string{"pkg/auth/auth.go"}},
		}
		if err := s.SetCommits(ctx, svc, commits); err != nil {
			t.Fatalf("SetCommits: %v", err)
		}
		got, err := s.ListCommits(ctx, svc)
		if err != nil {
			t.Fatalf("ListCommits: %v", err)
		}
		if len(got) != 1 {
			t.Fatalf("expected 1 commit, got %d", len(got))
		}
		if got[0].Message != body {
			t.Errorf("message round-trip mismatch:\n got:  %q\n want: %q", got[0].Message, body)
		}
		if !strings.Contains(got[0].Message, "\n") {
			t.Error("round-tripped message lost its newlines")
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
			{ClaimID: "claim-rt", Rel: core.RelCites, TargetRef: "CLAUDE.md#L10-L20", TargetHash: "hash1", AnchorText: "line ten\nthrough line twenty"},
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
		// AnchorText round-trip (T5-1/O13): the literal cited text the sweep
		// re-anchors from must survive the store, not just the hash.
		if gotLinks[0].AnchorText != "line ten\nthrough line twenty" {
			t.Errorf("AnchorText = %q, want the seeded snippet", gotLinks[0].AnchorText)
		}
		if gotLinks[1].AnchorText != "" {
			t.Errorf("scopes link AnchorText = %q, want empty (cites-only field)", gotLinks[1].AnchorText)
		}
	})

	// UpdateClaimLink: the sweep's citation re-anchor primitive (T5-1/O13) —
	// rewrites TargetRef/TargetHash for one matching link in place, leaves
	// AnchorText and every other link of the claim untouched, and is a
	// harmless no-op when the old ref no longer matches.
	t.Run("claims_link_update_reanchors", func(t *testing.T) {
		svc := "svc-claims-reanchor"
		claims := []core.SemanticClaim{
			{
				ClaimID: "c-ra", ServiceID: svc, Layer: "obligations", Kind: "obligation",
				Subject: "S", Rule: "R", Status: core.ClaimFresh,
				Tier: core.TierDocClaimed, CompiledAt: time.Unix(1, 0).UTC(),
			},
		}
		links := []core.ClaimLink{
			{ClaimID: "c-ra", Rel: core.RelCites, TargetRef: "CLAUDE.md#L5-L5", TargetHash: "old-hash", AnchorText: "the rule text"},
			{ClaimID: "c-ra", Rel: core.RelScopes, TargetRef: "internal/**"},
		}
		if err := s.PutClaims(ctx, claims, links); err != nil {
			t.Fatalf("PutClaims: %v", err)
		}

		if err := s.UpdateClaimLink(ctx, svc, "c-ra", core.RelCites, "CLAUDE.md#L5-L5", "CLAUDE.md#L7-L7", "new-hash"); err != nil {
			t.Fatalf("UpdateClaimLink: %v", err)
		}
		got, err := s.ClaimLinks(ctx, svc, []string{"c-ra"})
		if err != nil {
			t.Fatalf("ClaimLinks after update: %v", err)
		}
		if len(got) != 2 {
			t.Fatalf("expected 2 links after update (cites + scopes), got %d: %+v", len(got), got)
		}
		var cites core.ClaimLink
		for _, l := range got {
			if l.Rel == core.RelCites {
				cites = l
			}
		}
		if cites.TargetRef != "CLAUDE.md#L7-L7" {
			t.Errorf("TargetRef after update = %q, want CLAUDE.md#L7-L7", cites.TargetRef)
		}
		if cites.TargetHash != "new-hash" {
			t.Errorf("TargetHash after update = %q, want new-hash", cites.TargetHash)
		}
		if cites.AnchorText != "the rule text" {
			t.Errorf("AnchorText after update = %q, want unchanged %q", cites.AnchorText, "the rule text")
		}

		// A no-op update (stale oldTargetRef) must not error and must not
		// disturb the current link.
		if err := s.UpdateClaimLink(ctx, svc, "c-ra", core.RelCites, "CLAUDE.md#L5-L5", "CLAUDE.md#L99-L99", "unused-hash"); err != nil {
			t.Fatalf("UpdateClaimLink no-op: %v", err)
		}
		got2, err := s.ClaimLinks(ctx, svc, []string{"c-ra"})
		if err != nil {
			t.Fatalf("ClaimLinks after no-op update: %v", err)
		}
		for _, l := range got2 {
			if l.Rel == core.RelCites && l.TargetRef != "CLAUDE.md#L7-L7" {
				t.Errorf("no-op update disturbed the link: %+v", l)
			}
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

	// ------------------------------------------------------------------ //
	// Findings-as-evidence (docs/design/findings-as-evidence.md §3.2/§3.4,
	// §6 items 2/4): a finding-kind claim's Class round-trips through
	// PutClaims/ClaimsForService, and a "superseded" claim_event's
	// ResolutionKind/ResolutionCommit round-trip through
	// AppendClaimEvent/ClaimEvents — both fields additive on every backend.
	// ------------------------------------------------------------------ //
	t.Run("finding_class_round_trips", func(t *testing.T) {
		claim := core.SemanticClaim{
			ClaimID: "finding-class-1", ServiceID: "svc-findingclass", Layer: "findings", Kind: "finding",
			Subject: "arch_conformance#pkg.Foo", Rule: "no import cycle", Status: core.ClaimFresh,
			Class: core.FindingClassConformance,
		}
		if err := s.PutClaims(ctx, []core.SemanticClaim{claim}, nil); err != nil {
			t.Fatalf("PutClaims: %v", err)
		}
		got, err := s.ClaimsForService(ctx, "svc-findingclass", "findings")
		if err != nil {
			t.Fatalf("ClaimsForService: %v", err)
		}
		if len(got) != 1 || got[0].Class != core.FindingClassConformance {
			t.Fatalf("got %+v, want 1 claim with Class=%q", got, core.FindingClassConformance)
		}

		// A non-finding claim with no Class set round-trips as "".
		plain := core.SemanticClaim{
			ClaimID: "finding-class-2", ServiceID: "svc-findingclass", Layer: "obligations", Kind: "obligation",
			Subject: "x", Rule: "y", Status: core.ClaimFresh,
		}
		if err := s.PutClaims(ctx, []core.SemanticClaim{plain}, nil); err != nil {
			t.Fatalf("PutClaims plain: %v", err)
		}
		got2, err := s.ClaimsForService(ctx, "svc-findingclass", "obligations")
		if err != nil {
			t.Fatalf("ClaimsForService obligations: %v", err)
		}
		if len(got2) != 1 || got2[0].Class != "" {
			t.Fatalf("got %+v, want 1 claim with empty Class", got2)
		}
	})

	t.Run("claim_event_resolution_kind_round_trips", func(t *testing.T) {
		ev := core.ClaimEvent{
			ClaimID: "resolution-evt-1", Verdict: "superseded", Author: "review",
			Note: "finding fixed", At: time.Unix(5000, 0).UTC(),
			ResolutionKind: core.ResolutionCodeChange, ResolutionCommit: "abc123",
		}
		if err := s.AppendClaimEvent(ctx, ev); err != nil {
			t.Fatalf("AppendClaimEvent: %v", err)
		}
		got, err := s.ClaimEvents(ctx, "resolution-evt-1")
		if err != nil {
			t.Fatalf("ClaimEvents: %v", err)
		}
		if len(got) != 1 || got[0].ResolutionKind != core.ResolutionCodeChange || got[0].ResolutionCommit != "abc123" {
			t.Fatalf("got %+v, want ResolutionKind=%q ResolutionCommit=%q", got, core.ResolutionCodeChange, "abc123")
		}

		// A refuted event never sets ResolutionKind (it's a separate,
		// self-describing outcome — see ClaimEvent.ResolutionKind's doc
		// comment) — round-trips as empty.
		refuted := core.ClaimEvent{ClaimID: "resolution-evt-2", Verdict: "refuted", Author: "human", At: time.Unix(6000, 0).UTC()}
		if err := s.AppendClaimEvent(ctx, refuted); err != nil {
			t.Fatalf("AppendClaimEvent refuted: %v", err)
		}
		gotRefuted, err := s.ClaimEvents(ctx, "resolution-evt-2")
		if err != nil {
			t.Fatalf("ClaimEvents refuted: %v", err)
		}
		if len(gotRefuted) != 1 || gotRefuted[0].ResolutionKind != "" || gotRefuted[0].ResolutionCommit != "" {
			t.Fatalf("got %+v, want empty ResolutionKind/ResolutionCommit", gotRefuted)
		}
	})

	// ------------------------------------------------------------------ //
	// D2 (docs/plans/expert-depth.md): the refute/contradict write path
	// always pairs a status flip (SetClaimStatus) with an append-only audit
	// row (AppendClaimEvent) — the obligations tool's feedback action does
	// this for a human "refuted" verdict, and the post-compile sweep does it
	// automatically for anchor-dead claims (D2-2, pkg/client.SweepClaims)
	// with a "contradicted" verdict. Both effects must round-trip TOGETHER
	// through every backend, not just each in isolation (
	// claim_events_append_and_order above already covers the event log
	// alone; claims_status_flip above covers the status flip alone) — this
	// is the combined contract those two callers depend on. This subtest
	// models the SWEEP variant specifically (author "sweep", the anchor-dead
	// note text, status ClaimContradicted): P1-9 (docs/plans/
	// pulled-tools-excellence.md) fixed this fixture from an earlier draft
	// that paired sweep's own note/author with core.ClaimRefuted and
	// Verdict:"refuted" — the exact status/verdict vocabulary mismatch this
	// row now guards against. "refuted" (core.ClaimRefuted, never served
	// again) and "contradicted" (served WITH the status label) are different
	// claims about the world and must never share a verdict string.
	// ------------------------------------------------------------------ //
	t.Run("claim_sweep_contradicted_flips_status_and_appends_event", func(t *testing.T) {
		svc := "svc-feedback"
		claim := core.SemanticClaim{
			ClaimID: "cl-feedback-1", ServiceID: svc,
			Layer: "obligations", LayerVersion: "v1", Kind: "obligation",
			Subject: "CLAUDE.md#s1", Rule: "Run tests before committing.",
			Scope: []string{"**"}, Tier: core.TierDocClaimed, TierProfile: "engineering",
			Status: core.ClaimFresh, CompiledAt: time.Unix(1, 0).UTC(),
		}
		if err := s.PutClaims(ctx, []core.SemanticClaim{claim}, nil); err != nil {
			t.Fatalf("PutClaims: %v", err)
		}

		at := time.Unix(5000, 0).UTC()
		ev := core.ClaimEvent{ClaimID: claim.ClaimID, Verdict: "contradicted", Author: "sweep", Note: "citation anchor no longer resolves", At: at}
		if err := s.AppendClaimEvent(ctx, ev); err != nil {
			t.Fatalf("AppendClaimEvent: %v", err)
		}
		if err := s.SetClaimStatus(ctx, svc, claim.ClaimID, core.ClaimContradicted); err != nil {
			t.Fatalf("SetClaimStatus: %v", err)
		}

		claims, err := s.ClaimsForService(ctx, svc, "")
		if err != nil {
			t.Fatalf("ClaimsForService: %v", err)
		}
		found := false
		for _, c := range claims {
			if c.ClaimID == claim.ClaimID {
				found = true
				if c.Status != core.ClaimContradicted {
					t.Errorf("status = %q, want contradicted", c.Status)
				}
			}
		}
		if !found {
			t.Fatalf("claim %q not found after feedback", claim.ClaimID)
		}

		events, err := s.ClaimEvents(ctx, claim.ClaimID)
		if err != nil {
			t.Fatalf("ClaimEvents: %v", err)
		}
		if len(events) != 1 {
			t.Fatalf("expected 1 event, got %d: %+v", len(events), events)
		}
		if events[0].Verdict != "contradicted" || events[0].Author != "sweep" || !events[0].At.Equal(at) {
			t.Errorf("event = %+v", events[0])
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
				// A2-1/D3-1: a guard-override audit note must round-trip on
				// every backend — the fake matching the real stores here is
				// exactly what the bounce's fake-vs-real divergence check
				// exists for.
				Escape: "force: idempotent recompile of run run-z (watermark=\"abc\", prompt=\"obligations/v1\")",
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
		// A2-1/D3-1: Escape round-trips (run-b set it, run-a did not).
		if want := "force: idempotent recompile of run run-z (watermark=\"abc\", prompt=\"obligations/v1\")"; got[1].Escape != want {
			t.Errorf("run-b Escape = %q, want %q", got[1].Escape, want)
		}
		if got[0].Escape != "" {
			t.Errorf("run-a Escape = %q, want empty", got[0].Escape)
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

	t.Run("typed_edge_package_listing_and_delete", func(t *testing.T) {
		svc := "svc-typed-list"
		edgesA := []core.Edge{
			{FromService: svc, FromFile: "a/a.go", FromSymbol: svc + "::a.F", ToSymbol: svc + "::a.G", Kind: core.EdgeDirect},
		}
		if err := s.PutTypedEdges(ctx, svc, "example.com/a", edgesA); err != nil {
			t.Fatalf("PutTypedEdges a: %v", err)
		}
		if err := s.SetTypedEdgePackageFingerprint(ctx, svc, "example.com/a", "hash-a"); err != nil {
			t.Fatalf("SetTypedEdgePackageFingerprint a: %v", err)
		}
		// b has ONLY a fingerprint (a load that emitted zero edges) — the
		// listing must still surface it, or reconcile could never clear a
		// deleted package that happened to have no edges.
		if err := s.SetTypedEdgePackageFingerprint(ctx, svc, "example.com/b", "hash-b"); err != nil {
			t.Fatalf("SetTypedEdgePackageFingerprint b: %v", err)
		}
		// c has ONLY edges (fingerprint write failed mid-run) — must also list.
		edgesC := []core.Edge{
			{FromService: svc, FromFile: "c/c.go", FromSymbol: svc + "::c.F", ToSymbol: svc + "::c.G", Kind: core.EdgeDirect},
		}
		if err := s.PutTypedEdges(ctx, svc, "example.com/c", edgesC); err != nil {
			t.Fatalf("PutTypedEdges c: %v", err)
		}

		got, err := s.ListTypedEdgePackages(ctx, svc)
		if err != nil {
			t.Fatalf("ListTypedEdgePackages: %v", err)
		}
		want := []string{"example.com/a", "example.com/b", "example.com/c"}
		if len(got) != len(want) {
			t.Fatalf("ListTypedEdgePackages = %v, want %v", got, want)
		}
		for i := range want {
			if got[i] != want[i] {
				t.Fatalf("ListTypedEdgePackages = %v, want %v (sorted)", got, want)
			}
		}

		// Delete a: its edges AND fingerprint go; b and c untouched.
		if err := s.DeleteTypedEdgePackage(ctx, svc, "example.com/a"); err != nil {
			t.Fatalf("DeleteTypedEdgePackage: %v", err)
		}
		if _, found, _ := s.TypedEdgePackageFingerprint(ctx, svc, "example.com/a"); found {
			t.Error("fingerprint for deleted package survived")
		}
		edges, err := s.TypedEdges(ctx, svc)
		if err != nil {
			t.Fatalf("TypedEdges after delete: %v", err)
		}
		for _, e := range edges {
			if e.FromSymbol == svc+"::a.F" {
				t.Errorf("edge from deleted package survived: %+v", e)
			}
		}
		got, err = s.ListTypedEdgePackages(ctx, svc)
		if err != nil {
			t.Fatalf("ListTypedEdgePackages after delete: %v", err)
		}
		if len(got) != 2 || got[0] != "example.com/b" || got[1] != "example.com/c" {
			t.Errorf("ListTypedEdgePackages after delete = %v, want [example.com/b example.com/c]", got)
		}

		// Deleting a missing package is a no-op, not an error.
		if err := s.DeleteTypedEdgePackage(ctx, svc, "example.com/nope"); err != nil {
			t.Errorf("DeleteTypedEdgePackage(missing) = %v, want nil", err)
		}

		// A service with no typed-edge rows lists nil, no error.
		none, err := s.ListTypedEdgePackages(ctx, "svc-typed-list-empty")
		if err != nil || len(none) != 0 {
			t.Errorf("ListTypedEdgePackages(empty) = %v, %v; want nil, nil", none, err)
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

	// ------------------------------------------------------------------ //
	// RemoveService: registry surgery — EVERY row for the service goes
	// (PurgeService's content set PLUS identity rows, the semantic layer,
	// and the claim_id-keyed claim_events), a sibling service survives
	// untouched, and a second call is an idempotent removed=false no-op.
	// ------------------------------------------------------------------ //
	t.Run("remove_service_removes_every_row", func(t *testing.T) {
		svc, sibling := "svc-remove", "svc-remove-sibling"
		claimID, siblingClaimID := "claim-remove-1", "claim-remove-sibling-1"
		seedServiceEverywhere(ctx, t, s, svc, claimID)
		seedServiceEverywhere(ctx, t, s, sibling, siblingClaimID)

		removed, err := s.RemoveService(ctx, svc)
		if err != nil {
			t.Fatalf("RemoveService: %v", err)
		}
		if !removed {
			t.Error("RemoveService on a fully seeded service: removed = false, want true")
		}

		assertServiceAbsent(ctx, t, s, svc)

		// The removed service's claim events go with its claims...
		evs, err := s.ClaimEvents(ctx, claimID)
		if err != nil {
			t.Fatalf("ClaimEvents after remove: %v", err)
		}
		if len(evs) != 0 {
			t.Errorf("claim events survived RemoveService: %+v", evs)
		}
		// ...and so do its claim links.
		links, err := s.ClaimLinks(ctx, svc, []string{claimID})
		if err != nil {
			t.Fatalf("ClaimLinks after remove: %v", err)
		}
		if len(links) != 0 {
			t.Errorf("claim links survived RemoveService: %+v", links)
		}

		// The sibling's rows survive in every table class.
		assertServicePresent(ctx, t, s, sibling, siblingClaimID)

		// Idempotent: a second call finds nothing and reports removed=false
		// with no error.
		removed, err = s.RemoveService(ctx, svc)
		if err != nil {
			t.Fatalf("RemoveService second call: %v", err)
		}
		if removed {
			t.Error("RemoveService second call: removed = true, want false")
		}
	})

	t.Run("remove_service_unknown_is_noop", func(t *testing.T) {
		removed, err := s.RemoveService(ctx, "svc-remove-never-existed-xyz")
		if err != nil {
			t.Fatalf("RemoveService unknown: %v", err)
		}
		if removed {
			t.Error("RemoveService unknown service: removed = true, want false")
		}
	})

	// ------------------------------------------------------------------ //
	// RenameService: every service_id-scoped row moves to the new ID in one
	// step; claim_events stay put and claim IDs keep their minted values
	// (they are historical evidence — the event log they anchor must keep
	// resolving).
	// ------------------------------------------------------------------ //
	t.Run("rename_service_moves_every_row", func(t *testing.T) {
		oldID, newID := "svc-rename-old", "svc-rename-new"
		claimID := "claim-rename-1"
		seedServiceEverywhere(ctx, t, s, oldID, claimID)

		if err := s.RenameService(ctx, oldID, newID); err != nil {
			t.Fatalf("RenameService: %v", err)
		}

		// Every read path serves the seeded data under the NEW id...
		assertServicePresent(ctx, t, s, newID, claimID)
		// ...and the row payloads carry the new id where service identity is
		// a stored column, not just where it is the query key.
		if got, err := s.QueryObservations(ctx, newID, ""); err != nil || len(got) == 0 || got[0].ServiceID != newID {
			t.Errorf("observations after rename: got %d rows (err %v), want ServiceID %q on each", len(got), err, newID)
		}
		if meta, found, err := s.GetServiceMeta(ctx, newID); err != nil || !found || meta.ServiceID != newID || meta.Path != "/code/"+oldID {
			t.Errorf("service meta after rename = (%+v, %v, %v), want ServiceID %q with the old path preserved", meta, found, err, newID)
		}
		if claims, err := s.ClaimsForService(ctx, newID, ""); err != nil || len(claims) != 1 || claims[0].ServiceID != newID || claims[0].ClaimID != claimID {
			t.Errorf("claims after rename = (%+v, %v), want the seeded claim under %q with its MINTED id %q", claims, err, newID, claimID)
		}

		// The old id reads empty everywhere.
		assertServiceAbsent(ctx, t, s, oldID)

		// claim_events are untouched by a rename: the event log still
		// resolves by the claim's minted id.
		evs, err := s.ClaimEvents(ctx, claimID)
		if err != nil {
			t.Fatalf("ClaimEvents after rename: %v", err)
		}
		if len(evs) != 1 || evs[0].ClaimID != claimID {
			t.Errorf("claim events after rename = %+v, want the seeded event under the minted claim id %q", evs, claimID)
		}
	})

	t.Run("rename_service_refuses_existing_target", func(t *testing.T) {
		src, dst := "svc-rename-src", "svc-rename-taken"
		seedServiceEverywhere(ctx, t, s, src, "claim-rename-src-1")
		// The target only needs ANY row — service_meta alone suffices.
		if err := s.SetServiceMeta(ctx, ServiceMeta{ServiceID: dst, Path: "/code/" + dst}); err != nil {
			t.Fatalf("SetServiceMeta target: %v", err)
		}

		err := s.RenameService(ctx, src, dst)
		if err == nil {
			t.Fatal("RenameService onto an existing service: expected error, got nil")
		}
		var exists *ServiceExistsError
		if !errors.As(err, &exists) {
			t.Fatalf("RenameService onto existing = %v (%T), want *ServiceExistsError", err, err)
		}
		if exists.ServiceID != dst {
			t.Errorf("ServiceExistsError.ServiceID = %q, want %q", exists.ServiceID, dst)
		}
		// The refusal must not have moved anything.
		assertServicePresent(ctx, t, s, src, "claim-rename-src-1")
	})

	t.Run("rename_service_refuses_missing_source", func(t *testing.T) {
		err := s.RenameService(ctx, "svc-rename-missing-xyz", "svc-rename-target-xyz")
		if err == nil {
			t.Fatal("RenameService of a missing service: expected error, got nil")
		}
		var notFound *ServiceNotFoundError
		if !errors.As(err, &notFound) {
			t.Fatalf("RenameService missing = %v (%T), want *ServiceNotFoundError", err, err)
		}
		if notFound.ServiceID != "svc-rename-missing-xyz" {
			t.Errorf("ServiceNotFoundError.ServiceID = %q, want the missing source id", notFound.ServiceID)
		}
	})

	// review_runs (FL-15, docs/plans/findings-loop.md): the idempotency
	// cache `fathom review --idempotency-key` reads/writes.
	t.Run("review_run_roundtrip_and_replace", func(t *testing.T) {
		if _, found, err := s.ReviewRun(ctx, "svc-review-run", "key-1"); err != nil || found {
			t.Fatalf("ReviewRun before any record: found=%v err=%v, want not found", found, err)
		}
		recorded := time.Now().UTC().Truncate(time.Second)
		run := core.ReviewRun{
			ServiceID:      "svc-review-run",
			IdempotencyKey: "key-1",
			PayloadDigest:  "digest-a",
			ResponseJSON:   `{"schema_version":1}`,
			RecordedAt:     recorded,
		}
		if err := s.RecordReviewRun(ctx, run); err != nil {
			t.Fatalf("RecordReviewRun: %v", err)
		}
		got, found, err := s.ReviewRun(ctx, "svc-review-run", "key-1")
		if err != nil || !found {
			t.Fatalf("ReviewRun after record: found=%v err=%v", found, err)
		}
		if got.PayloadDigest != "digest-a" || got.ResponseJSON != run.ResponseJSON || !got.RecordedAt.Equal(recorded) {
			t.Errorf("ReviewRun = %+v, want %+v", got, run)
		}

		// A second record for the SAME key replaces the row (the caller
		// only calls this after confirming no prior row, or an identical
		// payload digest — see the Store interface doc comment).
		run.PayloadDigest = "digest-a"
		run.ResponseJSON = `{"schema_version":1,"replayed":true}`
		if err := s.RecordReviewRun(ctx, run); err != nil {
			t.Fatalf("RecordReviewRun replace: %v", err)
		}
		got, found, err = s.ReviewRun(ctx, "svc-review-run", "key-1")
		if err != nil || !found || got.ResponseJSON != run.ResponseJSON {
			t.Fatalf("ReviewRun after replace = %+v found=%v err=%v", got, found, err)
		}

		// A different service never sees another service's review run.
		if _, found, err := s.ReviewRun(ctx, "svc-review-run-other", "key-1"); err != nil || found {
			t.Fatalf("ReviewRun cross-service: found=%v err=%v, want not found", found, err)
		}
	})
}

// -------------------------------------------------------------------------
// Helpers
// -------------------------------------------------------------------------

// seedServiceEverywhere seeds serviceID with at least one row in every table
// class the store persists — observations, fingerprints, knowledge cards,
// doc summaries, service meta, commits, semantic claims + links, claim
// events, compile runs, typed edges and typed-edge fingerprints — the
// full-footprint fixture the RemoveService/RenameService subtests assert
// over. claimID names the seeded claim (and its link + event rows) so
// callers can follow the claim_id-keyed tables across a remove or rename.
func seedServiceEverywhere(ctx context.Context, t *testing.T, s Store, serviceID, claimID string) {
	t.Helper()
	if err := s.PutObservations(ctx, []core.Observation{makeObs(serviceID, "c1", "pkg/seed.go", "SeededFunc")}); err != nil {
		t.Fatalf("seed PutObservations: %v", err)
	}
	if err := s.SetFingerprint(ctx, serviceID, "pkg/seed.go", "seed-hash"); err != nil {
		t.Fatalf("seed SetFingerprint: %v", err)
	}
	if err := s.PutKnowledgeCards(ctx, []KnowledgeCard{
		{ServiceID: serviceID, Symbol: "SeededFunc", File: "pkg/seed.go", Summary: "seeded card", Tags: []string{"seed"}},
	}); err != nil {
		t.Fatalf("seed PutKnowledgeCards: %v", err)
	}
	if err := s.PutDocSummaries(ctx, []DocSummary{
		{ServiceID: serviceID, SourceRef: "docs/seed.md", Title: "Seed", Summary: "seeded summary"},
	}); err != nil {
		t.Fatalf("seed PutDocSummaries: %v", err)
	}
	if err := s.SetServiceMeta(ctx, ServiceMeta{ServiceID: serviceID, Path: "/code/" + serviceID, IndexedCommit: "abc1234", IndexedAt: 1}); err != nil {
		t.Fatalf("seed SetServiceMeta: %v", err)
	}
	if err := s.SetCommits(ctx, serviceID, []Commit{
		{ServiceID: serviceID, Hash: "seed-" + serviceID, Author: "Seeder", Date: 1, Message: "seed commit", Files: []string{"pkg/seed.go"}},
	}); err != nil {
		t.Fatalf("seed SetCommits: %v", err)
	}
	claims := []core.SemanticClaim{{
		ClaimID: claimID, ServiceID: serviceID, Layer: "obligations", Kind: "obligation",
		Subject: "docs/seed.md#rule", Rule: "Seeded rule.", Status: core.ClaimFresh,
		Tier: core.TierDocClaimed, CompiledAt: time.Unix(1, 0).UTC(),
	}}
	links := []core.ClaimLink{
		{ClaimID: claimID, Rel: core.RelCites, TargetRef: "docs/seed.md#L1-L2", TargetHash: "seed-link-hash", AnchorText: "seeded anchor"},
	}
	if err := s.PutClaims(ctx, claims, links); err != nil {
		t.Fatalf("seed PutClaims: %v", err)
	}
	if err := s.AppendClaimEvent(ctx, core.ClaimEvent{ClaimID: claimID, Verdict: "corroborated", Author: "seed", Note: "seeded event", At: time.Unix(100, 0).UTC()}); err != nil {
		t.Fatalf("seed AppendClaimEvent: %v", err)
	}
	if err := s.RecordCompileRun(ctx, core.CompileRun{RunID: "run-" + serviceID, ServiceID: serviceID, Layer: "obligations", RanAt: time.Unix(1, 0).UTC()}); err != nil {
		t.Fatalf("seed RecordCompileRun: %v", err)
	}
	if err := s.PutTypedEdges(ctx, serviceID, "example.com/seed", []core.Edge{
		{FromService: serviceID, FromFile: "pkg/seed.go", FromSymbol: serviceID + "::pkg.SeededFunc", ToSymbol: serviceID + "::pkg.Callee", Kind: core.EdgeDirect},
	}); err != nil {
		t.Fatalf("seed PutTypedEdges: %v", err)
	}
	if err := s.SetTypedEdgePackageFingerprint(ctx, serviceID, "example.com/seed", "seed-edge-fp"); err != nil {
		t.Fatalf("seed SetTypedEdgePackageFingerprint: %v", err)
	}
}

// assertServicePresent asserts every read path still serves the rows
// seedServiceEverywhere planted for serviceID.
func assertServicePresent(ctx context.Context, t *testing.T, s Store, serviceID, claimID string) {
	t.Helper()
	if got, err := s.QueryObservations(ctx, serviceID, ""); err != nil || len(got) == 0 {
		t.Errorf("%s observations = %d rows, %v; want >0, nil", serviceID, len(got), err)
	}
	if _, found, err := s.Fingerprint(ctx, serviceID, "pkg/seed.go"); err != nil || !found {
		t.Errorf("%s fingerprint = found:%v, %v; want true, nil", serviceID, found, err)
	}
	if got, err := s.KnowledgeCards(ctx, serviceID); err != nil || len(got) == 0 {
		t.Errorf("%s knowledge cards = %d, %v; want >0, nil", serviceID, len(got), err)
	}
	if got, err := s.DocSummaries(ctx, serviceID); err != nil || len(got) == 0 {
		t.Errorf("%s doc summaries = %d, %v; want >0, nil", serviceID, len(got), err)
	}
	if _, found, err := s.GetServiceMeta(ctx, serviceID); err != nil || !found {
		t.Errorf("%s service meta = found:%v, %v; want true, nil", serviceID, found, err)
	}
	if got, err := s.ListCommits(ctx, serviceID); err != nil || len(got) == 0 {
		t.Errorf("%s commits = %d, %v; want >0, nil", serviceID, len(got), err)
	}
	if got, err := s.ClaimsForService(ctx, serviceID, ""); err != nil || len(got) == 0 {
		t.Errorf("%s claims = %d, %v; want >0, nil", serviceID, len(got), err)
	}
	if got, err := s.ClaimLinks(ctx, serviceID, []string{claimID}); err != nil || len(got) == 0 {
		t.Errorf("%s claim links = %d, %v; want >0, nil", serviceID, len(got), err)
	}
	if got, err := s.ClaimEvents(ctx, claimID); err != nil || len(got) == 0 {
		t.Errorf("%s claim events = %d, %v; want >0, nil", serviceID, len(got), err)
	}
	if got, err := s.CompileRuns(ctx, serviceID, ""); err != nil || len(got) == 0 {
		t.Errorf("%s compile runs = %d, %v; want >0, nil", serviceID, len(got), err)
	}
	if got, err := s.TypedEdges(ctx, serviceID); err != nil || len(got) == 0 {
		t.Errorf("%s typed edges = %d, %v; want >0, nil", serviceID, len(got), err)
	}
	if _, found, err := s.TypedEdgePackageFingerprint(ctx, serviceID, "example.com/seed"); err != nil || !found {
		t.Errorf("%s typed-edge fingerprint = found:%v, %v; want true, nil", serviceID, found, err)
	}
}

// assertServiceAbsent asserts every read path returns empty/not-found for
// serviceID — the post-RemoveService (and old-id post-RenameService) shape.
// claim_events are asserted by the callers themselves: a remove clears them
// via the service's claims, while a rename deliberately leaves them alone.
func assertServiceAbsent(ctx context.Context, t *testing.T, s Store, serviceID string) {
	t.Helper()
	if got, err := s.QueryObservations(ctx, serviceID, ""); err != nil || len(got) != 0 {
		t.Errorf("%s observations = %d rows, %v; want 0, nil", serviceID, len(got), err)
	}
	if _, found, err := s.Fingerprint(ctx, serviceID, "pkg/seed.go"); err != nil || found {
		t.Errorf("%s fingerprint = found:%v, %v; want false, nil", serviceID, found, err)
	}
	if got, err := s.KnowledgeCards(ctx, serviceID); err != nil || len(got) != 0 {
		t.Errorf("%s knowledge cards = %d, %v; want 0, nil", serviceID, len(got), err)
	}
	if got, err := s.DocSummaries(ctx, serviceID); err != nil || len(got) != 0 {
		t.Errorf("%s doc summaries = %d, %v; want 0, nil", serviceID, len(got), err)
	}
	if _, found, err := s.GetServiceMeta(ctx, serviceID); err != nil || found {
		t.Errorf("%s service meta = found:%v, %v; want false, nil", serviceID, found, err)
	}
	if got, err := s.ListCommits(ctx, serviceID); err != nil || len(got) != 0 {
		t.Errorf("%s commits = %d, %v; want 0, nil", serviceID, len(got), err)
	}
	if got, err := s.ClaimsForService(ctx, serviceID, ""); err != nil || len(got) != 0 {
		t.Errorf("%s claims = %d, %v; want 0, nil", serviceID, len(got), err)
	}
	if got, err := s.CompileRuns(ctx, serviceID, ""); err != nil || len(got) != 0 {
		t.Errorf("%s compile runs = %d, %v; want 0, nil", serviceID, len(got), err)
	}
	if got, err := s.TypedEdges(ctx, serviceID); err != nil || len(got) != 0 {
		t.Errorf("%s typed edges = %d, %v; want 0, nil", serviceID, len(got), err)
	}
	if _, found, err := s.TypedEdgePackageFingerprint(ctx, serviceID, "example.com/seed"); err != nil || found {
		t.Errorf("%s typed-edge fingerprint = found:%v, %v; want false, nil", serviceID, found, err)
	}
	if got, err := s.ListTypedEdgePackages(ctx, serviceID); err != nil || len(got) != 0 {
		t.Errorf("%s typed-edge packages = %v, %v; want none, nil", serviceID, got, err)
	}
	if got, err := s.ListSourceRefs(ctx, serviceID); err != nil || len(got) != 0 {
		t.Errorf("%s source refs = %v, %v; want none, nil", serviceID, got, err)
	}
}

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
