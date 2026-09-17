## Where fathom stands (vs-pilot-5, second reading)

### Verdict first

The raw ledger has fathom in second place: 18 of 28 passes, behind grep's 22
and ahead of serena (20), gopls (17) and graphify (15). That ranking does not
survive a second reading. Twenty-two of the 137 scored cells are false
fails — every one of them a FAIL that the reader confirms is a correct,
substantive answer — and none of the 137 is a false pass. Correcting only
those 22 (denominator unchanged, FAIL flipped to PASS) moves every arm up,
but not by the same amount: **grep and serena end tied at 25 of 28, ahead of
fathom's corrected 24 of 28**, with graphify and gopls trailing at 20 of 26
and 20 of 27. Three more trials never reached the ledger at all — the harness
itself voided them at run time for an answer-key leak, not a reader finding.

fathom still wins on cost: $1.69 total against grep's $2.13 and serena's
$2.62, and about 7 cents per corrected pass against grep's 9 and serena's 10.
But on raw correctness, after the scorer bugs are fixed, this pilot does not
show fathom ahead of grep. It shows a tie on tools, a fathom win on price,
and one clean mechanism win: the verify-gate task, where only fathom's arm
can produce the literal "not found" phrasing the oracle needs, because that
phrase is fathom's own gate output, not something lying around in the repo's
prose. Against that, the obligations tool — the surface fathom's own
CLAUDE.md names for "what governs this file" — never once served the rule
either of the two tasks built around it predicted it would; both passed only
by falling back to search-plus-Read, the same route grep took. Report this
as it is: a tie on correctness, a win on cost, a miss on the tool the product
is supposed to be about.

### 1. Ledger vs. reader, per arm

Counts are pass/total where total = PASS + FAIL entries actually in
`workspace/logs/vs-pilot-5.log` (137 lines total across all 14 tasks: 130 for
the 13 H1-tasks + 7 recorded of a possible 10 for FRESH-M7-01... — see below,
overall 137 run + 3 lost = 140 planned). Cost is the sum of each logged
`cost=` field. "Reader re-score" replaces each of the 22 confirmed
scorer_false_fail cells' FAIL with PASS and leaves every other verdict as
logged; no scorer_false_pass was found anywhere in this run, so no arm's
count moves down.

| Arm | Ledger pass/total | Ledger cost | Reader re-score pass/total | $/pass (ledger) | $/pass (reader) |
|---|---|---|---|---|---|
| full (fathom) | 18/28 | $1.6860 | **24/28** | $0.094 | $0.070 |
| grep | 22/28 | $2.1285 | **25/28** | $0.097 | $0.085 |
| serena | 20/28 | $2.6180 | **25/28** | $0.131 | $0.105 |
| graphify | 15/26 (2 lost) | $2.6118 | **20/26** | $0.174 | $0.131 |
| gopls | 17/27 (1 lost) | $2.2352 | **20/27** | $0.132 | $0.112 |

