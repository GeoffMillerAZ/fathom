# SV-M1: `fathom plans lines` replayed over the 2026-09-29 consolidation

**Verdict in plain language.** `fathom plans lines` is useful for the question a clean-up asks first: which
lines are safely on `main`, and which hold work that could be lost. It named every line the 2026-09-29
consolidation removed without saving anything as landed (46 of 46), caught all three items the manual harvest
held back (both valuable kit items and the unfinished CSS worker read at risk), and found what `git branch
--merged` cannot see: a patch-equivalent branch, seven worktrees with uncommitted edits, and a workflow worktree
whose unpushed commits claim an open row (`ambient-fathom.md#SH-2`). It is worse than reading the lines by hand
in one way that the row's bar was written to catch: at the lexical tier the rows classes flag too much. Across the
two repositories 36 lines read claims rows, touches rows or at risk, against a bar of 25; 18 of them are old kit
branches whose work largely landed later in another shape (half to nine in ten of their files already read the
same on `main`), flagged because their diffs name an open row's symbol (`New` in `foundations/agent/mcp-client`). It pairs
with git (it only reads git) and replaces the first pass of a harvest, not the judgement. Keep the landed and at-risk
classes; cut the rows classes (claims rows, touches rows) as vision 41 §7 says; refuse SV-3, the attention count
built on them.

SV-M1 verdict: CUT

## The bars, fixed before the run

From the row (`docs/plans/stewardship.md` SV-M1) and vision 41 §4.1, written on 2026-09-30 and not changed since;
the run started 2026-10-01T16:19:17Z.

1. Both valuable held items and the unfinished worker read claims rows, touches rows or at risk.
2. At least 9 in 10 of the lines the consolidation removed as merged read landed.
3. At most 25 lines are flagged for a look (claims rows, touches rows or at risk).

A CUT is bar 1 or bar 3 failing (vision 41 §7: "a valuable held item missed, or more than 25 lines flagged");
it kills the rows classes, and the landed class stays.

## Method ($0, no model, nothing of the owner's touched)

- Input: a read-only copy of `workspace/durable-2026-09-29/cleanup/` (259 MB: `MANIFEST.md`, both all-refs
  bundles, the five stash bundles and messages, the worktree patches and untracked-file archives, the three held
  bundles). No git command ran in the owner's tree.
