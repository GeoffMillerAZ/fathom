# Crossplane calibration

The existing Go oracle measures Crossplane definitions and reference syntax. Its
`-tier-a` compatibility flag runs the official CLI against core CRDs. All metrics
remain tier C: its nested definition population comes from the authored walk,
not the native validator. `probe.py` separately records provider-schema calibration and
explicitly tier-C directional `fromFieldPath` diagnostics. It does not write a
Reading or promote an oracle tier.

## Frozen inputs

`pins-r1.json` fixes nine public composition/XRD subjects, three Upbound provider
source archives and the official CLI before the new probe. The original three
canonical corpus subjects are retained. The tier-C reading is newly measured
over their complete scope; it does not retain the legacy native-validation filter. The six additional
probe subjects are exploratory calibration inputs, not an independent holdout.
After the first six-subject probe found no matching Upbound managed-resource
bases, three release-pinned legacy network compositions/XRDs were added. The
versions were manually selected for known legacy network examples, with no
preregistered version rule. All 21 provider-matched bases come from those
post-hoc subjects; this is not broad or independent evidence. None
of the first six subjects was removed.
The older AWS tag has both its annotated tag object and peeled commit recorded.
Provider source CRDs are calibration inputs only. Source archives are not a
claim of equivalence with a published OCI package. GitHub codeload tarballs
are not guaranteed byte-stable; archive hashes record the downloaded bytes,
while the git SHA and independently derived CRD tree digest are the durable
source/content pins. A changed archive must be investigated and its CRD tree
verified, not silently accepted as the same archive.

All subjects, provider CRDs, raw reports, source manifests and executable tools
belong in an operator-specified ignored cache or `workspace/`. None are vendored.
Check each source's Apache-2.0 licence at its fixed commit. Fetching is a manual
calibration setup action. The product gains no downloader or dependency.

Verify every subject/provider archive against its own repository and peeled
commit, from the repository root. `--fetch` is an explicit setup download;
omit it to verify the cached archives offline. The command exits nonzero on any
digest, size or commit-root mismatch and publishes only counts and hashes:

```sh
python3 calibration/oracles/crossplane/verify_archives.py \
  --archives workspace/artifacts/crossplane-r3/archives \
  --out workspace/artifacts/crossplane-r3/archive-pins.json --fetch
```

Round 3 independently checked all twelve archives. The Azure/GCP network pins
had incorrectly reused other versions' archive metadata; both are corrected.
The legacy AWS pin previously described its annotated-tag-object archive. It
now describes the peeled-commit archive, whose 29 relative entries and contents
match the old archive. No subject commit, scope or measured reading changes.

## Reproduce the probe

Use Python 3.9 or later, the pinned Crossplane CLI and yq v4.47.1. Their SHA-256
values must match the host entry in `pins-r1.json` (`cli.platforms` and
`yq.platforms`) and the local manifest. Linux amd64/arm64 and Darwin arm64
are pinned; the legacy top-level tool hashes identify Darwin arm64 only. Provide a local JSON
manifest with these fields:

- `subjects`: objects with `id`, `root` (clean Git checkout), `sha` and `scope`
  glob strings. Use `*.yaml` and `*.yml` for all YAML paths, including root files.
- `provider_roots`: objects with `id`, `root`, `sha` and `crd_sha256`. The CRD
  digest must be independently derived from the pinned source archive, over
  sorted relative paths within `package/crds`, each path plus NUL plus the raw
  32-byte SHA-256 of that file. The probe verifies the on-disk digest.
- `core_crds`: the pinned Crossplane checkout's `cluster/crds` directory.
- `tool_sha256`: `crossplane` and `yq` binary SHA-256 values.
- `control_resources`: objects with `provider_id` and `path` relative to that
  provider checkout, plus `source_sha256` derived from the pinned archive and
  `document_index` (default zero), selecting one example resource per provider.

```sh
python3 -m unittest calibration/oracles/crossplane/probe_test.py
python3 calibration/oracles/crossplane/probe.py \
  --manifest workspace/artifacts/crossplane-r1/manifest.json \
  --out workspace/artifacts/crossplane-r1/before.json \
  --scratch workspace/tmp/crossplane-r1-before \
  --crossplane workspace/tools/crossplane \
  --yq workspace/tools/yq
```

