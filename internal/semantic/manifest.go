package semantic

// manifest.go — M1 (docs/plans/m1-manifest.md): schema/semantic/schema.cue
// stops being documentation and becomes a runtime artifact. LoadLayer embeds
// a package-local, byte-identical copy of the #Layer schema (manifest_test.go
// fails the gate on drift — see TestManifestSchemaMatchesSource) and decodes
// #Layer instances via cuelang.org/go, so corpus globs, tier, budgets, and
// serving caps all come from ONE authoritative source
// (schema/semantic/schema.cue) instead of hand-synced Go mirrors.
//
// LC-4 (docs/plans/layer-catalog.md, the pack-shape migration): a layer
// INSTANCE is no longer a top-level field of schema.cue — each built-in ships
// as a module pack, schema/semantic/layers/<name>/{layer.cue,prompt.md,
// eval/,KILL.md} (docs/vision/02-shape.md §3's exact shape), mirrored
// byte-identically into internal/semantic/layers/ (layer.cue + prompt.md
// only — eval/ and KILL.md are dev-time/gate artifacts, never needed by the
// compiled binary) and go:embed'd here. schema.cue keeps ONLY the #Layer
// definition; loadManifest compiles it once and unifies each pack's
// standalone (no package clause, no "#Layer &" wrapper) layer.cue data
// against it — a pack author edits values, never CUE.
import (
	"embed"
	"fmt"
	"io/fs"
	"path"
	"sort"
	"strings"

	"cuelang.org/go/cue"
	"cuelang.org/go/cue/cuecontext"
)

// manifestSchema is byte-identical to schema/semantic/schema.cue — kept in
// sync by TestManifestSchemaMatchesSource (manifest_test.go). Embedding it
// here (rather than reading schema/semantic/schema.cue off disk at runtime)
// is what makes the manifest a compiled-in artifact of the fathom binary.
// Post-LC-4 this file holds ONLY the #Layer definition — no instances.
//
//go:embed manifest_schema.cue
var manifestSchema []byte

// embeddedLayers mirrors schema/semantic/layers/<name>/{layer.cue,prompt.md}
// byte-identically (TestPackFilesMatchSource, manifest_test.go — extends the
// TestManifestSchemaMatchesSource/TestPromptFilesMatchSource drift-gate
// family to the pack-shape files, LC-4). Every directory under "layers" is
// one embedded (built-in) module pack.
//
//go:embed layers
var embeddedLayers embed.FS

