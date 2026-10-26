# Terraform two-hop ENGINE impact

## Verdict

**Measured, 2026-10-26:** the repaired engine finds all 3,528 native cross-file
references on the original Terraform subjects, recovering the 1,381 misses.
The current Terraform reading is the only reading requiring a measured regrade.
The shared resolver's other strategies retain their code paths. A separate
fixture replay checks all 59 currently routed document dialects, with the limits
below; it does not claim full-subject recalibration of those dialects.

This implements the approved two-hop design's IAC-5/IAC-6 and supports IAC-9/IAC-11,
GAP1-32 and LB-Q-terraform-rest. ADR-048 governs measured support; ADR-075 governs
the epoch entry. Queue, board and runtime-disclosure regeneration belong to the
integrator. No plan box is changed.

## Scope and ownership

**Measured:** recursive inventory includes all 185 JSON files under the language
reading tree. The Reading-schema scope is 154 files: 120 top-level, 28 holdout,
four tuned and two tuned/lf3. The 18 frozen baseline files and 13 different-schema
parity files are protected under recipe section 13.5.1, E2-R1/D20.

There are 63 document-resolver reading routes, 61 in the eligible scope, covering
59 distinct dialects. Forty-one eligible readings have positive cross-file
truth. Eligible nested document readings are `holdout/circleci-t-r3.json` and
`holdout/cue.json`. No tuned reading takes this route. Four historical
React/Tailwind readings have no current extractor route; they are recorded as
unavailable and their values are preserved, rather than called recalibrated.

**Measured source inspection:** only `HCLExtractor.ExtractDoc` emits the new
`terraform-module-output` strategy and `terraform.local_source_dir` label.
The added resolver arm handles that strategy only. Other strategy arms retain
their behavior. Additional labels do not change the existing equality-selector
test: all requested keys must still match. Terragrunt emits a `terraform.module`
selector but does not use `HCLExtractor` or emit the new strategy; its standalone
reading therefore receives no new intents.

## Cross-dialect replay

**Measured:** the committed replay instrument extracts repository-owned dialect
fixtures twice, restoring the two original engine files for the before run.
Every input digest, file/byte count and extraction-error count agrees across
runs. The receipt contains only counts, hashes and dialect identifiers.

- 311 fixture files, 192,875 bytes and zero extraction errors.
- Only Terraform's extraction digest changes. Every other extraction and
  per-dialect resolution digest is unchanged.
- Both mixed-dialect resolutions contain 77 edges with identical digests.
- The routed `fathom-config` dialect has no fixtures. This
  route is not counted as having an exercised input.

The Terraform fixtures contain no resolved module-output pair. Their unchanged
resolution digest is therefore not proof of the repair. The tracked regression
matrix and full native subject comparison supply that proof. This replay checks
existing fixture behavior, not all subject files behind the other 60 eligible
document readings. Strategy isolation bounds their impact; the receipt does not
rename that argument as a full-subject measurement.

Data: [counts and digest receipt](data/2026-10-26-iac-docxref-impact.json).

## Regraded reading deltas

**Measured:** both binaries were built from clean detached checkouts. Binary VCS
revision and clean state are checked by `grade.py`: baseline `6db920e72`, repaired
`2ff8b2ac3`. Both engines use the same native truth and exact subject SHAs, files
and byte counts. Three runs per engine agree on all counts and grades; tenancy
uses the run with median CPU throughput.

| Native subject | Files / bytes | Truth | Found before | Found after |
|---|---:|---:|---:|---:|
| EKS | 87 / 523,029 | 866 | 651 | 866 |
| VPC | 77 / 453,543 | 2,662 | 1,496 | 2,662 |
| Combined | 164 / 976,572 | 3,528 | 2,147 | 3,528 |

**Computed from measured counts:** recall rises from .6086 to 1.0000, with
precision .9940 to .9963. The 13 extra cross-file pairs remain in the precision
denominator. No subject, truth reference or hard file is removed. The native
cross-file oracle scores file-granularity targets; this is no claim of a typed
attribute-binding oracle. The computed support result belongs to
[the Terraform reading](../bench/readings/lang/terraform.json); this assessment
does not maintain a separate level table.

The [native calibration assessment](2026-10-26-terraform-two-hop-calibration.md)
records exact pins, before/after source and truth identity, tool versions and
the newly pinned ancillary-provider caveat.

## ENGINE, EPOCH and bounds

**Measured code and regression evidence:** extraction retains output intent
without opening sibling files. Fusion makes exactly two indexed hops and checks
the declaration's kind and source provenance. Duplicate calls/outputs and
same-file address collisions stay unresolved; repeated identical labels do not
create ambiguity. Every output intent enters one resolved/refusal counter, and
ingest discloses those counts. Registry, remote, escaping, caller-directory and
nonliteral sources remain unresolved. OpenTofu constant source expressions are
counted unsupported in this pass, rather than called invalid.

The 512-entry adversarial tests exercise distinct repeated addresses and repeated
labels. New index construction uses constant-time membership checks and no
recursive graph walk. No new product admission cap is introduced. Existing
pipeline file-size/binary gates remain in force. Calibration-only preseed limits
and their cap/cap+1 whole-manifest refusal tests are recorded separately.

`internal/fusion/epochs/iac-terraform-two-hop.md` forces re-extraction under
ADR-075; no numeric epoch is edited. The unique changelog fragment describes the
observable resolution change. The product executes or downloads no tool/provider.

## Reproduction and handoff

From the repository root with the lane's documented toolchain/cache environment,
run the fixture instrument under the shared machine's serialized load limit:

```sh
python3 calibration/oracles/terraform/impact_replay.py --base 6db920e72 --out workspace/artifacts/terraform-impact-before.json
python3 calibration/oracles/terraform/impact_replay.py --out workspace/artifacts/terraform-impact-after.json
python3 calibration/oracles/terraform/impact_replay.py --compare workspace/artifacts/terraform-impact-before.json workspace/artifacts/terraform-impact-after.json
python3 calibration/oracles/terraform/impact_replay.py --conservation-base 6db920e72
go test -race ./internal/extractor ./internal/fusion -count=1
```

**Measured:** both full package race suites pass on the merged branch. The strict
top-level reading round-trip, nested oracle/scanner race tests, preseed cap tests,
release-note check, merge-debris check and exact X-R3 command all pass. Final
conservation covers 185 JSON files: only Terraform changes, 184 files remain
byte-identical, and all 154 eligible readings preserve `required_kinds`. The 18
frozen baseline and 13 parity files are among those unchanged bytes. X-R5 scans
find only benign module-version, code and existing instruction matches; no
subject identities or name/email shapes. Both final source reviews report no
findings. Commands, counts and private-log hashes are in the paired receipt.
The controller owns full merged-head validate, verification claims and generated
surfaces. No plan checkbox is changed.
