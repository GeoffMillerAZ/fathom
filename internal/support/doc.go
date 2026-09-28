// Package support is the projection behind fathom's support matrix
// (docs/design/support-matrix.md, plan docs/plans/support-matrix.md, prefix
// SM): what fathom supports, computed from registries, declared targets,
// committed readings and plan verdicts — never a hand-kept catalog
// (ADR-048 decision 2, I54, extended to every kind).
//
// A level or status renders only when a committed reading (or, for a
// closed-vocabulary boolean capability, a registry entry) proves it;
// without one a cell is `tolerated` (registered, unmeasured) or `declared`
// (planned, unbuilt), never a claimed level. ComputeState is the pure
// function that enforces this; Project assembles the full matrix under an
// injected clock, with `regressed` computed from the readings ledger.
//
// This package never imports the extraction engines (internal/extractor's
// tree-sitter/wasm machinery): the language and tool adapters here read only
// the dialect manifests' JSON shape and the language queue file, so pulling
// in `support` for a CLI or console surface never drags in CGO/wasm
// dependencies it does not need.
package support
