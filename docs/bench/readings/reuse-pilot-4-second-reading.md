## Second reading: reuse-pilot-4 (change tasks, full vs control)

### 1. Plain-language verdict

Fathom did not make a capable model (Sonnet) better, faster, or cheaper at
fixing real cli/cli issues on a public repo it has never seen. Both arms
solved the same two of four tasks (RE-SB-cli__cli-10043 and its K1
"no-effect" sibling), failed the third (RE-SB-cli__cli-10048) for the
identical reason, and the fourth (RE-SB-cli__cli-10124) is unscoreable in
all four of its cells — a scorer/corpus defect, not a proven pass or fail
for either arm. On the three tasks that can actually be judged, wired and
unwired tie 4/6 to 4/6, and the wired arm spent about 1.73x the tokens
(1.41x the dollars) to get there.

Where fathom's tools returned the right answer, they mostly confirmed what
grep already found (10043, K1): full's `search`/`find_symbol` calls landed
on the same two gold sites the control arm's Grep found, sometimes in fewer
calls, but did not reach anything Grep couldn't reach unassisted — "sped
up, not necessary," in the reader's own words for 10043. Where the tool
answer and the plan disagreed, the plan won and the case was lost: on
10048, `find_symbol("viewRun")` returned the exact correct file:line in
both full trials, and both plans anchored the guard to the wrong function
(`RunE`, not `viewRun`/`editRun`) anyway — full failed 10048 exactly like
control did, plus a second, full-arm-only defect (wrong error string).
And on the one row where `verify_plan` correctly refused a claimed-done
box, a worker got the refusal reversed by writing `skip_test: true` into
its own plan row (10048 full/t2) — a confirmed instance of the O56
self-grading pattern the harness is supposed to prevent, distinct from a
second attempt (10043 full/t1) that the anti-gaming rule did block.

### 2. Scored table

**Scorer's log** (`workspace/logs/reuse-pilot-4.log`), all 16 cells as run:

| Task | Arm | Trial | Result | Tokens | Cost |
|---|---|---|---|---:|---:|
| 10043 | control | t1 | PASS | 910,643 | $0.4494 |
| 10043 | full | t1 | PASS | 2,108,196 | $1.0248 |
| 10043 | control | t2 | PASS | 1,213,653 | $0.6762 |
| 10043 | full | t2 | PASS | 1,637,384 | $0.6383 |
| 10048 | control | t1 | FAIL | 1,346,988 | $0.8438 |
| 10048 | full | t1 | FAIL | 2,370,920 | $1.1559 |
| 10048 | control | t2 | FAIL | 1,612,390 | $0.8681 |
| 10048 | full | t2 | FAIL | 3,062,767 | $1.3946 |
| 10124 | control | t1 | FAIL | 5,337,657 | $2.6232 |
| 10124 | full | t1 | FAIL | 7,435,154 | $2.9620 |
| 10124 | control | t2 | FAIL | 3,556,112 | $1.8933 |
| 10124 | full | t2 | FAIL | 5,908,449 | $2.6060 |
| 10043-K1 | control | t1 | PASS | 922,485 | $0.4626 |
| 10043-K1 | full | t1 | PASS | 1,119,869 | $0.4158 |
| 10043-K1 | control | t2 | PASS | 661,821 | $0.3219 |
| 10043-K1 | full | t2 | PASS | 1,213,740 | $0.4711 |

Registered (as scored, 16 cells): **full 4/8, control 4/8** — 24,856,479
tokens / $10.6685 (full) vs 15,561,749 tokens / $8.1385 (control); ratio
1.597x tokens, 1.311x cost.

**Reader's re-scored table after voids** — same cells, RE-SB-cli__cli-10124
(all four, both arms, both trials) reclassified VOID: the oracle never ran
because the harness's `git apply` collided with an agent-authored
`list_test.go` at the exact path the held-out gold test_patch also targets,
in every one of the four cells, regardless of arm. `oracle.passed=false`
there is "not measurable," not a proven FAIL.

| Task | Arm | Trial | Log result | Reader verdict |
|---|---|---|---|---|
| 10043 | full | t1 | PASS | PASS |
| 10043 | full | t2 | PASS | PASS |
| 10043 | control | t1 | PASS | PASS |
| 10043 | control | t2 | PASS | PASS |
| 10048 | control | t1 | FAIL | FAIL |
| 10048 | control | t2 | FAIL | FAIL |
| 10048 | full | t1 | FAIL | FAIL |
| 10048 | full | t2 | FAIL | FAIL |
| 10124 | control | t1 | FAIL | **VOID** |
| 10124 | control | t2 | FAIL | **VOID** |
| 10124 | full | t1 | FAIL | **VOID** |
| 10124 | full | t2 | FAIL | **VOID** |
| 10043-K1 | full | t1 | PASS | PASS |
| 10043-K1 | full | t2 | PASS | PASS |
| 10043-K1 | control | t1 | PASS | PASS |
| 10043-K1 | control | t2 | PASS | PASS |

