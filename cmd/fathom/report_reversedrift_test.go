package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/store"
)

// reverseDriftFixture seeds a memory store with two exported Go symbols in
// different packages — one under a path a spec-bindings claim's Scope
// covers, one not — plus an unexported symbol (must never appear in the
// surface at all) so the governed/ungoverned split has something real to
// compute over without any git/LLM shell-out (E6-1 is a read-only join over
// two already-tested mechanisms: apisurface.Surface + ClaimsForService).
func reverseDriftFixture(t *testing.T, st store.Store, service string) {
	t.Helper()
	ctx := context.Background()
	obs := []core.Observation{
		{
			ServiceID: service, Symbol: "Governed", Package: "pkg/governed",
			SourceKind: core.KindGoFunc, SourceRef: "pkg/governed/a.go", Line: 5,
			Claim: "func Governed()", EvidenceTier: core.TierVerified,
			IngestedAt: time.Unix(0, 0),
		},
		{
			ServiceID: service, Symbol: "Ungoverned", Package: "pkg/loose",
			SourceKind: core.KindGoFunc, SourceRef: "pkg/loose/b.go", Line: 9,
			Claim: "func Ungoverned()", EvidenceTier: core.TierVerified,
			IngestedAt: time.Unix(0, 0),
		},
		{
			ServiceID: service, Symbol: "unexported", Package: "pkg/loose",
			SourceKind: core.KindGoFunc, SourceRef: "pkg/loose/b.go", Line: 20,
			Claim: "func unexported()", EvidenceTier: core.TierVerified,
			IngestedAt: time.Unix(0, 0),
		},
		{
			// Exported but in a _test.go file: test helpers are not public
			// API surface and must be excluded before the drift math — on
			// the real fleet store 68% of the raw exported surface was
			// _test.go symbols, drowning the headline (critic finding 1).
			ServiceID: service, Symbol: "FixtureHelper", Package: "pkg/loose",
			SourceKind: core.KindGoFunc, SourceRef: "pkg/loose/b_test.go", Line: 3,
			Claim: "func FixtureHelper()", EvidenceTier: core.TierVerified,
			IngestedAt: time.Unix(0, 0),
		},
	}
	if err := st.PutObservations(ctx, obs); err != nil {
		t.Fatalf("PutObservations: %v", err)
	}
	claim := core.SemanticClaim{
		ClaimID:   core.ClaimID("spec-bindings", "spec-binding", "E-1", "test"),
		ServiceID: service,
		Layer:     "spec-bindings", LayerVersion: "v1", Kind: "spec-binding",
		Subject: "E-1", Rule: "pkg/governed is governed by plan row E-1",
		Scope: []string{"pkg/governed/**"},
		Tier:  core.TierDocClaimed, Status: core.ClaimFresh,
	}
	if err := st.PutClaims(ctx, []core.SemanticClaim{claim}, nil); err != nil {
		t.Fatalf("PutClaims: %v", err)
	}
}

