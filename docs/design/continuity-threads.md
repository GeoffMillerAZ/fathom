# Continuity threads — mechanics, hooks, and the measurement loop

> Binding promotion of `docs/vision/21-continuity-threads.md` (TH-1..5 in
> `docs/plans/ambient-fathom.md`). Owner directive 2026-08-13: *"spec it
> durably and then implement… design any metrics collection or flywheel
> features to see how much impact this is making and how much tokens it is
> likely saving — some way to improve and/or show an organization how much
> it is helping."*
>
> **Status:** v1 mechanics. The refusal boundary and tokenomics gate live
> in the vision doc and are not repeated here; they bind everything below.

---

## 1. Store

`~/.fathom/threads.jsonl` — user-central (threads span repos; law 13:
local, never shared), append-only, latest-row-wins by item id (the
ratings-log discipline: a correction is a new row, history stays
auditable).

```json
{"schema_version":1,"id":"th-8f2a","thread":"purge-rework","kind":"decision-pending",
 "text":"suppression moves to the query layer, not store.Open — daemon holds one handle",
 "pointers":{"session_id":"fff83839-…","repo":"sextant","plan_row":"docs/plans/ambient-fathom.md#SP-1"},
 "filed_at":"2026-08-13T02:11:00Z","filed_by":"agent",
 "completes_when":{"kind":"plan_row_checked","plan":"docs/plans/ambient-fathom.md","row":"SP-1"}}
```

- `thread` is a free string; threads exist by being named (no registry to
  desync). `fathom thread list` folds names from items.
- `kind` ∈ `decision-pending | spec-unbuilt | in-flight | side-task |
  handoff-note | session-end | pre-compact` (the last two are the
  mechanical deposit kinds).
- Closing is a new row `{"id":…,"closed_at":…,"closed_by":"human|agent|condition"}`.

### `completes_when` v1 — two checkable conditions, both cheap

| kind | true when | check cost |
|---|---|---|
| `file_exists` | the named path exists in the named repo | one stat |
| `plan_row_checked` | the row's checkbox is `[x]` in the plan file | one read + line scan |

Conditions are evaluated at **read time** (brief/list/flywheel — the same
no-sweep discipline as anchored memory); a satisfied condition appends the
closing row with `closed_by: "condition"` so the closure is durable and
attributed. Richer conditions (row *proven*, not merely checked) wait for
a version that can afford a daemon round-trip; v1 stays offline-safe.

## 2. Verbs

```
fathom thread brief   [--budget-bytes 2400] [--source startup|clear|resume|compact]
fathom thread note    "text" [--thread T] [--kind K] [--completes-when …]
fathom thread point   "text" --session S [--thread T]        # side-task pointer
fathom thread list    [--thread T] [--all]                   # includes dormant
fathom thread close   <id> [--why …]
```

`brief` is the only surface with a context cost, so it carries the budget:

- **≤ 2,400 bytes (~600 tokens), enforced, tested.** Items are ranked by
  recency-of-touch; what does not fit is summarized as one line — `and N
  more: fathom thread list` — never silently dropped (law 3 applied to a
  briefing).
- `--source resume|compact` prints **nothing and exits 0**: their context
  survives, and injecting there is the schema-tax mistake again.
- Composition order (highest value per byte first): handoff notes,
  decisions-pending, in-flight (derived: git stashes + non-main WIP branch
  + dirty-file count in the current repo), side-task pointers, then one
  line for open plan work (`run: fathom plans next --service <svc>` — a
  pointer, not the listing; the derived detail stays on demand).

## 3. Hook adapters (Claude Code)

| Hook | New verb | Behaviour |
|---|---|---|
| SessionStart | (existing `session-hook`) | parses `source`; on `startup`/`clear` appends the brief to its context output and **stamps** `~/.fathom/session-stamps/<sid>.json` `{source, brief_bytes}`; on `resume`/`compact` stamps only |
| SessionEnd | `session-end-hook` | on reason `clear`/`logout`: ONE mechanical deposit, and only when there is something to point at (uncommitted changes, stashes, or claimed-unproven rows) — an empty session deposits nothing; silence is fine |
| PreCompact | `pre-compact-hook` | one `pre-compact` pointer (session id + repo + trigger) so the pre-summary span stays findable; **never** reads the transcript content |
| Stop | (existing) | joins the stamp into the ledger row: `session_source`, `brief_injected`, `brief_bytes` |

All deposits are **mechanical facts** (git state, checkbox counts, ids) —
the transcript-mining refusal is structural: no deposit path receives
transcript *content*, only its path/id.

Codex (no lifecycle hooks): AGENTS.md instructs `fathom thread brief` at
session start; the store never knows which harness wrote to it (TH-5).

