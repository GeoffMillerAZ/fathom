# Coding-agent excellence — the companion spec

*Status: adopted 2026-07-07. Owner: the fathom program. Companion to
`product-vision.md` (use-case verdicts, H1 roadmap) and `semantic-layers.md`
(ADR-016, obligations layer). This document is the **full specification** of the
agentic-coding-companion use case: the loop model, tokenomics, SDD and planning
integration, the enforcement architecture, and the anti-staleness contract.
Where product-vision says "world-class — with delivery flipped to forced
channels", this doc says exactly what gets forced, where, and how it stays true.*

## 0. Definition of excellence

Fathom is excellent as a coding-agent companion when, in an indexed repo:

1. **Discovery is index-first by construction** — the agent's cheapest correct
   move is a fathom query, and ≥40% of discovery actions flow through fathom
   (sustained 4 weeks; measured share before enforcement was 0–7%).
2. **Every edit is informed** — ≥90% of Edit/Write operations in indexed repos
   receive an injected, ≤150-token, citation-bearing obligations payload
   (zero-output-on-clean, so "informed" costs nothing when there is nothing
   to say).
3. **Done means proven** — a task claimed complete (`[x]`) cannot pass the
   push/CI boundary unless its acceptance contract verifies against the index,
   with file:line evidence.
4. **The index is never silently wrong** — data, binary, and claim staleness
   are each detected automatically and surfaced honestly at the moment they
   matter (§6).
5. **The win is falsifiable** — context-bench, run for real (first run
   2026-07-23, docs/plans/falsifiability.md B1-3), measured equal task
   success (15/15 both arms) at 1.38× aggregate end-to-end token reduction
   and 2.81× mean wall-time reduction vs grep-only. The original
   "≤1/10th the discovery tokens" form of this sentence is **retracted**
   (the pre-registered <5× criterion fired); the standing bar is
   equal-or-better success at measurably lower tokens and wall time,
   re-verified by re-running the bench (`scripts/context-bench.sh`).

Every mechanism below exists to serve one of those five sentences; anything
that serves none of them is out of scope for the companion.

## 1. The loop model: decision moments, not features

A Claude Code session is a small set of recurring decision moments. The
companion is specified as *what fathom delivers at each moment, over which
channel*. Channels are ranked by force (see §5): **blocking gate** > **injected
context** > **pulled tool call** > **passive docs**.

| # | Moment | The agent's question | Fathom's answer | Channel | Status |
|---|--------|---------------------|-----------------|---------|--------|
| M1 | Session start | What index exists? How do I work here? | service, symbol count, operating contract | SessionStart hook (injected) | **live** (since 2026-06-17) |
| M2 | Task intake / planning | What's the shape of this codebase? What will this change touch? | `repo_map`, `brief`, `pack_context`, `blast_radius`, `test_impact` | pulled (companion surface) | live |
| M3 | Plan authoring | How do I write a plan whose "done" is checkable? | contract grammar in plan markdown (§4) | convention + `verify_plan` | live parser; checkbox semantics **this session** |
| M4 | Discovery hot loop | Where is X? Who calls it? What's its context? | `find_symbol` (tiered), `symbol_context`, `references`, `find_callers`, `outline`, `read_source` | pulled (16-tool companion surface) | live |
| M5 | About to edit | What is this file *obliged* to do? Is my view of it current? | obligations claims + staleness flag | **PostToolUse(Edit\|Write) hook (injected)** | **this session** (ADR-016 tracer bullet) |
| M6 | Claiming done | Is `[x]` actually true? | `verify_plan` verdicts citing file:line | pulled + **Stop-hook option** | live tool; gate **this session** |
| M7 | Commit | Does the binary/index track HEAD? | post-commit rebuild + re-ingest | lefthook post-commit | live |
| M8 | Push / CI | Do all claimed-done tasks verify? | `fathom verify` exit code | **lefthook pre-push + CI (blocking)** | **this session** |
| M9 | Review | What does this diff break? What spec does it satisfy? | `diff_impact`, `spec_coverage`, `references` | pulled (full surface) | live |

The moments where excellence was previously *lost* are M5, M6, M8 — precisely
the moments that had **no forced channel**. Telemetry proved the pulled-only
strategy fails (0–7% share); this spec's center of gravity is therefore §5
(enforcement).

