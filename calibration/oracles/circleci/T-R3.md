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


## Review follow-up: complete vendor population (r3)

The r2 receipt contains the 34 source-walk use sites and their native answers.
That containment check did not establish recall completeness. Its authority
claim is superseded by `lsp-protocol-r3.json` and `native-population-r3.json`.
The vendor server remains pinned at 0.54.0 with the same source/archive/binary
identities, admitted A for the complete evaluated family by Q6/T-R7/P-T1.

Every one of 20 command/executor declarations across all five canonical and
three separate holdout configs is now queried, including unused declarations.
All 43 returned locations are classified: 34 direct uses, eight workflow
executor-parameter/matrix values, and one same-name namespace error. The owner
of a workflow executor parameter is its invoked job declaration; owner-target
pairs deduplicate. Unknown locations fail the whole instrument.

Both vendor methods confuse the same-name command/job case: definition points
the command step to the job, while references on the command also return the
workflow job entry. Native compiler expansion adjudicates the command use;
the workflow entry is recorded as an oracle error. Every newly retained
executor-parameter binding is also checked against its compiled job instance.
All 11 compiler checks agree (eight parameter bindings plus the earlier three).
No pipeline, shell body, container or Docker runs. Two orb uses resolve
externally and remain unscored.

Canonical reference truth grows from 74 to 82 pairs. Fathom emits 75 and finds
74: precision .9867, recall .9024, L2 good. L1 stays good at 48/49 definitions.
The separate holdout stays 11/11 definitions and 8/8 references, excellent.
The eight missing bindings are an implementation gap, not missing native
oracle authority. DocRef.From selects the containing entity; joining a
workflow argument to an executor-typed parameter of its invoked job requires
shared strategy work held T-R2. No extractor, input matrix or shard changes.
Valid default-disabled workflows remain intact.

## Native reference reproduction

Use a private Python with PyYAML 6.0.3, the pinned subjects at the checkout paths
in `lsp-protocol-r3.json`, and verified vendor binaries at these scratch paths:

```sh
GOWORK=off go build -o workspace/tools/indexbench ./cmd/indexbench
GOWORK=off go -C calibration/oracles/circleci build -o ../../../workspace/tools/circleci-oracle .
python3 calibration/oracles/circleci/native_refs.py \
  -adapter workspace/tools/circleci-oracle -cli workspace/tools/circleci \
  -lsp workspace/tools/circleci-yaml-lsp -out-dir workspace/tmp/circleci-native
python3 calibration/oracles/circleci/summarize_native_population.py \
  -results workspace/tmp/circleci-native -out workspace/tmp/circleci-native/population.json
python3 calibration/oracles/circleci/grade_native.py \
  -indexbench workspace/tools/indexbench -native-dir workspace/tmp/circleci-native \
  -out-dir workspace/tmp/circleci-native-reading
python3 -m unittest discover -s calibration/oracles/circleci -p 'test_*.py'
```

The wrapper runs mandatory compiler adjudications before constructing truth.
The grade driver requires fresh output, emits the namespace defect through
`-note`, and retimes the separate cohorts three times. Canonical runs are
19.65, 18.66, 17.93 MB/CPU-s (median18.66); holdout runs17.58,8.34,17.25
(median17.25). These tiny warm measurements are non-informative for general
throughput. Full native messages stay in scratch; committed receipts carry
counts, identities and hashes. All 14 Python tests pass.

## X-R4 sources searched, 2026-10-25

| Required source | Named candidate / search result |
|---|---|
| Owner toolchain / compiler dumps | CircleCI CLI 1.7.0 `config process` and the preregistered parameter matrix measured; compiler expansions corroborate all eight new executor-parameter bindings and three earlier checks. The compiler evaluates one input environment at a time, so disabled definitions remain genuine L1 coverage misses; valid workflows are never deleted. |
| SCIP and LSIF lists plus GitHub | Read [Sourcegraph's supported indexers](https://sourcegraph.com/docs/code-navigation/precise-code-navigation), [SCIP's list](https://github.com/sourcegraph/scip#tools-using-scip), and Sourcegraph-maintained [LSIF.dev](https://lsif.dev/). No CircleCI config producer was listed. GitHub repository query `scip-circleci in:name` returned zero. These bounded misses are not a proof of universal absence. |
| Language servers | CircleCI's official Apache-2.0 server 0.54.0 at the pinned source/checksum above was queried at every command/executor declaration across both separate cohorts; all 43 returned locations are classified. Both methods confuse the same-name namespaces; compiler adjudication records one oracle error. No absent-native-binding ceiling remains. |
| Registry parsers / type checkers / AST dumpers | npm lookup of official `@circleci/circleci-config-sdk` 0.12.5 (MIT) describes construction of configs, not evaluation of existing source declarations. Lookup `@circleci/circleci-config-parser` returned 404; web npm query `circleci parser` supplied no additional native evaluator. PyYAML 6.0.3 and yaml.v3 remain independent/source-position instruments, with no new A label. |
| Runtime call / require tracing | Read CircleCI's [local execution documentation](https://circleci.com/docs/guides/toolkit/how-to-use-the-circleci-local-cli/) and [troubleshooting](https://circleci.com/docs/reference/troubleshoot/): local job execution follows config processing and uses Docker. Docker is forbidden on this machine and running subject job bodies is unnecessary for the measured compiler/LSP binding family. No runtime trace was executed or claimed to attest disabled definitions. |

X-R3 publication uses hashes and byte counts for the eleven compared compiler
fragments; subject source bodies and shell commands are not copied into the
receipt. Equality is checked before summarization. Native LSP ranges remain
semantic response evidence, with subject pins and no copied source bodies.
