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
integration branch and the rows open on it (SV-1, SV-2):

| Class | Means |
|---|---|
| `landed` | An ancestor of the integration branch, patch-equivalent to it (`git cherry` finds an equivalent for every commit), or content-equal (every path the line changed reads the same there, as after a squash merge), with a clean worktree. The note names how it landed and the command that would remove it, for example `git branch -d done` or `git worktree remove <path> && git branch -D picked`. |
| `claims-rows` | Its own plan edits (committed, or in its dirty worktree) flip rows that are still open on the integration branch. The row ids are named, `<plan>.md#<ID>`. |
| `touches-rows` | Its diff touches an open row's anchors at the lexical tier: a changed line in a file under the row's `package:` carries the row's symbol as a whole identifier, or the diff changes the row's oracle `path:`. Plan files are not read for this. |
| `at-risk` | A dirty worktree, a stash, or commits on no remote. The reasons are listed whatever the class. |
| `idle` | None of the above and no commit for `--idle-days` days. |
| `active` | None of the above, with a recent commit. |

SV-M1 (`docs/bench/readings/stewardship/lines-replay.md`) read CUT for the two rows classes:
over the 2026-09-29 consolidation they flagged 36 lines against a bar of 25. Landed and at
risk stand; `claims-rows` and `touches-rows` still print in this version until the owner
removes them (O293), so read them as hints, not as a work list.

A line takes the first class that holds, in that order. Open rows are the rows of the
integration branch's `docs/plans/*.md` (README.md excluded) that are neither `[x]` nor
`[-]`. Each line carries its rows, its age in days and its tip; `--json` prints the
whole report (`root`, `integration`, `integration_tip`, `open_rows`, `lines[]` with
`kind`, `name`, `tip`, `worktree`, `class`, `landed`, `claims`, `touches`, `tier`,
`at_risk`, `ahead`, `committed_at`, `age_days`, `removal`, and `counts`), a
compatibility surface (`docs/intent/scope.md`).

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
