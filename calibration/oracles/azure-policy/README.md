# Azure Policy: independent reading and open native experiment

The existing standard-library JSON walk emits C definitions and parameter
references. The canonical reading remains 55,711 matched definitions out of
55,714 emitted and 55,711 truth, and 21,137/21,137 references: L1/L2 good.
The walk's former assertion that no Azure tool evaluates policies offline is
withdrawn. No reading, tier or generated tracker changes in this audit.

## Native route found, not yet executed

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
129,976,447 uncompressed bytes. It has not been extracted or run here.

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

Next, while respecting the 20 GiB disk floor and the serial/load limit, pin
the actual native payload and test it in isolation with no Azure login,
external requests or telemetry. Inspect native parsing/evaluation interfaces
for a complete population of definition/parameter uses before comparison;
positions may be classified separately, with their authority disclosed.
Require authored invalid-binding, unused-declaration, namespace and identity
controls. A native success/error on pairs chosen by this JSON walk alone can
attest validity, but cannot prove A recall. Keep every original subject and
family in scope, and classify native refusals explicitly. Whole-population
measurement remains open; ARM/Bicep cross-file work awaits D19 authorization.
