# Kustomize: expanded native D19 evidence

## Verdict

**Measured:** the five fixed subjects grade L1 good and L2/L3 excellent.
Cross-file precision and recall are both 35/35, so each metric clears D19's
20-reference floor. All three pooled runs agree. Native truth, built-file lists,
receipts and tracked sources match before and after byte for byte. The eight
native resource no-ops remain in Fathom's definition precision denominator.
This evidence addresses [LB-Q-kustomize-rest](../plans/language-breadth.md),
GAP1-35 and IAC-17/IAC-18; no plan box is changed and no engine code changes.

## Frozen population and scope

The one Kustomize shard is `calibration/corpus.d/kustomize.yaml`. The fixed
pins in `calibration/oracles/kustomize/pins.json` are:

| Membership | Repository | Commit | Scope |
|---|---|---|---|
| Original | kubernetes-sigs/kustomize | `9790a1c3efd2fd35f1b768d495112834176581c1` | `examples/` |
| Original | argoproj/argocd-example-apps | `8088f4c0d970abb09e250248cc97e35623447cb5` | root |
| Expanded | fluxcd/flux2-kustomize-helm-example | `4f1a5b1051b469108d1bdf0ebec6c814427e3834` | root |
| Expanded | fluxcd/flux2-multi-tenancy | `16752eb2929feb1d0e7de6ad4fa0a2e1972825ac` | root |
| Expanded | GoogleCloudPlatform/microservices-demo | `38e7348eb289eb5b87c0c6e8cb19ced0449dc389` | root |

Earlier private exploratory probes informed expanded-subject selection. The
expanded population is therefore not a blind holdout. This measurement is
Kustomize's offline API and CLI only: no Helm rendering, controller, cloud,
plugin or network execution. Native builds use a scrubbed environment and dead
proxies. Remote and absolute paths in each reachable Kustomization closure
cause exclusion before build, with counts disclosed.

## Evidence contract

The pinned `sigs.k8s.io/kustomize/api` v0.21.1 typed
`types.Kustomization.Unmarshal` and native kyaml positions enumerate every
native-recognized Kustomization file and typed field. The pinned v5.8.1 CLI
builds eligible roots. Each entry is ablated in the actual build context;
unchanged outputs are no-ops, and failures and unsupported fields are counted.
Components are evaluated through every real, reachable non-Component parent,
not a synthetic wrapper or a Component-only build. The oracle's private
receipt records full population, exclusions, context counts and hashes.

## Per-subject native evidence and exclusions

The private `before-summary.json` has SHA256
`16eb23bd7fc44ec1d986de3c0260c8db965c97d0f2238452e437ecb585acd4e1`.
The native binary SHA256 is
`bc77e3505fbd414551cf7b15d1e772a288bc5afd97e31e49655b10524d1888e9`.
The tracked-set columns cover all source files under each frozen scope, not
just Kustomization files. Before and after counts and tracked-set hashes agree;
all five native truth files, built-file lists and receipts are byte-identical.

| Repository | Tracked files / bytes | Native files / built files | Definitions / references / cross-references | Native exclusions |
|---|---:|---:|---:|---|
| kubernetes-sigs/kustomize | 106 / 288,614 | 18 / 16 | 52 / 36 / 32 | 1 remote, 1 build failure |
| argoproj/argocd-example-apps | 95 / 86,416 | 4 / 3 | 37 / 34 / 34 | 1 decode rejection |
| fluxcd/flux2-kustomize-helm-example | 34 / 56,099 | 7 / 7 | 18 / 11 / 11 | 0 |
| fluxcd/flux2-multi-tenancy | 33 / 90,466 | 8 / 8 | 21 / 13 / 13 | 0 |
| GoogleCloudPlatform/microservices-demo | 364 / 12,744,777 | 21 / 12 | 66 / 54 / 54 | 9 Components without valid context |

The typed inventory below is declared native scope across every decoded
Kustomization, including entries later excluded or found to be no-ops. It is
not the effective-entry count or the graded product denominator. Patches sum
`patchesStrategicMerge`, `patches` and `patchesJson6902`; generators show
ConfigMap / Secret. Component contexts count valid / tried real parents.

