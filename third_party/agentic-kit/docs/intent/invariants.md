# Invariants

Statements that must always or never be true. **Enforced by** is honest: `lint <check>`
is a `cmd/constitution-lint` check run by `task validate`; `task <name>` is a gate task;
`guard` is a pre-edit rule in `fathom-pack/guard-pack.cue`; `test` names a test;
`stated` means the editing agent is the only guard. Audited 2026-08-25.

| # | Invariant | Enforced by | Source |
|---|---|---|---|
| I1 | The root module has no `require` block. Zero external Go dependencies, forever. | `lint empty-require` | VISION belief 7; uplift D1 |
| I2 | No org-specific coded names and no hardcoded model-ID literals anywhere in the tree, docs included. | `lint org-agnostic` | CLAUDE.md hard rules |
| I3 | No secret-shaped string is committed. | `lint secret-scan` (+ `TestSecretScanCanary`) | `threat-model.md` |
| I4 | Every capability has a `TRACKING.md` checkbox; `[x]` only when tests pass and the full gate is green. | `lint tracking-coverage` | CLAUDE.md §TRACKING.md discipline |
| I5 | Every package whose shape is data ships `schema.cue`; `cue vet` covers its fixtures. | `lint schema-presence`; `task cue-vet` | VISION belief 1 |
| I6 | Every package doc states its concurrency contract. | `lint concurrency-contract` | CLAUDE.md hard rules |
| I7 | Exported identifiers are documented name-first. | `lint doc-coverage` | CLAUDE.md |
| I8 | `docs/catalog.md`, `docs/compat.md` coverage, and the README package count match the live tree. | `lint catalog-fresh`, `compat-coverage`, `readme-count` | `compat.md` §Method |
| I9 | `foundations/governance/*` is WASM-safe: no file I/O, no goroutine leaks. | `task wasm-build` (CI); otherwise stated | CLAUDE.md hard rules |
| I10 | Replay cassettes carry the current version and shape. | `lint cassette-version` | VISION belief 2 |
| I11 | `go test -race ./...` is green; tests use `httptest`, fakes, and cassettes, never the network or a live key. | `task test`; discipline `stated` | CLAUDE.md; `practices.md` §6 |
| I12 | Library code never calls bare `time.Now()`; clocks are injected. | stated (no lint) | CLAUDE.md hard rules |
| I13 | Imports point downward: foundations → stdlib and sibling foundations only; kits → foundations; runtimes and surfaces above. | stated (no lint) | CLAUDE.md hard rules |
| I14 | Nothing imports a `_reference/` directory. | stated (Go ignores them; no test) | CLAUDE.md §Where things live |
| I15 | No `git commit --no-verify`; opt out only with `LEFTHOOK=0` when the hook itself is broken. | `guard` (command scope) | CLAUDE.md §Git; `lefthook.yml` |
| I16 | A `stable` or `internal-ish` package never breaks silently pre-1.0: migration note in `CHANGELOG.md` plus same-commit `TRACKING.md` update. At 1.0, strict semver. | stated (policy) | `compat.md` §Promise |
| I17 | `errors.Is`/`errors.As` on a `stable` package's sentinels keep working; a `stable` package's `schema.cue` change is breaking. | stated; back-stopped by `task cue-vet` | `compat.md` §Typed errors, §CUE |
| I18 | The kit never owns `main()`, the loop, or the process model. | stated | VISION §What the kit is NOT |
| I19 | Every measurement channel has an existence check; a metric whose source can vanish silently is theater. | `guard` (telemetry scope); `heartbeat.CheckExistence` | `guard-pack.cue` §telemetry; INC-1 |
| I20 | `coordination/*` and `topology/live-graph` are plan-only: they never spawn or execute workers. | stated | `compat.md`; uplift D7 |
| I21 | Consumer checks (`fathom`, `throughline`, `growing-gardens-bi`) are green against kit HEAD before a tag or an exported-API change lands. | `task downstream-check` (fail-open when a sibling is absent) | README §Downstream; uplift D3 |
| I22 | Commits carry no co-author trailer; only trailers the harness adds. | stated | CLAUDE.md §Git |

Revisit when: I12, I13, I14 gain a lint (they are the three `stated` rows most likely to
be violated silently; a 30-line `go/parser` test would raise each to `test`). I16 and I17
change form at 1.0 (uplift D4).
