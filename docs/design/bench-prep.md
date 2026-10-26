# Bench prep: third-party subjects, a vetted task bank and pre-registered reads before any spend

Status: proposed 2026-10-25 (the owner-architect's design; the roadmap draft and the critic's 16 corrections of
2026-10-25, every correction accepted by the owner-architect and binding here). Plan: `docs/plans/bench-prep.md`
(prefix BX). Roadmap: `docs/vision/38-proof-layer-roadmap.md` section 9.13. Applies ADR-031 to ADR-034 and changes
none of them except the amendment BX-40 records (section 1, item 2). This is the "credible tier" vision 27 section 9
leaves to an operator decision: everything here prepares it for $0, and the first dollar is an owner-approved
shakedown.

## 0. Where the program lives (the re-inventing check)

**Decision: a new plan, `docs/plans/bench-prep.md`, prefix `BX`.** No `BX-` id exists in the tree (`BP` is
bench-pyramid's, `PB` bench-public's).

Four plans own benchmark scope today, and each closes one decided slice:

| Plan | Owns | State |
|---|---|---|
| `bench-confidence.md` (BC) | replication floor, paired summary, the committed ledger (ADR-028) | 15 of 16 rows proven; BC-16 waits on a second same-corpus row |
| `bench-public.md` (PB) | the derived public Go corpus, the `none` arm and leak rule, kit statistics (ADR-031) | PB-19 (op) and PB-4's wiring open |
| `bench-pyramid.md` (BP) | L0 to L5 on the home corpus, cell keys, H2-3 (ADR-032) | BP-45 and BP-46 (op) open |
| `strengths-bench.md` (SB) | the matrix, the scenario driver with arms as data, competitors (ADR-033) | wave 6 pilots wait on the receipts reruns |

None of them owns what this program adds: subjects vetted across languages, a multi-family task bank with strata
that fathom does not compute, contamination control, dry-run harnesses that prove the graders discriminate, and
pre-registration with three outcomes. Adding about 100 rows under one of their prefixes would reopen a plan that
is nearly archivable and mix this program's decisions into an ADR that did not make them. ADR-075's rule points
the same way: a new line of work gets its own append point. The program reuses their instruments (section 4)
and names their open rows where it touches them.

## 1. Intent that applies, and the conflicts named before designing

1. **ADR-031 to ADR-034 hold.** Derived truth, never authored (031); free layers before spend and cells keyed by
   their invariants (032); every claim against its alternative, by an oracle fathom does not own, prediction first
   (033); instrument truth on every cell (034).
2. **Conflict, ADR-034 decision 2 against intention to treat.** ADR-034 refuses a reading on a `surface_unused` cell
   (the server connected, no fathom tool was called). The binding correction 1 scores every cell by its actual
   outcome under its assigned arm, so a primary analysis keeps those cells. Resolution: a LOST cell (the server did
   not connect) stays an instrument defect, voided and retried. A `surface_unused` cell is a treatment outcome: it is
   kept in the primary, counted on the zero-call line, and still refused as the anchor of an `oracle: ledger` row
   that claims a tool effect from one cell. BX-40 records this as an ADR-034 amendment before the first prereg is
   signed.
3. **Conflict, ADR-032's L0 floor against the draft's C4.** ADR-032 holds agent runs closed on a pinned repo whose L0
   precision or recall is under 0.95 for an adjudicable tool. The draft set C4 at 0.90. C4 here keeps 0.95 for
   `find_callers` and the resolved `blast_radius` walk; `references` is reported as a lower bound and never gated
   (ADR-032 amendment 2026-09-12). The language-breadth reading puts Go call-edge recall at 0.92 on two public Go
   repositories (ADR-048 amendment 2026-10-02). If BX-5 confirms a subject below 0.95, ADR-032's revisit trigger
   fires for that subject, it is kept for the free layers and out of agent runs, and the owner decides whether
   to amend the floor (Q8).
4. **ADR-029 decision 6: measurement stays Claude-only.** A Codex arm needs that clause amended first. BX-18 is
   deferred on Q4.
5. **ADR-049 decision 2, the table freeze (RT-M1 read FAIL on 2026-10-19).** Bench data are files: manifests,
   corpora, cells, ledgers, readings. No store table, column, cache or product background job. The scheduled free
   layer is a helper-Mac lane, not a daemon job.