| Repository | Resources / bases / components | Patches | Generators | Images | Component contexts |
|---|---:|---:|---:|---:|---:|
| kubernetes-sigs/kustomize | 29 / 0 / 0 | 8 | 4 / 0 | 4 | 0 / 0 |
| argoproj/argocd-example-apps | 33 / 1 / 0 | 0 | 0 / 0 | 0 | 0 / 0 |
| fluxcd/flux2-kustomize-helm-example | 13 / 0 / 0 | 4 | 0 / 0 | 0 | 0 / 0 |
| fluxcd/flux2-multi-tenancy | 17 / 0 / 0 | 10 | 0 / 0 | 0 | 0 / 0 |
| GoogleCloudPlatform/microservices-demo | 43 / 0 / 12 | 28 | 0 / 0 | 34 | 12 / 12 |

Across the pool: 58 native-recognized files, 46 built files, 248 typed
entries, 194 definitions, 148 references and 144 cross-references. The
oracle attempted 156 entry ablations: 148 effective and eight no-ops, all
eight in `resources`. One file was rejected at native decode, one root at
native build, and one remote closure before build. The typed inventory also
counts fields the graded oracle does not emit: `namePrefix` 12,
`commonLabels` 2, `configurations` 1, `vars` 3, `labels` 4 and `namespace` 4.
Those fields are disclosed, not called absent.

The GCP subject contains 15 native Components. Six have a valid real parent;
nine have no valid parent and are excluded. The six valid Components were
visited through 12 actual parent contexts, with 17 effective entries and zero
Component no-ops. No synthetic wrapper contributes truth. These native truth
counts are separate from the graded metric denominators below.

The exact tracked-set recipe in design §13.3.1 records source file counts,
bytes and hashes before and after. The native before/after truth JSON, built
file list and receipt must match byte for byte. Each of three full pooled
grades must use the native built-file scope, preserve the original subject
SHAs and cross-reference floors (12, 1; pooled 13), reconcile actual measured
files and bytes, and have zero faults. Native built files whose basename the
unchanged dialect does not support are counted separately. Per-subject
definition, reference and cross-reference counts and ratios remain visible;
unevidenced entries remain precision costs. Median CPU throughput is selected
only after all counts, metrics and grades agree across runs.

## Three-run grade

Each run measured the same 46 supported built files and 24,664 bytes. Per
subject, `defs` is matched / Fathom / native; references and cross-references
are matched / truth. All reference and cross-reference precision and recall
ratios are 1. The cross-reference denominator in each subject is its own
matched/truth count, so all five per-subject L3 samples remain thin under D19.

| Repository | Files / bytes | Defs | Symbol precision / recall | References | Cross-references | Per-subject L3 sample |
|---|---:|---:|---:|---:|---:|---|
| kubernetes-sigs/kustomize | 16 / 1,848 | 52 / 52 / 52 | 1 / 1 | 36 / 36 | 12 / 12 | Thin |
| argoproj/argocd-example-apps | 3 / 1,070 | 37 / 37 / 37 | 1 / 1 | 34 / 34 | 1 / 1 | Thin |
| fluxcd/flux2-kustomize-helm-example | 7 / 1,761 | 18 / 22 / 18 | 0.8182 / 1 | 11 / 11 | 2 / 2 | Thin |
| fluxcd/flux2-multi-tenancy | 8 / 3,091 | 21 / 25 / 21 | 0.84 / 1 | 13 / 13 | 4 / 4 | Thin |
| GoogleCloudPlatform/microservices-demo | 12 / 16,894 | 66 / 66 / 66 | 1 / 1 | 54 / 54 | 16 / 16 | Thin |

Pooled counts are 194 matched / 202 Fathom / 194 native definitions,
148 / 148 references, and 35 / 35 cross-references. Pooled symbol precision
is 0.9604 and recall is 1. Reference precision/recall and cross-reference
precision/recall are each 1. D19's **metric-own** L3 denominators are
`xref_fathom=35` for precision and `xref_truth=35` for recall. The unchanged
reader reports L1 good, L2 excellent and L3 excellent; the public reading
copies the selected median without changing any measured field.

The eight extra Fathom definitions are four in each Flux subject. The private
unmatched records locate all eight on unchanged `resources` source lines; none
appears in native truth. Both complete native subject receipts record exactly
four `resources` no-ops and no other no-ops, with all files in those subjects
built. This reconciles the eight precision costs to native ablation no-ops;
no definition is discarded or adjudicated away.

Three full-pool CPU throughput readings are 11, 12.74 and 12.36 MB/s. The
median is run 3 at 12.36 MB/CPU-s; it reports zero faults and 0 ms maximum
per-file time at the reading's precision. The three runs have identical
counts, metrics and grades. Timings are a floor check on this machine, not a
cross-machine benchmark.

## Controls and whole-unit bounds

