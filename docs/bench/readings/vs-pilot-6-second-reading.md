# Where fathom stands (vs-pilot-6, second reading)

This is the first run with ADR-037's raw-guide fallback wired into `obligations`,
and with tool-neutral prompts across all five arms. The run's own scoring pass
was also disrupted mid-flight: a `sandbox_escape` over-detector flagged 105 of
140 cells LOST on the first pass (`<scratchpad>/vs6-run2.log`); a rescore-from-raw
pass recovered 91 of those into properly filed cells, leaving 14 genuinely lost
(§6). The numbers below are read against the **filed** ledger — the state in
`<bench-root>/workspace/artifacts/scenariobench/cells` after that rescue, which
is also what `docs/bench/README.md`'s **vs-pilot-6** entry reports.

### 1. Verdict first

With neutral prompts and the raw-guide fallback live, fathom's arm passes 24 of
28 scored cells — ahead of every competitor on raw count (grep 18/26, gopls
18/23, graphify 20/26, serena 22/23) and the only arm with zero lost cells.
That lead survives a second reading essentially unchanged: two scorer bugs on
the fathom arm cancel out (one false fail, one false pass — see below), so
fathom's corrected score is still 24/28. One competitor cell (graphify,
H1-T02) is a genuine false fail and corrects up to 21/26; one grep cell
(H1-T04) is voided for cross-cell contamination, correcting grep to 18/25.
Nobody else's scored-cell verdicts move. **On the filed and corrected scored
tables, fathom leads every arm, by a clearer margin than vs-pilot-5 showed
(where the corrected read was a tie with grep and serena).** That lead does
not survive restoring the 14 cells this run's own pipeline lost, though:
serena alone lost 5 correct answers to the rescore-write gap (§6), and
adding them back puts serena at 27/28 against fathom's 24/28 — see the
falsifier in §8. Read this run's headline as "fathom leads on the ledger as
filed," not as a claim that would survive a clean, complete run.

Cost tells a mixed story. The filed ledger's own paired comparison (full vs.
grep, n=14 bootstrap) reports fathom using 0.813× the tokens but **1.096× the
cost** of grep — fathom wins on token economy but not on dollars, on this
pass. Average tokens per scored cell, computed from every cell's own token
count (including the 14 lost, for the arms that have them): full 103,011,
gopls 112,038, graphify 135,251, grep 136,424, serena 237,022 — fathom is
still the cheapest arm by tokens even though the dollar ratio runs the other
way, which is possible because per-token pricing differs by which tools ran
and how the harness billed them; these inputs don't carry a full per-arm
dollar total to reconcile the two, only the aggregate ratio and 35 of 140
cells' individual `cost=` values (§2). One of grep's own scored passes
(FRESH-M7-01) is a contaminated baseline — that trial built and ran the
fathom binary against itself — so grep's own cost/token profile this run is
not entirely clean either.

The obligations picture is the sharper finding. Across the five "what
governs this" tasks (H1-T01, T02, T08, T12, T13 — 10 fathom-arm trials), the
new raw-guide fallback fired in 8 of 10, up from not existing at all in
vs-pilot-5. But it never once served the rule the oracle wanted: in every
trial where it fired, the raw row was an unrelated decoy (a "Store purity
ADR-020/ADR-025" blurb, or a root CLAUDE.md header). The governing rule was
served correctly only on H1-T01 (2 of 2 trials) — and there it came from the
pre-existing doc-claimed compiled layer, not the raw fallback. On T02, T08
and T12 (6 trials) the governing rule was never served by `obligations` at
any tier; every one of those trials passed only because the model abandoned
obligations and fell back to `search`/Grep/Read. On T13 obligations was
never even invoked (no edit triggered the hook, and the model didn't call it
explicitly). Net: on the specific measure vs-pilot-5 reported as "obligations
served the predicted rule in 0 of 4 trials" (T02 + T08), vs-pilot-6 is
**still 0 of 4** — unchanged by ADR-037. The raw fallback is exercising the
tool far more often; it has not yet fixed the miss.

### 2. Scored table: filed ledger vs. reader re-score

**Filed** = the state of `workspace/artifacts/scenariobench/cells` after the
rescore-from-raw pass, reconstructed cell-by-cell from the reader's own
per-task notes and cross-checked against `docs/bench/README.md`'s **vs-pilot-6**
line (`full vs grep: success 24/28 vs 18/26` — matches exactly) and
`<scratchpad>/vs6-run2.log`'s 35 surviving pre-rescue rows (all 35 agree with
the filed verdict below). **Reader re-score** applies the four verdict-level
corrections from §1/§3 (two false fails flipped to PASS, one false pass
flipped to FAIL, one contamination cell excluded) and leaves every other
verdict as filed; the 14 lost cells stay excluded from both denominators
(§6 lists them separately — all 14 would be PASS if scored).

