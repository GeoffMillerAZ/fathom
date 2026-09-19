# The scenario bench — runbook

The strengths bench (`docs/plans/strengths-bench.md`, `docs/vision/27-strengths-bench.md`,
ADR-033) is one Go driver, `cmd/scenariobench`, over corpora of *scenarios*
(`testdata/scenariobench/*.json`, contract `internal/scenariobench/types.go`):
a task is a change to a pinned subject at a pinned commit, with a category,
gold, an oracle, and a prediction per arm registered before any spend. An
**arm is data, never code** — see "How to add an arm" below.

This page covers the free layers (`task bench-scenario-plan`,
`task bench-compare-edges`, `task scenariobench`) and the two layers that
cost real LLM tokens (`task bench-scenario-guards`,
`task bench-scenario-pilot`) — `docs/intent/autonomy.md` requires asking
before LLM spend, so neither of the paid tasks is ever wired into
`validate`, a hook, or CI.

Wave status (`docs/plans/strengths-bench.md`): this page ships with wave 5
(SB-33..36) — `ClassifyChange`, `DeriveScenarioTasks`, and
`testdata/scenariobench/public-go.json`. `cmd/scenariobench` itself
(SB-19), `guards-fathom.json` (SB-24), and `lookups-vs.json` (SB-32) are
other waves of the same plan; the Taskfile commands below are written
ahead of them so they are in place the day those land — until then,
`bench-scenario-guards` and `bench-scenario-pilot` fail with "no such
file or directory" for `cmd/scenariobench` or the corpus they name, and
`bench-compare-edges` the same for `cmd/indexbench`.

## The public Go corpus (`testdata/scenariobench/public-go.json`, SB-34)

35 tasks, categorised by `ClassifyChange` (SB-33) from the real pinned
Multi-SWE-bench Go snapshot — 21 `bug_fix`, 9 `cross_package_feature`, 5
`signature_change`. Nothing in it is authored: every gold file and gold
function is derived from a developer's own merged patch at its base
commit, the same discipline as the sibling `docs/bench/public-loc.md`
corpus. `internal/scenariobench/derive_test.go`'s
`TestPublicScenarioCorpusIsDerived` re-derives it from the pinned
snapshots and a cached subject clone and compares the result to the
committed file **byte-for-byte**; it skips (never fails) when a snapshot
is absent, which is the normal case on a fresh clone or CI (the snapshots
live under gitignored `workspace/`).

| Class | Truth | Tool |
|---|---|---|
| Category | patch shape: `go.mod` touched, an exported func/method signature differing between before/after (`go/parser`), ≥2 non-test Go packages touched, a governance file touched alongside code, else `bug_fix` | `scenariobench.ClassifyChange` |
| Gold files | non-test `.go` paths the gold patch modifies (ADR-031's rule: 1–5 files) | `publicloc.GoldFiles` |
| Gold functions | functions whose span at `base_commit` encloses a changed line | `publicloc.EnclosingFuncsForHunks`, read from a shallow single-commit git clone |
| `fail_to_pass` / `pass_to_pass` | the source's own `n2p_tests`/`f2p_tests` (fail→pass) and `p2p_tests` (pass→pass) test names | a side scan of the same pinned JSONL (`publicloc.ReadMultiSWEBench` itself discards these fields) |

**What SWE-Explore-Bench actually contributes: nothing, today, and that is
recorded, not hidden.** The plan and `docs/bench/public-loc.md` both
describe deriving from "the pinned Multi-SWE-bench Go split ... and
SWE-Explore-Bench Go JSONL". The published SWE-Explore-Bench snapshot
(verified 2026-09-13: `SWE-Explore-Bench/SWE-Explore-Bench`,
`bench.final.public.jsonl`, 848 rows, 84 Go-shaped by file extension)
carries `instance_id`, `repo_dir`, and
`ground_truth{read_core_files, modified_core_files, read_core_regions, ...}`
— exploration-reading truth, useful for M5 (`ReexplorationOverlap`,
wave 3) — but **no `base_commit` and no patch**, so it cannot seed this
corpus's `Subject` or `Oracle`. `DeriveScenarioTasks` still pins it
(URL, revision, sha256, license) for provenance and records
`swe_explore_usable_instances: 0` in the corpus's `notes`, with the reason,
rather than silently proceeding on 100% Multi-SWE-bench data while the
description implies two sources. Two more findings from the same
verification pass, also recorded in `notes`:

- The dataset's Hugging Face license is `cc-by-nc-nd-4.0` (a
  no-derivatives license), not the `MIT` `internal/contextbench/publicloc`
  records for it — this corpus's own `sources[].license` carries the
  verified value.
- Neither pinned source carries a merge or close timestamp, so "prefer
  `merged_at` after 2026-06-01" is a real, tested code path
  (`partitionByCutoff`) that is a documented no-op against these
  snapshots (0 of 391 eligible candidates have a known post-cutoff date).
- ADR-031's rule (dedupe, filter to 1–5 gold files, sort by
  `instance_id`, take the first 35) happens to draw all 35 from one repo,
  `cli/cli` — it alone supplied enough eligible candidates ahead of
  `grpc-go` and `go-zero` alphabetically. `notes.repo_diversity` names
  this; stratifying by repo is future work, not done here.

## Test patch and calibration

Multi-SWE-bench (and SWE-bench generally) ships each instance's tests in
**two** patches: `fix_patch` (the developer's real fix — never seen by the
agent) and `test_patch` (the tests that prove it — hidden from the agent
too, applied by the harness only at evaluation time). An earlier version
of this corpus applied only `fix_patch`'s own gold files and ran
`fail_to_pass` straight from source, which silently dropped `test_patch`
— found 2026-09-13 when all 28 `fail_to_pass` tests of `SB-cli__cli-10043`
turned out to already PASS at `base_commit`: without the hidden test file,
there was nothing for the fix to make pass. `DeriveScenarioTasks` now
carries the source's `test_patch` verbatim onto `Gold.TestPatch` (plus
`Gold.TestPatchSHA256`), and the oracle applies it itself:

- **`ApplyTestPatch`** (`oracle.go`) runs `git apply --3way` (falling back
  to a plain `git apply`) of `Gold.TestPatch` in the cell's worktree
  **after every session and before the tests** — the patch is never on
  disk while an agent can read it. A patch that fails to apply is
  recorded as `calibration.not_measurable: "test patch did not apply: …"`
  and the cell fails rather than silently running without it.
- **`CalibrateSubject`** (`oracle.go`) answers the question the raw
  source data cannot: of the named `fail_to_pass`/`pass_to_pass` tests,
  which ones actually behave the way SWE-bench assumes IN THIS
  environment? It applies the test patch in a **separate, throwaway
  worktree of the shared clone at `base_commit`** (never the cell's own
  worktree) with no fix, runs the task's test command once, and records
  which `fail_to_pass` tests FAILED at base (only those are the agent's to
  fix) and which `pass_to_pass` tests PASSED at base (only those can be
  broken BY the agent). The reading is cached at
  `<root>/workspace/scenariobench/calibration/<sha256>.json`, keyed on
  `(subject repo, base_commit, test-patch sha256)` — computed once no
  matter how many tasks, arms, or trials share the triple, never inside a
  cell's own worktree. This matters in practice: of the public corpus's
  3,849 `pass_to_pass` tests, 116 fail at `base_commit` in this
  environment (network, auth, wall-clock — not code), and calibration is
  what keeps them from being blamed on the agent.
- `RunOracle` folds calibration into the verdict: `FailToPassOK` requires
  every `fail_to_pass` test calibration found FAILING at base to pass now
  (one that already passed at base is dropped from the requirement, not
  counted against either arm); `PassToPassOK` requires every
  `pass_to_pass` test calibration found PASSING at base to still pass
  (one already failing at base is excluded — environment, not the agent).
  A task whose `fail_to_pass` set has no test failing at base is
  `calibration.not_measurable: "no fail_to_pass test fails at base with
  the test patch applied"` — not a pass, not a failure, a task this
  environment cannot measure. `OracleResult.Calibration` records the
  counts (`fail_to_pass_failed_at_base`/`_total`,
  `pass_to_pass_effective`/`_total`) on every cell so a reader never has
  to open the cache to see what was actually asked of the agent.

## Pins

The two files `DeriveScenarioTasks` was actually run against
(`internal/scenariobench.DeriveOptions.MultiSWE` / `.SWEExplore`):

