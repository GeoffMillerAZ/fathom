package fusion_test

import (
	"context"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/connector"
	"github.com/geoffmilleraz/fathom/internal/fusion"
	"github.com/geoffmilleraz/fathom/internal/store"
)

// TestIngest_WithGitLog_FullBody proves the pipeline wiring end-to-end
// (E2-1): a WithGitLog-injected raw git-log string carrying a multi-line
// commit body (subject + blank + prose + blank + a Co-Authored-By trailer)
// lands in the store's commits table with the FULL body intact — not
// truncated to the subject — via the ordinary Ingest → parseGitLog →
// SetCommits path used by the real (non-test) realGitLog.
func TestIngest_WithGitLog_FullBody(t *testing.T) {
	const rawLog = "\x02deadbeef1234567890\x1fAlice\x1f2024-01-15T10:00:00Z\x1fFix auth flow\n\nLonger explanation of the change.\n\nCo-Authored-By: Claude Fable 5 <noreply@anthropic.com>\x03\npkg/auth/auth.go\n\n"

	conn := connector.NewFauxConnector()
	st := store.NewMemoryStore()
	p := fusion.New(conn, st, defaultCfg())
	p.WithGitLog(func(_ string) string { return rawLog })

	ctx := context.Background()
	if _, err := p.Ingest(ctx, paymentSvc); err != nil {
		t.Fatalf("Ingest error: %v", err)
	}

	commits, err := st.ListCommits(ctx, paymentSvc.ID)
	if err != nil {
		t.Fatalf("ListCommits: %v", err)
	}
	if len(commits) != 1 {
		t.Fatalf("expected 1 commit, got %d", len(commits))
	}
	got := commits[0].Message
	if !strings.Contains(got, "\n") {
		t.Errorf("stored message has no newline (still subject-only): %q", got)
	}
	if !strings.Contains(got, "Co-Authored-By: Claude Fable 5 <noreply@anthropic.com>") {
		t.Errorf("stored message missing trailer line: %q", got)
	}
}

// TestIngest_WithGitLog_BackfillOnReingest proves E2-1's "re-ingest
// backfills bodies" claim: a legacy commit row written as subject-only
// (simulating data stored before this change) is fully replaced — full
// multi-line body and all — by the next Ingest, because SetCommits deletes
// and re-inserts a service's ENTIRE commit set on every call (there is no
// separate backfill mechanism to write or forget; the existing replace-all
// contract does the work once the format is widened).
func TestIngest_WithGitLog_BackfillOnReingest(t *testing.T) {
	ctx := context.Background()
	st := store.NewMemoryStore()

	// Seed a legacy, subject-only commit row directly (as if written by the
	// pre-E2-1 pipeline).
	const hash = "deadbeef1234567890"
	if err := st.SetCommits(ctx, paymentSvc.ID, []store.Commit{
		{ServiceID: paymentSvc.ID, Hash: hash, Author: "Alice", Date: 1, Message: "Fix auth flow"},
	}); err != nil {
		t.Fatalf("seed SetCommits: %v", err)
	}
	seeded, err := st.ListCommits(ctx, paymentSvc.ID)
	if err != nil || len(seeded) != 1 {
		t.Fatalf("seed ListCommits: %v (%d rows)", err, len(seeded))
	}
	if strings.Contains(seeded[0].Message, "\n") {
		t.Fatalf("seeded fixture should be subject-only (no newline), got %q", seeded[0].Message)
	}

	// Re-ingest with the new, full-body format for the SAME commit hash.
	const rawLog = "\x02deadbeef1234567890\x1fAlice\x1f2024-01-15T10:00:00Z\x1fFix auth flow\n\nCo-Authored-By: Claude Fable 5 <noreply@anthropic.com>\x03\npkg/auth/auth.go\n\n"
	conn := connector.NewFauxConnector()
	p := fusion.New(conn, st, defaultCfg())
	p.WithGitLog(func(_ string) string { return rawLog })

	if _, err := p.Ingest(ctx, paymentSvc); err != nil {
		t.Fatalf("Ingest error: %v", err)
	}

	after, err := st.ListCommits(ctx, paymentSvc.ID)
	if err != nil {
		t.Fatalf("ListCommits after re-ingest: %v", err)
	}
	if len(after) != 1 {
		t.Fatalf("expected 1 commit after re-ingest, got %d", len(after))
	}
	if after[0].Hash != hash {
		t.Fatalf("hash = %q, want %q", after[0].Hash, hash)
	}
	if !strings.Contains(after[0].Message, "\n") {
		t.Errorf("re-ingested message still subject-only (no backfill): %q", after[0].Message)
	}
	if !strings.Contains(after[0].Message, "Co-Authored-By") {
		t.Errorf("re-ingested message missing trailer (no backfill): %q", after[0].Message)
	}
}
