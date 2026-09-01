# ADR-0009: SQLite via ncruces/go-sqlite3 is the default durable store

- **Status:** Accepted
- **Date:** 2026-06-10
- **Decision drivers:** cgo-free cross-compilation (the two-command
  clean-machine build, `../../VISION.md` "Where does determinism live?");
  one driver for both kit apps and the Assistant's index; FTS5 + sqlite-vec
  in-process (D10); zero ops dependencies for personal-scale apps.

## Context

Every kit app needs durable storage, and the kit already has two other
data homes: JetStream KV for reactive state (D8,
`0008-embedded-nats-spine.md`) and Parquet for the component pipeline's
canonical artifacts (D10, `0010-component-data-pipeline.md`). What is
missing is the relational truth layer — user documents, app records,
anything whose loss is unacceptable — plus full-text and vector search
in-process, because the Assistant's index (D10/D11) is a SQLite file and
the kit should not run two database engines.

Kit apps are personal-scale (single-user or small-circle, see
`../../VISION.md` non-goals), ship as single binaries behind the Loopback
Invariant (D5), and must build on a clean machine with two commands. Any
store requiring a server process, a port, or an administrator fails before
evaluation begins.

The Go SQLite driver landscape as of 2026-06-10 (dated; re-verify at
upgrade):

