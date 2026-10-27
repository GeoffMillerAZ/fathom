# Protobuf: D19 evidence restored for the dialect

The dialect's calibration moves **L3 unmet → excellent**, with recall
**107/116 (0.9224)** and precision **107/107 (1)** at native tier A. D19's two
denominators rise from 11/11 to 116/107, both above 20; the pre-D19 excellent
grade had only 11/11. L1/L2 remain excellent. Default ingestion still dispatches
the incumbent `protoextract.go`: this branch changes no product extraction,
rpc signature, grammar, query, ENGINE, epoch or generated tracker.

## Fixed population and native authority

[Protocol](../../calibration/oracles/protoc-descriptors/d19/protocol-r1.json)
was committed at acf5e14c1 before any new binding/accuracy result. It retains
both canonical pins and the old protovalidate `-only` restriction. The added
official protobuf v34.1 source scope contains every tracked proto under
`src/google/protobuf`, selected from source metadata before results. All 92
files remain, including zero-reference and compiler-refused files. Its genuine
import root is `src`, represented by corpus `subpath: src`; the CLI receives
the verified full subject SHA explicitly for this non-root subject. No input
is edited, generated, renamed, duplicated or removed after measurement. There
is no holdout in the canonical shard.

Official protoc34.1 is the format owner's compiler, tier A under P-T1. Its
archive SHA-256 is pinned, all 22 extracted members (9,615,014 bytes) are
reverified before execution, and the compiler reports `libprotoc 34.1`.
The existing `main.go` projection reads its descriptors/source_code_info for
definitions, spans, references and resolved imports; neither a line scanner
nor Fathom chooses the native pair population. The projection now preserves each independent compiler unit
with its own ordered import roots and resolved descriptor type locations. Public [preparation](data/protobuf-d19-2026-10-27/prepare.json) and
[measurement](data/protobuf-d19-2026-10-27/receipt.json) receipts bind compiler,
source, binary and oracle identities. Raw names, file lists and errors remain
private. The three source licenses are Apache-2.0, Apache-2.0 and BSD-3-Clause.

All three complete native populations finish before any Fathom comparison.
The compiler enumerates 117 distinct cross-file pairs (245 reference sites);
116 are eligible under the disclosed discovery/scope rules, and nine are
missed. This is not a claim to find all 117 native pairs or all type references.
Repeated sites use the incumbent file/from/to-file unit, with target symbol
names dropped as before.

| Subject | Native files / compiled / refused | Native distinct pairs | Scored files / bytes | L1 matched / truth / Fathom | L2 found / truth / Fathom | L3 found / truth / Fathom |
|---|---:|---:|---:|---:|---:|---:|
| grpc-go | 13 / 13 / 0 | 0 | 10 / 16,113 | 46 / 46 / 46 | 35 / 35 / 35 | 0 / 0 / 0 |
| protovalidate | 156 / 126 / 30 | 11 | 118 / 478,546 | 1,146 / 1,146 / 1,146 | 421 / 421 / 421 | 11 / 11 / 11 |
| official protobuf source | 92 / 90 / 2 | 106 | 91 / 739,062 | 974 / 974 / 980 | 758 / 758 / 758 | 96 / 105 / 96 |
| Pooled | 261 / 229 / 32 | 117 | 219 / 1,233,721 | 2,166 / 2,166 / 2,172 | 1,214 / 1,214 / 1,214 | 107 / 116 / 107 |

Every scored family is A. Per-subject L1 recall is 1; precision is 1, 1 and
974/980 (0.9939). Pooled precision is 0.9972. All 2,166 matched ranges agree
with native spans. L2 precision/recall are 1 for every subject. L3 recall is
undefined on grpc-go's empty population, 1 on protovalidate's thin 11 pairs,
and 96/105 (0.9143) on official source; defined precision is 1. D19 has no
subject minimum. Six extra definitions all originate in the two new files
the compiler refuses because a fixture dependency lies outside the fixed
source scope. They remain precision errors; no adjudication removes them.
The nine missed file pairs remain misses and are not repaired here.

## Source and exclusion reconciliation

The exact design §13.3.1 manifest recipe runs before/after with identical
Git glob pathspecs covering the hydrated proto/config scopes. A sparse index
still lists unhydrated C++/Go files, so using `SUBPATH=.` failed during initial
preparation before binding results; the corrected pathspecs hash the complete
preregistered inputs rather than pretending the entire repository is hydrated.
Absent config scopes are explicitly absent, not successful empty manifests.
All nonempty manifests are identical before/after. Source checks report zero
modified, untracked or ignored inputs. The `src` measurement-root mapping changes
no source bytes; tracked manifests stay relative to the actual clone root.