| Arm | Filed pass/total | Reader re-score pass/total | Lost cells (excluded from both) |
|---|---|---|---|
| full (fathom) | 24/28 | **24/28** (net unchanged — one false fail up, one false pass down) | 0 |
| grep | 18/26 | **18/25** (one contamination cell voided) | 2 |
| serena | 22/23 | **22/23** | 5 |
| graphify | 20/26 | **21/26** (one false fail corrected) | 2 |
| gopls | 18/23 | **18/23** | 5 |

Totals: 126 cells scored in the filed ledger, 14 lost — 140 planned (14 tasks
× 5 arms × 2 trials). No cell anywhere in this run was found to be a
scorer_false_pass except the one full-arm cell named above; vs-pilot-5 found
zero false passes in its whole run, so this is a new defect class for the
scorer to fix, not a repeat.

Average tokens per scored cell (filed-ledger denominator, all 140 cells'
token counts including the lost ones for the arms that have them):

| Arm | Total tokens (scored cells) | Avg tokens/cell | Cells with a filed `cost=` |
|---|---|---|---|
| full | 2,884,295 | 103,011 | 35 (pre-rescue log only) |
| gopls | 2,576,875 | 112,038 | of 140 total cells, only these 35 (§below) carry a `cost=` value in these inputs |
| graphify | 3,516,527 | 135,251 | — |
| grep | 3,547,033 | 136,424 | — |
| serena | 5,451,502 | 237,022 | — |

Dollar cost is only known precisely for the 35 cells that survived the
original (pre-sandbox_escape-bug) pass in `<scratchpad>/vs6-run2.log`; the 91
cells recovered by the rescore-from-raw pass do not carry a refiled `cost=`
in these inputs, and README's vs-pilot-6 entry gives only the aggregate ratio
(tokens 0.813×, cost 1.096×, full vs. grep, paired bootstrap n=14) — not a
per-arm dollar total. A precise $/pass table like vs-pilot-5's is **not
reconstructable from what's available here**; this is worth flagging as
something the next filed ledger should carry forward (a per-cell `cost=`
survives the rescore, not just `tokens=`).

### 3. Voids

