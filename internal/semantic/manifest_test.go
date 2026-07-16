package semantic

import (
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
)

// TestManifestSchemaMatchesSource is the drift gate M1-1 requires: the
// embedded manifest_schema.cue (internal/semantic, //go:embed'd into the
// binary) must be byte-identical to schema/semantic/schema.cue, which stays
// the single authoritative source. A change to one without the other must
// fail this test, not silently diverge.
func TestManifestSchemaMatchesSource(t *testing.T) {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	src := filepath.Join(filepath.Dir(file), "..", "..", "schema", "semantic", "schema.cue")
	want, err := os.ReadFile(src)
	if err != nil {
		t.Fatalf("read source schema %s: %v", src, err)
	}
	if string(want) != string(manifestSchema) {
		t.Fatalf(
			"internal/semantic/manifest_schema.cue has drifted from %s — "+
				"copy the source file verbatim (schema/semantic/schema.cue is authoritative)",
			src,
		)
	}
}

func TestLoadLayer_Obligations(t *testing.T) {
	def, err := LoadLayer("obligations")
	if err != nil {
		t.Fatalf("LoadLayer(obligations): %v", err)
	}
	if def.Name != "obligations" || def.Version != "v1" || def.Kind != "obligation" {
		t.Errorf("identity = %+v", def)
	}
	wantInclude := []string{"CLAUDE.md", "ADR.md", "docs/plans/README.md"}
	if !reflect.DeepEqual(def.CorpusInclude, wantInclude) {
		t.Errorf("CorpusInclude = %v, want %v", def.CorpusInclude, wantInclude)
	}
	if len(def.CorpusExclude) != 0 {
		t.Errorf("CorpusExclude = %v, want none", def.CorpusExclude)
	}
	if def.Tier != "doc-claimed" {
		t.Errorf("Tier = %q, want doc-claimed", def.Tier)
	}
	if def.TierProfile != "engineering" {
		t.Errorf("TierProfile = %q, want engineering (default)", def.TierProfile)
	}
	if def.Budgets.MaxTokensPerRun != 200000 {
		t.Errorf("Budgets.MaxTokensPerRun = %d, want 200000", def.Budgets.MaxTokensPerRun)
	}
	if def.Budgets.MaxUSDPerRun != 1.0 {
		t.Errorf("Budgets.MaxUSDPerRun = %v, want 1.0", def.Budgets.MaxUSDPerRun)
	}
	// Not set on the obligations instance — must resolve to the schema's own
	// defaults (proves CUE default resolution flows through Decode).
	if def.Budgets.MaxRuleWords != 28 {
		t.Errorf("Budgets.MaxRuleWords = %d, want 28 (schema default)", def.Budgets.MaxRuleWords)
	}
	if def.Budgets.MaxClaimsPerFile != 40 {
		t.Errorf("Budgets.MaxClaimsPerFile = %d, want 40 (schema default)", def.Budgets.MaxClaimsPerFile)
	}
	if def.Serving.MaxRules != 6 {
		t.Errorf("Serving.MaxRules = %d, want 6 (schema default)", def.Serving.MaxRules)
	}
	if def.Serving.MaxTokens != 150 {
		t.Errorf("Serving.MaxTokens = %d, want 150 (schema default)", def.Serving.MaxTokens)
	}
	if def.PromptVersion == "" {
		t.Errorf("PromptVersion = %q, want a non-empty advisory label", def.PromptVersion)
	}
}

func TestLoadLayer_Unknown(t *testing.T) {
	_, err := LoadLayer("hypotheses")
	if err == nil {
		t.Fatal("LoadLayer(hypotheses): want error for a layer the manifest does not define")
	}
	if !strings.Contains(err.Error(), "obligations") {
		t.Errorf("error should list known layers (obligations): %v", err)
	}
	if !strings.Contains(err.Error(), "hypotheses") {
		t.Errorf("error should echo the requested name: %v", err)
	}
}

func TestLayerNames(t *testing.T) {
	names, err := LayerNames()
	if err != nil {
		t.Fatalf("LayerNames: %v", err)
	}
	if len(names) != 1 || names[0] != "obligations" {
		t.Errorf("LayerNames() = %v, want [obligations]", names)
	}
}