6. **ADR-011 and I19.** Public open-source subjects only. No private repository, host, machine or person is named in
   any manifest, reading or prereg.
7. **I38 and ADR-027.** No instrument reports a conclusion it could not earn: a capped run is a no-read, a refusal
   under C4 is published, lost cells are counted per arm.
8. **I56.** Fathom never downloads on its own authority. Caches are pre-warmed by operator-run bench binaries from
   the sources a committed manifest names. The daemon fetches nothing.
9. **T5 and law 12.** A criterion changes only before data exists under it. The prereg's n is set from the
   shakedown, whose tasks are then removed from the dev analysis set, so sizing data and confirmatory data never
   overlap.
10. **The budget hold (`docs/intent/autonomy.md`, 2026-09-29).** P0 and P1 spend $0. Every P2 and P3 run is a
    deferred row until the owner approves its envelope.
11. **Future F2 and intent question Q4 (`docs/intent/questions.md`).** Publishing outside `docs/bench/` stays the
    owner's call (this program's Q5).

Free-option test. The harness port stays one interface and arms stay data, so a Codex arm or a container runner can
be added later at no cost now. The manifest is language-agnostic, so Java and Rust need no schema change. Nothing
here builds a Codex harness, a container spike or a judged grader.

## 2. Thesis

1. Under pre-registered rules, with every arm seeing the same repository, tools and prompt except what the arm
   declares, on third-party public repositories: does adding fathom as shipped (A3) to a capable agent's normal
   tools (A1) raise task success, lower false "done" claims, or lower cost on the strata where it is predicted to
   help, and leave the predicted-null strata unharmed?
2. The program is built to refute this as readily as to confirm it. The agent-arm priors are null to negative (the
   home bench, LCA run 1, reuse-pilot-4, guards-pilot-8). A clean null is a valid result and is published with the
   same prominence as a win.
3. No dollar goes to agent runs on a subject until fathom's free layers read on it: L0 edges, review recall, test
   selection and the verify fixtures. An agent run over a wrong index measures nothing.

## 3. Hypotheses

| Id | Claim | Status |
|---|---|---|
| H4 | Localization is better with fathom when the issue text shares no identifier with the gold file | primary, needs spend |
| H1 | The verify gate cuts false "done" claims, and beats a run-the-tests stop hook at it | primary, needs spend |
| H2 | High fan-in behavioral changes break fewer callers with fathom | conditional: runs only if H4 shows a tool-driven lift (A2 or A3m carries it) |
| H3 | Review names the killing caller or test outside the diff | free reading, against rg and gopls |
| H5 | Cross-repository call-site recall | free reading, against grep, `go.work` and gopls |
| H8 | Test selection keeps safety recall 1.0 and runs less of the suite | free reading, against the whole suite, Go's test cache and pytest-testmon |
| H6 | Rework avoided | refused: two negative priors, the feature needs rework before measurement |
| H7 | Claims in the final answer are true | deferred: its grader needs claim extraction, an LLM-judge risk on a primary |

Correction 3 changes H4's strata: the primary stratum is defined by lexical overlap only. Hop distance from the
named symbol to the cause is a covariate with a "no graph path" bucket, because a "2+ hops" stratum keeps only tasks
whose cause is graph-connected to a named symbol, which is fathom's own mechanism. Two definitions are pre-declared
and BX-42 measures their yield on LCA (40 tasks) and public-go (35) at $0; the controller picks one before any agent
spend:

- **D-a.** Non-lexical when no issue identifier (3 or more characters, a fixed stopword list, camel and snake forms
  kept whole) occurs in any gold file at the base commit.
- **D-b.** Non-lexical when the gold file ranks outside the top 10 of a fixed BM25 over the issue's identifiers.

The BM25 rank (`lexical_rank`) is recorded on every F1 task whichever definition wins.

## 4. Reuse versus build

**Fathom's instruments, reused as they are:**

