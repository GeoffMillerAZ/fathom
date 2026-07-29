package guardspec

import (
	_ "embed"
	"fmt"
	"os"
	"strings"

	"cuelang.org/go/cue"
	"cuelang.org/go/cue/cuecontext"
)

// guardSchema is byte-identical to schema/guard/schema.cue — kept in sync
// by TestGuardSchemaMatchesSource, the same drift-gate convention
// internal/modspec/manifest.go established for #Module.
//
//go:embed guard_schema.cue
var guardSchema []byte

// DefaultMaxRules is #Guard.max_rules' schema default.
const DefaultMaxRules = 4

// Guard is one decoded #Guard entry.
type Guard struct {
	Scopes         []string
	Note           string
	MaxRules       int
	IncludeModules bool
	CommandScopes  []string
}

// GuardSet is the decoded runtime form of a `.fathom/guard.cue` file
// (#Guards, docs/design/guarded-edits.md §2.1).
type GuardSet struct {
	Guards []Guard
}

// FieldError names one guard field that failed to load or validate.
type FieldError struct {
	Path  string
	Field string
	Err   error
}

func (e *FieldError) Error() string {
	return fmt.Sprintf("%s: field %q: %v", e.Path, e.Field, e.Err)
}

func (e *FieldError) Unwrap() error { return e.Err }

// LoadError collects every FieldError a single LoadGuards/ValidateStrict
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
	return fmt.Sprintf("guardspec: %s: %s", e.Path, strings.Join(msgs, "; "))
}

func fieldErr(path, field string, err error) *FieldError {
	return &FieldError{Path: path, Field: field, Err: err}
}

// LoadGuards reads and decodes the guard file at path — the conforming
// READER (see package doc): unknown CUE fields are never inspected.
// Required-ness and format are enforced in Go against exactly the fields
// this package knows about; every violation across every guard entry is
// collected into one *LoadError. A guard naming NEITHER scopes NOR
// command_scopes is rejected — a guard governing nothing is dead by
// construction, and the CUE schema alone cannot express that
// either/or cleanly (same class of decision as bindspec's Go-enforced
// duplicate-module check).
func LoadGuards(path string) (GuardSet, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return GuardSet{}, fmt.Errorf("guardspec: read %s: %w", path, err)
	}
	cctx := cuecontext.New()
	v := cctx.CompileBytes(data, cue.Filename(path))
	if v.Err() != nil {
		return GuardSet{}, &LoadError{Path: path, Fields: []*FieldError{fieldErr(path, "<parse>", v.Err())}}
	}

	list := v.LookupPath(cue.ParsePath("guards"))
	if !list.Exists() {
		return GuardSet{}, &LoadError{Path: path, Fields: []*FieldError{fieldErr(path, "guards", fmt.Errorf("required, MUST be present"))}}
	}
	iter, err := list.List()
	if err != nil {
		return GuardSet{}, &LoadError{Path: path, Fields: []*FieldError{fieldErr(path, "guards", err)}}
	}

	var errs []*FieldError
	var out GuardSet
	idx := 0
	for iter.Next() {
		gv := iter.Value()
		prefix := fmt.Sprintf("guards[%d]", idx)
		idx++

		g := Guard{
			Scopes:         lookupStrings(gv, "scopes"),
			Note:           lookupString(gv, "note"),
			MaxRules:       lookupIntDefault(gv, "max_rules", DefaultMaxRules),
			IncludeModules: lookupBoolDefault(gv, "include_modules", false),
			CommandScopes:  lookupStrings(gv, "command_scopes"),
		}
		if g.MaxRules <= 0 {
			errs = append(errs, fieldErr(path, prefix+".max_rules", fmt.Errorf("must be > 0, got %d", g.MaxRules)))
		}
		scopesField := gv.LookupPath(cue.ParsePath("scopes"))
		if scopesField.Exists() && len(g.Scopes) == 0 {
			errs = append(errs, fieldErr(path, prefix+".scopes", fmt.Errorf("must be non-empty when present")))
		}
		cmdField := gv.LookupPath(cue.ParsePath("command_scopes"))
		if cmdField.Exists() && len(g.CommandScopes) == 0 {
			errs = append(errs, fieldErr(path, prefix+".command_scopes", fmt.Errorf("must be non-empty when present")))
		}
		if len(g.Scopes) == 0 && len(g.CommandScopes) == 0 {
			errs = append(errs, fieldErr(path, prefix, fmt.Errorf("must set at least one of scopes/command_scopes — a guard governing nothing is dead by construction")))
		}
		out.Guards = append(out.Guards, g)
	}
	if idx == 0 {
		errs = append(errs, fieldErr(path, "guards", fmt.Errorf("must be non-empty")))
	}
	if len(errs) > 0 {
		return GuardSet{}, &LoadError{Path: path, Fields: errs}
	}
	return out, nil
}

// ValidateStrict is the AUTHORING gate: it unifies the guard.cue instance
// at path against the embedded, CLOSED #Guards definition and requires
// full concreteness.
func ValidateStrict(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("guardspec: read %s: %w", path, err)
	}
	cctx := cuecontext.New()
	schemaDef, err := compileGuardSchema(cctx)
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

func compileGuardSchema(cctx *cue.Context) (cue.Value, error) {
	v := cctx.CompileBytes(guardSchema, cue.Filename("schema/guard/schema.cue"))
	if err := v.Err(); err != nil {
		return cue.Value{}, fmt.Errorf("guardspec: compile guard schema: %w", err)
	}
	def := v.LookupPath(cue.ParsePath("#Guards"))
	if err := def.Err(); err != nil {
		return cue.Value{}, fmt.Errorf("guardspec: schema has no #Guards definition: %w", err)
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
