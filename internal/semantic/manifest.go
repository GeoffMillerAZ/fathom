package semantic

// manifest.go — M1 (docs/plans/m1-manifest.md): schema/semantic/schema.cue
// stops being documentation and becomes a runtime artifact. LoadLayer embeds
// a package-local, byte-identical copy of the schema (manifest_test.go fails
// the gate on drift — see TestManifestSchemaMatchesSource) and decodes
// #Layer instances via cuelang.org/go, so corpus globs, tier, budgets, and
// serving caps all come from ONE authoritative source
// (schema/semantic/schema.cue) instead of hand-synced Go mirrors
// (ObligationsInclude and the systemPrompt const, both retired — M1-2).

import (
	_ "embed"
	"fmt"
	"sort"
	"strings"

	"cuelang.org/go/cue"
	"cuelang.org/go/cue/cuecontext"
)

// manifestSchema is byte-identical to schema/semantic/schema.cue — kept in
// sync by TestManifestSchemaMatchesSource (manifest_test.go). Embedding it
// here (rather than reading schema/semantic/schema.cue off disk at runtime)
// is what makes the manifest a compiled-in artifact of the fathom binary.
//
//go:embed manifest_schema.cue
var manifestSchema []byte

// LayerDef is the decoded runtime form of one #Layer instance
// (schema/semantic/schema.cue). It is the single source of a layer's corpus
// selector, tier, and budgets — nothing in internal/semantic hand-mirrors
// these values (M1-2).
type LayerDef struct {
	Name    string
	Version string
	Kind    string

	CorpusInclude []string
	CorpusExclude []string

	// PromptVersion is the manifest's ADVISORY prompt-version label — NOT
	// the compile-time identity. The obligations layer's real identity is
	// content-addressed from its Go prompttemplate.Template
	// (obligationsPromptVersion, prompts.go); see #Layer.prompt_version's
	// doc comment in schema.cue.
	PromptVersion string

	// PromptFile is the repo-relative path to this layer's compile system
	// prompt (LC-1). promptForLayer loads it and derives PromptVersion's
	// REAL, content-addressed value from its bytes — see #Layer.prompt_file's
	// doc comment in schema.cue.
	PromptFile string

	Tier        string
	TierProfile string

	Budgets LayerBudgets
	Serving LayerServing
}

// LayerBudgets bounds one compile run (schema.cue #Layer.budgets); the
// runner refuses to exceed them (ADR-016 §4.1, enforced by enforceBudgets —
// M1-7).
type LayerBudgets struct {
	MaxTokensPerRun  int
	MaxUSDPerRun     float64
	MaxRuleWords     int
	MaxClaimsPerFile int
}

// LayerServing caps the decision-moment payload (schema.cue #Layer.serving).
type LayerServing struct {
	MaxRules  int
	MaxTokens int
	// DriftTracked (schema.cue #Layer.serving.drift_tracked, LC-2) gates
	// remediation-drift anchoring: when true, verifyDraft persists scope-fp
	// links at compile time and DriftedFindings (sweep.go) computes
	// code_drifted for this layer's claims at serve time. Replaces the old
	// kind=="finding" Go hardcode at both sites — a schema field, not a type
	// check, so a future layer opts in by setting one CUE field.
	DriftTracked bool
}

// cueLayer mirrors #Layer's JSON shape for decoding. cuelang.org/go's
// Value.Decode follows the same json struct-tag convention as
// encoding/json, matching the field names in schema.cue exactly.
type cueLayer struct {
	Name    string `json:"name"`
	Version string `json:"version"`
	Kind    string `json:"kind"`
	Corpus  struct {
		Include []string `json:"include"`
		Exclude []string `json:"exclude"`
	} `json:"corpus"`
	PromptVersion string `json:"prompt_version"`
	PromptFile    string `json:"prompt_file"`
	Tier          string `json:"tier"`
	TierProfile   string `json:"tier_profile"`
	Budgets       struct {
		MaxTokensPerRun  int     `json:"max_tokens_per_run"`
		MaxUSDPerRun     float64 `json:"max_usd_per_run"`
		MaxRuleWords     int     `json:"max_rule_words"`
		MaxClaimsPerFile int     `json:"max_claims_per_file"`
	} `json:"budgets"`
	Serving struct {
		MaxRules     int  `json:"max_rules"`
		MaxTokens    int  `json:"max_tokens"`
		DriftTracked bool `json:"drift_tracked"`
	} `json:"serving"`
}

// LoadLayer decodes the named layer instance from the embedded manifest.
// Unknown names return an error naming every layer the manifest defines, so
// callers (cmd/fathom compile, the compile_layer MCP tool) surface the live
// layer list instead of a hardcoded guess (M1-3).
func LoadLayer(name string) (LayerDef, error) {
	layers, err := loadManifest()
	if err != nil {
		return LayerDef{}, err
	}
	def, ok := layers[name]
	if !ok {
		return LayerDef{}, fmt.Errorf("semantic: unknown layer %q (known layers: %s)", name, strings.Join(layerNames(layers), ", "))
	}
	return def, nil
}

// LayerNames returns the sorted names of every layer the manifest defines.
func LayerNames() ([]string, error) {
	layers, err := loadManifest()
	if err != nil {
		return nil, err
	}
	return layerNames(layers), nil
}

func layerNames(layers map[string]LayerDef) []string {
	names := make([]string, 0, len(layers))
	for n := range layers {
		names = append(names, n)
	}
	sort.Strings(names)
	return names
}

// loadManifest compiles the embedded schema and decodes every regular
// top-level field (i.e. every #Layer instance — Value.Fields excludes the
// #Layer definition itself, which is a CUE definition field, by default).
func loadManifest() (map[string]LayerDef, error) {
	cctx := cuecontext.New()
	v := cctx.CompileBytes(manifestSchema)
	if err := v.Err(); err != nil {
		return nil, fmt.Errorf("semantic: compile manifest: %w", err)
	}
	if err := v.Validate(cue.Concrete(false)); err != nil {
		return nil, fmt.Errorf("semantic: validate manifest: %w", err)
	}
	iter, err := v.Fields()
	if err != nil {
		return nil, fmt.Errorf("semantic: iterate manifest layers: %w", err)
	}
	layers := map[string]LayerDef{}
	for iter.Next() {
		name := iter.Selector().Unquoted()
		var cl cueLayer
		if err := iter.Value().Decode(&cl); err != nil {
			return nil, fmt.Errorf("semantic: decode layer %q: %w", name, err)
		}
		layers[name] = LayerDef{
			Name:          cl.Name,
			Version:       cl.Version,
			Kind:          cl.Kind,
			CorpusInclude: cl.Corpus.Include,
			CorpusExclude: cl.Corpus.Exclude,
			PromptVersion: cl.PromptVersion,
			PromptFile:    cl.PromptFile,
			Tier:          cl.Tier,
			TierProfile:   cl.TierProfile,
			Budgets: LayerBudgets{
				MaxTokensPerRun:  cl.Budgets.MaxTokensPerRun,
				MaxUSDPerRun:     cl.Budgets.MaxUSDPerRun,
				MaxRuleWords:     cl.Budgets.MaxRuleWords,
				MaxClaimsPerFile: cl.Budgets.MaxClaimsPerFile,
			},
			Serving: LayerServing{
				MaxRules:     cl.Serving.MaxRules,
				MaxTokens:    cl.Serving.MaxTokens,
				DriftTracked: cl.Serving.DriftTracked,
			},
		}
	}
	return layers, nil
}
