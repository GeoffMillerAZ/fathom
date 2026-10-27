# Local provider-schema feasibility probe

This is calibration tooling for IAC-2, not a product adapter or a language reading.
`protocol.json` fixes the authored control population and metric definitions;
`pins.json` fixes all three executable/archive identities before schema inspection.
AWS reuses the existing Terraform oracle pin **from the OpenTofu re-release**.
The immutable `pins.json` incorrectly attributes its AWS URL fields to HashiCorp;
[`pins-errata.json`](pins-errata.json) supplies the corrected source URLs and
checksum-manifest evidence. Use the erratum for AWS downloads; do not rewrite
the frozen pins or substitute the HashiCorp build. Their schema equality is
unmeasured. AzureRM and Google use the highest
stable releases in their official indexes at the recorded selection time.

Keep all downloaded artifacts, provider exports, native output and capture roots
under an ignored cache or `workspace/`. Download only the exact pinned archive URLs
using the AWS override above, and check the SHA-256 values before extracting
executables. For AWS, this is an exact download and verification recipe (choose
a new download directory if it already exists):

```sh
set -e
mkdir workspace/artifacts/provider-schema-aws-download
curl --fail --location --output workspace/artifacts/provider-schema-aws-download/aws.zip \
  https://github.com/opentofu/terraform-provider-aws/releases/download/v6.67.0/terraform-provider-aws_6.67.0_darwin_arm64.zip
printf '%s  %s\n' \
  25afdd7ee3235adec8c96ee2b12b9abecc00b56a1b8505acc6f2889798240d04 \
  workspace/artifacts/provider-schema-aws-download/aws.zip | shasum -a 256 -c -
```

AzureRM, Google and OpenTofu use their exact archive URLs and SHA-256 values in
`pins.json`. The harness separately verifies the installed provider executable
hash, so a ZIP checksum alone does not replace that check. The operator's private
input manifest has this shape:

```json
{
  "platform": "darwin_arm64",
  "tool": {"path": "<local-tofu>", "sha256": "<pinned-binary-hash>"},
  "providers": [{
    "name": "aws",
    "version": "<pinned-version>",
    "archive": "<local-provider-zip>",
    "archive_sha256": "<pinned-archive-hash>",
    "binary_sha256": "<pinned-binary-hash>"
  }]
}
```

Include exactly one entry each for `aws`, `azurerm` and `google`. Use absolute
local paths in that private manifest. No private paths or export contents belong
in committed receipts. The harness downloads nothing. From the repository root:

```sh
PYTHONPYCACHEPREFIX=workspace/tmp/pycache nice -n 10 python3 -m unittest discover -s calibration/oracles/terraform-provider-schema -p '*_test.py'
nice -n 10 python3 calibration/oracles/terraform-provider-schema/probe.py \
  --inputs workspace/artifacts/provider-schemas-probe-r1/preparation-inputs.json \
  --scratch workspace/tmp/ps1 \
  --out workspace/artifacts/provider-schemas-probe-r1/native-receipt.json
PYTHONPYCACHEPREFIX=workspace/tmp/pycache nice -n 10 python3 \
  calibration/oracles/terraform-provider-schema/summarize_exports.py \
  --capture workspace/tmp/ps1 \
  --receipt workspace/artifacts/provider-schemas-probe-r1/native-receipt.json \
  --out workspace/artifacts/provider-schemas-probe-r1/export-sizes.json
```

Use a fresh scratch/output path for each run; existing output is refused. The
Darwin-only harness requires `sandbox-exec` to deny outbound IP connections and
proves that denial against a real local TCP listener. **System DNS resolution
remains allowed**: the host resolver can emit DNS queries. This profile does not
prove DNS-free execution; the historical receipt is not relabeled as a stronger
sandbox. It supplies a filesystem-only
provider mirror, empty HOME, no inherited credentials and disabled backend access.
Only native initialization, schema export, configuration display and validation
run. No plan, state, apply, provider Configure call or cloud API is requested.
All native commands run serially, with a minimum nice priority of 10 and launch
load at most 9; high load waits. Time/output limits refuse a whole command.

Four output traversals per provider cover a resource scalar, an aliased resource
scalar, a data-source scalar and a resource map element. Each field is mutated
separately. Native traversal enumeration and field-validity diagnostics are
reported separately; neither proves all alias, module, dynamic-index or function
bindings. Native extra reference strings remain counted. Repeated schema and
configuration outputs test raw-truth conservation after restoring each fixture.
Authored controls are not public subjects, holdouts or an accuracy denominator
for a support level. Failure results remain visible for all three providers.

Whole-export sizes and explicit schema counts are measurements. Schema versions
are not provider versions. Missing namespace metadata, usable declaration anchors
and actual store costs cannot be inferred from native schema bytes. The future
dependency adapter and Terraform L4 policy are still prerequisites; this probe
persists no schema declaration or typed edge.


## Historical validation replay

Raw `validate -json` diagnostic order is unstable. Compare the added canonical
hash, which sorts complete diagnostic objects by severity, range and summary,
with a full-object tie-break. Keep raw hashes as capture identities, not a
byte-equality reproduction requirement. Schema/show raw conservation is unchanged.
The original native receipt and size receipt remain immutable historical outputs.
The corrected selector binds all three schema/show pairs to the native receipt,
including byte counts and successful launch records, before writing any output.
Its output adds receipt/show hashes and therefore has a new digest.

```sh
PYTHONPYCACHEPREFIX=workspace/tmp/pycache nice -n 10 python3 \
  calibration/oracles/terraform-provider-schema/replay_validation.py \
  --capture workspace/tmp/ps2 \
  --receipt docs/assessments/data/2026-10-26-provider-schema-native-probe.json \
  --compare-capture workspace/tmp/ps1 \
  --out workspace/artifacts/provider-schemas-probe-r2/validation-replay.json
PYTHONPYCACHEPREFIX=workspace/tmp/pycache nice -n 10 python3 \
  calibration/oracles/terraform-provider-schema/summarize_exports.py \
  --capture workspace/tmp/ps2 \
  --receipt docs/assessments/data/2026-10-26-provider-schema-native-probe.json \
  --out workspace/artifacts/provider-schemas-probe-r2/export-sizes-bound.json
```

The optional comparison capture checks restored fixture and schema/show identity;
its exit status is borrowed from the final receipt, not newly observed. These
commands reread private historical captures and launch no provider. Require load
<=9 and nice >=10 even for tests/replays, and serialize heavy work. Fresh native
runs use a new scratch/output path and independently verify all supplied artifacts.
