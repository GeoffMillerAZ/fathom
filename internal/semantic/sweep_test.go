package semantic

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/store"
)

// seedClaim writes one claim + its citation link built against docContent.
func seedClaim(t *testing.T, st store.Store, id, subject string, status core.ClaimStatus, docPath string, docContent []byte, start, end int) {
	t.Helper()
	hash, ok := core.HashLines(docContent, start, end)
	if !ok {
		t.Fatalf("seed cite %d-%d does not resolve", start, end)
	}
	claim := core.SemanticClaim{
		ClaimID: id, ServiceID: "svc",
		Layer: LayerObligations, LayerVersion: "v1", Kind: KindObligation,
		Subject: subject, Rule: "rule for " + id, Scope: []string{"**"},
		Tier: core.TierDocClaimed, TierProfile: "engineering", Status: status,
		PromptVersion: obligationsPromptVersion(), ReproKey: id, InputDigest: "d-" + id,
		CompiledAtWatermark: "manifest:aaa", CompiledAt: time.Date(2026, 7, 7, 0, 0, 0, 0, time.UTC),
	}
	links := []core.ClaimLink{{ClaimID: id, Rel: core.RelCites, TargetRef: core.CiteRef(docPath, start, end), TargetHash: hash}}
	if err := st.PutClaims(context.Background(), []core.SemanticClaim{claim}, links); err != nil {
		t.Fatalf("seed PutClaims: %v", err)
	}
}

func TestSweep_FlipsAndHeals(t *testing.T) {
	ctx := context.Background()
	st := store.NewMemoryStore()
	doc := []byte("line one\nline two\nline three\n")

	seedClaim(t, st, "c-fresh", "d.md#a", core.ClaimFresh, "d.md", doc, 1, 1)
	seedClaim(t, st, "c-drift", "d.md#b", core.ClaimFresh, "d.md", doc, 2, 2)
	seedClaim(t, st, "c-gone", "gone.md#a", core.ClaimFresh, "gone.md", []byte("x\n"), 1, 1)
	seedClaim(t, st, "c-superseded", "d.md#c", core.ClaimSuperseded, "d.md", doc, 3, 3)

	// Current corpus: d.md line 2 edited; gone.md deleted.
	edited := []byte("line one\nline TWO edited\nline three\n")
	fetch := func(path string) ([]byte, bool) {
		if path == "d.md" {
			return edited, true
		}
		return nil, false
	}

	rep, err := Sweep(ctx, st, "svc", fetch)
	if err != nil {
		t.Fatalf("Sweep: %v", err)
	}
	if rep.ClaimsChecked != 3 {
		t.Errorf("checked %d claims, want 3 (superseded is terminal, never swept)", rep.ClaimsChecked)
	}
	if rep.Fresh != 1 || rep.Stale != 1 || rep.Contradicted != 1 || rep.Flipped != 2 {
		t.Errorf("report %+v, want fresh/stale/contradicted 1/1/1, flipped 2", rep)
	}

	status := statusByID(t, st)
	if status["c-fresh"] != core.ClaimFresh {
		t.Errorf("c-fresh = %s", status["c-fresh"])
	}
	if status["c-drift"] != core.ClaimStale {
		t.Errorf("c-drift = %s, want stale (cited line changed)", status["c-drift"])
	}
	if status["c-gone"] != core.ClaimContradicted {
		t.Errorf("c-gone = %s, want contradicted (file gone)", status["c-gone"])
	}
	if status["c-superseded"] != core.ClaimSuperseded {
		t.Errorf("c-superseded = %s, must not be touched", status["c-superseded"])
	}

	// Healing: restore the original doc → the stale claim goes fresh again.
	rep2, err := Sweep(ctx, st, "svc", func(path string) ([]byte, bool) {
		if path == "d.md" {
			return doc, true
		}
		return nil, false
	})
	if err != nil {
		t.Fatalf("second Sweep: %v", err)
	}
	if rep2.Flipped != 1 {
		t.Errorf("second sweep flipped %d, want 1 (stale→fresh heal)", rep2.Flipped)
	}
	if s := statusByID(t, st)["c-drift"]; s != core.ClaimFresh {
		t.Errorf("c-drift after restore = %s, want fresh", s)
	}
}

func TestSweep_OutOfRangeCitationContradicts(t *testing.T) {
	ctx := context.Background()
	st := store.NewMemoryStore()
	doc := []byte(strings.Repeat("l\n", 10)) // 10 lines
	seedClaim(t, st, "c-range", "d.md#a", core.ClaimFresh, "d.md", doc, 9, 10)

	// Doc shrinks to 3 lines: the range no longer resolves.
	rep, err := Sweep(ctx, st, "svc", func(string) ([]byte, bool) { return []byte("a\nb\nc"), true })
	if err != nil {
		t.Fatalf("Sweep: %v", err)
	}
	if rep.Contradicted != 1 {
		t.Errorf("report %+v, want 1 contradicted", rep)
	}
	if s := statusByID(t, st)["c-range"]; s != core.ClaimContradicted {
		t.Errorf("c-range = %s, want contradicted", s)
	}
}

func statusByID(t *testing.T, st store.Store) map[string]core.ClaimStatus {
	t.Helper()
	claims, err := st.ClaimsForService(context.Background(), "svc", "")
	if err != nil {
		t.Fatalf("ClaimsForService: %v", err)
	}
	out := map[string]core.ClaimStatus{}
	for _, cl := range claims {
		out[cl.ClaimID] = cl.Status
	}
	return out
}

// ─── FT-6 DriftedFindings: bounded fingerprint reads on the serve hot path ───

// countingFPStore wraps MemoryStore, counting Fingerprint reads.
type countingFPStore struct {
	*store.MemoryStore
	fpCalls int
}