func TestRunReverseDriftReport(t *testing.T) {
	st := store.NewMemoryStore()
	reverseDriftFixture(t, st, "svc")

	doc, err := runReverseDriftReport(context.Background(), st, "svc")
	if err != nil {
		t.Fatalf("runReverseDriftReport: %v", err)
	}

	if !strings.Contains(doc, "reverse-drift findings — svc") {
		t.Errorf("doc missing header: %s", doc)
	}
	if !strings.Contains(doc, "| public API surface (total) | 2 |") {
		t.Errorf("doc missing surface-total row (want 2, unexported symbol excluded): %s", doc)
	}
	if !strings.Contains(doc, "| governed (covered by a spec-binding) | 1 |") {
		t.Errorf("doc missing governed row (want 1): %s", doc)
	}
	if !strings.Contains(doc, "| ungoverned (no spec-binding covers it) | 1 |") {
		t.Errorf("doc missing ungoverned row (want 1): %s", doc)
	}
	if !strings.Contains(doc, "pkg/loose/b.go:9") || !strings.Contains(doc, "pkg/loose.Ungoverned") {
		t.Errorf("doc missing the ungoverned symbol's anchor: %s", doc)
	}
	if strings.Contains(doc, "pkg/governed/a.go") {
		t.Errorf("doc lists a governed symbol in the ungoverned section: %s", doc)
	}
	if strings.Contains(doc, "unexported") {
		t.Errorf("doc leaks an unexported symbol into the public surface: %s", doc)
	}
	if strings.Contains(doc, "b_test.go") || strings.Contains(doc, "FixtureHelper") {
		t.Errorf("doc counts a _test.go symbol as public API surface: %s", doc)
	}
	if strings.Contains(doc, "No spec-bindings claims") {
		t.Errorf("no-claims banner must not appear when served claims exist: %s", doc)
	}

	// Determinism: re-running against an unchanged store reproduces the
	// identical bytes.
	doc2, err := runReverseDriftReport(context.Background(), st, "svc")
	if err != nil {
		t.Fatalf("second runReverseDriftReport: %v", err)
	}
	if doc != doc2 {
		t.Errorf("report is not byte-identical across runs:\nfirst:\n%s\nsecond:\n%s", doc, doc2)
	}
}

// TestRunReverseDriftReport_NoClaimsBanner is critic finding 1 (the C4-5
// honesty class): against a store with ZERO spec-bindings claims — today's
// real fleet state — "coverage 0.0%" plus a full ungoverned listing reads
// as measured drift when nothing was measured. The report must banner that
// the layer has not been compiled and that the list is the whole surface.
func TestRunReverseDriftReport_NoClaimsBanner(t *testing.T) {
	st := store.NewMemoryStore()
	ctx := context.Background()
	if err := st.PutObservations(ctx, []core.Observation{{
		ServiceID: "svc3", Symbol: "Lonely", Package: "pkg/x",
		SourceKind: core.KindGoFunc, SourceRef: "pkg/x/a.go", Line: 5,
		Claim: "func Lonely()", EvidenceTier: core.TierVerified,
		IngestedAt: time.Unix(0, 0),
	}}); err != nil {
		t.Fatalf("PutObservations: %v", err)
	}
	// No claims at all.
	doc, err := runReverseDriftReport(ctx, st, "svc3")
	if err != nil {
		t.Fatalf("runReverseDriftReport: %v", err)
	}
	if !strings.Contains(doc, "No spec-bindings claims") {
		t.Errorf("doc missing the no-claims banner: %s", doc)
	}
	if !strings.Contains(doc, "fathom compile --layer spec-bindings") {
		t.Errorf("banner missing the remedy command: %s", doc)
	}
	if !strings.Contains(doc, "not measured drift") {
		t.Errorf("banner missing the 'whole surface, not measured drift' honesty marker: %s", doc)
	}
	// The surface is still listed (it is real data) — only its framing changes.
	if !strings.Contains(doc, "pkg/x.Lonely") {
		t.Errorf("surface listing missing under the banner: %s", doc)
	}
}

// TestRunReverseDriftReport_FullyGoverned asserts the empty-ungoverned path
// renders honestly rather than an empty/misleading section.
func TestRunReverseDriftReport_FullyGoverned(t *testing.T) {
	st := store.NewMemoryStore()
	ctx := context.Background()
	if err := st.PutObservations(ctx, []core.Observation{{
		ServiceID: "svc2", Symbol: "Governed", Package: "pkg/governed",
		SourceKind: core.KindGoFunc, SourceRef: "pkg/governed/a.go", Line: 5,
		Claim: "func Governed()", EvidenceTier: core.TierVerified,
		IngestedAt: time.Unix(0, 0),
	}}); err != nil {
		t.Fatalf("PutObservations: %v", err)
	}
	claim := core.SemanticClaim{
		ClaimID:   core.ClaimID("spec-bindings", "spec-binding", "E-1", "test"),
		ServiceID: "svc2", Layer: "spec-bindings", LayerVersion: "v1", Kind: "spec-binding",
		Subject: "E-1", Rule: "governed", Scope: []string{"pkg/governed/**"},
		Tier: core.TierDocClaimed, Status: core.ClaimFresh,
	}
	if err := st.PutClaims(ctx, []core.SemanticClaim{claim}, nil); err != nil {
		t.Fatalf("PutClaims: %v", err)
	}

	doc, err := runReverseDriftReport(ctx, st, "svc2")
	if err != nil {
		t.Fatalf("runReverseDriftReport: %v", err)
	}
	if !strings.Contains(doc, "No ungoverned public surface") {
		t.Errorf("doc should say the surface is fully governed: %s", doc)
	}
	if !strings.Contains(doc, "| coverage | 100.0% |") {
		t.Errorf("doc missing 100%% coverage row: %s", doc)
	}
}

