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

// TestPackFilesMatchSource is LC-4's drift gate, extending the
// TestManifestSchemaMatchesSource family to the pack-shape files: every
// embedded (built-in) pack's layer.cue and prompt.md
// (internal/semantic/layers/<name>/, go:embed'd into the binary) must stay
// byte-identical to its schema/semantic/layers/<name>/ source — the
// authoritative directory a human, or `fathom layers install`, edits.
func TestPackFilesMatchSource(t *testing.T) {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	root := filepath.Join(filepath.Dir(file), "..", "..")
	for _, pack := range []string{"obligations", "findings", "adr-rules", "spec-bindings"} {
		for _, leaf := range []string{"layer.cue", "prompt.md"} {
			src := filepath.Join(root, "schema", "semantic", "layers", pack, leaf)
			want, err := os.ReadFile(src)
			if err != nil {
				t.Fatalf("read source %s: %v", src, err)
			}
			got, err := embeddedLayers.ReadFile("layers/" + pack + "/" + leaf)
			if err != nil {
				t.Fatalf("read embedded layers/%s/%s: %v", pack, leaf, err)
			}
			if string(want) != string(got) {
				t.Errorf("internal/semantic/layers/%s/%s has drifted from %s — copy the source file verbatim", pack, leaf, src)
			}
		}
	}
}