[Reconciliation](data/protobuf-d19-2026-10-27/reconciliation.json) uses the harness's
private `tenancy_files` list, never a replacement discovery implementation.
All 261 tracked proto inputs are accounted for: grpc-go excludes three
`testdata` files; the pre-existing protovalidate restriction excludes 38 (30
with missing external dependencies and eight compiling files in the same
excluded fixture family); official source excludes one `testdata` file. Measured
file counts and byte sums match exactly. Native distinct pairs remain 11→11 on
protovalidate and fall 106→105 on official source under those rules, before matching.
The compiler's two new refusals remain inside the measured set. Counts for the
two old subjects exactly reproduce the committed reading's definitions, local
pairs, file pairs, files and bytes.

The harness records zero faults, zero files with syntax errors, maximum file
14 ms, 297 peak module pages and 8.94 MB/CPU-s in this replay. This is one timing
capture at nice10/load below6/free above20GiB, not a new median-of-three speed
claim. No daemon participates in measurement.

## Controls, alternatives and remaining work

Authored compiler controls retain zero-reference messages and show two repeated
sites collapse to the existing single file-pair unit. Changing an input's import
and type changes native truth. Perturbing the actual Fathom comparison artifact
leaves the raw official-source oracle byte-identical, then the reading is
restored. [Control](data/protobuf-d19-2026-10-27/controls.json) and
[independence](data/protobuf-d19-2026-10-27/independence.json) summaries are committed.
Mutation: `replay.py controls` changes a private neutral output from A to C;
`native_value` refuses the whole artifact, then the original A artifact is
admitted again. This tests driver admission, not a changed compiler projection.

An initial replay was withdrawn after self-review found a compiler-context bug
in this bench oracle: its global fully qualified type map mixed independent
units that reuse a type name. An authored four-file control reproduced a caller
binding to the duplicate in a file it never imports. `main.go` now requests
`--include_imports`, keeps only the root file for declaration emission and
resolves each reference with that invocation's native descriptors and ordered
roots. The protocol amendment was committed after the preliminary results;
all subject pins, populations, restrictions and grade thresholds stayed fixed.
Every subject was then replayed native-first from the corrected instrument.
The two false native pairs were in already excluded protovalidate fixtures;
the scored 116/107 population is unchanged. The joint compile of 90 individually
accepted official inputs was diagnostic only: duplicate-name fixture units do
not have to coexist, and no file was removed to make them coexist.

The [actual compiler-context control](data/protobuf-d19-2026-10-27/compiler-context-control.json)
now binds each caller to its own imported duplicate. Two Go regression tests
and [mutants](data/protobuf-d19-2026-10-27/compiler-context-mutants.json) separately
restore the erroneous global fallback and reorder native import roots by length;
each fails its regression and restores GREEN. These are own-item bench changes,
with no product ENGINE or extraction change.

Caps: `test_prepare.py` exercises exact/cap+1 source file count, per-file and
aggregate bytes, archive count/member/expanded bytes, compressed download bytes,
traversal refusal, and cached-payload tamper/symlink refusal. `test_replay.py`
covers artifact byte cap+1 and authority refusal. Smaller equivalent limits
make each boundary explicit without allocating production limits. Production
bounds are 4,096 files, 32 MiB/source, 512 MiB/source population, 8 MiB compressed
archive, 128 archive members, 32 MiB/member, 128 MiB expanded, 64 MiB per output
file and 300 seconds per subject. A timed-out process group is killed, including
its compiler children. No partial native artifact is admitted as successful.

The [five-source search](../../calibration/oracles/protoc-descriptors/d19/candidate-search-r1.json)
and [named inventory](../../calibration/oracles/protoc-descriptors/d19/candidate-inventory-r1.json)
record real alternatives, including Sourcegraph's scip-protobuf producer, the
archived Buf language server and protocompile. They are untried, not failed or
absent. The existing owner compiler already supplies A; no new oracle ceiling
is asserted. The two prior README recipes and measurements remain documented.

