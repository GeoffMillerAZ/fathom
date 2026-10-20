package projectdir

import (
	"fmt"

	"cuelang.org/go/cue"
	"cuelang.org/go/cue/cuecontext"
)

// ValidatePack checks one tech pack file's source against schema/techpack's
// #Pack (the embedded copy), so a pack that carries no header, a key a kit
// may not carry (layer_rules, capabilities, bindings, exclude_packs,
// orientation) or a malformed section is named rather than read leniently.
// filename only labels errors.
func ValidatePack(data []byte, filename string) error {
	ctx := cuecontext.New()
	schema, err := compileSchemas(ctx)
	if err != nil {
		return err
	}
	pack := schema["techpack"].LookupPath(cue.ParsePath("#Pack"))
	v := ctx.CompileBytes(data, cue.Filename(filename))
	if v.Err() != nil {
		return v.Err()
	}
	if err := pack.Unify(v).Validate(cue.Concrete(true)); err != nil {
		return fmt.Errorf("not a valid tech pack: %w", err)
	}
	return nil
}
