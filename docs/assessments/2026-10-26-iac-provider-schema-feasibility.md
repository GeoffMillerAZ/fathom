# Provider-schema feasibility probe

Measured 2026-10-26 for IAC-2, on main `4e7eea67a` plus the design/probe branch.

## Verdict

All three pinned providers exported schemas offline. Native configuration display
enumerated all twelve authored output traversals. Native validation accepted the
three baselines and rejected all twelve individually renamed fields with one
attributable error each. Repeated schema and configuration outputs are byte-identical.
This establishes feasibility for these controls, not a language-support grade.

The implementation remains blocked by IAC-16's tooling-L4 policy and
IAC-12/SG-36/SG-37's dependency representation, source-anchor and budget decisions.
All three exports exceed the planned 2 MiB dependency-file cap; AWS and Google
also exceed the 8 MiB package cap. No schema reader, declaration store or typed
edge is implemented. This dated feasibility file intentionally leaves the IAC-2 calibration path
absent and does not satisfy its lexical proof contract. IAC-2 remains incomplete:
there are no public subject grades,
actual store-byte measurements or product-output independence control yet.

## Frozen inputs and population

The [protocol and pins](../../calibration/oracles/terraform-provider-schema/README.md)
were committed before the first provider-schema execution. The exact authored
fixture hashes were independently pinned before execution as well. AWS 6.67.0
and OpenTofu 1.13.0 reuse the committed Terraform oracle pins. AzureRM 5.11.0 and
Google 8.8.1 were selected by highest stable SemVer in each official release index
at the recorded fetch time, before archive inspection. AzureRM and Google archive
checksums match their HashiCorp downloads. AWS uses the OpenTofu re-release, not
the HashiCorp build. The native installer's executable bytes match the pinned
binary digests. The original pins' AWS URL fields were misattributed; the
[provenance erratum](../../calibration/oracles/terraform-provider-schema/pins-errata.json)
supersedes those source fields without changing the frozen hashes.

Every fixture has two resources (one using a provider alias), a data source, and
four outputs: resource scalar, aliased-resource scalar, data-source scalar and
resource map element. All twelve outputs are counted. These are authored controls,
not real public subjects, canonical corpus additions or independent holdouts.
No difficult subject or failed control was removed.

The capture uses a filesystem-only mirror under OpenTofu's
`registry.opentofu.org/hashicorp/*` addresses. The archive sources are mixed: AWS from the OpenTofu GitHub re-release;
AzureRM and Google from the HashiCorp URLs in the pins. The HashiCorp AWS
darwin_arm64 archive has SHA-256 `25e03c7f025ff4851889537605aa560d2e39bd738b33a5204b8037eb164b475f`
and was not run. Schema equality between the AWS builds is unmeasured. Neither
source attribution nor a mirror address certifies equivalence to an independently
queried registry catalog. Schema-export format 1.0 has no provider-release identity.
The version association here comes from the controlled native installation and
binary hash, not from a resource schema's internal `version` number.

## Native binding candidates (feasibility only)

| Native candidate | AWS | AzureRM | Google | Scope and limit |
|---|---:|---:|---:|---|
| `tofu show -json -config`: exact authored traversals emitted | 4/4 | 4/4 | 4/4 | Native traversal enumeration, including the literal map key; no exhaustive typed-binding certificate |
| All native output reference strings | 9 | 9 | 9 | Each has five extra address/attribute prefixes; all remain counted separately |
| Native provider configurations / alias configurations | 2 / 1 | 2 / 1 | 2 / 1 | Three resource/data mappings and one alias-resource mapping per fixture; no multi-root or module-provider proof |
| `tofu validate -json`: baselines valid | 1/1 | 1/1 | 1/1 | Static field existence/type diagnostics on these authored inputs |
| Individually renamed fields rejected with an attributable error | 4/4 | 4/4 | 4/4 | Each error names the injected field at its source range; unrelated failures do not count |

