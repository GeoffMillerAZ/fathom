package client_test

// projectlayer_test.go — LC-3 (docs/plans/layer-catalog.md): CompileLayer
// resolves a layer through the union of embedded packs and the SERVICE's own
// project-local .fathom/layers/ (semantic.LoadLayerIn, keyed off
// service_meta.Path — the same root SweepClaims/ObligationsIndexStale
// already resolve a service's on-disk root from). This is the real-usage
// counterpart to internal/semantic's LC-8 proof: a pack nobody wired any Go
// for compiles and serves through pkg/client exactly like obligations/
// findings do.

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/geoffmilleraz/agentic-kit/foundations/llmclient"
	"github.com/geoffmilleraz/fathom/internal/connector"
	"github.com/geoffmilleraz/fathom/internal/semantic"
	"github.com/geoffmilleraz/fathom/internal/store"
	"github.com/geoffmilleraz/fathom/pkg/client"
)

const projectLayerModSrc = `package widget

// Build assembles a widget.
func Build() int { return 1 }
`

// projectLayerDoc line numbers (1-based):
//
//	1: # Widget rule
//	2: (blank)
//	3: - Widgets must be built deterministically.
const projectLayerDoc = `# Widget rule

- Widgets must be built deterministically.
`

// writeProjectPack writes a minimal valid module pack at
// <root>/.fathom/layers/<name>/ — the shape `fathom layers install` produces
// and semantic.LoadLayerIn discovers by presence alone (ADR-018 reconciler
// doctrine: no enabled/disabled bit).
func writeProjectPack(t *testing.T, root, name string) {
	t.Helper()
	dir := filepath.Join(root, semantic.ProjectLayersDir, name)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	layerCUE := `name:    "` + name + `"
version: "v1"
kind:    "project-kind"
corpus: include: ["docs/**"]
prompt_version: "` + name + `/fixture"
prompt_file:    "` + name + `/prompt.md"
tier:           "doc-claimed"
budgets: {
	max_tokens_per_run: 50000
	max_usd_per_run:    0.5
}
`
	if err := os.WriteFile(filepath.Join(dir, "layer.cue"), []byte(layerCUE), 0o644); err != nil {
		t.Fatalf("write layer.cue: %v", err)
	}
	prompt := `You compile widget rules from governing documents into strict JSON.
Output ONLY a JSON object {"claims": [...]} whose claims array (possibly
empty) holds objects of the shape {"rule", "scope", "cite_start", "cite_end"}.
`
	if err := os.WriteFile(filepath.Join(dir, "prompt.md"), []byte(prompt), 0o644); err != nil {
		t.Fatalf("write prompt.md: %v", err)
	}
}

// TestClient_CompileLayer_ProjectLocalPack proves LC-3 end-to-end through the
// REAL compile path (pkg/client.CompileLayer), not just the loader in
// isolation: a project pack that ships with the service's own repo — no
// entry in schema/semantic/layers/, no Go anywhere — compiles and serves
// exactly like an embedded layer.
func TestClient_CompileLayer_ProjectLocalPack(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	writeCorpusFile(t, filepath.Join(root, "mod", "widget.go"), projectLayerModSrc)
	writeCorpusFile(t, filepath.Join(root, "docs", "widget.md"), projectLayerDoc)
	writeProjectPack(t, root, "widget-rules")

	st := store.NewMemoryStore()
	c := client.New(connector.NewLocalConnector(), st)
	defer c.Close()

	if _, err := c.Ingest(ctx, client.ServiceRef{ID: "widget-svc", Path: root}); err != nil {
		t.Fatalf("Ingest: %v", err)
	}

	// The layer name is not embedded — LoadLayer alone would reject it, but
	// CompileLayer must resolve it via the service's own root.
	dry, err := c.CompileLayer(ctx, "widget-svc", "widget-rules", true)
	if err != nil {
		t.Fatalf("CompileLayer dry-run (project pack): %v", err)
	}
	if !dry.DryRun || dry.SectionsTotal == 0 {
		t.Fatalf("dry-run = %+v, want DryRun=true and >0 sections", dry)
	}

	mock := llmclient.NewMockClient(llmclient.MockEntry{Response: llmclient.Response{
		Text: `{"claims":[{"rule":"Build widgets deterministically.","scope":["mod/**"],"cite_start":3,"cite_end":3}]}`,
	}})
	c.WithLLM(mock, "mock-model", 0, 0)
	run, err := c.CompileLayer(ctx, "widget-svc", "widget-rules", false)
	if err != nil {
		t.Fatalf("CompileLayer live (project pack): %v", err)
	}
	if run.Layer != "widget-rules" || run.ClaimsEmitted != 1 {
		t.Fatalf("run = %+v, want layer widget-rules with 1 claim emitted", run)
	}

	claims, err := st.ClaimsForService(ctx, "widget-svc", "widget-rules")
	if err != nil || len(claims) != 1 {
		t.Fatalf("ClaimsForService: %v / %d", err, len(claims))
	}
	if claims[0].Kind != "project-kind" {
		t.Errorf("Kind = %q, want project-kind (from the project pack's own layer.cue)", claims[0].Kind)
	}

	// Served through the SAME ObligationsFor path as every other layer —
	// zero layer-specific Go anywhere in the serving chain.
	obl, err := c.Obligations(ctx, "widget-svc", "mod/widget.go", 0)
	if err != nil {
		t.Fatalf("Obligations: %v", err)
	}
	if len(obl.Rules) != 1 || obl.Rules[0].Kind != "project-kind" {
		t.Fatalf("served rules = %+v, want 1 rule of Kind project-kind", obl.Rules)
	}
}