- `lines-replay.sh` (beside this file) rebuilds each repository from its all-refs bundle (every branch, remote
  ref, tag and archive ref), re-creates its stash list in order from the stash bundles, re-creates every linked
  worktree of `worktrees.tsv` (clean ones sparse, dirty ones whole with their saved patch and untracked files put
  back), and rebuilds each held item as the scratch clone it lived in (its own `main` as `origin/main`; for the
  CSS worker, whose bundle has no `main`, the merge base of its branch with today's `origin/main`). Then it runs
  `fathom plans lines --json` on each. `lines-replay-score.py` scores the JSON.
- Binary: this branch's `fathom` at the SV-1 change that lands a dirty worktree whose edits read the same on the
  integration branch (decided before the run, from the design's content-equal rule, not from this data).
- Integration branches as of the backup: fathom `main` f383a64a (equal to GitHub), agentic-kit `main` 53024ffa.

## Results

| Repository | Lines | landed | claims rows | touches rows | at risk | idle | Open rows |
|---|---:|---:|---:|---:|---:|---:|---:|
| fathom (sextant) | 59 | 46 | 1 | 2 | 10 | 0 | 536 |
| agentic-kit | 42 | 13 | 0 | 18 | 5 | 6 | 69 |
| held: wa_kit clone | 2 | 0 | 0 | 0 | 2 | 0 | 69 |
| held: agentsmd_kit clone | 1 | 0 | 0 | 0 | 1 | 0 | 69 |
| held: CSS worker clone | 1 | 0 | 0 | 0 | 1 | 0 | 539 |

Landed by how: fathom 45 ancestor, 1 patch-equivalent (`fix-f`); agentic-kit 10 ancestor, 3 patch-equivalent
(`uplift/capped-confidence`, `uplift/check-catalog`, `uplift/structural-refusal`). `git branch --merged main`
lists 52 fathom branches: the 45 ancestors plus the 7 whose worktrees held uncommitted edits, which it cannot
see; it misses `fix-f`.

**Bar 1: PASS.** `wa-kit` (5 commits on no remote), `wa-kit-lefthook` (6), `agentsmd` (7) and
`wip-css/lang/css` (1) all read at risk. None reads claims or touches rows: the kit items change a lefthook gate
and a skills directory, and the CSS worker's one commit names no open row's symbol.

**Bar 2: met on the consolidation's own record, short on a strict count.** The consolidation removed fathom's 53
non-main branches and 46 linked worktrees. `worktrees.tsv` records how: 39 worktrees removed "clean (HEAD in
bundle)", 7 only after their dirty diff was saved as a patch.

| Set | Read landed |
|---|---|
| branch lines removed with nothing saved (no worktree, or a clean one) | 46 of 46 (1.000) |
| those plus the 4 detached worktrees removed clean | 46 of 50 (0.920) |
| every removed branch line, the 7 with saved dirty diffs included | 46 of 53 (0.868) |
| every removed branch line and detached worktree | 46 of 57 (0.807) |

The 7 misses are worktrees whose branch commits are all on `main` but whose uncommitted edits are not (the findings
tests in three packages, `internal/bindspec/binding.go`, `internal/guardspec/guard.go`, and two bench and compile
tests);
the consolidation itself saved those diffs before removing them, so it handled them as at risk, which is what the
report says. The 4 detached worktrees are 2026-08-08 and 2026-08-12 workflow-worker heads with 1 or 2 commits on
no remote and none of their files equal on `main`; one claims `ambient-fathom.md#SH-2`, still open. The two
stashes were dropped as landed (their messages say so) but read touches rows: `main` moved on the same files
after they landed, so neither patch-id nor content equality can see it.

**Bar 3: FAIL.** 36 lines flagged in the two repositories (13 in fathom, 23 in agentic-kit), 40 with the three
held clones, against at most 25.

- 18 of the kit's 23 are touches rows, and every one names `handoff-h7-patterns-bench.md#md-001` (symbol `New`,
  package `foundations/agent/mcp-client`); 9 also name `decision-port.md#DP-24` (oracle path `docs/compat.md`,
  a hot file). These branches (base 3db571b, 2026-09-02) carry 4 to 22 commits of which a third to a half are
  patch-equivalent on `main`, and many of their files already read the same there (`feat/benchmark-evidence`
  184 of 208, `evidence-producer` 232 of 252, `worktree-prompt-screen` 197 of 220, `worktree-gw-mcp` 23 of 45):
  work that landed and was then edited on `main`. Of the kit's 38 branches, 31 are gone today and 7 remain (a
  read-only listing of the owner's ref files, no git run there); all 7 that remain are among the 23 flagged.
- In fathom, 7 of the 13 are the dirty agent worktrees, 4 the detached workflow heads, 2 the landed stashes.

## What the reading kills and what it does not

- Killed (vision 41 §7): the rows classes, claims rows and touches rows. SV-3 (the SessionStart count of
  unlanded lines that claim or touch open rows) is marked refused in the same change. Removing the two classes
  from `fathom plans lines` and from SV-1's row text is the remaining part of ADR-060's revisit trigger ("SV-M1
  reads CUT: remove what the reading killed"); it changes a shipped `--json` shape, so it is recorded as an open
  observation (O293) for the owner rather than done here.
- Kept: landed (46 of 46 on the clean record, and it finds patch-equivalent lines `git branch --merged` misses),
  and at risk (it caught every held item and every saved dirty diff).
- Not measured: the 28 scratch clones of 2026-09-29 (lines across clones are deferred, SV-D3), and lines that
  landed and were then edited on `main`. A per-commit "already applied" test (`git apply --reverse --check` of
  each unmatched commit against the tip) is the candidate; whether it would read the 18 kit lines and both
  stashes as landed is unmeasured, and only a new reading could show it.
