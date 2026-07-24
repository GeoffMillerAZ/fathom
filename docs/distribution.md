# Distribution — building fathom without agentic-kit access

fathom depends on one private module, `github.com/geoffmilleraz/agentic-kit`,
for governed-evidence primitives (evidence tiers, `llmclient`, `embedding`,
`security`, `telemetry`, `contracts` — ADR-009). Everyone on the fathom repo
does **not** also have access to that private repo. This doc is for them:
clone fathom, build it, done — no extra credentials, no extra clone.

## How it works (ADR-019)

`third_party/agentic-kit/` is a full, committed mirror of the agentic-kit
tree at **exactly** the commit `go.mod`'s `require` line pins (currently a
pseudo-version; the trailing 12 hex characters are the commit). `go.mod`
carries:

```
replace github.com/geoffmilleraz/agentic-kit => ./third_party/agentic-kit
```

Two resolution paths, both already proven against this repo:

- **Cold clone (no `go.work`, no sibling checkout):** `GOWORK` is unset, so
  the `replace` above governs — the build uses `third_party/agentic-kit`
  and nothing else. This is the path everyone without kit access takes.
- **Local dev (this repo's own `go.work`, sibling `../agentic-kit`
  checked out):** the workspace's `use ../agentic-kit` directive takes
  precedence over the `go.mod` replace, so day-to-day development inside
  the fathom+agentic-kit monorepo pair edits the sibling directly and sees
  changes immediately, same as before this existed.

You can confirm which copy a given invocation resolves to:

```sh
go list -m -f '{{.Dir}}' github.com/geoffmilleraz/agentic-kit               # dev:   .../agentic-kit (sibling)
GOWORK=off go list -m -f '{{.Dir}}' github.com/geoffmilleraz/agentic-kit    # cold:  .../fathom/third_party/agentic-kit
```

## Building fathom (no kit access)

```sh
git clone https://github.com/geoffmilleraz/fathom.git
cd fathom
go build ./...              # no go.work here, so the go.mod replace applies automatically
task validate                # or: go test -race ./...
```

Nothing else is required. `third_party/agentic-kit` is already in the repo;
there is no submodule to init, no extra `go mod download` step, no `../
agentic-kit` to clone.

## Air-gapped / fully offline builds

`go mod vendor` now succeeds standalone, because the one module that used to
require network+auth access (agentic-kit) resolves in-tree:

```sh
go mod vendor
```

This produces a local `vendor/` directory that `go build -mod=vendor` can
build from with zero network access. **`vendor/` is gitignored and is your
local artifact** — do not commit it; regenerate it on whichever machine
needs the air-gapped build. Only `third_party/agentic-kit` (716KB–13MB
depending on kit history, see the note below) is committed — the full
`vendor/` tree never is.

**CGO note (ADR-013/ADR-020/ADR-025).** `modernc.org/sqlite` — the pure-Go,
WAL-mode store backend (the only backend; ADR-020) — adds no CGO surface and
no static libraries. SS-8 (`docs/plans/store-sqlite.md`) deleted the DuckDB
backend, its driver (`github.com/marcboeker/go-duckdb/v2`), its per-platform
bindings packages, and the ~565MB of static libs they contributed to the
vendor footprint, outright — the trigger ("operator runs migrate-store on
the fleet DB and the daemon runs a full day green on sqlite") fired
2026-07-23. `internal/store` is now 100% pure Go: `go build`/`go test` pass
with `CGO_ENABLED=0` for that package with zero workarounds.

`CGO_ENABLED=1` and `MACOSX_DEPLOYMENT_TARGET=14.0` are still required for
`task validate` / a full `go build ./...`, but for an unrelated reason now:
the tree-sitter extractor bindings (`github.com/tree-sitter/go-tree-sitter`
and the per-language grammar packages, ADR-013) are CGO and unconditionally
imported by `internal/extractor`. Moving them to wazero/WASM is ADR-004's
documented future path — a separate, not-yet-triggered slice (XT-6b) — so
CGO is not yet zero for the whole binary, only for the store.

## For fathom maintainers: keeping third_party/agentic-kit in sync

Whenever `go.mod`'s `agentic-kit` pin moves (a normal kit-pin bump during
co-development), re-sync the mirror and commit it in the same change:

```sh
task sync-kit
git status third_party/agentic-kit   # review, then commit
```

`scripts/sync-kit.sh` re-archives `../agentic-kit` at the exact commit
`go.mod` pins (`git archive <commit> | tar -x`) into `third_party/agentic-kit`.
It's idempotent and refuses to run if the sibling checkout doesn't have that
commit yet (fetch/pull it there first).

**Forgetting this is a push-time gate failure, not a silent drift.**
`task vendor-check` (part of `task validate`, so part of the pre-push
lefthook gate) diffs `third_party/agentic-kit`'s content against what
`../agentic-kit` has at the pinned commit and fails the build if they
disagree. It skips gracefully (exit 0, with a warning) when `../agentic-kit`
isn't present — the expected case for anyone without kit access, including
CI runners that never clone the private repo.

## A note on size

`docs/distribution.md`'s ground assumption going in was that the kit's tree
archives to ~716KB. The commit actually pinned as of this writing archives
to **~13MB** (1,153 files) — the kit's own `foundations/` test suite
accounts for most of it, plus one large (~4.1MB) platform-specific compiled
binary (`kit-new`, a Mach-O executable) that appears to have been committed
to the kit by accident in a `docs:` commit. `third_party/agentic-kit` is a
faithful mirror of the pinned commit — including that binary — because the
mirror's whole reason to exist is exactness (and `vendor-check`'s content
hash needs both sides to match byte for byte). If a future kit commit prunes
it, the next `task sync-kit` will drop it here too automatically.
