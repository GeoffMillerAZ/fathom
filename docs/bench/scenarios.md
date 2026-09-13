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