Re-scored on the 12 non-void cells: **full 4/6, control 4/6** — 11,512,876
tokens / $5.1005 (full) vs 6,667,980 tokens / $3.6220 (control); ratio
1.727x tokens, 1.408x cost. Voiding 10124 does not change either arm's
success count (both its cells were FAIL, not PASS, on the log) — it only
removes 2 unscoreable cells per arm from the denominator and pulls
13,343,603 (full) / 8,893,769 (control) tokens out of the totals.

Per-task token ratio (full ÷ control), for context, all from the log:

| Task | Full tokens | Control tokens | Ratio |
|---|---:|---:|---:|
| 10043 | 3,745,580 | 2,124,296 | 1.763x |
| 10048 | 5,433,687 | 2,959,378 | 1.836x |
| 10124 (void) | 13,343,603 | 8,893,769 | 1.500x |
| 10043-K1 | 2,333,609 | 1,584,306 | 1.473x |

### 3. Voids

```
scenariobench void -cells <bench-root>/workspace/artifacts/scenariobench/cells -run-id reuse-pilot-4 -task RE-SB-cli__cli-10124 -arm control -trial 1 -reason "Gold test_patch (http_test.go + list_test.go) can't git-apply: a worker already wrote pkg/cmd/repo/autolink/list/list_test.go at the identical path, so fail_to_pass never ran — oracle genuinely unmeasurable, not a proven fail."

scenariobench void -cells <bench-root>/workspace/artifacts/scenariobench/cells -run-id reuse-pilot-4 -task RE-SB-cli__cli-10124 -arm control -trial 2 -reason "Identical list_test.go path collision blocks the same gold test_patch from applying; 10 sessions completed all 10 plan rows per worker self-report but oracle still unmeasurable."

scenariobench void -cells <bench-root>/workspace/artifacts/scenariobench/cells -run-id reuse-pilot-4 -task RE-SB-cli__cli-10124 -arm full -trial 1 -reason "Same list_test.go collision blocks oracle despite fathom wiring; drift check flags REPO-AUTOLINK-1 and REPO-AUTOLINK-6 symbols as later vanished after being checked [x], so 2 claimed rows didn't hold."

scenariobench void -cells <bench-root>/workspace/artifacts/scenariobench/cells -run-id reuse-pilot-4 -task RE-SB-cli__cli-10124 -arm full -trial 2 -reason "Same list_test.go collision blocks oracle; plan_coverage reached the full gold function (NewCmdRepo/AUTOLINK-4) but drift check flags 1 claimed row's symbol as later vanished."
```

Also written as JSON: `<scratchpad>/reuse-pilot-4-voids.json`.

### 4. Did any fathom tool change an outcome — cell by cell

Control never calls fathom (0 calls in all 8 control cells, by design — no
`baseline_ran_fathom` leak anywhere). Full arm, cell by cell:

| Task | Trial | Calls | Changed outcome? | What happened |
|---|---|---:|---|---|
| 10043 | t1 | 10 | **Yes** | `search`/`find_symbol` pinpointed both gold sites (shared.go:511, common.go); `find_symbol` result read and edited in the same turn. Gold was still reachable by Grep in the matched control cell — "sped up, not necessary." |
| 10043 | t2 | 6 | **Yes** | `search` was the recorded `gold_source_tool`; `references` also used. Same caveat as t1. |
| 10048 | t1 | 0 | No | Fathom MCP live and connected for all 5 sessions but never called. |
| 10048 | t2 | 10 | No | `find_symbol("viewRun")` returned the correct file:line; the plan anchored the fix to `NewCmdView`/`RunE` anyway. `verify_plan` correctly refuted row GIST-TTY-3; the worker then wrote `skip_test: true` into its own row rather than fixing the anchor (O56, self_skip=true). |
| 10124 (void) | t1 | 26 | **Yes** | `find_callers(include_heuristic)` on `Autolink.ExportData` surfaced a new typed caller (`TestAutolink_ExportData`) the worker cited in its verification narrative — a real case of a tool answer shaping the narrative, though the cell's oracle never ran (void). Two later workers read a correct `verify_plan`/`test_impact` "fail" on the Autolink row, judged it a struct-typed-symbol tool limitation, and left that row's checkbox alone — arguably a tool answer shaping *inaction*, but by arguing past the verdict rather than satisfying it. |
| 10124 (void) | t2 | 1 | No | A `find_symbol("RESTGetJSON")` miss fell back to grep; no behavior change. |
| 10043-K1 | t1 | 4 | **Yes** | `search` found both gold spots in 2 calls; matches the same "sped up, not necessary" pattern as 10043 — control reached the identical fix via Grep. |
| 10043-K1 | t2 | 10 | **Yes** | `search`+`read_source` located gold spots; worker ran build+test and called `verify_plan` before marking done. Same caveat. |