| Source | License | File | Revision | sha256 |
|---|---|---|---|---|
| [Multi-SWE-bench](https://huggingface.co/datasets/ByteDance-Seed/Multi-SWE-bench) — `go/cli__cli_dataset.jsonl` | CC0-1.0 | `multi-swe-cli__cli_dataset.jsonl` | `56ff018c04a38e27ada1e9d0a6d5839a51f88f0d` | `18abaac48227bf15a64bcf649a4fcdb34ea8148962f371241ea2bb91608cf4b1` |
| Multi-SWE-bench — `go/grpc__grpc-go_dataset.jsonl` | CC0-1.0 | `multi-swe-grpc__grpc-go_dataset.jsonl` | `56ff018c04a38e27ada1e9d0a6d5839a51f88f0d` | `cd8984b884679c44f71c2af958d74c692f4a1feb6ec68e74b0ddc312526b4e13` |
| Multi-SWE-bench — `go/zeromicro__go-zero_dataset.jsonl` | CC0-1.0 | `multi-swe-zeromicro__go-zero_dataset.jsonl` | `56ff018c04a38e27ada1e9d0a6d5839a51f88f0d` | `6047e4f92571292d8fa77bb866b875930c2aa600f46d408eb02bfc7ed3e48e0f` |
| [SWE-Explore-Bench](https://huggingface.co/datasets/SWE-Explore-Bench/SWE-Explore-Bench) — `bench.final.public.jsonl` | CC-BY-NC-ND-4.0 (pinned for provenance; contributes 0 tasks — see above) | `swe-explore-bench.final.public.jsonl` | `bdb0ae45d7c337d9e1dc3ebfe2a0af6bc7c1fbd9` | `dc4f114ececd0bfb987361c26ae5e2440456e2cccb36adfccb09ea5385aec202` |

The revision for each is the dataset repo's Hugging Face commit sha
(`GET https://huggingface.co/api/datasets/<repo>`, field `sha`) at fetch
time, recorded as `SnapshotPin.Revision`. The files themselves are not
committed (`workspace/` is gitignored) — only their metadata, ids, and the
derived symbols reach git, same rule as `docs/bench/public-loc.md`.

Subject clones live under `workspace/bench/subjects/<org>__<repo>.git`
(bare, `--filter=blob:none`), one per distinct repo, each commit fetched
with `git fetch --quiet --depth 1 origin <sha>` — no ref needed, no full
history, and idempotent (a commit already present is not re-fetched).

## Before any spend: the dry cell

Run one wired and one control cell with the fake harness against the real
subject first. It clones and worktrees the subject, builds the fathom
binary, starts the scratch daemon, compiles the obligations layer once
(about seven minutes and $0.12 on the fathom repo; later cells on the same
subject, commit and arm reuse the store — `compiled.ok` beside it), replays
a committed transcript instead of calling the model, and runs the real
tests and detectors:

    go run ./cmd/scenariobench run -corpus testdata/scenariobench/guards-fathom.json \
      -harness fake -fake-streams-dir testdata/scenariobench/streams \
      -fake-streams '{"guards-i31-workspace-artifact/full":"single-session.jsonl"}' \
      -only guards-i31-workspace-artifact -arm full -trials 1 -harness-version fake -run-id dry -root .

A replayed session edits nothing, so the oracle FAILS by design; what the
dry cell proves is that every detector can build and run in the worktree
(a subject with a `go.work` naming sibling modules builds under
`GOWORK=off`, which the driver sets for every subject subprocess) and that
the scratch daemon stops afterwards (`pgrep -fl serve-http`). Five driver
defects were found this way on 2026-09-13 (O35) before a dollar of pilot
spend.

A layer-dependent corpus (any task with `requires_layer` true, explicit or
defaulted) adds one more thing the dry cell must prove: the probe must
return rules. `PrepareSubject` calls the scratch daemon's own
`obligations` for the arm's `probe_file` right after compiling and
`post_wire_setup`; a `compiled layer serves no rules for <file>` refusal
here is the dry cell doing its job — it means `compile_layers` is unset,
the compile produced nothing reachable, or `probe_file` names a path the
layer never governs — never a bug in the dry cell itself (O45).

**Two real dry cells before the pilots.** The replayed cell proves the
driver; it cannot prove the session. Run one real session per arm on a
lookup task (`-only H1-T10 -trials 1 -model claude-haiku-4-5-20251001`,
about a cent each) and read each cell's `mcp_servers`, `hooks_fired` and
`surface_calls`; then one real session on a change-shaped task and check
its transcript for "requires approval" or "was blocked" — the first pilots
on the clean room lost fifty cells to a denied `mkdir` that no dry cell had
exercised (O43).

## Repeat it yourself

**What a stranger needs:** Go (the module's version — `go.mod`), the
`claude` CLI on `PATH` and an authenticated API key **only** for the two
paid tiers below (`bench-scenario-guards`, `bench-scenario-pilot`) — the
free tiers need neither. Network access to GitHub is needed once, to fetch
the pinned snapshots and clone the three subject repos (cached after
that). Competitors (`docs/bench/competitors.md`, another wave) are optional
and only matter for `bench-compare-edges`.

1. **Fetch the pins** (network, no LLM, one-time — cached under
   `workspace/`, gitignored):

   ```sh
   mkdir -p workspace/bench/sources
   curl -sL -o workspace/bench/sources/multi-swe-cli__cli_dataset.jsonl \
     https://huggingface.co/datasets/ByteDance-Seed/Multi-SWE-bench/resolve/main/go/cli__cli_dataset.jsonl
   curl -sL -o workspace/bench/sources/multi-swe-grpc__grpc-go_dataset.jsonl \
     https://huggingface.co/datasets/ByteDance-Seed/Multi-SWE-bench/resolve/main/go/grpc__grpc-go_dataset.jsonl
   curl -sL -o workspace/bench/sources/multi-swe-zeromicro__go-zero_dataset.jsonl \
     https://huggingface.co/datasets/ByteDance-Seed/Multi-SWE-bench/resolve/main/go/zeromicro__go-zero_dataset.jsonl
   curl -sL -o workspace/bench/sources/swe-explore-bench.final.public.jsonl \
     https://huggingface.co/datasets/SWE-Explore-Bench/SWE-Explore-Bench/resolve/main/bench.final.public.jsonl
   sha256sum workspace/bench/sources/*.jsonl   # compare against the table above
   ```

2. **Free tiers — no LLM spend, no `claude` CLI needed:**

   | Tier | Command | Cost |
   |---|---|---|
   | free (corpus) | `task bench-scenario-plan` | 0 — lists the public corpus's cells and keys |
   | free (index oracle) | `task bench-compare-edges` | 0 — competitor edges vs the go/types L0 truth |
   | free (gate) | `task scenariobench` | 0 — corpus load, category/prediction gate, driver unit tests; re-derives the public corpus when the pins from step 1 are present |

3. **Paid tiers — ask before running (`docs/intent/autonomy.md`), need
   `claude` on `PATH` and an API key:**

   | Tier | Command | Expected cost (vision 27 §9) |
   |---|---|---|
   | targeted | `task bench-scenario-guards` | ≈ $20–40 |
   | pilot | `task bench-scenario-pilot` | ≈ $90–130 |
   | lookups-vs | `task bench-scenario-lookups-vs` | fathom vs. graphify/serena/gopls/grep, 2 trials — 140 cells (`task bench-scenario-plan -- -corpus testdata/scenariobench/lookups-vs.json` lists them free first) |
   | lookups-ts | `task bench-scenario-lookups-ts` | COSTS TOKENS, NOT run this round (SU-23, ADR-041 decision 9): fathom vs. grep on the `nest` calibration subject (TypeScript), 2 trials — 24 cells (`task bench-scenario-plan -- -corpus testdata/scenariobench/lookups-ts.json` lists them free first) |
   | lookups-py | `task bench-scenario-lookups-py` | COSTS TOKENS, NOT run this round (SU-23, ADR-041 decision 9): fathom vs. grep on the `fastapi` calibration subject (Python), 2 trials — 24 cells (`task bench-scenario-plan -- -corpus testdata/scenariobench/lookups-py.json` lists them free first) |
   | findings loop | `task bench-scenario-findings` | 4 tasks, arms full vs control, n=3 (FL-24, `docs/plans/findings-loop.md`; `testdata/scenariobench/findings-loop.json`) — committed only, **not run** by this wave: the CLI it exercises (`fathom findings import`, the pre-edit "open findings" block) is other waves' work against sections A-D of that plan |
   | credible | 30 tasks × 3 trials per matrix row (`docs/vision/27-strengths-bench.md` §9) | several hundred dollars per row; operator decision, not a Taskfile target |

   Regenerate `testdata/scenariobench/public-go.json` itself (after a
   pin bump, or a `ClassifyChange`/`DeriveScenarioTasks` change) by
   calling `scenariobench.DeriveScenarioTasks` with the pins above and the
   `SubjectsDir` set to `workspace/bench/subjects` — free, no LLM, network
   only for the clones — then write the result with
   `encoding/json` (two-space indent, `SetEscapeHTML(false)`, trailing
   newline) to keep it byte-identical to what `TestPublicScenarioCorpusIsDerived`
   expects. `cmd/scenariobench` (another wave) is the intended long-term
   entry point for this once it lands.

## How to add a task

A task is one `internal/scenariobench.Task` (see `types.go`) inside a
corpus's `tasks` array: a pinned `subject{repo_url, base_commit, service,
language}`, `sessions{count, max_turns, mode}`, an `oracle`
(`fail_to_pass`/`pass_to_pass` test names, or `detectors` for a guards
task), `gold{files, functions}`, a `derivation` string explaining where
every field came from, and — non-negotiably — a `predictions` entry **per
arm the corpus declares** (`TestEveryScenarioTaskHasPredictions`, wave 3,
fails a corpus missing one). Hand-authoring one: run `ClassifyChange`
against the patch to get its category and evidence, use
`publicloc.GoldFiles`/`EnclosingFuncsForHunks` for gold, and write the
prediction from vision 27 §1's matrix row the category maps to — never
skip straight to "wins": a task with no registered no-effect case is not
falsifiable. Deriving many at once from a pinned public dataset: extend
`DeriveScenarioTasks` (`internal/scenariobench/derive.go`) the way it
already reads Multi-SWE-bench, and add the new source's pin to the table
above.

## The guards-m1 corpus (`testdata/scenariobench/guards-m1.json`, RC-50)

A third guards corpus, redesigned after `guards-fathom.json` (SB-24) and
`guards-adr.json` (SB-49): `guards-pilot-3`/`guards-pilot-4` and
`guards-adr-pilot-1` (`.fathom/observations.md`, "The guards pilot, read")
found the no-effect condition dominating because every rule sat in a
CLAUDE.md both arms could read, and even `guards-adr.json`'s ADR-only
rules left five of eight edit sites carrying an adjacent comment naming
the same rule. `guards-m1.json`'s six tasks were each checked for that
while being derived, and go one step further on placement: (a) a
determinism rule stated only in `internal/CLAUDE.md`, edited nowhere near
the package it governs; (b) ADR-011's org-agnostic rule, which names no
package, applied to a prompt that names no file either; (c) ADR-020/025's
CGO-purity rule, where the edited file's OWN comment argues the opposite
of the rule (a CGO accelerator here is "free" since the binary already
pays for CGO elsewhere -- true of the binary, false of this package); (d)
a rule that exists ONLY in the compiled `.fathom/guard.cue` layer and its
generated test (`rules/layer_rules_test.go`) -- no CLAUDE.md or ADR states
it in prose at all (the corpus's lexical-verify token, `served-only`, sits
in this task's own id); (e) one `docs/CLAUDE.md` bullet that governs three
separate files at once (a tool's registration, its catalog entry, and a
use-case doc's own `## Maintained` line) from a fourth directory the edit
never touches; and (f) a process rule -- a commit's own `Verified:`
trailer -- that fires at a `git commit` Bash call a `PostToolUse`
Edit/Write hook was never wired to gate, registered honestly as a
no-effect boundary case on both arms rather than a win.

Tasks (a) and (c) need an edit site this repo does not actually carry
(a small analyzer package, a store search helper with a deliberately
misleading CGO comment) — committing either to this repo would mislead a
real contributor reading `internal/analysis` or `internal/store`, so
instead each task's own `subject.setup` (`Subject.Setup`, `types.go`;
`PrepareSubject` runs it in the worktree BEFORE the first ingest, per its
own doc comment) materializes the scaffold with a heredoc the moment a
cell's worktree is prepared — real for the length of that one worktree's
life, never real on `main`. Tasks (b), (d), (e) and (f) need no scaffold:
they touch files this repo already carries.

Every task also carries a `violation_probe` (`ViolationProbe`,
`types.go`): a tiny scripted edit — a `sed`, a heredoc, or (for the
commit task) a real `git commit` — that violates the rule.
`internal/scenariobench/guardsm1_probe_test.go`
(`TestGuardsM1ViolationProbesFireAndClear`) applies each task's own
`subject.setup` (when it has one) to a disposable `git archive HEAD` copy
of the tree, then proves the primary detector actually discriminates:
green once the scaffold is in place as committed, red once probed (the
four "breakage" tasks), or red on a plausible partial completion and
green once the omitted piece is finished (the two "omission" tasks, e and
f, whose untouched-tree reading is never green — see that file's own
package doc comment for why). This is the corpus's own proof that its
detectors detect, not merely that `TestGuardsCorpusDetectorsExist`
(`guards_test.go`) proves they run. `task bench-scenario-guards-m1` is the
paid tier (`-trials 3`, matching RC-51's pilot); RC-51 is the
operator-run pilot itself.

## The findings-loop corpus (`testdata/scenariobench/findings-loop.json`, FL-24)

Vision 30 / ADR-040's own bench claim (§6): does an open, externally-sourced
finding served at the point of work change the fix the way a compiled
obligation does (`guards-fathom.json`'s M1 claim), or does a review-kit
finding need its own measurement? Four tasks, each a real, currently
unaddressed gap this repo's own tree carries — a bare `return err` with no
step named (`recordPurge`, error-handling), a map-derived field with no sort
call once it exists (`DerivePatterns`, contract-violation, the same
determinism contract `guards-i1-packagedeps-sorted` tests — a different
file), a doctor row that reports only the first of several violated
concerns with no bound (contract-violation), and a change detector that
never notices a deletion (`hasChanged`, correctness). Each is anchored to a
real `path#Lstart-Lend` in this repo at the corpus's own `base_commit`
(`anchor.digest` is that file's own sha256 — `TestFindingsLoopCorpusParses`
checks it still matches); the task's prompt is an ordinary-sounding small
ask whose correct completion also satisfies the finding's own remedy text,
mirroring how `guards-fathom.json`'s naive-fix tasks work but for an
imported review finding instead of a compiled rule.

`subject.setup` (shared by every task, since `Subject.Setup` runs before
either arm's own `Arm.Setup` — `subject.go`) appends a `[findings]` section
to this repo's own `.fathom/config.toml` and writes a five-event review-kit
projection outbox to `.reviewkit/outbox/projection.json` (schema
`schemaVersion`/`kind`/`outcome`/`producer`/`subject`/`visibility`/
`sourceId`/`sourceVersion`/`payloadDigest`/`findingId`/`observedAt`/
`anchor`/`rule`, vision 30 §3.3) — four events match a task's own
`rule.id`, a fifth (`genclaudemd.go`, `performance`) sits elsewhere in the
repo and is never touched by any task, so the outbox reads as a repo
carrying several open findings rather than exactly one per edited file.
Arms: **full** (the outbox stays materialized — once the CLI other waves
are building imports and serves it) vs **control**, whose own `Arm.Setup`
(`"rm -f .reviewkit/outbox/projection.json"`) deletes the same file before
session 1 — the "express the difference as an arm-specific setup, not a
second subject variant" shape `types.go`'s own `Arm.Setup` doc comment
supports, and the same one `guards-fathom.json` uses for `wired`. Both arms
are otherwise identical (`wired: true`, `compile_layers: true`): the
comparison isolates the outbox, not fathom's presence at all.

Paired criteria per task, scored once the importer/hook exist:
`finding_addressed` (the finding's own remedy, a detector distinct from
`change-present`) and `finding_cited` (`Oracle.AnswerAnyOf`: the task's
`rule.id` or the served "open findings on this file" line appearing in the
final session's answer), plus the same `no-unrelated-rewrite` scope
detector every corpus here uses (a diff confined to the edited file and its
own test). Kill criteria (vision 30 §6, `notes.kill_criteria`): no lift in
`finding_addressed` at n≥3; hook injection over the ADR-036 budget; a
disagreement or drifted finding served as current.

**This corpus is committed but not runnable yet.** `fathom findings import`
(FL-10), the pre-edit "open findings on this file" block (FL-20), and the
session-outbox sync (FL-22) are other waves' work against
`docs/plans/findings-loop.md` sections A-D and are not present in this
clone; `task bench-scenario-findings` will fail the same "no such file or
directory" way `bench-scenario-guards` once did until `cmd/scenariobench`
landed (see this page's own wave-status note above). `TestFindingsLoopCorpusParses`
(`internal/scenariobench/findingsloop_test.go`) is the corpus's own proof
of life in the meantime: it loads and validates the corpus through the
same gate every other one passes, and checks every projection event's
anchor path exists and its digest matches the file at HEAD.

## The language lookups corpora (`lookups-ts.json` / `lookups-py.json`, SU-23)

ADR-041 decision 9 (`docs/plans/substrate.md` section F): a TypeScript and a
Python lookups corpus, on the calibration corpus's own pinned subjects
(`calibration/corpus.yaml` — `nest` at `v11.1.26`, `fastapi` at `0.119.1`),
mirroring `lookups-vs.json`'s task classes on a non-Go, non-fathom subject:
where is X defined, who calls Y (`blast_radius`), what does Z import
(`imports`), which tests reach W (`test_reach`). Neither subject carries a
CLAUDE.md/AGENTS.md at its pinned commit, so the Go corpus's
`obligations`/`staleness`/`claim_provenance`/`verify` classes — which need a
compiled governance layer to answer — are dropped entirely rather than
stubbed. Two arms only, `full` (fathom, wired) and `grep` (no MCP at all):
the Go/LSP-specific competitors `lookups-vs.json` carries (graphify, serena,
gopls) do not apply to a TypeScript or Python subject. 6 tasks each, 2
trials, 2 arms — 24 cells per corpus (`task bench-scenario-plan -- -corpus
testdata/scenariobench/lookups-ts.json` / `lookups-py.json` list them free).

Every task's file/line anchor was verified directly against a shallow clone
of the pinned commit (`git clone --depth 1 --branch <tag> <url>`, then `git
fetch --depth 1 origin <pinned sha> && git checkout <pinned sha>` when the
tag itself does not already resolve there — `nest`'s own pinned `sha` is an
annotated tag object that dereferences to a different commit hash, `fastapi`'s
is a plain commit) — never against a live HEAD; each task's own `derivation`
field carries the exact command and output. `TestLanguageLookupsCorporaParse`
(`internal/scenariobench/lookups_lang_test.go`, this row's own symbol) loads
both through `LoadScenarioCorpus` and additionally checks the two-arm shape,
the registered criterion, and the truth-at-head / prompt-names-anchor rule:
no task's prompt already names the file its own oracle cites as the answer
(`citationFilesInOracle`) — the "imports" class is the one deliberate
exception, since asking "what does file Z import" necessarily names Z, and
is registered as each corpus's own no-effect prediction for exactly that
reason (a plain Read answers it as well as any tool).

**Typed-edge tier, stated honestly.** Neither the SCIP import path (SU-9)
nor the per-file resolver (SU-11) is wired in this tree at authoring time
(`docs/languages.md`); both corpora's `full`-arm predictions are written
against what TypeScript/Python extraction already provides today — symbol
definitions plus HEURISTIC, syntax-level call edges
(`treesitter.CollectCalls`, `internal/extractor/jsextract.go` /
`pyextract.go`) — never presented as typed. Each corpus's own
`notes.typed_edges` names this and asks for the predictions to be
re-registered once SU-9/SU-11 land; `notes.kill_criteria` names the bar (no
lift over grep at n>=2 on the where-defined/blast_radius/test_reach classes)
and the index-gap-vs-true-negative distinction a zero-edge `find_callers`
result must be read against. Committed and **NOT RUN** this round.

## Layer-dependent tasks and post-wire drift (O45)

Four corpus keys exist because a task's prompt can assume more than an
ingested index — a **compiled** layer, or a tree that has drifted since
the wired arm's own daemon last looked at it:

- **`compile_layers`** (an arm field): a wired arm sets this to actually
  run `fathom compile -layer obligations` after its first ingest, once per
  (subject, commit, arm) store. Without it a wired arm still serves an
  EMPTY obligations layer — every file-scoped `obligations` call returns
  no rules — so a task whose prompt opens "Per fathom's compiled
  obligations layer …" measures nothing (`lookups-vs.json`'s `full` arm
  sets this for exactly that reason).
- **`requires_layer`** (a task field, `*bool`): declares that this task's
  prompt assumes a compiled layer already serves rules. Unset defaults to
  `true` for `category` `obligations`/`staleness`/`claim_provenance`/
  `verify`, `false` otherwise (`requiresCompiledLayer`, `subject.go`); set
  it explicitly to override either direction.
- **`probe_file`** (a task field): the file `PrepareSubject` asks the
  wired cell's own scratch daemon to serve `obligations` for, right after
  compiling — a `requires_layer` task whose probe comes back with zero
  rules loses its cell (`compiled layer serves no rules for <file>`)
  rather than being stored as a reading against an empty layer. Unset
  defaults to `DefaultProbeFile` (`internal/core/model.go` in this repo).
- **`post_wire_setup`** (a `subject` field, alongside `setup`): commands
  that run AFTER the wired arm's scratch daemon has ingested once — and,
  when `compile_layers` is set, after it has compiled — never before.
  `setup` runs before any of that, so drift placed there is already
  visible to the first ingest and to a compiled claim's own citation,
  defeating the very staleness/freshness comparison the task exists to
  make. `PrepareSubject` re-ingests after `post_wire_setup` runs (for a
  wired cell) so the symbol index reflects the drift; a compiled
  obligations layer is deliberately NOT recompiled, so a staleness task's
  claim stays pinned to its pre-drift citation. `lookups-vs.json`'s
  `FRESH-M7-01` is the worked example.

## Plan-first only on change-shaped tasks (O45)

`runSingle`'s first session appends `PlanFirstInstruction` (and, on a
wired arm, its plan-grammar addendum) only when the task is
change-shaped — `Gold.Files`, `Gold.TestPatch`, or `Oracle.Detectors`
non-empty (`isChangeShaped`, `sessions.go`). A pure lookup task
(`Oracle.AnswerAllOf` alone, `lookups-vs.json`'s shape) has nothing to
plan and nothing for a plan to hand off across sessions; asking one to
plan before answering only spent its turn budget on a checklist no oracle
ever reads. `planner-workers` mode is unaffected — its planner session
always needs to write the plan its workers pick up.

An answer-only task's oracle scores `SessionResult.FinalAnswer` — normally
the terminal `result` event's own text, but a session that stated its
answer and then ran out of turns (`error_max_turns`) carries an EMPTY
result text; `FinalAnswer` falls back to the last assistant text block in
that case, and `AnswerFrom` (`"result"` or `"last_text"`) records which,
so a cell scored via the fallback is never silently indistinguishable from
one that answered inside its budget (O45 item 3).

## How to add an arm (arms are data)

An `Arm` (`types.go`) is never code — it is what varies between
treatments: `wired` (runs `fathom init`, hooks, MCP, the stanza),
`instructions` (`"stanza"`, `"plain"`, or `"none"`), `notes_file`,
`allowed_tools`/`disallowed_tools`, `setup` (a competitor's build step),
and `surface` (what re-keys its cells: the fathom commit for a wired arm,
a pinned version string for a competitor, empty for a pure control). To
add one: pick a `surface` value, decide `wired`/`instructions`/`notes_file`
for it, add it under the corpus's `arms` map, and give every existing task
a `predictions` entry for the new arm name — a corpus is not valid until
every task has predicted the new arm's outcome. Nothing about the harness
changes; `RunSessions`/`ClaudeHarness` (wave 2, another file) read the arm
the same way regardless of what it contains.

## Competitor arm parity (O53)

A second reading of pilot vs-pilot-3 found the comparison itself
instruction-limited, not merely instrument-limited: gopls made 0 MCP calls
in 28 trials, graphify 1, serena 1, because every competitor arm's own
`instructions_text` named its tools for where-defined / who-calls /
implements and then added a disclaimer — "has no view onto documented
conventions, decision records, or repository-wide freshness" / "is grep's
job, not `<tool>`'s" — that routed the model away from the tool on 22 of 28
trials per arm before it ever read the prompt, while the `full` arm alone
carried a full routing stanza (`fathom init`'s own template,
`cmd/fathom/init.go`'s `fathomStanzaTemplate` + `openingMovesTable`) with no
disclaimer at all. Parity by construction fixes this: every competitor
arm's `instructions_text` now follows ONE template —

- an opening line naming the tool, by its real tool names
  (`docs/bench/competitors.md`), as the way to answer where a symbol is
  defined, who calls it, and what breaks if it changes — the ONLY
  capability a tree-sitter graph or an LSP genuinely has; it never claims
  "what governs" or "what changed since a prior citation", since fathom's
  own stanza routes those shapes to `obligations` and `staleness_report`
  specifically, not to its where/who-calls/what-breaks tools either
  (review fix-first, 2026-09-14: the first draft of this template DID
  claim both, an overclaim no competitor tool backs);
- an "Opening moves by task shape" list keyed to the corpus's OWN eight
  categories (`obligations`, `claim_provenance`, `concern_search`,
  `contradiction`, `freshness`, `staleness`, `verify`, `blast_radius` — not
  fathom's own four dev-task shapes from `openingMovesTable`, which this
  corpus never asks). `blast_radius` alone sits inside the claimed
  capability (who-calls/what-breaks), so its line names the competitor's
  own chained query with no hedge. Every other line — including
  `freshness`, since a tool's live-query mechanism is not the same thing
  as a designed "is this fresh" capability — names the competitor's
  closest query first, described only as an attempt ("`query_graph` for
  the symbol's neighbourhood, then grep the nearest CLAUDE.md / ADR.md"),
  with `grep` always the named fallback in the same line, never a claim
  that the query resolves the shape;
- no sentence that routes away from the tool, and no sentence that claims
  a capability the tool does not have. Where a tool is genuinely expected
  to lose a category, that expectation lives in the task's own
  `predictions` field, never in the stanza a session reads.

`TestLookupsVSCorpus_CompetitorStanzaParity`
(`internal/scenariobench/lookups_vs_test.go`) checks what the template
above actually promises, nothing more: no banned disclaimer phrase
anywhere in the text; no overclaim phrase ("what governs", "what changed
since", "governs its conventions", "decision records") in the intro
paragraph specifically (`stanzaIntroParagraph`, the text between the
header and the "Reach for these..." sentence); `grep` named somewhere in
every non-`blast_radius` category's own line; and a routing section
(everything after the `## <tool> ... navigation` header) within ±15% of
the other two competitors' and of fathom's own stanza's routing section
(the paragraph plus `openingMovesTable`, `cmd/fathom/init.go` — 1,052
bytes, hardcoded in the test with a comment to keep in sync, since
`internal/scenariobench` cannot import `cmd/fathom`, `package main`).

**Delivery assumes append (wave-n1).** `RenderArm`'s `"text"` mode is
changing from overwriting the subject's CLAUDE.md to APPENDING
`InstructionsText` to it, so a competitor arm's session reads the subject's
real CLAUDE.md (including fathom's own stanza, inert on an arm with no
fathom MCP entry) followed by the competitor's own block. Every
`instructions_text` in `lookups-vs.json` is written as a standalone,
`##`-headed appendable section for exactly this reason — never as a
full-file replacement.

**`exclude_paths` (wave-n1 adds enforcement).** Every task's `subject` now
carries `exclude_paths: ["testdata/contextbench/**",
"testdata/scenariobench/**", "docs/bench/**"]` — the home corpus's own test
data and this bench's own docs, so no arm's session can read its way to the
gold answer or a competitor's predicted score instead of answering the
prompt. The field is declared on `Subject` (`types.go`) today; the loader
rule that actually denies a session those paths is wave-n1's.

**Turn budget is part of the instrument.** The 20-turn cap ended six
competitor cells with the answer already in hand but no turns left to
state it (`FinalAnswer`'s `error_max_turns` fallback, "Plan-first only on
change-shaped tasks" above) — raised to 30 for every task in this corpus.
A turn cap is itself something the bench is measuring, not a fixed
constant: change it deliberately, and note the change here, never
silently.

## Instrument truth

A wired or MCP-carrying arm's cell is only a reading if the tools it was
measured as having actually ran. `RenderArm` writes the config a harness is
handed (`.claude/scenariobench-mcp.json`), defaulting `alwaysLoad: true` on
every entry that does not already set it — Claude Code 2.1.x defers MCP
tool schemas behind `ToolSearch` otherwise, so a competitor arm can connect
and still never be called. After each session, `RunSessions` checks the
transcript's own `system`/`init` event against what was actually rendered
(`ArmFiles.MCPServerNames`, including a wired arm's merged-in `fathom`
entry): a declared server missing from that event, or reporting anything
but `connected`, sets the session's `Error` to `instrument: mcp server
<name> <status|absent>` and the cell is **LOST** — never stored, exactly
like an every-session-errored cell (this is the defect 72/72 "full"
sessions hit: the harness ran the config it wrote for itself, never the
one `RenderArm` rendered).

A cell that clears that gate still has a second thing to prove: that the
tools it connected to were actually *used*. Every `CellResult`/ledger row's
`readings.tool_calls` sums `SessionResult.ToolCalls` by name, and
`readings.surface_calls` counts the ones namespaced to the arm's own
`surface` (`mcp__<surface-without-@version>__*`). An arm with a non-empty
`surface` whose `surface_calls` comes out 0 is flagged `surface_unused:
true` on the ledger row; `scenariobench score` prints one warning line to
stderr for a cell in that state, and `scenariobench ledger`'s rendered
table shows a "surface calls" column (`N (unused)` when flagged) so the
gap is visible without opening the raw cells.

**The daemon URL itself is instrument truth too (O45 items 6–8).** A wired
cell's scratch daemon ingests ONLY that worktree, at a path no other
daemon (least of all the operator's own fleet index at
`FATHOM_HTTP`'s default `localhost:7575`) recognizes — so every `fathom`
subprocess the session spawns (its own SessionStart/PostToolUse hooks, any
`fathom` CLI call) must have `FATHOM_HTTP` pointed at the cell's own
daemon, which `sessionEnv` now sets whenever `Worktree.DaemonURL` is set
(no separate service-identity variable is needed — resolution is by
INGESTED PATH, `cmd/fathom/serviceident.go`, and the scratch daemon
ingested exactly `wt.Path`). Even with that set, `runOneSession` checks a
THIRD thing beyond MCP-connected and SessionStart-fired: the SessionStart
hook_response's own rendered primer must actually name the indexed
service (`primerInstrumentError`) — a primer reading "this repo (…) is not
an indexed service", or naming no service at all, means the session never
really reached its own daemon, and the cell is **LOST** exactly like an
absent MCP server. Separately, `readings.hooks_fired` gains a synthetic
`obligations_injected` key: among the `PostToolUse` hook_response events
already counted there, how many carried the obligations marker in their
own rendered text — always `<= PostToolUse`, never a new event class.

## The clean room

Every session (`buildClaudeArgs`, `harness_claude.go`, RC-6, vision 28 §2
F4) runs under Claude Code 2.1.270's own `--restricted` mode plus
`--disable-slash-commands` plus `--include-hook-events` (the last one is
clean room v3 — see below): `--restricted` ignores the operator's user,
project and local settings files — so the second reading's own defect
("every session inherited the operator's config: ~90 slash commands,
plugins, and fathom's own SessionStart primer on the control arm") cannot
recur — while still keeping auth, still honoring `--settings <file>` and
`--mcp-config`/`--strict-mcp-config`, and confining the file tools to the
working directory; `--disable-slash-commands` removes skills on top.
`--settings` is passed the arm's own rendered `.claude/settings.json`
(`SessionRequest.SettingsPath`) — for a wired arm this is the file `fathom
init` (`PrepareSubject`'s `wireScratchDaemon`, BEFORE `RenderArm` runs)
already wrote with the real hook stack (SessionStart/PreToolUse/
PostToolUse/Stop, each naming the cell's own binary) — via `fathom init
--hooks-scope project`, so a hook already wired at the OPERATOR machine's
Claude Code user scope (invisible to `--restricted --settings <file>`,
which reads only that file) still lands in the settings file the cell's
session actually gets; `RenderArm` only asserts that file still carries a
`hooks.SessionStart` entry (`assertWiredHooksPresent`), it never writes or
replaces the hooks key itself. (A first version of `RenderArm` overwrote that file with a
one-hook stand-in naming a fathom subcommand that does not exist —
`obligations-hook` — so every wired cell ran with none of fathom's real
hooks; the instrument-truth gate now also fails a cell whose transcript
shows no `SessionStart` hook_response at all, and — clean room v3,
`postToolUseInstrumentError` — a cell whose transcript shows an edit
(`FilesEdited` non-empty) but no `PostToolUse` hook_response at all, so
neither defect can recur silently.) `--tools` is exactly what the arm allows
(`restrictedToolsList`): the arm's own `AllowedTools` with every `mcp__`
entry stripped (MCP access is already governed by
`--mcp-config`/`--strict-mcp-config`, not `--tools`) when non-empty, else
the default working set `Bash,Read,Edit,Write,MultiEdit,Grep,Glob,
NotebookEdit,TodoWrite` — `--restricted` otherwise removes every
command/code-running tool outright. `buildClaudeArgs` still adds
`--permission-mode default` whenever the arm has a non-empty
`AllowedTools`, so a restriction actually denies instead of Claude Code's
`auto` default approving outside it anyway (F3: 35 writes from
`allowed_tools: [Read, Grep, Bash]` arms in the second reading).
`cleanRoomEnv`'s only remaining job is `FATHOM_THREADS`, a per-cell
`threads.jsonl` under the cell's own `CellDir`, so two tasks sharing a
worktree basename never share a thread scope either (F5). Every session also runs under `--permission-mode default` with an explicit allow-list — the arm's own when it has one, else every tool the session was given plus `mcp__<server>` for each of its servers — and `--add-dir <worktree>`; without those, a session under `--restricted` had every shell command answered "requires approval" and could not create a directory in its own worktree (reuse-pilot-2, 2026-09-13). Raw transcripts live under `<raw-dir>/<run-id>/` so a rerun never reads another run's sessions.

### The raw attempt dir (O76/O77)

Each cell's own raw artifacts land under `<raw-dir>/<run-id>/<task>-<arm>-t<n>/attempt-<k>/` (`CellRawBase` + `allocateRawAttemptDir`; a retried or resumed cell key gets a fresh `attempt-<k+1>`, never overwriting an earlier one). A `ClaudeHarness` session writes its own `session-<n>-<role>.stream.jsonl` (+ an occasional `.stderr` sibling) and `session-<n>-mcp.json` here; `-harness fake` writes neither (`FakeHarness` reads straight from its own committed `StreamsDir` and never touches this directory at all) — the attempt dir itself still gets created, empty, since `allocateRawAttemptDir` runs before either harness does.

`runOneCell` (`cmd/scenariobench/main.go`) additionally persists, unconditionally and BEFORE the leak/escape/LOST decision, exactly what `RematerializeCellWorktree` (`internal/scenariobench/rematerialize.go`) needs to rebuild this cell's worktree from raw alone, once `workspace/artifacts/scenariobench/worktrees/…` has been reset by a later cell sharing the same `(subject, base_commit, arm)` or removed by an operator reclaiming disk (guards-pilot-7 lost 31 cells exactly this way — O76):

- `agent.diff` (`scenariobench.CellRawAgentDiffPath`) — a byte-for-byte copy of `captureAgentDiff`'s own output (`git add -N .` then `git diff <LocalBase>`, instrument-owned files/regions already stripped).
- `untracked-snapshot/` (`scenariobench.CellRawUntrackedSnapshotDir`), plus `untracked-snapshot.json` — a byte-identical copy of every binary file the diff names as created or modified (a plain `git diff` never embeds binary patch data), each capped at 10 MiB; what was captured and what was skipped (and why, and its size) is recorded in the manifest, never left ambiguous with "nothing to capture".
- `cell-artifacts.json` — the task's base commit, `wt.LocalBase`, and the instrument-owned paths `captureAgentDiff` excluded.

Best-effort: a write failure here is reported on stderr and never fails the cell — a raw copy is a convenience for later re-scoring, never a precondition for the readings already computed from the live worktree.

**First design, replaced.** RC-6 first tried a scratch `CLAUDE_CONFIG_DIR`
per run, with a stripped `~/.claude.json`/`~/.claude/settings.json` copied
into it. Its own smoke test found that broke authentication ("Not logged
in · Please run /login", `total_cost_usd: 0`) under every variant tried —
whatever backs this CLI's macOS-keychain login lookup did not resolve
under a scratch `CLAUDE_CONFIG_DIR` in this environment, contrary to the
row's working assumption. `--restricted` needs no config-dir copying at
all: it isolates a session from the operator's config while running in
the operator's own, already-authenticated `CLAUDE_CONFIG_DIR`.

**Smoke test (RC-6, 2026-09-13, real spend $0.0037):** `claude -p "reply
with the single word ok" --max-turns 1 --model claude-haiku-4-5-20251001
--output-format stream-json --verbose --restricted --disable-slash-commands
--settings <temp settings.json with a SessionStart hook echoing a marker>
--tools Read` proved all three properties together in one run:

- **Auth works.** `is_error: false`, `result: "ok"`, `total_cost_usd:
  0.00368` — a real, billed turn.
- **The `--settings` hook fires.** The stream carries a `hook_response`
  event for `SessionStart:startup` with `"outcome":"success"` and the
  marker (`RC6-HOOK-FIRED-MARKER`) in its own `stdout`/`output`; the
  hook's side-channel marker file was also written.
- **No operator config reaches the session.** The `init` event shows
  `"tools":["Read"]` (exactly `--tools`' value), `"mcp_servers":[]`,
  `"slash_commands":[]`, `"plugins":[]`; the operator's own stanza text
  ("fathom code-intelligence is live") appears zero times in the stream.

**Clean room v3 (2026-09-13 experiment matrix): the RC-6 smoke test only
ever proved `SessionStart`.** Two real dry cells found `PostToolUse` never
showed a `hook_response` event under the RC-6 argv, on any arm, with or
without `--restricted` — the exact shape of `dry-r4-change`'s defect (a
`Write` with only `SessionStart` events, the obligations rule never
served). A one-turn Haiku matrix (`claude -p`, `--max-turns 4`,
`--model claude-haiku-4-5-20251001`, a side-channel marker file
independent of the transcript so a hook's *process* running could be told
apart from Claude Code *reporting* that it ran) isolated the cause:

| cell | settings source | `--restricted` | matcher | event | hook **process** ran (marker file) | hook **event** in the stream |
|---|---|---|---|---|---|---|
| a | `--settings <file>`, PostToolUse | no | present | PostToolUse | yes | **no** |
| b | cwd project `.claude/settings.json` | no | present | PostToolUse | yes | **no** |
| c | cwd project `.claude/settings.json`, **no `--settings` flag** | yes | present | PostToolUse | **no** | no |
| d | `--settings <file>` | no | omitted | PostToolUse | yes | **no** |
| e | `--settings <file>` | no | present | PreToolUse | yes | **no** |
| f | `--settings '<inline JSON>'` | yes (+ explicit `--settings`) | present | PostToolUse | yes | **no** |

Every cell's hook **ran** (its marker file landed, timestamped inside the
session's own window) — including the OPERATOR's real, already-registered
`~/.claude/settings.json` `PostToolUse`/`Stop` hooks in cells a/b/d/e,
which also wrote their own real side effects (`workspace/logs/sessions
.jsonl`, `workspace/artifacts/retros/*.json`) — yet **zero** cells showed a
`hook_started`/`hook_response` system event for anything but
`SessionStart`, in either transcript. Cell c is the one exception with a
different cause: `--restricted` with no explicit `--settings` flag
ignores the cwd's own project-scope `settings.json` entirely (no marker
at all) — expected, and exactly why `SessionRequest.SettingsPath` is
always passed via `--settings` today.

Re-running cell a with `claude --help`'s `--include-hook-events`
("Include all hook lifecycle events in the output stream") flipped every
`no` above to `yes`: `PreToolUse:Write`, `PostToolUse:Write` and `Stop`
`hook_started`/`hook_response` pairs all appeared, the `PostToolUse`
one carrying the hook's own `hookSpecificOutput.additionalContext`
verbatim. Re-running cell c (`--restricted`, explicit `--settings
<cwd>/.claude/settings.json`, `--include-hook-events`) showed exactly one
hook — the cell's own `PostToolUse` — with zero operator leakage,
confirming `--restricted` was never the gap: it excludes the operator's
real hooks/plugins/slash-commands in every configuration tried, and the
cell's own project-scope `settings.json` reaches a `--restricted` session
the same way it always required — named explicitly via `--settings`.

**The fix is one flag, not a redesign.** `buildClaudeArgs` now always
passes `--include-hook-events` alongside `--restricted` and
`--disable-slash-commands`; nothing else about the clean room's shape
changed. `runOneSession` gained a parallel gate to `hooksInstrumentError`
— `postToolUseInstrumentError` — that fails a wired cell whose transcript
shows an edit but no `PostToolUse` hook_response, the same way an absent
`SessionStart` hook_response already did.

**`FATHOM_CHANNELS=off` was checked and is not needed for isolation.**
`bin/fathom session-hook` and `bin/fathom edit-hook`, run directly with
realistic stdin, both go completely silent (exit 0, zero stdout) under
`FATHOM_CHANNELS=off` and render their normal output without it — the
kill switch works exactly as documented (`cmd/fathom/channels.go`). But
since `--restricted` already excludes the operator's hooks structurally
(proven above, cell c/f), the clean room needs no additional env-based
silencing of the operator's fathom hooks; `FATHOM_CHANNELS` stays a
manual operator escape hatch, not part of the harness's own argv or env.

## Freshness (M7)

M7 ("the answer reflects HEAD", `docs/vision/27-strengths-bench.md` §1/§6)
splits into a free half and a paid one. Free: `internal/indexbench.FreshnessProbe`
(VB-17, `task bench-freshness-probe`, part of `bench-l0-gate`) runs
continuously, on every commit — five scripted mechanical edits (rename, add
a parameter, delete, move, change a return literal) against a mutable copy
of the fixture fleet, each paired with a question only the current on-disk
state can answer, scored before and after the hash-gated re-ingest with no
model in the loop. Paid: `FRESH-M7-01` in `testdata/scenariobench/lookups-vs.json`
runs the same shape of question through a live agent session across fathom,
grep, and the competitors named in `docs/bench/competitors.md` — currently
scoring graphify's default one-shot build, which the free probe's own
staleness gap already predicts loses; a graphify arm run in its own
`--watch` mode (docs/vision/27-strengths-bench.md §3 wave D) is the paid
reading still to add, once per release, before this row can credit
graphify with the live-refresh mode its own docs recommend.

`FRESH-M7-01` was redesigned (SB-51, `docs/plans/verbs.md` VB-17/18) after
every arm answered the ORIGINAL version — "what integer does the function
FreshnessMarkerValue in internal/store/freshnessmarker.go return?" —
with a plain `Read` of the file the prompt named directly: no tool of any
kind is needed to answer a literal read once both the file and the
function are already named, so the freshness discriminator never fired.
The task now asks a call-graph fact instead, the same question shape as
`FreshnessProbe`'s own `add_param` edit ("how many callers does
applyEntry have now") above: `internal/metrics.IsPlumbingTool` already has
THREE committed callers at `base_commit` — the production function
`AggregateTraffic` (`internal/metrics/traffic.go`) and two tests in
`internal/metrics/discovery_test.go`
(`TestIsDiscoveryIndexTool_PlumbingTool_False`,
`TestIsDiscoveryIndexTool_UnknownTool_False`), verified directly against
that commit (`git archive 1b8765d | tar -x`, then `grep -rn
'IsPlumbingTool(' internal/metrics` — the task's own `derivation` field
carries the full command and output, including the two string-literal
false positives inside `Errorf`/`Error` messages that are not real calls).
Review fix-first (2026-09-14) caught an earlier draft asserting "exactly
one committed caller", which a live query would have contradicted outright
— the prompt now states the true three-caller baseline and asks for the
caller NOT among them. `Subject.PostWireSetup` adds a FOURTH call site in
a brand-new file the prompt never names
(`internal/metrics/toolkind_freshprobe.go`), and the task asks which
function now calls `IsPlumbingTool` that is not one of the three named
callers. A graphify graph built during this arm's own setup — before the
edit lands — still reports only the three pre-existing edges; a live gopls
query (serena, gopls) or a re-ingested fathom index sees all four; grep
across the tree for `IsPlumbingTool(` finds all four real call sites
regardless of any artifact's staleness, since the new caller is textually
present in the tree the moment it lands.

## Dropped tasks

A task can carry `"disposition": "dropped"` with a `"reason"` (RC-15,
vision 28 §2 F10): an oracle that cannot be won from the task text at all.
`SB-cli__cli-10009` (`testdata/scenariobench/public-go.json`) is the first:
its `fail_to_pass` names `internal/safepaths` tests, but that package does
not exist at `base_commit` — the gold patch's own `test_patch` is what
*introduces* `internal/safepaths/absolute_test.go`. No agent can satisfy an
oracle asserting a package it was never told to create. `planCells`
(`cmd/scenariobench/main.go`, shared by `plan`/`run`/`ledger`) skips a
dropped task entirely — it never becomes a cell, so it is never run,
never LOST, never scored, and never appears in the ledger. `scenariobench
score` operates on one already-stored `CellResult` with no corpus in hand
at all, so a dropped task simply never has a cell for it to rescore.

## Running arms concurrently

`scenariobench run -parallel-arms` (RC-16, default off) runs one task's
arms concurrently instead of one at a time: `PrepareSubject` keys a
worktree on `(subject, base_commit, arm)` and a wired arm's own scratch
daemon binds an independently allocated port, so a task's treatment and
control cells already share no worktree, no daemon, and no port —
`scenariobench.RunArmsConcurrently` (`internal/scenariobench/concurrent.go`)
is safe by construction, not by locking. Outcomes are always processed in
the cells' original `(task, arm, trial)` order, never completion order, so
the printed log and the stored/borrowed/lost counts are identical whether
or not `-parallel-arms` is set — only wall-clock time changes.

## Operating lessons (pilot rounds N16–N18)

Short rules an operator running a paid pilot needs, each paid for by a
real lost or delayed run this round:

- **Never delete a pilot's worktrees or subjects before the run is filed
  AND read.** guards-pilot-7 lost 31 cells exactly this way (O76) —
  reclaiming disk (or a later cell reusing the same
  `(subject, base_commit, arm)` key) is safe only after the ledger row is
  published and a human has read the transcripts, never merely after the
  driver process exits.
- **Every cell's diff persists under its raw attempt dir now, unconditionally,
  before the LOST decision (O77).** `agent.diff`, the untracked-file
  snapshot, and `cell-artifacts.json` land under
  `<raw-dir>/<run-id>/<task>-<arm>-t<n>/attempt-<k>/` regardless of
  whether the cell scores, is voided, or is lost — so re-scoring never
  needs a live worktree that a later cell or an operator's cleanup may
  already have reset or removed (see "The raw attempt dir" above).
- **The push gate uses its own Go cache.** `lefthook pre-push` runs `task
  validate` (a full `go vet`/`go test` pass) in whatever checkout you push
  from; a pilot round's own builds (scratch daemon binaries, subject
  exports) are also compiling concurrently on the same machine. Point the
  push gate's `GOCACHE`/`GOMODCACHE`/`GOTMPDIR` at a location the pilot
  round's own env does not also write to — sharing one build cache across
  concurrent, independent Go processes is exactly the kind of contention
  this round's own environment isolation (subject git, `GIT_CONFIG_GLOBAL`
  below) already treats as a hazard elsewhere.
- **Subject git runs under a derived `GIT_CONFIG_GLOBAL`, never a bare
  `/dev/null`.** `gitConfigIsolation` (`internal/scenariobench/gitenv.go`)
  isolates subject preparation from the operator's own global/system git
  config (autocrlf, fsck policy, aliases, signing) — but `/dev/null` alone
  also drops the keys a clone of a PRIVATE remote needs to authenticate at
  all (`credential.*`, `url.*.insteadOf`/`pushInsteadOf`, `http.proxy`/
  `sslcainfo`, `core.sshCommand` — `allowedOperatorConfigKey`). vs-pilot-6
  lost all 140 cells at subject preparation to the bare-`/dev/null` form;
  `ensureClone`/`ensureCommit` now point `GIT_CONFIG_GLOBAL` at a derived
  file carrying only those remote-reaching keys.
- **`-rescore-lost` re-scores a cell from the raw streams already on disk,
  at zero spend.** Reach for it before re-running a whole corpus after
  fixing a scoring or instrument defect — it never launches a session and
  never claims a reading that genuinely needs a live worktree (a task
  `NeedsLiveWorktree` is reported "not rescorable" rather than guessed).
- **`-strict-borrow=false` is a recorded per-run decision, never a
  default.** The default (`true`) refuses (exit 2) to file a row when a
  stored cell's own `run_id` differs from `-run-id`, rather than silently
  skipping it; passing `=false` to restore the old silent-skip behavior is
  something to write down in the run's own notes (a `cellsDir` deliberately
  shared across runs), not a flag to reach for out of habit.
- **A `tail -F` monitor on a running pilot's log never terminates on its
  own.** Use a waiter that exits on the driver's own completion (its exit
  code, or a marker line it writes at the end) instead of an open-ended
  follow a human has to remember to kill.

## How to publish a row

After a paid run, its per-cell results append one row to
`docs/bench/scenarios-ledger.jsonl` via `AppendScenarioLedger` (wave 2),
which `RenderScenarioLedger` turns into the marker-fenced table in
`docs/bench/README.md` — the same publish discipline as the existing
context-bench ledger (`docs/bench/README.md` "What a run is"): **a loss is
published as-is**, predictions stand beside verdicts whichever way they
fall, and raw transcripts are published as a release artifact
(`workspace/artifacts/context-bench/.../raw/`), never committed. Until
`cmd/scenariobench`'s `ledger` subcommand lands, a run's raw cells under
`workspace/artifacts/` are the evidence; do not hand-edit
`scenarios-ledger.jsonl`.

## What a row carries (the row-aggregation defect, 2026-09-13)

The first rows filed with tools present (`reuse-pilot-2`, `guards-pilot-4`)
showed the gap directly: a `ScenarioLedgerRow`'s `readings` carried only
`payload_bytes`/`surface_calls`/`tool_calls`/`total_cost_usd`/
`total_tokens`/`total_turns` — every other per-cell reading
(`hooks_fired`, `rule_served_before_edit`, `violation`, `overlap`,
`facts_reused`, `resumption`, `plan_coverage`, `false_done`, `drift`,
`compile_cost_usd`, `gold_source_tool` per trial) was silently dropped by
the row's own hand-rolled field summation. `AggregateRowReadings`
(`internal/scenariobench/cell.go`, called from `cmdLedger`) is now the
single place a row's `readings` is built from its group of cells: every
count is summed (`hooks_fired` per event, `tool_calls`, `facts_reused`),
every boolean-per-trial reading becomes a count of trials true
(`violation_trials`, `rule_served_before_edit_trials`), every ratio-per-
trial reading is averaged with its own n (`overlap_ratio_mean`/`_n`,
`resumption_tokens_to_first_edit_mean`/`_n`), and the full structured
reading for every trial (`gold_source_tool`, `plan_coverage`,
`false_done`, `drift`, `resumption`, `overlap`) is preserved verbatim
under the row's own `per_trial` array — nothing is summarized away.
`gold_source_rate` stays the one summary that was already correct. Every
new field is additive with `omitempty`; a row filed before this change
loads exactly as it did, with the new fields simply absent.
`RenderScenarioLedger`'s table fills these in only where the corpus
category makes them meaningful — a governance (guards) row shows hooks
fired/rule served/violation, a reuse row shows overlap ratio/facts
reused, a public row shows resumption/false done/plan coverage — every
other row's cell for that column renders `–`.

`scenariobench ledger -trials`, left unset, no longer defaults to a bare
1: it falls back to the corpus's own registered `Criterion.MinTrials`.
Either way that number is only a floor — if a cell key has MORE trials
actually stored on disk than the resolved floor asked for
(`StoredScenarioTrials` lists every `t<N>.json` under the cell's own
directory), every one of them is still counted, never capped at the
floor the way a run's real second trial was once silently dropped
because `-trials` defaulted to 1 while 2 trials were actually run. The
summary line prints each filed row's own trial count
(`<task>/<arm>:<trials>`) so this is visible without opening the ledger.

`scenariobench score -in <cell> -corpus <corpus.json>` recomputes
`gold_source_tool`/`gold_source_evidence` against the task's own gold —
`AggregateReadings` (the readings reset every `score` invocation performs)
does not touch either field, so recomputing needs the corpus in hand.
Without `-corpus`, `score` leaves the stored value exactly as it was and
says `gold_source_tool: not recomputed (no -corpus)` on stderr — it never
overwrites a measured value with `not_measured` (the defect a bare
re-score used to cause, visible as `gold_source_rate: {not_measured: 1}`
on a row whose cells really had been measured).

## Gold source, not just pass/fail (VB-4, ADR-035 decision 5)

A cell passes the task; a **tool** passes only when it was the gold
source. Every cell's `Readings.gold_source_tool` (`GoldSourceTool`,
goldsource.go) names the tool whose OWN result FIRST held the task's gold
signal — the `answer_all_of` strings for a lookup task, a `Gold.Files`
path for a change task, the rule's own source path for a guards task —
`"none"` when no tool result ever carried it (the agent knew or guessed).
A `ScenarioLedgerRow` rolls this up as `gold_source_rate` (the fraction of
that row's own cells attributable to each tool, `not_measured` naming
cells scored before this reading existed) and `gold_source_passes` (the
same reading narrowed to cells that PASSED — what a tool actually
*earned*); `scenariobench ledger`'s render adds a "Gold source by question
shape" table (`GoldSourceRateByCategory`, one row per task category, every
tool column present down to 0.00) and the verdict block's own "gold
source: `<tool>` earned N of M passes" line for the treatment arm — so
"obligations served six displaced rules and the arm passed the task by
reading the file grep's way" (E1/O23) is a number, not a narration.

## Trial escalation (RC-14)

Every task screens at n=2 first — a third trial is spent only where the
first two left the verdict genuinely undecided. After a run lands its rows
in the ledger:

```sh
scenariobench plan -corpus <corpus.json> -escalate \
  -ledger docs/bench/scenarios-ledger.jsonl [-run-id <id>] [-margin 0.15]
```

reads the ledger (optionally narrowed to one `-run-id`) and, for every task
whose treatment arm ran EXACTLY n=2, computes `internal/scenariobench.
EscalationPlan`'s two deltas against the corpus's own registered
`criterion`: the treatment/control **success-rate delta** against
`require_success_at_least_equal`'s zero line, and the **tokens-per-success
ratio** against `max_tokens_per_success_ratio`. A task is printed — and only
printed, never run — when either delta sits within `-margin` (default 0.10)
of the line that decides pass/fail: close enough that a third trial's
result could still flip the verdict. A task whose n=2 delta is already
decisive, or that already ran at n≥3, is not a candidate. No cell store is
touched and nothing spends: `-escalate` is a read of the ledger, the same
free-until-you-choose-to-spend shape as `plan`'s normal cell listing.

## External corpus (LCA bug localization) (VB-25, vision 29 Wave G)

A corpus fathom does not own: [Long Code Arena's bug-localization
track](https://huggingface.co/datasets/JetBrains-Research/lca-bug-localization)
(JetBrains Research, **Apache-2.0**, verified 2026-09-13) — Python, Java
and Kotlin issues from real GitHub repos, each carrying the upstream
developer's own changed-file set. This tests the `localize` verb (VB-26)
on a corpus this repo never wrote a task for by hand, "the same model with
and without the agent surface" (vision 29 §3 row G).

**Fetching it** (network, one-time, cached outside the repo — nothing here
downloads it automatically):

```sh
mkdir -p workspace/bench/sources
# Verified 2026-09-13: one parquet file per (language subset, split) —
# java/, kt/, py/, each with dev-/test-/train-00000-of-00001.parquet. Pick
# a language and split (py/test is the smallest, 50 hand-verified rows).
curl -sL -o workspace/bench/sources/lca-py-test.parquet \
  "https://huggingface.co/datasets/JetBrains-Research/lca-bug-localization/resolve/main/py/test-00000-of-00001.parquet"
# This loader reads JSONL, not parquet — convert the file you fetched
# before pointing -dataset at it, e.g.:
#   duckdb -c "COPY (SELECT * FROM read_parquet('workspace/bench/sources/lca-py-test.parquet')) TO 'workspace/bench/sources/lca-py-test.jsonl' (FORMAT JSON)"
# (a short pandas `read_parquet` + `to_json(orient="records", lines=True)`
# script works just as well). Re-verify the license tag on the dataset
# card before relying on the Apache-2.0 recorded above — a HF dataset's
# license tag can change after a repo is forked from it.
```

**Field mapping** — `internal/scenariobench/lca.go`'s `LCARecord` decodes
these upstream columns verbatim (the dataset card lists many more this
bench has no use for: token/line counts, repo stars, …):

| Upstream column | Task field |
|---|---|
| `repo_owner`, `repo_name` | `Subject.RepoURL` (`https://github.com/<owner>/<name>`) |
| `base_sha` | `Subject.BaseCommit` |
| `repo_language` | `Subject.Language` (informational only) |
| `issue_title`, `issue_body` | `Task.Prompt` (`issue_body` may be empty) |
| `changed_files` | `Task.Gold.Files` — the gold answer set, never an edit target |
| `text_id` | `Task.ID` (falls back to `<owner>__<name>-<line index>` when empty) |

**Running it** — `-corpus lca -dataset <path.jsonl>` on `plan`/`run`/`ledger`
builds the corpus in memory (`scenariobench.LoadLCARecords` +
`BuildLCACorpus`) instead of reading a committed corpus file, hashed over
its own canonical JSON bytes (`CanonicalCorpusSHA256`) so the cell key
stays stable across identical dataset content:

```sh
go run ./cmd/scenariobench plan -corpus lca -dataset workspace/bench/sources/lca-bug-localization.jsonl \
  -harness-version fake   # lists cells, no spend

go run ./cmd/scenariobench run -corpus lca -dataset workspace/bench/sources/lca-bug-localization.jsonl \
  -root . -harness fake -fake-streams-dir testdata/scenariobench/streams \
  -fake-streams '{"<a text_id>/fathom":"lca-fathom-hit.jsonl","<a text_id>/control":"lca-control-miss.jsonl"}' \
  -run-id dry -harness-version fake -only <a text_id>   # the dry cell — no model call
```

Every task carries `Oracle.Localization` (never `fail_to_pass`/
`pass_to_pass`): change-shaped=false, no plan-first (`isChangeShaped`
excludes it explicitly), because the diagnosis IS the answer — the prompt
says "do not edit anything". `internal/scenariobench.RunExternalCorpus` is
the same run+score+ledger pipeline as a single package-level call
(`PrepareSubject` → `RunSessions` → `RunOracle` → `StoreScenarioCell` →
`AggregateRowReadings` → `AppendScenarioLedger`), under the identical
instrument-truth gate (`CellLost`) every other cell runs under — server
connected, hooks fired, surface calls, never trusted merely because an arm
claims it is wired; a voided cell is never scored, only counted as lost.

**The files oracle** (`localization.go`) scores the last session's own
final answer, not a bare list: `ParseAnswerFiles` extracts a candidate
file set in first-mention order (a path-shaped token — contains `/`, or a
bare filename with a recognized extension — deduplicated on the exact
string; deliberately conservative, since no regex parses free-form
prose perfectly), then `ScoreLocalization` compares it against
`Gold.Files` (`sameFile`'s own path-suffix rule): `hit_at_1` (the
top-ranked answered file is gold — this decides `Oracle.Passed`, the
standard file-level accuracy@1 bar the literature reports, R10), `hit_at_5`
(any of the top 5), and set-based `recall`/`precision` over the full
answered set. Every field rides the cell's `Readings.localization` and the
ledger row's own `readings.localization_hit_at_1` /
`readings.localization_hit_at_5` / `readings.file_recall` (row-level means
across trials, `AggregateRowReadings`) — an empty answer scores a
legitimate zero, never `not_measurable`; only a task carrying no gold
files at all (a corpus bug `BuildLCACorpus` already refuses at load time)
is.

**The pre-registered kill criterion (VB-27, M12 and M16):** paired
statistics at n ≥ 3 over ≥ 30 tasks; a confidence interval that includes
zero on the treatment/control lift publishes the null and closes BC-15 on
it (vision 29 §5) — `Criterion.Evaluate`'s own pass/fail (registered here
as `lca-bug-localization-m12-m16`, `MinTrials: 3`) is the floor that makes
a row countable at all, never a substitute for that confidence interval;
VB-27 is deliberately `skip_test: true` (operator spend) and reads the
published ledger rows this section's own pipeline writes.

**Turn budget (`DefaultLCAMaxTurns`, `internal/scenariobench/lca.go`):**
raised 15 → 30 after lca-run-1 (2026-09-17,
`docs/bench/readings/lca-run-1-reading.md` §5): the wired arm hit
`error_max_turns` on three of its six "worse" tasks (`dokku`, `tock`,
`pip`) AFTER already locating the gold file, on hook chatter (~19-27
round-trips) and a 25-tool schema control's 3-tool surface does not
carry, and `edgedb/edgedb` hit the same cap in all six trials before
EITHER arm produced a scoreable answer — a fixed 15-turn cap was
punishing fathom's overhead and swallowing whole tasks as null results,
not measuring localization. `Sessions.MaxTurns` is one field per `Task`
(no per-arm override), so the raise applies to both arms identically;
`TestLCABuglocCorpus_LoadsAndBuilds` pins it.
