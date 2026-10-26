### Changed

- `indexbench lang-parity` takes `-scip-tier` and, like `lang-oracle`, refuses a scip-clang index that names no tier; a reading lists the ranges companion (`clang-ast-defs`) and the link-unit companion as oracles of their own.
- CUDA's call truth leaves out compiler stand-ins outside the files the reading measures, and takes a caller the index holds no occurrence in from the compiler's function ranges, never from fathom's definitions.
- `calibration/oracles/scip-clang/cuda-samples.sh` configures both CMake build trees outside the subject and records the measured set's file count, bytes and the sha256 of its sorted path list in the reading.
