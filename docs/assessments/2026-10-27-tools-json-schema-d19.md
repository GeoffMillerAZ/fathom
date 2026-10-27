# JSON Schema: D19 replay and authority correction

The full four-subject replay leaves L3 **unmet → unmet**, with **3 scored pairs; 291 more Fathom pairs unattested and unscored**.
D19 still sees 3/3 only because absent oracle families omit those Fathom pairs;
the displayed precision 1 covers the scored subset, not whole-format accuracy.
The pre-D19 grade was excellent.
L1/L2 change **excellent → good** because P-T1 classifies the actual independent
third-party resolver at C. Accuracy remains sym 1/1 and refs 1/0.9993. There is
no product extractor, shared ENGINE, epoch or generated-board change.

The existing oracle remains `@apidevtools/json-schema-ref-parser` 16.0.5 (MIT),
with `yaml` 2.8.2 (ISC) positions. It is neither the format owner's evaluation
nor a maintained SCIP producer. Historical A labels are withdrawn across the
fresh reading and its L1/L2/L3 metrics. This correction is not evidence that no
stronger instrument exists.

## Population and counts

[Protocol](../../calibration/oracles/json-schema/d19-protocol-r1.json) froze all
four complete scopes before new native or Fathom results. The fourth source-only
amendment followed SDK/source preparation alone. STAC and STIX remain canonical
even though neither adds a scored cross-file pair. There is no holdout, renamed
input, URL rewrite, generated reference, duplicate subject or post-result trim.
The STIX repository calls its schemas non-normative; this measures JSON Schema
file references, not STIX validity.

| Subject | Tracked JSON / bytes | Native schemas | Scored files / bytes | L1 matched / truth / Fathom | L2 found / truth / Fathom | L3 found / truth / Fathom |
|---|---:|---:|---:|---:|---:|---:|
| SchemaStore | 1,001 / 75,198,609 | 983 | 977 / 36,424,833 | 35,948 / 35,948 / 35,948 | 40,880 / 40,880 / 40,908 | no family; 107 Fathom pairs unscored |
| JSON Schema Test Suite remotes | 79 / 17,269 | 52 | 52 / 13,036 | 97 / 97 / 97 | 3 / 3 / 3 | 3 / 3 / 3 |
| STAC | 11 / 24,804 | 11 | 11 / 24,804 | 40 / 40 / 40 | 5 / 5 / 5 | no family; 14 Fathom pairs unscored |
| STIX | 57 / 188,583 | 57 | 57 / 188,583 | 142 / 142 / 142 | 5 / 5 / 5 | no family; 170 Fathom pairs unscored |
| Pooled | 1,148 / 75,429,265 | 1,103 | 1,097 / 36,651,256 | 36,227 / 36,227 / 36,227 | 40,893 / 40,893 / 40,921 | 3 / 3 / 3 scored; 291 additional unscored |

Every scored family is C. Per-subject L1 precision/recall are 1; L2 recall is 1,
and precision is 40,880/40,908 (0.9993) for SchemaStore and 1 for the other three.
Only the suite has an L3 family: 3/3 scored recall and precision. The precision
denominator is below 20 only because the oracle emits no xref family where it
attests none, so the harness omits 291 additional Fathom pairs. Those pairs are
neither true nor false without identifier-aware attestation. We do not supply an
empty family or change the scoring rule. No new subject minimum is applied.

The exact design §13.3.1 before/after manifests are equal in all six scopes;
counts/digests are in the [receipt](data/json-schema-d19-2026-10-27/receipt.json).
All subjects have zero modified, untracked or ignored inputs after measurement.
Raw manifests, subject file lists and oracle error text stay private.
The [reconciliation](data/json-schema-d19-2026-10-27/population-reconciliation.json)
accounts for every tracked JSON: 45 are not claimed as schemas and six claimed
SchemaStore files exceed the existing 2 MiB ingest gate (38,772,143 bytes).
No native file is rejected and no resolver invocation fails. The 64,151 neutral
entries on files outside the ingest population remain explicitly unscored.

This is a fresh scoped replay, not a numerical reproduction of the historical
reading. That reading had 978 SchemaStore files / 36,427,989 bytes and 54 suite
files / 21,607 bytes, versus 977 / 36,424,833 and 52 / 13,036 now. In particular,
the historical suite byte count exceeds the complete pinned remotes manifest.
The cause is the historical cone-mode sparse checkout including parent-directory
files. The suite included two root schemas: 5,725 + 2,846 = 8,571 bytes, exactly
21,607 - 13,036, and 54 - 52 = 2 files. SchemaStore included its parent validation
schema: 3,156 bytes, exactly 36,427,989 - 36,424,833, and 978 - 977 = 1 file. The
four suite and ten SchemaStore definitions removed by the explicit scopes were
all previously matched; this does not trim hard inputs to improve accuracy. The new `-only` scopes and native claim
set exactly reconcile to the current reading. Required kinds stay field,
package and type_alias.

