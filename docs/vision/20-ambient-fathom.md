# 20 — Ambient fathom: the dial at eleven, gated

> Owner ask, 2026-08-08: *"take the UI and dashboards to even higher levels
> of excellence… find ways to gather the impact fathom is having and tell
> that story… keep fathom always useful and wired into projects… maybe it
> can execute what needs done on your behalf… is fathom a good platform for
> memories… can we help keep subagent context concise… what is the ultimate
> potential… ask the questions I haven't asked, gate the things I've asked
> for that don't belong. Don't let fathom allow crap."*
>
> This document is the answer to the last sentence first. It states what
> fathom becomes, what it refuses to become, and which of the asks above I
> am **accepting, reframing, or rejecting** — with the reason attached to
> each, so a future reader can overturn a call by attacking its reason
> rather than re-litigating the whole thing.

---

## 1. The thesis, restated in one paragraph

Fathom's current identity is *a shared index that coding agents query
instead of grepping*. That is a retrieval product, and retrieval products
have a ceiling: they get faster and more accurate until the model's context
window makes the speed irrelevant. The next identity is **the memory and
judgement layer of a codebase** — the thing that knows what is true here,
how confident it is, why, when that stopped being true, and what rule
governs the file you are about to edit. Retrieval is one verb of that
system. The ceiling of *this* identity is much higher, because a bigger
context window makes it **more** valuable, not less: when a model can read
anything, the scarce good stops being access and becomes *knowing what is
worth reading and what is still true*.

Everything below is downstream of that sentence.

---

## 2. The existential question nobody asked

**If context windows keep growing, does fathom stop mattering?**

This deserves the first section because it is the only question that can
make the entire program worthless, and the program has never written down
an answer.

The answer is no, and the reason is that fathom's competition was never the
window size — it was **grep plus the agent's willingness to read**. Three
things get *worse* as windows grow, not better:

1. **Filling the window has a price.** A 1M-token window that an agent
   fills with 400k tokens of speculative file reads costs more per turn
   than a 200k window used carefully. Bigger windows raise the ceiling on
   waste. The exploration ledger (vision 19) measures exactly this.
2. **Recall degrades with irrelevant context.** More retrieved text is not
   more knowledge; past some point it is less. Precision beats volume, and
   precision is a ranking problem — which is a fathom problem.
3. **Truth is not in the window at all.** "This function is deprecated,"
   "this pattern was rejected in ADR-0044," "this file is governed by a
   rule you must not break," "we tried that in March and it broke
   production" — none of it is derivable from reading the code, at any
   window size. It has to be *stored, anchored, and expired*.

Point 3 is where fathom's durable value lives, and it is the least-built
part of the product. **That is the strategic finding of this document.**

**Consequence:** we stop investing marginal effort in *faster retrieval*
and start investing it in *anchored, expiring, evidence-tiered knowledge* —
which is exactly what §5 (memory) specifies.

---

## 3. Where we are genuinely weak

Written plainly, because a roadmap built on flattery is worthless.

| Weakness | Evidence | Severity |
|---|---|---|
| **No outcome measurement, only exposure** | 6,992 rule injections, 17,727 discovery actions, and **zero** measured mistakes prevented. The A/B protocol was pre-registered and never run. | **Critical** — every value claim rests on it |
| **The fleet decays silently** | 9 of 17 repositories below grade A; the remedy has existed for weeks and nobody ran it. Advice that requires a human to type a command is advice that does not happen. | **Critical** — this is the wiring flywheel failing |
| **Knowledge has no expiry** | Layers, obligations and modules are compiled once and trusted until a human recompiles. A rule about a function that was deleted last week still fires. | **High** |
| **One user, one machine** | Every instrument is calibrated to a single-person fleet on loopback. Nothing has been tested with a second writer, a second machine, or a repository someone else owns. | **High** — it is the whole adoption story |
| **Purge is impossible** | If a secret, a vendored corpus, or a customer file gets indexed, there is no supported way to remove it and prove it is gone. `.fathomignore` prevents future ingestion; it does not delete. | **High** — a correctness *and* a trust problem |
| **The index cannot be wrong on the record** | `docs/dogfood-observations.md` collects tool defects, but a *wrong answer* — a stale symbol, a bad edge — has no correction loop that feeds back into ranking. | **Medium** |
| **Extension is ad hoc** | Four extension seams exist (expert modules, layer packs, guards, MCP profiles). None of them appear in one list, none share a toggle vocabulary, and `doctor` cannot say why a given capability is off. | **Medium** |
| **No offline story** | Daemon down means every hook degrades. It degrades *gracefully*, which is correct, but there is no measurement of how often it happens. | **Low** |

