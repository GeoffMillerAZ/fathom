package modspec

// manifest.go — the manifest loader docs/spec/expert-module-v0.md §7 names
// as the honest gap: "no manifest loader ships in this repo... there is no
// `fathom module` CLI verb or Go package that reads a manifest at runtime
// and acts on it." This closes that gap (Phase J, docs/vision/13-expert-
// suite.md §4/§9 row J).
//
// Two distinct validation strengths, matching §3's own reconciliation of
// "closedness is an authoring gate, not a reader-conformance instrument":
//
//   - ValidateStrict unifies a module.cue instance against the embedded,
//     CLOSED #Module definition (mirroring internal/semantic's decodePack
//     for #Layer) and requires full concreteness. This is the AUTHORING
//     gate — `fathom module init` runs it against what it just wrote, and a
//     human hand-editing module.cue can run it to catch a typo'd field name.
//     A manifest with an unrecognized field FAILS this check, deliberately.
//
//   - LoadManifest is the READER: it compiles the instance, decodes only
//     the fields THIS package knows about (extra CUE fields are never
//     inspected, so they can never trigger a rejection — the exact
//     "reader MUST ignore unknown fields" contract §3 states), and then
//     hand-validates the required-ness/format rules for the fields it does
//     know, producing FieldError values that name the file and the CUE
//     field path. A manifest authored under a future #Module revision with
//     additional fields therefore still LOADS under this reader, even
//     though it would fail ValidateStrict against THIS revision's closed
//     schema (it wouldn't — ValidateStrict is authoring-time only, never
//     applied to a foreign manifest by this package).
import (
	_ "embed"
	"fmt"
	"os"
	"regexp"
	"strings"

	"cuelang.org/go/cue"
	"cuelang.org/go/cue/cuecontext"
)

// manifestSchema is byte-identical to schema/module/schema.cue — kept in
// sync by TestManifestSchemaMatchesSource (manifest_test.go), the same
// drift-gate convention internal/semantic/manifest.go already uses for
// #Layer.
//
//go:embed manifest_schema.cue
var manifestSchema []byte

// Manifest is the decoded runtime form of one #Module instance
// (docs/spec/expert-module-v0.md §3).
type Manifest struct {
	Name        string
	Version     string
	Description string

	Corpus Corpus

	// Watermark is the corpus freshness token at compile time — the
	// freshness doctrine (§3.3): a module without one is invalid.
	Watermark string

	// TierProfile names the tier vocabulary this module's claims use;
	// defaults to "engineering" when the manifest omits it (§3.4).
	TierProfile string

	EvalEvidence EvalEvidence
	Recompile    Recompile

	// License defaults to "unspecified" when the manifest omits it (§3.7).
	License string

	Provenance Provenance
}

// Corpus selects the module's source material (§3.2).
type Corpus struct {
	Include []string
	Exclude []string
}

// EvalEvidence is the pack-taxonomy D3 trust seam applied to a module
// (§3.5): an evaled:true claim MUST point at on-disk evidence.
type EvalEvidence struct {
	Evaled bool
	Path   string
}

// Recompile states whether and how a consumer may recompile this module
// (§3.6).
type Recompile struct {
	Allowed        bool
	PromptIdentity string
}

// Provenance records who/what compiled this module and when (§3.7).
type Provenance struct {
	CompiledBy string
	CompiledAt string
}

// FieldError names one manifest field that failed to load or validate —
// the file it came from, the CUE field path, and the underlying cause. A
// caller (cmd/fathom module verbs) prints Field/Err directly rather than a
// bare CUE diagnostic.
type FieldError struct {
	Path  string // manifest file path
	Field string // CUE field path, e.g. "watermark", "eval_evidence.path"
	Err   error
}

func (e *FieldError) Error() string {
	return fmt.Sprintf("%s: field %q: %v", e.Path, e.Field, e.Err)
}

func (e *FieldError) Unwrap() error { return e.Err }

// LoadError collects every FieldError a single LoadManifest/ValidateStrict
// call found, so a caller sees every problem in one pass instead of
// fixing-and-rerunning field by field.
type LoadError struct {
	Path   string
	Fields []*FieldError
}

