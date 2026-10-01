# Stewardship reports

Reports that join git or file state with a fact fathom already computes (ADR-060, vision
`docs/vision/41-stewardship.md`, plan `docs/plans/stewardship.md`). Each runs on the
operator's ask. Fathom reports and never acts: it deletes, merges, squashes, rebases and
checks out nothing, and prints the command a person or agent would run instead (I63, P15).

## `fathom plans lines`

```sh
fathom plans lines [--root DIR] [--integration main] [--idle-days 14] [--json]
```

Classifies every local branch, linked worktree (a detached one, or the integration
checkout while it holds uncommitted work) and stash of one repository against the
integration branch (SV-1, SV-2):

| Class | Means |
|---|---|
| `landed` | An ancestor of the integration branch, patch-equivalent to it (`git cherry` finds an equivalent for every commit), or content-equal (every path the line changed reads the same there, as after a squash merge), with a clean worktree. The note names how it landed and the command that would remove it, for example `git branch -d done` or `git worktree remove <path> && git branch -D picked`. |
| `at-risk` | A dirty worktree, a stash, or commits on no remote. The reasons are listed whatever the class. |
| `idle` | None of the above and no commit for `--idle-days` days. |
| `active` | None of the above, with a recent commit. |

The `claims-rows` and `touches-rows` classes (a line's plan edits flip open rows; its diff
names an open row's anchors) were removed on 2026-10-01: SV-M1
(`docs/bench/readings/stewardship/lines-replay.md`) read CUT for them, 36 lines flagged
over the 2026-09-29 consolidation against a bar of 25 (ADR-060 amendment 2026-10-01,
O293). The report reads no plan file. A line that landed and was then edited on the
integration branch still reads at risk, idle or active; a per-commit "already applied"
test is a recorded future (F60), not a class.

A line takes the first class that holds, in that order. Each line carries its age in days
and its tip; `--json` prints the whole report (`root`, `integration`, `integration_tip`,
`lines[]` with `kind`, `name`, `tip`, `worktree`, `class`, `landed`, `at_risk`, `ahead`,
`committed_at`, `age_days`, `removal`, and `counts`), a compatibility surface
(`docs/intent/scope.md`).

Every git read runs through `internal/gitro`: `git status` and `git diff` read a private
copy of the index, so the report never locks or rewrites `.git/index` in a checkout
someone is working in. `TestCmdPlansLinesReportsAndNeverActs` runs the verb with a git
runner that refuses any write subcommand and checks that refs, reflogs, every index and
the worktree list are byte-identical before and after.

## `fathom gc`: strays and binaries

```sh
fathom gc [--repo DIR] [--json] [--apply]
```

Besides its retention classes, `fathom gc` lists two report-only classes for the repository
(`--repo`, default the enclosing one; SV-4):

| Class | Lists |
|---|---|
| `strays` | Untracked files and directories git does not ignore, outside the declared homes: `workspace/` and the ADR-039 build-cache roots (`FATHOM_CACHE_ROOT`, `GOCACHE`, `GOMODCACHE`, `GOTMPDIR` when they lie inside the repository). A fully untracked directory is one item. |
| `binaries` | Executable binaries (an execute bit and an ELF, Mach-O or PE header; a script is not one), committed or lying untracked in the tree. |

Both carry the action `report`: `--apply` never removes them and the reclaimable totals
leave them out, because they are not fathom's (I63). The line says what to do instead:
commit it, ignore it, or move it under `workspace/`. Only the verb plans these classes;
the daemon's daily retention pass and `doctor` never do.

## `fathom plans status`: archivable plans

A plan with rows, none of them open or in progress, and every claimed row proven (none
claimed-unproven, none left unmeasured by a stale daemon) is archivable (SV-5). The human
table ends with one line per such plan naming the command, and `--json` adds
`"archivable": true` and `"archive_command"` to its element:

```text
archivable: activation.md (0 open, 0 claimed-unproven) — fathom plans archive --service fathom docs/plans/activation.md
```

Deferred and refused rows do not hold a plan open; `fathom plans archive` keeps them in the
archive with the rest. Status only names the command; the archive is the operator's call.

## `fathom report used`: a dependency's used closure

```sh
fathom report used <dir> [--root DIR] [--main PKGDIR]... [--css FILE]... [--json]
```

For a dependency directory the consumer's `go.mod` replaces with a local path (fathom's
`third_party/app-kit` and `third_party/agentic-kit`), lists what the product uses and why
(SV-6, SV-7):

| Kind | Unit | Reached through |
|---|---|---|
| `go-package` | a package of the directory's module | the import graph from the main packages (every one outside `<dir>` and `workspace/`, or each `--main`), files matched for this platform as `go/build` does, tests never. The set equals `go list -deps` of the same mains restricted to the module (tested on a fixture and on this repository). A package keeps every file in its directory, `testdata/` included: pruning inside a Go package is refused. |
| `embed` | a file a reached package embeds | its `//go:embed` patterns (a directory skips dot and underscore files unless `all:`) |
| `css-source`, `css-import` | an `@source` glob or `@import` file | a consumer CSS entry (each `--css`, or every `.css` file declaring one into `<dir>`), such as a Tailwind source list |
| `js-module` | a TypeScript or JavaScript module | relative import edges from the units above |
| `module-file` | `go.mod`, `go.sum` | the consumer's `replace` |

Each unit names its entry point and first importer (breadth first from the sorted entries,
so the reason is the shortest import chain); the module's packages and the TS/JS modules
nothing reaches are listed as unreached, with file and byte totals. `--json` prints the
whole result (`root`, `dir`, `module`, `entries`, `css_entries`, `used[]`, `unreached[]`
with `kind`, `id`, `dir`, `entry`, `first_importer`, `files`, `bytes`; `totals`;
`used_files`, every file of the used units), a compatibility surface. Nested modules under
`<dir>` are other modules and are never units.

The report reads files only: it runs no `go` command, fetches nothing, and copies, prunes or
vendors nothing (SV-X4). A pruning or sync script reads `used_files` from the JSON.
