package client_test

// expertmodule_test.go — the expert-module v1 contract
// (docs/design/expert-modules.md §7): a NON-GIT findings corpus becomes a
// standalone module — one .sqlite store carrying the index, compiled claims
// with citations, honest non-git freshness (manifest watermark + sweep), and
// portability (move corpus + remap → everything still answers). This test is
// D-EM1 (corpus-agnostic core) and D-EM2 (the store IS the module) in code.

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/internal/connector"
	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/semantic"
	"github.com/geoffmilleraz/fathom/internal/store"
	"github.com/geoffmilleraz/fathom/pkg/client"
	"github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/llmclient"
)

// The reviewed code and the review finding live in one module corpus (the
// "mixed" shape — expert-modules.md §2).
const modPricingSrc = `package pricing

// Quote returns a price quote.
func Quote() int { return 1 }
`

// Finding doc line numbers (1-based):
//
//	1: # WA-001: pricing calls lack retry
//	2: (blank)
//	3: Severity: high. Area: reliability.
//	4: The pricing module must add retry with exponential backoff to all
//	5: outbound quote calls before the next production deploy.
const findingDoc = `# WA-001: pricing calls lack retry

Severity: high. Area: reliability.
The pricing module must add retry with exponential backoff to all
outbound quote calls before the next production deploy.
`

func TestExpertModule_NonGitCorpus_EndToEnd(t *testing.T) {
	ctx := context.Background()

	// ── Create: a non-git corpus at location A ─────────────────────────────
	rootA := gitProofTempDir(t)
	writeCorpusFile(t, filepath.Join(rootA, "mod", "pricing.go"), modPricingSrc)
	writeCorpusFile(t, filepath.Join(rootA, "findings", "wa-001.md"), findingDoc)

	storePath := filepath.Join(t.TempDir(), "wa-review.sqlite")
	st, err := store.Open(storePath)
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	defer st.Close()
	c := client.New(connector.NewLocalConnector(), st)

	if _, err := c.Ingest(ctx, client.ServiceRef{ID: "wa-review", Path: rootA}); err != nil {
		t.Fatalf("Ingest: %v", err)
	}
	meta, found, _ := st.GetServiceMeta(ctx, "wa-review")
	if !found || !strings.HasPrefix(meta.IndexedCommit, "manifest:") {
		t.Fatalf("non-git module watermark = %q, want manifest: prefix", meta.IndexedCommit)
	}

	// ── Compile: findings → governed claims (mock LLM; the engine is the
	// same one the obligations layer uses — a module is configuration) ──────
	files, err := c.Sources(ctx, "wa-review")
	if err != nil {
		t.Fatalf("Sources: %v", err)
	}
	findings := semantic.SelectCorpus(files, []string{"findings/**"}, nil)
	if len(findings) != 1 {
		t.Fatalf("selected %d finding docs, want 1", len(findings))
	}
	var indexed []string
	obs, _ := st.QueryObservations(ctx, "wa-review", "")
	seen := map[string]bool{}
	for _, o := range obs {
		if !seen[o.SourceRef] {
			seen[o.SourceRef] = true
			indexed = append(indexed, o.SourceRef)
		}
	}

	comp := &semantic.Compiler{
		St: st,
		LLM: llmclient.NewMockClient(llmclient.MockEntry{Response: llmclient.Response{
			Text: `{"claims":[{"rule":"Add retry with exponential backoff to all outbound pricing quote calls.","scope":["mod/**"],"cite_start":4,"cite_end":5}]}`,
		}}),
		Model: "mock-model",
		Now:   func() time.Time { return time.Date(2026, 7, 8, 0, 0, 0, 0, time.UTC) },
	}
	run, err := comp.Compile(ctx, semantic.CompileInput{
		ServiceID:    "wa-review",
		Docs:         findings,
		IndexedFiles: indexed,
		Watermark:    meta.IndexedCommit,
	})
	if err != nil {
		t.Fatalf("Compile: %v", err)
	}
	if run.ClaimsEmitted != 1 || run.ClaimsRejected != 0 {
		t.Fatalf("emitted/rejected = %d/%d, want 1/0", run.ClaimsEmitted, run.ClaimsRejected)
	}

	// ── Query standalone: "what governs the file I'm editing?" ─────────────
	got, err := c.Obligations(ctx, "wa-review", "mod/pricing.go", 0)
	if err != nil {
		t.Fatalf("Obligations: %v", err)
	}
	if len(got.Rules) != 1 {
		t.Fatalf("got %d rules, want 1: %+v", len(got.Rules), got)
	}
	if got.Rules[0].Src != "findings/wa-001.md#L4-L5" {
		t.Errorf("citation = %q", got.Rules[0].Src)
	}
	if got.Rules[0].Status != "" {
		t.Errorf("fresh finding carries status %q", got.Rules[0].Status)
	}

	// ── Freshness without git: edit the finding, re-ingest → the claim is
	// served FLAGGED (stale), and the watermark drifts ──────────────────────
	edited := strings.Replace(findingDoc, "exponential backoff", "jittered exponential backoff", 1)
	writeCorpusFile(t, filepath.Join(rootA, "findings", "wa-001.md"), edited)
	if err := c.IngestService(ctx, "wa-review"); err != nil { // wired sweep runs post-ingest
		t.Fatalf("IngestService: %v", err)
	}
	wmNow, _ := c.ServiceWatermark(ctx, "wa-review")
	if wmNow == meta.IndexedCommit {
		t.Error("manifest watermark did not drift after corpus edit")
	}
	got, err = c.Obligations(ctx, "wa-review", "mod/pricing.go", 0)
	if err != nil || len(got.Rules) != 1 {
		t.Fatalf("Obligations after edit: %v / %+v", err, got)
	}
	if got.Rules[0].Status != string(core.ClaimStale) {
		t.Errorf("drifted finding status = %q, want stale (served flagged, never dropped)", got.Rules[0].Status)
	}

	// ── Ship: move the corpus, remap the root — the module keeps answering
	// with intact citations and honest freshness, zero recompile ────────────
	rootB := t.TempDir()
	if err := os.CopyFS(rootB, os.DirFS(rootA)); err != nil {
		t.Fatalf("copy corpus: %v", err)
	}
	if err := os.RemoveAll(rootA); err != nil {
		t.Fatalf("remove rootA: %v", err)
	}
	meta2, _, _ := st.GetServiceMeta(ctx, "wa-review")
	meta2.Path = rootB
	if err := st.SetServiceMeta(ctx, meta2); err != nil {
		t.Fatalf("remap: %v", err)
	}

	got, err = c.Obligations(ctx, "wa-review", "mod/pricing.go", 0)
	if err != nil || len(got.Rules) != 1 {
		t.Fatalf("Obligations after move: %v / %+v", err, got)
	}
	// The sweep at the new root still resolves the citation against the
	// (edited) doc — status stays honest, does not become contradicted.
	rep, err := c.SweepClaims(ctx, "wa-review")
	if err != nil {
		t.Fatalf("SweepClaims after move: %v", err)
	}
	if rep.Contradicted != 0 {
		t.Errorf("sweep after pure move contradicted %d claims — root remap failed to resolve citations", rep.Contradicted)
	}
	report, err := c.Ingest(ctx, client.ServiceRef{ID: "wa-review", Path: rootB})
	if err != nil {
		t.Fatalf("re-Ingest at B: %v", err)
	}
	if report.Extracted != 0 {
		t.Errorf("re-ingest after pure move extracted %d files, want 0 (hash-gated)", report.Extracted)
	}
}
