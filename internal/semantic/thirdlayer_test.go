package semantic

// thirdlayer_test.go — LC-8 (docs/plans/layer-catalog.md): the "no-Go-per-
// layer" claim under proof. testdata/lc8fixture/.fathom/layers/thirdlayer/ is
// a THIRD pack — not obligations, not findings, never shipped or embedded —
// loaded the exact way `fathom layers install` would leave a real one
// (semantic.LoadLayerIn against a project root). This test compiles it and
// serves it through ObligationsFor with ZERO code in this package written
// for "thirdlayer" specifically: no switch case, no string literal
// comparison, nothing. If the pipeline ever regresses to needing per-layer
// Go again, this is the test that catches it.

import (
	"context"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/store"
	"github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/llmclient"
)

func lc8FixtureRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	return filepath.Join(filepath.Dir(file), "testdata", "lc8fixture")
}

// thirdLayerDoc line numbers (1-based):
//
//	1: # Governance rule
//	2: (blank)
//	3: - Every migration must be reversible.
const thirdLayerDoc = `# Governance rule

- Every migration must be reversible.
`

// Five indexed files, only two under internal/migrate/** (40%) — kept below
// the compiler's >50% over-broad-scope guard (O8/OP-2, compile.go), which is
// unrelated to this test's purpose and would otherwise reject the claim.
var thirdLayerIndexedFiles = []string{
	"internal/migrate/apply.go", "internal/migrate/plan.go",
	"internal/query/engine.go", "internal/mcp/tools.go", "docs/x.md",
}

