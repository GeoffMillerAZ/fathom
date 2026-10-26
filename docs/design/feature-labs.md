# Feature labs: a lab per feature, firing reliability, a decision-experiment ledger and the loop from report to regression

Status: proposed 2026-10-25 (the owner-architect's program, GitHub issue
[#13](https://github.com/GeoffMillerAZ/fathom/issues/13)). Plan: `docs/plans/feature-labs.md` (prefix LAB). Roadmap:
`docs/vision/38-proof-layer-roadmap.md` section 9.14. Companion: the head-to-head benchmark program (issue #10,
`docs/design/bench-prep.md`, prefix BX). The bench asks whether fathom as shipped makes an agent better than its
alternative. The labs ask, one feature at a time, whether that feature fires when it should, stays quiet when it
should not, reaches the model, and helps when it does. Neither program runs the other's experiments.

## 0. Where the program lives (the re-inventing check)

**Decision: a new design and a new plan, `docs/plans/feature-labs.md`, prefix `LAB`.** No `LAB-` id exists in the
tree (`LB` is language-breadth's, `FL` is taken, `BX` is bench-prep's). Every existing record that owns part of the
scope keeps it, and the labs consume its instrument and cite its rows:

| Record | Owns | What the labs take from it |
|---|---|---|
| `advice-delivery.md` (AD, ADR-080) | hook advice: readiness, dedupe, holdouts and the eligible, fired, sampled-out and held-out log (AD-20), the $0 compliance replay (AD-21), the exploration classifier (AD-22), per-seam eligible and fired counts (AD-M3), AD-M1 to AD-M4 | eligible, acted on and helped for the edit, prompt and subagent hooks. The labs never re-implement them |
| `rule-selection.md` (RS, ADR-077) | governing rule at first touch, precision, RS-M1 to RS-M5 | the guard lab's readings |
| `readouts.md` (RO, ADR-071) | readouts, the test-run hook, RO-M0's eligible classes, RO-M1 to RO-M10, the acted-on rule (next 3 calls) | the readout lab, the acted-on definition, and `fathom readout replay`'s transcript walk, which LAB-7 moves into a shared package |
| `intervention-outcomes.md` (IR-1 to IR-5) | firing beats with session and file, `fathom retro` | the joinable firing record |
| `decision-port.md`, `agentic-decisions.md`, `judgment-foundations.md` (DP, EXP, JF, ADR-073) | the port, conformance (KS-8), stage gates, calibration readings | every decision reading. The labs add only the shared ledger |
| `feature-map.md` (FM, ADR-055) | the authored feature list (`docs/features.md`, FM-1, not yet written) | feature ids for cross-reference. Until FM-1 lands the cards carry the scorecard's `F-01` to `F-86` |
| `efficiency-wave.md` EW-75/76, `responsible-tenant.md` RT-M14 | hook latency and abandonment, `hook-path-harness.py` | the fired and delivered stages for hooks, and abandons as a cited quality reading |
| `agents-md.md` AG-OP1 | the stanza reaching a session | clause 2 becomes a lab reading |
| `bench-prep.md` (BX, on its own branch) | subjects, task bank, agent-level hypotheses H1 to H8, plugin against no plugin (H4: A3 against A1), and the records listed in the next paragraph | one record each, shared, never a labs copy |

Shared with bench-prep, one record each: pins (BX-14 `RecordArmPins`), the prereg template (BX-24), the knob map
file (BX-28), the held-out read log (BX-29 `RecordHeldOutRead`), the hermetic fixture set (BX-30), arm declaration
and parity (BX-59 `PreregArms`, BX-60 `CheckArmParity`), the shakedown reading (BX-70), the scheduled lane (BX-74),
and one upstream kit batch in `reliability-stats` (BX-25 McNemar, BX-37 Holm, BX-38, BX-39, BX-64 `PairedSampleSize`,
with LAB-13 and LAB-15). The labs feed BX-28 the text knobs and read the firing funnel over the bench's A3 cells.

None of them owns these five things, which are this program:

1. A card per feature naming its firing criterion, its helped metric and its kill.
2. An eligibility and firing measurement for the choices the agent makes (which MCP verb to call, which skill to
   invoke) from one session corpus, beside AD's logs for the hook seams.
3. Per-feature ablation (the plugin against the plugin minus one feature) and live forks that vary stanza,
   tool-description and skill text.
4. One ledger for lab and decision-experiment results.
5. The link from an observation to the test that proves its fix.

ADR-075's rule points the same way: a new line of work gets its own append point.

## 1. Intent that applies, named before designing

1. **ADR-049 decision 2, the table freeze (RT-M1 reads FAIL).** Cards, cases, corpora, ledgers and readings are files.
   No store table, column, cache, memo file or background job.
2. **ADR-011 and I19.** Committed cases run on public fixtures (`internal/connector/testdata/fauxtown/`, public
   subjects). Local transcripts and real prefixes never leave `workspace/`. Adopters stay letters, as in the triage
   reading.
3. **ADR-029 decision 6: measurement stays Claude-only.** The session corpus reads Claude Code transcripts only. The
   decision says "no `TranscriptReader` port until a second harness exposes transcripts". Codex rollouts exist and
   fathom already reads them for cost (`internal/harness/descriptors.go`), so the trigger's condition is arguably met.
   This design does not act on it: the reader stays Claude-only and the question goes to the owner (Q7).
4. **ADR-034 decision 2 and its pending amendment (BX-40).** A LOST cell is an instrument defect. A `surface_unused`
   cell is a treatment outcome, kept and counted on the zero-call line. The labs score by assignment the same way.
5. **ADR-046 and I46.** Judging and lab spend is metered under its own `spend_label` and never enters an arm or a bet
   unit.
6. **ADR-073, P24, I42, I61, I62.** Every decision reading runs provider `none` as its baseline arm. No decision
   provider ever receives a transcript (I61). A judgment never becomes a lab verdict on a code fact (I62).
7. **ADR-053 and P4.** The labs add no MCP tool and never change surface membership. Membership changes stay owner
   decisions fed by lab readings.
8. **Law 12 and P12.** Only an arm carries causality. Firing, delivery and acted-on are observations. "Helped" needs
   an arm without the feature.
9. **I38 and P3.** An unreadable transcript or an unrecoverable tree is counted and named, never read as an absent
   session. An unmeasured funnel stage prints "not measured: <reason>", never 0.
10. **P13 and P15.** Fathom never writes an adopter's journal. Closing a report back is a report the operator carries.
11. **Spend (`docs/intent/autonomy.md`, Held, corrected in this change).** Decision-model experiments through the
    port (OpenAI Decisions, Jev) need no ask since 2026-10-22. Since 2026-10-26, agent sessions, experiments and
    benchmark sessions may spend agent budget in a fixed order: first parallel sessions that build the harness rows
    (LAB-19, LAB-20, LAB-21, LAB-33 and what they need), then the no-claim shakedown (LAB-37), then preregistered
    runs. P2's preregistered runs spend inside an envelope the owner sizes from LAB-37's prices. P0 spends $0.
12. **Futures F12, F48, F49, F40.** The case bank is the corpus F12 (an LLM-judged evaluation lane) waits for, and F12
    stays unpromoted: its `Promote when` names M-A1. The skill lab measures triggering and builds nothing of F48 (skill
    suggestion at the prompt). Observations are linked to themes by hand, so F49 (model classification of friction)
    is not built. No sampler or background judge (F40).

Fired triggers: none fired by this design. Item 3 is the one condition that may already be met, and it is left to
the owner.

Free-option test. Cards carry a `harness` field, so a Codex arm needs no schema change. Ledger rows carry OTel GenAI
attribute names, so an OTLP export through the kit's `otel-genai` sink needs no reshaping. Decision rows are keyed by
the kit's `decision.ReadingKey`, so a new provider needs no change. Feature ids accept FM-1's ids beside `F-NN`.
Nothing here builds a Codex arm, an exporter, a judge lane or a table.

## 2. Thesis

1. Every agent-facing feature can fail in five places, and fathom today measures mostly the middle. The **firing
   funnel**: *eligible* (the moment called for the feature), *fired* (the agent called it or the hook ran),
   *delivered* (the payload reached the model), *acted on* (the next calls used it), *helped* (the outcome beat the
   same moment without the feature). Call logs and hook logs cover fired and part of delivered. Readouts and AD-21
   cover acted on for two channels. Eligible is covered for the hook seams: AD-20 logs eligible, fired, sampled-out
   and held-out, AD-M3 reports eligible and fired per seam, and RO-M0 classified the readout's eligible classes.
   Nothing covers eligible for the choices the agent makes, which MCP verb to call and which skill to invoke, so
   nothing can say what the agent missed.
2. A feature is excellent when its recall at eligible moments is high, its false fires at near-miss moments are low,
   it is delivered every time it fires, and the plugin with it beats or matches the plugin without it. The labs
   report all five stages per feature and claim only what an arm shows.
3. Text is a knob only if measurement says so. Description edits alone can swing tool use by 10x, and context-file
   mentions can raise use 100x with flat success. So every text change is judged on held-out should-fire and
   should-not-fire cases by a paired test corrected for the number of surfaces tried, with the helped metric held.
4. Replay is cheap and live is honest. Replay of real sessions finds where features were eligible and missed, at $0.
   Only a live fork shows what a different text would have done. Replay results tune nothing until a measured
   agreement with live forks from the same real prefixes says replay predicts live.

## 3. What exists and what is reused

**Fathom's instruments, used as they are:**

| Asset | Used for |
|---|---|
| The MCP tool registry (`tools/list` from `mcp.NewServer(nil)`, as `registeredToolNames` reads it), the hook table (`hookSpecsFor`) and the skills `fathom plugin render` writes (`skillsdist.Names`) | the closure check's list of shipped surfaces, so a new surface with no card fails it |
| Feature scorecard (`docs/assessments/data/2026-09-27-feature-scorecard.json`, 86 features with `better` notes) | the cards' `features` field for cross-reference and the first knob hypotheses. It predates readouts, offload and receipts, so closure never reads it |
| `fathom plugin render` (IO-25, RH-5 digest) and `scripts/plugin-install-check.sh` | the plugin and its minus-one-feature copies under `claude plugin eval`, the digest as a pin |
| `cmd/scenariobench` arms as data (`Instructions`, `Setup`, `MCP`, `Env`), `RenderArm`, clean room, cells, `surface_unused`, `gold_source_tool` | the stanza lab and anything that needs the real hook stack |
| `hook-path-harness.py` (scratch daemon, private `FATHOM_HOME`) | fired and delivered for every daemon-calling hook |
| `fathom readout replay`'s line and block walk (dedup by tool-use id) | the shared transcript decoder: LAB-7 moves it to `internal/transcript`, and readout replay and the labs both read it there |
| AD-20's eligible log, AD-M3, AD-21, AD-22, RO-M0, RO-M4's acted-on rule, `fathom retro` (IR-2) | eligible, acted on and helped for hook channels, consumed when built |
| `fathom decide calibrate`, `decision.NewReading` (bands, Wilson, Brier, AUROC), `decisiontest.RunConformance`, the EXP harnesses in `internal/decide` | every decision reading |
| The ledger oracle (`internal/verify/ledger.go`) | lab result rows verify plan rows with no gate change, because `AppendLabResult` writes one row per (run_id, task_id, arm) and refuses a duplicate key (section 4.6) |
| `ask.Wilson` | the intervals of `BinaryRates` |
| `bundle.LoadOrCreateSalt` and `bundle.HashService` (`workspace/state/research.salt`) | hashed session ids, no second salt |
| `fathom flywheel`, `fathom instrument --adoption` | the loop check, and the labs section of the adoption report (no separate status verb) |
| `.fathom/observations/O<n>.md` with its `Status: fixed` line, the adopter triage (T01 to T25) | intake, the fix commit, the seed cases |
| `indexbench`, `reviewbench`, `contextbench`, `langbench` | the instruments of features with no firing question, and the answer-quality readings the cards cite |

**agentic-kit, used as it is:** `reliability-stats` (`PairedBootstrapCI`, `Wilcoxon`, `A12`, `PassAtK`, `PassPowK`),
`scenario-bank` (versioned cases with provenance: production-captured refused in committed data,
production-derived-redacted, synthetic-adversarial never decisive), `eval-journal` (an append-only record per judge
call, if a judge is ever admitted), `qa-harness` `RubricJudge` (deterministic rubric scoring), `otel-genai` (the
future export). None of the four quality packages is used by fathom today. Added upstream in one brief with
bench-prep's BX-25, BX-37, BX-38, BX-39 and BX-64 (the same package, one `task sync-kit`): an unbiased empirical
pass^k and the Rogan-Gladen prevalence correction. Binary judge rates live in `internal/labs` on `ask.Wilson`,
because the kit cannot import fathom.

**External methods and tools:**

- `claude plugin eval` (Claude Code 2.1.310 here): 3 runs per case, free graders (`tool_used`, `tool_order`, `regex`,
  `file_exists`), `context.history_file` to resume a recorded prefix, `--max-cost-usd`, and `--allow-tools` gating a
  real server's tools. Its default `--ablation with-without` arm removes the whole plugin, while the with-arm carries
  every hook, the session brief and the skills. That is bench H4 (A3 against A1) in small and attributes nothing to
  one feature, so the labs run `--ablation none` twice, the plugin and the plugin minus one feature, and compute Δ
  themselves from `--json`. It publishes its HTML report to claude.ai by default, so every lab run passes
  `--no-publish`. Its default `--mocks record` never starts fathom's real server, so mocks can read firing only and a
  helped run uses `--mocks off` (every real server starts, its tools gated by `--allow-tools`) against a scratch
  daemon. Fathom never gates on `--threshold` (which scores the with-arm only).
- skill-creator's `run_eval.py` and `run_loop.py`, run as they are: eval sets of `{query, should_trigger}` with
  near-miss negatives, 3 runs per query, a seeded hold-out, an improvement round chosen on the held-out part. The
  labs import their JSON into the ledger and rebuild neither.
- BFCL's irrelevance category (not calling is scored), MCP-Bench and MCPToolBench++ (selection scored on tasks that
  never name the tool), τ-bench's pass^k (reliability across repeats).
- Description ablations: one rewrite fed false positives and false negatives captures most of the gain, and a large
  train-versus-validation gap means two scopes overlap (a design fix, not a wording fix).
- OTel GenAI semantic conventions for MCP (`execute_tool`, `gen_ai.tool.name`, `gen_ai.evaluation.result`) as the
  ledger's attribute names.
- The "commit point" method (replay a real log to a decision point, then run live) as K2's shape.
- Judge validation: binary verdicts, TPR and TNR on a labeled split, prevalence corrected, intervals reported. The
  classifier's labelers are judges too, so the rule applies to them (K1).

**Not built:** a new agent runner (labbench wraps `claude plugin eval`, scenariobench and skill-creator's scripts), a
fourth transcript decoder, a cell store, a statistics library, a second salt, a status verb, a labs test runner or
time budget inside the gate, an LLM judge on any primary metric, a theme classifier, a sampler, a background job, a
store table, an MCP tool, a harness-generic transcript port, a dashboard.

## 4. Architecture

### 4.1 Components

| Id | Component | Where | Spends |
|---|---|---|---|
| C1 | Lab registry: one card per lab (`#Lab`), a no-lab list, the closure check against what ships (MCP registry, hook table, plugin skills) | `schema/labs`, `calibration/labs.d/`, `internal/labs` | no |
| C2 | Session corpus: Claude Code transcripts decoded by the shared `internal/transcript` walk, cut into decision points, hashed with the research salt, no text | `internal/transcript`, `internal/labs`, `fathom labs corpus` | no |
| C3 | Eligibility classifier for the agent's choices: a rule table per MCP verb and shipped skill that labels each point should-fire, should-not or unclassified. Hook seams read AD-20's eligible log instead | `internal/labs` | no |
| C4 | Firing reading: the funnel per lab from C3's labels, AD-20's log, call and hook logs, and the delivered evidence in the transcript | `fathom labs fire` | no |
| C5 | Fork harness: lab cases rendered as `claude plugin eval` suites (`--no-publish`, the plugin and the plugin minus one feature), or as scenariobench arms for stanza and real-hook labs | `cmd/labbench` | yes, capped |
| C6 | Text knobs: the lab-only tool-description override for a scratch daemon, the `stanza-text` scenario mode on a wired arm, skill description variants through skill-creator's scripts | `cmd/fathom`, `internal/scenariobench` | no |
| C7 | Results ledger: one JSONL file for lab and decision results, one row per (run, case or site, arm), read by the existing ledger oracle | `docs/bench/labs-ledger.jsonl`, `internal/labs` | no |
| C8 | Decision readings: label sets per site, provider `none` as the baseline arm, TPR, TNR, coverage, abstention | `cmd/labbench decide`, `internal/labs` | provider calls, no ask (2026-10-22) |
| C9 | Feedback loop: observation links, fix proofs derived from the fix commit, the seed map from adopter themes, the flywheel check, the labs section of `fathom instrument --adoption`, the adopter fixed-since report | `internal/labs`, `cmd/fathom`, `cmd/labbench` | no |

The product CLI (`fathom labs corpus|fire` and the labs section of `fathom instrument --adoption`) reads local files
and never spends or runs an agent. Every run that spends goes through the bench binary `cmd/labbench`, which refuses
to start without `-max-usd`, a `spend_label` and a committed prereg.

### 4.2 The lab card

`calibration/labs.d/<lab-id>.yaml`, validated by `#Lab`:

| Field | Meaning |
|---|---|
| `id`, `features` | `lab-<slug>`, and the scorecard ids (FM-1 ids once they exist) it covers, for cross-reference |
| `surface` | the shipped keys it measures: `mcp:<tool>`, `hook:<verb> --event <e>`, `skill:<name>`, `stanza:agents-md`, `decide:<site>`. Closure is computed over these |
| `firing` | the should-fire rule, the should-not (near-miss) rule, the grader (`tool_used`, `tool_order`, `skill_invoked`, `hook_context`), or `n/a` with a reason |
| `helped` | the outcome metric at the feature's moments, its arm (the plugin minus this one feature: the verb left out of `--allow-tools`, the hook or skill removed from a rendered copy, the stanza block removed), direction, the minimum effect of interest. Plugin against no plugin is the bench's (H4) |
| `quality` | the answer-quality reading another instrument owns (indexbench or langbench for definition and caller recall, reviewbench for review recall, EW-76 for abandons), cited, never re-measured |
| `fixtures`, `harness` | the public fixtures, and which of replay, fork, scenario, hook-path, decide or an existing bench measures it |
| `kill` | one or more criteria, each naming the reading that decides it |
| `knobs` | name, kind (text, cap, threshold, membership, default) and where it lives |
| `owners` | the plan rows elsewhere that own a stage (`advice-delivery.md#AD-21`) |

A case (`#Case`) is the kit's `#Scenario` plus `lab`, `should_fire`, `split` (train, select or confirm),
`regression`, optional `observation` and `theme`, and `derivation` (the decision-point shape and sampling stratum it
was rebuilt from). Provenance follows the kit. A fork case rebuilt on a public fixture from a real decision point is
`production-derived-redacted` with its derivation recorded. A synthetic case (`synthetic-adversarial`) never decides
a verdict alone. A real prefix (`production-captured`) is never committed and runs locally only (K2).

### 4.3 The seed labs

| Lab | Features | Fires when | Must stay quiet when | Helped metric (plugin minus this feature) | Quality, owned elsewhere | Stages owned elsewhere |
|---|---|---|---|---|---|---|
| lab-find-symbol | F-01 | the next lookup is the definition of an identifier defined in the repository | a string literal, config value or log line | calls and tokens to the gold definition | definition recall: indexbench L0 | |
| lab-search | F-09 | literal text, a log line, a config value | a symbol definition | calls to the first correct hit | abandons: EW-76 (T02, T03) | |
| lab-symbol-context | F-03 | who calls a named symbol | a definition lookup | calls and tokens to an answer naming every caller in the key | caller recall: indexbench, langbench; abandons: EW-76 (T01, T06, T07) | RT-M14 |
| lab-read-source | F-08 | re-reading a cited line range | open discovery | bytes read per correct answer (Read stays available) | | |
| lab-blast-radius | F-12 | before changing an exported symbol with fan-in of 3 or more | a private helper with no callers | broken callers at the final tree | | agent-level H2: bench-prep |
| lab-review-diff | F-34 | before a commit or on a review request | a docs-only diff | seeded defects the agent's review names | review recall: reviewbench | |
| lab-verify-plan | F-36, F-37, F-39 | a plan box claimed in the session, a done claim at Stop | no plan touched | false done claims reaching Stop | verify correctness (T10, T11, T15): fathom's own tests | agent-level H1: bench-prep |
| lab-obligations | F-26, F-27, F-64, F-65 | an edit to a governed file | an edit outside every scope | compliance, relevance (AD-20's holdouts are this arm) | | AD-20, AD-21, AD-M1 to AD-M4 |
| lab-guards | F-29 | a pre-edit on a guarded path | any other path | governing rule at first touch | | RS-M1 to RS-M4 |
| lab-prompt-hook | F-28 | a prompt naming an indexed identifier | a prompt naming none | calls to the gold definition (the definitions line used in the next 3 calls is acted on, not helped) | | ADR-076 |
| lab-session-brief | F-48 | every session start | (deterministic) | calls to the first correct tool for the task's shape | | |
| lab-subagent-brief | F-48 | a subagent spawn | (deterministic) | the subagent's calls to its answer (map facts delivered, 0 of 1,841 today, is the delivered stage) | | AD-M2 |
| lab-readouts | (postdates the scorecard) | Bash output a rule matches | output no rule matches | RO's readings (acted on in the next 3 calls is the acted stage) | | RO-M1, RO-M4 to RO-M10 |
| lab-stanza | F-58, F-86 | always loaded | (always loaded) | task outcome against the wired arm with the stanza block removed (first-3-calls tool choice per task shape is its firing) | | AG-OP1 |
| lab-skills | F-60 | a request inside a skill's description | near-miss requests that share its keywords | the skill's task outcome (the plugin minus that skill) | | |
| lab-decision-port | F-69, F-70, F-71 | (sites are configuration) | (sites are configuration) | per-site TPR and TNR against provider `none`, coverage | | DP, EXP, JF |

Every other shipped surface (an extended-surface tool, a hook with no firing question, a skill no seed card covers)
gets a no-lab entry naming its instrument (`indexbench` L0, `reviewbench`, `langbench`, a bench-prep family) or the
reason a firing question does not apply (an extended-surface tool, ADR-053). The closure check computes the shipped
surfaces from the MCP tool registry, the hook table and the rendered plugin's skills, so a new surface with no card
and no entry fails it.

### 4.4 Measuring the funnel

| Stage | Replay ($0) | Live fork (priced) |
|---|---|---|
| eligible | MCP verbs and skills: C3 over decision points, deterministic rules on the call's input shape, the preceding calls and the repository tree. Transcripts carry `cwd` and `gitBranch`, not a commit, so the tree is the commit on that branch nearest the session's timestamp, and the definition check is a go/parser declaration scan of it (pure Go, ADR-066), never fathom's answer. A tree that cannot be recovered (a deleted clone) leaves the point unclassified and counted (I38). Hook seams: AD-20's eligible log, else "not measured" | the case's `should_fire` |
| fired | MCP and Skill `tool_use` events, hook `seen` beats | `tool_used`, `tool_order`, with-only indicators |
| delivered | a `tool_result` with no error or abandon, a `hook_additional_context` attachment in the transcript | the same, from the run transcript |
| acted on | AD-21, RO-M4's next-3-calls rule, where built. Else "not measured" | `tool_order` and `regex` graders on the next calls |
| helped | not measurable offline (law 12) | Δ against the plugin minus this feature (or the wired arm with the stanza block removed), scored by assignment |

Reported per lab: recall at should-fire points, the false-fire rate at should-not points, the substitute tool used at
a missed point (Grep, `rg` through Bash, Read), the delivered share, the acted-on share, pass^k over the 3 runs of a
fork case beside the mean, and Δ helped with a paired bootstrap interval. Session ids are hashed with the research
salt (`bundle.LoadOrCreateSalt`, `HashService`). Points carry tool names and shape classes, never text. The bench's
A3 cells are live wired sessions with known tasks, so the labs also read the funnel over their transcripts, with the
task's gold tool as eligible, at no extra spend (LAB-39).

### 4.5 Forks and text knobs

A fork case is a short prefix and a request. Cases are sampled from every stratum of decision points, not only the
missed ones: should-fire points the agent missed, should-fire points it hit, should-not points, and points where the
substitute (Grep, `rg`, Read) succeeded. Sampling only the shapes fathom's routing misses most would make any Δ an
upper bound. A P2 fork prefix is rebuilt on a public fixture from its point's shape and recorded as
`production-derived-redacted` with its derivation. K2 instead uses the commit-point method: the real prefix through
`context.history_file`, run locally against the tree at the nearest commit, never committed.

Each case runs 3 times in each arm, capped at a few turns. Every run passes `--no-publish`, and `labbench` refuses a
run without it. A firing-only run may use recorded mocks. A helped run uses `--mocks off` against a scratch daemon,
since mocks never start fathom's real server. Variants:

- **Tool descriptions.** `serve-http --lab-tool-descriptions <file>` replaces description text in `tools/list` for a
  scratch daemon. Names and schemas are unchanged. The managed daemon and the user's own `FATHOM_HOME` refuse the flag
  by name. There is no config key, so no adopter can turn it on by accident.
- **Stanza.** `claude plugin eval` loads no AGENTS.md, user hook or `.mcp.json`, so the stanza lab runs through
  scenariobench. Its `text` mode already appends to the subject's root CLAUDE file or to AGENTS.md and never creates
  a CLAUDE.md (ADR-059, `RenderArm`). The gap is a wired arm whose stanza block, the one `fathom init` writes, is
  replaced: a `stanza-text` mode swaps the text between the `fathom:begin` and `fathom:end` markers and keeps the
  hooks. The no-stanza arm is that block removed. The arms are declared through bench-prep's `PreregArms` (BX-59) and
  checked by `CheckArmParity` (BX-60), so the block is the only difference. Bench's A3m (no hooks, no stanza) is not
  the no-stanza arm, because it also drops the hooks.
- **Skills.** Description variants run through skill-creator's `run_eval.py` and `run_loop.py` as they are, on a
  scratch plugin copy, and their JSON is imported into the ledger.
- **Hook text.** Variants run through the hook-path harness for fired and delivered, and through AD-20 holdouts for
  helped once AD-20 lands.

Tuning discipline:

1. Three splits per surface by a seeded hash of the case id: train (the rewrite reads its misses and false fires),
   select (the variant is chosen here, `run_loop.py`'s held-out part for skills), and confirm (read once at a named
   release candidate, LAB-D4).
2. Every select and confirm read goes through bench-prep's held-out read log (BX-29), so a second read demotes that
   split.
3. One rewrite round.
4. Each case's outcome under the old text and the new is a pair. The pairs are compared by the exact McNemar test
   (BX-25), with Holm's correction across every surface tried in the round (BX-37: 12 skills and 8 verbs).
5. Set sizes come from `PairedSampleSize` (BX-64) with the discordant rate the shakedown measures. A set that cannot
   reach its n reads unpowered and reports counts.
6. A rewrite is kept only when its Holm-adjusted p is under 0.05 on select, the helped metric is non-inferior, and
   the confirm read holds.
7. The A/A run checks the instrument: if McNemar separates an unchanged variant from itself, the harness is too
   noisy to read.

A large train-versus-select gap is read as overlapping scopes (two skills or two verbs claim the same moment) and goes
to design, not wording. Every kept knob change cites its ledger `run_id` in its commit's `Verified:` line. It is
written under `## Lab knobs` in bench-prep's knob map file (`docs/bench/prereg/knobs.md`, BX-28).

### 4.6 The ledger

`docs/bench/labs-ledger.jsonl`, one row per (run_id, task_id, arm), validated by `#Result`, written only by
`AppendLabResult`. A rerun takes a new `run_id`.

`AppendLabResult` refuses a second row under an existing key. The ledger oracle matches a task-keyed row on that key
alone and lets the last match win. Its `criterion:` disambiguator applies only to run-keyed rows. So one row per
stage under a single key would let a `delivered` row certify a `helped` claim.

The keys the oracle reads come first: `run_id`, `task_id` (a case, a surface, or a decision site), `arm` (a variant, a
minus-one-feature copy, `replay` or a provider), `trials`, `evaluation{criterion_id, verdict, reason}`,
`surface_unused`, `instrument_defect`. `evaluation.criterion_id` names the one criterion the run was preregistered to
decide, and a plan row claims only that criterion.

Then come:

- `lab` and `feature`.
- `stages{eligible, fired, delivered, acted, helped}`. Each stage holds a value with its interval and n, or "not
  measured: <reason>".
- `quality`, a citation of the reading another bench owns.
- `pass_k`.
- `pins`, stamped by bench-prep's `RecordArmPins` (BX-14), plus the plugin digest and the stanza and description
  sha.
- `prereg_sha`, `spend_usd`, `spend_label`.
- `attributes`, with OTel GenAI names.

Decision rows add the `ReadingKey` (site, template@version, provider, resolved model), TPR, TNR, coverage,
abstention, Brier, AUROC and the Rogan-Gladen estimate. Their baseline is the provider `none` row of the same run.

Prose readings stay under `docs/bench/readings/labs/`, verdict line first.

### 4.7 The loop from report to regression

1. **Intake.** An observation (`fathom observe`) or a triaged adopter report.
2. **Link.** The entry gains `- **Lab:** <lab>/<case>`, by hand or by `fathom observe --lab` at filing. The bullet is
   additive and every reader tolerates its absence (the observation files are on the compatibility surface). A link
   is a pointer, never proof.
3. **Case.** A case on a public fixture that reproduces the report. Deadline themes (T01, T02) reproduce on an
   injected clock or a slow fixture store, never a cold service.
4. **Fix.** The entry's `Status: fixed` line names the fix commit. Where it carries no sha, the fix commit is the one
   whose message names the O-id. `ProveObservationFix` lists the tests that commit added, then runs them once in a
   scratch worktree: at the parent they must fail, and at the fix they must pass. The pair is written to
   `docs/bench/readings/labs/fix-proofs.jsonl`. A fixed entry with no recoverable sha or no added test is named
   unproven. No one writes a proof bullet by hand, so K5 never counts the controller's own claims.
5. **Regression.** The added test is an ordinary Go test under `go test -race ./...`, so the labs add no runner and no
   budget to the gate. A recorded-mock case joins bench-prep's hermetic set (BX-30). A live case reruns at a release
   candidate under an approved envelope.
6. **Report.** The labs section of `fathom instrument --adoption` lists:
   - themes with open entries and no case;
   - fixed entries with no fix proof;
   - red cases.

   The flywheel gains one check. `fathom instrument --adoption --adopter <journal>` reads an adopter's journal and
   lists the entries whose case is green at a named release, for the operator to carry back.

The 25 themes of the 2026-10-20 triage seed the bank. Each becomes a case or a named disposition: fixed and proved by
its fix commit's test, not fathom's, or no case possible.

## 5. Phases

| Phase | Spend | Content | Exit |
|---|---|---|---|
| P0, starts now | $0 | schema, registry, closure over shipped surfaces, the 16 seed cards and the no-lab list, the shared transcript decoder, decision points, classifier v0 for verbs and skills, firing reading, ledger, kit statistics, observation links, the triage seed map and the first fixture cases, fork-suite rendering, the lab prereg addendum, the ADR | LAB-M0: the plan verifies, closure reads every shipped surface mapped, the offline firing baseline (LAB-M0a) is committed, the ledger holds a row the ledger oracle reads |
| P1 | agent budget under the 2026-10-26 lift, and decision-port calls (no ask) | first parallel build sessions for the harness rows, then the no-claim shakedown (LAB-37); classifier labels by two labelers and K1; fix proofs; the flywheel check; the adoption report's labs section; the description override; the `stanza-text` mode; trigger and fork sets; the three-way split; decision readings, the backfill and the per-site comparisons (LAB-38) | LAB-M1: K1 read per surface, the seed cases proved, the decision backfill and comparisons committed, the shakedown read, every suite renders and validates, the P2 envelope sized from the shakedown's prices |
| P2 | preregistered runs inside the envelope sized from the shakedown | the A/A instrument check, skill triggers, verb descriptions, stanza variants, replay-to-live agreement (K2), the funnel over the bench's A3 cells | LAB-M2: K2 and K3 read, at least one knob decision recorded either way |
| P3 | per prereg | confirm-split reruns at a release candidate, a labs stage in the bench's scheduled lane, and the deferred options (Codex arm, OTLP export, a judged helped metric, a residue decision site) | LAB-M3: the program's keep-or-kill reading (K6) |

Kill criteria, each read by its own plan row:

- **K1, the classifier.** Per surface, on at least 40 labeled points of each class (should-fire and should-not), TPR
  and TNR each have a Wilson lower bound of at least 0.80. Forty is the floor because 12 of 12 reads 0.758, a perfect
  classifier needs 16 and one error needs 25 (0.805), while at 40 one error still reads 0.871. Labels come from two
  independent labelers, blind to the classifier and to each other. Cohen's κ between them is at least 0.8 per
  surface, and the owner adjudicates every disagreement. A surface under 0.8 is labelled by the owner or reports
  counts only (Q4). A surface that fails K1 reports counts only, never rates.
- **K2, replay predicts live.** On at least 60 real prefixes forked locally by the commit-point method, agreement
  between the replay prediction and the live first call reaches Cohen's κ of at least 0.6. Below it, replay may rank
  where to look and never selects or keeps a knob.
- **K3, text is a knob.** One rewrite round moves select-split recall or the false-fire rate with a Holm-adjusted
  exact McNemar p under 0.05, without worsening the other. Else text tuning stops for that surface with "text is not
  the knob" recorded.
- **K4, firing without helping.** A knob that raises firing and leaves the helped metric harmed or inconclusive toward
  harm is reverted.
- **K5, the loop earns its cost.** Sixty days after LAB-M1, at least half the observations fixed in the window have a
  fix proof: a test their fix commit added, red at the parent and green at the fix. Else the loop shrinks to what was
  used.
- **K6, the program.** After the confirm reads (LAB-D4), at least one knob change holds on the confirm split. Else the
  program stops at the P1 instruments (cards, ledger, fix proofs, firing reading) and P3 is refused. If K6 has not
  read by 2027-01-31 (the roadmap's horizon), the program stops there the same way (Q8).

## 6. Lanes

- *Codex on a helper machine* (isolated checkout, a written brief naming any old-behavior test it may change): every
  code row, the kit rows (agentic-kit checkout beside fathom's, landed through `task sync-kit`, one brief with
  bench-prep's kit rows), fixture cases on fauxtown, trigger and fork sets on public fixtures, suite rendering and
  validation.
- *Cloud:* the regression tests on Linux, a second-opinion review of trigger sets for near-miss quality and of preregs.
- *API-funded agent sessions.* Under the 2026-10-26 lift, the order is:
  1. parallel build sessions for the harness rows;
  2. the shakedown;
  3. the two labelers.

  The forks, stanza arms, the A/A check and the agreement reading are preregistered runs inside the envelope the
  owner sizes from the shakedown's prices. Decision-port comparisons need no ask (2026-10-22).
- *Controller:* the cards' firing, helped and kill choices, the eligibility table's sign-off, LAB-M0a (it reads this
  machine's transcripts), label adjudication, preregs, interpreting readings, every spend request, the ADR.

## 7. Risks and honesty traps

1. **Firing counted as helping.** Mention-driven use can rise a hundredfold with flat success and higher cost. Every
   card has both a firing criterion and a helped metric, and only the helped arm supports a claim (K4).
2. **Inflated contribution.** A whole-plugin baseline attributes every hook and skill to one verb. Gating on the
   with-arm score, or counting "it fired" graders inside Δ, flatters a feature too. The helped arm removes one
   feature, and fathom computes Δ from per-run JSON over outcome graders only.
3. **Favoring fathom by construction.** Cases drawn only from the shapes fathom's routing misses most make Δ an upper
   bound. Cases sample every stratum, including should-not points and points where Grep succeeded.
4. **Text overfit.** Without should-not cases, better wording means more false fires. Near-miss negatives, three
   splits, one rewrite round and a corrected paired test guard it (K3).
5. **The harness is not the product.** Plugin eval loads no AGENTS.md, user hooks or `.mcp.json`. Mocks show firing,
   never answer quality. Hooks fail silently, so "the hook ran" is not "the context landed". The stanza goes through
   the real delivery path, helped runs through real scratch daemons (`--mocks off`), and delivered reads the
   transcript.
6. **Publishing by default.** `claude plugin eval` publishes its report to claude.ai unless told not to. `labbench`
   refuses a run without `--no-publish`.
7. **Unvalidated instruments.** The classifier and its labelers (K1) and replay itself (K2) are validated before they
   steer. No published study shows offline replay predicting live agent results, so fathom measures its own.
8. **Circularity.** Eligibility never comes from fathom's answer. Definition checks are go/parser declaration scans
   of the tree at the nearest commit.
9. **Self-grading.** A fix is proved by the test its fix commit added, run red at the parent and green at the fix by
   the runner, never by a bullet someone wrote.
10. **Baseline drift.** RO-M7's baseline moved from 46.8% to 23.3% on new data. Every reading names its population and
    window, and a comparison across populations is refused until re-established.
11. **Privacy.** Transcripts and real prefixes stay in `workspace/`. Committed cases use public fixtures. The push
    gate's leak check runs over every committed lab file.
12. **Model and CLI drift.** Pins on every row (`RecordArmPins`). A pin change marks prior firing rows stale for
    comparison.
13. **Spend creep.** The product verb cannot spend. `labbench` refuses without a cap, a label and a prereg, and a
    binding cap makes the run a no-read. Agent budget goes to harness rows before any run that claims.
14. **Overlap with the bench.** The labs never run plugin against no plugin (H4) or a head-to-head on task success.
    The bench never reads a single feature's funnel. H1, H2 and H4 stay in bench-prep, and the shared records of
    section 0 have one owner each.

## 8. Two horizons

Built now: cards, cases, ledger and readings as files, Claude-only replay, deterministic graders, a product verb that
cannot spend, a bench binary that can only spend under a cap. Kept open at no cost: Codex arms (the `harness` field,
after an ADR-029 amendment), OTLP export (attribute names), FM-1 feature ids, new decision providers (`ReadingKey`), a
judged helped metric through the kit's `eval-journal` (F12, if promoted), a future table (rows already have a stable
key). Not pre-built: any of those.

## 9. Owner questions

These are this program's questions, not the ids of `docs/intent/questions.md`.

- **Q1 Agent spend (answered 2026-10-26).** Agent sessions, experiments and benchmark sessions may spend agent
  budget: harness first, then the no-claim shakedown, then preregistered runs. P2's envelope is sized from LAB-37's
  prices. The ruling was given in session, so the controller confirms it from the owner's words before this change
  merges.
- **Q2 Decision-model spend (answered 2026-10-22).** Experiments through the port need no ask, so LAB-38 is a live
  row. Whether the $10 OpenRouter cap of 2026-10-16 still binds Jev is not stated by that ruling.
- **Q3 Corpus scope.** May `fathom labs corpus` read every local project's transcripts, as RO-M0 did, or only this
  repository's? Only aggregates are ever committed.
- **Q4 Labels.** The default is two independent blind labelers with κ per surface and owner adjudication of
  disagreements. Would the owner rather label everything? At 40 per class per surface this is about 1,600 points.
- **Q5 Gate budget (withdrawn).** A deterministic case is a Go test under `go test -race ./...`, so the labs add no
  budget to the gate.
- **Q6 Close-back.** Is a per-adopter "fixed since" report the operator carries the right shape, given fathom never
  writes another repository's journal?
- **Q7 Codex.** ADR-029 decision 6's trigger may have fired (Codex rollouts exist). Amend it so labs can run a Codex
  arm in P3, or keep measurement Claude-only?
- **Q8 The program's end date.** K6 stops the program if it has not read by 2027-01-31. Is that the right date?

## Decision models

The labs measure the decision port. They are not a decision site, and every grader, classifier label, split, interval
and verdict is computed deterministically with provider `none`. Three typed questions could enter later, and none is
built here:

- **The residue predicate.** For a decision point C3 leaves unclassified, a `predicate` "is `<verb>` eligible here?"
  over the point's features (tool names and shape classes, never transcript text, I61). Fallback with `none`: the point
  stays unclassified and is counted as such. Opt-out: the site is absent from `[decide] sites` by default. Measurement:
  agreement with the K1 labels on the residue, kept only at a Wilson lower bound of at least 0.90, and built only if
  LAB-M1a shows the residue above 20% of points (deferred row LAB-D6).
- **A theme choice for observations.** Refused here: F49 says not to build it, and hand linking costs one line per fix.
- **A judged helped metric** for outcomes no grader computes. Deferred with F12. If proposed, it is a binary verdict
  recorded through the kit's `eval-journal`, validated by TPR and TNR on at least 100 labeled cases, never primary.

## 10. Readings and files

| Path | Committed | Holds |
|---|---|---|
| `calibration/labs.d/<lab>.yaml`, `no-lab.yaml`, `README.md` | yes | cards, the no-lab list, the card index |
| `calibration/labs.d/<lab>/cases/*.json`, `skills/<skill>.evalset.json` | yes | public-fixture cases, skill-creator eval sets |
| `calibration/labs.d/decide/<site>.labels.jsonl` | yes, public spans only | decision label sets |
| `calibration/labs.d/feedback-seed.md` | yes | the 25 triage themes to cases or dispositions |
| `docs/bench/labs-ledger.jsonl`, `docs/bench/readings/labs/*.md`, `docs/bench/prereg/labs.md` | yes | results, readings, the prereg addendum |
| `docs/bench/readings/labs/fix-proofs.jsonl` | yes | each fixed observation's fix sha, its added tests, red at the parent and green at the fix |
| `docs/bench/heldout-reads.md`, `docs/bench/prereg/knobs.md`, `docs/bench/readings/bench-prep/shakedown.md` | yes, bench-prep's | the held-out read log, the knob map and the shakedown reading, shared |
| `workspace/labs/sessions/`, `labels/`, `prefixes/`, `runs/<run_id>/` | no | decision points, private labels, real prefixes for K2, raw plugin eval and scenario output |
| `workspace/state/research.salt` | no | the session-hash salt, the research bundle's |
