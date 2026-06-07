package fusion_test

// polyglot_test.go exercises the fusion pipeline end-to-end against the
// polyglot-service fixture, which contains one small file per supported
// language. It verifies that symbols are extracted for each language and
// that the pipeline stores observations correctly.

import (
	"context"
	"testing"

	"github.com/geoffmilleraz/sextant/internal/connector"
	"github.com/geoffmilleraz/sextant/internal/core"
	"github.com/geoffmilleraz/sextant/internal/fusion"
	"github.com/geoffmilleraz/sextant/internal/store"
)

var polyglotSvc = core.ServiceRef{
	ID:   "polyglot-service",
	Path: "polyglot-service",
}

// TestPolyglot_E2E ingests the polyglot-service fixture and asserts that
// at least one symbol is extracted per language file.
func TestPolyglot_E2E(t *testing.T) {
	conn := connector.NewFauxConnector()
	st := store.NewMemoryStore()
	p := fusion.New(conn, st, defaultCfg())
	ctx := context.Background()

	report, err := p.Ingest(ctx, polyglotSvc)
	if err != nil {
		t.Fatalf("Ingest: %v", err)
	}

	if report.Discovered == 0 {
		t.Fatal("Ingest: discovered 0 files — fixture missing?")
	}

	obs, err := st.QueryObservations(ctx, polyglotSvc.ID, "")
	if err != nil {
		t.Fatalf("QueryObservations: %v", err)
	}

	if len(obs) == 0 {
		t.Fatal("no observations stored after polyglot ingest")
	}

	// Build a set of SourceKinds we observed.
	kindsSeen := make(map[core.SourceKind]bool)
	for _, o := range obs {
		kindsSeen[o.SourceKind] = true
	}

	// Each language must contribute at least one symbol kind.
	wantKinds := []struct {
		desc string
		kind core.SourceKind
	}{
		{"Python class or func", core.KindPyClass},
		{"JS function", core.KindJSFunc},
		{"Java class", core.KindJavaClass},
		{"Ruby module or class", core.KindRubyModule},
		{"Terraform resource", core.KindTerraformResource},
		{"K8s workload", core.KindK8sWorkload},
		{"CUE definition", core.KindCUEDefinition},
		{"Markdown doc chunk", core.KindDocChunk},
		{"Go module dep", core.KindModuleDep},
	}

	for _, w := range wantKinds {
		if !kindsSeen[w.kind] {
			// Some kinds might not be the exact kind stored — check alternatives.
			alt := kindAlternatives(w.kind)
			found := false
			for _, a := range alt {
				if kindsSeen[a] {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("no observations with kind %q (%s)", w.kind, w.desc)
			}
		}
	}
}

// kindAlternatives returns alternative SourceKinds that satisfy the same
// language coverage requirement. For example, Python files might yield
// KindPyFunc instead of KindPyClass if only functions are present.
func kindAlternatives(k core.SourceKind) []core.SourceKind {
	switch k {
	case core.KindPyClass:
		return []core.SourceKind{core.KindPyClass, core.KindPyFunc}
	case core.KindJSFunc:
		return []core.SourceKind{core.KindJSFunc, core.KindJSClass, core.KindJSMethod}
	case core.KindJavaClass:
		return []core.SourceKind{core.KindJavaClass, core.KindJavaInterface, core.KindJavaMethod}
	case core.KindRubyModule:
		return []core.SourceKind{core.KindRubyModule, core.KindRubyClass, core.KindRubyMethod}
	}
	return []core.SourceKind{k}
}

// TestPolyglot_Determinism verifies that ingesting the polyglot fixture twice
// produces identical observations.
func TestPolyglot_Determinism(t *testing.T) {
	conn := connector.NewFauxConnector()
	ctx := context.Background()

	st1 := store.NewMemoryStore()
	p1 := fusion.New(conn, st1, defaultCfg())
	if _, err := p1.Ingest(ctx, polyglotSvc); err != nil {
		t.Fatalf("first Ingest: %v", err)
	}
	obs1, err := st1.QueryObservations(ctx, polyglotSvc.ID, "")
	if err != nil {
		t.Fatalf("first QueryObservations: %v", err)
	}

	st2 := store.NewMemoryStore()
	p2 := fusion.New(conn, st2, defaultCfg())
	if _, err := p2.Ingest(ctx, polyglotSvc); err != nil {
		t.Fatalf("second Ingest: %v", err)
	}
	obs2, err := st2.QueryObservations(ctx, polyglotSvc.ID, "")
	if err != nil {
		t.Fatalf("second QueryObservations: %v", err)
	}

	if len(obs1) != len(obs2) {
		t.Fatalf("non-deterministic: first=%d obs, second=%d obs", len(obs1), len(obs2))
	}
	for i := range obs1 {
		if obs1[i].Symbol != obs2[i].Symbol || obs1[i].ContentHash != obs2[i].ContentHash {
			t.Errorf("obs[%d] mismatch: %q vs %q", i, obs1[i].Symbol, obs2[i].Symbol)
		}
	}
}

// TestPolyglot_HashGate verifies that re-ingesting the unchanged polyglot
// fixture skips all files on the second run.
func TestPolyglot_HashGate(t *testing.T) {
	conn := connector.NewFauxConnector()
	st := store.NewMemoryStore()
	p := fusion.New(conn, st, defaultCfg())
	ctx := context.Background()

	r1, err := p.Ingest(ctx, polyglotSvc)
	if err != nil {
		t.Fatalf("first Ingest: %v", err)
	}
	if r1.Discovered == 0 {
		t.Fatal("discovered 0 files")
	}

	r2, err := p.Ingest(ctx, polyglotSvc)
	if err != nil {
		t.Fatalf("second Ingest: %v", err)
	}
	if r2.Skipped != r2.Discovered {
		t.Errorf("second ingest: Skipped=%d, Discovered=%d; want all skipped",
			r2.Skipped, r2.Discovered)
	}
}

// TestPolyglot_ObservationFields verifies that all polyglot observations carry
// required fields (non-empty ServiceID, SourceRef, SourceKind, ContentHash).
func TestPolyglot_ObservationFields(t *testing.T) {
	conn := connector.NewFauxConnector()
	st := store.NewMemoryStore()
	p := fusion.New(conn, st, defaultCfg())
	ctx := context.Background()

	if _, err := p.Ingest(ctx, polyglotSvc); err != nil {
		t.Fatalf("Ingest: %v", err)
	}

	obs, err := st.QueryObservations(ctx, polyglotSvc.ID, "")
	if err != nil {
		t.Fatalf("QueryObservations: %v", err)
	}

	for _, o := range obs {
		if o.ServiceID == "" {
			t.Errorf("ServiceID empty for symbol %q", o.Symbol)
		}
		if o.SourceRef == "" {
			t.Errorf("SourceRef empty for symbol %q", o.Symbol)
		}
		if o.SourceKind == "" {
			t.Errorf("SourceKind empty for symbol %q", o.Symbol)
		}
		if o.ContentHash == "" {
			t.Errorf("ContentHash empty for symbol %q", o.Symbol)
		}
		if o.Symbol == "" {
			t.Error("Symbol is empty")
		}
	}
}
