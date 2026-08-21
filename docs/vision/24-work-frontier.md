# 24 — The work frontier: packs, eligibility, leases, and the dispatch boundary

> Owner ask, 2026-08-21: *"starting discovery trees… saved queries chosen
> by task type… discovery stored and assigned to implementation steps so
> discovery doesn't repeat… does fathom track the stories available to be
> implemented… a locking mechanism with heartbeat/TTL… harnesses watch
> particular buckets and kick off work as it becomes eligible and it's a
> DAG and everything says it's good."* Also binding, same day: *"whatever
> we do, it must at least be configurable to work in either ecosystem"*
> (Claude Code and Codex CLI).
>
> Method note: this doc is the synthesis of a 12-agent evaluation
> (2026-08-21) — five map readers over vision/design/ledgers, five idea
> evaluators, two vision panels — each verdict below cites the evidence
> that produced it.

---

## 1. The boundary that settles all four ideas

The four ideas are one idea: **agent fleets coordinating over fathom's
facts**. The pre-registered doctrine already draws the line that makes
this safe (09-deterministic-sdd.md:282-288 "general task tracker" and
"durable workflow engine" are out of scope forever; sdd-reconciler.md
doctrine 1 "state is computed, never stored"), and this doc registers the
corollary as the **dispatch boundary**:

> **Fathom answers "what is true about the work — which rows exist, which
> verify, which are eligible, who has declared intent to work one." It
> never watches, schedules, retries, spawns, or kicks off anything. The
> loop belongs to the harness.**

Everything adopted below sits on fathom's side of that line: one
annotation key, one derived flag, one materializer verb, one advisory
intent ledger, one pull primer. Everything on the far side — watchers,
buckets, supervisors, heartbeat reminders — is refused in fathom's core,
forever, exactly as 09 wrote it. A harness that wants the loop polls
`fathom plans next --json` at its own lifecycle moments (Claude Code:
Stop/SubagentStop hooks, already wired; Codex: agent-initiated at task
boundaries, documented in the shared AGENTS.md stanza — no faked parity).

Why the boundary is the feature: a wedged dispatcher can never corrupt
derived status, the reconciler's failure domain stays pure, and both
harnesses consume identical facts through vehicles they actually have.

## 2. I2 — context packs on plan rows (ADOPT — the strongest idea)

The measured problem: implementers re-derive the planner's discovery.
The exploration ledger already counts it — median 9 repeated file-reads
per row, raw:indexed discovery ≈ 5:1, and doc 19 names the subagent
duplicate-read multiplier without proposing a fix. This is the one idea
that is genuinely open whitespace, doctrine-clean, and cross-harness by
construction.

The shape (binding rows: docs/plans/work-frontier.md):

- **Pointers, not prose.** One new plan-row annotation key `ctx:` —
  space-separated pointers (symbol IDs, paths, concern IDs, thread IDs)
  the planner writes while planning, the same motion as `symbol:` today.
  Non-gating: verify ignores it entirely; lint only warns on
  unresolvable pointers. Storing rendered discovery TEXT is refused —
  it would be authored stale state (doctrine 1) and would rot against
  the live index (01-thesis freshness contract).
- **Fresh render at read time.** `fathom plans context <plan> --row <id>
  [--budget N]`: resolves each pointer through the live index —
  signature + file:line, direct callers, obligations covering the named
  files — budget-capped with an elision block (budgeted-answers §2). A
  pointer that no longer resolves prints `unresolved (moved since
  planning)`; staleness surfaces, never silently drops.
- **The handoff flow is the subagent contract.** The orchestrator runs
  the command and pastes the output into the worker's brief next to the
  output budget — the pack IS the brief body (operator contract §6).
  Plain CLI over the shared index, consumed as pasted text: identical on
  Codex, zero hooks anywhere in the path.
- **Pre-registered kill:** if raw discovery does not drop on ctx-bearing
  rows after ~20 of them (exploration ledger, existing instrumentation),
  delete the key — a one-line grammar revert.

## 3. I4 — eligibility, not an engine (ADOPT the delta, REFUSE the engine)

80% already ships: `fathom plans next --json` emits machine-readable
frontier rows with verify-derived state. The missing fact is
**dependency**: the frontier is document-order and nothing expresses
"row B is pointless until row A proves".

Adopted delta (S effort, inert without a consumer):

- `needs: <id>[, <id>…]` annotation on a plan row, referencing stable
  `id:` values. Authored intent, same class as `reason:`/`trigger:`.
  Lint: unknown id, self-reference, cycle.