func (c *countingFPStore) Fingerprint(ctx context.Context, serviceID, sourceRef string) (string, bool, error) {
	c.fpCalls++
	return c.MemoryStore.Fingerprint(ctx, serviceID, sourceRef)
}

// seedFindingWithScopeFP writes one finding-kind claim whose scope-fp links
// anchor the given files at the given fingerprint values.
func seedFindingWithScopeFP(t *testing.T, st store.Store, id string, anchors map[string]string) {
	t.Helper()
	claim := core.SemanticClaim{
		ClaimID: id, ServiceID: "svc",
		Layer: LayerFindings, LayerVersion: "v1", Kind: KindFinding,
		Subject: "findings/" + id + ".md#s", Rule: "finding " + id, Scope: []string{"**/*.go"},
		Tier: core.TierSMEStated, TierProfile: "engineering", Status: core.ClaimFresh,
		PromptVersion: findingsPromptVersion(), ReproKey: id, InputDigest: "d-" + id,
		CompiledAtWatermark: "manifest:aaa", CompiledAt: time.Date(2026, 7, 7, 0, 0, 0, 0, time.UTC),
	}
	links := []core.ClaimLink{{ClaimID: id, Rel: core.RelCites, TargetRef: "findings/" + id + ".md#L1-L1", TargetHash: "h"}}
	for f, fp := range anchors {
		links = append(links, core.ClaimLink{ClaimID: id, Rel: core.RelScopeFP, TargetRef: f, TargetHash: fp})
	}
	if err := st.PutClaims(context.Background(), []core.SemanticClaim{claim}, links); err != nil {
		t.Fatalf("seed PutClaims: %v", err)
	}
}

// TestDriftedFindings_QueriesEachFileOnce: DriftedFindings runs on EVERY
// ObligationsFor serve (the edit-hook hot path). N claims whose broad-but-
// legal scopes anchor the same files must not multiply store reads — each
// distinct file is fingerprint-queried at most once per call.
func TestDriftedFindings_QueriesEachFileOnce(t *testing.T) {
	ctx := context.Background()
	st := &countingFPStore{MemoryStore: store.NewMemoryStore()}

	shared := map[string]string{"mod/a.go": "fp-a", "mod/b.go": "fp-b"}
	for f, fp := range shared {
		if err := st.SetFingerprint(ctx, "svc", f, fp); err != nil {
			t.Fatalf("SetFingerprint: %v", err)
		}
	}
	// Three finding claims all anchoring the SAME two files.
	seedFindingWithScopeFP(t, st, "f1", shared)
	seedFindingWithScopeFP(t, st, "f2", shared)
	seedFindingWithScopeFP(t, st, "f3", shared)

	drifted, err := DriftedFindings(ctx, st, "svc")
	if err != nil {
		t.Fatalf("DriftedFindings: %v", err)
	}
	if len(drifted) != 0 {
		t.Errorf("drifted = %v, want none (fingerprints unchanged)", drifted)
	}
	if st.fpCalls > 2 {
		t.Errorf("Fingerprint queried %d times for 2 distinct files across 3 claims, want ≤2 (per-call cache)", st.fpCalls)
	}

	// Sanity: drift detection still works through the cache — change one file.
	if err := st.SetFingerprint(ctx, "svc", "mod/a.go", "fp-a-changed"); err != nil {
		t.Fatalf("SetFingerprint change: %v", err)
	}
	drifted, err = DriftedFindings(ctx, st, "svc")
	if err != nil {
		t.Fatalf("DriftedFindings after change: %v", err)
	}
	if len(drifted) != 3 || !drifted["f1"] || !drifted["f2"] || !drifted["f3"] {
		t.Errorf("drifted = %v, want all three claims (shared anchor changed)", drifted)
	}
}

// TestDriftedFindings_SurvivesBrokenProjectPack is critic finding 1 on the
// LC slice: layerIsDriftTracked resolves through the union loader, and a
// half-authored project pack under the service's .fathom/layers/ used to
// hard-fail the WHOLE union — silently disabling code_drifted for the
// embedded findings layer too. With the lenient union (packs.go), drift
// tracking for embedded findings must keep working while an invalid pack
// dir is present.
func TestDriftedFindings_SurvivesBrokenProjectPack(t *testing.T) {
	ctx := context.Background()
	st := store.NewMemoryStore()

	// The service's root contains a broken pack — exactly the state a
	// half-finished `mkdir .fathom/layers/wip` leaves a repo in.
	root := t.TempDir()
	brokenDir := root + "/.fathom/layers/wip"
	if err := os.MkdirAll(brokenDir, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := os.WriteFile(brokenDir+"/layer.cue", []byte("not { valid ] cue"), 0o644); err != nil {
		t.Fatalf("write broken layer.cue: %v", err)
	}
	if err := st.SetServiceMeta(ctx, store.ServiceMeta{ServiceID: "svc", Path: root}); err != nil {
		t.Fatalf("SetServiceMeta: %v", err)
	}

	// A findings claim whose scoped code HAS drifted.
	if err := st.SetFingerprint(ctx, "svc", "mod/a.go", "fp-a-changed"); err != nil {
		t.Fatalf("SetFingerprint: %v", err)
	}
	seedFindingWithScopeFP(t, st, "f1", map[string]string{"mod/a.go": "fp-a-original"})

	drifted, err := DriftedFindings(ctx, st, "svc")
	if err != nil {
		t.Fatalf("DriftedFindings with a broken project pack present: %v", err)
	}
	if !drifted["f1"] {
		t.Fatalf("drifted = %v, want f1 — a broken, unrelated project pack silently disabled drift tracking for the embedded findings layer", drifted)
	}
}