## Why the new corpus does not increase the denominator

The incumbent unit is a distinct source-file/root-entity to target-file pair.
It requires a literal file part, physical in-subject existence and that exact
file in the resolver's `$refs.paths()` output. It is not full `$id`, `$anchor`
or dynamic URI semantics. HTTP readers are stubbed, never fetched; unreadable
files also return empty schemas so they do not erase other document truth.

All 18 STAC and 236 STIX relative reference sites physically name subject files.
The full-population diagnostic finds zero such target files in native read sets:
STAC reports 17 URL read events and 11 initial subject-file reads, and STIX 170
URL events and 57 initial subject-file reads. Absolute schema identifiers move
resolution to URL space. The stub does not attest their local target bindings.
These 254 sites are disclosed unscored references, not valid measured pairs or
evidence of 100% whole-format L3 recall. The oracle's three in-subject SchemaStore file-reference sites are self references
and add no native cross-file pair. Fathom independently resolves 107 pairs there;
all are unattested and unscored. Together with STAC14 and STIX170 this is 291
unscored pairs beside the suite's three scored pairs, 294 Fathom pairs total. The suite's
three distinct pairs are the entire current scored L3 population.

Reproduce the diagnostic for each new subject with
`node calibration/oracles/json-schema/inspect_resolution_d19.mjs -tools workspace/artifacts/json-schema-d19-r1/tools -subject workspace/artifacts/json-schema-d19-r1/subject-2 -out workspace/artifacts/json-schema-d19-r1/replay/2/resolution-diagnostic.json`
(repeat for subject-3). Count-only results are [STAC](data/json-schema-d19-2026-10-27/resolution-2.json)
and [STIX](data/json-schema-d19-2026-10-27/resolution-3.json).

## Candidate search and remaining trigger

The [five-source search record](../../calibration/oracles/json-schema/candidate-audit-d19-r1.json)
lists every searched primary source, including misses. The separate
[named inventory](../../calibration/oracles/json-schema/candidate-inventory-d19-r1.json)
pins versions, licenses and dispositions: the owner's test suite is validation
data, scip-json converts existing indexes, Microsoft JSON language service is
an untried independent C alternative, Sourcemeta 17.3.0 has an AGPL license gate,
and Bowtie 2026.10.2 coordinates validators without an established file-pair dump
(license unresolved, not run). The actual resolver is now measured; its earlier
inventory's pending wording records the pre-run state. No generic absence claim
or untried-candidate failure is asserted.

The measured ceiling is this instrument/population: L3 has only three scored
pairs while 291 Fathom pairs lack attestation; the apparent thin precision
denominator is an instrument artifact. Independent authority caps otherwise
passing L1/L2 at good. Revisit when a
same-unit full replay has at least 20 truth and Fathom pairs per graded metric,
or an approved identifier-aware instrument measures the disclosed URI scope.
Excellence additionally requires owner evaluation or a maintained SCIP producer
under P-T1. Rewriting these subjects' identifiers to force local file reads would
change the evidence, so it is not this branch's route.

## Reproduction and controls

Use private TMPDIR/GOTMPDIR/GOCACHE/GOMODCACHE, GOWORK=off, one nice-10 job,
load at most 6 and at least 20 GiB free before each heavy step. From repository
root, with a fresh `workspace/artifacts/json-schema-d19-r1` directory:

```sh
python3 -B calibration/oracles/json-schema/prepare_d19.py sdk --out workspace/artifacts/json-schema-d19-r1
python3 -B calibration/oracles/json-schema/prepare_d19.py subjects --out workspace/artifacts/json-schema-d19-r1
python3 -B calibration/oracles/json-schema/controls_d19.py -tools workspace/artifacts/json-schema-d19-r1/tools -out workspace/artifacts/json-schema-d19-r1/controls
bash scripts/fathom-go.sh build -o workspace/artifacts/json-schema-d19-r1/bin/indexbench ./cmd/indexbench
bash scripts/fathom-go.sh build -o workspace/artifacts/json-schema-d19-r1/bin/fathom-pairs ./calibration/oracles/json-schema/fathom-pairs
python3 -B calibration/oracles/json-schema/measure_d19.py native --out workspace/artifacts/json-schema-d19-r1
python3 -B calibration/oracles/json-schema/measure_d19.py compare --out workspace/artifacts/json-schema-d19-r1 --indexbench workspace/artifacts/json-schema-d19-r1/bin/indexbench --pair-counter workspace/artifacts/json-schema-d19-r1/bin/fathom-pairs
python3 -B calibration/oracles/json-schema/measure_d19.py independence --out workspace/artifacts/json-schema-d19-r1
python3 -B -m unittest discover -s calibration/oracles/json-schema -p 'test_*.py'
```