The two tool paths are operator-selected local files with the pinned hashes.
Run native batches under `nice -n 10`, one at a time, after the host's 1-minute
load is at most 9. Repeat into separate `after` output/scratch locations. Freeze
the exact tracked-set manifests with design section 13.3.1 before and after;
review untracked and ignored paths as well. Publish counts and hashes only.

Native subprocesses use a fresh environment, scratch HOME, null kubeconfig and
dead proxies. No provider process, cluster, cloud API, Function or container is
started. Native validation can report valid, invalid and missing-schema rows.
Missing schemas and authored schema-path scans never become native bindings.
`notDeclared` means absent from the explicit source schema, not a native invalid
field. In particular, raw XRDs omit fields Crossplane adds to generated XR schemas.
Literal bases keep absent metadata names; the native empty-name report is
matched without inventing a runtime name. Dynamic KCL,
Go-template and other function output is not statically materialized.

A source-fact control validates each provider example, then adds an unknown
field under `spec.forProvider`. This tests schema sensitivity, not a definition
or reference metric. Raw reports remain local. Public receipts contain only
pins, counts and hashes. A timeout, cap, source drift or malformed report fails
the whole attempt and leaves no success receipt.

## Product responsiveness and native independence

Between the two native probes, run the committed control from the repository root:

```sh
mkdir -p workspace/tmp/crossplane-proof
cat calibration/oracles/crossplane/product_ablation.go.txt > workspace/tmp/crossplane-proof/product_ablation.go
go run ./workspace/tmp/crossplane-proof/product_ablation.go \
  -subject-file "$LEGACY_AWS_CHECKOUT/apis/cluster/composition.yaml"
```

The control removes the pipeline-step entity and its reference selectors from an
in-memory product specification. It requires an actual extracted step to disappear
while the root resource count and subject bytes stay unchanged. It does not
change files on disk, the native tool or native inputs. Equal before/after native
receipt and raw-report hashes establish independence for this perturbation only.
The control does not certify reference binding or a native definition population.

Run the Go oracle's full race suite with the pinned CLI on PATH and
`OX_CROSSPLANE_CRDS` pointing to the pinned core CRDs, so its offline integration
test runs rather than skips:

```sh
PATH="$TOOLS:$PATH" OX_CROSSPLANE_CRDS="$CROSSPLANE_CHECKOUT/cluster/crds" \
  go -C calibration/oracles/crossplane test -race -count=1 ./...
```

The Python suite tests refusals for malformed/missing reports, identity mismatches,
source drift, stale receipts, timeouts and output caps. File/byte/base/patch/call
cap tests use reduced limits with cap-plus-one fixtures; they do not allocate the
full production caps. A mixed-file XRD cannot supply a schema to an admitted
Composition. A native provider-base counter is separate from whole-file results.

## Reproduce the full canonical tier-C reading

Run from the repository root. `ENGINE_CHECKOUT` must be a clean checkout of the
full `ENGINE_COMMIT`, with no engine changes. The recorded round-2 engine is
`1b4c93a844e8b9727c2653636fd665a1748a2232`. Keep binaries and intermediates in
`workspace/` or the lane cache. Use Go 1.27.3, GOWORK=off, GOMAXPROCS=2 and
GOFLAGS=-p=2, with TMPDIR/GOTMPDIR/GOCACHE/GOMODCACHE in that cache.

```sh
nice -n 10 go -C "$ENGINE_CHECKOUT" build -trimpath -o "$TOOLS/indexbench-crossplane" ./cmd/indexbench
nice -n 10 go -C calibration/oracles/crossplane build -trimpath -buildvcs=false -o "$TOOLS/crossplane-oracle" .
go version -m "$TOOLS/indexbench-crossplane"
nice -n 10 python3 calibration/oracles/crossplane/grade.py \
  --cache workspace/artifacts/crossplane-r2 \
  --manifest workspace/artifacts/crossplane-r2/manifest.json \
  --indexbench "$TOOLS/indexbench-crossplane" \
  --oracle-bin "$TOOLS/crossplane-oracle" \
  --engine-commit "$ENGINE_COMMIT" \
  --out workspace/artifacts/crossplane-r2/canonical-reading.json
```

