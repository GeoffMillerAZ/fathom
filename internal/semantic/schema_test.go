package semantic

// schema_test.go — asserts the schema/semantic CUE negatives (repo
// convention: testdata/invalid.cue documents them commented-out so live
// `cue vet` stays green; this test proves each actually fails).

import (
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"testing"

	"cuelang.org/go/cue"
	"cuelang.org/go/cue/cuecontext"

	"github.com/geoffmilleraz/fathom/internal/core"
)

func schemaSource(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	path := filepath.Join(filepath.Dir(file), "..", "..", "schema", "semantic", "schema.cue")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read schema: %v", err)
	}
	return string(data)
}

func TestLayerSchema_Negatives(t *testing.T) {
	schema := schemaSource(t)
	cases := []struct {
		name string
		bad  string
	}{
		{"empty corpus include", `
bad: #Layer & {
	name: "x", version: "v1", kind: "x"
	corpus: include: []
	prompt_version: "x/v1"
	tier: "doc-claimed"
	budgets: {max_tokens_per_run: 1, max_usd_per_run: 0}
}`},
		{"serving cap above payload contract", `
bad: #Layer & {
	name: "x", version: "v1", kind: "x"
	corpus: include: ["docs/**"]
	prompt_version: "x/v1"
	tier: "doc-claimed"
	budgets: {max_tokens_per_run: 1, max_usd_per_run: 0}
	serving: max_rules: 12
}`},
		{"tier outside the v1+M1 claim model (ai-derived awaits its own revival evidence, ADR-016 §4.2)", `
bad: #Layer & {
	name: "x", version: "v1", kind: "x"
	corpus: include: ["docs/**"]
	prompt_version: "x/v1"
	tier: "ai-derived"
	budgets: {max_tokens_per_run: 1, max_usd_per_run: 0}
}`},
		{"stratum outside the three admitted values (ADR-024: dialect has no #Layer instance; provider was renamed tech)", `
bad: #Layer & {
	name: "x", version: "v1", kind: "x"
	corpus: include: ["docs/**"]
	prompt_version: "x/v1"
	tier: "doc-claimed"
	stratum: "provider"
	budgets: {max_tokens_per_run: 1, max_usd_per_run: 0}
}`},
	}
	cctx := cuecontext.New()

	// Sanity: the schema itself (with its obligations instance) validates.
	if v := cctx.CompileString(schema); v.Err() != nil || v.Validate() != nil {
		t.Fatalf("schema does not validate on its own: %v / %v", v.Err(), v.Validate())
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			v := cctx.CompileString(schema + "\n" + tc.bad)
			if v.Err() != nil {
				return // compile-time rejection is a pass
			}
			if err := v.Validate(); err == nil {
				t.Errorf("expected validation failure, got none")
			}
		})
	}
}

// TestLayerSchema_VerifiedTierAdmitted proves the M1-8 tier widening
// (ADR-016 amendment in ADR.md): "verified" — reserved for deterministic,
// Model-empty compile paths (ADR-018 oracle classes) — is now a legal
// #Layer.tier value, even though no shipped layer emits it yet.
func TestLayerSchema_VerifiedTierAdmitted(t *testing.T) {
	schema := schemaSource(t)
	good := `
verifiedLayer: #Layer & {
	name: "x", version: "v1", kind: "x"
	corpus: include: ["docs/**"]
	prompt_version: "x/v1"
	tier: "verified"
	budgets: {max_tokens_per_run: 1, max_usd_per_run: 0}
}`
	cctx := cuecontext.New()
	v := cctx.CompileString(schema + "\n" + good)
	if v.Err() != nil {
		t.Fatalf("compile: %v", v.Err())
	}
	if err := v.Validate(); err != nil {
		t.Fatalf("tier %q should validate after the M1-8 widening: %v", "verified", err)
	}
}

