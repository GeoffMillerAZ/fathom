// Package testlayer classifies a Go test file into a named test layer
// ("unit", "integration", "e2e", or a project-specific name) — RC-39,
// docs/vision/28-receipts.md §5.
//
// The boundary this package keeps (docs/vision/28-receipts.md §5): fathom
// ASSERTS which layer a test belongs to and, paired with
// internal/analysis/coverage, whether a changed symbol has a covering test
// — never a judgment about whether that coverage is adequate, which is the
// review kit's job, not fathom's.
//
// Classify is the pure, defaults-only classifier most callers want: no file
// I/O, no CUE — buildTags is whatever the caller already extracted from the
// file's `//go:build` line (or none). LoadLayers is the dual-mode Go-side
// loader for an OPTIONAL schema/testlayer/schema.cue-shaped config file:
// absent — the common case — returns DefaultLayers() (the built-in
// integration/e2e convention, matching Classify's own defaults); present,
// it is parsed and used exactly as declared, no merge with the built-in
// set. Mirrors internal/guardspec's LoadGuards/GuardSet split (lenient
// reader; every field error collected, not just the first).
package testlayer
