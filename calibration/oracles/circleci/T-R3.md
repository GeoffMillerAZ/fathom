# T-R3: fresh-subject parameter matrix

Protocol `parameters-t-r3.json` is committed before any oracle invocation on
these subject pins. It follows the controller's T-R3 ruling (2026-10-25).
The files and declared parameter types were inspected, not compiled or graded.

The two fresh cimg subjects each declare boolean `cron` with default false:
measure the default input and both boolean values, then union the compiler's
attested entities and workflow/job pairs. scikit-learn declares no pipeline
parameter, so its matrix is the default input alone. The pins are immutable;
source checkout identity must match the preregistration. A failed case fails
the subject, never silently narrows the input matrix. All three configurations
are measured whole. No declared workflow or job is deleted when disabled.
Public orbs are resolved by CircleCI itself; no stand-in is permitted. Only
`config process` executes, never a pipeline or one of its shell commands.

The corpus shard keeps the earlier subjects; these fresh subjects form the
holdout. Report a scope change explicitly beside any numeric comparison.
L1 uses the compiler's union at tier A; command/executor references remain C
until separate native ablation attests them. No tier is raised by this protocol.
No model judgment applies to these deterministic inputs or comparisons.

## Reproduce

The adapter used for the original reading is committed at 742a986ef. The
reading's f6e47f021 identifies the extractor and preregistration; the adapter
was uncommitted then. The review fixes add clean-content and parse refusal;
the original counts remain unchanged. Canonical has five subjects; the three
holdouts are isolated in `calibration/corpus.d/circleci-holdout-t-r3.yaml`.

From the repository root, with the pinned checkouts under
`workspace/calibration/circleci-t-r3/` and CircleCI CLI 1.7.0 at
`workspace/tools/circleci`:

```sh
mkdir -p workspace/tools workspace/tmp/circleci
GOWORK=off go -C calibration/oracles/circleci build -o ../../../workspace/tools/circleci-oracle .
GOWORK=off go build -o workspace/tools/indexbench ./cmd/indexbench
for entry in CircleCI-Public/cimg-python CircleCI-Public/cimg-node scikit-learn/scikit-learn; do
  name=${entry##*/}
  mkdir -p "workspace/tmp/circleci/$name-home"
  for mode in defs refs; do
    if [ "$mode" = refs ]; then extra=-refs; else extra=; fi
    workspace/tools/circleci-oracle -subject "workspace/calibration/circleci-t-r3/$name" \
      -subject-name "$entry" -cli "$PWD/workspace/tools/circleci" \
      -scratch "$PWD/workspace/tmp/circleci/$name-home" \
      -matrix calibration/oracles/circleci/parameters-t-r3.json $extra \
      > "workspace/tmp/circleci/$name-$mode.json"
  done
  workspace/tools/indexbench lang-oracle -dialect circleci \
    -subject "workspace/calibration/circleci-t-r3/$name" -subject-name "$entry" \
    -tool-oracle "workspace/tmp/circleci/$name-defs.json" \
    -tool-oracle "workspace/tmp/circleci/$name-refs.json" \
    -out workspace/tmp/circleci/holdout.json
 done
workspace/tools/indexbench lang-retime -reading workspace/tmp/circleci/holdout.json -runs 3 -write \
  -subject CircleCI-Public/cimg-python=workspace/calibration/circleci-t-r3/cimg-python \
  -subject CircleCI-Public/cimg-node=workspace/calibration/circleci-t-r3/cimg-node \
  -subject scikit-learn/scikit-learn=workspace/calibration/circleci-t-r3/scikit-learn
GOWORK=off go -C calibration/oracles/circleci test -race -count=1 ./...
ruby -ryaml -e 'a=YAML.load_file("calibration/corpus.d/circleci.yaml"); b=YAML.load_file("calibration/corpus.d/circleci-holdout-t-r3.yaml"); abort unless a["repos"].size==5 && b["repos"].size==3 && (a["repos"].map{|x|x["name"]} & b["repos"].map{|x|x["name"]}).empty?'
```

The CLI path must be absolute because the adapter changes its working directory.
Run under the lane's serial, nice-10/load-at-most-6 guard. Keep output paths
fresh; `lang-oracle` merges an existing reading. These commands never alter the
committed holdout or canonical reading.

CircleCI CLI 1.7.0's `circleci-cli_1.7.0_darwin_arm64.tar.gz` was checked against
its release checksums: SHA256
`c612876c351bb16de2f0fbabf81cd9a4cbcfc3a081f0be4aca27992e8be89e4f`.
The native compilation accepted every registered case. The holdout reads
11/11 definitions and 8/8 references, L2 (L1 excellent, L2 good); all earlier
canonical metrics remain unchanged. The 16.22 MB/CPU-s median was selected from
6.65, 16.22, and 17.00 on the development-machine class under nice 10, one job
at a time with the start load at most 6. No runtime workflow was started.

The three timing values are retained for provenance only: total extraction CPU
is sub-millisecond, the spread is 2.5 times, and this is not an informative
throughput reading or comparable to canonical throughput.


## Review follow-up: vendor language server

The handwritten timing/adapter notes have been removed from the measured JSON;
the disclosures stay here and in the assessment. Original instrument output is
preserved. The native-LSP comparison protocol is `lsp-protocol-r2.json`, committed
before querying the eight subject configurations. Vendor 0.54.0 resolves source
command/executor names; its full-cohort comparison and W4 ruling are pending.
The former native-ablation-only ceiling is reopened. No tier has changed.
