# Agent waves: the worker-brief contract

An orchestrator that dispatches parallel coding agents ("workers") against this repository puts
these rules in every worker brief, and follows the orchestrator rules itself. Each rule names the
row that built what it relies on. Written at stop 1 of the efficiency wave
(`docs/plans/efficiency-wave.md`, ADR-049's amendment of 2026-09-29); a rule is here only when its
feature landed, and the last section says what has not.

## Isolation: one environment per worker (EW-8)

Each worker gets its own clone (never the main checkout) and its own scratch directory, named by
the worker's label and index. A shared scratch path was once removed by a sibling worker. Inside
it, per worker:

| Variable | Set to | Why |
|---|---|---|
| `FATHOM_HOME` | `<scratch>/fathom-home` | every per-user fathom file (the verify and intent ledgers, the hooks log, session stamps, daemon records) resolves there instead of `~/.fathom` (PE-26), so a worker's `verify` or `intent audit` never writes the operator's ledgers (O121). `task test` sets its own under `workspace/tmp/` when none is set (PE-28). See `docs/usage.md` "Scratch and test runs" |
| `FATHOM_CACHE_ROOT` | `<scratch>/cache` (absolute, created first) | the private build-cache root: `task` targets derive `GOCACHE`, `GOMODCACHE`, `GOTMPDIR` and `TMPDIR` beneath it, and the scratch ingest store lives there too. Its size guard trims only entries unused for a day or more and never runs `go clean -cache` (EW-4, O124) |
| `GOTMPDIR` | `<scratch>/gotmp` | Go's link and test scratch, on the same volume as the cache and outside every module, so `go vet ./...` never walks leftover build packages |

Shared across workers: one Go build cache for plain `go` commands (`GOCACHE` on the volume the
builds use), never pruned while a wave runs. Go's cache is safe under concurrent builds; a prune is
what raced (O124). If a brief also gives each worker a private `HOME`, it sets `FATHOM_GATE_LOCK` to
one shared path, because the gate lease (`scripts/gate-lease.sh`) lives under `$HOME` and gates on
one machine must still queue on one lock. A private `HOME` also has no git identity, so git falls
back to `<user>@<hostname>`: the brief sets `user.name` and `user.email` in each clone (14 commits of
the 2026-09-29 wave carried a host name for want of it).

## The shared daemon is read-only to a worker

- A worker never ingests into the shared daemon (no `ingest` tool call, no shell `fathom ingest`
  against its store). Its read tools are fine, but they answer for `origin/main`, not the worker's
  branch.
- A worker proves its own rows on a fresh scratch ingest of its committed HEAD:
  `fathom ingest --store "$FATHOM_CACHE_ROOT/verify.sqlite" --service fathom --path .`, then
  `fathom verify --strict --store "$FATHOM_CACHE_ROOT/verify.sqlite" --service fathom --plan docs/plans/<plan>.md`.
- `fathom plans status` and `plans next` without `--plan` scan the plans under the service's
  registered root, which the daemon names: the owner's checkout, not the worker's clone. The
  command prints the directory it scanned. A worker passes its own files with `--plan`.
- An `ingest` with `remap: true` refuses to re-point a service whose root is a live checkout of the
  same repository unless the caller names that root in `remap_from` (EW-19, O127), so a slip cannot
  capture the owner's service. `fathom doctor`'s `service_roots` row names a service whose root is
  gone (EW-20).

## Read a hot file by the part you need (EW-10)

Before opening `ADR.md`, `CHANGELOG.md`, `Taskfile.yml` or a design doc whole, a worker asks docs
search: the `search` tool (on the default surface) with `kind: docs`, for example
`{"service": "fathom", "kind": "docs", "pattern": "build cache trim"}`. Each hit names
`path:lineStart-lineEnd` and its heading path; read only that range with `read_source`. When the
row carries `ctx:` pointers, `fathom plans context docs/plans/<plan>.md --row <ID> --service fathom`
renders them; most rows carry none.

- Query with keywords, not a sentence: every word must match, so "what did we decide about
  embeddings" finds almost nothing (O137, open).
- The doc index fills on an ingest by a binary with KR-8. The shared daemon has it after `task
  deploy` and its next ingest; a worker's scratch ingest has it at once. Before that ingest the
  query answers `no doc hits`, the same answer a query nothing matches gets (only a store with no
  doc index at all is refused), so an empty answer from the shared daemon proves nothing until it
  has re-ingested.

## Select tests before a whole-package run (EW-11)

Before any whole-package run (`go test ./...`, `./cmd/fathom`), a worker runs
`fathom tests select --since <base> --store "$FATHOM_CACHE_ROOT/verify.sqlite" --service fathom`
and runs the `go test` commands it prints. It leads with the certifying-first set, the fewest tests
that re-prove every live certificate the diff touches (TQ-15w, TQ-16w, SR-50). With no live
certificates that set is empty, so add `--safe`, which also selects every test that statically
reaches a changed symbol. The whole-package run is the gate's, once, at the end.

## The gate and the push

- The integrator merges the workers' branches, builds `bin/fathom` from the merged tree, ingests
  the committed HEAD into a fresh scratch store and runs `fathom verify --strict --since
  origin/main` over every plan with that binary. Its wired check refuses a row that no product
  entry point reaches; a worker never adds `library: true` or `skip_test` to pass it.
- One push per wave, by the integrator. Workers never push. The pre-push step reads its `--since`
  base from the remote sha lefthook passes (EW-16), so a stale local `origin/main` neither blocks the
  push nor widens the range.
- EW-12, which makes this gate the wave's contract, stays open until EW-0 lands the integration line.

## Ids and messages

- Each brief reserves the ids a worker may mint: plan row ids and observation ids (for example
  EW-23 to EW-29 and O131 to O137, one of each per worker). A worker uses only its own and reports
  the unused ones; two workers once minted the same ids on 2026-09-29.
- The orchestrator never messages a running workflow agent. A correction goes into a later brief or
  a follow-up worker.

## Not yet in the contract

- A wave map in every brief (EW-13): the maps slice is stage 3 and not built.
- Hook CPU time (EW-29): the hook-latency log holds wall time only, so a slow hook cannot yet be read
  as work or load from the log alone.
- The test-run hook is on in `.fathom/config.toml` (EW-9), but a session sees its line only once
  `.claude/settings.json` carries the two test-run hook rows `fathom init` writes and the deployed
  binary knows `--event test-run`.