// LayerDef is the decoded runtime form of one #Layer instance. It is the
// single source of a layer's corpus selector, tier, and budgets — nothing in
// internal/semantic hand-mirrors these values (M1-2).
type LayerDef struct {
	Name    string
	Version string
	Kind    string

	CorpusInclude []string
	CorpusExclude []string

	// PromptVersion is the manifest's ADVISORY prompt-version label — NOT
	// the compile-time identity. A layer's real identity is content-addressed
	// from its resolved prompt text (promptForLayer, prompts.go); see
	// #Layer.prompt_version's doc comment in schema.cue.
	PromptVersion string

	// PromptFile is the repo-relative path to this layer's compile system
	// prompt (LC-1) — declared manifest metadata, shown by `fathom layers
	// show` for provenance. Not itself used to locate the file at runtime.
	PromptFile string

	// PromptText is this layer's compile system prompt, resolved by
	// decodePack at load time (LC-3): the pack's prompt.md, read as a SIBLING
	// of layer.cue from whichever source (embedded or project-local disk)
	// this pack was loaded from. Empty when the pack's prompt.md could not be
	// read — promptForLayer (prompts.go) is where that becomes a loud error,
	// deliberately deferred past load time so `fathom layers list`/`show` can
	// still enumerate a pack whose prompt is missing instead of failing
	// every layer's lookup over one broken pack.
	PromptText string

	Tier        string
	TierProfile string

	// Role is this pack's serving role (schema.cue #Layer.role, ADR-032 §4
	// D1/D2): RoleGoverns for a layer that restates binding rules a human
	// wrote about the code, RoleClaims for a layer that restates what a
	// document claims about it. ObligationsFor splits the serving budget on
	// it — governs rules fill it first. Empty only for a pack that failed to
	// decode; LayerRole normalizes an unknown value to RoleGoverns, the
	// schema default, so a layer can never lose priority by surprise.
	Role string

	// Stratum names this pack instance's place in the four-stratum pack
	// taxonomy (ADR-024, docs/design/pack-taxonomy.md): "concern" (a
	// fathom-authored, eval-gated-embedded pack — every pack this repo
	// ships), "platform" (an org's own pack, authored and eval-gated
	// on-disk under a project's .fathom/layers/, the D3 trust seam) or
	// "tech" (a layer a pinned tech pack ships, FD-18).
	// Vocabulary only — it never changes resolution order or the eval
	// gate; see schema.cue's #Layer.stratum doc comment.
	Stratum string

	// Evaled declares whether this pack has a passing eval/RESULTS.md on
	// record (schema.cue #Layer.evaled's doc comment) — self-declared data,
	// since eval/ is deliberately excluded from the embedded pack mirror and
	// so isn't runtime-inspectable for built-in packs. `fathom compile`
	// refuses a non-dry-run compile of an evaled:false layer unless
	// --unsafe-skip-eval-gate overrides it (D3-1, docs/plans/expert-depth.md).
	Evaled bool

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

// BudgetOverride (wave O4, item 2 — instrument truth for a bench compile:
// docs/bench readings found a scenariobench subject's obligations compile
// stopping at budget_stopped 14/60 sections, so obligations served root
// boilerplate instead of the nested rule a task actually needed) lets a
// caller override one or both of a resolved layer's own MaxTokensPerRun/
// MaxUSDPerRun for a single compile call, without editing layer.cue. A nil
// field leaves that budget exactly as the layer defines it; a non-nil field
// REPLACES it outright (0 or negative means "unlimited" for both fields —
// the same convention enforceBudgets already applies to MaxTokensPerRun,
// extended to MaxUSDPerRun by this same wave). Threaded from
// `fathom compile --max-tokens-per-run/--max-usd-per-run` (cmd/fathom/
// compile.go, direct --store mode only) through pkg/client.
// CompileLayerWithBudget.
type BudgetOverride struct {
	MaxTokensPerRun *int
	MaxUSDPerRun    *float64
}

// Apply returns layer with any non-nil override field replacing the
// matching Budgets field — a pure function, never mutating layer's own
// Budgets in place, so a caller holding the original LayerDef (e.g. for
// logging) never sees it silently change under it. A nil o is a no-op,
// returning layer unchanged.
func (o *BudgetOverride) Apply(layer LayerDef) LayerDef {
	if o == nil {
		return layer
	}
	if o.MaxTokensPerRun != nil {
		layer.Budgets.MaxTokensPerRun = *o.MaxTokensPerRun
	}
	if o.MaxUSDPerRun != nil {
		layer.Budgets.MaxUSDPerRun = *o.MaxUSDPerRun
	}
	return layer
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
	Role          string `json:"role"`
	Stratum       string `json:"stratum"`
	Evaled        bool   `json:"evaled"`
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

// LoadLayer decodes the named layer instance from the embedded (built-in)
// packs. Unknown names return an error naming every layer the manifest
// defines, so callers (cmd/fathom compile, the compile_layer MCP tool)
// surface the live layer list instead of a hardcoded guess (M1-3).
//
// LoadLayer is embedded-only — it never consults a project's
// .fathom/layers/ (LC-3's LoadLayerFrom does). This is deliberate: callers
// with no natural project root (schema-level validation, the built-in
// default-obligations fallback in Compile, DriftedFindings' serve-time
// hot path) get a fast, side-effect-free lookup that can never depend on the
// filesystem outside the binary.
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

// LayerNames returns the sorted names of every embedded (built-in) layer.
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

// loadManifest compiles the #Layer definition from the embedded schema, then
// compiles and unifies every embedded pack's layer.cue against it (LC-4).
func loadManifest() (map[string]LayerDef, error) {
	cctx := cuecontext.New()
	layerSchema, err := compileLayerSchema(cctx)
	if err != nil {
		return nil, err
	}

	names, err := packNames(embeddedLayers, "layers")
	if err != nil {
		return nil, fmt.Errorf("semantic: list embedded layer packs: %w", err)
	}
	layers := map[string]LayerDef{}
	for _, name := range names {
		def, err := decodePack(cctx, layerSchema, embeddedLayers, "layers/"+name+"/layer.cue", name)
		if err != nil {
			return nil, err
		}
		layers[name] = def
	}
	return layers, nil
}

// compileLayerSchema compiles the embedded #Layer definition and returns it
// as a Value ready to Unify against a pack's data.
func compileLayerSchema(cctx *cue.Context) (cue.Value, error) {
	v := cctx.CompileBytes(manifestSchema, cue.Filename("schema/semantic/schema.cue"))
	if err := v.Err(); err != nil {
		return cue.Value{}, fmt.Errorf("semantic: compile layer schema: %w", err)
	}
	def := v.LookupPath(cue.ParsePath("#Layer"))
	if err := def.Err(); err != nil {
		return cue.Value{}, fmt.Errorf("semantic: schema has no #Layer definition: %w", err)
	}
	return def, nil
}

// packNames lists the pack directory names present under root in fsys (one
// per module pack — LC-3 calls this against project-local disk too). Hidden
// directories (".install-*" staging dirs, editor debris) are never packs.
func packNames(fsys fs.FS, root string) ([]string, error) {
	entries, err := fs.ReadDir(fsys, root)
	if err != nil {
		return nil, err
	}
	var names []string
	for _, e := range entries {
		if e.IsDir() && !strings.HasPrefix(e.Name(), ".") {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names)
	return names, nil
}

// decodePack reads, compiles, and validates one pack's layer.cue (plain
// data — no package clause, no "#Layer &" wrapper) against layerSchema,
// returning its decoded LayerDef. fsys/path abstract over embedded vs (LC-3)
// project-local disk sources, so this is the ONE place that turns
// "a directory with a layer.cue in it" into a LayerDef, embedded or not.
func decodePack(cctx *cue.Context, layerSchema cue.Value, fsys fs.FS, cuePath, dirName string) (LayerDef, error) {
	raw, err := fs.ReadFile(fsys, cuePath)
	if err != nil {
		return LayerDef{}, fmt.Errorf("semantic: pack %q: read layer.cue: %w", dirName, err)
	}
	packVal := cctx.CompileBytes(raw, cue.Filename(cuePath))
	if err := packVal.Err(); err != nil {
		return LayerDef{}, fmt.Errorf("semantic: pack %q: compile layer.cue: %w", dirName, err)
	}
	unified := layerSchema.Unify(packVal)
	if err := unified.Validate(cue.Concrete(false)); err != nil {
		return LayerDef{}, fmt.Errorf("semantic: pack %q: layer.cue does not satisfy #Layer: %w", dirName, err)
	}
	var cl cueLayer
	if err := unified.Decode(&cl); err != nil {
		return LayerDef{}, fmt.Errorf("semantic: pack %q: decode layer.cue: %w", dirName, err)
	}
	if cl.Name != dirName {
		return LayerDef{}, fmt.Errorf("semantic: pack directory %q declares name %q — a pack's directory name and its layer.cue name field must match", dirName, cl.Name)
	}

	// Prompt text is a SIBLING of layer.cue within the same pack directory
	// and the same fsys — resolved here, not via string-matching prompt_file
	// against a fixed root, so a project-local pack's prompt (LC-3) resolves
	// exactly like an embedded one's. A read failure is left for
	// promptForLayer to surface loudly at compile time (see PromptText's doc
	// comment above).
	promptText := ""
	if raw, perr := fs.ReadFile(fsys, path.Join(path.Dir(cuePath), "prompt.md")); perr == nil {
		promptText = string(raw)
	}

	return LayerDef{
		Name:          cl.Name,
		Version:       cl.Version,
		Kind:          cl.Kind,
		CorpusInclude: cl.Corpus.Include,
		CorpusExclude: cl.Corpus.Exclude,
		PromptVersion: cl.PromptVersion,
		PromptFile:    cl.PromptFile,
		PromptText:    promptText,
		Tier:          cl.Tier,
		TierProfile:   cl.TierProfile,
		Role:          cl.Role,
		Stratum:       cl.Stratum,
		Evaled:        cl.Evaled,
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
	}, nil
}
