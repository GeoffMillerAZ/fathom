### Changed

- `indexbench lang-oracle` requires `-scip-tier` for a scip-clang index, refuses any oracle file or companion (imports, ranges, link units) that names no tier, and refuses a subject with no commit of its own unless `-subject-sha` names one; `calibration/oracles/scip-clang/run.sh` takes the index's tier as an argument.
- CUDA's call truth takes its stand-in definitions (functions scip-clang writes none for, such as `extern "C"` wrappers) from the compiler's AST ranges, never from fathom's own definitions; `clang-ast-defs` reports the definitions an `extern "C"` block holds, marked `extern_c`.
