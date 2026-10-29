### Changed

- CUDA calibration reaches L4 (L1 excellent, L2 to L4 good under tier-B oracles) on NVIDIA/cuda-samples v12.9, confirmed by a development-class re-time at 2.19 MB/CPU-s with the subject-relative compile database at `docs/bench/readings/lang/compdb/cuda.compile_commands.json`. The CUDA driver names an explicit `--target` in every database entry and publishes that database.