**Measured independence:** the committed product-side control removes only the
actual document extractor's resource selectors in memory. KustomizeResource
output changes from two to zero while the Kustomization root stays one and
pinned source bytes stay unchanged. The native after phase retains every raw
truth digest. The source-fact control in `TestOracleCLIPopulatesComponent`
changes a Component's image selector; the actual CLI truth changes, the
Component package remains, and its effective entry becomes a no-op.

**Mutation:** omitting the real Component from `main.go`'s built-file population
makes `TestOracleCLIPopulatesComponent` fail with `component missing from files-out`.
The mutant exits 1; the restored test passes under race detection. Reproduce
from the repository root after the README's cache/environment setup:

```sh
python3 - <<'PYMUTATION'
import json
from pathlib import Path
src = Path('calibration/oracles/kustomize/main.go')
text = src.read_text()
needle = '\t\t\tbuilt = append(built, p)\n\t\t} else {\n'
assert text.count(needle) == 1
mutant = Path('workspace/tmp/kustomize-component-files-mutant.go')
mutant.parent.mkdir(parents=True, exist_ok=True)
mutant.write_text(text.replace(needle, '\t\t\t// mutant: omit Component file\n\t\t} else {\n'))
Path('workspace/tmp/kustomize-component-files-overlay.json').write_text(json.dumps(
    {'Replace': {str(src.resolve()): str(mutant.resolve())}}))
PYMUTATION
KUSTOMIZE_BIN="$KUST_CACHE/tools/kustomize-5.8.1" nice -n 10 go -C calibration/oracles/kustomize test -race -overlay "$PWD/workspace/tmp/kustomize-component-files-overlay.json" -run '^TestOracleCLIPopulatesComponent$' -count=1 .
KUSTOMIZE_BIN="$KUST_CACHE/tools/kustomize-5.8.1" nice -n 10 go -C calibration/oracles/kustomize test -race -run '^TestOracleCLIPopulatesComponent$' -count=1 .
```

**Caps:** `TestOracleCLIAdmissionCaps` sends 513 recognized files against 512,
8,193 entries against 8,192, 65 real parent contexts against 64, and 65,729
planned native calls against 65,536 through the actual CLI. Each refuses the
whole subject with no truth, file list or receipt. `TestCaps` and
`TestSourceAndBuildBudgets` check exact cap/cap+1 boundaries. Source admission
is bounded at 100,000 files and 256 MiB; `TestActualAdmissionCaps` also exercises
real preflight refusal with reduced file/byte budgets. Output is bounded at
16 MiB per command and the whole oracle at 30 minutes. Timeout, signal,
output, write and restore failures cannot become native semantic rejections.

The final full native-oracle race suite passes, including source-fact and CLI
admission tests. Binary identity regression tests prove a clean exact revision
is admitted while dirty, wrong or missing revisions refuse. The existing
`TestKustomizeBaseDirectoryNeedsTheOneFileRule` passes: when a directory also
contains Kubernetes documents, its unique Kustomize file remains preferred.
The old provenance's deferred trigger had already fired and is corrected here;
no resolver change is needed.

## Reproduction and handoff

The [oracle README](../../calibration/oracles/kustomize/README.md) gives the
serialized preparation, grading, product control and publication commands.
The [native and grade receipt](data/2026-10-26-kustomize-r1.json) contains public
counts and hashes. Source, raw file lists, people, attribution and unmatched
diagnostics remain private. No Helm, plugins, cloud calls or product downloads
are involved. The nine Components with no real parent remain unevidenced;
unsupported typed-field inventories remain explicit, and native destinations
outside the dialect-owned file scope are not certified by this reading.

The shared Apple Silicon helper used Go 1.27.3, nice 10, GOMAXPROCS=2 and
GOFLAGS=-p=2. Per-subject launch loads in all three runs are recorded and at
most 9. Concurrent jobs were not recorded. The median uses CPU throughput;
wall time and the changed-population throughput are not regression comparisons.
The strict reading round-trip, identity/D19 tests, release/debris and X-R3
checks pass. Conservation covers all 185 reading files: only Kustomize changes,
184 files and all 31 frozen/parity files remain byte-identical, and all 154
eligible readings preserve required kinds. No metric key is removed. The X-R5
scan finds only benign syntax, package names and guard patterns; no subject
identities, name/email shapes or local home paths. Counts and log digests are in
the [controls receipt](data/2026-10-26-kustomize-r1-controls.json). The controller owns full merged-head `task validate`, verification,
queue/board/runtime regeneration and push. No plan box is changed.