// TestLayerSchema_Stratum proves TX-2's additive #Layer.stratum field
// (ADR-024, docs/design/pack-taxonomy.md): unset defaults to "concern"
// (existing packs without the field keep validating), and "platform" is
// admitted for an org's own pack.
func TestLayerSchema_Stratum(t *testing.T) {
	schema := schemaSource(t)
	cctx := cuecontext.New()

	noStratum := `
noStratum: #Layer & {
	name: "x", version: "v1", kind: "x"
	corpus: include: ["docs/**"]
	prompt_version: "x/v1"
	tier: "doc-claimed"
	budgets: {max_tokens_per_run: 1, max_usd_per_run: 0}
}`
	v := cctx.CompileString(schema + "\n" + noStratum)
	if v.Err() != nil {
		t.Fatalf("compile: %v", v.Err())
	}
	if err := v.Validate(); err != nil {
		t.Fatalf("a #Layer instance with no stratum field should still validate (additive field): %v", err)
	}
	var got struct {
		Stratum string `json:"stratum"`
	}
	if err := v.LookupPath(cue.ParsePath("noStratum")).Decode(&got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got.Stratum != "concern" {
		t.Errorf("unset stratum decoded to %q, want default \"concern\"", got.Stratum)
	}

	platform := `
platformLayer: #Layer & {
	name: "x", version: "v1", kind: "x"
	corpus: include: ["docs/**"]
	prompt_version: "x/v1"
	tier: "doc-claimed"
	stratum: "platform"
	budgets: {max_tokens_per_run: 1, max_usd_per_run: 0}
}`
	v2 := cctx.CompileString(schema + "\n" + platform)
	if v2.Err() != nil {
		t.Fatalf("compile: %v", v2.Err())
	}
	if err := v2.Validate(); err != nil {
		t.Fatalf("stratum \"platform\" should validate: %v", err)
	}

	tech := `
techLayer: #Layer & {
	name: "x", version: "v1", kind: "x"
	corpus: include: ["docs/**"]
	prompt_version: "x/v1"
	tier: "doc-claimed"
	stratum: "tech"
	budgets: {max_tokens_per_run: 1, max_usd_per_run: 0}
}`
	v3 := cctx.CompileString(schema + "\n" + tech)
	if v3.Err() != nil {
		t.Fatalf("compile: %v", v3.Err())
	}
	if err := v3.Validate(); err != nil {
		t.Fatalf("stratum \"tech\" (a tech pack's layer, FD-18) should validate: %v", err)
	}
}

// findingClassSchemaSource reads schema/semantic/findingclass.cue (the
// findings-as-evidence closed vocabulary, docs/design/findings-as-
// evidence.md §3.2) the same way schemaSource reads schema.cue.
func findingClassSchemaSource(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	path := filepath.Join(filepath.Dir(file), "..", "..", "schema", "semantic", "findingclass.cue")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read findingclass.cue: %v", err)
	}
	return string(data)
}

// TestFindingClassVocabularyMatchesSchema cross-checks
// internal/core.ValidFindingClasses (the Go runtime enforcement point —
// compile/persist-time rejection of an unknown class) against
// schema/semantic/findingclass.cue's FindingClasses list, so the two
// vocabularies can never drift apart — mirrors internal/analysis/review's
// own TestClaimsVocabularyMatchesSemanticLayer precedent.
func TestFindingClassVocabularyMatchesSchema(t *testing.T) {
	cueSrc := findingClassSchemaSource(t)
	cctx := cuecontext.New()
	v := cctx.CompileString(cueSrc)
	if v.Err() != nil {
		t.Fatalf("compile findingclass.cue: %v", v.Err())
	}
	var cueClasses []string
	if err := v.LookupPath(cue.ParsePath("FindingClasses")).Decode(&cueClasses); err != nil {
		t.Fatalf("decode FindingClasses: %v", err)
	}

	var goClasses []string
	for _, c := range core.ValidFindingClasses {
		goClasses = append(goClasses, string(c))
	}

	sortedCue := append([]string(nil), cueClasses...)
	sortedGo := append([]string(nil), goClasses...)
	sort.Strings(sortedCue)
	sort.Strings(sortedGo)
	if len(sortedCue) != len(sortedGo) {
		t.Fatalf("vocabulary size mismatch: CUE has %d, Go has %d\nCUE: %v\nGo:  %v", len(sortedCue), len(sortedGo), sortedCue, sortedGo)
	}
	for i := range sortedCue {
		if sortedCue[i] != sortedGo[i] {
			t.Fatalf("vocabulary mismatch at sorted index %d: CUE=%q Go=%q\nCUE: %v\nGo:  %v", i, sortedCue[i], sortedGo[i], sortedCue, sortedGo)
		}
	}
}

// TestFindingClassSchema_UnknownClassRejected proves the closed vocabulary
// actually rejects an unrecognized class at CUE unification — the
// compile-time half of "an unknown class is REJECTED... never silently
// bucketed as other" (§3.2). The persist-time half (Go, internal/semantic's
// verifyDraft) has its own test in compile_test.go.
func TestFindingClassSchema_UnknownClassRejected(t *testing.T) {
	cueSrc := findingClassSchemaSource(t)
	cctx := cuecontext.New()
	bad := cctx.CompileString(cueSrc + "\nbad: #FindingClass & \"other\"")
	if bad.Err() != nil {
		return // compile-time rejection is a pass
	}
	if err := bad.LookupPath(cue.ParsePath("bad")).Validate(); err == nil {
		t.Error("expected \"other\" to be rejected by the closed #FindingClass vocabulary, got no error")
	}
}
