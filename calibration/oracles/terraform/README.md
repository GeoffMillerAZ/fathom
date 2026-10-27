# Terraform native calibration

`pins.json` fixes the two Git subjects, OpenTofu 1.13.0, eight provider packages,
and ten exact registry module versions across nine module repositories. The provider versions
were selected before this rerun. Only AWS 6.67.0 is identified in the prior
reading; the other seven versions are newly pinned compatible inputs, not
proven copies of the old mirror. The module versions match the subjects' major
ranges and were available by the prior reading date.

Run the preparation in a serialized low-load slot, with an operator-owned
cache outside the checkout. It uses Git fetch and curl, verifies full subject
and module commits and provider ZIP checksums, and writes a private summary
receipt at `$CACHE/terraform-native-preparation.json`:

```sh
python3 calibration/oracles/terraform/prepare.py --cache "$CACHE" \
  --step tool --step subjects --step modules --step providers
```

Run the tracked-set manifest command in `docs/design/language-breadth.md`
§13.3.1 on each `git-subjects/` checkout before and after native commands.
Set its `RECEIPT` to `$CACHE/native/manifests/{eks,vpc}-{before,after}`
for the appropriate subject and phase. The command writes the `.json`
summary required by `run_native.py`.
`manifests.py --cache "$CACHE" --phase before` (and later `after`) extracts
and executes that exact committed Python recipe, with no duplicate manifest
algorithm, and rejects a changed tracked set.
The older archive extracts are not Git checkouts and cannot supply this proof.
Keep full manifests and untracked lists local; publish counts and digests only.

Build the separate oracle and `preseedscan` modules into `$CACHE/tools/`
from the repository root. Use an absolute cache path. Fetch Go module
dependencies once before the read-only test and build commands:

```sh
mkdir -p "$CACHE/tools"
(cd calibration/oracles/terraform && go mod download && go test -mod=readonly -race -count=1 ./... && go build -mod=readonly -o "$CACHE/tools/terraform-oracle" .)
(cd calibration/oracles/terraform/preseedscan && go mod download && go test -mod=readonly -race -count=1 ./... && go build -mod=readonly -o "$CACHE/tools/terraform-preseedscan" .)
python3 calibration/oracles/terraform/smoke_preseed.py \
  --cache "$CACHE" --scanner-bin "$CACHE/tools/terraform-preseedscan" \
  --oracle-bin "$CACHE/tools/terraform-oracle"
```

`preseed_modules.py` checks the scratch `.tf` path set and bytes against the
clean pinned subject, removes every cached local manifest record, writes only
root and exact pinned registry records, then runs plain
`tofu init -backend=false -input=false` in each copied root. If installing a
local ancestor invalidates pinned registry descendants, the helper keeps the
new native local records, restores only missing exact pinned registry records
and their verified checkout symlinks, and retries within the 16-ancestor
bound. A failed partial install never feeds the oracle. OpenTofu supplies every
local module record. The helper compares native keys, sources, versions and
destinations to its separate proposal;
any difference rejects the whole subject. It records each native manifest
hash and local agreement count. It does not rewrite subject files. Subject-local
destinations are compared lexically. For a local child of
a pinned registry module, OpenTofu may write the physical checkout path in
place of its cache symlink; only equal resolved targets pass, and the receipt
counts these aliases separately. The native `Dir` is never rewritten. The oracle
uses `tofu init -get=false` only after native installer preflight. Then
run `run_native.py --phase before` and later `--phase after`. It uses a
dedicated scratch directory per subject under `$CACHE/native/`, the packed
`$CACHE/mirror`, and the checksum-verified OpenTofu binary. The same scratch
directory is reused across phases; preflight checks native manifests again.
The oracle fails if any root fails or any native definition cannot be located.
Provider installation is mirror-only. It invokes
`tofu graph -type=plan`, but never `tofu plan`, `tofu apply`, or a cloud API.
The wrapper compares source manifests, measured-file list hashes and bytewise
raw truth hashes across phases. Verify each raw JSON before grading:

```sh
python3 calibration/oracles/terraform/run_native.py \
  --cache "$CACHE" --oracle-bin "$CACHE/tools/terraform-oracle" \
  --scanner-bin "$CACHE/tools/terraform-preseedscan" --phase before
python3 calibration/oracles/terraform/verify_truth.py \
  --subject terraform-aws-modules/terraform-aws-eks --oracle "$CACHE/native/eks-before.json"
python3 calibration/oracles/terraform/verify_truth.py \
  --subject terraform-aws-modules/terraform-aws-vpc --oracle "$CACHE/native/vpc-before.json"
python3 calibration/oracles/terraform/prepare.py --cache "$CACHE" --step audit-modules
python3 calibration/oracles/terraform/probe_independence.py \
  --cache "$CACHE" --oracle-bin "$CACHE/tools/terraform-oracle" \
  --tofu "$CACHE/tools/tofu-1.13.0" --mirror "$CACHE/mirror"
```

The floors are EKS 866 and VPC 2662 cross-file truth edges, 3528 combined.
`prepare.py --step audit-modules` rejects unexpected selected registry module
versions. Preserve the raw truth JSON, module manifests, oracle logs and
subject manifests locally. Compare baseline and after-fix readings only on
identical pinned inputs and disclose per-subject denominators and misses.
`probe_independence.py` checks that unrelated JSON cannot affect native truth
and that a source fact can. It does not prove extractor independence; that
requires an actual indexbench extractor-output perturbation against the same
fixture and unchanged native truth digest.

The original comparison used clean, detached checkouts of baseline
`6db920e72898abb20bf86c22652b401902e34598` and the first repaired engine
`2ff8b2ac3d188268c52bff320eddf93242b8248a`. Its baseline grade and native
truth remain unchanged. Build the final engine commit
`df20b1f1810c96dde9c00d919166f44746376993` in a clean Git checkout,
then run the three-sample landing grade against the same native truth:

```sh
(cd "$LANDING_CHECKOUT" && go build -trimpath -o "$CACHE/tools/indexbench-terraform-landing-r3" ./cmd/indexbench)
python3 calibration/oracles/terraform/grade.py \
  --cache "$CACHE" --landing-only \
  --repaired-commit df20b1f1810c96dde9c00d919166f44746376993 \
  --repaired-bin "$CACHE/tools/indexbench-terraform-landing-r3" \
  --repaired-cwd "$LANDING_CHECKOUT" --runs 3
```

`grade.py` checks embedded Go build VCS identity, runs the landing engine three times
against the same native JSON, requires 87/523029 and 77/453543 measured files
and bytes and 866/2662 native cross-file denominators, and selects the median
CPU-throughput run. The original `comparison.json` and the landing report stay
under `$CACHE/native/grades`.
After `run_native.py --phase after` confirms bytewise raw truth identity,
publish the retained baseline, landing grade and final native installer receipts:

```sh
python3 calibration/oracles/terraform/publish.py \
  --cache "$CACHE" \
  --landing-report "$CACHE/native/grades/landing-df20b1f18.json" \
  --out docs/assessments/data/terraform-two-hop-2026-10-26/summary.json
```

The publisher checks both phases' tracked sets, measured sets and raw truth
against the grade hashes. It publishes only counts and hashes. The raw subject
manifests and file lists stay private.
