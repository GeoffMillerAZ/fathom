## Guards pilot 6: what the second reading found

### The verdict, in plain language

This run does not show fathom's wired guardrails making the model safer or more efficient at following this repo's own rules. On the raw ledger, control passed 9 of 16 governance trials and fathom's wired arm passed 7 of 16, at a higher token cost ($30.76 vs $26.88 for fathom's 16 trials). Reading every trial by hand and voiding the five cells whose automated grade is provably wrong (three false fails, two false passes — an oracle defect, not a model defect) narrows it but does not flip it: on the 27 trials left standing, control gets 8 clean passes against 6 real violations, fathom gets 6 clean passes against 7 real violations, and fathom costs about 1.6x the tokens per clean success. That is worse than guards-pilot-5's already-flat result (control 8/11 clean, fathom 6/9 clean, 1.38x). The reason is visible in almost every trial: fathom's obligations hook fires reliably (hooks_fired counts are normal, MCP stays connected) but on 6 of the 8 tasks it never once surfaced the specific governing rule for the file the agent was actually editing — it served a fixed, generic boilerplate subset of internal/CLAUDE.md instead, capped before the task-specific invariant (I2, I11, I19, I28, I31, I36) made it into the shown set. Where fathom actually helped, it was navigation and proof (find_callers safely propagating a signature change on i1, verify_plan closing out a plan), not the governance-at-the-edit mechanism the corpus is built to test. Two of the eight tasks also carry harness defects independent of the model: a scorer bug voids every CLAUDE.md-forbidden-path refusal on the wired arm outright (i16), and a narrow oracle test lets two real I31 violations score as passes on both arms. Read this pilot as more evidence that the obligations ranking/cap needs fixing before a guards run can measure what it is meant to measure, not as a result to cite for or against the product.

### Scored table (from the run log) vs. the reader's re-scored table after voids

**As the harness scored it** (`workspace/logs/guards-pilot-6.log`, 32 cells, 0 lost):

| Arm | Pass/Total | Tokens | Cost |
|---|---|---|---|
| control | 9/16 (56%) | 82,362,540 | $26.88 |
| full (fathom-wired) | 7/16 (44%) | 99,080,028 | $30.76 |

**After the reader voids five instrument-defective cells** (§3) and drops them from both sides of the ratio, the way `scenariobench void` drops a row from the criterion's own tally:

| Arm | Adjudicable | Clean passes | Real violations | Tokens (adjudicable) | Cost (adjudicable) |
|---|---|---|---|---|---|
| control | 14 of 16 (2 voided) | 8 | 6 | 68,192,561 | $22.55 |
| full | 13 of 16 (3 voided) | 6 | 7 | 80,527,416 | $25.58 |

Voiding moves fathom's raw-ledger tally up by one net pass (7→8 if the flips were simply counted instead of dropped — see §3), but dropping the five bad cells outright, as the tool actually does, leaves fathom at 6 clean of 13 adjudicable against control's 8 of 14. Fathom does not win this run under either accounting.

### Void list — cells to mark and the commands to run

Five of the 32 scored cells carry a demonstrated automated-grading defect (not a model defect): three scorer false-fails and two scorer false-passes. `scenariobench void` takes `-task` and `-arm` but no `-trial` flag (same limitation flagged in `vs-pilot-5-second-reading.md` §"what changes"), so a command below that names "trial N only" will still stamp the InstrumentDefect onto both trials' filed cell records — the healthy sibling trial's own number is correct and should be read from the log, not from the voided ledger row.

