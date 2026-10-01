# Project Intent — durable intent for agentic engineers

A repository practice that gives coding agents (and humans) the product intent, domain
truths, architectural reasoning, future awareness, and decision history they need to act
like responsible long-term owners of a project, without loading any of it into a session
that does not need it.

It lives in `docs/intent/` with a router at `docs/intent/README.md`. The procedure is
the `intent-*` skill family (`.claude/skills/intent-*/`, templates in `intent-init/`).
Tool wiring is in [integration.md](integration.md); worked examples, including what
*not* to record, are in [examples.md](examples.md). Change records (commits, changelog,
breaking changes, the release ritual) are the sibling layer in [changes.md](changes.md):
intent says why, change records say what changed and what it costs to follow, and the
two link rather than repeat. To adopt the practice in another repo, paste
[adopt-prompt.md](adopt-prompt.md) into that repo's agent session.

## 1. Why it exists

An Architecture Decision Record answers "why did we make this architectural choice".
An agent acting as product owner, designer, architect, and steward needs more: why the
project exists, what it should feel like, what must never break, what is deliberately
out of scope, what may come later, which decisions are settled, which alternatives lost
and why, and, above all, **what would make an old decision worth reopening**.

Without that, agents fail in two symmetric ways. They optimize the requirement in front
of them and casually wall off a future the humans were counting on. Or they hedge
against every imaginable future with abstractions nobody asked for. Both come from the
same gap: nobody wrote down which futures are real and what keeping them open costs.

## 2. The governing principle

> **Build for now. Foreclose nothing documented when keeping it open is free.**

Three moves an agent can make about the future, and the verdict on each:

| Move | Meaning | Verdict |
|---|---|---|
| **Build** | implement the future capability now | no (YAGNI) |
| **Prepare** | pay complexity now so the future is easier | no, almost always (YAGNI in a suit) |
| **Preserve** | pick the equally simple option that does not wall the future off | yes, when free |

"Free" is a three-part test, and all three parts must hold:

1. The future is written in `futures.md` with confidence *likely* or *plausible*.
   Speculative futures never justify anything.
2. The preserving choice is no more complex *today*: no extra abstraction, code path,
   interface, configuration, or indirection.
3. The alternative would make the future **destructive** to add later (data migration,
   API break, rewrite), not merely laborious.

If any part fails, build the simple thing. The classic example: one household per
deployment today, hosted multi-tenancy plausible later. Do not build tenants, tenant
auth, tenant billing, or a tenant abstraction. Do avoid a design that makes a tenant
boundary unnecessarily destructive to introduce when an equally simple one exists.

## 3. The model

Two kinds of content, deliberately different in how they change:

**Living documents** describe the present. They are edited in place, carry no dates or
owners, and git is their history. A living doc that reads like a journal is broken.

**Records** describe a moment. A decision record is appended, given a status, and later
superseded, never rewritten in substance.

| File | Answers | Changes |
|---|---|---|
| `README.md` | where is what, what to read for which task, when to update | rarely |
| `vision.md` | why this exists, the core insight, what it should feel like, what it is not | rarely |
| `principles.md` | how to resolve tradeoffs; the tensions that genuinely pull both ways | rarely |
| `invariants.md` | what must always or never be true, and how each is enforced | occasionally |
| `domain.md` | vocabulary, domain truths, platform facts, assumptions and what breaks if false | occasionally |
| `scope.md` | current goals with exit tests, current boundaries, non-goals | often |
| `futures.md` | plausible directions: confidence, horizon, do-not-build, preserve, promote-when, drop-if | occasionally |
| `questions.md` | what we do not know that changes design | often |
| `autonomy.md` | what an agent may do unasked, must ask first, never does; the branch model | rarely |
| `incidents.md` | what broke, what would have caught it, which invariant/test/decision it produced | append, then compress |
| `repo.md` (multi-product repos only) | why one repository, what belongs, the component map with split/merge/retire triggers, dependency direction ([monorepo.md](monorepo.md)) | rarely |
| `decisions/` | records: context, decision, alternatives, consequences, revisit triggers | append |
| `CHANGELOG.md` (repo root) | what changed for users since the last release ([changes.md](changes.md)) | per observable change |

Nine living documents plus records, and the changelog beside them. This is the ceiling, not the floor: a doc with no
evidence-backed content is omitted, and a repo that already has a good `VISION.md` or
`docs/adr/` keeps it and points at it from the router.

Why these are separate and not fewer:

