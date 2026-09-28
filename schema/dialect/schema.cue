// Package dialect defines the CUE schema for a dialect manifest,
// internal/extractor/dialects/<id>/dialect.json (docs/design/language-breadth.md
// §2.2, ADR-048, plan row LB-F29). The definitions live in dialect.cue; this
// file exists so `task cue-vet` (which walks every schema.cue) vets the
// package and its testdata. The Go side (internal/langbench) asserts the
// commented negatives in testdata/invalid.cue.
package dialect