// TestPackFilesMatchSource_NoExtraEmbeddedPacks catches the inverse drift:
// an embedded pack directory with no matching schema/semantic/layers/<name>/
// source (e.g. a stale mirror left behind after a pack was renamed/removed
// from the authoritative tree).
func TestPackFilesMatchSource_NoExtraEmbeddedPacks(t *testing.T) {
	names, err := packNames(embeddedLayers, "layers")
	if err != nil {
		t.Fatalf("packNames(embeddedLayers): %v", err)
	}
	want := []string{"adr-rules", "findings", "obligations", "spec-bindings"}
	if !reflect.DeepEqual(names, want) {
		t.Errorf("embedded pack directories = %v, want %v", names, want)
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
	// BP-32 widened the corpus to every NESTED instruction file (O23: the
	// rule governing internal/extractor/*.go lives in internal/CLAUDE.md and
	// was never compiled), with the build/vendor trees excluded.
	wantInclude := []string{"CLAUDE.md", "ADR.md", "docs/plans/README.md", "**/CLAUDE.md"}
	if !reflect.DeepEqual(def.CorpusInclude, wantInclude) {
		t.Errorf("CorpusInclude = %v, want %v", def.CorpusInclude, wantInclude)
	}
	wantExclude := []string{"third_party/**", ".claude/worktrees/**", "workspace/**", "node_modules/**", "**/testdata/**"}
	if !reflect.DeepEqual(def.CorpusExclude, wantExclude) {
		t.Errorf("CorpusExclude = %v, want %v", def.CorpusExclude, wantExclude)
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
	want := []string{"adr-rules", "findings", "obligations", "spec-bindings"}
	if !reflect.DeepEqual(names, want) {
		t.Errorf("LayerNames() = %v, want %v (sorted)", names, want)
	}
}

// TestLoadLayer_Findings pins the D3 findings layer's manifest identity
// (FT-1): kind "finding", corpus findings/**, tier sme-stated — the honest
// human-review-conclusion tier, ranked below doc-claimed/verified by the
// shipped tier ordering.
func TestLoadLayer_Findings(t *testing.T) {
	def, err := LoadLayer("findings")
	if err != nil {
		t.Fatalf("LoadLayer(findings): %v", err)
	}
	if def.Name != "findings" || def.Version != "v1" || def.Kind != "finding" {
		t.Errorf("identity = %+v", def)
	}
	wantInclude := []string{"findings/**"}
	if !reflect.DeepEqual(def.CorpusInclude, wantInclude) {
		t.Errorf("CorpusInclude = %v, want %v", def.CorpusInclude, wantInclude)
	}
	if def.Tier != "sme-stated" {
		t.Errorf("Tier = %q, want sme-stated", def.Tier)
	}
	if def.Budgets.MaxRuleWords != 28 {
		t.Errorf("Budgets.MaxRuleWords = %d, want 28 (schema default)", def.Budgets.MaxRuleWords)
	}
}

// TestLoadLayer_ADRRules pins the LC-5 layer-2 proof pack's manifest
// identity (docs/plans/layer-catalog.md): kind "adr-rule", corpus ADR.md +
// docs/design/*.md, doc-claimed tier — architectural constraints restated
// from decision records, citation-grounded like obligations.
func TestLoadLayer_ADRRules(t *testing.T) {
	def, err := LoadLayer("adr-rules")
	if err != nil {
		t.Fatalf("LoadLayer(adr-rules): %v", err)
	}
	if def.Name != "adr-rules" || def.Version != "v1" || def.Kind != "adr-rule" {
		t.Errorf("identity = %+v", def)
	}
	wantInclude := []string{"ADR.md", "docs/design/*.md"}
	if !reflect.DeepEqual(def.CorpusInclude, wantInclude) {
		t.Errorf("CorpusInclude = %v, want %v", def.CorpusInclude, wantInclude)
	}
	if def.Tier != "doc-claimed" {
		t.Errorf("Tier = %q, want doc-claimed", def.Tier)
	}
	if def.PromptText == "" {
		t.Error("PromptText not resolved for the embedded adr-rules pack")
	}
}

// TestLoadLayer_SpecBindings pins the LC-6 SDD pack's manifest identity: kind
// "spec-binding", corpus docs/plans/** excluding README.md (LC-6's
// motivating case for wiring CorpusExclude, corpus_test.go), doc-claimed
// tier.
func TestLoadLayer_SpecBindings(t *testing.T) {
	def, err := LoadLayer("spec-bindings")
	if err != nil {
		t.Fatalf("LoadLayer(spec-bindings): %v", err)
	}
	if def.Name != "spec-bindings" || def.Version != "v1" || def.Kind != "spec-binding" {
		t.Errorf("identity = %+v", def)
	}
	wantInclude := []string{"docs/plans/**"}
	if !reflect.DeepEqual(def.CorpusInclude, wantInclude) {
		t.Errorf("CorpusInclude = %v, want %v", def.CorpusInclude, wantInclude)
	}
	wantExclude := []string{"docs/plans/README.md"}
	if !reflect.DeepEqual(def.CorpusExclude, wantExclude) {
		t.Errorf("CorpusExclude = %v, want %v", def.CorpusExclude, wantExclude)
	}
	if def.Tier != "doc-claimed" {
		t.Errorf("Tier = %q, want doc-claimed", def.Tier)
	}
}

// TestLoadLayer_EmbeddedPacksAreConcernStratum is TX-2's own coverage
// (docs/plans/pack-taxonomy-adoption.md, ADR-024): every pack fathom ships
// embedded is fathom-authored and eval-gated-embedded, hence "concern"
// stratum — including obligations, whose corpus is org-specific content but
// whose pack (prompt/corpus-selector/eval) is not (see
// docs/design/pack-taxonomy.md's "The obligations call" for the documented
// reasoning). None of the four shipped packs' layer.cue relies on the CUE
// default here — each explicitly declares stratum: "concern" — so this test
// also proves decodePack actually threads the field through, not just that
// the schema default happens to be right.
func TestLoadLayer_EmbeddedPacksAreConcernStratum(t *testing.T) {
	for _, name := range []string{"obligations", "findings", "adr-rules", "spec-bindings"} {
		def, err := LoadLayer(name)
		if err != nil {
			t.Fatalf("LoadLayer(%s): %v", name, err)
		}
		if def.Stratum != "concern" {
			t.Errorf("LoadLayer(%s).Stratum = %q, want \"concern\"", name, def.Stratum)
		}
	}
}

// ─── wave O4, item 2: BudgetOverride ─────────────────────────────────────────

// TestBudgetOverride_Apply pins BudgetOverride.Apply's contract: nil fields
// leave the layer's own budget untouched, non-nil fields replace it
// outright (0 included — the "unlimited" sentinel enforceBudgets' own
// ">0"/"MaxUSDPerRun>0" checks already treat that way), and a nil
// *BudgetOverride is a complete no-op that never mutates the layer it was
// given (Apply returns a new value, not a pointer into the caller's own
// LayerDef).
func TestBudgetOverride_Apply(t *testing.T) {
	base := LayerDef{Name: "obligations", Budgets: LayerBudgets{MaxTokensPerRun: 200000, MaxUSDPerRun: 1.0}}

	t.Run("nil override is a no-op", func(t *testing.T) {
		var o *BudgetOverride
		got := o.Apply(base)
		if got.Budgets != base.Budgets || got.Name != base.Name {
			t.Errorf("Apply(nil) = %+v, want the layer unchanged (%+v)", got, base)
		}
	})

	t.Run("nil fields leave the layer's own budgets untouched", func(t *testing.T) {
		got := (&BudgetOverride{}).Apply(base)
		if got.Budgets != base.Budgets {
			t.Errorf("Apply({}) changed budgets: got %+v, want unchanged %+v", got.Budgets, base.Budgets)
		}
	})

	t.Run("a non-nil MaxTokensPerRun replaces the layer's own value, including to 0 (unlimited)", func(t *testing.T) {
		zero := 0
		got := (&BudgetOverride{MaxTokensPerRun: &zero}).Apply(base)
		if got.Budgets.MaxTokensPerRun != 0 {
			t.Errorf("MaxTokensPerRun = %d, want 0 (unlimited override)", got.Budgets.MaxTokensPerRun)
		}
		if got.Budgets.MaxUSDPerRun != base.Budgets.MaxUSDPerRun {
			t.Errorf("MaxUSDPerRun changed when only MaxTokensPerRun was overridden: got %v, want unchanged %v", got.Budgets.MaxUSDPerRun, base.Budgets.MaxUSDPerRun)
		}
	})

	t.Run("a non-nil MaxUSDPerRun replaces the layer's own value", func(t *testing.T) {
		big := 500.0
		got := (&BudgetOverride{MaxUSDPerRun: &big}).Apply(base)
		if got.Budgets.MaxUSDPerRun != 500.0 {
			t.Errorf("MaxUSDPerRun = %v, want 500.0", got.Budgets.MaxUSDPerRun)
		}
		if got.Budgets.MaxTokensPerRun != base.Budgets.MaxTokensPerRun {
			t.Errorf("MaxTokensPerRun changed when only MaxUSDPerRun was overridden: got %d, want unchanged %d", got.Budgets.MaxTokensPerRun, base.Budgets.MaxTokensPerRun)
		}
	})

	t.Run("Apply never mutates the layer it was given", func(t *testing.T) {
		zero := 0
		original := base.Budgets
		_ = (&BudgetOverride{MaxTokensPerRun: &zero}).Apply(base)
		if base.Budgets != original {
			t.Errorf("Apply mutated the caller's own LayerDef: got %+v, want unchanged %+v", base.Budgets, original)
		}
	})
}
