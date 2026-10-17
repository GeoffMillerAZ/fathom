# Decision models: a hexagonal port, consent and kill, and stage-gated agentic experiments

Status: accepted 2026-10-16 (ADR-073). Plans: `docs/plans/decision-port.md` (KS-, DP-41),
`docs/plans/agentic-decisions.md` (EXP-, AQ-), `docs/plans/readouts.md` (RJ-), `docs/plans/judgment-foundations.md`
(JG-). Goal G42. Assessment: `docs/assessments/2026-10-16-decision-models.md` (the 2026-10-16 decision-model pass,
eight research agents, report only). Questions: Q37. Invariant: I66.

This document extends, and does not repeat, three designs: `docs/vision/32-decision-port.md` (the port, ADR-042),
`docs/design/judgment-foundations.md` (per-site opt-in, rules R1 to R8, calibration readings) and
`docs/design/judged-predicates.md` (J0 and `fathom ask`, section 3). Where they disagree with this one, ADR-073's
amendment lines say which wins.

## 1. Verdict

Decision models pay now only as zero-spend scaffolding: the opt-out and consent fix, disclosure, the fitted readout
table, and the `ask` spine at provider `none`. No judged site has passed a bar: route was right 45.7% of the time at
confidence 0.9 or more and its confidence did not track correctness
(`docs/bench/readings/decision-port-calibration-2026-09-24.md`), and the within-tier re-rank read -0.19 on governed
edits (`docs/bench/readings/decision-port-ab-2026-09-23.md`). The one unknown that decides everything else is whether
a fast decision model reads code well, and it has never been measured (Q31). So the owner's direction of 2026-10-16
splits the program: make the port hexagonal and killable now, skip provider calibration, and spend a modest budget on
experiments that grade a model against truth fathom computes.

## 2. The port is hexagonal

Fathom's core speaks only the port's three kinds. A site builds a `decision.Question` from facts it computed and reads
a `decision.Response`; everything vendor-shaped stays in an adapter package.

| Port (`decision.Kind`, agentic-kit `foundations/decision/kind.go`) | Jev (TypeSafe) | OpenAI Decisions | Fathom config and `decisions.jsonl` |
|---|---|---|---|
| `predicate`, a yes or no question; decodes the alias `noul` | `noul` | `predicate`, answered by `probability` | rows that say `noul` still read (JF-24) |
| `choice` among named options | `choice` | `choice` with choices (value and description), answered by `choice`, `probabilities[]` and `confidence` | `choice` |
| `score` on a range or ordered levels | `score` | `score` with ordered levels (label and description), answered by a probability-weighted level index, `probabilities[]` and `confidence` | `score` |