The D19 evidence trigger has fired. The separate dispatch trigger remains:
rerun `lang-parity` at a head with this reading after an owner decision on the
rpc Signature contract (source span versus the incumbent request-response arrow,
or an engine template). Retiring the incumbent needs its own change and epoch
bump. This branch does not claim runtime dispatch changed or that parity passed.

## Reproduction and READY checks

Use a private TMPDIR/GOTMPDIR/GOCACHE/GOMODCACHE and GOWORK=off. Run one nice10
heavy job at a time, checking load≤6 and at least20GiB free before each step.
From repository root, using a fresh private output directory:

```sh
python3 -B calibration/oracles/protoc-descriptors/d19/prepare.py --out workspace/artifacts/protobuf-d19-r1
bash scripts/fathom-go.sh build -o workspace/artifacts/protobuf-d19-r1/bin/indexbench ./cmd/indexbench
(cd calibration/oracles/protoc-descriptors && bash ../../../scripts/fathom-go.sh build -o ../../../workspace/artifacts/protobuf-d19-r1/bin/protoc-oracle .)
python3 -B calibration/oracles/protoc-descriptors/d19/replay.py controls --out workspace/artifacts/protobuf-d19-r1
python3 -B calibration/oracles/protoc-descriptors/d19/replay.py context-control --out workspace/artifacts/protobuf-d19-r1
python3 -B calibration/oracles/protoc-descriptors/d19/context_mutants.py --out workspace/artifacts/protobuf-d19-r1/context-mutants
python3 -B calibration/oracles/protoc-descriptors/d19/replay.py native --out workspace/artifacts/protobuf-d19-r1
python3 -B calibration/oracles/protoc-descriptors/d19/replay.py compare --out workspace/artifacts/protobuf-d19-r1
python3 -B calibration/oracles/protoc-descriptors/d19/replay.py reconcile --out workspace/artifacts/protobuf-d19-r1
python3 -B calibration/oracles/protoc-descriptors/d19/replay.py independence --out workspace/artifacts/protobuf-d19-r1
python3 -B -m unittest discover -s calibration/oracles/protoc-descriptors/d19 -p 'test_*.py'
(cd calibration/oracles/protoc-descriptors && bash ../../../scripts/fathom-go.sh test -race . -count=1)
bash scripts/fathom-go.sh test ./internal/lint/tenancy/...
bash scripts/fathom-go.sh test -race ./internal/extractor -run '^TestProto' -count=1
bash scripts/fathom-go.sh test -race ./cmd/indexbench -run '^(TestCorpusShardsParse|TestCmdLangOracleTagsToolFormatXrefs)$' -count=1
bash scripts/fathom-go.sh test -race ./internal/langbench -run '^(TestCommittedReadingsRoundTrip|TestCommittedReadingsAreCanonical)$' -count=1
bash scripts/fathom-go.sh run ./third_party/agentic-kit/cmd/release-notes -check
bash scripts/check-merge-debris.sh
bash scripts/fathom-go.sh test -count=1 -run TestNoOrgSpecificNames ./cmd/fathom
```

Required kinds remain enum, message, rpc and service. No nested or frozen baseline
reading is touched. X-R10 requires the tenancy lint for the bench Go changes; reading/queue
lexical consumers require a clean scratch ingest and strict language-breadth
verification before READY. Build the committed CLI into the same private bin
directory and run it with `--store` and a private `--ledger`; no global index or
ledger is used. No plan box is flipped by this lane.

Clean-head X-R10 proof at `8e29e92cc`: scratch ingest exit0, strict verification
exit0, all 569 claimed rows verified (797 total; zero claimed failures). The
tenancy lint passed for the two changed bench Go files. Release-note, merge
debris, ADR-011, focused race and recursive reading serialization checks passed.
The final docs-only proof record is re-ingested and verified before READY.

```sh
bash scripts/fathom-go.sh build -o workspace/artifacts/protobuf-d19-r1/bin/fathom ./cmd/fathom
FATHOM_TENANCY_BUDGET=4GiB workspace/artifacts/protobuf-d19-r1/bin/fathom ingest --service fathom --path . --store workspace/artifacts/protobuf-d19-r1/verify-clean.sqlite --json
workspace/artifacts/protobuf-d19-r1/bin/fathom verify --strict --since origin/main --service fathom --store workspace/artifacts/protobuf-d19-r1/verify-clean.sqlite --ledger workspace/artifacts/protobuf-d19-r1/verify-ledger.jsonl --plan docs/plans/language-breadth.md --quiet
```