Four cells need a verdict correction and one needs a cost/instrument-truth
exclusion; each below is a single-trial void using `-trial` (this run's void
tool now accepts it — vs-pilot-5 had to encode the trial number in free-text
`-reason` because it didn't).

```
CELLS=<bench-root>/workspace/artifacts/scenariobench/cells

scenariobench void -cells "$CELLS" -run-id vs-pilot-6 -task H1-T01 -arm full -trial 1 -reason "scorer_false_fail: rescore-from-raw scored this FAIL, but final_answer literally contains both 'fusion.New' (multiple times) and 'golden-test' (inside the verbatim blockquote) byte-for-byte; sibling t2 (not rescored) shows this task shape scores correctly, so the bug is specific to this cell's pass through the rescore path"

scenariobench void -cells "$CELLS" -run-id vs-pilot-6 -task H1-T02 -arm graphify -trial 1 -reason "scorer_false_fail: answer correctly cites ADR-021 sec.3 and the Edge type's Kind EdgeKind field but phrases the type as 'the \`Edge\` type' rather than the literal 'core.Edge'/'Edge struct' the answer_any_of group requires -- a third phrasing variant not covered even after O53's earlier fix for the same defect class"

scenariobench void -cells "$CELLS" -run-id vs-pilot-6 -task H1-T05 -arm full -trial 1 -reason "scorer_false_pass: final answer names 'run' as the two-hop caller of Summarize via RenderMarkdown, which is wrong (run is three hops out and never calls RenderMarkdown directly); the correct name 'cmdReport' appears only in an intermediate reasoning aside from the model's own first find_callers result, not in the stated answer -- the lexical answer_all_of scorer matched on the aside instead of the asserted conclusion"

scenariobench void -cells "$CELLS" -run-id vs-pilot-6 -task H1-T04 -arm grep -trial 1 -reason "instrument-caused cross-cell contamination: Bash grep on shared/reused worktree slot 80fe955fc016 (reused by 25+ cells this run) returned 'func ln(...)' instead of 'func cmdVerify(...)' -- not the pinned base_commit's real content, confirmed by grep-t2 on the identical path showing normal content and 20+ reflog resets from concurrent reuse; not a model error, exclude from both pass and fail counts"

scenariobench void -cells "$CELLS" -run-id vs-pilot-6 -task FRESH-M7-01 -arm grep -trial 2 -reason "baseline_ran_fathom: this grep-baseline session built the fathom binary (go build -o /tmp/fathom-local ./cmd/fathom) and used it to fathom ingest + fathom verify its own plan file mid-session -- a contaminated baseline even though the final answer was already correct and unaffected; its cost/turn/token readings (47 turns, 791,735 tokens) are not comparable to the other arms' readings"
```

Five commands covering the five cells that need a verdict or accounting
correction. The 14 genuinely lost cells need no void command — there is no
filed cell to void — and are listed separately for completeness.

Full machine-readable list (5 voids + 14 lost, each `{task, arm, trial,
reason}`): `<scratchpad>/vs-pilot-6-voids.json`

### 4. Gold-source tools per arm

Counted from each cell's own `gold_source_tool` field, corrected to the
reader's independently-verified value where the reader explicitly flagged a
misattribution (140 cells total, lost cells included since a raw session and
a source tool exist for those too).

| Arm | Own dedicated MCP tool was gold source | Read | built-in Grep | Bash | none |
|---|---|---|---|---|---|
| full | **10/28** (obligations 1, verify_plan 1, find_symbol 1, find_callers 4, references 1, search 2) | 15 | 2 | 0 | 1 |
| gopls | **1/28** (go_search, H1-T04) | 9 | 12 | 4 | 2 |
| serena | **3/28** (find_symbol ×2, find_referencing_symbols ×1) | 12 | 8 | 4 | 1 |
| graphify | **0/28** — no graphify MCP tool is the gold source in any cell, scored or lost | 13 | 10 | 4 | 1 |
| grep | n/a — no MCP by design | 9 | 12 | 4 | 3 |

Same pattern as vs-pilot-5: gopls's own tool decided 1 of 28 answers, serena's
3, graphify's **none at all** — for those three arms this pilot again mostly
measures "Sonnet with grep, Bash and an idle MCP connection," not the vendor
tool it names. fathom's own tools decided 10 of 28 (down from vs-pilot-5's
13/28 in absolute terms); Read still supplied the single largest share of
fathom's own results, 15 of 28 across all filed cells and 13 of the 24 cells
that scored PASS specifically. That 13 does not match README's own "gold
source: Read earned 11 of 24 passes" line for this run — the gap is not
explained by anything in these inputs (it isn't the denominator, both are
24; the two gold_source_tool corrections applied in §3/§4 move cells between
"none" and a named fathom tool, not into or out of Read) and is worth
reconciling against however README's own figure was computed.