All four native populations finish before Fathom comparisons. SDK source versions,
integrities, lock hash, binary hash and extractor tree are pinned in the protocol
and public receipts. The disclosure replay uses the product extractor at b6abd86a2; the bench-only
counter and driver changes do not alter product extraction. The harness recorded zero faults,
maximum file 125 ms and 20.52 MB/CPU-s for this disclosure replay. This is one timing capture,
not a separate median-of-three throughput experiment or speedup claim.

Independence is structural source/argv review: the oracle receives only its SDK
and pinned subject and has no Fathom comparison artifact in its reviewed reads.
The old unrelated-output perturbation could not falsify this and is withdrawn
as a control, including the equivalent authored-fixture perturbation. The
[structural record](data/json-schema-d19-2026-10-27/independence.json) explicitly
makes no dynamic-control claim. All native truth hashes match the prior run.
A genuine input-sensitivity control changes an authored reference target.
Zero-reference schemas remain in scope and repeated sites preserve the incumbent
distinct-file pair unit. Mutation: `controls_d19.py` changes each emitted C family
to A in a private copy of `json_schema_oracle.mjs`; the focused
`OracleControlTest.test_native_truth_is_input_sensitive_at_tier_c`
fails on `C != A` for each mutant, then restores GREEN (no skips). The committed
[control summary](data/json-schema-d19-2026-10-27/controls.json) and
[structural review](data/json-schema-d19-2026-10-27/independence.json) bind results.

Caps: `test_prepare_d19.py::PrepareBoundsTest.test_population_caps_refuse_whole_scope_and_symlinks_are_not_followed`
tests exact and +1 file count, source-file bytes and aggregate bytes using smaller
equivalent bounds; every over-cap input raises ValueError for the whole scope.
`test_measure_d19.py::ArtifactBoundsTest.test_artifact_cap_plus_one_refuses_whole_json`
does the same for output bytes. Production caps are 4,096 files, 32 MiB per source,
512 MiB source per subject, 64 MiB per oracle artifact (process FSIZE bound plus
bounded reads), 1 GiB Node heap and 300 seconds per subject. These limits do not
expand native URI scope or silently admit a partial success.


Original validation before T-R18, after the main merge: origin/main 3118be853 merged cleanly at 25e153422;
its changes do not alter JSON Schema extraction or the measured Go implementation.
`bash scripts/fathom-go.sh test -race ./internal/langbench -run '^(TestCommittedReadingsRoundTrip|TestCommittedReadingsAreCanonical)$' -count=1`
and `bash scripts/fathom-go.sh test -race ./internal/extractor -run '^TestJSONSchema' -count=1`
pass. Release-note, merge-debris and ADR-011 checks pass. The YAML shard parses
with pinned yaml 2.8.2; its four distinct names/pins exactly match the reading.
X-R10 finds no lane-changed Go file, and four lexical consumers in one plan
(three queue paths and this reading). The first scratch attempt ingested cleanly
but verify exited operationally (2): an uncommitted change-note fragment held ten
gone rows unknown under I44. Commit that fragment and repeat the scratch ingest
and strict check from a clean HEAD; this attempt is not a passing verification.


Clean-HEAD X-R10 rerun at 34f547acb: scratch ingest outcome `ok`; strict
language-breadth verification exits 0 with 569 claimed rows verified and none
failing (797 rows total). The ten held gone rows now verify. Reproduce without a
daemon using the committed CLI implementation and a private store/ledger:

```sh
bash scripts/fathom-go.sh build -o workspace/artifacts/json-schema-d19-r1/bin/fathom ./cmd/fathom
FATHOM_TENANCY_BUDGET=4GiB workspace/artifacts/json-schema-d19-r1/bin/fathom ingest --service fathom --path . --store workspace/artifacts/json-schema-d19-r1/verify-clean.sqlite --json
workspace/artifacts/json-schema-d19-r1/bin/fathom verify --strict --since origin/main --service fathom --store workspace/artifacts/json-schema-d19-r1/verify-clean.sqlite --ledger workspace/artifacts/json-schema-d19-r1/verify-ledger.jsonl --plan docs/plans/language-breadth.md --quiet
```

The validation record changes no plan lexical input. Subjects, SDK and builds are
pruned after the final check; reproduction recreates them with the commands above.

T-R18 follow-up: the bench-only counter consumes the harness's exact tenancy_files
membership and calls the same ResolveDocXrefs and ResolveIdentityXrefs passes.
It deduplicates the existing file/root-entity/target-file unit, emits counts only
with graded=false, and supplies no truth or empty oracle family. Each compare
first enumerates that population, then regenerates the canonical reading with
per-subject -note disclosures and rechecks membership and pair counts. No reading
was hand edited. Two Go tests cover repeated-site collapse, identifier behavior
and refusal of invalid/duplicate membership. X-R10 tenancy and clean-head strict
verification are required before READY. TestLanguagesTrackerIsGenerated is
expected red until the controller regenerates docs/languages.md at integration
(F2); the lane does not modify generated files.
