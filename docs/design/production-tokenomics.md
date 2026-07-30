# Production tokenomics — measuring the coding aid in the wild

**Status:** adopted design, owner-commissioned 2026-07-30 ("does this tell us
anything about its role in saving tokens or not? is there more we'd have to
put in to increase our ability to understand how well it's being used as an
agentic aid and for reducing token usage?"). Successor instrument to
`docs/plans/falsifiability.md` (the synthetic benches) and
`docs/design/research-flywheel.md` (channel liveness). Registered as phase P
in `docs/vision/13-expert-suite.md` §9.

## 1. What the 2026-07-30 flywheel data actually proves — and doesn't

First real production read (`fathom research report --since 168h`, 7,466
calls):

| Signal | Value | What it licenses us to say |
|---|---|---|
| injection coverage | 91% (4,106 seen → 3,753 answered → 1,966 served) | the forced channel is **alive and near-universal** |
| guarded-edit pre-hook | 280 seen / 96 served / 15 deduped, within an hour of shipping | the new channel **works in real sessions** |
| latency | p50 130 ms / p95 1.54 s | the hot loop is **not paying a visible stall** |
| verify gate | instrumented, 57 failing rows caught | the proof channel **bites** |
| **pulled discovery tools** | `references` 42, `find_callers` 10, `find_symbol` 10, `read_source` 9, `outline` 7, `query_symbols` 7, `symbol_context` 5, `pack_context` 2 — **≈130 of 7,466 calls (1.7%)** | the agent still **rarely chooses** the index |
| `list_services` | **3,624 calls** — the single most-called tool | hook identity resolution is the **dominant traffic** |

**The honest verdict on "is it working as an agentic coding aid":** the
*forced* half is demonstrably working (91% of edits governed, gates biting,
sub-second). The *pulled* half is still ~2% of traffic — consistent with the
program's own repeated finding that forced channels win and habit doesn't
form on its own. That is a product truth, not a measurement artifact.

**The honest verdict on token savings: this data cannot answer it at all.**
Nothing in the report carries a token dimension. We know the channel fired
1,966 times; we do not know what those payloads cost, nor what they
prevented. The only token evidence in the whole program is synthetic
(context-bench: 0.570× home turf, 1.96× lookup, n=1 per task, 2026-07-24).
**We have a liveness instrument and a lab benchmark. We have no production
tokenomics instrument.** Everything below exists to close that gap.

## 2. Doctrine (new law 12)

12. **No saving is claimed without a counterfactual.** A token number is a
    *cost* until something establishes what would have happened otherwise.
    Payload cost is measurable directly; savings require substitution
    evidence (§4) or an A/B (§5). Reporting cost as if it were saving is
    the exact dishonesty this program retracted a ×10 claim over.

## 3. Instrument A — payload cost accounting (cheap, deterministic, do first)

Every injected payload's size is knowable at emit time and currently thrown
away. Record, per beat: rendered bytes, an explicit token estimate (bytes/4,
labeled an *estimate* — never presented as measured), rule count, and phase
(pre/post). Aggregate in `research report` as:

- total injected tokens in window, per channel (session brief, post-edit,
  pre-edit guard, module rules);
- per-edit distribution (p50/p95) — the number that answers "what does
  governance cost per edit";
- **cost ledger, not savings ledger** (law 12): this section of the report
  is titled "what fathom SPENT".

This alone makes the honest sentence sayable for the first time:
*"governance cost N tokens per edit across M edits."*

## 4. Instrument B — the discovery-share denominator (the missing metric)

The program has wanted "fathom-share of discovery actions" since
`docs/design/product-vision.md` §2 and never had a denominator: we count
fathom's tool calls but never the greps they were supposed to replace.

**Mechanism (no new tool, no LLM):** extend the PreToolUse observer that
guarded edits already installs to a *counting-only* matcher on the
harness's own discovery tools (`Grep`, `Glob`, and `Read` when the target is
outside the current edit set), plus `Bash` commands whose argv starts with
`grep`/`rg`/`find`/`ag`. It injects nothing and never blocks — it emits one
beat per discovery action with a coarse kind (`grep|glob|read|shell-search`).
Cost: the same local fast path guarded edits already proved (no daemon call).

Then `research report` can state, deterministically:

```
discovery actions in window: 1,412
  fathom index tools  130  ( 9%)
  harness grep/glob   983  (70%)
  shell search        299  (21%)
```

That is the adoption number the vision docs have been asserting blind. It is
also the *precondition* for §5: substitution can only be observed against a
denominator.

**Honesty constraints:** `Read` is dual-use (reading a file you already
located is not discovery) — count it in its own bucket, never folded into
"grep", and say so in the report. A rise in fathom share is *correlational*
until the A/B runs; the report must label it so.

## 5. Instrument C — the A/B protocol (the only causal instrument)

Pre-registered, owner-run, and the only thing that licenses a production
savings claim:

- **Unit:** a working session in an indexed repo.
- **Arms:** obligations injection + index tools ON (control = today) vs
  a `FATHOM_CHANNELS=off` session (hooks silent, MCP surface unregistered).
- **Measure:** end-of-session harness token total (recorded by the Stop
  hook, §6), turns, and a subjective 1–5 "did I get where I wanted" note.
- **Criterion, registered before any run:** ON must show lower median
  tokens-per-session at equal-or-better self-rated outcome across ≥20
  sessions per arm. Recorded either way, per house record.
- **Kill:** if ON is not better after 40 sessions, the production tokenomics
  claim is retracted in every doc that makes it (same mechanism as B1-4).

## 6. Instrument D — session token ledger

The Stop hook already runs. Extend it to append one line per session:
session id, repo, duration, turn count, harness-reported token totals if the
environment exposes them (and an explicit `tokens: unavailable` when it does
not — never a zero), plus the counts from A and B for that session. This is
the join key that makes A, B, and C a single dataset instead of three
disconnected tallies.

## 7. Efficiency finding this analysis produced (fix alongside)

`list_services` at **3,624 calls** is the fleet's most-called tool, and every
one is hook identity resolution (`candidatesWithDaemon`, the O15 fix). It is
pure plumbing paid on every edit. **Fix:** cache the service-identity
resolution per (repo root, daemon PID/boot id) in the same short-lived local
stamp area guarded edits use, with an explicit TTL and a `--no-cache` escape;
report cache hit rate in the flywheel. Expected: ~3,600 round trips → tens.
This is a latency and load win, not a token win — label it as such.

## 8. Non-goals

No harness-internal transcript parsing (fragile, version-coupled); no
per-token billing reconstruction; no claim that correlation is causation
before §5 runs; no telemetry leaving the machine (the research-bundle
allowlist governs anything shared, unchanged).

## 9. Deliverables (phase-P plan rows derive from here)

1. Payload cost accounting in every injection beat + the "what fathom SPENT"
   report section (§3).
2. The counting-only discovery observer + the discovery-share section (§4),
   with `Read` bucketed separately and correlation labeled.
3. Session ledger via the Stop hook, with honest `unavailable` (§6).
4. `FATHOM_CHANNELS=off` kill switch (needed by §5, useful for debugging).
5. The A/B protocol committed as a pre-registered plan section BEFORE any
   run, criterion and kill clause included (§5).
6. Service-identity caching + hit-rate reporting (§7).
7. Docs: research-flywheel use-case + adoption runbook gain the new
   sections; law 12 into `docs/vision/13-expert-suite.md` §8.