| Asset | Used for | Added |
|---|---|---|
| `cmd/scenariobench` (arms as data, cell store and keys, clean room, resume, `-rescore-lost`, `plan`) | every run | `vet`, `bank` and `dryrun` verbs; `-harness null, oracle, wrong-patch, scripted-*`; `-max-usd`; `-prereg`; pins; interleaving |
| `RunOracle`, `CalibrateSubject`, `FalseDoneRate`, `ScoreLocalization`, `ClassifyChange` | F2 and F1 graders, the base-commit check, the false-done reading | hit@3, function hit@5, stratum fields |
| `DeriveScenarioTasks`, `BuildLCACorpus`, `publicloc` | Multi-SWE-bench and LCA tasks | filters to vetted subjects |
| `cmd/loctruth`, `cmd/indexbench` (`IndexOracleReport`, `compare`, `ProbeReferencesViaMCP`), the language-breadth oracles | C4 and BX-5, the H3 and H5 baselines | run per subject |
| `contextbench` `LeakRule` | the A0 leak rule | ported to scenariobench (BX-58) |
| `cmd/reviewbench`, the tests-select replay, the cv-m4 apidiff pipeline | H3, H8, H5 free readings | pointed at the kept subjects |
| agentic-kit `reliability-stats` (`PairedBootstrapCI`, `Wilcoxon`, `A12`, `PassAtK`) | intervals and effect sizes | McNemar, Holm, wild-cluster bootstrap, TOST, sample size, upstream first |
| `calibration/corpus.d` repo-entry shape | the subject manifest's repo entry | a sibling `calibration/bench.d/`, because corpus.d's file name is the language-queue id |

**External assets:** Multi-SWE-bench (CC0, already in `derive.go`), the SWE-PolyBench 500 subset, ContextBench gold
contexts (retrieval truth better than "files the fix touched"), SWE-rebench and SWE-bench-Live slices (post-cutoff),
LCA bug localization (in use), go-mutesting, mutmut and Stryker (seeded faults), and the CodeProbe and Sourcegraph
method (information parity, a strict arm, paired deltas, cost caps, the zero-call audit) adopted as doctrine, not as
code. Each dataset's licence is checked at its pin (BX-41). SWE-Explore is CC-BY-NC-ND (`docs/bench/public-loc.md`),
so nothing derived from it is committed.

**Not built:** a new runner, cell store or statistics library; a second task schema (the scenario JSON grows
additively); a custom mutator or contamination detector; an LLM-judge grader on any primary metric; a bench
dashboard; any store table; synthetic fixture fleets as evidence (self-authored, so harness tests only); a Codex
harness or a container runner before Q4.

**Dropped from the draft:** BX-2 (`calibration/clone.sh` clones `--depth 1` and cannot mine fix commits; the miner
uses scenariobench's full clones under `workspace/`, correction 12) and BX-6 (the PB-14 import is capped at 30 to 40
Go tasks by its own pre-registered rule, is cli/cli-heavy, and its SWE-Explore half cannot be committed, correction
10).

## 5. Subjects: C1 to C7

A subject is kept only when it passes all seven. C3 is read on macOS (BX-3) and again on Linux (BX-45).

| Criterion | Rule |
|---|---|
| C1 licence | MIT, Apache-2.0, BSD or CC0, read from `LICENSE` at the pinned sha; copyleft and source-available subjects are never used for task derivation |
| C2 size | 5k to 150k lines of the primary language, spread over three size bands |
| C3 suite | green 3 times at the pin on both systems, under 15 minutes on 12 cores, and runnable with **no network at all** from a pre-warmed cache (correction 7) |
| C4 index | `find_callers` and resolved `blast_radius` precision and recall at least 0.95 against go/types or SCIP (ADR-032, section 1 item 3); gopls and Serena are measured on the same subject (BX-5); the refusal rate is published as a finding and every claim is scoped to "subjects passing C4" (correction 14) |
| C5 yield | at least 30 fix commits whose tests fail before the fix and pass after it, at least 15 dated after the pinned model's training cutoff (the second half waits on Q2) |
| C6 fan-in | at least 3 symbols with fan-in 5 or more across at least 3 packages, from the truth graph, never from fathom |
| C7 independence | not fathom, not a repository fathom's own fleet indexes, at most 2 subjects per organization |

A task from a dataset repository outside the kept subjects enters the bank only when that repository has its own C1
and C4 receipts (correction 9).

