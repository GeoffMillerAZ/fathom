// Package labs embeds the file-only feature-lab contracts for Go consumers.
package labs

import _ "embed"

// Source is the same CUE schema vetted by the schema tests and cue-vet.
//
//go:embed schema.cue
var Source string
