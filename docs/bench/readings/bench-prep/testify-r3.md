# Testify readonly-module remeasurement

Full-suite remeasurement at `87a7b9d57689f6579db2da795ab8deeab29cb724`,
Linux arm64, four CPUs, the original pinned Go 1.27.3 image. Setup fetches
packages with `go mod download`; one warm-up and three fresh timed processes
run offline using `GOFLAGS=-mod=readonly GOPROXY=off GOTOOLCHAIN=local
GOMAXPROCS=4 go test -count=1 -timeout=14m ./...`. The harness mounts the input
clone read-only and stages tracked source and Git metadata under `/work/source`.
Module/build caches and temporary files stay under `/work`. Neither original
nor staged tracked files changed, and the go.sum hash remained unchanged.

```sh
BENCH_JOB="$job/workspace" python3 calibration/bench/tools/remeasure.py "$config"
python3 calibration/bench/tools/check_testify.py
```

Warm-up: **27.679s, exit 0**. Timed trials: **13.679, 17.072, 12.643s, exits 0/0/0**.
The network-denial probe exits 7. Setup and warm-up are excluded; trials are
serialized by the job-local lease. `testify-r3.json` binds the pin, lock, image,
commands, UTC starts, monotonic durations, raw-log hashes and exact edited
harness/helper hashes. Six raw logs (setup, probe, warm-up and three trials)
are committed in `raw/bench-subjects-r3-testify-logs.tar.gz`, SHA-256
`8db72686ada18197128d8aa7a5e051e7715a04279de90a79a8ce378760580c19`. Tar metadata contains no account names.

The current wrappers are **edited after round-2 trials**; the per-instrument
labels and hashes are in `instruments-r3.json`. No claim is made that edited
copies are the exact source executed in the older readings. The two initial
round-3 source-staging attempts failed before setup completed because local
Git clone transport rejected shared-mount ownership; they are preparation
failures, not C3 trials. Staging tracked files and Git metadata resolved that
harness issue. No subject patch, passing subset or network-enabled test was
used. Original round-1 mutable-module timings remain historical.

Testify nevertheless remains **refused on C6 under D26**, with zero product-only
qualifying symbols. This C3 result does not admit it. Native macOS, twelve
cores and C4 remain unmeasured.
