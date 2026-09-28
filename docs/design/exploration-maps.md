# Exploration maps: shared from planner to sub-agents, composable by reference, delivered rather than looked up

Status: accepted 2026-09-29 (ADR-056) as an experiment, measurement first: the design below is binding for
how a map is built if the reading says it is worth building. Vision:
`docs/vision/40-judgment-and-exploration-maps.md`. Plan: `docs/plans/exploration-maps.md` (XM-). Goal
G30. Builds on continuity threads (`docs/design/continuity-threads.md`, `docs/vision/21`), receipts
(RC-28, the SubagentStart handoff), semantic layers (ADR-016) and the join rule (ADR-051 decision 1).
Constraints carried in: the dispatch boundary (fathom stores and serves, never spawns, schedules or
retries), P16 (delivery is configuration, not agent habit), P4 and ADR-053 (no new default tool), law 15
(fathom writes only what it owns), R5 (no new table before RT-M1).

## 1. Problem, and what already exists

An orchestrating agent explores a codebase, decides what matters, and writes a brief for a sub-agent. The
sub-agent then re-derives part of that exploration. The handoff path already exists: SubagentStart injects
the parent session's discoveries since its start (RC-28, `subagentDiscoveries`, a 1,200-byte cap; 584
SubagentStart events in 4 days) and threads already give lifetimes (`completes_when`). What is missing is
an anchored, fresh, shareable record of which code matters for a purpose.

**Why it could succeed where discoveries and anchored memory failed.** Both writing and delivery happen at
forced moments. SubagentStart fires for every sub-agent and the planner writes a brief anyway. Threads show
agents file at explicit moments (452 handoff notes, 550 automatic pre-compact pointers), while tools an
agent had to choose to use died (discoveries 13 calls; anchored memory 0 memories and deleted).

**Why it might still fail.** If RC-28's pointer injection already stops re-derivation, the extra lines add
little (and 283 of 510 emitted RC-28 rows are exactly 591 bytes, inferred to be boilerplate, to verify), and
many sub-agent reads are reads in order to edit, which are not waste. So measure first (section 5).

## 2. The map model (binding)

A map holds its own anchored entries (a symbol key or `path#Lx-Ly`, plus a content hash, plus a why line)
and `includes` references to other map ids. Entries are never copied.

- **A DAG.** Shared bases are allowed, cycles are refused, depth is capped at 4 so expansion stays bounded.
- **One id at handoff.** The brief always carries one id. When a caller wants several maps, fathom mints a
  tiny composite map made only of references, and the brief carries that id. Budgeting and ranking happen
  once, at expansion. A sub-agent can pass its own composite to its sub-agents.
- **Resolution** is a union deduplicated by anchor (symbol or span plus content hash). Conflicting
  annotations: the nearest include wins and both are kept with provenance. Freshness is checked once per
  entry however many maps include it: a drifted entry is re-resolved by symbol key or flagged stale with its
  diff size, never served silently wrong.
- **Lifetimes follow references.** An ephemeral map is pinned while a live map includes it; its idle clock
  restarts when the last reference drops. Sub-agents append entries with provenance (agent id). An entry
  becomes permanent only after measured reuse across at least 3 runs, then becomes a concern-pack entry.
- **Live maps** (stored queries) compose the same way.
- **Why not a list of ids:** every sub-agent would re-implement merge, dedup and budgeting, and ids could
  not be handed down a level.
- **Deferred (F44):** set algebra over maps (intersection, difference). Promote when a real brief needs
  "base minus what is already done"; drop if union covers every observed handoff for 60 days.

**Storage.** Thread fields in the existing threads store plus the memo registry; no new table (R5). Commands
(sketch, CLI first and on the extended surface only): `fathom thread note --map <run> --anchor <symbol|path#Lx-Ly>
--why '...'` appends entries; `fathom map show <id> --budget 1200` re-resolves every anchor at read time.

## 3. Live maps and semantic layers are complementary (binding)

They live on different axes. A semantic layer says what is TRUE or BINDING about code (governed claims:
rules, bindings, labels, and the proposed self-invalidating semantic columns). A live map says which part of
the code MATTERS for a purpose (a saved, re-run query: a working set). Layers are the vocabulary; live maps
are saved questions written in that vocabulary. The layer side of this pairing is
`docs/design/semantic-layers.md` section 4.9.