No rename: the shipped name stays and this table is the mapping. Refusals arrive as `Declined` and are never read as an
answer (ADR-042's 2026-10-09 amendment).

**What never crosses the port.** A provider's type, a wire field name, a threshold or a prompt. Prompts and wire
fields live in `foundations/decision/{llm,typesafe,openai}`; thresholds are per-site configuration
(`[decide] compile_threshold`, `route_confidence`, and the bands JF-23 reads from a calibration reading); only
`internal/decide` imports an adapter package (KS-9 pins it).

**Conformance before use.** The kit's conformance kit, `decisiontest.RunConformance`, runs against each of its
adapters today (`llm`, `typesafe`, `openai`; each package's `conformance_test.go`). A site may use only an adapter that
passes it; KS-8 fails when `decide.Open` can build a provider without such a run.

**Switching providers is a calibration rerun.** A new adapter or model needs no site change: `fathom decide calibrate
--provider X` (JF-12) re-grades each site and the bands follow the newest reading (JF-23). Calibrating Jev against
OpenAI is deferred by the owner (2026-10-16, DMP-6, EXP-D2); until a site's reading clears ADR-056 R3, a judged answer
is shown `uncalibrated` and never prunes (I62).

## 3. Provider landscape

Read 2026-10-16. Grades: *measured* (fathom read it), *claimed by a vendor*, *secondary* (a public article, not the
vendor's own page), *unverified*.

| Provider | Returns | Calibrated | Latency | Price | Access | Verdict |
|---|---|---|---|---|---|---|
| `none` | the rule's or table's answer | not applicable (computed) | local | $0 | the default (I42) | the product, and every fallback |
| Jev native (`typesafe`) | choice, score and predicate with confidence, batched | vendor claim only, unverified; route failed R3 | 70 to 500 ms (secondary); fathom's own p50 337 ms, p95 1,075 ms (measured, 2026-09-23 A/B) | $0.042 per million input tokens, output free (secondary) | waitlist since 2026-09-15 | primary; a reading per site first |
| Jev through OpenRouter (`transport = "openrouter"`) | the same | the same | 256 to 516 ms (measured, `docs/bench/readings/decide/provider-smoke-2026-10-09.md`) | 8 calls for $0.00017 (measured, same) | live | the working lane, and the experiments' lane |
| OpenAI Decisions (`openai`) | predicate: `probability`; choice: `choice`, `probabilities[]`, `confidence`; score: `score`, `probabilities[]`, `confidence`; refusal answers | no claim; the guide advises thresholds from one's own labelled examples | "about 10x faster than the Responses API", no absolute figure (vendor); about 268 ms median (secondary) | $0.10 per million input tokens, input only, regional and long-context multipliers (vendor) | public beta (GA "in the coming weeks"), `POST /v1/decisions`, the one model `gpt-6-luna` (read from the vendor's guide 2026-10-16; another public source names a different model: unverified); zero data retention and HIPAA for eligible customers, US and EU residency, a Go SDK; limits, batching, rate limits and caching not stated; fathom's account had no credit (DP-38) | second; its own reading; spend outside the $10 Jev budget, asked first |
| A general LLM's enum with log probabilities, or verbalized confidence | log probabilities or a stated number | self-calibration, unverified | about 10x slower (vendor comparison) | the model's rate | general availability | rejected as a decision provider (the `llm` adapter stays for the CLI and background work) |
| A local log-probability classifier (`llm` on a loopback profile) | enum log probabilities | calibratable | unverified | $0 | self-hosted | a future no-egress arm |

## 4. Consent, kill and disclosure

**The hole.** A repository may set `[decide] provider` and `transport` (behaviour class), and with no key source
named the adapter reads the transport's well-known variable; the default sites are on. A cloned repository can so make
`fathom compile`, `certify suggest`, `assess` or `plans context --preflight --score` spend the user's key (ADR-073's
context cites the lines). Provider `none` itself is safe: it returns `decision.None{}` before any key read, exec or
dial.

**The opt-out (KS-1 to KS-7).**

| Control | Scope | Default | Effect |
|---|---|---|---|
| `FATHOM_DECIDE=off` | environment | unset | beats every layer; checked first in `decide.Open`, so per process: a shell export never reaches a running daemon |
| `~/.fathom/decide.off` | a file | absent | a live kill in every process, a running daemon included: `stat` before each ask, never read |
| `[decide] enabled` | user-only | `true` | `false` is a hard off, re-checked before each ask (the user config reloaded when its mtime changed) |
| `[decide] allow_repo_provider` | user-only | `false` | a provider or transport only the repository names stays closed; a well-known key variable counts only when the user layer named the provider |
| `[decide] session_sites` | user-only | `[]` | the only place `readout_mode` and `compact_advice` are opted in; any layer's `sites` naming one is dropped and named |
| `[decide] deny_sites` | user-only | `[]` | refuses a site whatever any list says; a repository vetoes by narrowing its own `sites` |
| `[decide] max_asks_per_site_per_session` | behaviour | 200 | the minimum across layers |
| `[readouts] mode` | user-only | `off` | readouts' own switch (ADR-071) |

Precedence: environment, then file, then `enabled`, then consent, then `deny_sites`, then `sites` and `session_sites`;
the first that closes the port is named as `kill_source`. Off means zero key-variable lookups, key-file reads,
`api_key_cmd` execs and dials, I42's bytes, zero `judged` tokens, and a doctor row that names the source and calls
nothing. `decisions.jsonl` gains `kill_source` and `consent_layer` (additive fields in the existing file).

**Disclosure (JG-1 to JG-5).** `decided_by: computed | table | override | judged(<site> · <provider>/<model> · conf
<x> · calibration <status> <n>/100)` on every answer a configured port served; an override shows the table's answer
too; a miss is disclosed once per session, site and class; a judged badge is never styled as a pass and never enters
the mods' evidence band (M3) or proof line (M4); the status line and band carry `J:off`, `J:on(<n>)` or `J:killed`.
With provider `none` the field is absent, so I42 holds.

## 5. The idea list

Value uses the scale of `docs/design/readouts.md` section 20.1. Status: *planned* (open rows), *done* (recorded and
proven), *gated on the owner* (deferred rows), *future* (a futures entry).

| Id | Idea | Site | Value | Gate | Phase | Status and rows |
|---|---|---|---|---|---|---|
| DMP-1 | Kill switch and user consent | `decide.Open` | Very high | none | readouts wave ro1, first | planned: KS-1 to KS-7; I66 |
| DMP-2 | R-12 split: readouts' view on stated terms, command execution refused | ADR-071 | Very high | owner | with readouts | done: ADR-071 status line and decisions 2 and 3; RO-1, RO-5, RO-6 carry the contract |
| DMP-3 | Disclosure: `decided_by`, a badge never styled as a pass | served surfaces, mods | High | readouts phase 1; mods | readouts and mods | planned: JG-1 to JG-5 |
| DMP-4 | Outcome labels and replay sets for five sites | rerank, route, certify, Q2 | High | the owner lifting calibration | none now | gated on the owner: EXP-D1 |
| DMP-5 | Rich mode's contested band answers from a fitted table; the port only in its residual band | `readout_mode` | High | RO-17's replay | readouts phase 2 | planned: RJ-1, RJ-M1 |
| DMP-6 | Each provider an adapter with its own reading, Jev against OpenAI | the port | High | the owner; DP-38 credit | none now | gated on the owner: EXP-D2 |
| DMP-7 | The agent A/B bench, arms agent-reads, ask at `none`, ask on Jev, raw ask | `ask` | High | stage 1's signal, the owner | stage 2 | gated on the owner: `judged-predicates.md` JP-14, trigger amended |
| DMP-8 | The question spec: schema, refusals, caps, dry run | `ask` | Medium | none | after readouts and mods | planned: AQ-1, AQ-3 |
| DMP-9 | `fathom ask` at provider `none` and its skill | `ask` | Medium | Q-DM3 (yes) | after readouts and mods | planned: AQ-2, AQ-5, AQ-6 |
| DMP-10 | Compute-first columns | `ask` | Medium | AQ-2 | after readouts and mods | planned: AQ-4, sharing EXP-2's facts |
| DMP-11 | The judged column, batched, on the ledger | `ask` | Medium | EXP-M4 OPEN | stage 1 | planned: AQ-7, EXP-M5 |
| DMP-12 | Forced-moment templates: callers in a blast radius, the edited function in a readout | `judge_*` | Medium | R3, spend | none now | gated on the owner: AQ-D1 |
| DMP-13 | Asymmetric options at `readout_mode` | `readout_mode` | Medium | RO-32 | readouts phase 4 | planned: RJ-2 |
| DMP-14 | The enrichment-record choice | `readout_mode` | Medium | RO-32, RO-24 | readouts phase 4 | planned: RJ-3 |
| DMP-15 | Agent-authored readout rules, checked by held-out replay | `readout learn` | Medium | RO-27 | readouts phase 2 | planned: RJ-4, RJ-M2 |
| DMP-16 | An offline claim-phrase miner for H2's table | H2 | Medium | labels beyond ADR-056 decision 14 | none now | gated on the owner: RJ-D1 |
| DMP-17 | Caps and deadlines | the port | Medium | none | with readouts | planned: KS-10 |
| DMP-18 | In-process cache keys | the port | Medium | none (durable after RT-M1) | with readouts | planned: KS-11 |
| DMP-19 | Per-site kill criteria | every site | Medium | none for misses; JF-11 for readings | with readouts | planned: KS-12, KS-13 |
| DMP-20 | Template promotion from recurring free-form questions | `ask` | Low | 30 days of `ask` use | future | future: F80 |
| DMP-21 | Evidence timing: when the mods show a fact | M2, M3, M14 | Low | RT-M1, events | future | future: F81 |
| DMP-22 | Model-ordered tests at H4 and M12 | `tests select` | Low | 100 fail-then-green runs | future | future: folded into F35 |

Also from ADR-073: the hexagonal pins KS-8 (conformance) and KS-9 (imports), and DP-41 (every new record carries a
`## Decision models` section).

## 6. The agentic query concept: F34 `fathom ask`

The level-10 idea (the agent writes the questions; agentic-kit `docs/fast-judgment.md`) made safe by fathom computing
first. Promoted in part on 2026-10-16 (Q-DM3, ADR-073 decision 10): the spine at provider `none` is built; the judged
column waits for stage 1.

1. **The agent writes a typed question spec** (the "decision config"): a kind (`predicate`, `choice`, `score`), the
   question, options drawn `from:` an index fact (the repository's arch-rule layer names, symbol kinds, a field list)
   with a mandatory `none` exit, and `cannot_tell` counted with its own rate (an unknown answer never abstains
   silently). The schema refuses a security property (I62, JF-20), a shape fathom computes (counting, dates,
   reachability, callers, exported: the refusal names the verb that answers it, R1), double negatives, and any field
   naming an executable, a URL or a provider. Caps: rows (default 60, at most 255) and questions.
2. **Fathom selects rows.** Rows arrive as an existing verb's JSON on stdin (`fathom callers`, blast radius, tests
   reaching), so the selectors are the verbs and no operator grammar is added (JP-X1; ADR-056 decision 13).
3. **Fathom computes first.** For Go rows, facts computed from the syntax tree answer their questions outright: a test
   with zero assertions only executes; a returned error is checked or not; errors are wrapped with `%w` or not; a
   lock is taken before a guarded field is written; a context is threaded; the row's arch-rule layer. These are the
   same facts E1 uses as truth (EXP-2).
4. **The model labels only the residue,** and only in stage 1: one batched request per chunk (`decisionBatchPlan`:
   about 2k tokens per row, 64 questions and 96 KiB per request), each answer labelled `judged` and `uncalibrated`.
5. **A receipted table** keeps computed and judged rows apart: `{key, span, unit_hash, tier, answer, p, band,
   template@v, calibration, cache, decided_by}`, `pruned[]` naming every row a computed operator dropped and why, and
   cost `{calls, input_tokens, usd, source}`. At provider `none` the judged tier reads `judged: not consulted
   (provider none)`.
6. **Never proof** (I62): a judged answer sets no status, blocks nothing, enters no verify, certificate or plan box,
   and prunes or reorders nothing below R3's bar. A CLI verb and a lazily loaded skill; no MCP tool (R6).

## 7. Experiments and stage gates

Stage-gated (the owner-architect's verdict, 2026-10-16). Every call goes through `decide.Open` with provider `typesafe`
and `transport = "openrouter"`, the key read only through `api_key_file` and never printed, so a result carries to any
adapter. Budget: up to $10 in all for Jev on OpenRouter (owner, 2026-10-16); each reading records its spend from the
provider's reported usage, else estimated and labelled; OpenAI Decisions spend is not covered. Bars are fixed in
EXP-1 before any call (K19).

| Stage | Spend | Runs | Gate to the next stage |
|---|---|---|---|
| 0 | at most $2 | E1, E2, E3, beside readouts wave ro1, after pol1 lands, on independent files (a harness and readings) | EXP-M4: a template at least 0.9 accurate in a probability band holding at least 50% of its items, beating the lexical baseline |
| 1 | at most $8 | `fathom ask`'s judged column on the winning templates, in the owner-architect's waves for a week | EXP-M5 KEEP |
| 2 | $90 or more | the agent A/B bench (JP-14) | the owner's separate approval |

- **E1, judged predicates with computable truth.** Four templates over call-graph-selected Go spans, about 200 spans
  each, from fathom and two open-source Go repositories at pinned commits: does the caller check the callee's
  returned error; are errors wrapped with `%w`; does a test assert or only execute; is the lock taken before a
  guarded field is written. Truth from the syntax tree (EXP-2). A lexical baseline arm per template (the model has to
  beat grep, J0's rule). Recorded: accuracy by probability band with each band's share and Wilson bounds, the lift
  over the baseline, p50 and p95 latency, cost per 100 spans. This is J0's oracle half at stage-0 scale.
- **E2, questions with no computable truth.** About 100 items: does this caller depend on the changed behaviour;
  what kind of failure is this test failure (F35's question). Labelled by a Claude reviewer reading the span on
  subscription time (no API spend; provenance `frontier`); an owner spot-check is recorded when the owner chooses to
  (Q-DM5). Agreement by band, labelled agreement-with-frontier, never accuracy.
- **E3, the agentic query prototype.** Five real questions from recent waves. The agent writes a spec (section 6's
  shape), fathom selects rows (callers, blast radius, tests reaching) and computes first, Jev labels the residue;
  against the agent reading the files. Graded by a Claude reviewer against the files; recorded per question: rows
  selected, computed and residue shares, judged rows, correctness, context tokens per arm, dollars.

Calibration of Jev against OpenAI is deferred (owner, 2026-10-16): the experiments grade a model against truth, not one
provider against another.

## 8. Deferred and rejected

Deferred on the owner: DMP-4 (EXP-D1), DMP-6 (EXP-D2), DMP-7 (JP-14), DMP-12 (AQ-D1), DMP-16 (RJ-D1). Futures: F80,
F81, F35. Rejected: ADR-073's list (policy or provider chosen by a model, a cross-vendor cascade, a rename, an MCP
`ask` tool, judged security or compliance, answer-only output, judgments in proof surfaces, re-rank through `ask`,
model verdicts at H2, M9 or M13, model promotion of lean, questions over output text, the port per readout call, a
durable cache before RT-M1, a repository override of the user's kill, an operator-grammar verb, command execution).

## Decision models

This design is the decision-model program, so its typed questions are the ones section 5 lists, each behind its own
row. With provider `none` every site keeps its deterministic path, byte-identical (I42), and section 4's controls are
the opt-out for every one of them. The measurements that prove or kill the program: KS-1 to KS-7's tests (the opt-out
holds), EXP-M1 to EXP-M4 (whether a fast decision model reads code), RJ-M1 (whether the port adds anything over the
fitted readout table) and EXP-M5 (whether the judged column is used in real waves).

## 9. Readings

Under `docs/bench/readings/decide/`: `exp-preregistration.md` (EXP-1), `exp-e1.md` (EXP-M1), `exp-e2.md` (EXP-M2),
`exp-e3.md` (EXP-M3), `exp-stage-gate.md` (EXP-M4), `exp-stage1.md` (EXP-M5). Readouts' under
`docs/bench/readings/readouts/`: `fitted-table.md` (RJ-M1), `authored-rules.md` (RJ-M2).
