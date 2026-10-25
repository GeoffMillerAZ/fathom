### Added

- CUDA (`.cu`, `.cuh`) extraction at L4 (good at L2 to L4, capped by a tier-B oracle), measured against universal-ctags, scip-clang, clang and nvcc's preprocessor on NVIDIA/cuda-samples v12.9; a `.cu` file's edges into the headers its includes reach are drawn and measured.
- `indexbench lang-oracle -scip-tier` (an index's tier is an input), `-subject-sha` and `-note`; the clang oracles' `-tier` is required.
