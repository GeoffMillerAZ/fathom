package client_test

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"

	"github.com/geoffmilleraz/agentic-kit/foundations/llmclient"
	"github.com/geoffmilleraz/fathom/internal/connector"
	"github.com/geoffmilleraz/fathom/internal/store"
	"github.com/geoffmilleraz/fathom/pkg/client"
)

// ─── helpers ────────────────────────────────────────────────────────────────

func newTestClient(t *testing.T, dir string) *client.Client {
	t.Helper()
	st := store.NewMemoryStore()
	conn := connector.NewLocalConnector()
	return client.New(conn, st)
}

func fp(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

// ─── corpus with CLAUDE.md + one Go file ───────────────────────────────────

func TestSemanticEndToEnd(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()

	// Governing document.
	claudeMD := []byte(`# Rules

- Run tests before committing.
- Sort all exported slice results.
`)
	if err := os.WriteFile(filepath.Join(dir, "CLAUDE.md"), claudeMD, 0o644); err != nil {
		t.Fatal(err)
	}

	// Source file that will be indexed.
	goFile := []byte("package main\nfunc Foo() {}\n")
	if err := os.MkdirAll(filepath.Join(dir, "pkg"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "pkg", "foo.go"), goFile, 0o644); err != nil {
		t.Fatal(err)
	}

	c := newTestClient(t, dir)
	defer c.Close()

	// Ingest so the store has service_meta and observations.
	if _, err := c.Ingest(ctx, client.ServiceRef{ID: "svc", Path: dir}); err != nil {
		t.Fatal("Ingest:", err)
	}

	// Dry-run compile (no LLM needed).
	run, err := c.CompileObligations(ctx, "svc", true)
	if err != nil {
		t.Fatalf("CompileObligations dry-run: %v", err)
	}
	if run.DryRun != true {
		t.Errorf("DryRun flag not set")
	}
	if run.SectionsTotal == 0 {
		t.Errorf("dry-run: expected >0 sections, got %d", run.SectionsTotal)
	}
	t.Logf("dry-run: sections=%d tokensIn=%d", run.SectionsTotal, run.TokensIn)

	// Live compile with mock LLM.
	mockJSON := `{"claims":[{"rule":"Run tests before committing.","scope":["**"],"cite_start":3,"cite_end":3},{"rule":"Sort all exported slice results.","scope":["pkg/**"],"cite_start":4,"cite_end":4}]}`
	mock := llmclient.NewMockClient(llmclient.MockEntry{
		Response: llmclient.Response{
			Text:  mockJSON,
			Usage: llmclient.Usage{InputTokens: 80, OutputTokens: 30},
		},
	})
	c.WithLLM(mock, "mock-model", 0.001, 0.005)

	run2, err := c.CompileObligations(ctx, "svc", false)
	if err != nil {
		t.Fatalf("CompileObligations live: %v", err)
	}
	if run2.ClaimsEmitted < 1 {
		t.Errorf("expected ≥1 claims emitted, got %d (rejected=%d)", run2.ClaimsEmitted, run2.ClaimsRejected)
	}
	t.Logf("live: emitted=%d rejected=%d tokens=%d/%d cost=%.6f",
		run2.ClaimsEmitted, run2.ClaimsRejected, run2.TokensIn, run2.TokensOut, run2.CostUSD)

	// Obligations returns rules for pkg/foo.go.
	obs, err := c.Obligations(ctx, "svc", "pkg/foo.go", 0)
	if err != nil {
		t.Fatalf("Obligations: %v", err)
	}
	if len(obs.Rules) == 0 {
		t.Errorf("expected obligations for pkg/foo.go, got none")
	}
	t.Logf("obligations for pkg/foo.go: %d rules", len(obs.Rules))

	// ObligationsIndexStale: file not changed → false.
	stale := c.ObligationsIndexStale(ctx, "svc", "CLAUDE.md")
	// After ingest CLAUDE.md is in the store; content same on disk → not stale.
	// (May be false even if not ingested since we can't fingerprint what we don't have.)
	t.Logf("ObligationsIndexStale(CLAUDE.md) = %v (may be false if not fingerprinted)", stale)

	// SweepClaims: should run without error.
	sr, err := c.SweepClaims(ctx, "svc")
	if err != nil {
		t.Fatalf("SweepClaims: %v", err)
	}
	t.Logf("sweep: checked=%d fresh=%d stale=%d contradicted=%d",
		sr.ClaimsChecked, sr.Fresh, sr.Stale, sr.Contradicted)
}

// TestDryRunWithoutLLM: no LLM configured, dry_run → ok; !dry_run → error.
func TestDryRunWithoutLLM(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "CLAUDE.md"), []byte("# R\n\n- Do X.\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	c := newTestClient(t, dir)
	defer c.Close()
	if _, err := c.Ingest(ctx, client.ServiceRef{ID: "svc", Path: dir}); err != nil {
		t.Fatal("Ingest:", err)
	}

	if _, err := c.CompileObligations(ctx, "svc", true); err != nil {
		t.Errorf("dry-run without LLM should succeed: %v", err)
	}
	if _, err := c.CompileObligations(ctx, "svc", false); err == nil {
		t.Errorf("live compile without LLM should return error")
	}
}

// TestObligationsIndexStale: edit file on disk, stale should flip.
func TestObligationsIndexStale(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	fpath := filepath.Join(dir, "CLAUDE.md")
	initial := []byte("# R\n\n- Keep tests green.\n")
	if err := os.WriteFile(fpath, initial, 0o644); err != nil {
		t.Fatal(err)
	}

	c := newTestClient(t, dir)
	defer c.Close()
	if _, err := c.Ingest(ctx, client.ServiceRef{ID: "svc", Path: dir}); err != nil {
		t.Fatal("Ingest:", err)
	}

	// Not stale immediately after ingest.
	if c.ObligationsIndexStale(ctx, "svc", "CLAUDE.md") {
		t.Error("expected not stale right after ingest")
	}

	// Edit the file.
	if err := os.WriteFile(fpath, []byte("# R\n\n- Keep tests green.\n- New rule.\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if !c.ObligationsIndexStale(ctx, "svc", "CLAUDE.md") {
		t.Error("expected stale after editing CLAUDE.md on disk without re-ingest")
	}
}