// TestThirdLayer_CompileAndServe_ZeroLayerSpecificGo is the LC-8 proof:
// load → compile → serve, entirely through generic APIs.
func TestThirdLayer_CompileAndServe_ZeroLayerSpecificGo(t *testing.T) {
	ctx := context.Background()
	root := lc8FixtureRoot(t)

	// 1. LOAD: the union loader resolves "thirdlayer" purely from the
	// project's .fathom/layers/ — nothing in this package knows its name in
	// advance.
	layer, err := LoadLayerIn(root, "thirdlayer")
	if err != nil {
		t.Fatalf("LoadLayerIn(fixture root, thirdlayer): %v", err)
	}
	if layer.Kind != "third-kind" || !layer.Serving.DriftTracked {
		t.Fatalf("fixture layer decoded wrong: %+v", layer)
	}

	st := store.NewMemoryStore()
	// Serve-time drift resolution (DriftedFindings/layerIsDriftTracked,
	// sweep.go) resolves a service's project root via ServiceMeta.Path — the
	// SAME mechanism pkg/client.CompileLayer and SweepClaims already use —
	// so it can re-consult the union loader that compiled this layer in the
	// first place. A real `fathom ingest` sets this; this test does what
	// ingest does.
	if err := st.SetServiceMeta(ctx, store.ServiceMeta{ServiceID: "svc", Path: root}); err != nil {
		t.Fatalf("SetServiceMeta: %v", err)
	}
	for _, f := range thirdLayerIndexedFiles {
		if err := st.SetFingerprint(ctx, "svc", f, "fp-"+f); err != nil {
			t.Fatalf("SetFingerprint: %v", err)
		}
	}

	// 2. COMPILE: the SAME Compiler every layer uses, configured only by the
	// loaded LayerDef.
	c := &Compiler{
		St: st,
		LLM: llmclient.NewMockClient(llmclient.MockEntry{Response: llmclient.Response{
			Text: `{"claims":[{"rule":"Every migration must be reversible.","scope":["internal/migrate/**"],"cite_start":3,"cite_end":3}]}`,
		}}),
		Model: "mock-model",
		Layer: layer,
		Now:   func() time.Time { return time.Date(2026, 7, 17, 0, 0, 0, 0, time.UTC) },
	}
	run, err := c.Compile(ctx, CompileInput{
		ServiceID:    "svc",
		Docs:         []core.SourceFile{{Path: "governance/migrations.md", Content: []byte(thirdLayerDoc)}},
		IndexedFiles: thirdLayerIndexedFiles,
		Watermark:    "manifest:aaa",
	})
	if err != nil {
		t.Fatalf("Compile(thirdlayer): %v", err)
	}
	if run.Layer != "thirdlayer" || run.ClaimsEmitted != 1 || run.ClaimsRejected != 0 {
		t.Fatalf("run = %+v, want layer=thirdlayer, 1 emitted, 0 rejected", run)
	}
	if run.PromptVersion == "" {
		t.Errorf("PromptVersion empty — the fixture's own prompt.md should have resolved")
	}

	claims, err := st.ClaimsForService(ctx, "svc", "thirdlayer")
	if err != nil || len(claims) != 1 {
		t.Fatalf("ClaimsForService(thirdlayer): %v / %d", err, len(claims))
	}
	if claims[0].Kind != "third-kind" || claims[0].Tier != core.TierDocClaimed {
		t.Errorf("claim = %+v, want Kind=third-kind Tier=doc-claimed", claims[0])
	}
	// LC-2 proof: drift_tracked:true on a THIRD layer must have written a
	// scope-fp anchor at compile time, exactly like findings does — via the
	// schema field, not a kind=="finding" check.
	links, err := st.ClaimLinks(ctx, "svc", []string{claims[0].ClaimID})
	if err != nil {
		t.Fatalf("ClaimLinks: %v", err)
	}
	var scopeFP int
	for _, l := range links {
		if l.Rel == core.RelScopeFP {
			scopeFP++
		}
	}
	if scopeFP == 0 {
		t.Errorf("no scope-fp links written for a drift_tracked=true third layer")
	}

	// 3. SERVE: ObligationsFor — all-layers by construction, no name check.
	obl, err := ObligationsFor(ctx, st, "svc", "internal/migrate/apply.go", 0)
	if err != nil {
		t.Fatalf("ObligationsFor: %v", err)
	}
	if len(obl.Rules) != 1 {
		t.Fatalf("served rules = %+v, want 1", obl.Rules)
	}
	rule := obl.Rules[0]
	if rule.Rule != "Every migration must be reversible." {
		t.Errorf("Rule = %q", rule.Rule)
	}
	if rule.Kind != "third-kind" {
		t.Errorf("served Kind = %q, want third-kind (a non-obligation kind must be labeled)", rule.Kind)
	}
	if rule.Tier != string(core.TierDocClaimed) {
		t.Errorf("served Tier = %q", rule.Tier)
	}
	if rule.Src == "" {
		t.Errorf("served Src (citation) is empty")
	}
	if rule.CodeDrifted {
		t.Errorf("CodeDrifted = true immediately after compile with unchanged fingerprints, want false")
	}

	// Change the scoped file's fingerprint (simulating a code edit after
	// compile) — the drift annotation must flip live, with STILL zero
	// layer-specific Go: DriftedFindings resolves drift_tracked generically.
	if err := st.SetFingerprint(ctx, "svc", "internal/migrate/apply.go", "fp-changed"); err != nil {
		t.Fatalf("SetFingerprint change: %v", err)
	}
	obl2, err := ObligationsFor(ctx, st, "svc", "internal/migrate/apply.go", 0)
	if err != nil {
		t.Fatalf("ObligationsFor after drift: %v", err)
	}
	if len(obl2.Rules) != 1 || !obl2.Rules[0].CodeDrifted {
		t.Fatalf("served rules after code drift = %+v, want CodeDrifted=true", obl2.Rules)
	}
}

// TestThirdLayer_ListedByUnionLoader is a lighter companion proof: the
// fixture pack shows up in ListLayersIn beside the two built-ins, sourced as
// "project" — the same union `fathom layers list` reads from.
func TestThirdLayer_ListedByUnionLoader(t *testing.T) {
	infos, err := ListLayersIn(lc8FixtureRoot(t))
	if err != nil {
		t.Fatalf("ListLayersIn: %v", err)
	}
	var found bool
	for _, info := range infos {
		if info.Name == "thirdlayer" {
			found = true
			if info.Source != SourceProject {
				t.Errorf("thirdlayer Source = %q, want project", info.Source)
			}
		}
	}
	if !found {
		t.Fatalf("thirdlayer missing from ListLayersIn: %+v", infos)
	}
	want := numEmbeddedLayers(t) + 1
	if len(infos) != want {
		t.Errorf("ListLayersIn = %d entries, want %d (every embedded layer + thirdlayer)", len(infos), want)
	}
}
