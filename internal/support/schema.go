package support

// schema.go — the CUE Go API seam onto schema/support/schema.cue (ADR-018:
// CUE validates, it never authors or executes). supportSchema is
// byte-identical to schema/support/schema.cue, kept in sync by
// TestSupportSchemaMatchesSource — the same drift-gate convention
// internal/modspec/manifest.go established for #Module.

import (
	_ "embed"
	"fmt"

	"cuelang.org/go/cue"
	"cuelang.org/go/cue/cuecontext"
)

//go:embed support_schema.cue
var supportSchema []byte

// compileSupportDef compiles the embedded support schema and returns the
// named definition (e.g. "#Matrix", "#Targets", "#LedgerRow"), ready to
// Unify against an instance.
func compileSupportDef(cctx *cue.Context, def string) (cue.Value, error) {
	v := cctx.CompileBytes(supportSchema, cue.Filename("schema/support/schema.cue"))
	if err := v.Err(); err != nil {
		return cue.Value{}, fmt.Errorf("support: compile embedded schema: %w", err)
	}
	d := v.LookupPath(cue.ParsePath(def))
	if err := d.Err(); err != nil || !d.Exists() {
		return cue.Value{}, fmt.Errorf("support: embedded schema has no %s definition", def)
	}
	return d, nil
}

// ValidateMatrixJSON unifies data (the JSON bytes of a `fathom support
// --json` document) against the embedded #Matrix definition. A cell without
// a reading claiming a level, a state outside the closed vocabulary, or any
// other shape #Matrix refuses all fail here (schema/support/testdata/
// invalid.cue's #Cell cases, embedded in a matrix).
func ValidateMatrixJSON(data []byte) error {
	cctx := cuecontext.New()
	def, err := compileSupportDef(cctx, "#Matrix")
	if err != nil {
		return err
	}
	val := cctx.CompileBytes(data, cue.Filename("matrix.json"))
	if err := val.Err(); err != nil {
		return fmt.Errorf("support: compile matrix JSON: %w", err)
	}
	unified := def.Unify(val)
	if err := unified.Validate(cue.Concrete(true)); err != nil {
		return fmt.Errorf("support: matrix does not validate against #Matrix: %w", err)
	}
	return nil
}
