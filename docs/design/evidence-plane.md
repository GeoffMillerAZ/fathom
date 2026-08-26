# The evidence plane — intent corpus, unproven claims, change records

*Status: adopted (ADR-026) after the adversarial panel of 2026-08-26 (workflow
`wf_3a238f27-885`: law-and-intent, second-system, tokenomics-and-config,
implementability, gap-finder); every finding is disposed in §12. Owner-commissioned
("should fathom absorb the concepts we adopted from agentic-kit's Project Intent —
optionally; should honesty debt be tracked; should fathom be more git-aware; do it without
bloating global or project agent config"). Design protocol: the router routed to scope,
invariants, principles, the decisions index, futures, domain; no `Promote when` fired
(the panel refuted the draft's claim that F9 had — see §10).*

## 0. The verdict

**Narrow form: yes. Broad form: no.**

- Fathom does not become a planning tool, a task tracker, an intent editor, or a second
  home for any of it. The Project Intent practice stays what it is: human-authored
  markdown, git as history, procedures in agentic-kit's `intent-*` skills, nothing forced
  on any repo.
- Fathom does to the intent docs what it does to every other binding document: reads
  them, makes their claims **checkable**, and **lists what no instrument proves**. It
  does not evaluate prose; it evaluates only what an author annotated; it never edits.
- One verb (`fathom intent audit`, `fathom intent export`), one doctor row pair, one
  attention line shared with the custodian, one amended stanza sentence. No new MCP tool,
  no new hook, no new edit-time channel. Global agent config: +0 bytes.

Why this is fathom's grain: vision 23 accepted "decided means honored"; the custodian
already audits agent configuration mechanically; `verify` already proves plan claims; the
sweep already flips claims stale when their sources drift. The evidence plane points those
instruments at one more corpus and adds one honest ledger of what none of them prove.

Why the broad form loses (the panel's steelman answered): one binary already parses
plans, holds the ledger, runs the proven gate, and intent docs are plan-shaped — so owning
propose/decide/close would "close the loop". But the loop's missing piece is the human
decision, not detection: Q9 listed three fired clocks known for weeks and undecided. An
engine would add detection latency where decision latency is the bottleneck, and would
make fathom the owner of prose it must never edit (law 15).

## 1. Boundaries

| Boundary | Rule | Source |
|---|---|---|
| Ownership | Fathom never edits `docs/intent/`, `ADR.md`, `CHANGELOG.md`, or a condition's wording; `export` prints proposals a human applies. | law 15; custodian PROPOSE |
| Authoring | Prose conditions are human-judged, always. Evaluation exists only for a condition an author annotated (§3); no NLP, no LLM, over condition lines, ever. Nothing generated is then edited by hand. | ADR-018; practice §13 |
| State | Every verdict is a pure re-derivation; the only persistence is an append-only instrument record, a separate `~/.fathom/intent-ledger.jsonl` (never the verify ledger, whose readers count rows). | ADR-018; sdd-reconciler doctrine 1 |
| Surface | No new MCP tool. One CLI verb with two subverbs. Doctor rows never FAIL on this plane. | law 4, law 10, T6 |
| Channels | No new injected channel. The session-start attention line is the custodian's, extended. The edit-time path is the existing pre-edit guard (`guard.cue`), fed by a proposal, not a second injector. | I35, budgeted-answers, law 4 |
| Force | A repo without `docs/intent/` gets one informational doctor line and still gets unproven claims from what it does carry (plans, verify ledger). | product-vision §2 |
| Scoring | Unproven claims are a list with remedies, never a grade, never a savings claim; nothing here is summed into a score. | law 3, law 12 |
| Dispatch | The plane answers "which condition fired, which claim is unproven". It never schedules, retries, or acts. | vision 24 |
| Hooks | The session hook evaluates nothing and makes no RPC for this plane; it reads the newest ledger rows at HEAD and cites their age (≤200 ms, `intent_ms` on the session beat). No oracle of any class runs in any hook. | I9, I12, tokenomics |
| Kill | Each feature carries a kill criterion whose instrument exists (§8). | law 5, law 7, law 8 |
| Trust | Annotations are untrusted repository content (a clone, a PR, a sibling committer). Oracles are read-only, root-confined (`paths` may not leave the repo; `..` and absolute paths lint loud), and `service` must be a service rooted in this repo (else `unevaluable: cross-repo refused`, F18). Blast radius is a count. The threat model itself is still Q5. | law 13, F15, Q5 |
| Population | The consumer population is computed (router present ∧ `.fathom/` present), printed by `fleet report`, never a list in prose. v1 audits the nearest router at the repo root only; nested routers (a monorepo's `kits/*/docs/intent/`) are found and named as unaudited in the doctor row; ids reserve a path qualifier (`kits/ci-kit:F2#1`) for v2. | practice monorepo rule; law 3 |

## 2. The intent corpus

**Input.** `docs/intent/README.md` (router: docs map, sensitive-surface table with `Done
needs`), the living docs it names, decision records (`ADR.md` sections or `docs/adr/*.md`;
"ADR-" means any decision record), and every line carrying one of the four labels
`Done when`, `Promote when`, `Drop if`, `Revisit when`. `docs/vision/` is never read
(aspirational prose; a domain truth). Capability `intent-corpus` (seam core, any scope,
switchable) is on when the router exists and off-able; off silences the delivery channels
(attention line, stanza sentence), never the verb.

**Parser** (`internal/intent`; `internal/adr.Parse` extended for `## ADR-NNN` sections and
the `**Revisit when.**` trailer rather than a second ADR parser):

- Condition grammar: a label followed by `:` or `.**`, outside backticks; the text runs to
  the next label, blank line, next list item, or end of table cell (conditions wrap). The
  annotation is the last `{…}` block inside that text. Id = `<record>#<n>`: record = the
  nearest preceding `## F<n>` | `## ADR-NNN` | `**G<n>.**` | `**Q<n>.**` | `### INC-<n>` |
  `| I<n> |` | non-goal `NG-<first three words>`; none → file stem; `n` = ordinal within
  the record. `Revisit when: never` → verdict `terminal`, excluded from every total.
- Invariant rows: backticks and bold stripped before classification; `VIOLATED` anywhere
  → violated; `gate task <target>` → Taskfile target, `gate lefthook <hook> <step>` →
  lefthook step, other `gate …` → unevaluable; `test <Name>` with `Test\w+\*?` (prefix
  match on `*`); `` `code` ``, `` `stated` ``, `lint …`.
- Sensitive surfaces: globs are the backticked tokens of the Surface cell, matched by
  `internal/pathglob.Match` (promoted from `internal/memory`'s `**`-aware matcher; three
  private copies today); all matching rows count; `Done needs` cells yield test names and
  commands.
- Decision records: the home is read from the router's docs-map "Decisions" row (the
  router is the manifest of homes), falling back to `ADR.md` then `docs/adr/`; a home that
  resolves to nothing is reported as such, never as zero decisions. A repo with a decisions
  home but no router is the *decisions-only* tier: its `Revisit when` trailers are listed
  and audited, the doctor says "N decisions, M with Revisit when, no router (intent-init)",
  and the capability probe reports the tier (router | decisions-only | none). Nine fleet
  repos are in that tier today; this is vision 23's `adr audit` without a router precondition.
- Aliases and shapes: `Revisit when: never` and `none identified` are `terminal`; domain.md's
  `If false:` is accepted as the assumption alias the practice names; a present table that
  yields zero rows (invariants, surfaces, futures, goals) is `unparsed (table shape)`, never
  a healthy empty (the INC-8 class).
- Rungs beyond the ladder's tokens: `reviewed` → unenforced (listed, uncounted, like
  `stated`); `proven <instrument>` → backed by a ledger row; `lint <name>` and `guard` resolve
  against Taskfile targets, lefthook steps, `scripts/*`, and `guard.cue` scopes, else
  `unevaluable` named in U4; `Done needs` commands reuse the custodian's
  `command_documented_and_exists` resolution.
- Tolerance is measured, not asserted: golden fixtures from the consumer repos including one nested monorepo and one router-without-`.fathom/`;
  unparsed lines are reported with file:line and counted per repo in the doctor row (the
  drift gauge). Where the three corpora disagree (rung vocabulary, decision-record shape)
  the fix is a kit-side ask (§11), not a third adapter.

**What the corpus feeds:** the audit (§4), the export (§5), the doctor row, the attention
line. Explicitly not: the LLM obligations compiler (`ADR.md` is already in its corpus and
is therefore dual-served — the audit dedupes by ADR id; the layer gains an explicit
`corpus: exclude: ["docs/intent/**"]` pinned by a test so it never silently widens).

## 3. Annotated conditions (opt-in; the annotation is the demand signal)

A condition may carry one annotation using the plan grammar's tokenizer
(`internal/verify.ParseAnnotation`, exported for this; its suspect-key set gains the oracle
keys so a mangled block lints loud instead of vanishing — the O4 class):

```
Promote when: a second repo needs different budgets {oracle: lexical, paths: ".fathom/capabilities.cue", pattern: "budgets:", min: 1}
Revisit when: XT-6b lands {oracle: index, service: fathom, symbol: tsengineCppExtractor, present: true}
Done when: every custodian row proves {oracle: plan, plan: "docs/plans/custodian.md", rows: "all-proven"}
Drop if: no external consumer by 2027-01-01 {oracle: date, after: "2027-01-01", partial: true}
```

- Classes in v1: `index`, `plan` (both answered by `verify.Gate`'s criteria), `lexical`,
  `date`. `plan` and `date` are new to the reconciler's forward registry
  (`docs/design/sdd-reconciler.md` §5 is amended in the same change). `cmd` is
  recorded-not-built (§9).
- An annotation covers the whole condition or declares `partial: true`, which renders
  `due — human-judged`, never `fired`. An unannotated condition is `human-judged`: listed,
  never evaluated, never counted as fired. `index … present: false` is `unevaluable` when
  the service's index is empty or stale (I25's lesson).
- Evaluation runs only in `fathom intent audit` (and `doctor --oracles`), never in hooks,
  never in `prime`. Deterministic: sorted ids, injected clock, no map order.
- **First-seen ledger.** Every verdict transition per `(id, verdict, commit)` appends
  `{kind: oracle, id, verdict, evidence, commit, at}` to the intent ledger, so "fired at"
  exists for every class. **Decided** iff the record's own section changed after
  first-fired-at in one of three ways: its `**Status.**` line, a dated
  `**Revisited (YYYY-MM-DD).**` paragraph naming the id or `(n)`, or the goal/future/id
  absent from HEAD. An edit that touches only the annotation is listed as `criterion
  amended after firing`, never decided. Unknown → `undecided` with the reason.
- First-seen rows live in `~/.fathom` and are therefore per-machine evidence: a verdict
  reads "fired (first seen on this machine 2026-09-02)" or "fired (first seen unknown
  here)"; rows keep `repo_root` and `commit`. Whether a committed `.fathom/` ledger should
  carry first-seen across machines is Q19.
- The oracle runtime ships only after the demand signal exists: ≥3 hand-written
  annotations across ≥2 consumer repos (wave-2 gate in the plan). Until then the audit
  lists conditions with ids and labels, all human-judged.

## 4. Unproven claims (the honest ledger)

The verb prints "unproven claims", never "debt": a list with remedies, never a total that
invites a score. Sources in v1, each present only when its input exists:

| # | Source | Proven by | Remedy |
|---|---|---|---|
| U1 | Plan rows claimed `[x]` that verify Fail | newest verify-ledger row per plan at HEAD (age cited; `--fresh` re-verifies) | fix the code or uncheck |
| U2 | Plan rows claimed with no contract | same rows | anchor a symbol or `skip_test` with a `skip:` reason |
| U3 | Invariants `VIOLATED` | `invariants.md` | fix or restate; `stated` rows are listed as *unenforced (not debt)*, never counted |
| U4 | Invariants or `Done needs` naming a test or gate that does not exist | one synthesized in-memory plan → one `verify_plan` RPC; cached per HEAD in the ledger | write or rename the test / gate step |
| U5 | Conditions fired and undecided (§3) | intent ledger | `intent-decide` |
| U6 | Open incidents whose `Outcome:` is absent or `pending` past 30 days (the practice's own `intent-doctor` check 14 threshold) | `incidents.md` | `intent-incident` |
| U11 | Backticked repo paths, `Test\w+` names, and `pkg.Symbol` tokens in `docs/intent/**` and decision trailers that the tree or the index does not know ("a decision that cites code that moved") | custodian `pointers_resolve` over the intent corpus + one index lookup per token, cached per HEAD | fix the name or mark it historical |

Wave 2 adds U12: plan rows whose contract symbol lives under a sensitive-surface glob and
whose `reachable_from` is none of that surface's `Done needs` tests — listed, never gated
(the router's evidence column becomes a criterion without touching exit codes).
Deferred sources with revive triggers (§9): U7 `[Unreleased]` lines without a reference,
U8 commits without a resolvable `Verified:`, U9 served claims `stale`/`contradicted`,
U10 `## Maintained` names not registered (needs a dispatched-verb table).

Output: `fathom intent audit [--json] [--kind …] [--fresh] [--top N]` — sections
*conditions* (id · label · verdict · evidence), *invariants* (claimed rung vs what the
index and gates show), *unproven claims* (kind · ref · remedy), each capped at `--top 20`
with an `elided` line naming the count; `--json` unbudgeted; no `--strict` (the verify gate
is the gate). Each run appends `{kind: debt, by_kind, commit, at, ms}` to the intent
ledger; the attention line derives `+k` against the newest row older than the current session (24 h) and omits it otherwise.

**Fleet.** `fathom fleet report` gains an `unproven` column read from the ledger by repo
root (`n/a: no audit recorded` when absent; never a verify run per repo). The observatory's
SDD panel (SD-2) takes these kinds as rows with the claimed-vs-proven gap as headline.

## 5. Delivery — where each thing appears, with numbers

| Moment | Channel | Content | Budget (measured field) |
|---|---|---|---|
| Session start | the custodian attention line, extended | `⚠ attention: config N · unproven M (+k) · fired F — fathom doctor` | ≤160 B; only while N+M+F > 0; a whole-primer cap of 3,200 B with a test (I12 amended: the thread section keeps 2,400 B); `intent_ms` ≤200 ms on the session beat; Codex via `prime` with the same source gating |
| Edit | nothing new | the existing pre-edit guard path serves `guard.cue`; `fathom intent export --guard` proposes entries from the router's sensitive-surface rows (ids · rungs · `Done needs` tests, ≤240 B per entry) for a human to apply | guard's own `max_rules` and dedup stamp; `payload-cost/pre-edit-guard` |
| Doctor | `intent` (pass/warn, never fail), `unproven_claims` (warn, never fail) | counts by rung, conditions total/annotated/fired/undecided, unparsed count + first line, clock age; first remedy | `intent` ≤500 ms p95, `unproven_claims` ≤2 s p95, per-check `ms` in `doctor --json` |
| Verb | `fathom intent audit`, `fathom intent export --guard` | §4, §5 | `--top 20` per kind |
| Stanza | the existing "Obligations —" sentence amended (+≈60 B): "…and, where `docs/intent/` exists, its matched invariants" — rendered only when `guard.cue` actually holds ≥1 intent-provenance entry, so the stanza never claims a channel that is not wired (I37's lesson) | net-zero new lines; `intentOn` = router present ∧ guard covered, resolved per repo by reconcile; 3 physical lines stay 3 |
| Guard drift | proposed entries carry `note: "intent: <glob> @ <router blob sha>"`; `intent audit` reports `guard_entry_stale` when the source row changed or vanished (remedy: re-run `export --guard`); the doctor row counts them | one fact, one home, one detector (coding-agent-excellence §6) |
| Codex parity | attention line via `prime` (same source gating); no edit-time channel (guard entries ride Claude's PreToolUse only); the commit-msg shell step is the one harness-neutral channel; K3/K5 denominators are split by harness | adoption-doc parity table |
| Global config | nothing | — | +0 B; the kit-side ask (§11) shrinks the practice's global paragraph instead |

Cost model stated for every injected channel: face tokens × remaining turns × ~0.1 (the
cache-rent line vision 21 measured at 22×). The attention line is ≤40 tokens once per
session; the plane adds no per-edit tokens.

## 6. Verbs and rows (law 10)

```
fathom intent audit [--json] [--kind conditions|invariants|unproven] [--fresh] [--top N] [--oracles]
fathom intent export --guard        # PROPOSE-only guard.cue entries from the sensitive-surface table
fathom intent export --cue          # deferred: generated shadow catalog (DP-1 reframed) — see §9
```

Every `fathom intent` invocation appends a local beat `intent-cli/<subverb>` (law 7) so its
use is measurable without a daemon RPC. Exit codes mirror `config audit`: 0, or 1 when
`--strict`-shaped callers ask (none in v1).

## 7. Change records (git-aware) — what ships now, what waits

Ships now (each with a named consumer and a fired clock):

- **Archive absorption via git history** (Q14: 36 proven plans unarchivable; pre-push cost
  O(43)). `fathom plans archive` extends its corpus with
  `git log <first-commit-touching-plan>..HEAD --format=%s%n%b -- . ':!docs/plans'` —
  commits that touched non-plan paths, so a plan's own bookkeeping commits cannot absolve
  it (the panel's weakening, stated). Measured: full log 16 ms, `--follow --diff-filter=A`
  44 ms.
- **`commit` on every record.** Thread items gain `commit`; intent-ledger rows carry it;
  verify-ledger rows already do. Metadata, never a citation.
- **Commit-msg advisory** as a shell step in `init`'s printed MANUAL lefthook block
  (`grep -q '^Verified:'`, advisory), no verb. Baseline recorded: 5 of the last 100
  commits carried `Verified:`.
- **`adr_references_resolve`** — a built-in custodian kind: every `ADR-\d+` cited in
  `docs/design/*.md` status lines and `docs/intent/**` resolves to an existing record. It
  would have caught this design's own first header (INC-14).

Deferred with triggers (§9): the Go custodian kinds `commits_cite_instruments` (citations
resolved against the index, Taskfile, and the verify ledger; `not run:` counts as absent),
`changelog_lines_reference` (skips headings, blanks, `Release impact:`),
`release_impact_consistent` (from `[Unreleased]` headings and `!`, since commits here are
untyped), `compat_surface_paths_resolve`; and `fathom changes impact` is **cut** — the
`intent-release` skill already does it in prose and the commits carry no types to derive from.

## 8. Kill criteria and their instruments (laws 5, 7, 8)

Consumer population: repos with `docs/intent/` and fathom wired (fathom, conjure,
agentic-kit). The clock starts with a `{kind: clock, population, commit, at}` row written by
the first `fathom intent audit` that sees ≥1 sibling repo (law 6); `intent audit` prints
the clock's age and doctor names its absence.

- **K0 annotations:** <3 annotated conditions across the population 30 days after the clock
  → the oracle runtime is not built (or is deleted); the listing stays. Instrument: the
  debt row's `annotated` count.
- **K1 decision latency:** annotated conditions' median days from first-fired to decided is
  no shorter than the pre-clock git baseline (days from a condition's last text edit to the
  Status or `Revisited` edit that answered it, over the 90 days before the clock) → delete
  the evaluator, keep the listing. Instrument: intent ledger first-seen rows vs `git log`
  on the record (law 12's counterfactual, measurable because both ends are dated).
- **K3 verb use:** zero `intent-cli` beats in 30 days with the population present → fold
  the audit into the doctor row only.
- **K5 attention line:** among sessions whose stamp carries `intent_line: true`, ≥80 % have
  neither an `intent-cli` nor a `doctor` beat in the session window over 30 days → remove
  the line, keep the doctor row. Instrument: session stamp + beat join (the custodian
  `attention` precedent; obeying the line by running doctor counts; never a narrate story
  row — law 14).
- **K6 change-record kinds** (when they ship): zero violations for 90 days with the
  population present → fold into the `intent-doctor` walk.
- **K-plane:** at day 30, if the only sessions calling `fathom intent` are in this repo,
  the plane freezes as one verb + one doctor row; no channel grows.

## 9. Deferred and rejected (each with its evidence)

| Item | Disposition | Revives / never |
|---|---|---|
| `cmd` oracle | deferred | ≥3 annotations in ≥2 repos name it; then argv without a shell, 10 s per command / 30 s per run, stdout ≤1 MB, cwd = root, `FATHOM_ORACLE_DEPTH=1` (a fathom child refuses to run oracles), `FATHOM_CHANNELS=off`, scratch verify ledger, default allowlist `fathom <verb> --json` and `task <target>`; never in doctor, hooks, `prime`, or `fleet report` |
| Edit-hook intent block | rejected | duplicates the pre-edit guard path; measured post-edit payload p50 99 / p75 328 / p90 354 tokens against a 150-token ceiling — no residual budget; `export --guard` is the channel |
| Second attention line | rejected | one line, the custodian's, extended |
| `fathom changes impact` | rejected | the release skill does it in prose; commits are untyped |
| Change-record custodian kinds | deferred | `intent-release` reports ≥2 missed `[Unreleased]` lines or ≥5 `Verified:`-less commits in one release, and `.fathom/concerns.cue` exists in ≥2 repos (CC-12 packs) |
| Unproven sources U6–U10 | deferred | the first month in which `intent-audit` or `intent-release` reports ≥2 misses of that kind by hand |
| LLM compile of `docs/intent/**` | deferred | a measured precision gap the deterministic corpus cannot close |
| `intent export --cue` (DP-1 reframed: the catalog is output) | deferred | a second tool consumes intent as data |
| Cross-repo federation | rejected (F18 speculative) | never for now |
| A fathom intent skill | rejected | the procedure is agentic-kit's; its skills consume `fathom intent audit --json` when present |
| Annotations on plan `trigger:` rows (the real F9) | not built | F9 stays a future; its own `Promote when` is unfired |
| Incident candidates proposed from instruments (a `[x]` flipped with no verify-ledger row at that commit; a worktree older than 7 days) | deferred | the ledger has carried `commit` for ≥30 days and the walk finds ≥1 such case by hand; `--propose` prints an INC stub, never files it |
| `surface_commits_cite_done_needs` (commits touching a sensitive surface whose `Verified:` names none of its `Done needs`) | deferred | the `Verified:` share reaches ≥50 % over 30 days (baseline 5/100 — it floods before that) |
| `Verified against:` tripwire in fathom (harness version vs the router footer) | deferred | `intent-doctor` check 18 reports drift twice by hand |
| Any fathom-side writer of `Status`, `Revisited`, or a condition's wording | rejected | never — the ownership boundary |
| LLM-suggested `{oracle: …}` annotations from prose | rejected | never — the annotation is the demand signal; machine-written ones would make K0 meaningless |

Explicit non-goals, in one place: no oracle over prose; no oracle in any hook, in `prime`,
or in plain `doctor`; no cross-service or cross-root oracle; fathom never writes an
annotation, a condition, a status, or an incident; never a score, never a cross-repo trend;
`docs/vision/` is never read.

## 10. Decisions this design records, and how they were authorized

The owner delegated the design decisions of this session in writing ("don't even ask me,
just go ahead and spec it out"); each decision below carries its own record and its own
`Revisit when`, and the owner may reverse any of them by editing that record.

- **F9 did not fire.** Its condition is about plan `trigger:` vocabulary repeating; the
  four intent labels repeat because one template was copied into three repos. The draft
  said otherwise; the ADR-018 amendment is corrected and F9 stays in `futures.md`.
- **Fired now by this design:** I35's `Revisit when` (a serving-side token cap — the
  honesty-fixes plan measures precision first, then enforces), I36's (a `## Maintained`
  parser test), and the decisions-index row for vision 23 (`fathom adr audit` ships as
  `fathom intent audit` over ADR trailers). The contract for conditions lives in markdown
  annotations, not CUE (one tokenizer, the plan precedent); `export --cue` is the CUE face
  when a consumer appears.
- **Answered under delegation** (each an ADR amendment or a living-doc edit with a
  trailer): Q1 trunk = `main`; Q2 post-commit refresh routes through the daemon; Q7 one
  friction log; Q8 active program = this plane; Q9 ADR-018 pilot KEEP, ADR-014 group
  surfaces deleted (on law 4 — unreachable by any wired client — not the call-log clock);
  Q12 ADR-006 re-scoped; Q13 the ceiling is enforced after a precision measurement; Q14
  archive via git history.
- **Wave 1 stands on its own.** If the practice's owner declines annotations (Q18), K0 is
  pre-fired and the plane is the listing plus the enforcement audit (U1–U6, U11) and the
  decisions-only tier — all index- and git-backed, none dependent on annotations. That is
  why wave 1 ships before Q18 is answered.
- **Still the owner's:** Q4 (product or force-multiplier), Q5 (threat model — now concrete:
  annotations are untrusted content), Q16 (whether unproven counts may leave the machine),
  Q18 (does the practice's owner accept annotations in its template docs), Q19 (where
  first-seen lives across machines), Q20 (who owns the `{oracle: …}` grammar if a second
  evaluator appears).

## 11. Partition with the practice's skills, and the kit-side asks

One source per check, so two verdicts cannot drift:

| Check | Owner | The other side reads |
|---|---|---|
| U1–U6, U11, enforcement rungs, first-seen, fired/undecided, guard drift | fathom (`intent audit --json`) | `intent-audit` reads the JSON when the binary is present, else its grep |
| Router budget, doc budgets, index rows = records, record shape, CHANGELOG shape, stanza identity, skills resolve, tool versions | agentic-kit (`intent-doctor`) | fathom reports a structural defect only as `unparsed` with remedy `intent-doctor`, never a verdict |
| Open incidents past 30 days | agentic-kit's check 14 is the rule; fathom's U6 cites it | both name the same threshold |
| Pointers resolve | custodian `pointers_resolve` (fathom) over agent config and, via U11, the intent corpus | `intent-doctor` 19/20 keep the router-only check |

Kit-side asks (filed in `agentic-kit/docs/plans/fathom-uplift-asks.md`):

One rung ladder across the three repos (`gate`, `task`, `guard` added; `lint <check>`),
one decision-record shape rule; a `Status: proposed` decision that condition lines may
carry `{oracle: …}` annotations and that fathom may evaluate them; `intent-audit` and
`intent-doctor` consume `fathom intent audit --json` when present and fall back to the
grep; the global "Project intent (all projects)" paragraph shrinks to one pointer line
(≈200 tokens × every session in every repo — the only global-config number in this design); the practice's own §13 condition on lint tools ("a second repo shows index drift the walk did not catch") is named as under test — U4-class drift is exactly what the walk misses — so the kit decides it in its own record; `intent-prune` and `intent-decide` preserve `{…}` blocks verbatim; §11b gains `proven` (`fathom intent audit`); `monorepo.md` gains the nearest-router-plus-root rule for tools.

## 12. Panel disposition

| Finding (lens) | Disposition |
|---|---|
| Adopted before the record existed (law, second-system, tokenomics) | accepted — ADR-026 written first; INC-14 recorded; `adr_references_resolve` kind |
| F9 not fired (law, second-system) | accepted — §10; ADR-018 amendment corrected |
| "Decided" not computable without first-seen (law, implementability) | accepted — first-seen ledger for every class; three decided markers |
| K5 on narrate rows violates law 14 (law, second-system, tokenomics) | accepted — session stamp + beat join |
| Edit-hook block duplicates guards and cannot fit (second-system, tokenomics, implementability) | accepted — cut; `export --guard` |
| `plan`/`index` not cheap; hook must not evaluate (tokenomics, implementability) | accepted — hooks read the ledger only; oracles in the verb |
| Verify-ledger pollution (all four) | accepted — separate intent ledger, `commit` field |
| Oracle classes "already exist" is false (second-system) | accepted — v1 = index, plan, lexical, date; `plan`/`date` declared new in sdd-reconciler §5; cmd deferred |
| `cmd` hazards: recursion, spend, shell (law, tokenomics, implementability) | accepted — §9 mechanism, never in doctor/hooks |
| `stated` as perpetual debt erodes honesty (law) | accepted — U3 = VIOLATED only |
| `+k` needs stored counts (law, implementability) | accepted — from two ledger rows or omitted |
| Attention line outside the thread cap (tokenomics, implementability) | accepted — whole-primer cap 3,200 B, I12 amended |
| Stanza +1 line duplicates two existing paragraphs (tokenomics) | accepted — amend the existing sentence; kit ask for the global paragraph |
| Change records: population of one; `impact` vacuous (second-system, implementability) | accepted — deferred kinds with triggers; `impact` cut; shell commit-msg step |
| Archive via git: plan's own commits absolve it (implementability) | accepted — corpus excludes commits touching only `docs/plans` |
| Doctor FAIL rows everywhere (implementability) | accepted — warn, never fail |
| Three capabilities vs one (law, implementability) | accepted — one, `intent-corpus`, probe fields added |
| Parser grammar gaps: wrapped conditions, two labels per line, trailers, `**` globs, rung cells, `Test*` prefixes (implementability) | accepted — §2 grammar; `internal/pathglob`; golden fixtures from three repos |
| Owner questions decided by the agent (law) | accepted in part — delegation cited in §10; Q13 gated on measurement; wave 4 moved to `honesty-fixes.md` |
| Second parser for ADRs (law) | accepted — extend `internal/adr` |
| Rename "honesty debt" (second-system) | accepted for output and rows ("unproven claims"); the concept keeps its name in this doc's title only |
| Practice §13 rejections not named (law) | accepted — §1 Authoring; fathom is the repo's instrument reading the practice's docs, not the practice's lint; Q18 asks the kit |
| Population hand-listed; nested routers unaddressed (gap) | accepted — computed population, root-only v1 with nested routers named, path-qualified ids reserved |
| Annotations are untrusted input (gap) | accepted — Trust row; root-confined `paths`, own-service `service`; Q5 named |
| Decisions home hardcoded; nine ADR-only repos get nothing (gap) | accepted — home from the router; decisions-only tier |
| Rung words `reviewed`/`proven`/`lint`/`guard` unclassified (gap) | accepted — §2 |
| Stanza clause could claim an unwired channel; guard entries a second home without a detector (gap) | accepted — guard-covered predicate; `guard_entry_stale` |
| K1 unmeasurable as written; K5 fires on obedience (gap) | accepted — git baseline; doctor beats count |
| Healthy-empty tables; `none identified`; `If false:` (gap) | accepted — §2 aliases and shape checks |
| First-seen per machine unstated (gap) | accepted — §3; Q19 |
| No partition with the kit's skills; no Codex parity (gap) | accepted — §11 table; §5 row |
| U6 already has input and a threshold (gap) | accepted — wave 1 |
| U11 decisions citing moved code; U12 surface rows uncovered (gap) | accepted — U11 wave 1, U12 wave 2 |
| Incident candidates, surface commits, version tripwire (gap) | deferred with triggers — §9 |
| Fathom-side status writer; LLM-suggested annotations (gap) | rejected — §9 |