`grade.py` checks clean subject pins and indexbench VCS identity. It uses only
the original three canonical subjects. The independent oracle runs without
`-tier-a`; all composed-resource elements and all 244 Crossplane files count.
The core subject's `-only` globs are the original corpus shard scope, not the
189 native-accepted-file list. Three full pooled runs must agree in counts and
metrics; the driver copies the median CPU-rate harness bytes. Any achieved or
unmet grade is recorded. Do not append or reorder notes after serialization.
That output is the frozen extraction snapshot. The current committed reading
is regenerated under R-D19 by the exact main harness as described below.

## Bounded owner-tool probes

The Upbound tool versions and official checksum URLs are recorded in the
round-2 public receipt. Add the chosen host Up binary hashes as `tool_sha256.up` (0.34.2) and
`tool_sha256.up_modern` (0.38.0) in the local manifest before executing them.
The public receipt includes official Darwin/Linux checksum metadata; the
recorded executions used Darwin arm64. Run from the repository root:

```sh
nice -n 10 python3 calibration/oracles/crossplane/owner_probes.py \
  --manifest workspace/artifacts/crossplane-r2/manifest.json \
  --crossplane "$TOOLS/crossplane" --up "$TOOLS/up-v0.34.2" \
  --up-modern "$TOOLS/up-v0.38.0" \
  --out workspace/artifacts/crossplane-r2/owner-probes.json \
  --scratch workspace/tmp/crossplane-r2-owner-probes
```

The driver records local package builds, exact legacy render attempts and xpls
initialization/diagnostics over the nine scopes. It uses dead proxies, an empty
HOME, null kubeconfig and an unreachable Docker endpoint. Tool failures, missing
prerequisites and empty diagnostics are results, never binding certificates.
Native rendering does not translate legacy resources into a Pipeline. Raw
outputs stay private; the assessment publishes only counts, hashes and measured
dispositions. These are candidates toward stronger evidence, not a ceiling.

For the plain walk's independence control, save `crossplane-oracle -subject
"$SUBJECT"` JSON for each original subject before the committed product-control
command, run that command, and save the same three oracle commands afterward.
The raw JSON hashes must agree. For source-fact sensitivity, use an authored
minimal Composition in a separate scratch directory, change only
`metadata.name`, and require different oracle JSON with the changed definition
name. This fixture is a control, never a scored subject. Round-2 records both
controls separately from the earlier native-schema before/after experiment.

## R-D19: current main-harness reading

The frozen lane snapshot at `6dd4a0a29` keeps the complete 244-file tier-C
population. A clean main `5d41806d0` indexbench, built with `-trimpath`, applies
both D19 evidence rules without extraction or native calls. Set `D19_INDEXBENCH`
to that executable and run serially at actual nice>=10 and load<=9:

```sh
python3 calibration/oracles/crossplane/regrade_d19.py --indexbench "$D19_INDEXBENCH" --lane-commit 6dd4a0a29 --main-commit 5d41806d0 --receipt docs/assessments/data/2026-10-26-crossplane-d19-regrade.json
```

The script reads both parents from Git, generates the reason-separated reading
note before invoking main's harness, checks frozen measurements and copies its
exact canonical bytes. Denominators are 33/33; D19's general floor is met, no
manifest subject bar applies, and tier C still caps L3 at good. Old R2/R3 hashes
are historical extraction identities; the new receipt binds the current bytes.
Raw owner-tool output hashes/lengths, private manifest hashes, package-output
hashes and raw-log digests are private artifacts, not public evidence. Publish
owner-tool outcomes as aggregate counts/status; retain source pins and
path-independent content identities, not path-bearing output fingerprints.

The regrade wrapper enforces actual nice>=10 and waits for load<=9 before each
Go metadata/harness launch. Raw command output stays private and is not hashed
into the regrade receipt.
