# Calibration-only OpenCL candidate: admission failed

These patches are experimental measurement inputs, **not an admitted product
OpenCL grammar**. Neither has a `grammars.lock.json` row, dialect or runtime blob.
The native candidate at the first review failed the unchanged pooled grammar
gate at 63/386 ERROR-or-MISSING files (16.32%); follow-up grammar experiments
and their counts are recorded in the assessment below.

The calibration builder deliberately reuses the P-R1 verified patch applier
and its existing patch-root layout. The product build applies only patches
explicitly listed in the lock; no directory scan loads these files. Their presence
must not be used as evidence of OpenCL support or as permission to register it.

Do not add them to the product lock unless a later whole-pool native and product
WASM qualification passes the unchanged admission and runtime gates, with
controller review and the independent reading required by ADR-048.

See `docs/assessments/2026-10-26-opencl-c-grammar-gate.md` and
`calibration/oracles/opencl-c/grammar-candidates.json` for the current candidate
identity and measurements. No failed candidate is shipped by this directory.