---

## 4. The gate — accepted, reframed, rejected

The owner asked me to gate. Here is the gate.

### ACCEPTED as asked

**A1 — Execute remedies on the user's behalf, from the UI.** Correct, and
the single highest-leverage item in this document. Advice-that-requires-
typing is the reason 9 of 17 repositories are below grade A. Specified in
`docs/design/self-healing-fleet.md`, with the non-destructive constraint
promoted to a law (§6, law 15).

**A2 — Path suppression and purge, from the UI.** Correct and currently
impossible. Suppression and purge are *different operations* and the design
keeps them separate: suppression is reversible and instant, purge is
irreversible and must be proven. Specified in
`docs/design/scope-and-purge.md`.

**A3 — Per-project analytics as deep as the fleet's.** Correct. The fleet
view answers "which repository needs attention"; the project view must
answer "what is fathom doing *here*, and what is it missing here". Both
already share one model, so this is depth, not a new system.

**A4 — Global and per-project feature toggles.** Correct, and a
precondition for everything else in this document. Specified in
`docs/design/capability-registry.md`. It obeys law 13: user scope may set
defaults, only project scope may govern.

**A5 — Track which projects use fathom optimally.** Correct, and the grade
already exists — but it grades *wiring* only. It must grade **realization**:
wired, fresh, queried, preventing, gating. A repository can be perfectly
wired and deliver nothing.

### REFRAMED (accepted in substance, rejected in the form asked)

**R1 — "Can fathom keep subagent context concise, like the caveman skill?"**

Half yes. Split it:

- **Not ours:** how an agent *phrases* its output. Prose compression is a
  skill, skills already exist, and a code-intelligence engine that starts
  rewriting an agent's English is out of its lane. **Rejected.**
- **Ours, and badly neglected:** the size of what *fathom itself* returns.
  Fathom emits tool results into the agent's context on every call, plus
  11,839 bytes of standing tool schema before any work happens. That is
  fathom's own footprint and it is entirely fathom's to control.

Reframed as **budgeted answers**: every fathom tool accepts a token budget,
returns the highest-value subset within it, and states plainly what it
elided and how to ask for more. An answer that silently truncates is a lie;
an answer that says "12 of 47 callers shown, ordered by call depth; ask with
`limit: 50` for the rest" is a contract. Specified in
`docs/design/budgeted-answers.md`. This also makes fathom's footprint a
*measurable* quantity, which feeds the tokenomics instruments already built.

**R2 — "A plugin system for optional capabilities."**

Rejected as a new subsystem; accepted as an inventory. A plugin API is how
good tools become junk drawers — it converts every future quality decision
into someone else's problem while keeping the blame. Fathom already has
**four** extension seams, all typed, all CUE-validated:

| Seam | Extends | Ships as |
|---|---|---|
| Expert modules | domain knowledge answerable by query | `.fathom/modules.cue` binding |
| Layer packs | semantic layers over a corpus | `fathom layers install` |
| Guards | pre-edit / pre-command guidance | `.fathom/guard.cue` |
| MCP profiles | which tools a surface exposes | server config |

The correct work is not a fifth seam. It is: **put all four in one
registry, give them one toggle vocabulary, make `doctor` able to say why
any capability is off, and refuse a fifth seam unless it can prove none of
the four fits.** Specified in `docs/design/capability-registry.md`.

**R3 — "Is fathom a good platform for memories?"**

For *general* memory — "Geoff prefers tabs", "we use Linear" — no, and it
should refuse. A file does that better and fathom would be a worse text
editor with extra steps.

For **anchored memory** — a claim bound to a symbol, a file, a path or a
commit — fathom is the best-positioned system I know of, for one reason
nobody else has: **it can detect when the thing a memory is about has
changed, and expire the memory automatically.** Every memory system in
common use is append-only and rots silently. Fathom already owns symbol
identity, commit drift, staleness detection and evidence tiers — every
primitive an expiring memory needs, already built and already tested.

