# Azure Policy: independent reading and open native experiment

The existing standard-library JSON walk emits C definitions and parameter
references. The canonical reading remains 55,711 matched definitions out of
55,714 emitted and 55,711 truth, and 21,137/21,137 references: L1/L2 good.
The walk's former assertion that no Azure tool evaluates policies offline is
withdrawn. No reading, tier or generated tracker changes in this audit.

## Native route found; Linux x64 run pending

Microsoft's `AzurePolicy.azurepolicyextension` 0.2.6 (MIT) contains a
self-contained `osx-arm64/PolicyLanguageServer`, .NET/ASP.NET 9.0.12 runtime
metadata, and native policy-expression/evaluation assemblies. Its JavaScript
selects the architecture at runtime and posts policy/resource JSON to the
server's loopback `evaluatePolicy` endpoint. The bundled XML documents
`PolicyEvaluationEngine.EvaluateAsync`, native policy-definition parsing and
parameter-type diagnostics. It also documents the cache-folder command.

The [Marketplace page](https://marketplace.visualstudio.com/items?itemName=AzurePolicy.azurepolicyextension)
still describes an x64 limitation; the pinned package actually includes ARM64
payloads and architecture-selection code. That old platform statement does
not reject this candidate. The UI's sign-in and alias-cache setup does not
prove the native evaluator requires an Azure connection. Its offline behavior
and any reference/position API remain unmeasured. Protocol method strings in
the bundled generic LSP client are not evidence of server capabilities.

`extension-source-r1.json` pins the versioned archive directory and four
source/metadata entries by SHA-256. These are selective source identities,
not a checksum or execution receipt for the complete native distribution.
The native ARM64 directory totals 455 entries, 49,565,609 compressed bytes and
129,976,447 uncompressed bytes. It was prepared and probed here. The ARM64 wrapper bundles AMD64 policy
libraries; the official ARM64 .NET 9.0.12 host reaches managed Main but fails
to load them. Direct apphost and hostfxr attempts terminate before output.
No native evaluation or complete binding population has been measured.
T-R9 directs the staged [Linux x64 probe](LINUX-X64-PROBE.md) to a controller
runner; no Rosetta or global installation is authorized.

## Reproduce the bounded source audit

```sh
python3 -B -m unittest discover -s calibration/oracles/azure-policy -p test_audit_extension.py
python3 -B calibration/oracles/azure-policy/audit_extension.py -out workspace/tmp/azure-policy-extension-audit.json
```

The driver reads only the pinned ZIP directory and those four entries using
HTTP ranges. It refuses a full-response fallback, mismatched ranges, excessive
sizes, directory drift, CRC failures and source-digest drift. It neither
extracts native binaries nor activates VS Code, authenticates, requests Azure
resources, calls a paid service or starts a build. Private raw package
inventories remain scratch; committed results are counts and digests.

## Work still required

This audit does not close X-R4 or establish a new ceiling. Existing checks
include the owner's extension/toolchain above, the Sourcegraph SCIP and precise
navigation catalogues, LSIF.dev, and the recorded GitHub `scip-azure-policy`
repository-name search (zero hits). Those searches do not reject the vendor
server. Registry parser/typechecker candidates and runtime tracing routes need
their own versioned audits before any renewed ceiling claim. Historical
EnterprisePolicyAsCode 11.5.8 property reads and Bicep validation were checked
by the prior assessment; neither supplied positioned parameter-use bindings.

Next, run the pinned Linux x64 probe on the controller-provided runner.
Both native payloads and the ARM64 .NET host have been measured for identity
and removed locally after receipts were packaged. Inspect native parsing/evaluation interfaces
for a complete population of definition/parameter uses before comparison;
positions may be classified separately, with their authority disclosed.
Require authored invalid-binding, unused-declaration, namespace and identity
controls. A native success/error on pairs chosen by this JSON walk alone can
attest validity, but cannot prove A recall. Keep every original subject and
family in scope, and classify native refusals explicitly. Whole-population
measurement remains open; ARM/Bicep cross-file work awaits D19 authorization.

## Prepared native experiment

`native-protocol-r1.json` is preregistered before native extraction/execution.
`prepare_native.py` retrieves only the bounded ARM64 archive span, verifies the
whole extracted unit before publishing it, and records its manifest/apphost
SHA-256. It executes no vendor code. Commit that preparation identity before
the separately isolated startup/API probe. Under the serial/load guard, after
checking `df -h ~` and `uptime`:

```sh
nice -n 10 python3 -B calibration/oracles/azure-policy/prepare_native.py -cache workspace/oracle/azure-policy-native
```

The command also checks the 20 GiB disk floor internally. Tests exercise a
bad CRC, escaping path, absent apphost, 513 entries against the 512-entry cap,
64 MiB + 1 single-file expansion and 160 MiB + 1 total expansion. A preparation
receipt proves neither native evaluation nor reference completeness.


## T-R11 staged runner (binding review fixes)

The current startup protocol is native-protocol-r2.json and
[LINUX-X64-PROBE.md](LINUX-X64-PROBE.md). The controller copies the staged
probe-linux-x64.workflow.yml to a disposable push-triggered run branch, never
merged. It requires a GitHub-hosted ubuntu-22.04 x64 job, no container, no
persisted checkout credentials and cache/output outside checkout. Only the
outer-validated receipt.json is uploaded; raw native/LSP/evaluation/stderr
outputs stay on the runner and are deleted. The ARM startup/PE-header reports
are explicitly unreceipted local observations, not calibration proof. The
current C reading and deferred whole-population work remain unchanged.

Round-3 filesystem controls give the vendor a private verified payload copy,
HOME and TMP under one writable scratch mount. Reachable mounts outside
scratch are remounted read-only. Capability dropping
and EROFS checks of the checkout, runner directories, cache and launcher use
plain, `/..` and `/proc/..` spellings before execution. Only successful checks
set the readonly and dot-dot-denial receipt flags. Round 2's root clone and
chroot claim is withdrawn: it allowed a writable `/..` route. No root clone or
chroot remains. A real namespace regression is required on the Linux runner;
local ARM tests skip that one test and establish no Linux execution proof. The fixed
run branch, Node 24 action pin and dated runner-successor condition are in
LINUX-X64-PROBE.md. Local mocked tests grant no Linux isolation proof.

T-R12 supplies a verbatim 29-line public ubuntu-22.04 runner mountinfo fixture
(run 40394016064, image ubuntu22/20261012.322, kernel 6.8.0-1064-azure).
Regression tests cover its topmost binfmt mount and a clearly authored fresh
procfs overmount. A captured ubuntu-24.04 uid_map refusal is replayed as a
negative control. Source, scope and limits are in LINUX-X64-PROBE.md; full
Linux kernel regression and vendor startup still await the controller run.

T-R13 follow-up: the first actual T-R11 run **40407284519** passed isolation
and executed the vendor, then failed initialization with SIGXFSZ (-25), empty
stderr/LSP and no file attribution. Its unchanged receipt and v2 schema are
committed; it grants no API, calibration or tier result. The preceding pending-run
statements describe earlier checkpoints. T-R13a stages one 256 MiB vendor-child
rerun, preserving 8 MiB output captures and a 128 KiB receipt bound. It adds
scratch-relative HOME/TMP write metadata, the largest final file, exact
EFBIG/SIGXFSZ attribution where observed, and a mandatory authored Linux
tracing regression. N3-m1 refuses a stacked namespace root. See
[the current runner contract](LINUX-X64-PROBE.md#t-r13a-rerun-finite-child-bound-and-write-attribution).
No new native API success, reading or tier is claimed before that rerun.
