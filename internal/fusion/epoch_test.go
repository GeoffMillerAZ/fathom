package fusion_test

// epoch_test.go — regression for dogfood O9: extractor-semantics changes must
// invalidate stored fingerprints, or hash-gated files keep observations
// produced by the OLD extractor forever (the H1 callgraph fixes were
// invisible on the fleet store until forced). The fingerprint is therefore
// salted with fusion.ExtractorEpoch.

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/connector"
	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/fusion"
	"github.com/geoffmilleraz/fathom/internal/store"
)

func TestIngest_ExtractorEpochInvalidatesOldFingerprints(t *testing.T) {
	conn := connector.NewFauxConnector()
	st := store.NewMemoryStore()
	p := fusion.New(conn, st, defaultCfg())
	ctx := context.Background()

	rep1, err := p.Ingest(ctx, paymentSvc)
	if err != nil {
		t.Fatalf("Ingest: %v", err)
	}
	if rep1.Skipped != 0 {
		t.Fatalf("first ingest skipped %d files, want 0", rep1.Skipped)
	}

	refs, err := conn.Discover(ctx, paymentSvc)
	if err != nil || len(refs) == 0 {
		t.Fatalf("Discover: %v (%d refs)", err, len(refs))
	}
	ref := refs[0]
	content, err := conn.Fetch(ctx, ref)
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	plain := sha256.Sum256(content)
	plainHex := hex.EncodeToString(plain[:])

	// The stored fingerprint must NOT be the plain content hash — the epoch
	// salt is load-bearing.
	stored, found, err := st.Fingerprint(ctx, paymentSvc.ID, ref.Path)
	if err != nil || !found {
		t.Fatalf("Fingerprint after ingest: found=%v err=%v", found, err)
	}
	if stored == plainHex {
		t.Fatal("stored fingerprint equals the unsalted content hash — ExtractorEpoch is not in the fingerprint")
	}

	// Salted fingerprints are stable: a re-ingest skips everything.
	rep2, err := p.Ingest(ctx, paymentSvc)
	if err != nil {
		t.Fatalf("re-Ingest: %v", err)
	}
	if rep2.Skipped == 0 {
		t.Fatal("re-ingest skipped nothing — salted fingerprint is not stable")
	}

	// Simulate a store written before the epoch salt existed (or under an
	// older epoch): the plain content hash. The next ingest must RE-EXTRACT
	// that file rather than skip it.
	if err := st.SetFingerprint(ctx, paymentSvc.ID, ref.Path, plainHex); err != nil {
		t.Fatalf("SetFingerprint: %v", err)
	}
	rep3, err := p.Ingest(ctx, paymentSvc)
	if err != nil {
		t.Fatalf("post-downgrade Ingest: %v", err)
	}
	if rep3.Skipped != rep2.Skipped-1 {
		t.Errorf("pre-epoch fingerprint was not invalidated: skipped %d, want %d", rep3.Skipped, rep2.Skipped-1)
	}
	after, found, err := st.Fingerprint(ctx, paymentSvc.ID, ref.Path)
	if err != nil || !found {
		t.Fatalf("Fingerprint after re-extract: found=%v err=%v", found, err)
	}
	if after != stored {
		t.Errorf("re-extracted fingerprint %q, want the salted value %q (deterministic)", after, stored)
	}
}

