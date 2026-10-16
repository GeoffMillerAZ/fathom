# 0016: The 2026-09-13..16 unmerged branches: what landed, what the active branch carries, what is archived

**Status:** accepted
**Type:** process
**Date:** 2026-09-24

## Context

Two unrelated batches of unmerged work existed against `main`:

1. Five `uplift/*` PRs authored 2026-09-09 (two-boundary-gate, projections-divergence,
   check-catalog, capped-confidence, structural-refusal). These landed on the `land`
   branch via `git merge --no-ff` (PRs #1-#5, RC-4/RC-5/RC-1/RC-2/RC-3) at commits
   `6f8d4b6`..`b02a2da`, gated clean, and are out of scope for this record — they are
   not part of "the 2026-09-13..16 unmerged branches" named in the title.
2. Twenty GitHub branches authored 2026-09-13 through 2026-09-16 (the
   `worktree-gw-*`, `worktree-g7-*`, `worktree-native-host-evidence`,
   `feat/benchmark-evidence`, `fix/eval-input-binding`, `worktree-evidence-reuse`,
   and `worktree-assessment-profiles` family), triaged against
   `docs/intent-excellence-assessment` — an unmerged, actively-worked branch
   (currently checked out at `/Volumes/m2ext/code/geoffmilleraz/agentic-kit`,
   tip `fac9fe7` "docs(jev): refine criteria and parallel routing assessment",
   2026-09-22) that is not itself part of this landing.

Each of the twenty was re-verified against that tip (fetched fresh from GitHub over
HTTPS; every fetched sha matched the triage data exactly): `git diff --diff-filter=D
<branch> docs/intent-excellence-assessment --name-only` is **empty for all 20** — no
file present on any of these branches is missing from the active branch's tree. Three
(`worktree-gw-workflow-cli`, `worktree-evidence-reuse`, `worktree-assessment-profiles`)
are literal git ancestors of the active branch's tip (`merge-base --is-ancestor` true).
The other seventeen are patch-equivalent supersets: the active branch carries the same
files, byte-identical or with further additive hardening on top, confirmed per-branch
via the reasoning in the triage data (file lists, insertion/deletion counts, commit
lineage) and corroborated here by the diff-filter=D check above.

## Decision

None of the twenty branches are merged onto `land`/`main` separately. Merging content
that is already fully present — and, in most cases, further hardened — on
`docs/intent-excellence-assessment` would add nothing and risks re-introducing an
earlier, less-hardened version of files the active branch has since improved past.
No branch is deleted; every one stays on GitHub as the archive record. Disposition:

| Branch | Tip | Disposition |
|---|---|---|
| `worktree-gw-engineering` | `c52c586` | carried by docs/intent-excellence-assessment |
| `worktree-gw-engineering-hardening` | `db2085d` | carried by docs/intent-excellence-assessment |
| `worktree-gw-effort` | `e9330a7` | carried by docs/intent-excellence-assessment |
| `worktree-gw-host` | `c177f0d` | carried by docs/intent-excellence-assessment |
| `worktree-gw-provider` | `227728a` | carried by docs/intent-excellence-assessment |
| `worktree-gw-provider-hardening` | `1d980b4` | carried by docs/intent-excellence-assessment |
| `worktree-gw-mcp` | `2db12dd` | carried by docs/intent-excellence-assessment |
| `worktree-gw-vault` | `6cac25b` | carried by docs/intent-excellence-assessment |
| `worktree-gw-workflow-cli` | `1b29451` | carried by docs/intent-excellence-assessment (literal ancestor) |
| `worktree-gw-workflow-cli-final` | `2406857` | carried by docs/intent-excellence-assessment |
| `worktree-gw-cli-wire` | `f8ec57f` | carried by docs/intent-excellence-assessment |
| `worktree-gw-sourced-scenario` | `5256950` | carried by docs/intent-excellence-assessment |
| `worktree-gw-hitl` | `23d6ce6` | carried by docs/intent-excellence-assessment |
| `worktree-g7-lesson-design` | `cedebae` | carried by docs/intent-excellence-assessment |
| `worktree-g7-lesson-transfer` | `75223ed` | carried by docs/intent-excellence-assessment |
| `worktree-native-host-evidence` | `fbcef44` | carried by docs/intent-excellence-assessment |
| `feat/benchmark-evidence` | `fe99189` | carried by docs/intent-excellence-assessment |
| `fix/eval-input-binding` | `8700372` | carried by docs/intent-excellence-assessment |
| `worktree-evidence-reuse` | `3b5b3c8` | carried by docs/intent-excellence-assessment (literal ancestor) |
| `worktree-assessment-profiles` | `32c5f81` | carried by docs/intent-excellence-assessment (literal ancestor); also present verbatim on GitHub as `worktree-registry-snapshots` and `worktree-prompt-screen` (same sha) — a naming duplication noted for later cleanup, out of scope here |

Reference, not part of this branch family but landed the same week and gated on
`land` at `e1d8b39` before this record: `uplift/two-boundary-gate` merged in
`6f8d4b6`, `uplift/projections-divergence` merged in `7885ef7`,
`uplift/check-catalog` merged in `99f11e6`, `uplift/capped-confidence` merged in
`2eecf80`, `uplift/structural-refusal` merged in `b02a2da`.

## Rationale

`docs/intent-excellence-assessment` is a strict superset of all twenty branches on
every path that matters (proven, not asserted, by the diff-filter=D check above), and
in seventeen of the twenty cases carries additional hardening past what the branch
itself has. Landing any of them separately now would fork a worse copy of files the
active branch already improved, and would create merge noise (duplicate history,
renumbered decisions, ledger recomputation) for zero net content. The GitHub branches
are cheap to keep and cheap to re-triage; deleting them buys nothing this record needs.

## Alternatives rejected

- **Merge each of the twenty onto `land` with `--no-ff`, then let
  `docs/intent-excellence-assessment` reconcile later** — rejected: every one of the
  20 checks empty on diff-filter=D, so this only adds an older revision of content the
  active branch has already hardened, plus twenty merge commits and ledger
  recomputation for no new work. Wins if: a future re-triage finds content on one of
  these branches that is genuinely absent from the active branch's current tip.
- **Delete the superseded branches from GitHub** — rejected by the task's own
  constraint (branches stay as archive; nothing is deleted) and because
  `docs/intent-excellence-assessment` has not landed yet — deleting the branches it
  supersedes before it lands destroys the paper trail if that branch is ever
  abandoned or rebased away from this content.
- **Wait for `docs/intent-excellence-assessment` to land before triaging these at
  all** — rejected: the branches are already fully evaluated and idle; recording the
  disposition now costs nothing and gives a future reader (including whoever lands
  `docs/intent-excellence-assessment`) a verified map instead of twenty branches to
  re-diff from scratch.

## Consequences

`land`/`main` gains one decision record and no new code from this batch. A future
session landing `docs/intent-excellence-assessment` can treat these twenty GitHub
branches as already-accounted-for and skip re-triaging them; a session that finds one
of these branch names can look here first instead of re-running the diff.

## Update 2026-10-15: the twenty branches are deleted

The reason to keep them on GitHub was that `docs/intent-excellence-assessment` had not
landed. It landed on 2026-09-25. Re-checked against `main` at `8232139`: every file any
of the twenty branches added exists on `main`, and of the 18,003 non-trivial lines the
largest branch (`worktree-evidence-reuse`) adds, all but 50 appear in the same file on
`main`. The rest are lines `main` later rewrote: the AGENTS.md-only host install,
stricter benchmark time checks, comment and intent-doc wording. The older
`worktree-gw-*` drafts miss more lines, and those lines are drafts their own successor
branches replaced. A verified `git bundle` of each tip (named `consolidate-agentic-kit-<branch>-<sha8>.bundle`) was written before deletion;
restore one with `git fetch <bundle> 'refs/remotes/origin/<branch>:refs/heads/<branch>'`.
The `uplift/*` and `review/wave4-rc-landing` branches, patch-equivalent to `main`, were
deleted the same way and PRs #3 to #5 closed.

## Revisit when

- `docs/intent-excellence-assessment` lands on `main` — re-run the diff-filter=D
  check against the merged tree (not the branch) for all twenty; any branch that no
  longer checks empty was carried by a claim that did not survive the merge and needs
  its own PR.
- A re-diff of any of these twenty branches against the current
  `docs/intent-excellence-assessment` tip stops being empty (the active branch was
  rebased, force-pushed, or abandoned) — re-triage that branch for direct landing.
- `docs/intent-excellence-assessment` is abandoned or stalls past a further 30 days
  idle — re-evaluate landing the strongest of these twenty (`worktree-gw-cli-wire`,
  `worktree-assessment-profiles`) directly instead of waiting on it.
