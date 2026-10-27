# Lock-bound round-2 measurements

`inventory.py SUBJECT LANGUAGE` reads tracked blobs at HEAD with `git archive`,
counts primary-language LF lines (including tests/generated/examples), hashes
licences and locks, and counts history co-changes in the fixed UTC window using
`\b(fix(es|ed)?|bugs?|regressions?)\b`. It emits JSON to stdout. Full-history status is
reported; counts are estimates, never fail-before/pass-after replay evidence.

`BENCH_JOB="$job" python3 calibration/bench/tools/remeasure.py CONFIG.json`
accepts a JSON array with `id`, `source` (a detached clone), `language`, `lock`
(relative to the clone), `setup` and `test`. An optional `image` is an exact digest override for a subject runtime (trpc and redux-toolkit use Node 24.21.0). `stop_on_failed_warmup` records a complete failed warm-up as a C3 counterexample and leaves timed trials unrun. Setup is online for package fetches;
one warm-up and exactly three timed trials use `docker --network none --cpus 4`
and images pinned to the host’s native architecture. Every container runs as uid/gid 1000 via `--user 1000:1000` and `--platform`; each phase logs container `uname -m` and `id -u` as `BENCH_UNAME_M=` and `BENCH_UID=`. The runner checks the native architecture and UID 1000 and copies both witnesses into every phase receipt. `BENCH_PHASE` separates machine-readable reports from the warm-up and three timed processes. The `/work/tmp` tmpfs explicitly sets mode 1777 and uid/gid 1000. Corepack shims live in writable `/work/bin`. Synthetic passwd/group files in the job are mounted read-only at `/etc/passwd` and `/etc/group`, so uid 1000 can be resolved without exposing host accounts. Ephemeral Linux tmpfs at `/work/tmp` supports Unix sockets that the shared mount rejects. Source is mounted read-only at `/subject`, tracked source is staged at `/work/source`, and job-owned work at
`/work`. Logs and JSON receipts go to the job. A job-local flock serializes every
offline subprocess; lock waiting is excluded from elapsed time. Each trial records its raw log hash, exit code and command. The final harness also records UTC starts; early valid retry receipts have elapsed times only. The optional `--trials-only`
reuses a successful setup receipt at the same SHA and lock hash. It does not
repair a failed setup. A timeout or interruption kills its named container as well as the CLI; completed stages are checkpointed immediately.

TypeScript containers enable Corepack shims so nested package scripts use the
prepared package manager. pnpm 11/12 read `PNPM_CONFIG_OFFLINE=true`, not npm's
old variable. `pnpm_config_verify_deps_before_run=warn` prevents pnpm's default
implicit reinstall when container-local workspace state changes. It executes
scripts against the frozen installation; the lock hash is checked again after
trials. Network denial remains the actual egress boundary. These settings are
explicit measurement-environment changes, not subject patches.

`BENCH_TYPESCRIPT="$oracle_tools/node_modules/typescript" node
calibration/bench/tools/c6_ts_project.cjs SUBJECT [TSCONFIG]` resolves call
signatures with the project's TypeScript options, including path aliases, and
disambiguates declarations by their line. TSCONFIG is relative to SUBJECT and
defaults to `tsconfig.json`. The oracle is TypeScript 5.9.3, installed separately.
Interface method/call signatures are excluded; diagnostics and dynamic dispatch
are not C4 claims. `recount.py LANGUAGE SUBJECT RESOLVED_ROWS.json` applies D26 on D24 units
to the full resolved site lists: directory units for Python/TypeScript, import
packages for Go, defining-unit calls excluded; test, example, benchmark, sample, docs and tool callers excluded. Use `--policy D24` only for historical recounts.

Python uses uv 0.13.5 `sync --frozen --no-default-groups --group GROUP` (dev for
Starlette, tests for marshmallow/Click/attrs), then invokes the installed Python
directly offline. Rich uses Poetry 2.2.1 and export plugin 1.9.0 to export its
unchanged lock using the matching historical manifest, because its current
manifest fails Poetry's freshness check. Install exported requirements with
`pip install --require-hashes`, then install the subject editable with `--no-deps`.
The committed requirements hash, export base SHA and commands are in the Python
reading. Build-isolation tooling is outside the runtime dependency lock.

C4, formal graph certification, model-cutoff assignment and fix replay remain
controller work. No paid model API is used by any instrument here.

`verify_r2.py --archive "$archive"` checks the committed structured archive
checksum, all 73 member hashes, the 42 timed trials and their 39 green exits
against the committed readings. It rejects a different or altered archive.

The current scripts are edited after the historical round-2 runs. Read their
status and hashes in `instruments-r3.json`; old receipt commands remain unchanged
as historical evidence. `BENCH_JOB` must be under `workspace/`. The section-5
mount contract supplies `/bench-inputs:ro`; uv 0.13.5 is installed using the
committed hash-pinned `uv-requirements.txt`. Rich copies its committed export
from `/bench-inputs` before setup reads `/work/requirements.txt`.

The D28 amended application reading is checked with
`python3 calibration/bench/tools/verify_apps_r2.py`. It reads the archive format
from the JSON path (including `.tar.xz`), hashes every member, enforces a
6,000,000-byte cap, checks UID witnesses on the newly measured receipts, counts
skips from all four machine-readable processes, and binds Markdown exclusion
quotes to JSON. Historical receipt bytes remain historical; the new receipts
have separate archive names. `apps-r2-tools.json` pins native tool downloads by
version and SHA-256; `apps-r2-python-tools.txt` freezes Python tool dependencies
with hashes. `apps-r2-setup.py` installs them under `/work` without privilege.
The final GoReleaser setup schedules `go mod download all` in the subject and its external test modules, and prefetches pinned Node targets. The bulk module download hit the workspace cap before completion; no final offline suite is claimed. Upstream Node downloads have no disk cache, so verified prefetched bytes cannot satisfy them without a subject patch. GoReleaser is refused on C3.
A source guard requiring Docker cannot be repaired by starting an unrelated
standalone database or supplying a DSN which the guard never reads.
