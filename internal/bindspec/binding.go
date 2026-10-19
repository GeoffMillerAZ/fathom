package bindspec

import (
	_ "embed"
	"fmt"
	"os"
	"regexp"
	"strings"

	"cuelang.org/go/cue"
	"cuelang.org/go/cue/cuecontext"

	"github.com/geoffmilleraz/fathom/internal/projectdir"
)

// bindingSchema is byte-identical to schema/binding/schema.cue — kept in
// sync by TestBindingSchemaMatchesSource, the same drift-gate convention
// internal/modspec/manifest.go established for #Module.
//
//go:embed binding_schema.cue
var bindingSchema []byte

// Binding is one decoded #Binding entry.
type Binding struct {
	Module   string
	Scopes   []string
	MaxRules int
	MinTier  string
}

// BindingSet is the decoded runtime form of a `.fathom/modules.cue` file
// (#Bindings, docs/design/platform-modules.md §3.1).
type BindingSet struct {
	Bindings []Binding
}

// FieldError names one binding field that failed to load or validate — the
// file it came from, the CUE field path (including the binding's index,
// e.g. "bindings[1].module"), and the underlying cause.
type FieldError struct {
	Path  string
	Field string
	Err   error
}

func (e *FieldError) Error() string {
	return fmt.Sprintf("%s: field %q: %v", e.Path, e.Field, e.Err)
}

func (e *FieldError) Unwrap() error { return e.Err }

// LoadError collects every FieldError a single LoadBindings/ValidateStrict
// call found.
type LoadError struct {
	Path   string
	Fields []*FieldError
}

func (e *LoadError) Error() string {
	msgs := make([]string, 0, len(e.Fields))
	for _, f := range e.Fields {
		msgs = append(msgs, fmt.Sprintf("field %q: %v", f.Field, f.Err))
	}
	return fmt.Sprintf("bindspec: %s: %s", e.Path, strings.Join(msgs, "; "))
}

func fieldErr(path, field string, err error) *FieldError {
	return &FieldError{Path: path, Field: field, Err: err}
}

// moduleNameRe mirrors schema/binding/schema.cue's #Binding.module regex
// exactly — the reader enforces the same shape the authoring schema does.
var moduleNameRe = regexp.MustCompile(`^[a-z][a-z0-9-]*$`)

// validTiers mirrors #Binding.min_tier's closed enum (schema/binding/
// schema.cue) — kept as a set literal rather than importing internal/core
// (bindspec stays a leaf package with no dependency on the evidence-tier
// vocabulary's home package, matching modspec's own leaf-package shape).
var validTiers = map[string]bool{
	"verified":    true,
	"pre-scan":    true,
	"doc-claimed": true,
	"sme-stated":  true,
	"assumed":     true,
}

// DefaultMaxRules is #Binding.max_rules' schema default.
const DefaultMaxRules = 2

// LoadBindings reads and decodes the binding file at path — the conforming
// READER (see package doc): unknown CUE fields are never inspected, so a
// binding file authored under a newer #Bindings revision still loads.
// Required-ness and format are enforced in Go against exactly the fields
// this package knows about; every violation across every binding entry is
// collected into one *LoadError, rather than stopping at the first one.
// Duplicate module names across bindings are rejected — house convention
// (the CUE schema cannot express list-element uniqueness cleanly, so this
// reader enforces it directly, same class of decision as modspec's
// Go-enforced regex/required-ness checks).
//
// Three states (O22 as revised by plan IO-7): the top-level "bindings"
// field being ABSENT — an all-commented scaffold (exactly what
// ExampleTemplate/`fathom init` write) or a blank file — is an EMPTY
// config, no error, zero Bindings. An explicit `bindings: []` is the
// SAME valid, inert state: it is the sanctioned none-configured literal
// the activate skill and the kit packs standardize, and O22's earlier
// sub-choice to reject it put two sanctioned vocabularies in the fleet —
// the fix-pass wrote the literal into an adopter repo and doctor FAILed a
// file the tooling itself prescribes. One vocabulary now: absent and
// explicit-empty both load clean; only a PRESENT-and-malformed entry
// (bad field, duplicate module, empty scopes) fails, loudly.
func LoadBindings(path string) (BindingSet, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return BindingSet{}, fmt.Errorf("bindspec: read %s: %w", path, err)
	}
	cctx := cuecontext.New()
	v := cctx.CompileBytes(data, cue.Filename(path))
	if v.Err() != nil {
		return BindingSet{}, &LoadError{Path: path, Fields: []*FieldError{fieldErr(path, "<parse>", v.Err())}}
	}
	return decodeBindings(path, v)
}

// BindingsFromProject reads the bindings key from a loaded project value
// (docs/design/fathom-dir.md section 5) with no second parse: the reading,
// value and error text, is LoadBindings' on the file that sets the key, or
// on .fathom/modules.cue when no file does.
func BindingsFromProject(p projectdir.Project) (BindingSet, error) {
	st := p.Source("bindings", projectdir.Modules)
	if st.ReadErr != nil {
		return BindingSet{}, fmt.Errorf("bindspec: read %s: %w", st.Path, st.ReadErr)
	}
	if st.Value.Err() != nil {
		return BindingSet{}, &LoadError{Path: st.Path, Fields: []*FieldError{fieldErr(st.Path, "<parse>", st.Value.Err())}}
	}
	return decodeBindings(st.Path, st.Value)
}

