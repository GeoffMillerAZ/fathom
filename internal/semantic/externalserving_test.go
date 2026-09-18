package semantic

// externalserving_test.go — the "Serving fields" section of the
// findings-loop wave (vision 30 §4 item 4): a claim imported by
// internal/findings/external (PromptVersion "external-import/v1") is
// joined against its store.ExternalFinding cursor row at serve time, and
// AnchorCurrent is recomputed on every call from the CURRENT file bytes —
// never cached, never coerced to false when the file cannot be read.
//
// internal/semantic must stay free of any internal/findings dependency
// (leaf-package layering), so this test seeds the claim and cursor row
// directly through core/store types rather than importing the importer.

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/store"
)

func externalStore(t *testing.T, root string) store.Store {
	t.Helper()
	st := store.NewMemoryStore()
	if err := st.SetServiceMeta(context.Background(), store.ServiceMeta{ServiceID: "svc", Path: root}); err != nil {
		t.Fatalf("SetServiceMeta: %v", err)
	}
	return st
}

func seedExternalClaim(t *testing.T, st store.Store, claimID, anchorPath, anchorDigest string, unanchored bool) {
	t.Helper()
	claim := core.SemanticClaim{
		ClaimID: claimID, ServiceID: "svc",
		Layer: "findings", LayerVersion: "v1", Kind: "finding",
		Subject: "ext:reviewkit.system-review/acme/widget/api/rule-1",
		Rule:    "possible nil dereference", Tier: core.TierPreScan, TierProfile: "engineering",
		Status: core.ClaimFresh, PromptVersion: "external-import/v1", ReproKey: claimID, InputDigest: "d",
		CompiledAt: time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC), Class: core.FindingClassCorrectness,
	}
	var links []core.ClaimLink
	if !unanchored {
		claim.Scope = []string{anchorPath}
		links = append(links, core.ClaimLink{ClaimID: claimID, Rel: core.RelScopes, TargetRef: anchorPath})
	}
	if err := st.PutClaims(context.Background(), []core.SemanticClaim{claim}, links); err != nil {
		t.Fatalf("PutClaims: %v", err)
	}
	ef := core.ExternalFinding{
		ServiceID: "svc", ClaimID: claimID,
		Producer: "reviewkit.system-review", ProducerVersion: "1.0.0",
		SourceID: "rule-1", SourceVersion: 1, PayloadDigest: "pd",
		SourceRevision: "rev123",
		RuleID:         "R-NIL", RuleVersion: "1", Remedy: "add a nil check",
		AnchorPath: anchorPath, AnchorDigest: anchorDigest,
		ObservedAt: time.Date(2026, 9, 1, 10, 0, 0, 0, time.UTC),
		ImportedAt: time.Date(2026, 9, 1, 10, 5, 0, 0, time.UTC),
		Unanchored: unanchored,
	}
	if err := st.PutExternalFinding(context.Background(), ef); err != nil {
		t.Fatalf("PutExternalFinding: %v", err)
	}
}

func TestObligations_ExternalFinding_ServesProvenanceAndAnchorCurrent(t *testing.T) {
	root := t.TempDir()
	path := "internal/foo/bar.go"
	if err := os.MkdirAll(filepath.Join(root, "internal", "foo"), 0o755); err != nil {
		t.Fatal(err)
	}
	content := []byte("package foo\n\nfunc Bar() {}\n")
	if err := os.WriteFile(filepath.Join(root, path), content, 0o644); err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(content)
	digest := hex.EncodeToString(sum[:])

	st := externalStore(t, root)
	seedExternalClaim(t, st, "ext-1", path, digest, false)

	got, err := ObligationsForLayer(context.Background(), st, "svc", path, 0, ObligationLayerAll)
	if err != nil {
		t.Fatalf("ObligationsForLayer: %v", err)
	}
	if len(got.Rules) != 1 {
		t.Fatalf("Rules = %+v, want exactly one", got.Rules)
	}
	ob := got.Rules[0]
	if ob.Producer != "reviewkit.system-review" {
		t.Errorf("Producer = %q", ob.Producer)
	}
	if ob.RuleID != "R-NIL" {
		t.Errorf("RuleID = %q", ob.RuleID)
	}
	if ob.SourceID != "rule-1" {
		t.Errorf("SourceID = %q", ob.SourceID)
	}
	if ob.Class != string(core.FindingClassCorrectness) {
		t.Errorf("Class = %q", ob.Class)
	}
	if ob.Remedy != "add a nil check" {
		t.Errorf("Remedy = %q", ob.Remedy)
	}
	if ob.ObservedAt != "2026-09-01T10:00:00Z" {
		t.Errorf("ObservedAt = %q", ob.ObservedAt)
	}
	if ob.SourceRevision != "rev123" {
		t.Errorf("SourceRevision = %q", ob.SourceRevision)
	}
	if ob.Unanchored {
		t.Error("Unanchored = true, want false")
	}
	if ob.AnchorCurrent == nil || !*ob.AnchorCurrent {
		t.Fatalf("AnchorCurrent = %v, want *true (file unchanged)", ob.AnchorCurrent)
	}
}

