# Anchored memory — the only memory system that can expire itself

> Phase W of `docs/vision/20-ambient-fathom.md`. Answers the owner's ask:
> *"Is fathom a good platform for memories? Do we have unique ways to apply
> memories and make sure they stay fresh? Work out a vision for that."*
>
> **Status:** design. This is the highest-ceiling item in the program and
> the one with the most ways to become junk, so the refusals come first.

---

## 1. The answer, in two halves

**For general memory — no, and fathom should refuse.** "Geoff prefers
tabs", "we use Linear", "the standup is at 9" — a file does this better.
A code-intelligence engine that becomes a worse notes app has lost the
plot, and the market is full of tools that made exactly that mistake.

**For anchored memory — fathom is the best-positioned system I know of**,
for one reason no general memory system has: **it can detect when the thing
a memory is about has changed, and expire the memory automatically.**

Every memory system in common use — `CLAUDE.md`, a wiki, a vector store of
notes, an agent's scratchpad — is append-only and rots silently. Its worst
failure is not forgetting; it is confidently recalling something that
stopped being true. A note saying "`parseConfig` validates the schema" is
worse than no note at all once `parseConfig` has been split into three
functions, because it is *specific*, *plausible*, and *wrong*, and the
agent that reads it will not check.

Fathom already owns every primitive needed to fix that, all built and all
tested:

| Primitive | Already exists as |
|---|---|
| symbol identity across commits | `fingerprints`, `ListSourceRefs`, the hash gate |
| commit drift detection | `staleness_report`, auto-refresh on HEAD drift |
| evidence tiers and ceilings | `review/scoring`, law 14, `ApplyCeiling` |
| governed injection before an edit | the obligations layer + PostToolUse hook |
| claim history | `claim_events`, append-only |

The gap between "fathom today" and "a memory system that expires itself" is
a schema and a drift evaluator. Almost nothing else.

---

## 2. What a memory is

```
memory := {
  id
  service
  claim         // one sentence, present tense, falsifiable
  anchors[]     // ≥1 — what this claim is ABOUT
  evidence[]    // ≥1 — what makes it credible
  tier          // derived from evidence, never asserted (law 14)
  state         // fresh | suspect | orphaned   (derived from anchors)
  created_at, last_confirmed_at
  provenance    // who or what wrote it
}
```

Two required fields carry the entire design.

### 2.1 Anchors — what the memory is about

| Anchor kind | Binds to | Drifts when |
|---|---|---|
| `symbol` | service + symbol name + kind | the symbol's body hash changes |
| `file` | service + source_ref | the file's content hash changes |
| `path` | service + glob | any matching file changes |
| `commit` | service + commit sha | never (a historical fact) |
| `claim` | another claim's id | that claim's state changes |

`symbol` is the sharpest and should be the default. `path` is the loosest
and should be discouraged in review — a memory anchored to `**/*.go` never
drifts usefully, which makes it indistinguishable from an unanchored note.

**A memory with no anchor is refused.** Not stored at a low tier —
refused, with the reason: *"this is a preference, not a memory about this
codebase; put it in CLAUDE.md."* This refusal is the feature. It is what
keeps the store from becoming the junk drawer that every other memory
system becomes.

### 2.2 Evidence — what makes it credible

Reusing the existing tier vocabulary exactly, no new ladder:

| Tier | Means | Ceiling under law 14 |
|---|---|---|
| `verified` | a machine check enforces it | full |
| `pre-scan` | derived from the index (a real call edge, a real type) | high |
| `doc-claimed` | an ADR or comment says so | capped |
| `sme-stated` | a human asserted it | capped |
| `assumed` | inferred by a model | floor |

`ApplyCeiling` — which exists in agentic-kit and which fathom calls **zero
times today** — is called here, at the write path, deriving the ceiling
structurally from the evidence rather than trusting the caller's claimed
tier. Phase S specified this; anchored memory is its first real consumer,
and shipping them together is cheaper than shipping either alone.

---

## 3. The lifecycle — the part nobody else has

```
        write
          │
          ▼
      ┌────────┐   anchor content changed    ┌─────────┐
      │ fresh  │ ─────────────────────────▶  │ suspect │
      └────────┘                             └─────────┘
          ▲                                    │     │
          │  re-confirmed against new content  │     │ anchor deleted
          └────────────────────────────────────┘     ▼
                                              ┌──────────┐
              anchor deleted                  │ orphaned │
        ──────────────────────────────────▶   └──────────┘
```

**fresh** — every anchor's content hash matches what it was when the memory
was last confirmed. Surfaces normally.

**suspect** — an anchor's content changed. The memory *still surfaces*,
because a suspect memory is usually still true and suppressing it silently
would lose more than it saves — but it surfaces **labeled**, with what
changed and when:

> ⚠ `parseConfig` has changed since this was recorded (2026-07-02 → 3 commits
> ago). *"parseConfig validates the schema before returning."*

Law 14's ceiling drops one tier while suspect. A suspect `verified` memory
is not verified any more — the machine check that verified it ran against
different code.

**orphaned** — an anchor no longer exists (symbol deleted, file removed).
The memory **stops surfacing** and moves to a review queue. It is not
deleted: "we removed the retry logic in July" is exactly the kind of thing
worth keeping, and the anchor's disappearance may itself be the point.

### 3.1 When drift is evaluated

At **read time**, from hashes fathom already maintains, not on a timer.
Three reasons: it cannot be missed while the daemon is down; it costs one
hash comparison per anchor on a code path that already loads them; and it
means a memory is never wrong *because a sweep had not run yet*.

The evaluation is cached per (anchor, index-generation) so a session that
surfaces the same memory forty times hashes once.

