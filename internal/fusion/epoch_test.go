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
	"testing"

	"github.com/geoffmilleraz/fathom/internal/connector"
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