Accepted in that form only. Specified in `docs/design/anchored-memory.md`.
The refusal is load-bearing: a memory with no anchor and no evidence is a
prompt, and law 14 caps a prompt at the bottom tier or refuses it outright.

### REJECTED

**X1 — Auto-apply of remedies without a human action.** The daemon must
never rewrite a repository's configuration on a timer or on its own
judgement. A background process that edits files in repositories it was
never invited into is indistinguishable from malware, and one bad
reconciliation across 17 repositories is unrecoverable in a way one bad
reconciliation in one repository is not. **The button is the consent.**
Dry-run always precedes apply; apply always writes a receipt; a receipt
always supports undo.

**X2 — Projections of what fathom "would" save.** The brief already refuses
these and the dashboards must too. "Enabling this would save ~30%" is the
single easiest way to make every honest number on the page untrustworthy by
association. The permitted form is: *this is unmeasured here, and this is
the one command that measures it.*

**X3 — A general note store, a wiki, or freeform memory.** See R3.

**X4 — Hosted or cross-machine sharing of any observability surface.**
Unchanged from law 13. Sharing is a human verb; `fathom brief` writes a
local file and stops.

**X5 — Silent mutation of another tool's configuration.** Fathom writes
only inside regions it owns and marks. If a fathom-owned region has been
edited by hand, reconciliation **stops and reports** rather than resolving.
This becomes law 15.

---

## 5. What the product becomes

Four properties, in the order they should be built.

### 5.1 Self-healing (the wiring flywheel closes)

Today: fathom *detects* drift and *prints a command*. The command does not
get run, so the fleet decays and the impact numbers understate reality
because badly-wired repositories sit outside the denominator.

Tomorrow: the observatory shows drift **with the diff already computed**
and a button that applies it, non-destructively, with a receipt and an
undo. The fleet page stops being a report and becomes a control surface.
The measurable claim: *time-to-remedy*, currently unbounded, becomes a
number on the dashboard.

### 5.2 Self-cleaning (scope becomes a first-class, reversible thing)

Today: what gets indexed is decided once, by a file, at ingest time. There
is no way to say "not this path, for now", and no way to remove something
that should never have been there.

Tomorrow: **suppression** (reversible, instant, optionally time-boxed,
query-time) and **purge** (irreversible, provable, storage-level) are
separate verbs, both available from the CLI and the UI, both audited.

Purge writes an **ingest-time exclusion** as well as deleting, because a
removal that the next auto-refresh undoes is not a removal — and it is not
complete until a proof *independent of the deletion* shows zero remaining:
a per-location scan derived from each table's own path column, plus a
raw-bytes probe of the store file. Both corrections came from adversarial
review of the design (2026-08-08), which reproduced the residue against
this repo's own store.

### 5.3 Self-expiring (knowledge stops rotting)

Today: a compiled rule outlives the code it describes. Nothing decays.

Tomorrow: every stored claim — obligation, memory, module answer, finding —
carries an **anchor**, and the anchor's drift determines the claim's state:
`fresh` → `suspect` (anchor moved) → `orphaned` (anchor gone). Suspect
claims still surface, but *labeled*, and law 14's ceiling applies. Orphaned
claims stop surfacing and appear on a review queue. This is §2's point 3
made operational and it is the highest-ceiling item in the document.

### 5.4 Self-accounting (fathom's own cost is a first-class metric)

Today: fathom measures what agents do. It measures its own footprint in
exactly one place (the 11,839-byte schema tax on the brief).

Tomorrow: every tool result carries its token cost, every answer respects a
budget, every elision is stated, and the ratio *value returned / tokens
spent* is a number per tool on the dashboard. A tool whose ratio is bad is
a tool we fix or delete. This is the mechanism that keeps the surface from
bloating back to 40 tools.

---

## 6. Law 15 — fathom writes only what it owns, and never resolves a conflict

> When fathom modifies a file it did not create, it writes only inside a
> region it marked as its own. If that region has drifted from what fathom
> last wrote, reconciliation **stops and reports the drift**; it never
> merges, never overwrites, and never "helpfully" resolves. Every write
> produces a receipt sufficient to undo it.

