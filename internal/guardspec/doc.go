// Package guardspec is the Go-side dual-mode loader for a repo's guarded-
// edits configuration file, `.fathom/guard.cue` (schema: schema/guard/
// schema.cue; docs/design/guarded-edits.md §2.1). Mirrors internal/modspec
// and internal/bindspec's LoadX/ValidateStrict split exactly: LoadGuards is
// the lenient READER; ValidateStrict is the closed-schema AUTHORING gate.
package guardspec