// decodeBindings decodes the bindings key of a compiled modules file.
func decodeBindings(path string, v cue.Value) (BindingSet, error) {
	list := v.LookupPath(cue.ParsePath("bindings"))
	if !list.Exists() {
		// Absent, not malformed — see the doc comment above.
		return BindingSet{}, nil
	}
	iter, err := list.List()
	if err != nil {
		return BindingSet{}, &LoadError{Path: path, Fields: []*FieldError{fieldErr(path, "bindings", err)}}
	}

	var errs []*FieldError
	var out BindingSet
	seen := map[string]bool{}
	idx := 0
	for iter.Next() {
		bv := iter.Value()
		prefix := fmt.Sprintf("bindings[%d]", idx)
		idx++

		b := Binding{
			Module:   lookupString(bv, "module"),
			Scopes:   lookupStrings(bv, "scopes"),
			MaxRules: lookupIntDefault(bv, "max_rules", DefaultMaxRules),
			MinTier:  lookupString(bv, "min_tier"),
		}
		if !moduleNameRe.MatchString(b.Module) {
			errs = append(errs, fieldErr(path, prefix+".module", fmt.Errorf("must match %s, got %q", moduleNameRe.String(), b.Module)))
		} else if seen[b.Module] {
			errs = append(errs, fieldErr(path, prefix+".module", fmt.Errorf("duplicate binding for module %q — one binding per module", b.Module)))
		} else {
			seen[b.Module] = true
		}
		if b.MaxRules <= 0 {
			errs = append(errs, fieldErr(path, prefix+".max_rules", fmt.Errorf("must be > 0, got %d", b.MaxRules)))
		}
		if b.MinTier != "" && !validTiers[b.MinTier] {
			errs = append(errs, fieldErr(path, prefix+".min_tier", fmt.Errorf("must be one of verified/pre-scan/doc-claimed/sme-stated/assumed, got %q", b.MinTier)))
		}
		scopesField := bv.LookupPath(cue.ParsePath("scopes"))
		if scopesField.Exists() && len(b.Scopes) == 0 {
			errs = append(errs, fieldErr(path, prefix+".scopes", fmt.Errorf("must be non-empty when present")))
		}
		errs = append(errs, checkNoOptionalFields(path, prefix, bv)...)
		out.Bindings = append(out.Bindings, b)
	}
	if len(errs) > 0 {
		return BindingSet{}, &LoadError{Path: path, Fields: errs}
	}
	return out, nil
}

// ValidateStrict is the AUTHORING gate: it unifies the modules.cue instance
// at path against the embedded, CLOSED #Bindings definition and requires
// full concreteness — an unrecognized field name, a missing required
// field, or an abstract (unset) required field all fail here.
func ValidateStrict(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("bindspec: read %s: %w", path, err)
	}
	cctx := cuecontext.New()
	schemaDef, err := compileBindingSchema(cctx)
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

func compileBindingSchema(cctx *cue.Context) (cue.Value, error) {
	v := cctx.CompileBytes(bindingSchema, cue.Filename("schema/binding/schema.cue"))
	if err := v.Err(); err != nil {
		return cue.Value{}, fmt.Errorf("bindspec: compile binding schema: %w", err)
	}
	def := v.LookupPath(cue.ParsePath("#Bindings"))
	if err := def.Err(); err != nil {
		return cue.Value{}, fmt.Errorf("bindspec: schema has no #Bindings definition: %w", err)
	}
	return def, nil
}

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

func lookupIntDefault(v cue.Value, p string, def int) int {
	fv := v.LookupPath(cue.ParsePath(p))
	if !fv.Exists() {
		return def
	}
	i, err := fv.Int64()
	if err != nil {
		return def
	}
	return int(i)
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

// checkNoOptionalFields returns a FieldError for every field WRITTEN on bv
// with a trailing '?' (CUE's optional-field marker) — critic finding 1
// (phase-O merge, BLOCKING): '?' is meaningful in the SCHEMA (#Binding),
// never in a plain instance a human authors by hand; written there, it
// silently declares the field optional-and-therefore-ABSENT rather than
// setting it — the one misconfiguration LookupPath's "field simply wasn't
// written" path can never distinguish from genuine absence on its own.
// bv.Fields(cue.Optional(true)) is the one CUE API that CAN see the
// difference: it iterates every field actually present in the source,
// including optional-marked ones, and Iterator.IsOptional() reports which
// (a field never written at all does not appear in this iteration either
// way, so this never flags genuine absence).
func checkNoOptionalFields(path, prefix string, bv cue.Value) []*FieldError {
	iter, err := bv.Fields(cue.Optional(true))
	if err != nil {
		return nil
	}
	var errs []*FieldError
	for iter.Next() {
		if !iter.IsOptional() {
			continue
		}
		name := iter.Selector().String()
		errs = append(errs, fieldErr(path, prefix+"."+name, fmt.Errorf(
			"field %q is written with a trailing '?' — that syntax marks a field OPTIONAL IN A SCHEMA, but in this instance it silently declares %q ABSENT instead of setting it (see schema/binding/schema.cue's #Binding for where '?' actually belongs); remove the '?' to set a concrete value, or delete the line entirely to omit it", name, name,
		)))
	}
	return errs
}
