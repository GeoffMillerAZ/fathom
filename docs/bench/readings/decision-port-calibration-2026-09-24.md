# The decision port's calibration check (CV-M11, ADR-044 decision 8)

## Verdict, plainly

**The `route` site fails "0.9 right nine times in ten," and — unlike a
simple under-confidence problem — raising the bar does not fix it: on this
sample, confidence and correctness are not even positively correlated.** At
confidence ≥ 0.9, route's suggested verb matched the cell's own gold-source
tool only 45.7% of the time (16/35), far under the 90% bar. Sweeping every
possible cutoff from 0 to 1.0 never crosses 90% — the best any threshold
achieves on this data is 58.5% (at ≥0.52), and the narrowest, most confident
slice (confidence exactly 1.0) is actually *worse* than the 0.9 cut it sits
inside: 28.6% (4/14). **No `route_confidence` value passes on this data, so
none was set — `internal/config.DefaultDecide.RouteConfidence` stays 0.9.**
Raising it further would not recover the calibration the check requires and
the data does not support pretending otherwise; ADR-042 is amended below
with this finding instead of a threshold bump. `certify_suggest`'s own
calibration (the second half of CV-M11) is unaffected and continues to look
good: 5/5 hand-reviewed proceed-band suggestions were correct
(`test-certifications-suggest.md`, unchanged by this reading).

