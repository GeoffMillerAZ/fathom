### Added

- `scenariobench vet` vets each `calibration/bench.d` subject against C1 to C7 from operator-prepared clones, offline, and writes `workspace/bench/vet/<name>.json` with each criterion's status and evidence (criteria: docs/design/bench-prep.md). Any failure refuses a subject, and any pending criterion keeps it pending.