## 2. Tokenomics — the measured cost model

The original complaint ("fathom is 34% of my Claude usage") decomposed into
facts worth restating because they drive every budget rule below:

- **Schema cost is solved and is not the problem.** Claude Code defers MCP tool
  schemas: the 86-tool surface costs ~570 tokens of names per session, ~8,650
  only if every schema were loaded (they aren't; schemas load on demand).
  Cutting tool *count* saves almost nothing per session; cutting **response
  payloads** and **wasted calls** saves everything. (Tool count still matters
  for *selection quality* and support surface — hence the companion allowlist
  and the H1 kill-gate — but not for tokens.)
- **Payloads dominate.** The expensive failure mode is a discovery sweep:
  Grep → 4–6 Read calls at ~500–2,000 tokens each (5–20k tokens for one
  "where is this handled?"). The fathom counterfactual is one `symbol_context`
  or tiered `find_symbol` call at ~300–800 tokens. **Measured
  (context-bench first run, 2026-07-23): that payload-level gap does not
  survive end-to-end** — each subagent call carries a symmetric ~30–100k
  session baseline that dominates totals, and the measured end-to-end
  reduction was 1.38× aggregate / 1.79× mean per-task, not ×10. The ×10
  form is retracted (docs/plans/falsifiability.md B1-3/B1-4); payload
  discipline still matters, but its end-to-end effect must be stated at
  the measured magnitude.
- **Injection is cheap because silence is free.** Hook-injected context costs
  tokens only when emitted. The obligations payload is capped at ≤150 tokens,
  zero-output-on-clean; the session-hook line is ~80 tokens once per session.
  A silent hook costs 0 — which is why forced channels can be defaulted on.

**The payload contract** (binding on every companion-surface tool, enforced by
review + byte-golden tests):

1. Bounded output always: hard caps (find_symbol substring tier caps at 20,
   suggestions at 5; staleness examples at 3) — never unbounded lists.
2. `omitempty` everywhere; empty sections are absent, not `[]`.
3. Compact identifiers the agent can act on (`file:line`, symbol keys), never
   file content dumps — `Read` is for editing, fathom is for locating.
4. Descriptions are budgeted prose (the verify_plan description was cut 27%
   with zero semantic loss; that discipline is the norm).
5. Every response that *could* mislead carries its honesty marker: tier,
   staleness flag, `matched_via`, watermark.

**Budget ledger per moment** (steady-state session in an indexed repo):
M1 ~80 tokens; M4 ~300–800/query (vs 5–20k counterfactual); M5 0–150/edit;
M6/M8 verdict payload ~200–600, out-of-band in lefthook/CI (0 session tokens);
M2 `brief`/`pack_context` are the largest single payloads (1–3k) and exist to
*replace* multi-file Read chains (5–30k). Net: fathom-first sessions spend
fewer tokens in absolute terms while touching more of the codebase.

`verify_plan` is the ledger's one latency-budgeted (not token-budgeted) line
(B3, docs/plans/falsifiability.md): measured p50 2.2s / p95 4.5s, against an
explicit **p95 ≤ 5s SLO**. It is the single expensive companion call — every
other moment above is bounded in tokens because it answers from the index in
one hop; `verify_plan` additionally walks the call graph and the contradiction
index per claimed task, so its cost shows up in wall time, not payload size.
The Stop-hook caller (§5.2, cmd/fathom `stophook.go`) budgets a client
timeout at the same 5s ceiling: a `verify_plan` call that exceeds it degrades
to an advisory warning ("verify timed out — treat as unconfirmed"), never a
blocked session end (§5.3 — hooks never block on infrastructure).

## 3. Power use cases for Claude Code (the effectiveness catalog)

What the companion makes *possible or measurably cheaper* in Claude Code
specifically (the "10× cheaper" phrasing is retracted — see §0.5 and
docs/plans/falsifiability.md B1-3; measured: ~1.4–1.8× tokens, ~2.8× mean
wall time, plus the governed capabilities — verify gate, obligations,
staleness honesty — that have no grep equivalent and were not measured by
that bench):

- **Cold-start comprehension:** `repo_map` + `brief` give a new session the
  load-bearing shape of a repo in one call each — replacing the first-hour
  Read/Grep spiral. Excellence bar: first-hour grep count <10 in a new repo.
- **Cross-repo questions in one query:** the fleet index makes "who, anywhere,
  calls this?" (`references`, `cross_repo_callers`, `blast_radius`) a single
  call — Claude Code has no native cross-repo capability at all. This remains
  the companion's unique structural advantage over harness-bundled context.
- **Plan-mode grounding:** in plan mode, `pack_context` + `test_impact` turn
  "I think this touches X" into an evidence-cited plan with a checkable
  contract per task (§4) — the plan itself becomes a verifiable artifact.
- **Subagent economy:** background/parallel subagents inherit MCP access to the
  daemon; a Sonnet worker resolving symbols through fathom instead of Grep
  keeps worker context small, which is what makes large worker fleets viable.
- **Honest self-doubt:** `staleness_report` + injected staleness flags give the
  agent a machine answer to "can I trust what I remember about this file?" —
  no other channel in a Claude Code session provides that.
- **Evidence-backed completion:** `verify_plan` lets the agent *disagree with
  itself* — a claimed-done checklist item is checked against symbols, call
  reachability, static test coverage, and spec references before the human
  ever reviews. This is the headline (product-vision §3 row 1).

## 4. SDD + planning: the full spec

Fathom's SDD stance: **specs are IDs, code cites them, plans carry contracts,
and "done" is a verifiable proposition** — all deterministic, no LLM in the
verify path.

### 4.1 Spec references

A spec is any stable bracketed ID (`[US-042]`, `[EP-013]`, `[ADR-017]`)
declared in docs and cited in code comments/names. `internal/sdd.ExtractRefs`
harvests them at ingest; `spec_refs`, `spec_coverage`, `spec_status`, and
`traceability` serve the matrix (spec → implementing symbols/files → tests).
Convention for fleet repos: new work that implements a documented decision
cites its ID at the primary implementation site — one citation at the
load-bearing symbol, not scattered.

### 4.2 Plan files and the contract grammar

Plans live in the repo (convention: `docs/plans/<slug>.md`) as ordinary
Markdown checklists — the format humans and plan mode already write — with an
optional **acceptance contract** per line:

```markdown
- [ ] Wire token validation into the router [US-042] {symbol: ValidateToken, reachable_from: Router, min_tier: verified}
- [ ] Document the retry policy {symbol: RetryPolicy, skip_test: true}
- [x] Spike: is DuckDB VSS viable?            ← no contract: verifies as Unknown, never a false pass
```

Grammar (parsed by `verify.ParseMarkdownPlan`): checklist lines `- [ ]` /
`- [x]` / `- [-]`; bracketed spec IDs anywhere in the title; one `{key: value,
…}` annotation block with keys `symbol`, `reachable_from`, `min_tier`
(verified | pre-scan | doc-claimed | sme-stated | assumed), `skip_test`,
`allow_contradiction`.

**Checkbox semantics (normative, new):**

- `[ ]` **open** — listed in the report, never gates.
- `[x]` **claimed done** — MUST verify. This is the entire point: checking the
  box is a *claim*, and claims are checked.
- `[-]` **skipped/abandoned** — exempt, reported as skipped.

A claimed task with a contract that fails ⇒ **FAIL**. A claimed task with no
contract and no spec IDs ⇒ **Unknown**: reported loudly, gates only under
`--strict` (recommended for CI, not for pre-push — see §5.3 fail policy).

### 4.3 The verify loop in practice

1. Plan mode (or the human) writes `docs/plans/<slug>.md` with contracts —
   grounded via `pack_context`/`test_impact` so `reachable_from` names real
   entrypoints.
2. Work proceeds; boxes get checked as work completes.
3. `fathom verify --plan docs/plans/<slug>.md --service <svc>` runs at M6
   (agent, in-session), M8 (lefthook pre-push, blocking), and CI. It re-parses
   the plan, queries the live index, and returns per-task verdicts with
   file:line citations.
4. A `[x]`-but-unproven item blocks the push with the exact failing criterion
   ("symbol Ghost: not found in index"; "ValidateToken: no test statically
   reaches it"). The fix is either doing the work or honestly unchecking the box.

Static honesty is preserved end-to-end: a reached test is not a passing test —
`task validate` (also in pre-push) owns *pass*; fathom owns *wired, reachable,
covered, spec-cited, tier-sufficient*. The two gates are complements, never
substitutes.

## 5. Enforcement architecture — forced channels, honest failure

The measured lesson (0–7% pulled-tool share) hardened into doctrine in
product-vision §2: **delivery is configuration, not agent habit.** The
enforcement stack, strongest first:

### 5.1 L1 — blocking gates (lefthook + CI)

- **pre-push `verify`** (new): `fathom verify` on the repo's active plan(s),
  gating only `[x]` claims (§4.2). Runs beside the existing `task validate`.
- **CI**: same command with `--strict` in `scripts/ci.sh` — CI is the
  fail-closed environment (§5.3).
- Existing and unchanged: pre-commit gofmt/cue-vet, pre-push `task validate`,
  post-commit binary rebuild + re-ingest (the M7 staleness defenses).

### 5.2 L2 — injected context (Claude Code hooks)

- **SessionStart** (live): `fathom session-hook` — daemon health, service
  identity, operating contract, honest degradation ("daemon down — Grep/Read
  expected this session").
- **PostToolUse(Edit|Write)** (new): `fathom edit-hook` reads the hook JSON
  from stdin, resolves the edited file, and injects: (a) obligations claims
  scoped to that file (≤150 tokens, ≤6 rules, each with `src:` citation and
  tier, stale claims flagged — ADR-016 §4.5), and (b) an index-staleness
  warning when the file's indexed hash drifted from disk. Zero output on
  clean — the common case costs nothing.
- **Stop-hook strict mode** (opt-in, per repo): at Stop, run `fathom verify
  --quiet-pass` on the active plan; if a `[x]` item is unproven, emit blocking
  feedback so the session cannot *end* on an unproven claim. Off by default
  (repos opt in when their plans carry contracts); this is the strongest
  in-session form of "done means proven".

### 5.3 Failure policy (the trust-fatal rule)

A forced channel that lies or blocks for infrastructure reasons gets
uninstalled within a week — that is the real-world failure mode of every
over-eager gate, and it is treated as a design constraint:

- **Hooks never block on infrastructure.** Session/edit hooks always exit 0;
  a down daemon degrades to an honest one-line notice (session) or silence
  (edit), never an error, never a stall (2s HTTP timeouts; the one recorded
  exception is the Stop-hook's `verify_plan` call, budgeted at that tool's
  p95 ≤ 5s SLO — §2 — because a 2s cut-off would fail-open on roughly half
  of legitimate verifies at the measured 4.5s p95).
- **Pre-push fails open on operational error, closed on evidence.**
  `fathom verify` distinguishes exit 1 (a claimed task FAILED verification —
  block the push) from exit 2 (daemon unreachable / service unindexed —
  **warn loudly, allow the push**). A gate that blocks pushes because a
  launchd job died punishes the wrong behavior.
- **CI fails closed** (`--strict`, exit 2 also fails): CI owns determinism —
  it builds its own store (`fathom ingest` into a temp store, then `verify
  --store`), so "daemon down" is not a valid CI state.
- **No grep-blocking, by decision.** PreToolUse interception of Grep/Read is
  rejected: false-positive friction at the agent's most reflexive action is
  exactly the trust-fatal pattern, and the miss-taxonomy work (H1.5) showed we
  don't yet know which greps the index could have answered. Revival evidence:
  if post-injection telemetry shows ≥30% of greps in indexed repos were
  index-answerable, reconsider a *hint* (never a block).

### 5.4 L3 — advisory (docs, CLAUDE.md, gen-claude-md)

Unchanged, and demoted honestly: passive guidance sets vocabulary, it does not
change behavior (measured). CLAUDE.md text about fathom is kept short and
points at the forced channels; `gen-claude-md` regenerates per-repo guidance
opportunistically.

### 5.5 Distribution of the enforcement stack

One command wires a repo (H1.6, `fathom init`): `.fathomignore`, MCP
registration (or daemon reuse), SessionStart + PostToolUse hook entries in
`.claude/settings.json`, lefthook snippets, and a starter plan file. Until
`init` ships, `docs/use-cases/getting-started.md` carries the manual recipe;
the fathom and one sibling fleet repo are wired by hand this session as the
two reference installations (H1 exit criterion).

## 6. Anti-staleness — the four classes, each with a detector and an owner

Staleness is the way this whole design dies (DeepWiki lesson: days-stale
answers with no indicator). Every class of drift has an automatic detector, an
automatic or one-command remediation, and an honest surface:

| Class | Drift | Detector | Remediation | Honest surface |
|---|---|---|---|---|
| **Data** (index vs repo) | commits land, index lags | daemon auto-refresh on git-HEAD drift per request; hash-gated (~0.2s clean) | automatic; `ingest` tool to force | `staleness_report`; edit-hook per-file flag (M5) |
| **Working-tree** (index vs uncommitted edits) | heavy local edits mid-session | per-file content-hash vs disk at edit-hook time | none needed — read the file you edit | edit-hook injects "index is behind your working tree for this file" |
| **Binary** (daemon vs code) | committed fix, stale daemon binary | post-commit rebuild keeps `bin/fathom` at HEAD; `fathom version` vs `git rev-parse HEAD` | `launchctl kickstart` (operator-run, never from an agent session) | session-hook daemon-version line; dogfood discipline in CLAUDE.md |
| **Claim** (compiled knowledge vs corpus) | governing doc edited after compile | post-ingest citation sweep re-hashes cited lines → status flip; query-time status check | `fathom compile --layer` (digest-gated, pennies) | claims served with `stale`/`contradicted` flags, never silently dropped or served (ADR-016 §4.3) |

Plus the meta-class this program was just bitten by: **instrument staleness** —
the adoption script silently measuring nothing after the rename. Rule: every
measurement instrument gets an existence check in its output ("0 matching
transcript dirs" is an error, not an empty report), and renames are a named
migration concern (`docs/rename-migration.md` pattern).

## 7. Excellence gates (what "don't stop until excellent" means, measurably)

From product-vision §3/§4, restated as this use case's release gate:

1. Instrument valid ≥4 weeks (weekly baseline committed; both name eras
   counted).
2. Injection coverage ≥90% of Edit/Write in indexed repos (from
   `mcp-calls.jsonl` + hook logs), payload p95 ≤150 tokens.
3. Verify gate live in ≥2 repos ≥4 weeks; ≥1 recorded catch of an
   `[x]`-but-unproven item (the demo).
4. Fathom share of discovery actions ≥40% sustained 4 weeks (from 0–7%).
5. The four recurring violation classes (`time.Now()` in query logic, unsorted
   outputs, org-specific names, non-`workspace/` outputs) trending to zero in
   review.
6. Context-bench: equal-or-better task success at lower cost. First run
   (2026-07-23): equal success 15/15, 1.38× aggregate tokens, 2.81× mean
   wall time — the "≤1/10th discovery tokens" form of this gate is
   retracted (docs/plans/falsifiability.md B1-3/B1-4).

Failing gates trigger the ADR-016 fallback ladder (fold obligations into
`outline`/`symbol_context` as a capped rider; kill layers that miss precision),
not silent persistence.

## 8. Implementation deltas (this session's checklist)

1. `verify.Task` gains claimed/skipped checkbox state; `verify_plan` reports
   it; **`fathom verify` CLI** (HTTP-first against the daemon, `--store` for
   CI/no-daemon; exit 0/1/2 per §5.3) with `--strict`.
2. lefthook pre-push verify gate here + one sibling repo; CI strict step;
   `docs/plans/` established with the H1 plan as the first contract-bearing
   dogfood plan.
3. `fathom edit-hook` (PostToolUse): obligations + working-tree staleness,
   zero-on-clean; wired in `.claude/settings.json` here + one sibling.
4. Adoption instrument repaired (name-era aliasing, weekly rollup, call-log
   section).
5. ADR-017 portability + Watermark SPI (prerequisite for claims DDL —
   product-vision H1.3), then the ADR-016 claims schema + obligations compile
   + citation sweep (H1.4).