- **ncruces/go-sqlite3 v0.34.4** (2026-06-04): SQLite compiled to WASM,
  executed via wazero — cgo-free, cross-compilable, supports FTS5 and can
  load sqlite-vec (https://github.com/ncruces/go-sqlite3/releases).
- **sqlite-vec v0.1.10-alpha.4** (2026-05-18; active, ~7.7k stars):
  brute-force KNN via `vec0` virtual tables — no ANN, which is irrelevant
  at the kit's scale (~7,900 corpus vectors ≈ 24MB float32; brute force
  is sub-millisecond). Official Go bindings ship an `ncruces` package
  that swaps in a sqlite-vec-enabled WASM build (bindings v0.1.6 stable,
  alphas to v0.1.7-alpha.2)
  (https://github.com/asg017/sqlite-vec/releases,
  https://github.com/asg017/sqlite-vec-go-bindings).
- **mattn/go-sqlite3 v1.14.45** (repo active June 2026): the venerable
  cgo driver.
- **modernc.org/sqlite v1.52.0** (2026-06-06): pure-Go translation of
  SQLite 3.51.x, but it **cannot load sqlite-vec**
  (https://pkg.go.dev/modernc.org/sqlite).
- Platform caveat: wazero JIT-compiles on macOS/Windows/Linux amd64+arm64,
  but **iOS forbids JIT** — wazero falls back to interpreter mode with a
  large slowdown. Today this is dormant: mobile tier-1 is PWA (D6), where
  the store lives server-side; it becomes real only if the Wails mobile
  path (PR #5562) graduates into a native iOS target.

## Decision

**ncruces/go-sqlite3 v0.34.x is THE SQLite driver for the kit.** Pinned
exactly in `go.mod`; every kit app and the Assistant's index file use it.
One driver means one set of pragmas, one WASM blob (embedded via the
driver's embed package, so binaries carry their own SQLite — nothing to
install), one mental model, and identical behavior across every
cross-compiled target. Apps talk to it through a thin kit package,
`appkit/store`, which owns open/configure/migrate and hands back a
standard `*sql.DB`; the package is an opt-out-able convenience, not an
ORM, and apps write SQL.

**Division of labor — each datum has exactly one home:**

| Store | Role | Examples | Loss tolerance |
|---|---|---|---|
| SQLite | durable relational truth | user documents, app records, the Assistant's search index | none — backed up (`appkit/store` pre-upgrade snapshot + restore helpers) |
| JetStream KV | reactive, ephemeral state | UI/session state, presence, job progress driving SSE re-renders | rebuildable on restart |
| Parquet | analytical / pipeline artifacts | TailwindPlus typed corpus shards, bulk exports | regenerable from source |

The rule of thumb, stated once and cited everywhere: *if losing it would
hurt, SQLite; if a restart may rebuild it, KV; if it is columnar bulk for
tools to chew on, Parquet.* A datum never lives in two stores; derived
copies (e.g. the SQLite index generated from Parquet, D10) declare their
source and are rebuilt, never edited.

**Defaults set by `appkit/store`:** WAL journal mode, foreign keys on, a
sane busy timeout. FTS5 is available in every app by default; sqlite-vec
loads through the official `ncruces` bindings package wherever semantic
search is wanted (the Assistant requires it; apps may use it).

**Migrations are embedded SQL, forward-only.** Each app ships a
`migrations/` directory of ordered `NNNN_name.sql` files via `go:embed`.
On open, `appkit/store` applies pending migrations transactionally and
records each in a simple version table (`_appkit_migrations`: version,
name, checksum, applied_at). Checksums make edited history fail loudly.
No down migrations — the rollback story is the pre-upgrade snapshot
taken by `appkit/store`'s backup helpers (restored by the same package),
which is honest for single-user apps in a
way that reversible-migration theater is not. No third-party migration
framework: the loop is ~100 lines we fully control.

```go
//go:embed migrations/*.sql
var migrations embed.FS

db, err := store.Open(ctx, store.Config{
    Path:       cfg.DataDir.Join("app.db"), // CUE-validated config (D7)
    Migrations: migrations,
    VecEnabled: false,
})
```

**The iOS plan is named now, executed later.** If a native iOS target
arrives (Wails PR #5562 graduating), we benchmark wazero's interpreter
fallback first; if unacceptable, plan-B is modernc.org/sqlite on mobile
for relational + FTS5 — with vector scoring done in plain Go, since
brute-force dot products over ~8k float32 vectors need no extension — or
a cgo build (mattn + sqlite-vec cgo bindings) for mobile only, where
native toolchains are already mandatory. The `appkit/store` interface is
the seam that makes the swap a build-tag, not a rewrite.

Boundaries: this ADR does not design the Assistant's index schema (D10
owns it), does not govern the bus (D8), and does not forbid an individual
app from outgrowing SQLite — but that app writes its own superseding ADR
and carries its own ops burden; the kit default does not move.

## Alternatives considered

- **mattn/go-sqlite3 (cgo).** Attractive: the battle-tested default of
  the Go ecosystem, native C performance, every extension loadable. Lost
  because cgo poisons exactly what the kit prizes: cross-compiling to
  every desktop target from one machine requires per-target C toolchains,
  clean-machine builds stop being two commands, and build determinism
  inherits the host's C compiler. cgo everywhere, forever, for every app,
  is the one cost the kit refuses categorically (D2's spirit).
- **modernc.org/sqlite (pure-Go translation).** Attractive: cgo-free like
  ncruces, no WASM runtime, mechanical translation of real SQLite.
  Lost on one hard fact: it cannot load sqlite-vec (verified 2026-06-10),
  and the Assistant's hybrid search (D10) is non-negotiable. Running
  modernc in apps and ncruces in the Assistant would mean two drivers and
  two behavior profiles for zero benefit. It survives as the named
  mobile plan-B, which is exactly the right job for it.
- **bbolt / badger (pure-Go embedded KV).** Attractive: simple, fast,
  zero dependencies. Lost because the kit already has a KV — JetStream
  (D8) — and a second KV answers no open question, while losing SQL,
  joins, FTS5, and vectors. Relational queryability is the point.
- **Postgres (or any client-server store).** Attractive: the most capable
  open-source database; the enterprise-mean answer. Lost because it is an
  ops dependency a personal app must not carry: a server process to
  install, start, upgrade, and back up — on every machine the app runs
  on, for decades. It breaks single-binary distribution, the two-command
  build, and the day-one acceptance test. Personal-scale data fits in
  SQLite with orders of magnitude to spare.

The real costs of the winner: a WASM runtime between us and SQLite
(performance tax versus cgo accepted as the price of cross-compilation;
unmeasured for our workloads — measure before optimizing), the iOS JIT
hole, a younger driver than mattn, and sqlite-vec's pre-1.0,
single-maintainer, slow-alpha cadence (bindings' last stable tag Nov
2024) — pin exact versions and vendor the WASM blobs.

## Consequences

- Easier: every desktop/server target cross-compiles from one machine;
  backup = copy one file; FTS5 and semantic search are in-process library
  calls; the Assistant's index and app stores share one driver and one
  set of lessons.
- Harder: we forgo the mattn ecosystem's decades of folklore; ncruces
  quirks are ours to discover. Mitigation: `appkit/store` centralizes
  pragmas and workarounds so each lesson is learned once.
- Risk: ncruces/go-sqlite3 stalls or wazero breaks on a new Go release.
  Tripwire: six months without a release, or a Go upgrade blocked by the
  driver. The store interface plus standard SQL makes modernc the tested
  fallback (minus vec, which falls back to plain-Go scoring).
- Risk: sqlite-vec abandonment. Tripwire: a needed fix unmerged for two
  quarters. At our scale, brute-force scoring in Go is a weekend
  replacement, not a crisis.
- Risk: the WASM performance tax bites a real workload. Tripwire: a
  measured, user-visible store bottleneck in a kit app. Then: profile,
  and only then consider a cgo build for that app.
- Risk: iOS interpreter slowdown. Tripwire: Wails mobile (PR #5562)
  merges and an iOS app is planned — execute the benchmark-then-plan-B
  sequence above before committing to ship.

## Revisit when

- A native iOS target becomes real (Wails PR #5562 graduates) — run the
  mobile plan now, not then.
- ncruces/go-sqlite3 reaches v1.0 (re-pin and adopt stability guarantees)
  or goes quiet for six months (activate fallback).
- sqlite-vec ships a stable v1.x with refreshed Go bindings, or stalls
  past two quarters on a needed fix.
- modernc.org/sqlite gains the ability to load sqlite-vec — the
  one-driver calculus changes and this ADR gets re-argued.
- Any kit app's data outgrows single-file SQLite in measured fact (not
  vibes) — that app supersedes this default for itself, with its own ADR.