A second, recurring instrument problem sits underneath this table: in at
least 10 of the 140 cells the recorded `gold_source_tool` disagrees with what
the reader found by reading the raw session in call order — most commonly
crediting `Grep` when a plain `Read` (at a fixed offset the model already
knew to check) surfaced the answer first. All six of H1-T05's non-fathom
scored cells carry this pattern. It looks like a "first tool call whose
output happens to contain the answer string" heuristic rather than "first
tool call an attentive reader would credit," and it does not change any
verdict — only the tool-attribution numbers above, which is exactly the kind
of thing that quietly overstates or understates a competitor's tool usage if
nobody reads the raw session.

### 5. Obligations on the what-governs tasks

H1-T01, H1-T02, H1-T08, H1-T12 and H1-T13 are the five tasks built around
"what governs this file/repo" — the surface fathom's own CLAUDE.md names for
that question. This is the first run where `obligations`'s ADR-037 raw-guide
fallback exists at all, so there is no prior-run raw-tier baseline to compare
against; vs-pilot-5's comparable number is "served the predicted rule in 0 of
4 trials" (T02 + T08 only, since T01/T12/T13 either didn't exist yet in that
form or weren't part of that count).

| Task | Trial | raw row served | governing rule served | where it actually came from |
|---|---|---|---|---|
| H1-T01 | t1 | yes | **yes** | doc-claimed compiled layer, not the raw row — the raw row itself was an unrelated "Store purity" decoy |
| H1-T01 | t2 | yes | **yes** | same — doc-claimed layer |
| H1-T02 | t1 | yes | no | never served at any tier; model fell back to `search` + Read |
| H1-T02 | t2 | yes | no | same |
| H1-T08 | t1 | yes | no | never served; model fell back to Read |
| H1-T08 | t2 | yes | no | never served; model fell back to `search` |
| H1-T12 | t1 | yes | no | the correct rule (CLAUDE.md#L48) was among "14 more eligible rules not shown (cap)"; model fell back to Read |
| H1-T12 | t2 | yes | no | same |
| H1-T13 | t1 | no | no | obligations never invoked — no edit triggered the hook, and the model never called it explicitly |
| H1-T13 | t2 | no | no | same |

Reading the row totals plainly: the raw fallback fired in 8 of 10 trials
(new mechanism, exercising far more than vs-pilot-5's zero), but in every
one of those 8 the raw row it served was never the rule the oracle wanted —
0 of 10 trials got the correct rule *from the raw row itself*. The governing
rule was served at all in only 2 of 10 trials (H1-T01, both), and both of
those came from the ordinary doc-claimed layer that already existed before
ADR-037. On the exact two tasks vs-pilot-5 measured (T02 + T08, 4 trials),
vs-pilot-6 is **still 0 of 4** — the raw-guide fallback has not moved that
number. Every trial that passed on these five tasks did so by the model
abandoning `obligations` and falling back to `search`, Grep or Read — the
same route every competitor arm takes by default.

### 6. The 14 genuinely lost cells

Distinct from the sandbox_escape over-detector that flagged 105 of 140 cells
on the first pass (§ intro) — that bug was largely fixed by the rescore-from-
raw pass, which recovered 91 of those 105 into properly filed cells (many
carrying `rescored_from_raw: true`). These 14 are what the rescue pass itself
failed to write, despite every one of them having a complete raw session
(`is_error: false`, normal `stop_reason`) that the reader read end-to-end and
found to contain a correct final answer:

| Task | Arm | Trial |
|---|---|---|
| H1-T02 | serena | 2 |
| H1-T03 | graphify | 1 |
| H1-T04 | gopls | 1 |
| H1-T04 | serena | 1 |
| H1-T04 | serena | 2 |
| H1-T05 | gopls | 2 |
| H1-T11 | gopls | 1 |
| H1-T11 | gopls | 2 |
| H1-T11 | grep | 2 |
| H1-T11 | serena | 1 |
| FRESH-M7-01 | gopls | 2 |
| FRESH-M7-01 | graphify | 1 |
| FRESH-M7-01 | grep | 1 |
| FRESH-M7-01 | serena | 1 |

The pattern is irregular by arm and by trial number — gopls loses one trial
on some tasks and the other trial on others, and within a single task
(FRESH-M7-01) the full arm keeps both trials while gopls/graphify/grep/serena
each lose exactly one of their two — which argues for a race or partial-write
bug in the rescore-from-raw batch itself (it wrote all 6 of that task's
survivors at one timestamp, 03:01:54, while the 4 lost raw sessions for the
same task completed earlier, 02:31:58–02:32:42) rather than a systematic
"always drop trial N" rule. None of the 14 show any sign of a genuine model
failure, a leak, or a sandbox escape — this is purely a scoring-pipeline gap,
and it undercounts gopls's and serena's win rates in this run's ledger by 5
cells each (gopls would be 23/28, serena 27/28, if these were scored and
credited at the reader's verdict).

### 7. Instrument defects, with counts

| Defect | Count | Example |
|---|---|---|
| genuinely lost cells (rescore-pipeline write gap) | 14 | see §6 |
| gold_source_tool misattribution (verdict unaffected) | 10 | all 6 of H1-T05's non-fathom scored cells credit Grep when a plain Read (at a known offset) surfaced the answer first |
| scorer_false_fail | 2 | H1-T01/full/t1; H1-T02/graphify/t1 |
| shared-worktree cross-contamination | 1 | H1-T04/grep/t1 — a reused worktree slot showed a renamed function (`ln` for `cmdVerify`) from another cell's edit |
| scorer_false_pass | 1 | H1-T05/full/t1 — new defect class this run; vs-pilot-5 found zero false passes anywhere |
| baseline_ran_fathom | 1 | FRESH-M7-01/grep/t2 — grep-baseline session built and ran the fathom binary against itself |
| fabricated/ungrounded citation (content happened to be accurate) | 1 | H1-T09/gopls/t2 — quotes doc-comment lines its own tool calls never displayed |
| low-severity sandbox path exposure (not exploited further) | 1 | H1-T05/grep/t1 — a `find`/`fd` call surfaced real paths outside its worktree (a globally-installed skill directory); the model didn't pursue it |
| mcp_not_connected | 1 | H1-T04/gopls/t1 — gopls server never connected; model fell back to Bash/Grep and still answered correctly |
| hooks_not_fired | 1 | H1-T06/full/t2 — PostToolUse:Bash didn't fire after the 4th of 6 Bash calls, which had errored |
| answer_key_leak | **0** | none found across all 126 scored + 14 lost cells checked (no session touched `testdata/scenariobench/**`, `testdata/contextbench/**` or `docs/bench/**`) |
| prompt_named_mechanism (task-design note, not an execution defect) | 10 | every H1-T03 cell — the prompt itself names the `--url` flag verbatim to every arm, a task copied from `testdata/contextbench/tasks-home.json` predating the tool-neutral-prompts goal; disclosed in the task's own derivation note |

No cell in this run shows a stale line-number oracle the way seven of
vs-pilot-5's 22 voids did (H1-T07's `ADR.md#L597` vs. `#L590` defect) — that
class of defect appears fixed; H1-T07's oracle line number was independently
re-derived against the pinned `base_commit` and every disagreement on that
task this run is a genuine model miss, not a stale citation.

### 8. What changes before the next run

1. **Fix the rescore-from-raw write path.** It recovered 91 of 105
   falsely-flagged cells but silently dropped 14 more that had complete,
   correct raw sessions — an irregular pattern consistent with a race or
   partial write in the batch itself (§6). This is now the largest single
   source of missing data in the run, larger than the scorer bugs.
2. **Fix or replace the `sandbox_escape` detector.** Flagging 105 of 140
   cells (75%) on the first pass and needing a full rescore-from-raw to
   recover most of them is not a usable signal at that false-positive rate;
   find what specifically trips it (a tool result naming the operator's push
   clone, another wave's clone, or benchroot) and narrow it before the next
   run, rather than relying on rescue-after-the-fact every time.
3. **Isolate or detect reused worktree slots.** H1-T04/grep/t1's shared
   worktree (reused 25+ times, 20+ reflog resets) showed another cell's edit
   mid-trial. One instance found here; a worktree-reuse audit across the
   full 126 scored cells was not performed by this reading and should be,
   since the mechanism that caused this one could affect others silently.
4. **Sandbox the non-fathom arms away from the fathom binary and source.**
   FRESH-M7-01/grep/t2 built and ran fathom against itself mid-trial. Either
   strip the fathom source tree from non-fathom-arm worktrees or block
   `go build`/exec of it, the same fix vs-pilot-5's write-up already
   recommended for baseline contamination generally.
5. **File a per-cell `cost=` on every rescored cell, not just `tokens=`.**
   91 of 126 scored cells carry no dollar figure in what's available here,
   which is why §2 can report token ratios precisely but not a $/pass table.
6. **Investigate the two fathom-arm scorer bugs directly** (H1-T01/full/t1
   false fail, H1-T05/full/t1 false pass) — same lexical answer_all_of
   matcher, opposite failure modes, both on the arm the run is trying to
   measure.
7. **Re-scope the obligations compile budget, or change what the raw
   fallback targets.** It fired on 8 of 10 what-governs trials this run
   (new, up from 0) but never once carried the rule the oracle wanted;
   H1-T12's correct rule was explicitly named as one of "14 more eligible
   rules not shown (cap)." Raising the section budget or re-ranking by
   topical relevance instead of alphabetical order (the same defect
   vs-pilot-5 flagged for the compiled layer) would need to reach the raw
   layer too, not just the compiled one.
8. **Fix `gold_source_tool` attribution.** Wrong in at least 10 of 140 cells,
   concentrated in one task (all 6 non-fathom H1-T05 cells) — looks like a
   keyword-match heuristic crediting the first tool call whose *output*
   contains the answer string rather than the first call an attentive reader
   would credit.
9. **Either rewrite H1-T03's prompt or stop counting it as a differentiator.**
   It still names its own mechanism (`--url`) to every arm, inherited
   verbatim from `testdata/contextbench/tasks-home.json` before the
   tool-neutral-prompts goal existed; all 10 cells converged on the same
   grep-based strategy and the same correct answer this run, exactly as its
   own derivation note predicts.

Falsifier for the headline verdict above: with the 4 verdict-affecting
scorer bugs corrected, the 1 contamination cell excluded, and the 14 lost
cells recovered and scored (all 14 would add a PASS per the reader's
reading, added to both the pass count and the denominator of their arm),
fathom's lead over grep holds but its lead over serena disappears: fathom
24/28, serena 27/28, gopls 23/28, graphify 23/28, grep 20/27. Serena is the
arm this run's own pipeline gap hurt the most — 5 of its 23 scored cells
were dropped, every one a correct answer. If a future clean run (rescore
pipeline fixed, worktrees isolated) does not put serena back ahead of
fathom once its lost cells are restored, that specific correction is what
this pilot's "fathom beats grep and serena on accuracy" claim rests on —
report it as resting on a fixed pipeline, not yet a clean run.

### Addendum (2026-09-17, after wave N19)

The 14 cells §6 called "genuinely lost" were re-read by the driver's own
rescore once the sandbox-escape scan was repaired a second time (O74: a
sentence period glued to a path, terminal colour codes in `ls` output, the
shared subjects pool one level above the worktree, and `/tmp` versus
`/private/tmp`). Ten were recovered; two remain lost as commands genuinely
broader than the pool, one as harness-truncated content, and one
(H1-T04/gopls/t1) as a real `mcp server gopls failed` instrument fault. After
those recoveries and this reading's five voids the filed table is:

| Arm | Scored | Passes |
|---|---|---|
| serena | 27 | 26 |
| full (fathom) | 28 | 24 |
| graphify | 27 | 22 |
| gopls | 26 | 21 |
| grep | 27 | 20 |

fathom's arm ran at 0.76× grep's tokens and 0.995× its cost. The verdict in
§1 stands on the mechanism, not the lead: serena, not fathom, is the most
accurate arm on this corpus once its lost cells are restored, and the
obligations finding in §5 is unchanged.
