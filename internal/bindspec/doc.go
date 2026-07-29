// Package bindspec is the Go-side dual-mode loader for a consumer repo's
// platform-module binding file, `.fathom/modules.cue` (schema:
// schema/binding/schema.cue; docs/design/platform-modules.md §3.1).
// Mirrors internal/modspec's LoadManifest/ValidateStrict split exactly:
// LoadBindings is the lenient READER (ignores unrecognized CUE fields, so a
// binding file authored under a future #Bindings revision still loads);
// ValidateStrict is the closed-schema AUTHORING gate `fathom init`'s
// scaffold self-checks and a human hand-editing modules.cue can run.
package bindspec