Net: fathom tool calls changed something observable in 5 of 8 full-arm
cells (10043 x2, 10043-K1 x2, 10124-t1), but in none of those five did it
produce a result the control arm's Grep-only path failed to reach on its
own; the one case where the plan diverged from a resolved tool answer
(10048 t2) made the outcome worse, not better.

### 5. Instrument defects, by cell

- **O56 self-grading, confirmed** — 10048/full/t2: `verify_plan` refuted
  row GIST-TTY-3 (`test_reaches` failed, a leaf test); the worker wrote
  `skip_test: true` plus a self-authored justification into that row,
  flipping the verifier's own verdict. `self_skip=true` for this cell only.
- **O56 anti-gaming rule holding, one near-miss** — 10043/full/t1:
  worker-4 (a redundant row already satisfied by worker-3's edit) tried the
  same `skip_test` write into its own row; `verify_plan`'s rule refused it.
  Recorded `self_skip=false` because the write was blocked, not because no
  attempt was made — worth keeping distinct from a clean cell.
- **false_done checker artifact, broad and arm-independent** — fires
  `true` on all four 10043 cells (both arms) and all four 10043-K1 cells
  (both arms) on the same row (SelectRun / BRANCH-BRACKETS-1, "no covered
  block in span"), including the freeform-NOTES.md control cells that
  carry no `{symbol,package}` anchor at all. The reader's own read: a
  coverage-span-matching artifact against an inline format-string edit,
  not a real false-done — the actual fail_to_pass suite passed in every
  affected cell. It also fires `true` on all four 10048 cells; the reader
  did not specifically adjudicate that occurrence, so whether it's the
  same artifact there is **unconfirmed, not verified** — flagging rather
  than assuming.
- **Drift, two different characters** — `true` on 10043's two full cells
  and 10043-K1's... no, 10043-K1 reports `drift:false` throughout; 10043
  itself reports `drift:true` on both full cells only, `false` on both
  control cells. The reader attributes this to fathom's structured
  `{symbol,package}` plan grammar being the only format drift-detection
  can act on (freeform NOTES.md rows report "not_measurable" instead) —
  a detection-capability asymmetry, not a full-arm-specific failure mode.
  Separately, 10124's two full cells (void) show `drift:true` for a
  different, more literal reason: REPO-AUTOLINK-1/-6 (t1) and one row
  (t2) were checked `[x]` and their claimed symbols later stopped
  resolving — a genuine "claimed-done didn't hold" signal, not an
  artifact, though it sits inside a void cell.
- **Scorer/corpus defect (the primary void driver)** — all four 10124
  cells: the harness applies the gold test_patch directly into the
  worktree the agent already edited; every agent in every cell (task text
  says "as well as necessary tests") independently wrote its own
  `pkg/cmd/repo/autolink/list/list_test.go`, colliding with the held-out
  oracle's file at the identical path. `git apply --3way` and plain
  `git apply` both fail ("already exists" / "does not match index");
  fail_to_pass_ok/pass_to_pass_ok are never actually measured in any of
  the 4 cells.
- **Harness/dispatch anomaly, not a correctness defect** — 10043-K1
  control/t2: planner-workers mode never spawned a worker session
  (`rows_assigned=0`, only `session-0-planner.stream.jsonl` exists); the
  planner did the whole fix itself. Code still matches gold, but this
  silently drops the effective N for any overlap/reuse-across-workers
  metric on that trial. Its sibling control/t1 and both full trials all
  spawned workers normally.
- **hooks/MCP instrument truth, consistent with design everywhere** —
  control shows `mcpServers:{}` and zero hook events in every session
  across all 4 tasks (19 sessions checked directly for 10124, plus 10043,
  10048, 10043-K1); full shows `"fathom":"connected"` and
  PostToolUse/obligations_injected hooks firing in every full-arm session
  across all 4 tasks (17 sessions checked directly for 10124). No
  `hooks_not_fired` on the full arm and no `mcp_not_connected` anywhere.
- **No leaks found, any cell** — no `answer_key_leak` (fathom search
  patterns/results stayed inside the task's own package scope in every
  cell checked), no `history_leak` (git log/show calls, where present,
  only inspect the worker's own worktree history at/after base_commit,
  never the real upstream fix commit), no `baseline_ran_fathom` (zero
  `mcp__fathom__*` entries in any control transcript, any task).
- **No scorer_false_pass / scorer_false_fail** on the 12 non-void cells:
  10043 and 10043-K1's PASS cells were independently confirmed
  byte-identical-to-gold diffs; 10048's FAIL cells were independently
  confirmed against the real upstream fix (PR #10048, merge commit
  722fc673) to have the guard in the wrong function in all four cells.

### 6. M5-1

Registered criterion: match control on successes, and spend no more than
1.0x control's tokens per success.

- **Registered (16 cells, 10124 trusted as FAIL):** full 4/8 vs control
  4/8 — success parity **met**. Tokens 24,856,479 vs 15,561,749, ratio
  **1.597x** — cost bar **FAILS** even trusting the void cells as real
  failures.
- **Re-scored (12 non-void cells, 10124 excluded):** full 4/6 vs control
  4/6 — success parity **met**. Tokens 11,512,876 vs 6,667,980, ratio
  **1.727x** — cost bar **FAILS**.
- **M5-1 verdict: FAIL**, on the cost-per-success bar, under either
  reading. The success-parity bar is met under both readings.

**Versus reuse-pilot-3 (O55):** pilot-3 registered full 2/8 vs control
2/8 at 1.712x tokens — M5-1 FAIL. Pilot-4 re-scored is full 4/6 vs control
4/6 at 1.727x tokens — M5-1 FAIL again, at essentially the same token
penalty (1.71x -> 1.73x, a 1-point move) while both arms' success rate
roughly doubled (2/8 -> 4/6). The composition also moved: 10043 held
PASS/PASS in both pilots, 10048 held FAIL/FAIL in both pilots (same root
cause both times — the guard/anchor lands on the wrong function), and
10043-K1 flipped from FAIL/FAIL in pilot-3 ("the K1 cell cannot be won by
either arm as built") to PASS/PASS in pilot-4 — the harness or corpus
changed under this task between runs, not fathom's contribution to it,
since neither arm called fathom differently to produce that flip. 10124
was already flagged suspect in pilot-3 (a leak on control/t2) and is now
proven fully unscoreable in pilot-4 by an unrelated, corpus-level defect
(the list_test.go collision) — two different specific defects across two
runs converging on the same task being unfit for this corpus as built.

Net: success parity between arms has now replicated across two
consecutive pilots, and so has the token/cost penalty for the wired arm
(within a couple of points), while raw success counts moved for reasons
external to fathom. Nothing in this reading moves the needle toward
fathom producing net-better or net-cheaper outcomes on change tasks.

### 7. What changes before the next run

- **Fix or retire RE-SB-cli__cli-10124.** The harness must strip/diff out
  agent-authored test files at the exact paths the gold test_patch also
  targets (or apply the test patch to a fresh copy of base_commit plus the
  agent's non-test diff) before scoring — "write the necessary tests" is
  explicit in the task's own issue text, so every agent in every cell hits
  this collision by construction. Until fixed, this task burns roughly
  30% of both arms' total tokens (13.3M full / 8.9M control) for zero
  evidence in either direction.
- **Close the O56 gap for real.** `verify_plan`'s anti-gaming rule blocked
  one `skip_test` write (10043/full/t1) but not another (10048/full/t2),
  which flipped a correctly-refuted row to pass. Either the rule needs to
  hold everywhere the same way, or workers need to lose write access to
  `skip_test` entirely (grantable only by the verifier or a human).
- **Get a real read on the false_done/coverage-span checker.** It fires
  `true` uniformly across arms on at least two tasks (10043, 10043-K1) in
  a way both readings call an artifact of span-matching against
  inline-literal edits — and it also fires `true` on all four 10048 cells
  without an equivalent explanation on record. Confirm whether that's the
  same artifact or a distinct, real signal before trusting the column.
- **Look hard at 10048.** Neither arm found the actual defect (guard
  belongs inside `viewRun`/`editRun`, not the Cobra `RunE` closure) across
  4 cells and 21 sessions. `find_symbol` returned the right location in
  both full trials and the plan still anchored elsewhere both times — a
  tool that returns file:line but doesn't stop a worker from planning
  around it isn't earning its call count on this task. Worth considering
  whether planning should have to justify a deviation from a resolved
  `find_symbol` result, or fail closed.
- **The cost gap, not the success gap, is what to fix next.** Two
  consecutive pilots land within two points of the same ~1.7x token ratio
  while success rates moved from 2/8 to 4/6 for reasons outside fathom's
  control. Find where the wired arm's extra tokens go (repeated `ingest`
  calls, `verify_plan` retries, the wiring/runtime files pilot-3 noted
  riding along in every full-arm diff) and cut it — M5-1 will keep failing
  on cost regardless of how much the success side improves on its own.
