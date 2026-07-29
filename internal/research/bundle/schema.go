package bundle

import (
	_ "embed"
	"encoding/json"
	"fmt"

	"cuelang.org/go/cue"
	"cuelang.org/go/cue/cuecontext"
)

// SchemaVersionCurrent is the bundle schema version stamped into every
// Bundle (Bundle.SchemaVersion) — a recipient can tell which shape they are
// looking at without guessing or trusting the sender's binary version.
const SchemaVersionCurrent = "v1"

// researchBundleSchema is byte-identical to
// schema/research-bundle/schema.cue — kept in sync by
// TestResearchBundleSchemaMatchesSource (schema_test.go), the same
// drift-gate pattern internal/config/schema_test.go uses for
// schema/config/schema.cue.
//
//go:embed research_bundle_schema.cue
var researchBundleSchema []byte

// Validate unifies b (marshaled to JSON, then compiled as a CUE value)
// against the embedded #Bundle definition. Because #Bundle is a closed
// struct, this also re-checks the Go-side allowlist independently: a field
// Build somehow wrote that isn't named in the schema fails here even if the
// Go type system didn't catch it.
func Validate(b Bundle) error {
	data, err := json.Marshal(b)
	if err != nil {
		return fmt.Errorf("marshal bundle: %w", err)
	}
	cctx := cuecontext.New()
	schema := cctx.CompileBytes(researchBundleSchema)
	if err := schema.Err(); err != nil {
		return fmt.Errorf("compile embedded schema: %w", err)
	}
	def := schema.LookupPath(cue.ParsePath("#Bundle"))
	if !def.Exists() {
		return fmt.Errorf("embedded schema has no #Bundle definition")
	}
	val := cctx.CompileBytes(data)
	if err := val.Err(); err != nil {
		return fmt.Errorf("compile bundle JSON: %w", err)
	}
	unified := def.Unify(val)
	if err := unified.Validate(cue.Concrete(true)); err != nil {
		return fmt.Errorf("bundle does not validate against #Bundle: %w", err)
	}
	return nil
}