// TestReport_ReverseDrift_CLIWiring is the end-to-end path: `fathom report
// --analyzer reverse-drift` over a real ingested fixture, with a
// spec-bindings claim inserted directly into the store afterward (spec-
// bindings claims are normally LLM-compiled by `fathom compile`, which this
// test does not invoke — inserting the claim directly keeps the fixture
// deterministic and fast, matching E6-1's "two already-tested mechanisms"
// framing: only the join here is new, not either input).
func TestReport_ReverseDrift_CLIWiring(t *testing.T) {
	root := t.TempDir()
	// Two separate files: spec-bindings claims are PACKAGE/file scoped, not
	// per-symbol (LC-6), so the governed/ungoverned split must be tested
	// across files, not two symbols in one file the claim would cover
	// together.
	governedSrc := "package fixture\n\n// Governed is covered by a spec-binding.\nfunc Governed() {}\n"
	looseSrc := "package fixture\n\n// Loose is not.\nfunc Loose() {}\n"
	if err := os.WriteFile(filepath.Join(root, "governed.go"), []byte(governedSrc), 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "loose.go"), []byte(looseSrc), 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	storePath := filepath.Join(t.TempDir(), "s.sqlite")
	outDir := filepath.Join(t.TempDir(), "findings")

	var ingestOut, ingestErr bytes.Buffer
	if code := run([]string{"ingest", "--service", "rd-svc", "--path", root, "--store", storePath}, &ingestOut, &ingestErr); code != 0 {
		t.Fatalf("ingest exit = %d, stderr: %s", code, ingestErr.String())
	}

	// Insert the spec-bindings claim directly (see doc comment above).
	st, err := store.Open(storePath)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	claim := core.SemanticClaim{
		ClaimID:   core.ClaimID("spec-bindings", "spec-binding", "E-9", "test"),
		ServiceID: "rd-svc", Layer: "spec-bindings", LayerVersion: "v1", Kind: "spec-binding",
		Subject: "E-9", Rule: "Governed is governed by plan row E-9",
		Scope: []string{"governed.go"}, Tier: core.TierDocClaimed, Status: core.ClaimFresh,
	}
	if err := st.PutClaims(context.Background(), []core.SemanticClaim{claim}, nil); err != nil {
		t.Fatalf("PutClaims: %v", err)
	}
	st.Close()

	var out, errBuf bytes.Buffer
	code := run([]string{
		"report", "--store", storePath, "--service", "rd-svc",
		"--analyzer", "reverse-drift", "--out", outDir,
	}, &out, &errBuf)
	if code != 0 {
		t.Fatalf("report exit = %d, stderr: %s", code, errBuf.String())
	}

	outPath := filepath.Join(outDir, "rd-svc-reverse-drift.md")
	body, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("read %s: %v", outPath, err)
	}
	doc := string(body)
	if !strings.Contains(doc, "reverse-drift findings — rd-svc") {
		t.Errorf("doc missing header: %s", doc)
	}
	if !strings.Contains(doc, "fixture.Loose") || !strings.Contains(doc, "loose.go:4") {
		t.Errorf("doc missing the ungoverned Loose anchor: %s", doc)
	}
	if strings.Contains(doc, "fixture.Governed") {
		t.Errorf("doc should not list the governed symbol as ungoverned: %s", doc)
	}
}