- Frontier rows gain derived `eligible` + `blocked_on` — eligible iff
  every `needs` target is claimed-and-verified or skipped-with-
  disposition. Pure function of (plan, evidence); nothing stored, no
  subscribe/watch surface, computed at query time. Human output prints
  `(blocked on md-3)`.
- `--eligible-only` is the entire "bucket". The DAG the owner asked for
  is the plan file itself; "everything says it's good" is the verify
  gate it already has.

Refused, forever, per 09: transition engines, lifecycle enums, event
logs, fathom launching work. Fleet-scale consumption stays
entry-conditioned on trajectory stage 3 (SubagentStop wired to verify,
stage-1 reconciler metrics green) — the grammar ships now only because
it is inert without a consumer. Sequencing dependency: fix O4 (the
brace-in-annotation parser bug) first — a corrupted annotation silently
drops the id a `needs:` points at.

## 4. I3 — leases as declared intent (ADOPT v0, panel dissent recorded)

"Who is working what" is a fact about the work — the same carve-out
doctrine 4 already grants authored dispositions ("intent is not
derivable from evidence"). Multi-agent-same-repo is lived reality
(throughline commits landing between sextant commits; collisions avoided
by luck), and no instrument measures a claim collision today.

Adopted shape — the minimum that is also the missing instrument:

- `fathom plans lease <row-id> --plan <file> [--ttl 2h]` appends one row
  `{plan, row_id, holder, harness, filed, expires}` to
  `.fathom/leases.jsonl` — repo-local, append-only, flock'd, mirroring
  threads.jsonl. Holder defaults to the session id on Claude Code, a
  free string on Codex. `--release` exists but is never required.
- All state is DERIVED: row exists ∧ now<expires = leased; else
  available. No status field, no lifecycle enum, no release ceremony.
  Never enters the store, never touches the plan file, never gates
  verify — `plans next` merely decorates (`leased (holder, 43m left)`)
  and `--free` filters.
- **Heartbeats, supervisors, and reminder hooks are REFUSED** — pure
  ceremony; a dead holder ages out at TTL and the row reappears. TTL-only
  expiry is chosen precisely because Codex has zero lifecycle hooks: no
  design element exists that Codex cannot execute. (Claude Code may later
  add an optional Stop-hook auto-release — honest tightening, not
  load-bearing.)
- `fathom plans leases` counts overlaps (two fresh leases, one row) —
  the first-ever collision measurement, which is the evidence-first
  justification for shipping v0 at all.
- **Panel dissent, recorded:** the systems panel would put even this
  outside fathom ("external flock'd consumer or nothing"); the product
  panel would kill it pending a measured collision. The owner-side
  ruling: the ledger IS the collision instrument — a collision without
  leases leaves no trace — and a sidecar JSONL with derived-only state
  crosses none of doctrine's red lines (not in the store, not in the
  grammar, not gating). **Pre-registered kill:** if after 6 weeks of use
  the ledger shows zero overlaps and zero decoration value, delete the
  verb and record the null.

## 5. I1 — the router is declined; the opening moves ship in the stanza

The literal shape — saved queries chosen at session start by task type —
is structurally impossible on both harnesses: Claude Code's SessionStart
fires before any prompt exists (task-blind by construction) and Codex
has no session hooks at all. Saved query *results* would rot against the
live index; a new pulled surface inherits the measured 0–7% pulled-tool
habit. The owner's own hedge ("maybe redundant") was right about the
mechanism, wrong about the need: the need is real and its home already
exists.

Adopted shape: a **task-shaped opening-moves table** in the existing
stanza (one Go const, dual-written to CLAUDE.md and AGENTS.md, updated by
`fathom init` re-runs — "periodic maintenance" collapses to redeploying a
text table):

    Opening moves by task shape (first 2–3 calls, then edit):
    - debug/trace:    find_symbol → find_callers/references → symbol_context
    - feature/change: repo_map → pack_context(concern) → references
    - review/verify:  verify_plan → obligations → references on changed symbols
    - refactor:       references → outline → symbol_context

~60–80 standing tokens, no hooks, reaches Codex through AGENTS.md.
**Falsifiability floor:** before/after on the zero-fathom-call session
rate (baseline 29/135) and the raw:indexed ratio (~5:1), read from the
existing ledger over ~4 weeks of ON-arm sessions; if neither moves,
revert the lines and record the null.

## 6. I5 — Codex CLI: the delta, not a subsystem

The idea as stated is ~85% shipped: MCP parity, the dual-file stanza,
dual-format skills, harness-aware doctor with honest `skipped`,
git-native gates, and the maintained per-channel parity table
(docs/adoption/codex.md §6) already constitute "work or degrade
cleanly". The inject tier (obligations push, guards, stop-verify, brief
push) is structurally absent on Codex — zero lifecycle hooks — and
honest degradation is the product, not a gap to paper over.