The reason: fathom's writes land in `CLAUDE.md`, `AGENTS.md`,
`lefthook.yml`, `.claude/settings.json` — files that carry a human's own
configuration and other tools' configuration alongside fathom's. The cost
of a false stop is a human reading a diff. The cost of a false merge is
someone's settings silently destroyed across seventeen repositories at
once. Those are not comparable, so the rule is not a judgement call.

---

## 7. The questions I am raising that were not asked

1. **Who is the second user, and what breaks?** Every instrument assumes
   one person, one machine, one loopback daemon, one writer. The first
   colleague to adopt fathom will hit single-writer contention, path
   assumptions, and a session ledger keyed on a home directory. *This is
   worth one deliberate hostile test before it is worth another feature.*

2. **What is the threat model?** Fathom indexes source across every
   repository on the machine and serves it, unauthenticated, on loopback,
   to any MCP client that connects. Any process on the machine can read the
   whole fleet's source through it. That may be entirely acceptable for a
   single-user workstation — but it has never been *written down and
   accepted*, and it must be before anyone else adopts it.

3. **When the index is wrong, what corrects it?** Findings-as-evidence
   collects defects in fathom's *tools*. Nothing collects "this answer was
   wrong" and feeds it back into ranking. A retrieval system with no
   negative-feedback loop cannot improve past its initial heuristics.

4. **What is the smallest useful fathom?** Everything is designed for the
   17-repository fleet. A single-repository developer with no daemon and no
   launchd is the actual median adopter, and no one has ever tried to be
   them. *The onboarding defects of the last two weeks (O22, O23, the hook
   duplication, the CUE `?:` inversion) were all one root cause: the tool
   was only ever dogfooded in its own repository.*

5. **What do we delete?** Growth has been monotonic. Nothing has been
   removed. The budgeted-answer ratio in §5.4 is the first instrument that
   can *justify* a deletion, and the surface should shrink at least once
   before it grows again.

---

## 8. Sequence

Ordered by leverage, not by size. Each phase has a terminal condition and a
kill condition, per house style.

| Phase | Ships | Terminal | Kill |
|---|---|---|---|
| **T — Capability registry** | one typed registry of the four seams, global + project toggles, `fathom capabilities`, doctor integration | `doctor` can state why any capability is off, in one line, in a repository that has never been opened before | a capability that cannot be toggled, or a fifth seam added without proving the four don't fit |
| **U — Self-healing fleet** | law 15 merge engine, receipts, `fathom fleet undo`, UI apply-with-diff | a below-A repository is raised to A from the browser, with a visible diff, and undone cleanly | any reconciliation that resolves a conflict instead of stopping |
| **V — Scope and purge** | suppression (reversible, time-boxable) and purge (irreversible, proven) across store, vector, cache **and the store file's own free pages and WAL**; UI for both | a deliberately-indexed secret is purged, and **after a forced full re-ingest** the verification still proves zero — including a raw-bytes probe of the store file | purge that reports success without a proof **independent of the deletion**: a re-query cannot see freed pages, the WAL, or rows anchored outside the resolved ref set |
| **W — Anchored memory** | anchor types, drift → fresh/suspect/orphaned, review queue, surfacing through obligations, law 14 ceilings | a memory about a renamed function is automatically marked suspect and says why | a memory accepted without an anchor or without evidence |
| **X — Budgeted answers** | per-tool budgets, explicit elision, per-tool cost/value ratio on the dashboard | every agent-surface tool respects a budget and states what it elided | any tool that truncates silently |
| **Y — Realization grade** | grade extended from wiring to wired/fresh/queried/preventing/gating; per-project depth | the fleet page ranks repositories by value realized against value available | a grade that rewards configuration rather than outcome |

Phase T is first because every other phase needs a toggle. Phase U is
second because it is the one that stops the fleet decaying while the rest
is built.

---

## 9. What would make me wrong

Stated up front so the program can be falsified rather than defended:

- If the A/B pilot runs and shows fathom-on sessions are **not** cheaper or
  better, §5.4's ratio work is the only part of this document that
  survives, and the retrieval story should be retracted where it appears.
- If anchored memory ships and the suspect/orphaned queue is empty after a
  month of real churn, the anchoring is too coarse and should be rebuilt at
  symbol-body granularity or abandoned.
- If self-healing ships and reconciliation stops-and-reports more often
  than it applies, law 15 is too strict in practice and needs a narrower
  ownership region — not a looser merge.
