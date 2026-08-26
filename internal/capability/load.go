package capability

import (
	_ "embed"
	"errors"
	"fmt"
	"os"
	"sort"
	"strings"

	"cuelang.org/go/cue"
	"cuelang.org/go/cue/cuecontext"
)

//go:embed capability_schema.cue
var capabilitySchema []byte

// UserFileName and ProjectFileName are the two configuration locations.
const (
	UserFileName    = "capabilities.cue" // under ~/.fathom/
	ProjectFileName = "capabilities.cue" // under <repo>/.fathom/
)

// LoadError collects every problem one file had. Reporting all of them
// beats reporting the first: a file with three mistakes should send its
// author round the loop once, not three times.
type LoadError struct {
	Path  string
	Scope Scope
	Errs  []error
}

func (e *LoadError) Error() string {
	msgs := make([]string, 0, len(e.Errs))
	for _, err := range e.Errs {
		msgs = append(msgs, err.Error())
	}
	return fmt.Sprintf("capabilities: %s: %s", e.Path, strings.Join(msgs, "; "))
}

func (e *LoadError) Unwrap() []error { return e.Errs }

// OptionalFieldError names a known field declared optional in an instance.
//
// This is the phase-O trust-boundary inversion, caught at the boundary
// where it does damage. In CUE, `on?: false` inside a *configuration
// instance* does not set the field — it declares that the field may
// exist. The file looks like it switches something off and does nothing.
// A capability silently staying on when its owner believes they disabled
// it is exactly the failure this loader must refuse rather than tolerate.
type OptionalFieldError struct {
	ID    string
	Field string
}

func (e *OptionalFieldError) Error() string {
	return fmt.Sprintf("capability %q declares %q as OPTIONAL (%s?:) — in a configuration file that declares "+
		"the field may exist rather than setting it, so this line has no effect; write %q: without the question mark",
		e.ID, e.Field, e.Field, e.Field)
}

// LoadSettings reads one scope's configuration file.
//
// Three states, not two — the O22 lesson, applied again here:
//
//   - the file is ABSENT: no settings, no error. A repository that has
//     never configured capabilities is normal, not broken.
//   - the `capabilities` field is absent (an all-commented scaffold): no
//     settings, no error.
//   - the file is PRESENT and malformed: a loud *LoadError naming every
//     problem. Silently ignoring configuration is the defect class this
//     package exists to remove, so nothing here is ever dropped quietly.
func (r Registry) LoadSettings(path string, scope Scope) (Settings, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("capabilities: read %s: %w", path, err)
	}

	cctx := cuecontext.New()
	v := cctx.CompileBytes(data, cue.Filename(path))
	if v.Err() != nil {
		return nil, &LoadError{Path: path, Scope: scope, Errs: []error{v.Err()}}
	}

	root := v.LookupPath(cue.ParsePath("capabilities"))
	if !root.Exists() {
		return nil, nil // an all-commented scaffold, or a blank file
	}

	fields, ferr := root.Fields(cue.Optional(true))
	if ferr != nil {
		return nil, &LoadError{Path: path, Scope: scope, Errs: []error{fmt.Errorf("capabilities: %w", ferr)}}
	}

	out := Settings{}
	var errs []error
	for fields.Next() {
		id := fields.Selector().Unquoted()
		entry := fields.Value()

		// The `on` field must be a concrete, non-optional bool. Iterating
		// WITH optionals is what lets us see and refuse `on?:` rather than
		// silently reading past it.
		sub, serr := entry.Fields(cue.Optional(true))
		if serr != nil {
			errs = append(errs, fmt.Errorf("capability %q: %w", id, serr))
			continue
		}
		var (
			sawOn bool
			onVal bool
		)
		for sub.Next() {
			name := sub.Selector().Unquoted()
			if sub.IsOptional() {
				errs = append(errs, &OptionalFieldError{ID: id, Field: name})
				continue
			}
			if name != "on" {
				errs = append(errs, fmt.Errorf("capability %q: unknown field %q (only \"on\" is settable)", id, name))
				continue
			}
			b, berr := sub.Value().Bool()
			if berr != nil {
				errs = append(errs, fmt.Errorf("capability %q: field \"on\" must be true or false: %w", id, berr))
				continue
			}
			sawOn, onVal = true, b
		}
		if !sawOn {
			// An entry that sets nothing is a line its author believed did
			// something. Say so rather than dropping it.
			if !hasOptionalErrFor(errs, id) {
				errs = append(errs, fmt.Errorf("capability %q sets nothing — write `%s: on: false` to switch it off", id, id))
			}
			continue
		}
		out[id] = onVal
	}

	// Registry-level checks: unknown ids, and scopes overreaching (law 13).
	errs = append(errs, r.ValidateSettings(out, scope)...)
	if len(errs) > 0 {
		sort.SliceStable(errs, func(i, j int) bool { return errs[i].Error() < errs[j].Error() })
		return nil, &LoadError{Path: path, Scope: scope, Errs: errs}
	}
	return out, nil
}