Adopted delta (respecting 04-coding-agents' sequencing rule — only
AGENTS.md/pull-tier work before Claude numbers are green):

- **`fathom prime`** — one CLI verb wrapping the existing
  buildSessionContext + thread brief + custodian line: the exact
  SessionStart payload as plain text. The stanza's Codex instruction
  upgrades from "run `fathom thread brief`" to "run `fathom prime`" —
  full content parity on the pull tier in one command. Harmless
  idempotent read on Claude Code.
- **`fathom init` prints the exact `codex mcp add …` line** (HTTP +
  stdio fallback) next to the `claude mcp add` line it already prints.
- **One honesty line in codex.md §6:** Codex sessions are outside the
  impact observatory's instrumentation (transcript/hook-fed); adoption
  there is unmeasured by design, and law 12 forbids extrapolating
  Claude-arm numbers to Codex.

The architectural enabler behind all of it — registered as direction,
built when it pays: the **channel/vehicle seam**. Payload rendering
(obligations, brief, drift, threads, done-means-proven) factored out of
the hook binaries into one payload layer with per-vehicle emitters:
hook-inject (Claude), MCP pull (portable), stanza file (passive), gate
output (agnostic). The same seam is where **role-variant briefs** land —
the orchestrator brief vs the telegraphic budgeted subagent brief the
operator contract (§6) requires. The split is payload vs vehicle, never
Claude vs Codex; parity becomes a config change instead of a rewrite.

## 7. What the evidence says, and what we may say about it

The 2026-08-21 audit's verdict on "enough feedback to act?": **partial —
act on quality, not yet on savings.** Per channel:

- **Verify gate — the strongest channel, actionable today.** 436
  ledger'd gate runs in five days; 180 of them caught claimed>verified
  gaps (up to 53-claimed/25-verified on one plan); refusals are
  concrete, per-plan, per-commit. This is the product spine and the
  first story to tell. But three verify figures circulate (215/4940
  lifetime-instrumented, 163/608 window, the 436-row ledger) and they
  must be reconciled (the O6 double-count family) before any number
  leaves the repo.
- **Continuity threads — collecting, per the registered instrument.**
  threadBetCheck admits 4 of the 20 fresh-with-brief units the vision-21
  kill requires (≥10-turn, source-stamped, brief-injected sessions
  only). A looser row count says 22 and reads favorably (median first
  edit turn 66 with brief vs 120 resumed) — that is direction, not
  verdict. Keep collecting; do not extend or shortcut the registered
  floor. What IS actionable now is hygiene: condition-closure
  demonstrably works (a spec-unbuilt item auto-closed in 0.6h) while
  unconditioned machine notes rot (0/2 pre-compact ever closed, 14 open
  items, 12 older than a week). Machine-minted notes must carry
  `completes_when` at mint, or not be minted.
- **The A/B is structurally one-armed.** OFF arm: 110 admissible units —
  5× the ≥20 floor (with a named caveat: 12 sessions, nested windows —
  cluster by session, never present N=110 as independent). ON arm: **0
  admissible units** (all 11 rows predate the coverage schema). No
  savings claim is admissible until ON collects. Continuing to burn the
  remaining 23 OFF sessions collects redundant data while the comparison
  stays impossible — so the arm flips ON as soon as the instrument-truth
  wave lands (the OFF arm currently silences its own counting
  instruments, and O2 suppresses the ON arm's treatment effect; fixing
  those first is what makes the next ON window admissible instead of a
  third wasted cycle).
- **Obligations/exploration** — rich lifetime data (7k injections; 5:1
  raw:indexed) but observational; the OFF window's zeros are the
  experiment, not adoption failure.

**The story program.** When a claim survives its instrument, it gets
told properly: `fathom research bundle` (sanitized, shareable) is the
vehicle; the dossier leads with the verify gate (the one channel that is
heavily used, catching real over-claims, and 100% Codex-portable),
states the thread bet and A/B as *registered experiments in flight with
their floors named*, and never ships a figure whose denominator isn't in
the sentence. Honest "collecting, verdict at n=20" is itself the
differentiator — no surveyed competitor pre-registers kills.

## 8. Promotion

Binding rows: `docs/plans/instrument-truth.md` (the honest-absence /
measurement-integrity wave — gates the re-arm) and
`docs/plans/work-frontier.md` (this doc's §2–§6). The operator-contract
custodian concerns (stanza-current, thread hygiene, completion-cites-
instruments) graduate through vision 22's existing engine and ride the
same plans. Kill criteria above are part of the rows they govern.
