# Budgeted answers — fathom accounts for the context it spends

> Phase X of `docs/vision/20-ambient-fathom.md`. This is the **reframe** of
> the owner's ask: *"does fathom have ways we could help with keeping
> subagent context threads concise when needed? Like the caveman skills? Is
> that our place?"*
>
> **Status:** design.

---

## 1. The reframe, and why the original ask was half right

Half of the ask is not fathom's place and should be declined:

> **Rejected:** compressing how an *agent* phrases its output. Prose style
> is a skill; skills already exist and do it well. A code-intelligence
> engine that starts rewriting an agent's English has left its lane, and
> would be worse at it than the thing already doing it.

The other half is fathom's place, entirely, and is currently unmanaged:

> **Accepted:** the size of what **fathom itself** puts into an agent's
> context. Every tool result is context fathom spent on the agent's behalf.
> Plus 11,839 bytes of tool schema in every connected session before any
> work happens. That is fathom's own footprint, nobody else can control it,
> and today nothing measures it per call.

The irony is worth stating: fathom's entire measurement program exists to
find out whether it saves the agent tokens, and fathom has never measured
how many tokens it *costs* the agent. The denominator has been missing from
its own instrument.

---

## 2. The contract

Every tool on the agent surface gains an optional `budget` (approximate
tokens for the response) and every response gains an elision block.

```json
// request
{ "name": "find_callers", "arguments": { "symbol": "buildObservatoryModel", "budget": 800 } }

// response
{
  "callers": [ ...12 entries... ],
  "elided": {
    "count": 35,
    "of": 47,
    "ordering": "call depth, then fan-in",
    "how_to_get_more": "re-call with budget: 4000, or limit: 50",
    "cost_estimate_tokens": 2900
  }
}
```

Three rules, and the third is the one that matters:

1. **A budget is a target, not a hard cap.** Refusing to answer because the
   answer is large is worse than answering partially. If the smallest
   useful response exceeds the budget, return it and say so.
2. **Ordering is declared.** "12 of 47" is only useful if the reader knows
   *which* 12. Every tool states its ranking, and the ranking is
   deterministic (house rule).
3. **Silent truncation is a defect, always.** A tool that drops results
   without an elision block is lying about completeness, and an agent that
   believes it will conclude "there are only 12 callers" and reason from a
   false premise. This is worse than any amount of verbosity. **Every tool
   that can truncate must emit `elided`, enforced by a test that iterates
   the registry.**

---

## 3. Defaults, so the common case needs no thought

An agent that never passes a budget must still get sane behaviour, and the
defaults come from measurement rather than taste — the call log already
records response sizes per tool.

| Tool class | Default budget | Rationale |
|---|---|---|
| point lookup (`find_symbol`, `read_source`) | none | the answer is inherently small |
| list (`references`, `find_callers`, `query_symbols`) | 1,500 | the common case fits; the pathological case is the reason this exists |
| composite (`symbol_context`, `pack_context`) | 4,000 | the point of these is breadth |
| map (`repo_map`, `outline`) | 1,200 (was 2,500; cut by SR-41, `docs/plans/surface-reckoning.md`) | orientation only — every `repo_map` answer says so in its envelope; a caller that wants more reaches for `query_symbols`/`pack_context` |

Defaults are per-capability-registry settings (phase T), so a project that
wants tighter or looser budgets sets them once in
`.fathom/capabilities.cue` rather than at every call site.

---

## 4. The schema tax

11,839 bytes of tool schema enter every connected session's context before
a single question is asked. Across 188 sessions that is real money, and it
is fathom's own overhead.

Three reductions, in order of how much they help:

1. **Trim descriptions to their working minimum.** Tool descriptions were
   written as documentation. They are prompts. A measured pass over the 16
   agent-surface tools, keeping recall constant, is the cheapest win
   available and should be done first.
2. **Profile-scoped surfaces** already exist (`/rpc/agent` is 16 of the
   full registry). Add narrower ones — `/rpc/lookup` for a subagent that
   only needs `find_symbol` + `read_source` — so a delegated agent carries
   3 tools of schema, not 16. **This is the honest answer to the original
   subagent-concision ask:** a subagent's context is kept small by giving
   it a smaller surface, not by compressing its prose.
3. **Measure before and after.** `server_stats` already reports schema
   bytes per surface; the brief already prints it. Any trimming pass
   reports the delta, and any tool added to a surface reports its cost.

---

## 5. The instrument that justifies deletion

For each tool, from data already in the call log plus the new response-size
accounting:

```
tool             calls   median resp    total tokens   elided rate   outcome
find_symbol      2,104         180 t         379,000          0.2%   hit 94%
references         420       1,900 t         798,000         31.0%   hit 88%
repo_map            38       6,200 t         236,000          0.0%   hit 100%
pack_context        12      11,400 t         137,000          0.0%   hit 75%
```

The column that does not exist yet and matters most is **total tokens** —
what this tool has cost the fleet. Paired with outcome, it answers a
question the program has never been able to ask: *is this tool worth its
context?*

Vision 20 §7 question 5 — "what do we delete?" — becomes answerable. A tool
with high cost and low hit rate is a deletion candidate with evidence
behind it, which is the only kind of deletion that ever actually happens.

**This is also the honesty check on phase X itself.** If budgeting ships
and the total-token column does not fall, the budgets are set wrong or the
tools were already lean, and the phase should be declared complete-and-
ineffective rather than extended.

---

## 6. Where this lands in the dashboards

A **Cost** panel on `/observatory/impact`:

- Total tokens fathom has put into agent contexts, by tool, over the window
- The standing schema tax per surface, and its trend
- Elision rate per tool — a high rate means the default budget is too tight
  for how that tool is actually used, which is a tuning signal, not a
  failure
- The one honest ratio available without an A/B: **tokens fathom spent**
  against **discovery actions it served**. Labeled observational, never
  captioned as a saving (law 12).

---

## 7. Test obligations

- Every tool that can truncate emits `elided`; enforced by a registry
  iteration test so a new tool cannot skip it.
- `elided.count + len(results) == total` — the arithmetic is checked, so
  the disclosure cannot drift from reality.
- Ordering is deterministic: the same query at budget 500 and budget 5,000
  returns the smaller set as a strict prefix of the larger.
- A budget too small for the minimum useful response returns that response
  and says the budget was exceeded, rather than returning nothing.
- Response-size accounting is recorded per call and totals correctly across
  a fixture window.
- A narrow profile carries only its own tools' schema; the byte count is
  asserted, so an accidental widening fails the build.

---

## 8. Explicitly out of scope

- Rewriting, summarizing or compressing agent prose (§1).
- Model-side context management. Fathom accounts for what it emits; what
  the agent does with its window is the agent's business.
- Hard caps that refuse to answer (§2 rule 1).
