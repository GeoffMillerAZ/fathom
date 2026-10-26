### Added

- CUDA (`.cu`, `.cuh`) extraction, measured against universal-ctags, scip-clang, clang and nvcc's preprocessor on NVIDIA/cuda-samples v12.9: its metrics grade L1 excellent and L2 to L4 good (capped by tier-B oracles), and a `.cu` file's edges into the headers its includes reach are drawn and measured. The committed reading claims no level yet: its throughput was measured in a cloud VM under the 1 MB/s floor, and the re-time on the development-machine class follows the next CUDA round.
- `indexbench lang-oracle -scip-tier` (an index's tier is an input), `-subject-sha` and `-note`; the clang oracles' `-tier` is required.