func hasOptionalErrFor(errs []error, id string) bool {
	for _, e := range errs {
		var oe *OptionalFieldError
		if errors.As(e, &oe) && oe.ID == id {
			return true
		}
	}
	return false
}

// ValidateSchema checks a file against the embedded #Capabilities
// definition. LoadSettings does the load-bearing refusals in Go (they
// carry better messages and see optionality); this is the belt to that
// braces, used by `fathom doctor` and by the config-validity check.
func ValidateSchema(path string) error {
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("capabilities: read %s: %w", path, err)
	}
	cctx := cuecontext.New()
	schema := cctx.CompileBytes(capabilitySchema, cue.Filename("schema/capability/schema.cue"))
	if schema.Err() != nil {
		return fmt.Errorf("capabilities: embedded schema is broken: %w", schema.Err())
	}
	def := schema.LookupPath(cue.ParsePath("#Capabilities"))
	if !def.Exists() {
		return fmt.Errorf("capabilities: embedded schema has no #Capabilities")
	}
	instance := cctx.CompileBytes(data, cue.Filename(path))
	if instance.Err() != nil {
		return fmt.Errorf("capabilities: %s: %w", path, instance.Err())
	}
	if err := def.Unify(instance).Validate(cue.Concrete(false)); err != nil {
		return fmt.Errorf("capabilities: %s: %w", path, err)
	}
	return nil
}

// ExampleTemplate is what `fathom init` writes: entirely commented, so the
// file exists, teaches, and parses as an empty configuration. The loader's
// absent-field path (§LoadSettings) is what makes an all-commented
// scaffold legal — the O22 regression, pinned by a test that loads THESE
// bytes rather than a hand-written approximation of them.
func ExampleTemplate(r Registry) []byte {
	var b strings.Builder
	b.WriteString("// .fathom/capabilities.cue — which fathom features are on in this repository.\n")
	b.WriteString("//\n")
	b.WriteString("// Uncomment a line to override fathom's default. This file GOVERNS: a\n")
	b.WriteString("// setting here beats anything in ~/.fathom/capabilities.cue (law 13).\n")
	b.WriteString("//\n")
	b.WriteString("// Run `fathom capabilities` to see what is on, and `fathom capabilities\n")
	b.WriteString("// explain <id>` for what one of them costs.\n")
	b.WriteString("//\n")
	b.WriteString("// capabilities: {\n")
	for _, c := range r.All() {
		def := "false"
		if !c.Default {
			def = "true"
		}
		fmt.Fprintf(&b, "//     %-14s on: %-5s // %s\n", c.ID+":", def, firstSentence(c.Summary))
	}
	b.WriteString("// }\n")
	b.WriteString("//\n")
	b.WriteString("// Default answer budgets per tool class, in tokens (docs/design/budgeted-answers.md §3).\n")
	b.WriteString("// Uncomment to size them for this repository; absent means fathom's measured default.\n")
	b.WriteString("// Each class sizes only the tools named beside it; find_symbol, find_callers,\n")
	b.WriteString("// symbol_context and outline are not budgeted by class.\n")
	b.WriteString("//\n")
	b.WriteString("// budgets: {\n")
	b.WriteString("//     list:      1500 // query_symbols, references\n")
	b.WriteString("//     composite: 4000 // pack_context\n")
	b.WriteString("//     map:       2500 // repo_map\n")
	b.WriteString("// }\n")
	return []byte(b.String())
}

func firstSentence(s string) string {
	if i := strings.IndexByte(s, '.'); i > 0 {
		return s[:i]
	}
	return s
}
