### `tests ingest` / `tests status` — test-kit evidence (vision 33 / ADR-043 stage 0)

```sh
fathom tests ingest --manifest <dir> --results <file|dir> \
  [--service <id>] [--store <path>]

fathom tests status [--service <id>] [--store <path>] [--json]
```

`tests ingest` loads a service's declared `<dir>/tests/*.cue` manifests
(validated against the mirrored test-kit schema, `schema/testkit/`) and
attested `#TestResult` envelopes from `--results` (a single file, a
directory of `*.json` files, plain JSON, NDJSON, or a DSSE/in-toto-wrapped
envelope — the payload is decoded and its digest recorded as the run's
identity; fathom never verifies signatures itself) into the additive
`test_suites`/`test_runs`/`test_cases` store tables, joining each case to a
`Test*` symbol by exact name (`TestX/sub` joins to `TestX`). It also scans
`--manifest`'s whole tree for `// certifies: <kind> <id> [(<strength>)]`
markers above `Test*` functions, writing one declared fact per entry into
`certify_declarations` (a malformed marker is a warning naming its
file:line, never a silent skip). "no manifests found" and "no runs
ingested" print as named lines — never a silently clean run.

`tests status` prints, per declared suite, its class, tier, the last
ingested run's verdict and totals, and `joined N of M cases` — the
case-to-symbol join rate is a reported fact, never a pass/fail. A service
with no declared suites prints "no manifests found" instead of an empty
table.

fathom never runs a test itself (design doc §2): both verbs ingest
test-kit's own record. A run's `env.commit`/`env.dirty`/`env.race` (vision
34 / ADR-044 stage 1, O114) bind the run to the commit it was attested
against — the commit-blind half of `internal/certify`'s state ladder that
let a certificate read `live` after the code it certified had already
changed; an envelope with no commit (the common case for one produced
before this field existed, or by an adapter never told) leaves its
certificate `unproven`, never a silent guess. `task tests:attest` produces
a commit-bound envelope for this repository's own suite — see below.

**`task tests:attest`** re-attests this repository's own suite with a
commit-bound envelope: `go test -json ./...`, converted to a `#TestResult`
envelope by test-kit's go-test-json adapter (env `TESTKIT_GO_JSON` — the
path to a built test-kit `surveyor` binary; the task prints a clear skip,
never a failure, when it is unset), bound to `git rev-parse HEAD` / `git
status --porcelain`, and written under `[tests] results_dir`
(`docs/configuration.md`). The task never ingests the envelope itself — a
plain `fathom ingest`, or `fathom tests ingest --results
workspace/artifacts/test-results`, picks it up on its own next run.
`TESTKIT_RACE=1` adds `-race` to the run and `--race` to the envelope (off
by default — a full `-race ./...` run is slow, and the task's own job is
producing a commit-bound envelope, not deciding this repository's race
policy).

What reads it: an ingested run is what `fathom verify`'s `runtime_pass`
reports for a row's certifying tests (TC-29, [verify.md](verify.md)), and the
coverage files the same `[tests] results_dir` holds — the attested Go pair
(`coverage.meta.json` + `coverage.out`) and any report of a known name
(`lcov.info`, `coverage-final.json`, `coverage.json`, `coverage.xml`,
`cobertura.xml`, `cobertura-coverage.xml`, `jacoco.xml`,
`jacocoTestReport.xml`, `llvm-cov.json`) — are what the risk rank's
uncovered share reads in `test_impact rank:"risk"`, the console's `/tests`
and its hotspots (TQ-26). Coverage is read from those files at request time,
never stored, and each answer names the file it read.

| Flag (`tests ingest`) | Default | Description |
|---|---|---|
| `--manifest` | `.` | service repo root: `tests/*.cue` live under `<dir>/tests/`, and `// certifies:` markers are scanned under `<dir>` |
| `--results` | `""` | attested `#TestResult` envelope file, or a directory of `*.json` envelopes |
| `--service` | resolved from cwd | Service ID |
| `--store` | `<repo>/workspace/db/fathom.sqlite` | store path |

| Flag (`tests status`) | Default | Description |
|---|---|---|
| `--service` | resolved from cwd | Service ID |
| `--store` | `<repo>/workspace/db/fathom.sqlite` | store path |
| `--json` | `false` | Emit JSON instead of a table |