The candidates are native for their reported facts. The custom accounting and
selected-attribute JSON proxy are not native binding oracles. Neither candidate
has been graded for support precision/recall on public subjects. Dynamic indexes,
ambiguous roots, incompatible provider versions, module provider mappings,
provider functions and newer namespaces remain ungraded. There is no exhaustive
oracle ceiling. The versioned [schema contract](https://opentofu.org/docs/cli/commands/providers/schema/),
[configuration display](https://opentofu.org/docs/cli/commands/show/) and
[validation contract](https://opentofu.org/docs/cli/commands/validate/) define the
native commands' scopes; no plan or apply was run.

AWS's baseline includes one deprecation warning, retained in the receipt. AzureRM
and Google have none. Each mutation has exactly one attributable error and zero
unattributable errors; warnings are counted separately. Raw validation diagnostic
order is unstable, so raw hashes identify a capture rather than define repeatable
byte equality. The repaired accounting adds a canonical hash by sorting complete
diagnostics by severity, range and summary, with a full-object tie-break. The
[historical replay](data/2026-10-26-provider-schema-validation-replay.json) binds
the fifteen final validation streams to the original receipt, checks restored
source and schema/show identities, and compares canonical hashes with the first
capture. All fifteen canonical hashes agree between captures; only thirteen raw hashes
agree, with AWS mutation1 and mutation3 differing by diagnostic order. No
provider is executed by this replay. Comparison exit statuses are
borrowed from the final receipt, not observed anew.

## Native schema population and representation costs

| Provider | Export bytes | Resource schemas | Data-source schemas | Explicit attributes in examined blocks | Selected attribute JSON bytes |
|---|---:|---:|---:|---:|---:|
| AWS 6.67.0 | 19,517,134 | 1,728 | 683 | 117,264 | 739 |
| AzureRM 5.11.0 | 3,204,662 | 1,107 | 397 | 28,209 | 699 |
| Google 8.8.1 | 9,782,236 | 1,381 | 472 | 37,172 | 833 |

Attribute counts include explicit attribute entries in the provider, resource
and data-source blocks, recursively through nested blocks. They are not a count
of all object-type members or every namespace's declarations. The custom cost
proxy serializes the three distinct selected attribute objects and their source,
namespace, type, field and schema-version identifiers per provider. It retains
native attribute metadata. Four references share three attribute objects across
two resource/data types. These byte counts are neither stored rows nor actual
store bytes, and exclude admitted anchors, indexes, graph links and transactions.
There is no measured per-service or cross-service deduplication cost.

| Additional native namespace | AWS | AzureRM | Google | Disposition |
|---|---:|---:|---:|---|
| Functions | 4 | 2 | 6 | Metadata present; function bindings ungraded |
| Ephemeral resource schemas | 11 | 2 | 6 | Present, unsupported by this control population |
| Resource identity schemas | 540 | 198 | 942 | Present, unsupported by this control population |

All other map sections are enumerated in the raw-count receipt. Their absence
in these three exports is version-specific, not general support. Each raw export
has one physical line. Usable declaration anchor counts remain unmeasured;
no accepted JSON-pointer/byte-range source contract exists in the product.
The tiny selected-JSON proxy does not justify raising whole-source caps or
creating a duplicate declaration store. IAC-12 needs an explicit ecosystem-cap,
anchor and representation decision through the dependency plane first.

## Reproduction, controls and limits

Follow the committed [setup and commands](../../calibration/oracles/terraform-provider-schema/README.md).
The [native receipt](data/2026-10-26-provider-schema-native-probe.json) is the
historical harness's exact output bytes from `ed0081bb0`, not a rerun of the
repaired accounting tools. The [size receipt](data/2026-10-26-provider-schema-export-sizes.json)
is the separate custom summarizer's exact output. Raw exports, authored capture
roots, installer files and native diagnostics stay private. Public evidence has
counts, hashes, technical provider identities and bounded outcomes only.
Original pins, protocol, native receipt, size receipt and verification receipt
are preserved byte-for-byte. The corrected selector's
[receipt-bound size output](data/2026-10-26-provider-schema-export-sizes-bound.json)
adds explicit native-receipt/show identities; its metric values are compared
with the original output before publication.

All 32 final native launches ran serially at verified nice 10, launch load
2.76 through 3.58, with GOMAXPROCS=2. The historical OS profile denies direct outbound IP
connections and first proved TCP denial against a real local listener. It leaves
name resolution through the system resolver available, which can make the host
emit DNS queries. No DNS-denial or DNS-free execution claim is made for this
capture. Provider plugin Unix RPC remains available. HOME is empty, credentials are not inherited, proxies are dead,
and backend access is disabled. No cloud call, state read, plan or apply occurs.
The scope is Darwin arm64; this is not Linux native-runtime evidence.

Each mutated source is restored to its independently pinned bytes. Native schema
and show output hashes agree before and after all four mutations for each
provider. That is source sensitivity and restored-source conservation, not the
unperformed product-extractor perturbation. The first capture's parser expected
a wrapped configuration object and failed all three accounting checks. The
actual native output is unwrapped. A failing-then-passing regression corrected
that parser, and the same fixture population was rerun in a fresh directory;
the initial failed capture remains private. The [verification receipt](data/2026-10-26-provider-schema-probe-checks.json)
records its hash and the final commands. No native failures were relabeled.

The original ten offline harness tests, including the independent fixture-hash
regression, covered
artifact substitution, wrong populations, stale output, unrelated errors,
exact traversal accounting, priority, timeout and output-cap refusals. No
reading, queue, generated board, plan checkbox or schema persistence changes.
The [repair check record](data/2026-10-26-provider-schema-repair-checks.json)
records twenty passing tests and the new replay results separately. Diagnostic
ordering, output-clobber and capture-binding mutants each fail the corresponding
focused tests; the unmutated suite passes. Original selector metric fields are
conserved exactly. The controller owns full
merged-head validation, verification and CI.
