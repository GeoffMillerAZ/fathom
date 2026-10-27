# Local provider-schema feasibility probe

This is calibration tooling for IAC-2, not a product adapter or a language reading.
`protocol.json` fixes the authored control population and metric definitions;
`pins.json` fixes all three executable/archive identities before schema inspection.
AWS reuses the existing Terraform oracle pin. AzureRM and Google use the highest
stable releases in their official indexes at the recorded selection time.

Keep all downloaded artifacts, provider exports, native output and capture roots
under an ignored cache or `workspace/`. Download only the exact pinned archive URLs
and check the SHA-256 values before extracting executables. The operator's private
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
PYTHONPYCACHEPREFIX=workspace/tmp/pycache python3 -m unittest discover -s calibration/oracles/terraform-provider-schema -p '*_test.py'
nice -n 10 python3 calibration/oracles/terraform-provider-schema/probe.py \
  --inputs workspace/artifacts/provider-schemas-probe-r1/preparation-inputs.json \
  --scratch workspace/tmp/ps1 \
  --out workspace/artifacts/provider-schemas-probe-r1/native-receipt.json
PYTHONPYCACHEPREFIX=workspace/tmp/pycache nice -n 10 python3 \
  calibration/oracles/terraform-provider-schema/summarize_exports.py \
  --capture workspace/tmp/ps1 \
  --out workspace/artifacts/provider-schemas-probe-r1/export-sizes.json
```

Use a fresh scratch/output path for each run; existing output is refused. The
Darwin-only harness requires `sandbox-exec` to deny outbound IP connections and
proves that denial against a real local TCP listener. It supplies a filesystem-only
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
