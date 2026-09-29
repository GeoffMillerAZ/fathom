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
little (and 283 of 510 emitted RC-28 rows are exactly 591 bytes: boilerplate, verified by XM-V1), and
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
  The idle clock's duration is left open on purpose: no reuse data exists yet, so it is set when XM-D1 is
  built, from XM-M3's reuse-gap reading (how long a handed-off map sat before its later mounts), and not
  chosen now.
- **Live maps** (stored queries) compose the same way.
- **Why not a list of ids:** every sub-agent would re-implement merge, dedup and budgeting, and ids could
  not be handed down a level.
- **Deferred (F44):** set algebra over maps (intersection, difference). Promote when a real brief needs
  "base minus what is already done"; drop if union covers every observed handoff for 60 days.

**Storage.** Thread fields in the existing threads store plus the memo registry; no new table (R5). Commands
(sketch, CLI first and on the extended surface only): `fathom thread note --map <run> --anchor <symbol|path#Lx-Ly>
--why '...'` appends entries; `fathom map show <id> --budget 1200` re-resolves every anchor at read time.

**As built (wave 2 core, 2026-09-29).** The model is `internal/explmap` (no I/O, no clock, no model); storage is
two row kinds in the threads log and nothing else (the memo registry is not used): a map header (`map:<id>`,
restated on every change, carrying `completes_when`) and entries (a fresh row id per append, so no append edits
another's entry). `thread note --map` files entries; `map name|include|compose|list` names (purpose required),
includes (DAG checked), composes (the one id) and lists lifetimes. "Pinned while a live map includes it" is
read as pinned while an alive map includes it: a live stored-query map, or any map whose own condition has not
ended it, and a map pinned by an alive map is alive in turn. With no live maps before XM-D1 the narrower
reading would never pin anything, and the reference rule is the same when live maps arrive.

**As built (wave 2 edge, XM-9, XM-10).** `fathom map show ID --budget N` (`cmd/fathom/mapshow.go`) expands the
includes once, re-resolves every entry at read time (the counts cover the whole map, shown or cut) and prints
within `N` bytes, the unit RC-28's 1,200 names: the header and counts lead, the entries follow nearest first, and
what does not fit is one `cut N of M entries` line naming it, stale ones marked; the output never exceeds the
budget. With the index unreachable a drifted symbol entry is flagged stale and the read says the index was out of
reach. An orchestrator passes one id with `map new ID --purpose`, `map append ID --anchor A --why W`,
`map compose ID... --id-only` and `map show`; `usage.md` has the recipe. XM-10's "lives on the extended surface and
the CLI only" is read as a ceiling: no tool is registered. The sketch says CLI first, the threads log is per-user
state resolved from the caller's environment (`FATHOM_THREADS`, `FATHOM_HOME`) that a shared daemon does not see
for an isolated worker, and the surface reckoning asks a measured case before a tool (SR-29 removed a pull-form
tool the hook's push form covered). `TestDefaultSurfaceUnchangedByMap` pins the default at its eight and fails a
map-shaped tool that is not on the extended surface.

**As built (naming, XM-32, 2026-09-29).** "Named" is what makes a map durable, and the lifetime rule now says
so in one place: `Map.Ended` is false for a named map, so `mapLifetime` and the alive test read a named map as
permanent (and as keeping what it includes) whether its filing condition (the session-end default, a
`--completes-when`) held before the name or after it. Before this the rule tested "ended" ahead of "named", and
`map name` restated the header with the first header's `completes_when` carried forward, so a map the planner
named at the end of a session was gone from the next session's brief and resume (O143). Naming does not rewrite
the condition (the threads fold carries a restated header's missing `completes_when` forward from the previous
row, so dropping it would not stick); the header keeps it as a record, the closure row is not written for a named
map, and a map named after its session ended comes back. Unnamed maps end as before.

**As built (anchors, XM-31, 2026-09-29).** A symbol anchor is resolved by `explmap.ResolveAnchorSymbol`, one
function over an index's symbols with `find_symbol`'s ladder (exact, a bare method name, `package.Name`,
case-insensitive; the first tier that matches anything decides), used by the append and by the read-time
re-resolution alike. The entry stores the canonical `dir.Name` key whatever spelling was typed (the typed one stays
as its `anchor`), so one symbol has one key and a Type.Method symbol is no longer cut through its method at the
last dot. Ambiguity is an error that lists candidates, never first-match-wins; an index that could not be reached
(the service unnamed, the daemon warming or down, the store unusable) is its own error with the cause and the
retry, never "does not resolve". The store path lists the service's symbols; the daemon path asks `find_symbol`
for the query and its bare name and applies the same tiers to the answer, which carries no package name, so its
`package.Name` tier reads the directory's tail. Read-time re-resolution of a key filed before this change works
through the same tiers, and `map show` and `map compose` take `--service` as an append does, so a worker in an
isolated clone reads its map through its own scratch store.

**As built (dedupe on append, XM-33, 2026-09-29).** Resolution already merged entries with one dedup key at read
time; the append now checks the map's own entries by that key (a symbol key without its `svc::` prefix, or a
span, plus the content hash) before it files. A repeated why files nothing. A different why is still a row in
the log, since an append adds and never edits, but a row that names the entry it annotates (`annotates`), and the
read folds it into that entry's `Notes` (why, agent id, session, time), so the map holds one entry per anchor and
every hook that quotes an entry count counts it once. Notes serve as annotations after the entry's own why, the
same rule as a farther map's. Included maps are not consulted at append: a base's entry and a top map's entry of
one anchor stay two rows that resolution merges, as XM-4 says. Rows filed before this are not rewritten.

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
Their use as documentation for agents is section 3b.

## 3b. Live maps as documentation for agents (binding frame; built when live maps are promoted, XM-D1)

Owner question 2026-09-29, architect decision. Can live maps replace documentation for agents? The structural
half, yes; the rest, no.

**Frame.** Structure is computed, reasons are written, and each points to the other. Live maps replace the
structural half of documentation (what exists, what connects, where to look), which is the half that rots
because a saved query is recomputed on read. ADRs, intent records, semantic layers and procedures keep the
reasons and the how-to, which no query can derive. A map without its reasons is a list, and a document without
its structure is a claim nobody can check.

**Accepted into the live-map design** (rows XM-17 to XM-19; each needs XM-D1, so none is built before live
maps are promoted):

1. **Maps carry their reasons.** Each live map links the ADRs and rules governing its footprint, a
   deterministic join with obligations by scope, so a handoff delivers structure plus rationale in one id.
2. **Explainable membership.** Each live-map entry's why line is the query clause that selected it: computed,
   never authored. A reader can always see why a file is in the map.
3. **Structural claims become checkable.** A sentence in a doc or an instruction file about structure ("all
   handlers pass through the auth middleware") is compiled into a live-map assertion, reusing the config
   custodian's falsifiable concerns as a new check kind, `structural_claim`
   (`docs/design/config-custodian.md` section 7). When the map's answer changes, the sentence is flagged with
   its `file:line`. The compiled concern is proposed and never written by fathom (vision 22 section 3b), and
   a daemon that is down reads unable-to-check, never a violation.

**Experiments** (rows XM-20 to XM-22, readings XM-M4 and XM-M5; each has a kill path):

4. **Instruction files point to maps.** The custodian proposes replacing structural prose in CLAUDE.md and
   AGENTS.md with named map references, as a diff the owner accepts entry by entry, shrinking instruction files
   toward rules and reasons. Fathom never applies it. Reading XM-M4 is an A/B on identical tasks with the bar
   fixed before the run: instruction-file bytes fall, structural questions are answered as accurately, tokens to
   proof are not worse; otherwise the prose stays.
5. **Map membership diffs as signals.** A structural changelog per named map (the entries that entered or left
   since the last read), and a change that alters a named map's membership shows on the change risk receipt as
   a listed line. Reading XM-M5 counts real structure moves against query noise and asks whether a receipt line
   changed a review or an agent action; the receipt line is dropped when it never did and the changelog stays.

**Deferred** (after live maps exist; trigger "live maps promoted (XM-D1)"):

6. **Human docs rendered from maps** (XM-D7): a deterministic console view first, model narration later, and
   only through a calibrated text-shaped site (R2, R3).
7. **Undocumented-territory coverage** (XM-D8): code covered by no map and no doc, as a computed complement.

**What does not change.** No new default tool (P4, ADR-053), fathom writes only what it owns (law 15), the
dispatch boundary, and R5 (storage stays in existing thread fields until RT-M1 reads PASS). With provider
`none` items 1 to 5 work as written: every one is a deterministic join, clause or comparison. $0 egress.

## 3c. Concern scopes and incident maps (binding frame; built when live maps are promoted, XM-D1)

**Amended 2026-09-29 (ADR-058).** Incident maps are committed declarations, not a map kind; see
`docs/design/incident-maps.md`. That document replaces the incident-map paragraphs below on two points, the kind (a
declaration in `.fathom/incidents.cue`, not a live map) and the gate (its own readings IM-M0, IM-M2 and a named
consumer, not XM-D1 or XM-M2), and it carries rows XM-26 to XM-30, XM-M6 and XM-M7 forward as IM rows. What stays
here: findings are not maps, a concern's scope is a live map (XM-23 to XM-25), a typed model may only order a
suspect list after calibration (XM-D9), fathom never mitigates (XM-X3) and there is no command center (XM-X4).

Owner question 2026-09-29, architect decision. Can live maps carry review concerns, well-architected concerns
and incident readiness? The scope of a concern and the prepared paths of an incident, yes. The findings, no.

**Findings are not maps.** A finding is a point claim with a severity and a lifecycle, under Review Kit's
authority (ADR-040, ADR-055): a verdict on one place. A map is a working set: a purpose and the code that
matters for it. Fathom serves an imported finding beside the governing rules at the edit and never converts,
scores or judges one, and a finding is never stored as a map entry.

**A concern's scope is a live map** (rows XM-23 to XM-25, each needs XM-D1). A review or well-architected concern
("every outbound network call", "every write path to table X", "every flag reader") is a set of code, and
findings are the violations inside it. Review Kit concern packs may declare that scope as a fathom live-map
query carried over the evidence contract (ADR-055 decision 8), so a review always covers the current code and
code that newly enters a concern's scope is caught, where a fixed file list goes stale. Fathom's side is an
optional `scope` field on the evidence contract, a query in the layers' vocabulary that fathom evaluates and
serves as a map whose entries carry the clause that selected them (section 3b, item 2). Review Kit's side is a
future on its own plan; a pack with no scope field works exactly as it does today, and Review Kit keeps its
fathom-less path. Fathom computes membership. It does not choose which concerns matter or decide whether the
code inside one conforms (future F52).

**Well-architected concerns become continuous.** A concern map plus a check turns a periodic review into a
standing property. The map says which code the concern covers, the check (a guard, or an assertion evaluated
whenever the map is served, the same shape as section 3b's `structural_claim`) says what must hold there, and
both are recomputed on read instead of re-reviewed on a calendar. A daemon that is down reads unable-to-check,
never a violation.

**Incident maps** (superseded by the amendment above: originally rows XM-26 to XM-28, and the form future F4 was
then thought to take). An incident map was specified as a live map prepared ahead for a critical path, and each one
joins:

- owners (the ownership file);
- the recent changes inside its footprint, the suspect list: commits in the last N hours or days, N set on the
  map;
- the covering tests and their certificates, and recent failures;
- the flags and configuration the footprint reads, the knobs;
- linked findings and past incidents (`docs/intent/incidents.md` records that name the path);
- runbook and dashboard links, stored as data and never fetched.

**The incident flow.** During an incident, `localize` given a stack trace or an error maps its frames to the
incident maps that contain those symbols and returns the suspect list, the reach, the owners and the knobs. It
is computed locally, with no model and no network: a runbook link is shown, never opened. The default order is
deterministic. A typed judgment model has one optional role, ordering the suspect list after a calibration
reading, and never decides membership (section 6; XM-D9).

**Boundaries.** There is no incident command center: the non-goal in `docs/intent/scope.md` stands, and a map is
served data, not a workspace, a chat or a timeline anyone edits during the incident. Fathom never mitigates: it
never toggles a flag, rolls back, restarts, pages or opens a ticket. The knobs are listed so a human, or an agent
holding its own authority, can act. The flow is read-only (XM-28), inside the dispatch boundary and law 15.

**Experiments** (originally rows XM-29 and XM-30 and readings XM-M6 and XM-M7, now IM-16, IM-14, IM-M3 and IM-M4 in
`docs/plans/incident-maps.md`; each has a kill path):

1. **Postmortems update the map.** An incident's outcome names the incident map it created or sharpened for the
   affected path, extending the rule in `docs/intent/incidents.md` (an invariant, a test or a decision) with the
   map, once incident maps exist. That extension is planned and marked in the rule text, not in force. Reading
   XM-M6 asks, on incidents over mapped paths, whether `localize` named the map and whether its suspect list held
   the change that proved to be the cause. The bar is fixed before the run.
2. **A readiness report per critical incident map.** Owners present, runbook link present, live certificates,
   flags present, change volume. Review Kit owns the standard of what ready means; fathom computes the facts and
   grades nothing. Reading XM-M7 asks whether a gap the report named was later confirmed by a real incident or
   review and closed by an owner. The bar is fixed before the run.

**What does not change.** No new default tool (P4, ADR-053), fathom writes only what it owns (law 15), R5
(storage stays in existing thread fields until RT-M1 reads PASS). With provider `none` everything above works as
written: every item is a deterministic join, clause or comparison. $0 egress; links are data, never fetched.

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

**As built (hooks, 2026-09-29; XM-8, XM-11 to XM-13).** `cmd/fathom/maphooks.go`; all four channels are deterministic
with provider none, need no daemon, and cost one file read in a repository whose threads log holds no map row.

- *Handoff.* The SubagentStart input carries no prompt (XM-V1), so the id is chosen from the parent session's state: its
  alive, non-composite maps whose header or any entry carries the parent's session id, minus any map another of them
  includes. One map is handed as it is; several become a minted composite over at most six of them (most recently used
  first, then taken in id order so the composite's id does not move when a different map was used last), stored once
  with `completes_when` the parent session's end so it does not pin them past it. The map line leads the discoveries chunk
  and carries the read command, the sub-agent's own id for `--agent` (the payload is the only place fathom sees it, so the
  codec now exposes `HookInput.AgentID`) and the purpose, which yields bytes first; the brief is rendered to whole pointer
  lines inside what the base primer leaves, and only a base too long to leave the id its room is trimmed. The id survives
  RC-28's 1,200-byte cap by construction; the reading measured 28 of 325 briefed payloads cut at the cap.
- *Relevance by location.* A footprint is the map's own entries: a touched file, a file under a touched directory, or a
  touched symbol (the prompt's paths, packages and bare symbols come from the same extraction the obligations half of the
  prompt hook uses; the edit hook's is the edited file). At most two lines a firing, once per session per map, kept as a
  per-(session, map) marker under `$TMPDIR/fathom-map-pointer`, the mechanism the Bash-write hook's doubt marker uses and
  `fathom gc` prunes; this is bookkeeping about what was said, not map storage. Freshness in the line is a file re-hash of
  the map's own entries (capped at 60), with no index and no git, so a moved symbol reads as changed and `map show`
  is what re-resolves and sizes it. No session id, no line: a line that cannot be once-only is not sent.
- *Session brief.* At most three bullets ranked by being named by an open note of the thread, then by overlap with the
  thread's recent files (the working tree's changed files and the file pointers its open notes carry), then by recent
  use; the rest are counted. Composites are never named, and a map that has ended is not alive.
- *Threads.* `thread note --map-ref` names maps a note references (each must exist); the pre-compact pointer carries the
  alive maps the session filed or added to; the brief and the handoff show them as `[maps …]`. "Re-mounts" is read as
  re-surfaced: a fresh session's brief (startup and clear, as ever) names them; nothing is injected at compact.
- *Not built.* The pointer's "shown" count is not recorded anywhere, so XM-M3's precision has no denominator yet; the
  reading adds the beat when it runs. The hint each channel gives names `fathom map show <id> --budget 1200` (XM-9).

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
channel. The parenthesis is kept true by F50's trigger and XM-D1's: RT-M1 PASS and either XM-M2 PASS or a
named consumer of a non-handoff use case with its own bench metric, so an XM-M1 kill, after which XM-M2 never
runs, does not close the route to live maps (their value in concern scopes, incident maps, structural claims
and saved questions does not depend on sub-agent re-derivation; P20 still demands the named consumer).

**Open facts, verified (Q32, XM-V1, `docs/bench/readings/exploration-maps/handoff-facts.md`):** the
SubagentStart input does not carry the sub-agent's prompt, so choosing entries by relevance inside the hook is
impossible and the handoff carries the id the parent filed; RC-28's 591-byte rows are the constant base primer
with no discoveries brief (549 of 874 emitted rows over 15 days carried none).

## 6. What a typed judgment model does here (optional, and mostly nothing)

The planner already decided what matters, so a model is mostly unnecessary. Optional roles, none required for
any map to work: drafting a purpose line (an llm, human-committed); computing an expensive judged column only
inside a map's footprint (after J0 and RT-M1); selecting entries for a task on a map over budget
(`map show --for '<task>'`, one noul per entry, deferred until the hook input carries the prompt); ordering an
incident map's suspect list (section 3c, XM-D9, deferred until a calibration reading); and a
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
| XM-M4 | the instruction-file swap A/B (section 3b, item 4): bytes fall, structural answers as accurate, tokens to proof not worse, bar fixed before the run; otherwise the prose stays |
| XM-M5 | membership signals (section 3b, item 5): structure moves against query noise, and whether a receipt line changed an action; the receipt line is dropped when it never did |
| XM-M6 | postmortem map updates (section 3c, experiment 1): on incidents over mapped paths, whether `localize` named the map and its suspect list held the cause; bar fixed before the run (moved to IM-M3, ADR-058) |
| XM-M7 | the readiness report (section 3c, experiment 2): whether a named gap was confirmed by a real incident or review and closed; bar fixed before the run (moved to IM-M4, ADR-058) |