**Languages and split.** P1: Go, Python and TypeScript, 6 subjects each, 3 dev and 3 held-out by repository
(correction 15 grows held-out beyond 2). About 8 candidates per language are vetted to keep 6. P3 (deferred): Java
and Rust, 3 each.

**Manifest.** `calibration/bench.d/<subject>.yaml`: the corpus.d repo entry unchanged (`name`, `url`, `ref`, `sha`,
`licence`, `subpath`) plus a `bench:` block with `toolchain` (exact versions, lockfile sha256), `setup`, `test`,
`suite_seconds`, `flake_runs`, `l0_reading`, `split`, `task_sources` (dataset, revision, licence each) and
`contamination` (repository age, popularity band, `fresh_after`). `schema/bench` validates it (BX-1).

## 6. Task families and graders

No primary metric is graded by a language model. Every grader is a test run, a set overlap or the type checker.

| Family | Tests | Source | Grader | Baseline it must beat |
|---|---|---|---|---|
| F1 LOC | H4 | Multi-SWE-bench, PolyBench, ContextBench, LCA, post-cutoff slices, on vetted repositories | gold file hit@3, function hit@5 against the fix commit | A1 (agent with grep), A4 (Serena) |
| F2 FIX | H1, H2 | fix commits replayed with the fix removed (revert and redo) | hidden fail-to-pass and pass-to-pass tests (`RunOracle`), false-done rows (`FalseDoneRate`), broken callers from go/types at the final tree | A1, A1+T |
| F3 PLAN | H1 free gate | true rows per gold function and its test; false rows by construction (stub, deleted test, non-reaching test, unwired entry, rename) and refuted rows harvested from real agent plans in prior scenario runs | hidden-test labels | the coverage check (`go test -coverprofile` plus a symbol check, `FalseDoneRate`'s own join) |
| F4 REV | H3 | mutants at fan-in 3 or more killed only by a test outside their package, plus mined review pairs | names a killing caller or test outside the diff; false findings per change | rg and gopls callers of the changed symbols |
| F5 XR | H5 | apidiff breaks and the consumers' adaptation commits | consumer builds and tests, call-site recall | grep, `go.work` and gopls |
| F6 TS | H8 | mined commits with a failing test | safety recall 1.0, share of the suite run | the whole suite, Go's test cache, pytest-testmon |

Strata and truth come only from go/types, SCIP, the lexical baseline and the fix commit, never from fathom's index
(circularity). Gold patches are fetched from the pinned dataset revision and checked by sha256 at run time, never
committed. The scenario JSON gains `stratum`, `split`, `source{dataset, revision, licence}`, `created_at`,
`lexical_rank` and `hop_distance` (an integer, `no-graph-path` or `no-oracle`), all additive.

**Targets, set after BX-42 and BX-12 read the yield.** The draft's 240 F1 tasks per split are not assumed: about
three in four localization tasks are grep-solvable in published measurements, so the non-lexical stratum may not
fill. A shortfall is named in the prereg, which then either widens to D-b, lowers n with the power that remains
stated, or declares the hypothesis unpowered and does not run it.

## 7. Arms and information parity

Every arm runs in a worktree that is a single root commit at the base (no git history), under the clean room, the
same model, the same turn cap and the same prompt except what the arm declares. Fathom's history features are
therefore off in every arm, and every reading says so.

| Arm | Tools | Fathom | Runs in |
|---|---|---|---|
| A0 | none | none | F1, 3 trials: the leak and contamination control |
| A1 | Bash, Read, Edit, Write, MultiEdit, Grep, Glob, NotebookEdit, TodoWrite | none | F1, F2 |
| A2 | Read only | MCP agent surface, no hooks, no stanza | F1 (the strict mechanism arm; Bash, Grep and Glob denied) |
| A3 | A1's | as shipped: MCP agent surface with `alwaysLoad`, hooks, stanza | F1, F2 |
| A3m | A1's | MCP agent surface only, no hooks, no stanza | F1 (separates the server from the hooks and stanza, correction 13) |
| A4 | A1's | none; Serena's MCP server as its README documents, with a routing stanza of fathom's shape (ADR-034 decision 1) | F1, in the H4 dev run (correction 5) |
| A1+V | A1's | fathom's stop hook only (verify on newly claimed rows, blocking) and the scratch daemon it reads | F2 |
| A1+T | A1's | none; a stop hook that runs the manifest's test command and blocks on failure | F2 |

- **H4** tests A3 against A1. A2, A3m and A4 are secondary contrasts. A0 marks leaked tasks.
- **H1** tests A1+V against A1, and A1+V against A1+T, the wheel the gate must beat (correction 5). A3 against A1 is
  a secondary. Every H1 arm gets the same plan-first instruction (grammar v2 rows with `{symbol, package}`), so false
  "done" is adjudicable in every arm and the arms differ only by their stop hook.
- **Parity is a test, not prose.** `CheckArmParity` (BX-60) renders each arm and A1 into harness inputs and fails on
  any difference the arm does not declare: prompt, tools, settings, notes file, turn cap, model and worktree tree
  hash.
- **Pins.** Every ledger row records the exact model id, the `claude` CLI version, the fathom binary sha, the MCP
  surface, the tool list and the measured tool-schema tokens. A run given a model alias is refused (BX-14).

## 8. Contamination control

1. **No network inside a cell.** Package caches are pre-warmed outside the cell (`go mod download` at the base,
   a pip wheelhouse, an npm cache). Inside it: `GOPROXY=off`, `GOFLAGS=-mod=readonly`, `PIP_NO_INDEX=1` with
   `PIP_FIND_LINKS`, `npm_config_offline=true`, and all network denied to the agent's tool subprocesses except
   loopback to the cell's own scratch daemon. The harness process keeps its API connection. Package proxies are a
   leak path, because later releases of a subject contain the fix (correction 7). A spike (BX-55) picks the
   mechanism: Claude Code's Bash sandbox settings or a macOS sandbox profile on the tool subprocesses. Acceptance
   (BX-R2): inside a cell `curl https://github.com` fails, `go mod download <subject>@latest` fails, and the
   subject's tests pass from the cache. WebFetch and WebSearch are already outside the `--tools` allowlist.
2. **The A0 leak arm.** A task A0 localizes (hit@3) in 2 or more of 3 trials is leaked: excluded from the verdict and
   listed. The per-subject A0 pass rate is reported, and a subject above 50% leaves the primary (ADR-031's own
   trigger, applied per subject).
3. **Post-cutoff slices.** Once Q2 pins the model, its training cutoff sets each subject's `fresh_after`. The
   post-cutoff slice is a reported stratum, and the held-out split prefers it. Most dataset tasks predate any
   cutoff and their patches are public, which compresses headroom toward a null; the per-subject A0 rate makes
   that visible (correction 10).
4. **Patches stay out of the tree.** Gold patches are always fetched and checked by sha256. Test patches for
   held-out tasks and non-CC0 sources are stored as sha256 only and fetched at run time (correction 11); a committed
   held-out test patch becomes training data for later model pins. `LoadScenarioCorpus` refuses a held-out or
   non-CC0 task that carries one verbatim (BX-47). The 35 public-go tasks are cli/cli only and were read by
   public-pilot-2 and reuse-pilots 1 to 4, so they are dev only.

## 9. Dry-run harnesses: proof that the instrument discriminates

These are harnesses behind the existing port, selected with `-harness`. None spends a token.

| Harness | Does | Every kept task must |
|---|---|---|
| null | edits nothing, answers nothing | FAIL |
| oracle | applies the verified gold patch (F2) or answers the gold files (F1) | PASS fail-to-pass and pass-to-pass, hit@3 = 1 |
| wrong-patch | applies the test patch alone or a killed mutant (F2), answers a fixed non-gold file set (F1) | FAIL |
| flake | each of the three above, 3 times | give the same result each time |
| scripted fathom | sends the issue's identifiers through `search`, `find_symbol` and `symbol_context` over the cell's MCP | show `surface_calls > 0` in 100% of wired cells |
| scripted grep | ranks files by rg over the issue's identifiers | complete the A1 pipeline end to end |

`DryRunVerdict` (BX-36) reads the cells and names each task to keep or drop with the reason. Scripted agents are
harness checks only (correction 16): a script is a floor, not a ceiling, and the grep script ranks with the same
lexical ranking that defines the strata, so its per-stratum scores are tautological and never reported. The scripted
fathom agent exists because a wired arm with no MCP tools connected voided the first strengths-matrix round (O41,
ADR-034).

## 10. Pre-registration, outcomes and scoring

A prereg is two committed files: `docs/bench/prereg/<id>.md` (prose, signed by the controller) and
`docs/bench/prereg/<id>.json` (the machine-evaluable criterion, validated by `#Prereg`, the precedent of BC-6). Its
sha256 is stamped on every ledger row, and `scenariobench run -prereg` refuses a dirty tree or a file that differs
from HEAD.

Each prereg fixes, before any data under it: hypotheses, strata and arms; a prediction per arm including the
predicted nulls; the primary metric and its minimum effect of interest (MEI); the equivalence margin δ for each
predicted-null stratum; alpha 0.05 with Holm across the prereg's primaries; n from the power table; the turn cap; the
cost cap; the seed; exclusions; and a deviation log.

**Three outcomes (correction 2).** For a predicted-win primary, on the Holm-adjusted interval:

- **Confirmed:** the lower bound is above 0.
- **Refuted:** the upper bound is below the MEI. An interval entirely inside (0, MEI) reads refuted, with the
  estimate printed: the effect exists and is smaller than worth claiming.
- **Inconclusive:** anything else.

For a predicted-null stratum, by TOST at alpha 0.05 (the 90% interval):

- **Equivalent:** the 90% interval lies inside (-δ, +δ).
- **Harm:** the 95% upper bound is below 0. Harm counts as a loss.
- **Inconclusive:** anything else. An interval above +δ is reported as an unexpected effect, not a win.

Proposed values, set at sign-off: H4's MEI is +10 percentage points of hit@3 in the non-lexical stratum and its δ is
5 points in the lexical stratum; cost is a secondary with a non-inferiority margin of 1.3 times A1's. H1's MEI is a
30% relative cut in the per-task false-done rate, with F2P success held by a 5-point δ. The draft's +15 points is far
above any prior (LCA run 1: 13 of 39 tasks discordant, a one-cell effect), which is why n comes from measurement.

**Scoring by assignment (correction 1).** Every cell is scored by its actual outcome under its assigned arm. A
wired cell whose surface was never called keeps its outcome; it is not a failure by fiat and not dropped. The
zero-call rate of each wired arm is printed on its own line. A per-protocol secondary (wired cells with
`surface_calls > 0` only) may be reported and is labelled "biased toward fathom: selects on tool use". A LOST cell is
an instrument defect, handled by the lost-cell policy.

**No-reads.** A run is a no-read, with no outcome claimed, when its cost cap binds, when lost cells exceed 5% of the
planned cells in any arm (differential loss biases the contrast), or when any arm's surface failed to connect in
more than 5% of its cells. A no-read's cells may be borrowed by key into a completed run under the same prereg only
if the owner raises the cap before anyone reads them.

**Lost-cell policy.** Infrastructure failures (a LOST server, a harness crash, API transport, oracle infrastructure)
are retried at most twice. Model outcomes (wrong answer, refusal, a max-turns stop) are never retried. Lost cells
and max-turns stops are reported per arm.

**Results template (BX-27).** One table, losses first: per primary the estimate, interval and outcome; per-repo
effects; cost and tokens including tool-schema tokens; the zero-call line; lost cells and max-turns stops per arm;
A0 leaks; deviations; and a mandatory "Where fathom lost" section. Every artifact opens with the verdict in plain
language (ADR-034 decision 2).

## 11. Statistics: power and clustering

**Power is measured, not assumed (correction 4).** The shakedown measures, per task shape, the standard deviation of
per-task mean differences in hit@3, the intra-repository correlation and the false-done base rate. `PowerTable`
(BX-26) turns them into n per primary: a paired t or Wilcoxon size on task means for H4 (each task's mean over 3
trials, not a binary McNemar), and a discordant-pair size for H1's per-task false-done outcome, both at 80% power
with the Holm-adjusted alpha and a design effect for repository clustering. If the n a MEI needs exceeds the bank or
the envelope, the prereg says so and either raises the MEI (before data, recorded) or declares the hypothesis
unpowered and does not run it. No run goes below its n, and n is fixed with no optional stopping.

**Clustering (correction 15).** Three dev repositories per language are too few for clustered standard errors. The
primary interval is a wild-cluster bootstrap with repositories as clusters (Webb's six-point weights, seed
recorded), over the 9 dev repositories of the P1 languages. The task-clustered paired bootstrap the scenario ledger
already computes (`ComputePairedRunStats`) is printed beside it. When the two disagree on the outcome, the more
conservative decides. Per-repository effects are always printed.

## 12. Caps and cost

The turn cap is pre-registered and generous: LCA run 1's 15-turn cap is what produced fathom's `error_max_turns`
losses, and those stops are now reported per arm. The `-max-usd` cap for a dev run is 1.25 times the planned cells
priced at the shakedown's 95th-percentile cost per task shape, and a binding cap makes the run a no-read
(correction 8). A larger model is re-priced from `internal/cost`.

Basis: LCA run 1 cost $0.09 to $0.12 a localization session under a 15-turn cap (a floor, since the cap ended
sessions early); reuse-pilot-4 change sessions cost $0.32 to $2.96, mean about $1.17. Stop-hook loops add turns to
the H1 arms.

| Run | Shape | Expected | Cap |
|---|---|---|---|
| Shakedown (no claim) | 10 F1 (A0 ×3, A1, A3) + 10 F2 (A1, A1+V, A1+T, A3), 1 trial | about $55 | $120 (the worst observed session cost on every F2 cell) |
| H4 dev (indicative) | 240 F1 tasks × A1, A3, A4 × 3 trials, A0 × 3 | $260 to $550 | from the shakedown |
| H1 dev (indicative) | 75 F2 tasks × A1, A1+V, A1+T, A3 × 3 trials | $1,050 to $1,400 | from the shakedown |

Only the shakedown is asked for now (Q1). The dev envelopes come back with the shakedown's measured costs and the
power table.

## 13. The tuning loop and held-out discipline

1. A losing stratum maps to the knobs that can move it (BX-28): verify criteria, `search` receipts, `blast_radius`
   depth, surface membership of `localize`, `find_callers` and `cross_repo_callers`, the review byte cap, and
   obligations ranking.
2. Tuning runs on dev only. Every tuning commit cites the stratum and the dev delta it moved (BX-31). Agent spend for
   tuning needs its own prereg row.
3. A held-out read is logged in `docs/bench/heldout-reads.md` by the runner itself (BX-29), which refuses a held-out
   task without a named prereg. A second held-out read after a change demotes that split to dev, and a fresh time
   slice is re-mined after each tuning freeze.
4. Regression: hermetic fixture versions (F3 fixtures, a small committed F4 set, L0 on fauxtown) run in `task
   validate` (BX-30). The full free layer over the corpus runs as a scheduled helper-Mac lane, and a breached
   threshold files a `fathom observe` entry (BX-74).

## 14. Phases, readiness and lanes

| Phase | Spend | Content | Exit |
|---|---|---|---|
| P0, starts today | $0 | schema and manifest loader, vetting and fix-commit mining, the stratifier and hop covariate, the three dry-run harnesses and their verdict, kit statistics, the prereg template, the ADR-034 amendment; vetting of 15 candidates, stratum yield and dry runs of the existing banks (public-go 35, reuse-go 4, LCA 40) | BX-M0: the plan verifies, the three readings are committed, at least 10 candidates pass C1 to C4, C6 and C7 |
| P1 | $0 | the bank to its targets, harness hardening, the free readings, both preregs, readiness | BX-M1: the readiness checklist reads PASS on every line |
| P2 | owner-approved | shakedown, n set, H4 dev, H1 dev | BX-M2: readings published either way |
| P3 | per prereg | tuning on dev, held-out confirmatory reads, further competitor arms, the conditional and deferred hypotheses | BX-M3: held-out readings |

**Readiness for the first approved run (BX-R1 to BX-R10):** every kept task passes the null, oracle, wrong-patch and
flake checks; egress denial is proven inside a cell; the model id and CLI version are pinned; the cost and turn caps
are set; the preregs are committed and the run config carries their sha; the held-out split is committed and
unread; the scripted fathom agent shows MCP calls in every wired cell; `scenariobench plan` cell counts equal the
prereg's n; arm parity passes; and the host has headroom (one run at a time, on the helper Mac).

**Lanes.**

- *Codex on the helper Mac* (isolated checkouts, written briefs that name any old-behavior test they may change):
  every code row, vetting and mining runs, L0 readings, dry runs and the free readings. The kit statistics rows
  need the agentic-kit checkout beside fathom's and land through `task sync-kit`.
- *Cloud:* Linux re-validation of C3, the licence audit of subjects and datasets, and a second-opinion review of the
  prereg drafts.
- *Controller only:* hypothesis choice, predictions and strata definitions, the held-out commit, dropping a task or
  subject after the dry runs, prereg sign-off, interpreting readings, and every spend request.

## 15. Owner questions

Q1 to Q7 keep the roadmap draft's numbering, which the critique cites. They are this program's questions, not the
ids of `docs/intent/questions.md`.

- **Q1 Spend.** Approve the shakedown alone, about $55 behind a $120 cap? The dev envelopes return sized from it.
- **Q2 Models.** Sonnet only (matching every prior) or Sonnet plus one larger model, and which exact pinned ids? The
  answer sets C5's post-cutoff half, the live slice and the held-out split, so it is needed before BX-R6.
- **Q3 Competitor arms.** A4 (Serena) runs in H4 dev. Do A2 and A3m run in dev or only in the shakedown, and do
  gopls and graphify arms run in P3 or never? Each extra arm adds about a third to a run.
- **Q4 Codex arm.** In P3 with an ADR-029 amendment, or not at all? Is a container spike allowed within the 8 GB VM
  cap?
- **Q5 Publication.** Nulls and losses are published in `docs/bench/` by default. Is an external write-up wanted,
  and at which milestone?
- **Q6 Held-out reads.** Recommended: only at a named release candidate. A read "as-is before tuning" spends the
  held-out set's one clean read, and the dev run already measures as-is.
- **Q7 Blind transcript review.** Will the owner review a blind sample of about 20 transcripts per experiment, as an
  audit of the graders and never a metric?
- **Q8 The C4 floor.** Keep ADR-032's 0.95 for bench subjects, or amend it to 0.90 for subjects whose free layers
  read fine?

## 16. Risks

1. Another null is the likeliest outcome. The prereg makes it publishable, the strata isolate where an effect is
   predicted, and the free layers carry the claims they can prove.
2. An instrument defect voids a run, as it already has (O41, the reuse-pilot-4 scorer). The dry runs and the
   scripted fathom agent run before any spend.
3. Leakage through the network or memorization. No network in a cell, post-cutoff slices, the A0 arm.
4. A grep-solvable task mix hides any effect. Strata are precomputed and the yield is measured before targets.
5. Circularity. Strata and truth never come from fathom's index.
6. An underpowered or peeked result. Measured n, fixed n, the shakedown labelled no claim.
7. The host destabilizes long runs. The helper Mac, one run at a time, resume, the lost-cell policy.
8. Model drift mid-run. Pinned ids, interleaved arms and shuffled tasks from a recorded seed (sampling itself
   cannot be seeded, and readings say so).
9. Tuning overfits. Held-out by repository, logged reads, demotion on a second read, re-mined slices.
10. A licence or ADR-011 leak in a public repository. C1, patches fetched rather than committed, the push-gate leak
    check.
11. Cost overrun. Caps that make a run a no-read, sized from the shakedown's 95th percentile.
12. The bank stays Go-heavy. P1's exit needs Python and TypeScript at their targets or a named shortfall.

## Decision models

None applies. Every grader, stratifier, split, power figure and outcome is computed deterministically with provider
`none`, and the program's own rule refuses a model grader on any primary metric. The agent sessions are the subject
under measurement, not decision-port sites. The one place a typed question could enter is H7's claim extraction (a
`choice` per claimed fact: true, false or unknown), and it stays deferred until a deterministic extractor exists. If
it is ever proposed, it ships opt-in, falls back to `unknown` with provider `none`, has its own opt-out, and is kept
or killed by agreement with the owner's blind labels (Q7) on at least 100 claims.