1. Live maps query layers ("handlers where 'validates input' is no", "code governed by ADR-048 rules and
   touched by the language plan"), so maps get sharper as layers grow.
2. Maps scope layers: an expensive column (a model judgment) is computed only inside the maps that need it,
   not across the whole repository. That is the cost control for judged columns
   (`docs/design/judged-predicates.md` section 5).
3. Graduation: a live map reused by many agents or sessions becomes a named concern in a layer, and layer
   claims gain "used by map X" signals that feed rule effectiveness.
4. One freshness machine: both are invalidated by the same content-hash drift.

**Why durable live maps are useful:** they are saved questions that stay correct as code changes, unlike a
wiki diagram or a document list. Examples: the auth surface (handlers, middleware, policies) for a security
review; the payment path from entry point to data store; release blockers (claimed-unproven plan rows for
the next release intersected with touched code); the public API without a live certificate; a migration's
remaining sites; the hot zone (high debt interest intersected with churn); everything that reads an
environment variable or flag; the code footprint of an active program that every worker on it mounts;
dead-code candidates; the product's core loop for onboarding.

**Lifetime.** A live map is permanent only when named and used; idle unnamed maps decay. Layers keep their
own freshness and lifecycle (candidate, active, retired). Live maps are the designed extension (future F50; rows XM-D1, XM-D4, XM-D5); they are
built after the handoff A/B passes, and their storage stays in existing thread fields until RT-M1 reads PASS.

## 4. Discoverability: maps arrive, agents do not have to look (binding)

Discoveries and anchored memory failed because they depended on agents choosing to look. Fathom's founding
rule is delivery by configuration. Four channels:

1. **Handoff.** A brief's map id is expanded by the session hook within budget.
2. **Relevance by location.** When a prompt or edit touches code inside a map's anchored footprint, the
   prompt or edit hook injects one line naming the map, its purpose, size and freshness and how to expand
   it; deduplicated per session, budgeted, like obligations at the edit.
3. **Session brief.** At most 3 maps, by recent use and overlap with the thread's recent files; never a full
   list.
4. **Continuity threads.** Thread notes may reference maps, the pre-compact pointer carries active map ids,
   and resuming a thread re-mounts its maps. (This carries no working set injection at compact; that is a
   separate experiment, `docs/design/carry-across.md`.)

**Descriptions** follow MCP tool-description practice: a required purpose line (the question the map
answers; a named map without one is refused), when to use, scope summary, live freshness counts, creator and
last used. Fathom computes scope, freshness and usage (XM-15); the creator writes the purpose (a cheap model may
draft it, optional). **Surface:** no new default tool; `map` on the extended surface and the CLI; reminders
are injected context. **Measure:** pointer precision (expanded over shown) and reuse rate (mounted after
handoff); pointers expanded under 10 percent are cut.

## 5. The experiment (E2): measurement first

**Step 1, a bench script over 14 days of transcripts (zero product build).** Measure the share of sub-agent
Read calls that hit files the parent read or edited before spawning, split into reads followed by an edit
(legitimate) and reads not followed by an edit (re-derivation). Discovery read rows carry no path or agent
id, so this must come from a transcript scan, run as bench code.

**Step 2, only if re-derivation is at least 15 percent of sub-agent reads.** Build the smallest slice:
anchored entries on threads with `completes_when`; RC-28 carries one map id within its 1,200-byte cap;
anchors are re-resolved at read time and stale entries flagged; then an A/B on identical briefs.

**Bar:** at least 25 percent fewer sub-agent tokens, no accuracy loss, under 5 percent stale entries (the
owner's proof bar). **Kill:** re-derivation below 15 percent means no handoff maps (durable live maps, an
owner concept outside this experiment, are unaffected); if the A/B misses its bar, remove the handoff
channel.

**Open facts to verify (Q32):** whether the SubagentStart input carries the sub-agent's prompt (if not,
choosing entries by relevance inside the hook is impossible), and whether RC-28's 591-byte rows are
boilerplate.

## 6. What a typed judgment model does here (optional, and mostly nothing)

The planner already decided what matters, so a model is mostly unnecessary. Optional roles, none required for
any map to work: drafting a purpose line (an llm, human-committed); computing an expensive judged column only
inside a map's footprint (after J0 and RT-M1); selecting entries for a task on a map over budget
(`map show --for '<task>'`, one noul per entry, deferred until the hook input carries the prompt); and a
planner's question-directed traversal becoming a map (`docs/design/judged-predicates.md` section 4, F36,
triggered by XM-M2 passing). With provider `none` every map feature works.

## 7. Cost and privacy

$0 egress. The tenancy cost is the thread-field bytes. Nothing leaves the machine; maps hold anchors and
why lines, never source text beyond spans a reader could already read.

## 8. Readings

| Reading | Threshold |
|---|---|
| XM-M1 | re-derivation share of sub-agent reads over 14 days; below 15 percent kills handoff maps |
| XM-M2 | the A/B on identical briefs: at least 25 percent fewer sub-agent tokens, no accuracy loss, under 5 percent stale entries |
| XM-M3 | pointer precision at least 10 percent (expanded over shown) and the reuse rate reported |