## 4. The measurement loop (the owner's org-evidence ask)

**What gets recorded** (all mechanical, all joined by session id):

- ledger rows gain `session_source` + `brief_injected` + `brief_bytes` —
  the *labeling* that makes cohorts comparable;
- the cost of the feature is thus measured, not estimated: sum of
  `brief_bytes` over sessions IS what continuity spent.

**The pre-registered bet, evaluated as code** (`threadBetCheck`):

- **Cohorts:** `fresh-with-brief` (source `startup|clear`, brief injected)
  vs `resumed` (source `resume`). Units need `first_edit_turn` and the
  token split; the ≥10-turn floor applies as in the arm comparison.
- **Warm-up:** median `first_edit_turn` per cohort — did the brief make a
  fresh session start editing as soon as a resumed one?
- **All-in per-turn cost:** `(in + out + cache_read×0.1) / turns` — the
  figure that carries the 22× cache-rent line. Cache-read weighting is
  named on every surface that shows it (it is a pricing assumption, not a
  measurement).
- **Verdict states:** `collecting` (n<20 fresh units, counts shown) →
  `met` / `not-met`. The registered kill is wired as the check's own
  output: on `not-met` at n≥20 the verdict text says the injection is to
  be removed — the instrument carries its own consequence.

### §4.1 Outcomes are DERIVED, not self-reported (owner field report, 2026-08-14)

The original design required `fathom rate <1-5>` as the outcome half of
every comparison — and the owner reported the honest killer: *"I feel
unlikely to do that."* An instrument its user won't feed is a broken
instrument (the O28 lesson, one loop up), and fathom's own doctrine
already names the fix: **derived beats deposited.** A session's outcome
is not a feeling; it is on disk.

- **Standing lane (mechanical, zero cost): the derived outcome.** Per
  measured unit, computed from artifacts the window already touches:
  commits landed in the window, plan rows flipped AND proven (`verify`,
  not claimed), gate outcomes recorded, pushes. A unit that landed proven
  work and a unit that spun are now distinguishable without asking
  anyone. Comparisons condition on outcome-comparable units.
- **Benchmark lane (LLM, priced, occasional): `assess`.** Owner-authorized
  same day: *"it's ok to spend tokens for benchmarking… I won't be
  constantly benchmarking."* An explicitly-invoked assessment judges the
  unit's **work products on disk** — diffs, plan deltas, gate results —
  and NEVER transcripts (the transcript-mining refusal is not softened
  for evaluation; what shipped is also simply more honest than what was
  said). Runs only when invoked or inside a declared bench window; each
  run reports its own token cost.
- **`fathom rate` demotes to an optional human override** — kept for the
  case where the human disagrees with the derived read, never again a
  precondition for a verdict. Surfaces that today say "self-rated
  outcomes are missing" change to report the derived outcome, with the
  override named when present.

Rows: DO-1..4 in `docs/plans/ambient-fathom.md`.

**Where it surfaces:**

- `fathom flywheel` gains a **continuity** loop: CLOSED = deposits are
  landing AND briefs are being injected AND the bet is collecting/met;
  OPEN = briefs injected but never deposited-to (a conveyor moving
  nothing), or stamps missing (the labeling broke — the O24 class);
  IDLE = feature present, no fresh session yet.
- `fathom brief` (the org artifact) gains a **Continuity** section: units
  per cohort, both medians with denominators, the feature's own measured
  cost (`brief_bytes` total), and the verdict — absences named, never
  zeros. This is the "show an organization" surface, and it inherits the
  value brief's whole honesty covenant.

## 5. Test obligations (the load-bearing ones)

- the brief respects its byte budget at the boundary (a fixture that
  overflows must truncate WITH the `and N more` line) — and `resume`
  prints nothing;
- a `completes_when: plan_row_checked` item closes itself when the box
  flips, attributed `closed_by: condition`;
- SessionEnd deposits on `clear` with something to point at, and deposits
  NOTHING for a clean empty session;
- the stamp → ledger join labels rows correctly, and a missing stamp
  yields `session_source: ""` (unknown, never guessed);
- `threadBetCheck` refuses a verdict below 20 fresh units and names the
  counts; the cache-read weighting appears in its output text;
- falsification runs: budget enforcement removed → the budget test goes
  red; source gating removed → the resume test goes red.

## 6. Explicitly out of scope for v1

- `thread promote` / `thread handoff` (TH-5 — next slice);
- daemon-verified `completes_when` (row *proven*); MCP thread tools
  (blocked on phase X pricing); UserPromptSubmit injection (held, vision
  21 §9); any LLM summarization lane (refused, permanently).