// TestIngest_ExtractorEpoch_CUEDepthChangeForcesReExtraction is LT-5: the CUE
// extractor v2 (nested-field recursion, docs/plans/lang-truth.md) changed
// extraction semantics for every already-indexed .cue file, so
// ExtractorEpoch was bumped "2"→"3". The seeded fingerprint below is salted
// with epoch "2" — the TRUE prior epoch — deliberately: it pins the bump
// itself, so a merge-conflict revert of ExtractorEpoch back to "2" makes the
// seeded fingerprint match again, the file is skipped, and this test fails
// loudly. (Seeding an older epoch like "1" would mismatch under both "2" and
// "3" and let a revert slip through green.) The re-extraction must also
// surface a nested dotted field, proving the epoch bump and the CUE depth
// fix are wired together end to end.
func TestIngest_ExtractorEpoch_CUEDepthChangeForcesReExtraction(t *testing.T) {
	dir := t.TempDir()
	cueSrc := []byte("package svc\n\n#Layer: {\n\tbudgets: {\n\t\tmax_usd_per_run: number\n\t}\n}\n")
	if err := os.WriteFile(filepath.Join(dir, "schema.cue"), cueSrc, 0o644); err != nil {
		t.Fatalf("write schema.cue: %v", err)
	}

	conn := connector.NewLocalConnector()
	st := store.NewMemoryStore()
	svc := core.ServiceRef{ID: "cue-epoch-svc", Path: dir}
	ctx := context.Background()

	oldSalted := sha256.Sum256(append(append([]byte{}, cueSrc...), []byte("\x00fathom-extractor-epoch:2")...))
	if err := st.SetFingerprint(ctx, svc.ID, "schema.cue", hex.EncodeToString(oldSalted[:])); err != nil {
		t.Fatalf("SetFingerprint: %v", err)
	}

	p := fusion.New(conn, st, defaultCfg())
	rep, err := p.Ingest(ctx, svc)
	if err != nil {
		t.Fatalf("Ingest: %v", err)
	}
	if rep.Skipped != 0 {
		t.Fatal("schema.cue was skipped (a pre-epoch-3 fingerprint was honored) — ExtractorEpoch was not bumped for the CUE depth change")
	}

	obs, err := st.QueryObservations(ctx, svc.ID, "")
	if err != nil {
		t.Fatalf("QueryObservations: %v", err)
	}
	var sawNested bool
	for _, o := range obs {
		if o.Symbol == "#Layer.budgets.max_usd_per_run" {
			sawNested = true
		}
	}
	if !sawNested {
		t.Error("nested CUE field #Layer.budgets.max_usd_per_run not observed after forced re-extraction")
	}
}

// TestIngest_ExtractorEpoch_ToolIdentityForcesReExtraction is TI-8: the
// ADR-021 §2 tool-identity classifier generation changed yaml-family
// extraction semantics, so ExtractorEpoch was bumped "3"→"4". The seeded
// fingerprint below is salted with epoch "3" — the TRUE prior epoch —
// deliberately (the Wave-1 lesson, docs/plans/lang-truth.md): the test must
// fail if the bump reverts, which a seed of "2" or "1" would not catch
// (either would already mismatch and re-extract regardless of whether "4"
// ever existed).
//
// kustomization.yaml is the sharpest proof available: pre-classifier, it was
// dispatched to the generic k8s extractor, which only emits a workload node
// when BOTH kind and metadata.name are present — kustomization.yaml has no
// metadata.name, so it produced exactly zero symbols (silently). Forced
// re-extraction under the new KustomizeExtractor must surface real resource
// and image symbols from the very same bytes.
func TestIngest_ExtractorEpoch_ToolIdentityForcesReExtraction(t *testing.T) {
	dir := t.TempDir()
	kustomizeSrc := []byte("apiVersion: kustomize.config.k8s.io/v1beta1\n" +
		"kind: Kustomization\n" +
		"resources:\n  - deployment.yaml\n  - service.yaml\n" +
		"images:\n  - name: myapp\n    newTag: \"1.2.3\"\n")
	if err := os.WriteFile(filepath.Join(dir, "kustomization.yaml"), kustomizeSrc, 0o644); err != nil {
		t.Fatalf("write kustomization.yaml: %v", err)
	}

	conn := connector.NewLocalConnector()
	st := store.NewMemoryStore()
	svc := core.ServiceRef{ID: "kustomize-epoch-svc", Path: dir}
	ctx := context.Background()

	oldSalted := sha256.Sum256(append(append([]byte{}, kustomizeSrc...), []byte("\x00fathom-extractor-epoch:3")...))
	if err := st.SetFingerprint(ctx, svc.ID, "kustomization.yaml", hex.EncodeToString(oldSalted[:])); err != nil {
		t.Fatalf("SetFingerprint: %v", err)
	}

	p := fusion.New(conn, st, defaultCfg())
	rep, err := p.Ingest(ctx, svc)
	if err != nil {
		t.Fatalf("Ingest: %v", err)
	}
	if rep.Skipped != 0 {
		t.Fatal("kustomization.yaml was skipped (a pre-epoch-4 fingerprint was honored) — ExtractorEpoch was not bumped for the tool-identity change")
	}

	obs, err := st.QueryObservations(ctx, svc.ID, "")
	if err != nil {
		t.Fatalf("QueryObservations: %v", err)
	}
	var sawResource, sawImage bool
	for _, o := range obs {
		if o.SourceKind == core.KindKustomizeResource && o.Symbol == "deployment.yaml" {
			sawResource = true
		}
		if o.SourceKind == core.KindKustomizeImage && o.Symbol == "myapp" {
			sawImage = true
		}
	}
	if !sawResource || !sawImage {
		t.Errorf("kustomization.yaml re-extraction did not surface resource/image symbols (resource=%v image=%v) — tool-identity classifier not wired", sawResource, sawImage)
	}
}
