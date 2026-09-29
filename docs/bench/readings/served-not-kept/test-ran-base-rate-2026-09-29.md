# A `test_ran` check would read kept about nine times in ten on code scopes (2026-09-29)

**Verdict (plain language).** A check that asks "did a pinned test run and pass after the last in-scope edit?"
has almost no headroom on code. In 404 pairs of an agent transcript and a governing guard, taken from the last
16 days of this repository's own agent work, the answer was already yes for **93.6 percent of code-scope
pairs on a lenient reading and 85.2 percent on a strict one**. So a per-edit or turn-end `test_ran` promise would
mostly confirm what already happened. It also has a hole a transcript scan cannot see: after a test is
renamed, `go test -run <old name>` finds no test, prints `[no tests to run]` and exits `ok`, which reads as
kept. This reading is the reason `test_ran` is deferred (PR-D1 in `docs/plans/served-not-kept.md`, ADR-057),
not refused: if a served-population reading ever falls under 70 percent on code scopes and the commit pin line
does not already catch what `test_ran` would, it can be revived under kill criterion K8 (one false kept on a
renamed or deleted pin turns it off again).

This is a reading over transcripts, not an experiment. It measures a base rate; it does not measure whether a
pin line changes behavior (that is PR-M2).

## Method

**Unit.** One agent transcript: a top-level session or one sub-agent or worker run, each on its own. Transcripts
modified in the last 16 days before the reading (2026-09-28) were scanned; 233 distinct transcripts produced
at least one pair.

**Guards.** The 15 "done needs" guards of this repository: for each, a set of scope globs and the named tests
that pin it, with the package each test lives in. They are the rows of the sensitive-surfaces table in
`docs/intent/README.md` (store, verify, extractor and fusion, MCP, semantic, hooks, observatory, plans, gates and
scripts, the kit pin, the custodian, metrics, the agent-config files, the read-coverage adapter and the intent
corpus). Eleven of them had at least one pair in the window.

**Pair.** A (transcript, guard) pair exists when the transcript made at least one Edit, Write or MultiEdit
inside the guard's scope. The **last** in-scope edit is the anchor.

**Kept, lenient.** A Bash command after the last in-scope edit, that did not error, runs a `go test` whose
package pattern covers the package of at least one of the guard's named tests and whose `-run` expression (if any)
admits that test's name, or runs `task validate`, `task test`, `task verify` or `task gate`. **Kept, strict:**
the same for every named test of the guard. Both are upper bounds: they see the command line and its error flag,
never its output, so `[no tests to run]` and a cached result read as passing.

**Excluded by construction.** Whether the run was after the last edit within the same turn or a later one is not
distinguished; a test run in another transcript (the parent's) is not credited to a sub-agent.

## Results (404 pairs, 233 transcripts)

| Scope group | Pairs | Lenient kept | Strict kept |
|---|---|---|---|
| `internal/mcp/**` | 78 | 75 | 60 |
| `internal/extractor/**`, `internal/fusion/**` | 57 | 47 | 47 |
| `internal/semantic/**`, `schema/semantic/**` | 33 | 32 | 32 |
| `internal/store/**` | 26 | 26 | 21 |
| hook files (`cmd/fathom/*hook*.go`, `brief.go`, `sessionhook.go`) | 24 | 23 | 23 |
| `internal/verify/**`, `cmd/fathom/verify.go` | 18 | 17 | 17 |
| `.fathom/*.cue`, `internal/custodian/**` | 6 | 6 | 6 |
| `cmd/fathom/observatory*.go`, `cmd/fathom/ui/**` | 5 | 5 | 5 |
| `internal/metrics/**`, `cmd/fathom/research_report.go` | 3 | 3 | 2 |
| **Code scopes, total** | **250** | **234 (93.6 percent)** | **213 (85.2 percent)** |
| `docs/plans/*.md` | 127 | 81 | 79 |
| `docs/intent/**`, `ADR.md`, `.fathom/guard.cue` | 27 | 20 | 20 |
| **Doc scopes, total** | **154** | **101 (65.6 percent)** | **99 (64.3 percent)** |
| **All** | **404** | **335 (82.9 percent)** | **312 (77.2 percent)** |

Four pairs had no `go test` or `task validate` in the whole transcript.

## What the numbers mean

1. **Code scopes have no headroom.** A `test_ran` promise would record kept in about 85 to 94 percent of pairs
   that were going to be kept anyway, and pre-push `task validate` already runs every test with `-race`.
2. **Doc scopes read lower, and the reading is misleading.** The doc-scope `done needs` tests (the plans and
   intent guards) run on `t.TempDir()` fixtures, so a doc edit cannot change their result: "kept" or "not kept" for
   them says nothing about whether the edited document is right.
3. **The false-kept hole is the i28 shape.** In guards-pilot 8, task i28, trial 2, the agent renamed the test that
   pinned the rule; `go test -run` on the old name matched nothing and reported `ok`. A `test_ran` check would
   have recorded that trial as kept. If it is ever revived it must require a `--- PASS: Name` line (or a JSON pass
   event) for the name, never a package `ok` (K8).
4. **The pin line does not have this problem.** It reads the diff, not the run: a renamed pinned test is a lost
   pin whether or not any test ran afterwards (ADR-057, PR-1 to PR-3).

## Limits (what this does not claim)

- Not causal: it says nothing about whether a check would change what an agent does.
- Lenient means an upper bound; strict is also an upper bound, since neither reads output.
- The window is 16 days of one repository's own agent traffic, mostly the owner's; other repositories and
  other agents may differ.
- The guard set is this repository's; an adopter's `done needs` tests may be run less (or more) often.

## Reproduce

The scan is a small script over the agent transcripts under the operator's home (Claude Code's per-project
JSONL files): it extracts Edit, Write and MultiEdit paths and Bash commands with their error flags, orders them by
time, and evaluates the two rules above against the 15 guards. It was run once, by the critic reviewing the
promises design, and kept outside the repository with its per-pair output (404 rows); the method above is the
whole contract. A re-run belongs to PR-M0's replay in `docs/plans/served-not-kept.md`.
