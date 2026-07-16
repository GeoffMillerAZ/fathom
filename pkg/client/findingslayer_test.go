package client_test

// findingslayer_test.go — the D3 findings-tracer-bullet contract (FT-1/FT-2/
// FT-6, docs/plans/findings-tracer.md): unlike expertmodule_test.go (which
// proves the non-git-corpus loop under the obligations kind, since
// Kind:"finding" was still gated off), this exercises the SAME corpus shape
// through the now-generalized pkg/client.CompileLayer("findings", ...) and
// asserts the finding-specific serving contract: Kind "finding", tier
// sme-stated, and the FT-6 remediation-drift annotation.

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/geoffmilleraz/agentic-kit/foundations/llmclient"
	"github.com/geoffmilleraz/fathom/internal/connector"
	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/store"
	"github.com/geoffmilleraz/fathom/pkg/client"
)

const findingsModPricingSrc = `package pricing

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
const findingsWA001Doc = `# WA-001: pricing calls lack retry

Severity: high. Area: reliability.
The pricing module must add retry with exponential backoff to all
outbound quote calls before the next production deploy.
`

const findingsMockClaim = `{"claims":[{"rule":"Add retry with exponential backoff to all outbound pricing quote calls.","scope":["mod/**"],"cite_start":4,"cite_end":5}]}`

func findingsMockLLM() llmclient.LLMClient {
	return llmclient.NewMockClient(llmclient.MockEntry{Response: llmclient.Response{Text: findingsMockClaim}})
}

// TestFindingsLayer_CompileLayer_EndToEnd is FT-1's E2E test: a findings
// corpus doc compiled through pkg/client.CompileLayer("findings", ...) (not
// the obligations layer expertmodule_test.go used) yields a claim with
// Kind "finding" at tier sme-stated, served labeled as such (FT-2).
func TestFindingsLayer_CompileLayer_EndToEnd(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	writeCorpusFile(t, filepath.Join(root, "mod", "pricing.go"), findingsModPricingSrc)
	writeCorpusFile(t, filepath.Join(root, "findings", "wa-001.md"), findingsWA001Doc)

	st := store.NewMemoryStore()
	c := client.New(connector.NewLocalConnector(), st)
	defer c.Close()

	if _, err := c.Ingest(ctx, client.ServiceRef{ID: "wa-findings", Path: root}); err != nil {
		t.Fatalf("Ingest: %v", err)
	}

	// Dry-run first (mandatory before spend, ADR-016) — no LLM needed.
	dry, err := c.CompileLayer(ctx, "wa-findings", "findings", true)
	if err != nil {
		t.Fatalf("CompileLayer dry-run: %v", err)
	}
	if !dry.DryRun || dry.SectionsTotal == 0 {
		t.Fatalf("dry-run = %+v, want DryRun=true and >0 sections", dry)
	}

	c.WithLLM(findingsMockLLM(), "mock-model", 0, 0)
	run, err := c.CompileLayer(ctx, "wa-findings", "findings", false)
	if err != nil {
		t.Fatalf("CompileLayer live: %v", err)
	}
	if run.Layer != "findings" {
		t.Errorf("CompileRun.Layer = %q, want findings", run.Layer)
	}
	if run.ClaimsEmitted != 1 || run.ClaimsRejected != 0 {
		t.Fatalf("emitted/rejected = %d/%d, want 1/0", run.ClaimsEmitted, run.ClaimsRejected)
	}

	// CompileObligations must still work unaffected (F1 fix must not have
	// regressed the wrapper) — a plain obligations compile on the same
	// service with no CLAUDE.md/ADR.md present simply emits nothing.
	if _, err := c.CompileObligations(ctx, "wa-findings", true); err != nil {
		t.Errorf("CompileObligations wrapper: %v", err)
	}

	claims, err := st.ClaimsForService(ctx, "wa-findings", "findings")
	if err != nil || len(claims) != 1 {
		t.Fatalf("ClaimsForService(findings): %v / %d", err, len(claims))
	}
	if claims[0].Kind != "finding" {
		t.Errorf("Kind = %q, want finding", claims[0].Kind)
	}
	if claims[0].Tier != core.TierSMEStated {
		t.Errorf("Tier = %q, want sme-stated", claims[0].Tier)
	}

	// Served payload: labeled as a finding (FT-2), not silently indistinguishable
	// from an obligation.
	obl, err := c.Obligations(ctx, "wa-findings", "mod/pricing.go", 0)
	if err != nil {
		t.Fatalf("Obligations: %v", err)
	}
	if len(obl.Rules) != 1 {
		t.Fatalf("got %d rules, want 1: %+v", len(obl.Rules), obl)
	}
	if obl.Rules[0].Kind != "finding" {
		t.Errorf("served Kind = %q, want finding", obl.Rules[0].Kind)
	}
	if obl.Rules[0].Tier != string(core.TierSMEStated) {
		t.Errorf("served Tier = %q, want sme-stated", obl.Rules[0].Tier)
	}
	if obl.Rules[0].CodeDrifted {
		t.Errorf("fresh compile, unchanged code: CodeDrifted = true, want false")
	}
}

// TestFindingsLayer_RemediationDrift is FT-6: a finding claim scoped to
// mod/** whose CITED DOC never changes stays freshness-fresh (Sweep only
// re-hashes cites links), but once the scoped CODE changes, the served
// payload must carry code_drifted:true — a live annotation, never a status
// flip (docs/vision/03-findings-and-experts.md §2).
func TestFindingsLayer_RemediationDrift(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	writeCorpusFile(t, filepath.Join(root, "mod", "pricing.go"), findingsModPricingSrc)
	writeCorpusFile(t, filepath.Join(root, "findings", "wa-001.md"), findingsWA001Doc)

	st := store.NewMemoryStore()
	c := client.New(connector.NewLocalConnector(), st)
	defer c.Close()

	if _, err := c.Ingest(ctx, client.ServiceRef{ID: "wa-drift", Path: root}); err != nil {
		t.Fatalf("Ingest: %v", err)
	}
	c.WithLLM(findingsMockLLM(), "mock-model", 0, 0)
	if _, err := c.CompileLayer(ctx, "wa-drift", "findings", false); err != nil {
		t.Fatalf("CompileLayer: %v", err)
	}

	// Immediately after compile: no drift.
	obl, err := c.Obligations(ctx, "wa-drift", "mod/pricing.go", 0)
	if err != nil || len(obl.Rules) != 1 {
		t.Fatalf("Obligations before change: %v / %+v", err, obl)
	}
	if obl.Rules[0].CodeDrifted {
		t.Fatalf("CodeDrifted = true before any code change")
	}
	if obl.Rules[0].Status != "" {
		t.Fatalf("Status = %q, want fresh (omitted) before any change", obl.Rules[0].Status)
	}

	// Change the SCOPED CODE only — the finding doc is untouched.
	changed := `package pricing

// Quote returns a price quote. Now with an extra field.
func Quote() int { return 2 }
`
	writeCorpusFile(t, filepath.Join(root, "mod", "pricing.go"), changed)
	if err := c.IngestService(ctx, "wa-drift"); err != nil {
		t.Fatalf("IngestService after code change: %v", err)
	}

	obl, err = c.Obligations(ctx, "wa-drift", "mod/pricing.go", 0)
	if err != nil || len(obl.Rules) != 1 {
		t.Fatalf("Obligations after code change: %v / %+v", err, obl)
	}
	// Doc unchanged: the freshness axis (Sweep) must still say fresh.
	if obl.Rules[0].Status != "" {
		t.Errorf("Status = %q, want fresh (omitted) — doc unchanged, only scoped code changed", obl.Rules[0].Status)
	}
	// Code changed: the drift annotation must now be true.
	if !obl.Rules[0].CodeDrifted {
		t.Errorf("CodeDrifted = false, want true after the scoped file's content changed")
	}
}
