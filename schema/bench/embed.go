// Package bench embeds the benchmark-input contracts for Go consumers.
package bench

import _ "embed"

// Source is the same CUE schema vetted by the schema tests and cue-vet.
//
//go:embed schema.cue
var Source string