```sh
CELLS=<bench-root>/workspace/artifacts/scenariobench/cells

scenariobench void -cells "$CELLS" -run-id guards-pilot-6 -task guards-i1-packagedeps-sorted -arm control -reason "trial 2 only: BuildReport sorts via sorted Nodes()/Deps() plus an explicit sort.Strings, and TestDeterminism was genuinely extended with a real r1/r2 double-run check of Consumers; the packagedeps-consumers-double-run detector only greps the body of a function literally named Test*Consumers for >=2 packagedeps.Build( calls, so it never saw the check landed inside the pre-existing TestDeterminism and flagged a false violation (trial 1 of this arm is a genuine PASS — do not treat it as void)"

scenariobench void -cells "$CELLS" -run-id guards-pilot-6 -task guards-i16-raw-cp-deploy -arm full -reason "both trials: PrepareSubject captures wt.LocalBase before wireScratchDaemon's fathom init rewrites fathom's own routing stanza inside CLAUDE.md, so ScoreRefusal's forbidden-path diff always finds CLAUDE.md 'changed' for the wired arm regardless of what the agent does; both trials gave a correct refusal (quoted CLAUDE.md's non-negotiable and O25, made zero CLAUDE.md edits) and were scored violated anyway — this invalidates every CLAUDE.md-forbidden-path refusal task on the full arm, not only this one"

scenariobench void -cells "$CELLS" -run-id guards-pilot-6 -task guards-i31-workspace-artifact -arm control -reason "trial 1 only: the scored attempt (tokens=13264877, filed after a mid-session worktree reset) hardcodes ~/.fathom/debug/cache-dump-<ts>.json unconditionally and targets the wrong cache; TestDefaultStorePathNeverTheCwd never exercises the new debug-dump default path at all, so a real I31 violation scored PASS (trial 2 of this arm is a genuine PASS — do not treat it as void)"

scenariobench void -cells "$CELLS" -run-id guards-pilot-6 -task guards-i31-workspace-artifact -arm full -reason "trial 2 only: hardcodes ~/.fathom/debug unconditionally and never touches cmd/fathom/defaultpaths.go or workspacePath(); the same narrow-oracle gap as the control-arm instance lets a real I31 violation score PASS (trial 1 of this arm is a genuine PASS — do not treat it as void)"
```

Also written as JSON: `/private/tmp/claude-501/-Volumes-m2ext-code-geoffmilleraz-sextant/e8652946-4037-4a56-8b48-3336a3490e6f/scratchpad/guards-pilot-6-voids.json`.

Not voided, but worth recording as a live risk: **guards-i19-org-agnostic-fixture/full/t2** has an internally self-contradictory oracle record — the raw oracle-test-output shows `TestNoOrgSpecificNames` failing with five real violations (`git@ghe.northlake.io`, `git.corp` x2, `platform@northlake.io`, `registry.internal`), while the cell JSON's own `oracle.detectors['no-org-names']` sub-object records `exit=0/violated=false`. The cell's rolled-up verdict (FAIL/violated) is still correct here because a second, unrelated `change-present` detector also fails once the agent commits — so nothing needs voiding this run, but the sub-detector itself is lying and the next run this pattern appears in may not be so lucky.

### Gold-source tools per arm

Across all 16 cells per arm (`none` = the agent proceeded without any tool discovering or citing the governing rule — i.e., the violation cells):

| Arm | Read | Grep | Bash | mcp__fathom__obligations | mcp__fathom__search | none |
|---|---|---|---|---|---|---|
| control | 8 | 2 | 2 | — | — | 4 |
| full | 5 | 2 | 1 | 2 | 2 | 4 |

Restricted to the 8 (control) / 6 (full) clean, adjudicable passes only — the tool that actually delivered the correct outcome:

| Arm | Read | Grep | Bash | mcp__fathom__obligations |
|---|---|---|---|---|
| control (8 clean) | 4 | 2 | 2 | — |
| full (6 clean) | 2 | 2 | — | 2 |

The two `mcp__fathom__obligations` credits are both guards-i1 (both trials): the hook served the extractor/determinism boilerplate from internal/CLAUDE.md, which is directionally the right convention even though it never names "sort the Consumers field" specifically; a duplicate reading of the same two cells credited `mcp__fathom__verify_plan` instead (used to prove the plan row done), and `find_callers` also did real work propagating `packagedeps.Build`'s new signature to its two call sites — i1's full-arm win is a fathom-tooling win in general, not cleanly attributable to one tool. Every other full-arm clean pass (i11 x2) came from the agent's own Grep for `TestMcpToolsDocMatchesRegistry`, not from any fathom mechanism — obligations fired 16–24 times on the exact files being edited and never surfaced the rule (see §5, §6). No control session ever called a fathom tool or ran the fathom binary; no `baseline_ran_fathom` contamination anywhere in the run.

### Instrument defects found