- **Invariants vs principles.** An invariant is a test ("never emit a read-only
  symlink"). A principle is judgment ("prefer real files the app can write, even when
  enforcement gets harder"). Agents violate invariants silently during implementation
  and need them cheap to load alone. Principles are for design sessions.
- **Vision vs scope.** Vision is stable, scope churns. Mixing them makes the vision
  stale by association.
- **Futures vs scope.** The line between "might happen" and "we are building this" is
  the line the whole practice exists to hold. A future becomes a goal only by moving
  files and writing a decision.
- **Domain vs invariants.** Domain truths are facts about the world (cfprefsd rewrites
  plists in binary). Invariants are commitments about the system (drift is compared on
  semantic hashes). Facts explain commitments; they are not the same thing.
- **Assumptions live in domain.md**, not their own file, and only when something
  depends on them. An assumption without an "if false" consequence is trivia.

Concepts considered and folded in rather than given their own document: product
experience principles (in `vision.md` "what it should feel like" and `principles.md`),
capability maps and boundaries (`scope.md` "current boundaries"), intentional technical
debt and temporary compromises (a decision record with a `Revisit when`), risk
registers (the fired-trigger review *is* the risk register), success criteria (`Done
when` on goals), reversibility and cost of delay (stated in a record's Consequences when
they matter, not as fields), maturity stages (the repo's own roadmap or spec vocabulary,
pointed to from `scope.md`).

## 4. Conditions everywhere

The most valuable addition over classic ADRs is not a field on decisions. It is that
**every entry that can go stale carries its own expiry condition**, using four fixed
labels so one grep enumerates all of them:

| Label | On | Means |
|---|---|---|
| `Done when:` | goals | the observable exit test; met → delete the goal |
| `Promote when:` | futures | condition that turns a direction into a goal |
| `Drop if:` | futures | condition that kills a direction |
| `Revisit when:` | decisions, non-goals, assumptions, invariants | condition that makes the old reasoning worth reopening |

A good condition tells a future agent *what changed*. "Revisit if scale increases" is
useless. "Revisit SQLite when writes must originate concurrently from multiple
application nodes, or the deployment model requires a shared remote database" is
actionable: an agent designing multi-node writes will match it by keyword and say
"trigger fired" before designing around the old decision. Numeric thresholds are good
when they exist; named events (a spec ships, a second consumer appears, a milestone is
hit) are just as good.

`grep -rn -E "(Revisit when|Promote when|Drop if|Done when)" docs/intent/ docs/adr/`
is the trigger registry. There is no generated one.

## 5. Decision records

General decision records, not only architecture. The type is metadata, not a new
acronym: product, ux, domain, architecture, data, security, ops, dependency, platform,
process, strategy.

**Format** (`templates/decisions/0001-template.md`): a label block (`Status`, `Type`,
optional `Date`, optional `Supersedes`) then `Context`, `Decision`, `Rationale`,
`Alternatives rejected`, `Consequences`, `Preserves / forecloses`, `Deferred`,
`Revisit when`. Only `Context`, `Decision`, and `Revisit when` are required. Empty
sections are deleted. Under 80 lines.

**Lifecycle:** proposed → accepted → superseded by <id> | deprecated. Two more states
carry real information: **rejected** (a proposal that lost; kept because it will be
proposed again, with the condition under which it would win) and **deferred** (decided
not to decide; the `Revisit when` says what un-defers it). Superseding edits only the
old record's `Status:` line.

**Threshold.** Record when any holds: reversal is expensive; it constrains code outside
its own package; it creates or removes an invariant, non-goal, or boundary; it accepts a
notable tradeoff or dependency; it defers something significant and names what would
un-defer it; it rejects an alternative someone will propose again; without the record a
future reader would rebuild this differently. Otherwise a comment, a test name, or a
commit message carries it. The last criterion is the litmus test.

**Rejected ideas.** There is no separate rejections log. A rejection is only useful
later if it records why it lost and what would make it win, so:
- inside a record, each rejected alternative gets a one-line reason and, where useful,
  `wins if: <condition>`;
- a standalone proposal that was declined and passes the threshold becomes a record
  with `Status: rejected` and a `Revisit when`;
- session-scale rejections during planning stay in the plan (a plan's disposition rows
  are the right home; they die with the plan, as they should).

## 6. Future directions

A future direction is not a requirement. Each entry has: confidence (**likely**: a
commitment or sponsor statement exists; **plausible**: fits the vision, no commitment;
**speculative**: imaginable, must not influence design, listed only to stop
re-proposal), horizon (**next**, **later**, **someday**, never a date), motivation and
evidence, `Do not build:` (the concrete things that must not exist yet), `Preserve:`
(the free choices that keep it open, or "nothing"), `Promote when:`, `Drop if:`.

Promotion is a physical move: the entry leaves `futures.md`, a goal appears in
`scope.md`, and a decision record explains why now. That friction is intentional.

## 7. Consumption: routing, not loading

Always-loaded instructions (global `~/.claude/CLAUDE.md` / `~/.codex/AGENTS.md`, project
`AGENTS.md`) are a
**knowledge router**: they say the system exists, where the router is, when to consult
it, and that only the routed sections get loaded. They never contain the knowledge.

The router's routing table maps task classes to reading lists:

| Task | Read |
|---|---|
| typo, test fix, refactor inside one package, no behavior change | nothing |
| implementation touching a listed sensitive surface | `invariants.md`, decisions index |
| feature or behavior change | `scope.md`, `invariants.md`, `principles.md`, decisions index, matching futures |
| new subsystem, architecture change, major refactor | the above plus `vision.md`, `domain.md`, `futures.md`, relevant records in full |
| strategy or roadmap | everything |

The decisions index (`decisions/README.md`) is scanned by title and short trigger;
records are opened only when their title or trigger touches the task. The design
protocol (`intent-design`): classify, load only what is routed, extract what
applies, **state conflicts and fired triggers before designing**, apply the two-horizon
test, resolve missing authority or material uncertainty, design, record what passes the threshold, report what
was consulted.

## 8. Maintenance

Agents update the system as part of ordinary work, in the same change, when the truth
changed: a decision passed the threshold; a goal met its `Done when`; a future met
`Promote when` or `Drop if`; an assumption, invariant, or non-goal changed; a trigger
fired (state it; reuse explicit owner authorization when already given). Trivial work never touches
`docs/intent/`.

Run `intent-audit` and `intent-prune` at a release or milestone: walk every condition,
delete met goals, promote or drop futures, check assumptions, check sizes and index
consistency. Ten minutes, not a ceremony.

Staleness defenses, in order of strength: conditions on every entry (an entry says when
it dies); present-tense living docs with git as the only journal (nothing to keep in
sync); the physical move on promotion (a future cannot quietly become a requirement);
size budgets (a 400-line scope file is a roadmap in disguise); and the routing table (a
doc nobody is routed to is a doc nobody maintains, so it should not exist).

## 9. Context budget

Router ≤ 80 lines. Each living doc ≤ ~150 lines. Each record ≤ 80 lines. Global routing
instruction ≤ 10 lines. Project stanza ≤ 6 lines. Skill description ≤ 3 sentences; body
loads only on invocation. A trivial task costs zero intent tokens. A feature costs the
router plus three short docs plus an index. Only a redesign loads most of it.

## 10. Bootstrapping

**Existing repo.** Inventory prior art (vision, ADRs, architecture, roadmap, specs,
agent instructions, plans, tests that encode rules, git log for reversals). Point at
good existing documents; never copy them. Keep an existing decisions directory and its
numbering (`docs/adr/ADR-011` after `ADR-010` is fine; the router says "ADR here means
any decision record"). Populate from evidence, quote sharp phrasing, mark inferences
`(inferred)`, omit what has no evidence, put the rest in `questions.md`. Add a
`## Revisit when` trailer to existing ADRs that already contain a reconsider or defer
condition in prose, so the grep finds them. Add the project stanza to `AGENTS.md`
only (a `CLAUDE.md`, if the repo must keep one, holds just `@AGENTS.md`). Deliver five sharp open questions, not thirty.

**New repo.** Same templates. `vision.md` and `scope.md` come from the brief;
`futures.md` holds only what the brief names; `decisions/` starts with the two or three
choices the brief already made that pass the threshold; `invariants.md` may be empty
and omitted until the first one is real.

## 11. Monorepos and nesting

A repository holding several products, or components with different evolution paths,
adds the **repository charter** (`docs/intent/repo.md`): why one repository, what
belongs, a component map whose evolution column carries `split when` / `merge into` /
`retire when` / `inbound from` triggers, the dependency direction, and the ecosystem it
sits in. A component gets its own `docs/intent/` only when it passes the own-intent test
(its own audience, cadence, or roadmap); everything else is covered by root. Skills route
by affected paths, not just cwd, and read applicable root constraints once. A component
decision narrows a root decision, never contradicts it; references are path-qualified.
Full guidance, examples, and the doctor checks: [monorepo.md](monorepo.md).

## 11a. Composable routing

The practice is one rule applied to every kind of project knowledge: **each fact has
exactly one home, declared by the question it answers, its audience, its budget, its
lifetime, and the moment it is loaded.** Everything else links to it. The router is a
per-repo manifest of those homes; the always-loaded stanzas are the pointer to the
router; the skills are the procedures that read and write homes at the right moment.

That is what makes it composable:

- **Across kinds.** Intent ([README](README.md)), change records
  ([changes.md](changes.md)), and operating records ([operating.md](operating.md)) are
  three families with the same shape. A new kind (say, a threat model) is added by
  declaring its home, budget, lifetime, and load moment, and adding one docs-map row.
  It never adds a new always-loaded instruction.
- **Across levels.** Global stanza (every repo) → project stanza (this repo) → nested
  stanza (a product in a monorepo) → router → doc. Each level only points down.
- **Across tools.** The homes are files; the stanzas are text; the skills are
  `SKILL.md` with two frontmatter fields. Claude Code, Codex, and any `AGENTS.md`-aware
  agent read the same homes.
- **Across time.** Load moments differ by kind: intent at design time, change records
  at commit time, autonomy at action time, incidents at failure time, tripwires at
  session start (`intent-doctor`). Nothing is loaded at a moment it is not needed.

The test for any proposed addition: can you fill the five fields (question, audience,
home, budget, load moment)? If the answer to "load moment" is "always", the addition is
wrong or it is a three-line summary of something with a real home.

## 11b. Evidence ladder and evidence per surface

"Done" is a claim; evidence is what makes it true. The ladder, weakest to strongest:
`stated` (a sentence in a doc) → `reviewed` (a human or adversarial agent read it) →
`test` (a named test asserts it) → `gate` (CI or a hook refuses the commit) →
`proven` (an index or instrument confirms it after the fact, e.g. `fathom verify`).

Two places carry it. `invariants.md`'s Enforced-by column says where each invariant sits
on the ladder, honestly, so an agent knows when it is the only guard. The router's
sensitive-surface table has a **Done needs** column naming the evidence a change to that
surface must produce before it is done. An incident of class `wrong-done-claim` or
`gate-bypassed` usually ends by raising one row one rung.

## 12. Failure modes and their defenses

| Failure | Defense |
|---|---|
| context bloat | routing table, size budgets, skill body on demand, zero cost for trivial tasks |
| agents ignore it | the global instruction makes "state conflicts and fired triggers before designing" a required output, so silence is visible |
| docs drift from reality | conditions on every entry, present tense, `refresh` walk, enforcement column on invariants |
| over-future-proofing | the three-part free-option test; speculative futures are inert by rule |
| under-considering the future | futures with keywords agents will match; `Preserves / forecloses` section on records |
| stale roadmaps | `scope.md` holds current goals with exit tests; the roadmap stays a sequence and is pointed to, not copied |
| duplicate sources of truth | point, never copy; existing docs stay canonical; one stanza, in the one `AGENTS.md` |
| too many document types | omit empty homes; inherit shared authority and incidents; split only by a routed question |
| records for trivia | the threshold, and "would a reader rebuild this differently" as the litmus test |
| conflicting decisions | supersession edits the old status; the index shows both |
| multiple agents editing | small files, append-only records, git merges; the index is the only shared table and rows are independent |
| humans forget | agents update in the same change; audit and prune at a milestone |
| instructions too broad to mean anything | every sentence in the system must help someone decide something; that is the writing rule |

## 13. Considered and rejected for the practice itself

- **The name "PIDS" (Project Intent and Decision System).** Collides with process IDs
  in speech and grep; "system" implies tooling. "Project Intent" and `docs/intent/` say
  what it is and give agents a stable path to detect.
- **A separate rejections log.** Unbounded, second place to search, and rejections
  without a "wins if" are noise. Folded into records.
- **YAML frontmatter on records.** A `**Status:**` line is greppable and reads as
  prose; frontmatter adds a parser nobody runs.
- **A generated trigger registry or index.** One grep and one hand-kept table with
  independent rows. Generators go stale in a different way and need a build step.
- **Treating structural lint as truth.** Budget, link, index, stanza and snapshot
  checks are useful deterministic controls, not proof that a decision is still right
  or an invariant executes. The scoped/distribution practice in decision 0007 admits
  these checks; `intent-audit` still tests claims and fired triggers against evidence.
- **Per-decision dependency graphs, confidence scores, reversibility ratings.** Fake
  precision. `Consequences` prose and the `Revisit when` list carry what matters.
- **One umbrella skill with modes.** The first cut. Replaced by the `intent-*` family:
  eight descriptions cost a few hundred tokens at session start, and a named skill is
  discoverable in the `/` menu and in `$` completion where a mode argument is not.
- **Host-specific plugin packaging as the required distribution.** Plain hyphen-
  prefixed skills keep the practice portable; optional host packaging must not become
  a second maintained source or imply identical execution semantics.
- **Loading invariants via always-on rules.** Path-scoped rules (`.claude/rules/` with
  `paths:`) are a legitimate option when an invariant is silently violated often, but
  they duplicate `invariants.md`. Recommended only per-invariant, per-repo, with a
  pointer back to the source. Repos with an obligations engine (fathom) already have
  this channel.