func (e *LoadError) Error() string {
	msgs := make([]string, 0, len(e.Fields))
	for _, f := range e.Fields {
		msgs = append(msgs, fmt.Sprintf("field %q: %v", f.Field, f.Err))
	}
	return fmt.Sprintf("modspec: %s: %s", e.Path, strings.Join(msgs, "; "))
}

func fieldErr(path, field string, err error) *FieldError {
	return &FieldError{Path: path, Field: field, Err: err}
}

// nameRe/versionRe/watermarkRe/compiledAtRe mirror schema/module/schema.cue's
// regexes exactly — the reader enforces the same formats the authoring
// schema does, just without unifying against the closed struct (see the
// package doc above for why).
var (
	nameRe       = regexp.MustCompile(`^[a-z][a-z0-9-]*$`)
	versionRe    = regexp.MustCompile(`^v[0-9]+$`)
	watermarkRe  = regexp.MustCompile(`^([0-9a-f]{40}|manifest:[0-9a-f]{64})$`)
	compiledAtRe = regexp.MustCompile(`^[0-9]{4}-[0-9]{2}-[0-9]{2}T[0-9]{2}:[0-9]{2}:[0-9]{2}`)
)

// LoadManifest reads and decodes the module manifest at path — the
// conforming READER (see package doc): unknown CUE fields are never
// inspected, so a manifest authored under a newer #Module revision still
// loads. Required-ness and format are enforced in Go against exactly the
// fields this package knows about; every violation is collected into one
// *LoadError naming path + field, rather than stopping at the first one.
func LoadManifest(path string) (Manifest, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Manifest{}, fmt.Errorf("modspec: read %s: %w", path, err)
	}
	cctx := cuecontext.New()
	v := cctx.CompileBytes(data, cue.Filename(path))
	if v.Err() != nil {
		return Manifest{}, &LoadError{Path: path, Fields: []*FieldError{fieldErr(path, "<parse>", v.Err())}}
	}

	m := Manifest{
		Name:        lookupString(v, "name"),
		Version:     lookupString(v, "version"),
		Description: lookupString(v, "description"),
		Corpus: Corpus{
			Include: lookupStrings(v, "corpus.include"),
			Exclude: lookupStrings(v, "corpus.exclude"),
		},
		Watermark:   lookupString(v, "watermark"),
		TierProfile: lookupStringDefault(v, "tier_profile", "engineering"),
		EvalEvidence: EvalEvidence{
			Evaled: lookupBoolDefault(v, "eval_evidence.evaled", false),
			Path:   lookupString(v, "eval_evidence.path"),
		},
		Recompile: Recompile{
			Allowed:        lookupBoolDefault(v, "recompile.allowed", true),
			PromptIdentity: lookupString(v, "recompile.prompt_identity"),
		},
		License: lookupStringDefault(v, "license", "unspecified"),
		Provenance: Provenance{
			CompiledBy: lookupString(v, "provenance.compiled_by"),
			CompiledAt: lookupString(v, "provenance.compiled_at"),
		},
	}

	if errs := validateManifest(path, m); len(errs) > 0 {
		return Manifest{}, &LoadError{Path: path, Fields: errs}
	}
	return m, nil
}

// validateManifest applies §3's required-ness/format rules to the fields
// this reader decoded, mirroring schema/module/testdata/invalid.cue's six
// cases plus §3.1/§3.7's MUST-be-present fields.
func validateManifest(path string, m Manifest) []*FieldError {
	var errs []*FieldError
	if !nameRe.MatchString(m.Name) {
		errs = append(errs, fieldErr(path, "name", fmt.Errorf("must match %s, got %q", nameRe.String(), m.Name)))
	}
	if !versionRe.MatchString(m.Version) {
		errs = append(errs, fieldErr(path, "version", fmt.Errorf("must match %s, got %q", versionRe.String(), m.Version)))
	}
	if m.Description == "" {
		errs = append(errs, fieldErr(path, "description", fmt.Errorf("required, MUST be present")))
	}
	if len(m.Corpus.Include) == 0 {
		errs = append(errs, fieldErr(path, "corpus.include", fmt.Errorf("must be non-empty")))
	}
	if !watermarkRe.MatchString(m.Watermark) {
		errs = append(errs, fieldErr(path, "watermark", fmt.Errorf("a module without a watermark is invalid (docs/spec/expert-module-v0.md §3.3): must be a 40-hex git SHA or manifest:+64-hex, got %q", m.Watermark)))
	}
	if m.EvalEvidence.Evaled && m.EvalEvidence.Path == "" {
		errs = append(errs, fieldErr(path, "eval_evidence.path", fmt.Errorf("required when eval_evidence.evaled is true")))
	}
	if m.Recompile.Allowed && m.Recompile.PromptIdentity == "" {
		errs = append(errs, fieldErr(path, "recompile.prompt_identity", fmt.Errorf("required when recompile.allowed is true")))
	}
	if m.Provenance.CompiledBy == "" {
		errs = append(errs, fieldErr(path, "provenance.compiled_by", fmt.Errorf("required, MUST be present")))
	}
	if !compiledAtRe.MatchString(m.Provenance.CompiledAt) {
		errs = append(errs, fieldErr(path, "provenance.compiled_at", fmt.Errorf("must be RFC3339, got %q", m.Provenance.CompiledAt)))
	}
	return errs
}

