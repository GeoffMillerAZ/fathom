# AWS CDK: cheap oracle-first triage

2026-10-26. Main `01ea58c6b`.

This separate note **feeds IAC-20 and GAP1-37 but does not satisfy them**. The canonical combined assessment `docs/assessments/2026-10-26-iac-oracle-first-triage.md` remains absent. Under the current NP-Q5 deferred scope, each new native or language route is **needs-owner-approval (untried)**. No measured ceiling or new grade follows.

The existing oracle pins `aws-cdk` CLI 2.1144.0 and runs `cdk context --json -vv`, which reads `cdk.json` without starting the app. Its entry-program reference comes from parsing the app command and remains tier C. The pinned `aws-samples/aws-cdk-examples` source is sparse to `cdk.json`, so it cannot supply a runnable app. Source and protocol hashes are in the [receipt](data/aws-cdk-triage-2026-10-26/receipt.json).

| Candidate | Potential output | Disposition |
|---|---|---|
| Existing context oracle | Configuration values without app execution | Tried by the existing instrument; no new attempt |
| Owner [`cdk synth`](https://docs.aws.amazon.com/cdk/v2/guide/ref-cli-cmd-synth.html) | Cloud Assembly, stack templates, and manifest from an audited full app | needs-owner-approval; untried |
| Construct [`tree.json` and metadata/debug traces](https://docs.aws.amazon.com/cdk/api/v2/docs/aws-cdk-lib.AppProps.html) | Possible construct/source mapping when `treeMetadata` and `stackTraces` are enabled | needs-owner-approval; untried |
| Ordinary TypeScript `tsc` + `scip-typescript` route | Static program symbols and references, without certifying runtime stacks | needs-owner-approval; untried |

The read-only inventory includes hidden and ignored directories in scoped cache tools, direct subject roots, and workspace. It emits role names and aggregate counts, raises on unreadable directories, and records missing roots. At the measured roots: Node and npm are on PATH; zero CDK executables or installed `aws-cdk`, `aws-cdk-lib`, or `yaml` package trees in tools; zero `cdk.json` files in subject roots; zero `cdk.out` manifests. These counts do not establish machine-wide absence. Opaque npm tar cache entries are not counted as installed packages. Set `IAC_CACHE` to the private lane cache root before reproducing:

```sh
python3 -I calibration/oracles/aws-cdk-triage/inventory.py --cache-root "$IAC_CACHE"
```

**Non-executable future command shape:** `<pinned-cdk> synth --no-lookups --output <guarded-scratch-assembly>` from `<audited-app-copy>`. A runnable attempt requires a full pinned example and language dependencies, audited app and cached context, verified OS-level network denial, an empty scrubbed HOME and environment without cloud credentials, read-only source/cache, and nice/load/time/output limits. `--no-lookups` blocks CDK context lookup, not arbitrary app code.

Owner documentation checked 2026-10-26. [AppProps](https://docs.aws.amazon.com/cdk/api/v2/docs/aws-cdk-lib.AppProps.html) documents `treeMetadata` and `stackTraces`/`aws:cdk:trace`. These are candidates for construct/source positions, not measured binding or completeness; inspect support in the selected SDK/CLI before execution. No complete oracle search or ceiling is claimed. The IaC lane owns the remaining HCL-tool-config/Packer triage under IAC-20/GAP1-37; it remains open outside this I-R5 three-item pass, with separate approval required for Packer plugins.
