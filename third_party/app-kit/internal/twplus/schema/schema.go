// Package schema embeds the TailwindPlus CUE schema and validates normalized
// components against it (D7: CUE is the source of truth; this is its runtime
// enforcement). Each component is unified with #Component individually so a
// failure names the offending component instead of producing one opaque wall.
package schema

import (
	_ "embed"
	"fmt"

	"cuelang.org/go/cue"
	"cuelang.org/go/cue/cuecontext"
)

//go:embed tailwindplus.cue
var schemaCUE []byte

// Validator validates components against the embedded #Component definition.
// It is not safe for concurrent use (it carries a cue.Context); construct one
// per goroutine if validating in parallel.
type Validator struct {
	ctx  *cue.Context
	comp cue.Value
}

// NewValidator compiles the embedded schema and resolves #Component.
func NewValidator() (*Validator, error) {
	ctx := cuecontext.New()
	v := ctx.CompileBytes(schemaCUE, cue.Filename("tailwindplus.cue"))
	if err := v.Err(); err != nil {
		return nil, fmt.Errorf("twplus/schema: compile: %w", err)
	}
	comp := v.LookupPath(cue.ParsePath("#Component"))
	// Note: we do NOT call comp.Err()/Validate() on the bare definition. Its
	// `if section == "Ecommerce"` guards can't evaluate while section is still
	// the abstract #Section disjunction; they resolve only once the definition
	// is unified with a concrete component (see ValidateComponent).
	if !comp.Exists() {
		return nil, fmt.Errorf("twplus/schema: #Component not found in schema")
	}
	return &Validator{ctx: ctx, comp: comp}, nil
}

// ValidateComponent encodes a JSON-marshalable component (matching the
// #Component field names via its json tags) and unifies it with the schema.
// A non-nil error describes the first conflict concretely.
func (v *Validator) ValidateComponent(component any) error {
	cv := v.ctx.Encode(component)
	if err := cv.Err(); err != nil {
		return fmt.Errorf("encode: %w", err)
	}
	u := v.comp.Unify(cv)
	if err := u.Err(); err != nil {
		return err
	}
	return u.Validate(cue.Concrete(true))
}
