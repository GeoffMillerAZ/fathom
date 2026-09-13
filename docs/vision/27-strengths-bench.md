# 27 — The strengths bench: every claim fathom makes, measured against the alternative, with an oracle fathom does not own

**Status:** ACCEPTED 2026-09-13 as ADR-033. Plan: `docs/plans/strengths-bench.md`.
**Builds on:** 26 (proof credibility), ADR-031 (method), ADR-032 (pyramid, cells,
H2-3), the 2026-09-13 reading (`docs/bench/README.md`, O31–O33).

## 0. The verdict in one paragraph

The home bench measured one thing — a cold single lookup with the arms locked
to one tool each — and said fathom answers more often and costs more. That is
true and narrow. Every claim fathom actually makes to an adopter is about a
*change*: fewer wrong edits because the rule arrives at the edit, no false
"done" because the gate refuses it, no re-exploration because the planner's
map travels to the workers, no stale answer because the index moves with
HEAD, blast radius across a fleet that grep cannot see. None of those were on
a ledger. This vision puts each on one, as a row in a matrix whose columns
are fixed: the claim, the thing it is compared against, an oracle fathom does
not own, the readings, and a prediction written before spend of where fathom
wins and — as important — where it must not. A run that only visits the rows
where we expect to win is marketing; the matrix carries the rows where a
language server or a static graph should beat us, and says so first.

## 1. The matrix

| # | Claim | Versus | Oracle (not fathom) | Predicted win | Predicted no-effect |
|---|---|---|---|---|---|
| M1 | A rule served at the edit moment prevents the violation | the same rule as prose in CLAUDE.md | the invariant's own detector: a test, a lint, `cue vet` | rule far from the edit or evicted from context | rule at the top of a short CLAUDE.md |
| M2 | The verify gate lowers false "done" | a plain checklist the agent ticks itself | the PR's failing-to-passing tests and a `go test -cover` profile: the row's function exists and a test executes it | cross-package features, governance-bound changes | single-file bug fixes |
| M3 | Index-grounded plans decompose the real change | plans written from grep | the gold patch's touched functions (`go/parser` over hunks) | changes spanning packages | one-function fixes |
| M4 | Continuity: a later session resumes without re-discovery | a notes file | tokens before the first edit; edits to rows already proven | any change that needs two or more sessions | one-session changes |
| M5 | Exploration once, reused by K workers | each worker explores from a prose row | bytes each worker reads that the planner already read | K ≥ 3, change spans packages | K = 1, one package |
| M6 | The index is right | a static graph's edges (graphify) and LSP servers (Serena, gopls) | `go/types` truth (the L0 oracle) | resolved callers on multi-package repos | single-language references, where an LSP should match or beat fathom |
| M7 | The answer reflects HEAD | a static artifact built before the edit | truth at HEAD after a scripted change | any question about the changed code | questions about unchanged code |
| M8 | Blast radius across a fleet | grep over every repo checked out locally | build and test of every consumer after the change | a shared library with ≥ 3 consumers | a change with one consumer |
| M9 | Breadth beyond Go | the same rows on a TypeScript and a Python repo | the language's own type checker | none claimed | expected to expose weakness; that is the point |

M6 and M7 run free (no model); M8 and M9 are registered here and scheduled
behind the paid pilots (plan wave 6).

## 2. One driver, arms as data (plan wave 2)

The change bench is a single Go driver, `cmd/scenariobench`, over a corpus
of *scenarios* (`testdata/scenariobench/*.json`): a task is a change to a
pinned subject at a pinned commit, a category, a prompt, a session budget,
an oracle, gold, and a prediction per arm. An **arm** is data, never code:
whether the fork is wired (`fathom init`, hooks, MCP, compiled obligations,
the stanza) or stripped; its MCP config; its allowed tools; its instruction
variant; whether it gets a notes file; a setup script for a competitor.
The realistic arm — fathom *plus* Read, Grep and Bash — is the default
treatment; the exclusive arms of the home bench remain for isolating the
index (vision 26). The control is the same fork with the stanza removed,
no hooks, no MCP, and the same instruction to keep notes in a file, so the
bench measures fathom's continuity and not "having any notes".

**Sessions by budget.** A session is one headless run with a turn cap. When
the cap ends it, the next session starts cold in the same worktree with only
what persists: the repo, the notes file, and for the wired arm the thread
brief and primer. Modes: `single` (one agent, N sessions) and
`planner-workers` (one planning session, then K worker sessions each given
one row). Nothing about the prompt differs between arms except what the arm
declares.

**Oracles.** The subject's failing-to-passing and passing-to-passing tests
(from the PR, CC0 sources per ADR-031) decide success. A coverage profile from
the same test run is the reachability oracle for SDD readings. Detectors —
named tests, lints, `cue vet` — decide rule violations for M1. All run after
the last session, in the worktree, by the driver, never by the agent.

**Cells and keys.** ADR-032's contract holds: a cell is remembered under
`sha256(corpus_sha256 · task · arm · model_id · harness_version · scoring ·
prompt_subject · arm_surface)`; `arm_surface` is the fathom commit for wired
arms and empty for controls, so control cells survive every fathom change and
a re-run pays only for the treatment. A competitor arm's surface is its
pinned version.

