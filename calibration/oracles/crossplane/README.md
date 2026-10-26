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
canonical corpus subjects and their scores are retained. The six additional
probe subjects are exploratory calibration inputs, not an independent holdout.
After the first six-subject probe found no matching Upbound managed-resource
bases, three release-pinned legacy network compositions/XRDs were added. None
of the first six subjects was removed.
The older AWS tag has both its annotated tag object and peeled commit recorded.
Provider source CRDs are calibration inputs only. Source archives are not a
claim of equivalence with a published OCI package.

All subjects, provider CRDs, raw reports, source manifests and executable tools
belong in an operator-specified ignored cache or `workspace/`. None are vendored.
Check each source's Apache-2.0 licence at its fixed commit. Fetching is a manual
calibration setup action. The product gains no downloader or dependency.

## Reproduce the probe

Use Python 3.9 or later, the pinned Crossplane CLI and yq v4.47.1. Their SHA-256
values must match `pins-r1.json` and the local manifest. Provide a local JSON
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

Between the two native probes, run the committed control from within this module:

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