**Why it still shipped fail-open and why this isn't an emergency.** `POST
/route` never asserts a fact — a firm (`applied`) line is only rendered at
the *proceed* band already (`decision.BandProceed`, confidence ≥ 0.9,
`RouteConfidence` on top of that), every other case degrades to a hedged or
silent suggestion, and a wrong verb suggestion only misdirects an agent
toward the wrong first MCP call — it never corrupts an index fact or
executes an action on the agent's behalf (vision 32 §3, I14). This reading
exists so that unproven trust does not silently calibrate itself into
"proven" — exactly the anti-Blindness posture ADR-044 is for.

## What was measured, and how

**Inputs (read-only):**

- Route decisions: every `decisions.jsonl` under
  `/Volumes/m2ext/tmp/sextant-r3-workspace/artifacts/scenariobench/worktrees/*/`
  (110 files found; 44 at the direct one-level-per-worktree location this
  reading uses; 22 of those are non-empty — the daemon only writes the file
  at all once the decision port is configured and a call is actually made).
  `.../scratchpad/wave-r3/workspace/` (the task's second named root) is
  **empty** — no scenario artifacts were ever written there in this
  session; every real row came from the `sextant-r3-workspace` root.
- Gold labels: `/private/tmp/.../scratchpad/benchroot/cells/<cell_key>/t{1,2}.json`
  (941 cell files scanned), each carrying `readings.gold_source_tool` — the
  tool that, empirically, surfaced the task's gold evidence in that
  session's own transcript (`Grep`, `Read`, `Bash`, an `mcp__fathom__*`
  tool, or `none` when no single tool applies, e.g. every `governance_bound`
  guards scenario).
- Join: a worktree's directory basename (e.g. `H1-T05-1b8765d-9cacc0fc4493`)
  was matched against every cell's `worktree_path`/`agent_diff_path`/
  `subject_clone`/`raw_path` fields, split on `/` and matched as an exact
  path component (never a bare substring — `mitmproxy` alone over-matched
  before this was tightened). All 22 non-empty worktrees matched exactly
  two cells (trial 1, trial 2) from run families `lookups-vs-jev-r1` (14
  worktrees, `H1-T01`..`T13` + `FRESH-M7-01`) and `guards-jev-r1` (8
  worktrees, `guards-i*`). No `lca-jev-r1` worktree had a non-empty
  `decisions.jsonl` at this location — that corpus's own routing effect was
  already measured separately in `decision-port-ab-2026-09-23.md`
  (a *different* metric: paired win rate between an off/on arm, not this
  reading's per-decision gold-tool match).
- Normalization: `readings.gold_source_tool` values of the form
  `mcp__fathom__<verb>` are stripped to `<verb>` before comparison (the
  same tool identity route's own `choice` field already names bare) —
  every other value (`Grep`, `Read`, `Bash`, `none`) is compared literally.
  **A `route` choice can never equal `Grep`/`Read`/`Bash` by construction**
  (fathom's verb space has no such names) — those rows are real, intended
  misses, not a normalization gap: they test whether route recognizes when
  a plain built-in tool, not one of its own verbs, is genuinely the right
  first move, which on this sample it does not.
- Two of a worktree's trials sometimes disclosed **different**
  `gold_source_tool` values for the same `task_id` (`H1-T01`:
  `none`/`obligations`; `H1-T11`: `Grep`/`Bash`) — the underlying
  `decisions.jsonl` is written once per worktree, shared by both trials
  (they reuse the same checkout sequentially), so a route decision cannot
  be attributed to one trial over the other. The headline numbers below use
  the **union** of both trials' gold labels (a route choice counts as a hit
  if it matches *either* trial's label) — the more generous reading. A
  trial-1-only variant is reported alongside; it is strictly worse (34.3%
  vs 45.7% at ≥0.9), never better, so the union variant is not hiding a
  more favorable result by omission.
- 2 of 84 route rows were `RecordError` rows (a hard Decide failure, e.g. a
  timeout) — excluded from the denominator entirely (nothing was answered,
  so nothing can be graded); 82 answered rows remain.

## Results

### By confidence bucket, union-of-trials gold (the headline numbers)

| Bucket | Hits / n | Fraction |
|---|---|---|
| high (≥ 0.9) | 16 / 35 | **0.457** — fails the 0.9 bar |
| mid (0.5–0.9) | 23 / 31 | 0.742 |
| low (< 0.5) | 1 / 16 | 0.062 |

Trial-1-only gold (the stricter, single-trial variant): high 12/35 = 0.343,
mid 23/31 = 0.742 (unchanged — no trial ambiguity in that bucket), low
1/16 = 0.062 (unchanged). Either way, the high bucket misses the bar by a
wide margin.

### By category (confidence ≥ 0.9 only)

| Category | Hits / n | Fraction |
|---|---|---|
| `governance_bound` (guards) | 12 / 12 | 1.000 |
| `obligations` (lookups-vs) | 4 / 7 | 0.571 |
| `claim_provenance` (lookups-vs) | 0 / 8 | 0.000 |
| `staleness` (lookups-vs) | 0 / 8 | 0.000 |

The `governance_bound` rows are gold-labeled `none` (no single tool is the
gold answer for a governance-following check) and route answered `none`
back every time — a trivially easy match that is real signal ("route
correctly recognizes when none of its own verbs apply") but not a test of
verb-*picking* accuracy. Excluding every `gold == ["none"]` row narrows the
sample to 46 non-trivial answered decisions; at confidence ≥ 0.9 those hit
only **4/19 = 21.1%** — the honest floor once the easy cases are set aside.

### Confidence is not monotonic with correctness (the important finding)

Sweeping every observed confidence value as a cutoff `T` (union gold,
hit = choice ∈ gold set for `T` and above):

| T | Hits/n | Fraction |
|---|---|---|
| 1.000 | 4/14 | 0.286 |
| 0.966 | 12/26 | 0.462 |
| 0.886 | 17/36 | 0.472 |
| 0.771 | 25/48 | 0.521 |
| 0.611 | 30/57 | 0.526 |
| 0.531 | 38/65 | 0.585 — the best any cutoff achieves |
| 0.497 | 40/67 | 0.597 |
| 0.314 (everything) | 40/81 | 0.494 |

**No cutoff in `[0, 1]` reaches 0.9.** The maximum over the whole sweep is
0.585, and the narrowest, highest-confidence slice (exactly 1.0) is *worse*
than the 0.9-and-above bucket it sits inside (0.286 vs 0.457) — meaning the
21 decisions with confidence in `[0.9, 1.0)` are collectively *more*
accurate (12/21 = 57.1%) than the 14 at exactly 1.0 (4/14 = 28.6%). A
well-calibrated model's accuracy should never fall as its own stated
confidence rises to the maximum; this one's does. Per the task's own
contingency ("raise the proceed threshold to the smallest value that passes
on this data") — **there is no such value**, so none was set.

### Per-worktree detail (union gold)

| Worktree (task) | Category | Gold (union) | n | Hits | Choices | Confidences |
|---|---|---|---|---|---|---|
| FRESH-M7-01 | freshness | Grep | 4 | 0 | find_callers ×4 | 0.75–0.76 |
| H1-T01 | obligations | none, obligations | 4 | 4 | obligations ×4 | 1.00 |
| H1-T02 | obligations | Grep | 3 | 0 | obligations ×3 | 1.00 |
| H1-T03 | verify | Grep | 4 | 0 | find_callers ×4 | 0.34–0.43 |
| H1-T04 | verify | none | 4 | 0 | staleness_report, find_symbol ×3 | 0.31–0.36 |
| H1-T05 | blast_radius | find_callers | 4 | 4 | find_callers ×4 | 0.82–0.89 |
| H1-T06 | blast_radius | Grep | 4 | 0 | find_callers ×4 | 0.35–0.39 |
| H1-T07 | claim_provenance | Read | 4 | 0 | obligations ×4 | 0.99–1.00 |
| H1-T08 | claim_provenance | none | 4 | 0 | obligations ×4 | 0.94–0.97 |
| H1-T09 | concern_search | search | 4 | 4 | search ×4 | 0.54–0.61 |
| H1-T10 | concern_search | search | 3 | 3 | search ×3 | 0.50–0.59 |
| H1-T11 | contradiction | Bash, Grep | 4 | 0 | search ×4 | 0.81–0.84 |
| H1-T12 | staleness | Grep | 4 | 0 | staleness_report ×4 | 0.94–0.95 |
| H1-T13 | staleness | Grep | 4 | 0 | staleness_report ×4 | 1.00 |
| guards-i1-packagedeps-sorted | governance_bound | none | 3 | 0 | find_callers ×3 | 0.42–0.44 |
| guards-i11-mcp-tool-doc | governance_bound | none | 3 | 3 | none ×3 | 0.76–0.82 |
| guards-i16-raw-cp-deploy | governance_bound | none | 3 | 3 | none ×3 | 0.77–0.82 |
| guards-i19-org-agnostic-fixture | governance_bound | none | 4 | 4 | none ×4 | 0.97 |
| guards-i2-store-purego | governance_bound | none | 4 | 4 | none ×4 | 0.94–0.95 |
| guards-i28-duckdb-path | governance_bound | none | 2 | 2 | none ×2 | 0.63–0.68 |
| guards-i31-workspace-artifact | governance_bound | none | 4 | 4 | none ×4 | 0.97–0.98 |
| guards-i36-usecase-maintained-line | governance_bound | none | 5 | 5 | none ×5 | 0.52–0.62 |

Worth naming directly: `guards-i1-packagedeps-sorted` is gold `none` but
route confidently (0.42-0.44, admittedly mid-band) suggested
`find_callers` — the one governance_bound worktree route got wrong, out of
eight. Every `H1-T*` lookups-vs worktree where route chose `obligations` or
`staleness_report` at very high confidence (T01/T02/T07/T08, T12/T13) was
wrong whenever the gold tool was an external one (`Grep`/`Read`) — route's
highest-confidence answers cluster on `obligations`/`staleness_report`
specifically, and those are exactly the verbs most often wrong against
these particular tasks' gold tool.

## `certify_suggest`'s own calibration (the other half of CV-M11)

Unchanged from `test-certifications-suggest.md` (2026-09-23): 5 of 5
hand-reviewed proceed-band suggestions were correctly classified (all
`asserts`-strength, confidence 0.90–1.00, `asserts_noul` 0.92–0.97) — no
new spot-check was warranted, since nothing about `certify_suggest`'s own
decision port wiring changed in this wave and no new proceed-band run was
produced to review. That site's calibration is not in question here.

## What this changes

- `internal/config.DefaultDecide.RouteConfidence` **stays 0.9** — no
  passing threshold exists on this data to raise it to (see "Confidence is
  not monotonic" above). Raising it anyway, to any value, would either do
  nothing new (this data's own maximum-confidence slice is *worse* than
  0.9's) or falsely imply a fix that the evidence does not support.
- ADR-042 is amended (`ADR.md`) with one dated paragraph recording this
  measured failure and the non-monotonicity finding, alongside the
  2026-09-23 `rank_within_tier` amendment it sits next to.
- This is a live, unresolved finding, not a closed one: the fix this data
  points toward is a better route prompt/model (or a materially larger,
  less `governance_bound`-heavy sample to confirm the shape holds), not a
  threshold. Tracked as a follow-up rather than invented here.

Measured 2026-09-24 against `sextant-r3-workspace`'s `lookups-vs-jev-r1`
and `guards-jev-r1` runs (typesafe/Jev, live). Every number above is
reproducible from the two read-only sources named in "What was measured."