func TestObligations_ExternalFinding_AnchorCurrentFalseWhenFileMoved(t *testing.T) {
	root := t.TempDir()
	path := "internal/foo/bar.go"
	if err := os.MkdirAll(filepath.Join(root, "internal", "foo"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, path), []byte("package foo\n\nfunc Bar() { /* changed */ }\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	st := externalStore(t, root)
	// AnchorDigest deliberately does not match the current file's hash.
	seedExternalClaim(t, st, "ext-2", path, "stale-digest-from-import-time", false)

	got, err := ObligationsForLayer(context.Background(), st, "svc", path, 0, ObligationLayerAll)
	if err != nil {
		t.Fatalf("ObligationsForLayer: %v", err)
	}
	if len(got.Rules) != 1 {
		t.Fatalf("Rules = %+v, want exactly one", got.Rules)
	}
	if got.Rules[0].AnchorCurrent == nil || *got.Rules[0].AnchorCurrent {
		t.Errorf("AnchorCurrent = %v, want *false (code moved since observed)", got.Rules[0].AnchorCurrent)
	}
}

func TestObligations_ExternalFinding_AnchorCurrentNilWhenUnreadable(t *testing.T) {
	root := t.TempDir() // the anchor file is never written under root
	st := externalStore(t, root)
	seedExternalClaim(t, st, "ext-3", "internal/never/written.go", "some-digest", false)

	got, err := ObligationsForLayer(context.Background(), st, "svc", "internal/never/written.go", 0, ObligationLayerAll)
	if err != nil {
		t.Fatalf("ObligationsForLayer: %v", err)
	}
	if len(got.Rules) != 1 {
		t.Fatalf("Rules = %+v, want exactly one", got.Rules)
	}
	if got.Rules[0].AnchorCurrent != nil {
		t.Errorf("AnchorCurrent = %v, want nil (unreadable, never coerced to false)", got.Rules[0].AnchorCurrent)
	}
}

// TestObligations_ExternalFinding_UnanchoredNeverGetsAnchorCurrent isolates
// fillExternalFinding's join logic: a claim whose ExternalFinding cursor row
// is Unanchored always serves with AnchorCurrent nil, regardless of what its
// own Scope happens to be. (Real unanchored process findings carry an empty
// Scope per FL-5 — ClaimFromEvent's own coverage proves that end-to-end in
// internal/findings/external; this test scopes the claim to "**" purely so
// ObligationsForLayer's file-scoped match has something to serve, keeping
// the two concerns — "is this claim served at all" and "does a served
// external claim's Unanchored flag suppress AnchorCurrent" — independently
// testable.)
func TestObligations_ExternalFinding_UnanchoredNeverGetsAnchorCurrent(t *testing.T) {
	root := t.TempDir()
	st := externalStore(t, root)
	claim := core.SemanticClaim{
		ClaimID: "ext-4", ServiceID: "svc",
		Layer: "findings", LayerVersion: "v1", Kind: "finding",
		Subject: "ext:human.review/acme/widget/api/process-1",
		Rule:    "the release process has no rollback step", Tier: core.TierSMEStated, TierProfile: "engineering",
		Scope:  []string{"**"},
		Status: core.ClaimFresh, PromptVersion: "external-import/v1", ReproKey: "ext-4", InputDigest: "d",
		CompiledAt: time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC),
	}
	links := []core.ClaimLink{{ClaimID: "ext-4", Rel: core.RelScopes, TargetRef: "**"}}
	if err := st.PutClaims(context.Background(), []core.SemanticClaim{claim}, links); err != nil {
		t.Fatalf("PutClaims: %v", err)
	}
	ef := core.ExternalFinding{
		ServiceID: "svc", ClaimID: "ext-4",
		Producer: "human.review", SourceID: "process-1", SourceVersion: 1,
		Unanchored: true,
	}
	if err := st.PutExternalFinding(context.Background(), ef); err != nil {
		t.Fatalf("PutExternalFinding: %v", err)
	}

	got, err := ObligationsForLayer(context.Background(), st, "svc", "any/file.go", 0, ObligationLayerAll)
	if err != nil {
		t.Fatalf("ObligationsForLayer: %v", err)
	}
	if len(got.Rules) != 1 {
		t.Fatalf("Rules = %+v, want exactly one", got.Rules)
	}
	if !got.Rules[0].Unanchored {
		t.Error("Unanchored = false, want true")
	}
	if got.Rules[0].AnchorCurrent != nil {
		t.Errorf("AnchorCurrent = %v, want nil for an unanchored finding", got.Rules[0].AnchorCurrent)
	}
}

// TestObligations_ExternalFinding_ServiceIsolation is plan FL-13 (vision 30
// §3.3 item 5): a claim imported for service A is invisible to
// ObligationsForLayer for service B — the claims store's existing
// service_id scoping, exercised end-to-end through the exact layer:"all"
// call the findings-loop hooks use.
func TestObligations_ExternalFinding_ServiceIsolation(t *testing.T) {
	root := t.TempDir()
	path := "internal/foo/bar.go"
	st := store.NewMemoryStore()
	if err := st.SetServiceMeta(context.Background(), store.ServiceMeta{ServiceID: "svc-A", Path: root}); err != nil {
		t.Fatalf("SetServiceMeta A: %v", err)
	}
	if err := st.SetServiceMeta(context.Background(), store.ServiceMeta{ServiceID: "svc-B", Path: root}); err != nil {
		t.Fatalf("SetServiceMeta B: %v", err)
	}

	claim := core.SemanticClaim{
		ClaimID: "ext-iso-1", ServiceID: "svc-A",
		Layer: "findings", LayerVersion: "v1", Kind: "finding",
		Subject: "ext:reviewkit.system-review/acme/widget/api/rule-iso",
		Rule:    "possible nil dereference", Scope: []string{path},
		Tier: core.TierPreScan, TierProfile: "engineering",
		Status: core.ClaimFresh, PromptVersion: "external-import/v1", ReproKey: "ext-iso-1", InputDigest: "d",
		CompiledAt: time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC),
	}
	links := []core.ClaimLink{{ClaimID: "ext-iso-1", Rel: core.RelScopes, TargetRef: path}}
	if err := st.PutClaims(context.Background(), []core.SemanticClaim{claim}, links); err != nil {
		t.Fatalf("PutClaims: %v", err)
	}
	if err := st.PutExternalFinding(context.Background(), core.ExternalFinding{
		ServiceID: "svc-A", ClaimID: "ext-iso-1", Producer: "reviewkit.system-review",
		SourceID: "rule-iso", SourceVersion: 1, AnchorPath: path,
	}); err != nil {
		t.Fatalf("PutExternalFinding: %v", err)
	}

	gotA, err := ObligationsForLayer(context.Background(), st, "svc-A", path, 0, ObligationLayerAll)
	if err != nil {
		t.Fatalf("ObligationsForLayer(svc-A): %v", err)
	}
	if len(gotA.Rules) != 1 {
		t.Fatalf("svc-A Rules = %+v, want exactly one (its own claim)", gotA.Rules)
	}

	gotB, err := ObligationsForLayer(context.Background(), st, "svc-B", path, 0, ObligationLayerAll)
	if err != nil {
		t.Fatalf("ObligationsForLayer(svc-B): %v", err)
	}
	if len(gotB.Rules) != 0 {
		t.Fatalf("svc-B Rules = %+v, want none — a claim imported for svc-A must be invisible to svc-B", gotB.Rules)
	}
}

// TestObligations_NoExternalClaims_PayloadByteIdentical pins that a
// payload with no external claims never serves any of the new fields —
// existing consumers see exactly what they always have.
func TestObligations_NoExternalClaims_PayloadByteIdentical(t *testing.T) {
	root := t.TempDir()
	st := externalStore(t, root)
	seedRoleClaim(t, st, "gov-1", "obligations", "Wire every extractor into fusion.New.",
		[]string{"internal/extractor/**"}, core.ClaimFresh, "CLAUDE.md#L4-L4", "")

	got, err := ObligationsFor(context.Background(), st, "svc", "internal/extractor/goextract.go", 0)
	if err != nil {
		t.Fatalf("ObligationsFor: %v", err)
	}
	if len(got.Rules) != 1 {
		t.Fatalf("Rules = %+v", got.Rules)
	}
	ob := got.Rules[0]
	if ob.Producer != "" || ob.RuleID != "" || ob.SourceID != "" || ob.Class != "" || ob.Remedy != "" ||
		ob.ObservedAt != "" || ob.SourceRevision != "" || ob.AnchorCurrent != nil || ob.Unanchored {
		t.Errorf("native obligation carries findings-loop fields: %+v", ob)
	}
}