Totals: 137 cells scored, 3 lost (matches the log's own `137 cell(s) run, 3
lost`). The log's final line reads `pilot vs-pilot-5 exit=1` with no named
criterion — which threshold failed is unknown from these inputs.

### 2. Voids — exact commands to run

`scenariobench void` takes `-task` and `-arm` but has **no `-trial` flag**.
Sixteen of the 22 confirmed voids apply to only one of a task+arm's two
trials (the other trial's PASS or genuine FAIL is correct and must not be
touched); only three task+arm pairs need both trials voided (H1-T07/grep,
H1-T07/serena, H1-T11/graphify). Where only one trial is affected, the
trial number is carried in `-reason` since there is nowhere else to put it —
flagged again in §7 as something to fix before the next run so a blind
re-run of these commands can't accidentally void a correct sibling trial.

```
CELLS=/private/tmp/claude-501/-Volumes-m2ext-code-geoffmilleraz-sextant/e8652946-4037-4a56-8b48-3336a3490e6f/scratchpad/benchroot2/workspace/artifacts/scenariobench/cells

scenariobench void -cells "$CELLS" -run-id vs-pilot-5 -task H1-T01 -arm full -reason "trial 1 only: sole 'golden-test' occurrence sits inside the quoted CLAUDE.md blockquote and is excluded by the oracle's quoted-line filter; answer correctly names fusion.New + connector and quotes the rule in full as asked"

scenariobench void -cells "$CELLS" -run-id vs-pilot-5 -task H1-T01 -arm gopls -reason "trial 2 only: identical blockquote-exclusion pattern as H1-T01/full/t1"

scenariobench void -cells "$CELLS" -run-id vs-pilot-5 -task H1-T02 -arm full -reason "trial 1 only: names 'the \`Edge\` struct' + ADR-021 sec.3 correctly; backtick markup around Edge breaks the literal 'Edge struct' substring match"

scenariobench void -cells "$CELLS" -run-id vs-pilot-5 -task H1-T02 -arm graphify -reason "trial 2 only: same backtick-breaks-substring pattern as H1-T02/full/t1"

scenariobench void -cells "$CELLS" -run-id vs-pilot-5 -task H1-T02 -arm serena -reason "trial 2 only: names core.Edge at internal/core/model.go:371-377 + ADR-021 sec.3 correctly but never writes 'core.Edge'/'Edge struct' verbatim"

scenariobench void -cells "$CELLS" -run-id vs-pilot-5 -task H1-T04 -arm full -reason "trial 1 only: concludingSpan takes only the last paragraph, missing 'DEMO-H1' in paragraph 1 and misreading the bolded 'not found' in paragraph 2 as negated"

scenariobench void -cells "$CELLS" -run-id vs-pilot-5 -task H1-T04 -arm serena -reason "trial 2 only: answer literally states 'Row A (DEMO-H1) ... symbol not found'; bare-negation heuristic misfires on an unrelated earlier 'not' in the same unpunctuated clause"

scenariobench void -cells "$CELLS" -run-id vs-pilot-5 -task H1-T07 -arm full -reason "trial 2 only: mcp__fathom__search hit ADR.md:591; model correctly cited ADR.md#L590 ('Backend selection') for pinned base_commit 1b8765d; oracle's #L597 is stale, written against commit cb4119b two days later after ADR-011 grew the file +7 lines (trial 1 of this arm is a genuine FAIL — do not void)"

scenariobench void -cells "$CELLS" -run-id vs-pilot-5 -task H1-T07 -arm gopls -reason "trial 2 only: same stale-oracle defect as full/t2; Grep hit ADR.md:591, correctly reasoned back to L590 (trial 1 of this arm cites L591 and is a genuine FAIL — do not void)"

scenariobench void -cells "$CELLS" -run-id vs-pilot-5 -task H1-T07 -arm graphify -reason "trial 1 only: same stale-oracle defect, correct ADR.md#L590 citation for the pinned commit (trial 2 of this arm is LOST, not scored — see §5)"

scenariobench void -cells "$CELLS" -run-id vs-pilot-5 -task H1-T07 -arm grep -reason "both trials: same stale-oracle defect (ADR.md#L597 vs. the pinned commit's actual L590) — correct ADR.md#L590 citation in each"

scenariobench void -cells "$CELLS" -run-id vs-pilot-5 -task H1-T07 -arm serena -reason "both trials: same stale-oracle defect, correct ADR.md#L590 citation in each"

scenariobench void -cells "$CELLS" -run-id vs-pilot-5 -task H1-T08 -arm graphify -reason "trial 2 only: final answer reads 'Found it... \`CLAUDE.md#L15\`' — the exact oracle substring appears verbatim — yet was scored FAIL"

scenariobench void -cells "$CELLS" -run-id vs-pilot-5 -task H1-T11 -arm serena -reason "trial 1 only: answer quotes both required substrings ('lock error', 'busy_timeout') verbatim from docs/guides/fleet-index.md"

scenariobench void -cells "$CELLS" -run-id vs-pilot-5 -task H1-T11 -arm grep -reason "trial 1 only: same verbatim-both-substrings pattern as H1-T11/serena/t1"

scenariobench void -cells "$CELLS" -run-id vs-pilot-5 -task H1-T11 -arm gopls -reason "trial 1 only: same verbatim-both-substrings pattern; built-in Grep surfaced the correct passage"

scenariobench void -cells "$CELLS" -run-id vs-pilot-5 -task H1-T11 -arm graphify -reason "both trials: same verbatim-both-substrings pattern in each"

scenariobench void -cells "$CELLS" -run-id vs-pilot-5 -task H1-T13 -arm full -reason "trial 2 only: answer quotes ADR.md line 176 verbatim including 'ADR-010' plus states 'verdict: stale' — identical in substance to H1-T13/serena/t1, which passed on the same task"

scenariobench void -cells "$CELLS" -run-id vs-pilot-5 -task FRESH-M7-01 -arm full -reason "trial 2 only: same mcp__fathom__find_callers evidence and the same correct answer as the passing t1 (ToolKindForTrafficRollup + internal/metrics/toolkind_freshprobe.go)"
```

19 commands covering the 22 voided trials. The 3 auto-lost cells
(H1-T05/gopls/t1, H1-T05/graphify/t1, H1-T07/graphify/t2) need no command —
the harness already excluded them (see §5); they are listed in the JSON
output for completeness, not as commands to run.

Full machine-readable list (25 entries: 22 to void + 3 already-lost, each
`{task, arm, trial, reason}`):
`/private/tmp/claude-501/-Volumes-m2ext-code-geoffmilleraz-sextant/e8652946-4037-4a56-8b48-3336a3490e6f/scratchpad/vs-pilot-5-voids.json`

### 3. Gold-source tools per arm

Counted from each cell's own `gold_source_tool` field (140 cells, LOST
included since a raw session and a source-tool exist for those three too).

| Arm | Own dedicated MCP tool was gold source | Read | built-in Grep | Bash | none |
|---|---|---|---|---|---|
| full | **13/28** (obligations 1, verify_plan 2, find_callers 4, references 1, find_symbol 1, search 4) | 10 | 4 | 1 | 0 |
| gopls | **2/28** (go_search, both on H1-T04) | 7 | 17 | 2 | 0 |
| serena | **3/28** (find_symbol x2, find_referencing_symbols x1) | 9 | 12 | 4 | 0 |
| graphify | **0/28** — no graphify MCP tool is the gold source in any cell, scored or lost | 9 | 16 | 3 | 0 |
| grep | n/a — no MCP by design | 6 | 17 | 4 | 1 |

Read this plainly: gopls's own tool decided 2 of 28 answers, serena's 3,
graphify's **none at all**. For those three arms this pilot mostly measures
"Sonnet with grep and an idle MCP connection," not the vendor tool it names.
fathom's own tools decided under half its answers (13/28); the other 15 were
Read/Grep/Bash, same as every other arm.

### 4. Instrument defects found

| Defect | Count | Example cell |
|---|---|---|
| scorer_false_fail (FAIL scored, reader confirms PASS) | 22 | H1-T07/grep/t1 — correct ADR.md#L590 citation, stale oracle |
| answer_key_leak, cell auto-voided by the harness (LOST, not scored) | 3 | H1-T07/graphify/t2 — log: "a tool result named a path subject.exclude_paths should have removed from the worktree" |
| obligations tool never served the predicted rule | 4 | H1-T02/full/t1 — compiled-rule layer hit `budget_stopped` (14/60 sections), displaced ADR-021 to root/parent tiers; both H1-T02 and both H1-T08 full-arm trials fell back to `mcp__fathom__search` + Read instead |
| hooks_not_fired (PreToolUse silent on fathom MCP calls) | 2 | H1-T06/full/t1 — PreToolUse fired for the Grep call but not for either of the trial's 2 `mcp__fathom__*` calls |
| fathom's own tool handed the right evidence, model reasoned past it (genuine wrong answers, not scorer bugs) | 3 | H1-T05/full/t1+t2 — `find_callers(RenderMarkdown, depth=2)` listed `cmdReport` directly; final answer named `cmdResearch`, 3 hops away, instead. H1-T07/full/t1 — `mcp__fathom__search` hit `ADR.md:591`; answer cited a different document |
| live network reachable from sandbox (no leak of the real repo, but non-hermetic) | 1 | H1-T10/graphify/t2 — `gh search code`/`gh search repos` returned real public GitHub results before the session fell back to local Grep |
| scorer_false_pass | 0 | none found anywhere in this run |

No cell's tool result named `testdata/scenariobench/**` or `docs/bench/**` in
any of the 137 scored cells (only the 3 auto-lost ones leaked a
`testdata/contextbench/**`-adjacent path, and the harness caught those
itself). No baseline arm (gopls/graphify/grep/serena) built or ran the
fathom binary in any cell.

### 5. FRESH-M7-01 — the paid second half of M7 (VB-18)

Ledger: full 1/2 (t2 a confirmed scorer_false_fail), gopls 2/2, graphify
2/2, grep 2/2, serena 2/2. After voiding: **all five arms go 2/2.** Every arm
answered the post-edit question correctly.

That result does not, by itself, move the freshness needle for gopls,
graphify or serena, because none of their four cells that got a correct
answer ever touched the vendor's own indexed/cached surface — every one used
plain, live Grep against the working tree (`graphify` explicitly reasoned
that it "distrusted the graph" per a comment in the file rather than
consulting `graph.json`; the same pattern holds for gopls and serena). Live
grep is immune to index staleness by construction, so this task exercised
that immunity, not any competitor's freshness. The free-half companion
reading (`vb18-freshness-graphify-watch.md`) already tested graphify's own
query surface directly against the same kind of post-edit probe and found
its real answer there (4 of 5 correct, kill clause fired: "freshness leaves
positioning"); this paid run's graphify cells neither confirm nor
contradict that, since graphify's tools were never called.

fathom is the one arm whose own tool was actually exercised here:
`mcp__fathom__find_callers`, called post-setup in both trials, returned all
four real callers including the newly added
`internal/metrics/toolkind_freshprobe.go:7` — correct in both trials once
the scorer bug in t2 is corrected. That is a genuine, positive data point
for fathom's live-index freshness on this one edit; it is not evidence about
any competitor's freshness, because their arms answered around their tools
rather than through them.

### 6. What changes before the next run

1. **Re-derive doc-line-number oracles against the pinned `base_commit` at
   scoring time, not a document state that drifted later.** H1-T07's single
   oracle (`ADR.md#L597`, actually correct only as of commit `cb4119b`, two
   days after the pinned `1b8765d`) alone produced 7 of the 22 voids in this
   run.
2. **Fix the three scorer heuristics behind the other 15 voids**, all
   previously known defect classes: (a) `isQuotedPosition` drops a required
   token that appears only inside a properly quoted blockquote, even when
   the prompt explicitly asked the model to quote the rule (H1-T01); (b) the
   literal-substring match breaks on the model's own markdown backticks
   around a type name (H1-T02); (c) `concludingSpan`/bare-negation on a
   multi-paragraph or unpunctuated answer excludes a plainly-stated
   conclusion (H1-T04, H1-T11, H1-T13).
3. **Fix or re-scope the obligations compile-budget.** It served the
   predicted rule in 0 of 4 trials built to test it (H1-T02, H1-T08); both
   tasks passed only via `mcp__fathom__search` + Read as a fallback. Either
   raise the section budget so ADR-cited rules aren't displaced to root
   tier, or stop writing task predictions that assume it will.
4. **Strip `subject.exclude_paths` from the worktree before the trial
   starts**, not just detect the leak afterward. All 3 lost cells share one
   root cause (a tool result naming an answer-key-adjacent path) and it cost
   3 of 140 planned trials this run.
5. **Give `scenariobench void` a `-trial` flag.** 16 of this run's 22 voids
   apply to only one of two trials sharing a task+arm cell key; the
   supplied command syntax has no way to say which, so every single-trial
   void above had to carry the trial number in free-text `-reason` instead.
6. **Push tool use in the competitor arms, or stop reporting them as
   capability comparisons.** graphify's own MCP tool was the gold source in
   0 of 28 cells, gopls's in 2, serena's in 3 — this pilot mostly measured
   Sonnet-plus-grep three more times, not the vendor tools it named.
7. **Fix the PreToolUse hook gap** — it fired for Grep/Read but not for
   `mcp__fathom__*` calls in H1-T06's full-arm trials, undercounting fathom
   tool usage in anything derived from hook events.
8. **Re-run FRESH-M7-01 with competitor arms directed at their own
   indexed/graph tool** (or with plain Grep restricted) so the freshness
   comparison this task exists to make is actually exercised, rather than
   solved by construction-immune live grep in 4 of 5 arms.

Falsifier for the headline verdict above: with the 22 scorer bugs fixed and
the 3 leaked cells re-run cleanly, if fathom still does not clear grep and
serena on raw pass count, the "fathom beats grep on correctness" claim has
no support from this pilot and the roadmap should say so plainly — cost
advantage and the verify-gate win would still stand on their own.
