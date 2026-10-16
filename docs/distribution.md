# Distribution — building fathom without kit access

fathom builds on two modules that are not published:
`github.com/geoffmilleraz/agentic-kit`, for governed-evidence primitives
(evidence tiers, `llmclient`, `security`, `telemetry`, `contracts`;
ADR-009), and `github.com/geoffmilleraz/app-kit`, the observatory's UI kit.
Both are mirrored in this repository (`third_party/agentic-kit/`,
`third_party/app-kit/`), and since ADR-070 the mirrors are **packages of
fathom's own module**, not separate modules. fathom is one self-contained
Go module: its `go.mod` requires no private module and carries no `replace`
or `exclude`, so the public module proxy can serve it like any other.

```sh
go install github.com/geoffmilleraz/fathom/cmd/fathom@<tag>   # a stranger's install
go get github.com/geoffmilleraz/fathom/pkg/client@<tag>        # a Go consumer of the client
```

Both need no credentials from the first tag that carries ADR-070; the
reading after that release is `docs/plans/distribution.md` DS-M1. (`go
install pkg@version` refuses any module whose `go.mod` has a `replace`, which
is why the mirrors could not stay replaced modules.) The kit types `pkg/client` takes (`WithDecision`'s
`decision.Decider`, `WithLLM`'s `llmclient.LLMClient`) live at
`github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/...`.

## How it works (ADR-019, ADR-070)

Each `third_party/<kit>/` is a committed copy of the kit's tree at the
commit its `.vendor-commit` records (agentic-kit's whole, app-kit's reduced
to fathom's build closure, below) — the mirror's single pin — in one
reversible, mechanical form (`cmd/kitrewrite`, `internal/kitrewrite`):

- the kit's `go.mod`, `go.sum`, `go.work` and `go.work.sum` are kept as
  `go.mod.upstream`, `go.sum.upstream`, ... (content intact, provenance only),
  so the directory is not a nested module;
- every import of the kit's upstream path is rewritten into fathom's module:
  `github.com/geoffmilleraz/app-kit/appkit/ui/card` becomes
  `github.com/geoffmilleraz/fathom/third_party/app-kit/appkit/ui/card`. Only
  import path literals change (found by `go/parser`, in `.go` and `.templ`
  files); comments and strings that merely name a kit, `testdata/`, and the
  kits' own nested modules (`kits/docextract/pdf`, `spikes/wails-gallery`,
  ...) are left exactly as upstream wrote them, and nothing is reformatted.

`go run ./cmd/kitrewrite -reverse` (or `-reverse -kit <dir>` for one copy)
restores the upstream form byte for byte; that is how `vendor-check` proves a
mirror is still its pinned commit. `go run ./cmd/kitrewrite -check` names
every way the tree is not self-contained; the same check is a test in the
gate (`TestRepositoryIsSelfContained`, DS-1).

**app-kit's mirror holds only fathom's build closure (DS-4).** Since ADR-070
the mirror ships in every module zip the Go proxy caches, so it keeps only what
fathom builds: `scripts/kit-closure.sh` computes the app-kit packages fathom's
own packages (tests included) and app-kit's `cmd/appkit` (which
`scripts/ui-css.sh` builds) reach over five GOOS/GOARCH pairs, their non-test
files and `go:embed` targets, every mirror path a non-Go file outside
`third_party/` names, and the identity and provenance files and
`fathom-pack/`, into the committed `third_party/app-kit.keep`. `task sync-kit`
reduces a fresh copy to it, `task vendor-check` compares only its paths, and
`task scripts-check` fails when the manifest and the computed closure part.
app-kit's design-asset data (the TailwindPlus manifest and indexes), docs,
spikes and other apps are not in fathom's tree from then on; history is not
rewritten (the licensed dump was never committed, and a rewrite would move
every tag and the SHAs fathom's evidence cites; ADR-070 decision 8).

`go.mod` requires what the kits' packages need (app-kit's requirements, at
app-kit's own pins), even where `cmd/fathom` never links them: every package
under `third_party/` is a package of this module and must build.

fathom's own gates leave kit code alone: `fmt`, `vet` and every `go test` in
`Taskfile.yml` run over `go list ./... | grep -v /third_party/`, and the
lefthook gofmt skips `third_party/`. Kit tests run in the kit's own CI.

**Live co-development changed.** Before ADR-070 a local, uncommitted
`go.work` with `use ../agentic-kit` let fathom build against an edited
sibling directly. That no longer applies: fathom's imports no longer name
the kit's module, so a workspace module called
`github.com/geoffmilleraz/agentic-kit` satisfies nothing. The path is the
sync: commit in the sibling, then `bash scripts/sync-vendor.sh agentic-kit`
(the sibling's HEAD, or a named commit) and build.

## Building fathom (no kit access)

```sh
git clone https://github.com/geoffmilleraz/fathom.git
cd fathom
go build ./...
task validate
```

Nothing else is required: no submodule, no extra `go mod download` step, no
sibling checkout, no `go.work`.

## Air-gapped / fully offline builds

`go mod vendor` succeeds standalone, because fathom requires no private
module (the kits are packages of fathom's own module, ADR-070):

```sh
go mod vendor
```

This produces a local `vendor/` directory that `go build -mod=vendor` can
build from with zero network access. **`vendor/` is gitignored and is your
local artifact** — do not commit it; regenerate it on whichever machine
needs the air-gapped build. Only `third_party/agentic-kit` (716KB–13MB
depending on kit history, see the note below) is committed — the full
`vendor/` tree never is.

**CGO note (ADR-066, superseding ADR-013; ADR-020/ADR-025).** `modernc.org/sqlite` — the pure-Go,
WAL-mode store backend (the only backend; ADR-020) — adds no CGO surface and
no static libraries. SS-8 (`docs/plans/store-sqlite.md`) deleted the DuckDB
backend, its driver (`github.com/marcboeker/go-duckdb/v2`), its per-platform
bindings packages, and the ~565MB of static libs they contributed to the
vendor footprint, outright — the trigger ("operator runs migrate-store on
the fleet DB and the daemon runs a full day green on sqlite") fired
2026-07-23. `internal/store` is now 100% pure Go: `go build`/`go test` pass
with `CGO_ENABLED=0` for that package with zero workarounds — and the gate
proves it on every push: `task store-purego` (a `task validate` dep,
invariant I2) runs `CGO_ENABLED=0 go test -race ./internal/store/`.

The rest of the binary followed (ADR-066, 2026-10-10): every tree-sitter
grammar runs as WASM on **wazero** (`internal/extractor/tsengine`, ADR-048),
the CGO walkers left in wave lf3, and the last importer of
`github.com/tree-sitter/go-tree-sitter` with LB-F19, so `go.mod` requires no
tree-sitter binding and the vendored tree carries no C sources. A Linux
build is `CGO_ENABLED=0`, a static pure-Go binary that needs no C compiler
(`task build` pins it; `cmd/fathom/cgofree_test.go` guards the dependency
set, CS-9). On macOS `task build` keeps cgo for two files only,
`cmd/fathom/host_darwin.go` and `rss_darwin.go` (the daemon's memory readers,
`proc_pid_rusage` and `proc_listchildpids`), with
`MACOSX_DEPLOYMENT_TARGET=14.0` setting the binary's minimum macOS; a darwin
build without cgo works and reads those Unknown. `task validate` on Linux
still needs cgo and a C compiler, for `go test -race`. The C++
row also carries a `doctor` check (`cpp_toolchain`: a CMake repo should have
`compile_commands.json` for clang-based tooling); that is a repo-side
readiness check, not a build requirement for fathom itself.

## For fathom maintainers: keeping the mirrors in sync

The pin is each mirror's `third_party/<kit>/.vendor-commit`; `go.mod` does
not name the kits. To move a pin, commit in the sibling checkout, then
re-sync and commit the mirror in the same change:

```sh
task sync-kit                                     # both kits, each at its sibling's HEAD
bash scripts/sync-vendor.sh agentic-kit <commit>  # one kit at a named commit
git status third_party/                           # review, then commit
```

`scripts/sync-vendor.sh` archives the sibling at the commit
(`git archive <commit> | tar -x`) into a scratch copy, rewrites it into
fathom's module (`go run ./cmd/kitrewrite -kit <copy>`), replaces
`third_party/<kit>` with it and stamps `.vendor-commit`. Re-syncing at the
recorded commit reproduces the committed mirror byte for byte. If a kit adds
a requirement, `go mod tidy` at fathom's root takes it in.

**A hand edit is a push-time gate failure, not a silent drift.**
`task vendor-check` (part of `task validate`, so part of the pre-push
lefthook gate) copies each mirror, reverses the rewrite
(`cmd/kitrewrite -reverse -kit`), and compares its content with the
sibling's archive at `.vendor-commit`; it also fails a mirror that still
holds its own `go.mod` (an unrewritten copy). It skips gracefully (exit 0,
with a warning) when the sibling isn't present or lacks the commit — the
expected case for anyone without kit access, including CI runners that never
clone the private repos.

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