// lookupString returns the string at dotted path p, or "" when absent /
// not a concrete string — LoadManifest's own validateManifest pass is what
// turns an empty required field into a named error, so a missing field is
// never silently treated as valid here.
func lookupString(v cue.Value, p string) string {
	fv := v.LookupPath(cue.ParsePath(p))
	if !fv.Exists() {
		return ""
	}
	s, err := fv.String()
	if err != nil {
		return ""
	}
	return s
}

// lookupStringDefault mirrors lookupString but returns def when the field
// does not exist at all (cue.Value.Exists distinguishes "absent" from "set
// to the empty string") — this is how the reader applies §3.4/§3.7's schema
// defaults (tier_profile: "engineering", license: "unspecified") without
// unifying against the schema.
func lookupStringDefault(v cue.Value, p, def string) string {
	fv := v.LookupPath(cue.ParsePath(p))
	if !fv.Exists() {
		return def
	}
	s, err := fv.String()
	if err != nil {
		return def
	}
	return s
}

func lookupBoolDefault(v cue.Value, p string, def bool) bool {
	fv := v.LookupPath(cue.ParsePath(p))
	if !fv.Exists() {
		return def
	}
	b, err := fv.Bool()
	if err != nil {
		return def
	}
	return b
}

func lookupStrings(v cue.Value, p string) []string {
	fv := v.LookupPath(cue.ParsePath(p))
	if !fv.Exists() {
		return nil
	}
	var out []string
	if err := fv.Decode(&out); err != nil {
		return nil
	}
	return out
}

// ValidateStrict is the AUTHORING gate (see package doc): it unifies the
// module.cue instance at path against the embedded, CLOSED #Module
// definition and requires full concreteness — an unrecognized field name,
// a missing required field, or an abstract (unset) required field all fail
// here, exactly like schema/module/testdata/invalid.cue's cases. Used by
// `fathom module init` to self-check what it just wrote, and available to
// a human hand-editing module.cue as a stricter check than LoadManifest.
func ValidateStrict(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("modspec: read %s: %w", path, err)
	}
	cctx := cuecontext.New()
	schemaDef, err := compileModuleSchema(cctx)
	if err != nil {
		return err
	}
	instance := cctx.CompileBytes(data, cue.Filename(path))
	if instance.Err() != nil {
		return &LoadError{Path: path, Fields: []*FieldError{fieldErr(path, "<parse>", instance.Err())}}
	}
	unified := schemaDef.Unify(instance)
	if err := unified.Validate(cue.Concrete(true)); err != nil {
		return &LoadError{Path: path, Fields: []*FieldError{fieldErr(path, "<schema>", err)}}
	}
	return nil
}

// compileModuleSchema compiles the embedded #Module definition, ready to
// Unify against a module.cue instance — mirrors internal/semantic's
// compileLayerSchema for #Layer.
func compileModuleSchema(cctx *cue.Context) (cue.Value, error) {
	v := cctx.CompileBytes(manifestSchema, cue.Filename("schema/module/schema.cue"))
	if err := v.Err(); err != nil {
		return cue.Value{}, fmt.Errorf("modspec: compile module schema: %w", err)
	}
	def := v.LookupPath(cue.ParsePath("#Module"))
	if err := def.Err(); err != nil {
		return cue.Value{}, fmt.Errorf("modspec: schema has no #Module definition: %w", err)
	}
	return def, nil
}