### 3.2 Re-confirmation

The review queue is where suspect memories go to be resolved, and it must
be cheap to work or it will not be worked:

```
fathom memory review [--service S]
```

For each suspect memory: the claim, the anchor's diff, and three keys —
**still true** (re-confirm, back to fresh, `last_confirmed_at` updated),
**no longer true** (retire, retained with a `retired_at`, never surfaces),
**rewrite** (edit the claim, re-confirm against current content).

Re-confirmation records *who* confirmed and against *which* commit, so a
memory confirmed by a model carries a lower tier than one confirmed by a
human, automatically, without anyone having to remember the rule.

---

## 4. How memories reach the agent

Through the path that already exists. No new surface, no new hook.

The obligations layer already injects governing rules before an edit via
the PostToolUse hook. Anchored memories join that payload, in a separate
labeled block, ordered by tier then specificity:

```
Obligations for internal/fusion/pipeline.go:
  • [rule] Determinism is a contract: sort outputs; inject clocks.

Memories about this file:
  • [pre-scan · fresh] The post-fetch pass must stay deterministic — it is
    the only place the hash gate can observe order. (confirmed 2026-08-01)
  • [sme-stated · suspect ⚠] Budget gating happens before extraction, not
    after. (`runPostFetch` changed 2 commits ago — re-confirm?)
```

Three properties this inherits for free, and each would be real work to
build standalone:

- **It is already measured.** Obligation injections are counted (6,992 in
  the current window) and appear in the brief and the observatory. Memory
  surfacing is measurable on day one with no new instrument.
- **It is already budgeted.** Phase X's budget applies to the whole
  payload, so memories cannot quietly inflate every edit's context.
- **It is already governed.** Scope rules, capability toggles, and law 13
  all apply without a second implementation.

Memories are also queryable directly — `memory_search`, one tool, on the
agent surface — but injection is the primary path. A memory nobody asks for
is a memory that does not help, and the whole point is that the agent does
not know to ask.

---

## 5. Where memories come from

Ordered by how much I trust them, which is also the order to build them.

1. **Explicit** — `fathom memory add --anchor symbol:parseConfig --claim '…'
   --evidence …`, or the `memory_write` tool. The agent that just learned
   something records it, with an anchor, or it is refused.
2. **From findings** (phase R already built) — a resolved finding is a
   memory with a symbol anchor and `verified` evidence, for free. *"This
   nil check is load-bearing; removing it caused the 2026-07 incident."*
   This is the highest-value source and it requires almost no new code.
3. **From verify catches** — a plan claim that failed the gate, and what
   actually turned out to be true, is a memory with excellent provenance.
4. **From ADRs and design docs** — `doc-claimed`, capped, anchored to the
   symbols the document names.

**Not from transcript mining.** An LLM summarizing a session into "lessons
learned" produces plausible, unfalsifiable, unanchored text at scale — the
exact material this design exists to exclude. It would fill the store with
`assumed`-tier claims faster than anyone could review them, and their sheer
volume would drown the four sources above. Refused.

---

## 6. What makes this defensible rather than a feature

Stated plainly because it is the reason to build it at all:

- **Every other memory system rots silently.** This one knows when it has
  rotted, says so at the point of use, and can be repaired in one
  keystroke.
- **The anchor is free.** Fathom already computes every hash this needs, on
  a path it already runs. The marginal cost of expiry is a comparison.
- **The refusal is the product.** A system that declines to store an
  unanchored, unevidenced claim stays useful at year three. One that
  accepts everything is a wiki with worse search by month six.
- **It closes the loop the program is missing.** Vision 20 §7 question 3:
  "when the index is wrong, what corrects it?" A retired memory *is* a
  correction, recorded, anchored and surfaced.

---

## 7. Falsification — how we learn this was a bad idea

Pre-registered, per house discipline:

- **If, after 30 days of real churn, the suspect queue is empty**, the
  anchoring is too coarse (probably too many `path` anchors, or symbol
  hashing at file rather than body granularity). Fix the granularity or
  abandon the phase — an expiry mechanism that never fires is dead weight
  that still costs a read-time hash on every injection.
- **If the suspect queue grows faster than it is worked**, surfacing
  suspect memories is net-negative noise; suppress them at a threshold and
  reconsider whether re-confirmation can be automated (it probably can, for
  `pre-scan` evidence, by re-running the derivation).
- **If memories are written but never surface in an injection**, the
  anchoring does not match how people actually edit, and the write path is
  the problem, not the read path.

---

## 8. Test obligations

- A memory with no anchor is **refused** with the reason; it is not stored
  at a low tier.
- A memory whose claimed tier exceeds what its evidence supports is stored
  at the ceiling, with the cap and its reason recorded — proven by calling
  the write path with a lying caller.
- Anchor drift moves fresh → suspect at read time, with no sweep, and the
  label names what changed and when.
- Anchor deletion moves suspect → orphaned, and an orphaned memory does
  not appear in an injection.
- Re-confirmation returns a memory to fresh and updates
  `last_confirmed_at`; a model-confirmed memory lands at a lower tier than
  a human-confirmed one, without the caller specifying it.
- A retired memory never surfaces again but is still retrievable by id.
- Injection ordering is deterministic (tier, then specificity, then id) and
  respects the phase-X budget.
- Drift evaluation is cached per index generation: forty surfacings of one
  memory hash its anchors once.

---

## 9. Explicitly out of scope

- Freeform, unanchored, or preference memory (vision 20 X3).
- Cross-machine or cross-user memory sharing (law 13).
- Transcript mining (§5).
- Any memory that surfaces without its tier and state visible. An
  unlabelled memory is a claim fathom is making on its own authority, and
  fathom does not have that authority.
