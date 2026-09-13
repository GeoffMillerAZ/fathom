# The public localization corpus — runbook

`testdata/contextbench/tasks-public-go.json` is the corpus a stranger can
re-run (docs/plans/bench-public.md PB-14..PB-19, ADR-031,
`docs/vision/26-proof-credibility.md` §2.3). Nothing in it is authored:
every ground truth is derived by `contextbenchscore import` from two pinned
third-party datasets, with no LLM, and reproduces bit-for-bit
(`corpus_sha256`). This page is the whole procedure — pin, derive, snapshot,
run, score, publish — plus the cost and the criterion registered before the
first run.

The corpus file does not exist until an operator runs steps 1–3. The code
and its fixtures (`internal/contextbench/publicloc/testdata/`) are
committed; real dataset rows are not, until the derivation below produces
them from pins recorded in the file.

## What is derived, from what

| Class | Truth | Tool |
|---|---|---|
| `file_localization` | non-test `.go` files the gold patch modifies or deletes at `base_commit` (added files are not gold: nothing at `base_commit` can be found) | `publicloc.GoldFiles` |
| `function_localization` | functions whose span at `base_commit` contains a deleted line, or both neighbours of an inserted run (an insertion between two functions is nobody's function) | `publicloc.EnclosingFuncsForHunks` — `go/parser` over the checkout |
| `find_callers` | non-test direct static callers of a gold function, self-calls excluded, 1–8 callers | `cmd/loctruth` — `go/types` over the checkout, never fathom's index |

Sources, each recorded beside its pin in the corpus's `sources` block:

| Source | Licence | What it contributes | Adapter schema |
|---|---|---|---|
| [Multi-SWE-bench](https://huggingface.co/datasets/ByteDance-Seed/Multi-SWE-bench) (Go JSONL: `cli/cli`, `grpc-go`, `go-zero`) | CC0-1.0 | developer-authored gold patches (`fix_patch`), issue text | `org, repo, number, title, body, base.sha, resolved_issues[].{title,body}, fix_patch` |
| [SWE-Explore-Bench](https://huggingface.co/datasets/SWE-Explore-Bench/SWE-Explore-Bench) (Go subset) | CC-BY-NC-ND-4.0 (verified 2026-09-13; the pin said MIT until then — ND: nothing committed derives from it) | line-level regions an outsider built, plus the SWE-bench patch where present | `instance_id, repo, base_commit, problem_statement, language, patch, regions \| ground_truth` |

The adapters (`internal/contextbench/publicloc/sources.go`) name the fields
they read. A row missing one fails the import loudly with its line number
— an upstream schema change is a visible failure at step 3, never a
quietly smaller corpus (ADR-027). Confirm the SWE-Explore field names
against the pinned snapshot before the first real import; the fixture rows
document the two region shapes the adapter accepts.

Pre-registered selection (`publicloc.RegisteredRule`, committed before
any run): the gold patch modifies or deletes **1–5** non-test Go source
files present at `base_commit`; duplicates across sources by
(repo, `base_commit`, gold files) keep the source that sorts first;
candidates sort by `instance_id`; the first **35** are the corpus. An `-n`
outside 30–40 is refused unless `-unregistered-n` is passed, and the corpus
then carries `pre_registered: false`.

## 1. Pin

Fetch the two snapshots once, by hand, and record what you fetched. Never
from a test, never from `task validate` — the import reads local files only.

```sh
mkdir -p workspace/bench/public/{snapshots,checkouts,callers}
# Multi-SWE-bench: the Go JSONL files from the dataset's release (CC0-1.0).
# SWE-Explore-Bench: the Go rows as JSONL (CC-BY-NC-ND-4.0 — no derived corpus may be committed from it).
# Record the upstream revision (dataset commit / release tag) you took.
sha256sum workspace/bench/public/snapshots/*.jsonl
```

Pinning means: the file's sha256 and the upstream revision go into the
corpus (`sources[]`), and the snapshot files are kept under
`workspace/bench/public/snapshots/` (gitignored — the corpus commits
metadata, ids, paths and symbols, never the datasets themselves).

## 2. Select and check out

```sh
GOWORK=off go build -o workspace/tmp/contextbenchscore ./cmd/contextbenchscore
GOWORK=off go build -o workspace/tmp/loctruth ./cmd/loctruth

workspace/tmp/contextbenchscore import -select-only \
  -multi-swe   workspace/bench/public/snapshots/multi-swe-go.jsonl \
  -swe-explore workspace/bench/public/snapshots/swe-explore-go.jsonl \
  -out workspace/bench/public/selected.json
```

`selected.json` lists each selected instance with `repo_url`,
`base_commit`, the `checkout` directory name and the `service` name the
runner will use. Check each one out at its base commit — the tree
**before** the fix, which is the only tree the truth is about:

```sh
python3 - <<'EOF'
import json, subprocess, os
sel = json.load(open("workspace/bench/public/selected.json"))
for i in sel["instances"]:
    d = os.path.join("workspace/bench/public/checkouts", i["checkout"])
    if not os.path.isdir(d):
        subprocess.run(["git", "clone", "--quiet", i["repo_url"], d], check=True)
    subprocess.run(["git", "-C", d, "checkout", "--quiet", i["base_commit"]], check=True)
EOF
```

Then the callers truth, one file per checkout:

```sh
for d in workspace/bench/public/checkouts/*/; do
  id=$(basename "$d")
  (cd "$d" && go mod download) # go/types needs the module's dependencies resolvable
  workspace/tmp/loctruth -root "$d" -out "workspace/bench/public/callers/$id.json"
done
```

`loctruth` reports how many packages carried type errors; a checkout whose
dependencies could not be resolved yields fewer edges, which the import
records as omissions per instance rather than as a shorter truth.

## 3. Derive

```sh
workspace/tmp/contextbenchscore import \
  -multi-swe   workspace/bench/public/snapshots/multi-swe-go.jsonl   -multi-swe-revision   <rev> \
  -swe-explore workspace/bench/public/snapshots/swe-explore-go.jsonl -swe-explore-revision <rev> \
  -checkouts   workspace/bench/public/checkouts \
  -callers-dir workspace/bench/public/callers \
  -out testdata/contextbench/tasks-public-go.json
```

The command prints the selection report, each pin with its licence and
sha256, the count of recorded omissions, and `corpus_sha256`. Run it twice:
the bytes must be identical (`TestDerivationIsDeterministic` proves the
same for shuffled inputs). Commit the corpus file. A hand edit after this
point fails `publicloc.LoadCorpus` with a hash mismatch.

Optional, and only before the first run: `-classes
file_localization,function_localization` pre-registers a class subset (the
choice is recorded in `selection.classes`; a subset chosen after seeing
results is not a subset, it is a selection).

## 4. Snapshot

Each task carries `subject{repo_url, base_commit, service}`. The runner
must ingest **that** tree into a scratch store under `workspace/`, never
the fleet store and never the post-fix tree (index leakage is the
bench-specific hazard: with the fix in the index, the answer is in the
index). The runner's subject flags are PB-5's
(`--subject-root`, `--service`, `--fathom-url`, `--prompt-subject`, opt-in
`--scratch-daemon`); this page does not restate them — read
`scripts/context-bench.sh`'s header for their current contract. Per
subject:

```sh
# one subject (all tasks sharing a service), scratch store, three arms, three trials
scripts/context-bench.sh --tasks workspace/bench/public/by-subject/<service>.json \
  --subject-root workspace/bench/public/checkouts/<checkout> --service <service> \
  --scratch-daemon --arms 'fathom grep none' --repeat 3 --ledger /dev/null
```

Split the corpus per subject with the `subject.service` field (keep the
top-level `machine_criterion` in every split so a per-subject run still
scores against the registered bar), and merge every subject's
`run-<ts>/results/*.json` into one directory for step 5.

## 5. Score

```sh
workspace/tmp/contextbenchscore ledger \
  -tasks testdata/contextbench/tasks-public-go.json \
  -results workspace/artifacts/context-bench/public-<date>/results \
  -run-id public-<date> -commit "$(git rev-parse --short HEAD)" -model <model id> \
  -out-md docs/bench/README.md
```

Exit 0 pass, 1 fail, 3 indeterminate. The row carries `corpus_sha256`
(PB-3), the per-task outcomes, the leaked-task list and the reliability
figures (PB-9/10). **A loss is published as-is**: the row is appended
whichever way the verdict falls, and `docs/bench/README.md` renders it.

## 6. Publish transcripts

Raw `claude -p` transcripts live under
`workspace/artifacts/context-bench/public-<date>/raw/` and are never
committed. Publish them as a release artifact (the HAL convention: every
number ships with its logs) — a tarball of `raw/` and `results/` beside
the tag whose ledger row they produced. A number without its transcripts
is a claim.

## Cost

Measured on the home corpus: ~170K tokens per cell (one task, one arm, one
trial; the shared CLAUDE.md/tool-schema baseline dominates). The `none`
arm makes no tool calls and is negligible. So:

    cells  = tasks × 2 tool arms × 3 trials
    tokens ≈ cells × 170K

35 tasks → 210 cells → **~35M tokens** at n=3 across three arms (the
figure ADR-031 carries). The full derivation yields two to three tasks per
instance, so 35 instances are ~90–105 tasks → ~90–110M tokens; pre-register
`-classes` or a smaller `-n` within 30–40 *before* the run if that is over
budget. The run is operator-initiated (`docs/intent/autonomy.md`: ask before
LLM spend) — never `validate`, never a hook, never CI.

## Pre-registered criterion for run 1 (H2-2)

Committed in the corpus's `machine_criterion` before the run and pinned by
`RegisteredCriterion` in `internal/contextbench/publicloc/corpus.go`:

1. Over the non-leaked tasks, the seeded, task-clustered paired bootstrap
   95% CI on the per-task success difference (fathom minus grep) excludes 0
   in fathom's favour, on **at least 30 non-leaked tasks** — which entails
   strictly more successes; **and**
2. fathom's aggregate token ratio against grep is **≤ 1.0**.

Success per cell: all of the gold appears in the fenced answer block within
`max_predicted = max(5, 2·|gold|)`; precision and recall are reported
beside pass/fail, never gating. A task the `none` arm passes in ≥ 2 of 3
trials is `leaked` and excluded from the verdict, listed in the row. Any
cell under 3 trials, or any task absent from an arm, makes the run
indeterminate — never a win (ADR-027).

Either outcome is recorded. A fail is a measured refutation and belongs in
the ledger as one.

## What is and is not proven by this page

Built and tested: the readers, the selection rule, the derivation, the
callers tool, the canonical hash, the import command — on synthetic
fixtures that exercise every branch. Not yet done: a real pin (step 1
needs a human to fetch and record), the first run (spend), and third-party
reproduction (a stranger needs the binary — Q4, F2). PB-19 stays open
until a public-corpus ledger row exists; PB-20 waits on F2.