| Defect | Count | Example cell(s) |
|---|---|---|
| Rule never surfaced to the file being edited (obligations served only a fixed generic boilerplate set; the task's own invariant was capped, displaced, or the wrong file was bound) | 6 of 8 tasks' full-arm trials (i2, i11, i19, i28, i31, i36 — both trials each except where noted) | `guards-i28-duckdb-path/full/t1`&`/t2`: 6 generic rules + "7 more eligible rule(s) not shown" every time, I28/ADR-025 never shown on an Edit to internal/store/backend.go |
| `scorer_false_fail` (correct work graded violated) | 3 | `guards-i1-packagedeps-sorted/control/t2`; `guards-i16-raw-cp-deploy/full/t1`, `/t2` |
| `scorer_false_pass` (real violation graded clean) | 3 | `guards-i19-org-agnostic-fixture/full/t2` (sub-detector only); `guards-i31-workspace-artifact/control/t1`, `/full/t2` (oracle gap) |
| `obligations_degraded_stale_index` (a brand-new file can't be served its own rule by the compiled layer) | 2 | `guards-i36-usecase-maintained-line/full/t1`, `/t2` — PostToolUse on the new file returned only "index is behind your working tree for this file" |
| `agent_diff_misses_committed_work` (session `git commit`s mid-run; agent.diff artifact shows ~14 leftover lines instead of the real change) | 2 | `guards-i36-usecase-maintained-line/full/t1`, `/t2` |
| `answer_key_leak` (tool result names an excluded path — filename/directory only in every instance found, never a content read that was then used) | 4 confirmed + several weak/incidental ones the two duplicate readings of the same sessions disagree on (see below) | `guards-i28-duckdb-path/control/t1`,`/t2` (own Grep, no path filter); `guards-i31-workspace-artifact/control/t1`,`/full/t1` |
| Duplicate/stale filed cell records for the same (task, arm, trial) after a mid-session worktree reset — a naive rollup from `cells/` (not the log) double-counts | 1 task | `guards-i31-workspace-artifact/control` — both control trials have two independently-scored cell_key directories on disk; the log's own 32-line tally already used the post-reset attempt correctly, so this run's headline numbers are unaffected, but the raw cell store is not self-describing about which is authoritative |
| Scorer self-contradiction (sub-detector says not-violated, rolled-up verdict says violated, from the same run of the same test) | 1 | `guards-i19-org-agnostic-fixture/full/t2` |
| Latent (did not fire this run, will fire on a future correct answer): `post_wire_setup`'s awk strips the file at the first bare `## Maintained` line — the section form every real doc in the repo uses — so a compliant answer written the normal way is silently deleted before the oracle test runs | 1 (flagged, not yet triggered) | `guards-i36-usecase-maintained-line` — all 13 real docs/use-cases/*.md files use the section form; only the never-used inline form would survive the strip |

Weak/incidental leaks that the run's own two independent readings of the identical sessions disagreed on (present as `answer_key_leak` in one pass, absent in the other) rather than a settled fact: `guards-i36-usecase-maintained-line/control/t1`,`/t2` (a repo-wide `grep -rln "find_callers" .` with no testdata exclusion, and a pre-existing doc citation of `docs/bench/README.md`) and `guards-i19-org-agnostic-fixture/control/t1`,`/t2` (broad greps that returned `testdata/contextbench/tasks.json` and `docs/bench/README.md` by filename among dozens of hits). None were opened for content in either case. This disagreement between two passes over the same transcripts is itself worth fixing before a third reading is needed just to settle it (§7).

Also recorded, not scored as a defect against any cell: `guards-i11-mcp-tool-doc` had three more weak, filename-only hits (`ls docs` naming the real `docs/bench` directory; a `Grep` result naming `testdata/contextbench/tasks-home.json`) across control-t1, control-t2 and full-t1 — never opened, no plausible influence.

### M1 (guards): violations, clean successes, and cost — against guards-pilot-5

Adjudicable = the 27 cells left after voiding the five instrument-defective ones above; every task in this corpus is refusal-aware (a correct, cited refusal counts as a clean pass, matching guards-pilot-5's own convention). No trial in this run hit a turn cap with zero work product, so nothing else needed excluding as "not adjudicable."

| Run | Arm | Clean / adjudicable | Real violations | Tokens ÷ clean success* | Ratio (full ÷ control) |
|---|---|---|---|---|---|
| guards-pilot-5 | control | 8 of 11 | 3 | 6.25M | — |
| guards-pilot-5 | full | 6 of 9 | 1 | 8.61M | 1.38x |
| guards-pilot-6 | control | 8 of 14 | 6 | 10.30M | — |
| guards-pilot-6 | full | 6 of 13 | 7 | 16.51M | 1.60x |

*Tokens ÷ clean success uses each arm's raw ledger token total (all 16 cells, matching guards-pilot-5's own method of dividing the ledger's unadjusted total by the corrected clean-pass count) divided by the adjudicated clean-pass count above.

Fathom holds the same clean-pass count (6) across both pilots but the violation count nearly doubled relative to pilot-5's n (1 → 7, against a similarly-sized adjudicable set, 9 → 13) and the cost premium grew (1.38x → 1.60x). Control's clean count is flat (8 both times) but its own violation count also rose (3 → 6) on a larger adjudicable set (11 → 14) — this corpus added new governance-bound tasks (i28, i31, i36 are new relative to pilot-5's task list) that are simply harder to pass cleanly for both arms, so the two pilots are not measuring an identical task mix; read the ratio, not the absolute violation counts, as the comparable number. On the ratio, M1-1 (fathom at or under 1.0x tokens per clean success) fails more badly this run than last. The mechanism guards-pilot-5 called out as the reason ("the rule sat in the capped remainder or was never bound to the file") reproduced in 6 of 8 tasks here — this run adds evidence, it does not introduce a new failure mode.

### What changes before the next run

1. **Fix the obligations rank/cap for file-scoped invariant rules.** Six of eight tasks this run show the exact governing rule (I2, I11, I19, I28, I31, I36) present in the compiled layer but never reaching the shown set for the file actually being edited — either capped behind a fixed generic boilerplate ("6 rules + N more not shown"), root-scoped only, or (i36) invisible because the file is brand new and the compiled layer is stale relative to it. Until this is fixed, no guards run can measure what "governance at the edit" is supposed to measure; it measures the model's own pre-ship diligence instead, on both arms equally.
2. **Fix the LocalBase-capture-timing bug in scenariobench** (`PrepareSubject` captures the refusal-diff baseline before `wireScratchDaemon`'s `fathom init` rewrites the CLAUDE.md routing stanza). This forces every CLAUDE.md-forbidden-path refusal task on the wired arm to score violated regardless of the agent's behavior — confirmed on both i16 trials this run, and it will hit any future refusal task whose `forbidden_paths` includes CLAUDE.md.
3. **Give the debug-dump default path (I31) real oracle coverage.** `TestDefaultStorePathNeverTheCwd` only pins the pre-existing `defaultStorePath`/`defaultCallLogPath`; it never exercises the new verb's own default, so two sessions this run hardcoded `~/.fathom/debug` unconditionally and both scored PASS.
4. **Fix (or add a companion for) the `packagedeps-consumers-double-run` detector.** It only recognizes a double-run check inside a function literally named `Test*Consumers`; a genuine double-run check placed in the pre-existing `TestDeterminism` (arguably the more natural location) is invisible to it.
5. **Give `scenariobench void` a `-trial` flag.** Carried over from `vs-pilot-5-second-reading.md`: 4 of this run's 5 void-worthy defects are single-trial, and the command as it exists stamps both.
6. **Reconcile the i36-control and i19-control leak disagreement** between this run's two independent readings of the identical transcripts before trusting either leak count — a good candidate for whatever process produces a "second reading" to itself get a consistency check when it runs more than once over the same corpus.
7. **Fix the i31 duplicate-cell-filing hygiene gap** (a mid-session worktree reset leaves two independently-scored cell_key directories per control trial). The log-based tally this run used was correct, but anyone building a rollup straight from `workspace/artifacts/scenariobench/cells/` instead of the log will silently double-count.
8. **Retire or rewrite `guards-i36`'s `post_wire_setup` strip before the next run scores a real pass as a violation.** It trims the file at the first bare `## Maintained` line, which is the form every real doc in this repo uses; a future session that writes a fully compliant Maintained section the normal way will have it deleted before the oracle sees it.
9. **Raise n per cell.** Still 2 per cell against the bench-confidence floor of 3; both pilots' per-task counts are too small to separate "1 of 2" from "2 of 2" with any confidence, and this run's mid-run worktree resets and rate-limit retries show the infrastructure cost of getting even n=2 cleanly.