**Readings on every cell.** Success; tokens, turns and cost per session and
in total; tokens to first edit per session; files read per session; box
checks (`[x]` edits) with turn and symbol; the oracle's verdicts. Everything
below is computed from those, offline, and re-computable (`scenariobench
score`) without spend.

## 3. SDD readings (M2–M4; plan wave 3)

- **False-done rate.** Each `[x]` the agent wrote is matched to the final
  tree: does the row's symbol exist, and does any covered block of the
  coverage profile fall inside its span? A claimed row that fails either is a
  false done. The fathom arm's gate refuses at claim time; the reading is what
  the *independent* oracle says afterwards, for both arms.
- **Plan coverage.** Before the first edit, the fraction of the gold patch's
  functions the plan names; spurious rows counted against both arms. A lower
  bound: a different valid implementation scores low while passing every
  test, so this reading is reported and the tests gate.
- **Resumption.** Tokens before the first edit in sessions ≥ 2, and edits to
  rows already proven done (duplicated work).
- **Contract drift.** After the change, rows whose symbol no longer exists or
  is no longer test-reached.

## 4. Guards (M1; plan wave 3)

Tasks derive from the fleet's own invariants: each is a change a documented
rule constrains, where the naive fix violates it, and the detector is the
invariant's enforcement test. The corpus records the rule, its source line,
and the detector command; the reading is violation rate at the first "done",
tokens to a violation-free completion including redo loops, and whether the
rule was served before the claim (the obligation ceiling, Q21, as a per-task
number). For a public number the same shape applies to repos with a real
`CONTRIBUTING.md`.

## 5. Exploration reuse (M5; plan wave 3)

`planner-workers` mode. The overlap reading is direct: for each worker, the
bytes of files it read that the planning session had already read, over the
bytes it read in total. Beside it: total tokens (planner + Σ workers), tokens
to first edit per worker, tests passed.

## 6. Competitors (M6, M7; plan wave 4)

Each is run exactly as its own documentation says, at a pinned version named
on the row, and `docs/bench/competitors.md` carries the commands. Two
comparisons. Free: their edges against the L0 `go/types` truth beside
fathom's — a static graph's call edges loaded from its output, an LSP's
references probed through its MCP tools for every truth symbol. Paid: the
home lookup tasks with each tool as an arm, every arm with Read and Grep,
plus a freshness task (M7) where the code changes after the artifact is
built. Predicted before spend: a community summary wins whole-repo
orientation; an LSP matches or beats fathom on single-language references;
fathom's edge is resolved callers across packages, freshness, and everything
that is not a symbol lookup at all.

## 7. Repeatable by a stranger (plan wave 5)

Every corpus is derived from pinned public sources with the derivation
recorded per task (ADR-031), categories assigned by the patch's shape
(`ClassifyChange`), tasks merged after the model's cutoff preferred and the
leak rule kept. `docs/bench/scenarios.md` is the runbook: pins, one command
per layer, expected cost per tier, how to add an arm, how to add a task, how
to publish a row. `task bench-scenario-*` wraps each. The free layers run in
`task validate`.

## 8. The doctrine changes (plan wave 1)

The run showed the "index before any grep" instruction is over-broad. The
stanza fathom writes, the skills and the adoption guides route **by question
shape**: where is it, who calls it, what breaks, what governs this file,
what did the last session know — the index; a literal string, a config
value, a log line — grep. Tool descriptions say when a result *is* the
answer. `find_symbol` on an interface carries its implementations;
`read_source` carries a budget and says when not to reach for it;
`references` costs ≤ 100 bytes an occurrence and refuses a query that is not
a symbol, a dotted path or a declared regex.

## 9. Cost and sequence

| Tier | What | Cost |
|---|---|---|
| free | L0 oracle, payload replay, corpus gate, competitor edge comparison, fake-harness driver tests | 0 |
| targeted | `bench-targeted` over the losing lookup tasks; guards pilot on the fathom repo | ≈ $20–40 |
| pilot | change bench, 8 public Go tasks × 2 arms × 2 trials × 2 sessions; exploration reuse K=3 on 3 tasks | ≈ $90–130 |
| credible | 30 tasks × 3 trials per matrix row | several hundred dollars per row; operator decision |

Order: wave 1 (lessons, doctrine) → 2 (driver) → 3 (readings, guards) → 4
(competitors) → 5 (public corpus, runbook) → 6 (pilots, ledger rows) → 7
(docs, skills, artifact). Control cells borrow forward across every fathom
change.

## 10. What would falsify this vision

- M2 or M3 shows no effect on the cross-package and governance-bound
  categories at n=3: the SDD story is prose, not product.
- M5's overlap is under 30% in the control: workers do not re-explore much,
  and the pack story is cheap either way.
- M6 shows an LSP beating fathom on multi-package resolved callers: the
  index's core claim is wrong and vision 21's typed edges are the fix, not
  more bench.
- Payload stays over 1.0× after wave 1 on the targeted run: the lookup tools
  are the wrong shape and should be replaced by answer-shaped tools rather
  than trimmed.
