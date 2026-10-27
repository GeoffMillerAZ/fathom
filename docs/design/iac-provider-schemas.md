# IaC provider schemas: local evidence before typed claims

Design for issue #55, approved by the controller and merged as PR #59. The
current provider-schema follow-up is design and probe only, based on main
`4e7eea67a`. It changes no reading or product behavior and authorizes no schema
reader, declaration persistence or typed edge. The policy and dependency-plane
prerequisites below remain open.

Owner request: "let's add the major terraform providers and pulumi providers and
crossplane providers if applicable ... the providers for aws, azure and gcp, no?"

The unit of support is the configuration that uses a provider. Provider schemas
are evidence inputs, not languages or new board entries. Terraform/OpenTofu L3
comes first at T3, target L3 (I-R1 amended; GAP1-32). L4 is a separate measured result, never inferred from loading
a schema. Kustomize joins this lane under I-R4. ADR-048 currently limits tool formats
to L1-L3; Terraform is catalogued as tooling. The owner-requested L4 design
therefore needs an explicit grading-policy amendment before any L4 queue or
reading claim, even if the implementation proves native typed bindings.

## Binding intent and fired triggers

- G15 and ADR-048 require a committed, oracle-graded reading for a support level.
  I54 forbids a level from registration alone. Owner ruling D19 (#7) applies
  each L3 metric's own denominator floor of 20, but the harness on this main
  does not yet enforce it. Kustomize's historical 13/13 is not measured under
  that ruling; IAC-3 and IAC-17 enforce the floor in their own contracts.
- I56 and ADR-050 allow read-only local dependency inputs and forbid autonomous
  downloads. I14 bounds egress. The product runs no init, schema command, provider
  executable, registry client or cloud operation on the user's behalf.
- I3 requires repository-relative source anchors. I24 requires counted target
  exclusions. I1 and P2 require deterministic output. I47-I53 and P23 require
  bounded ingest work, cancellation and whole-input refusal at limits.
- I62 and ADR-073 keep judgments out of code facts. No decision model resolves a
  field, chooses a provider identity or upgrades an evidence tier.
- The lane's table freeze forbids a new table, column, memo file or background job.
  Existing store-v0 fields and edge kinds keep their meanings. This proposal adds
  no public tool, command, configuration key or compatibility change by itself.
- Crossplane's explicit schema-fetch trigger has fired on the owner's request.
  A Development runtime can use a local process; runtime evaluation remains
  outside this provider-schema probe. The former container-only premise is
  superseded.
- Kustomize's directory-preference trigger has already been addressed in code:
  `oneDocFile` prefers the referring dialect, and
  `TestKustomizeBaseDirectoryNeedsTheOneFileRule` includes Kubernetes manifests.
  The provenance's future-tense warning is stale. Verify it, then correct the
  provenance only after I-R0 opens. Larger real component/overlay evidence is
  still needed.
- F25 (infrastructure migration conservation) remains deferred: nobody requested
  a state migration. Keep provider and schema identity in evidence; build no
  migration engine or future dependency database for this lane.

## Delivered prerequisite and remaining schema boundary

The Terraform/OpenTofu two-hop module-output change landed as PR #85 in v0.23.0.
`terraformModuleOutputIntents` carries explicit module/output intent from sibling
files; `resolveTerraformModuleOutputs` resolves the bounded caller-directory,
local module-source and output join. Plain lexical override ordering and
provider-free installer destinations have native regression evidence. The
committed reading now finds 3,528/3,528 cross-file references on the same two
canonical subjects. See the Terraform two-hop calibration assessment for its
population, native truth, throughput and regrade limits. That delivery does not
resolve a provider attribute or admit external schema declarations.

On the current main, SG-30/SG-36/SG-37 are still undelivered and
`internal/depplane` does not exist. Existing `typed_edges` represent call edges;
repo-relative governance anchors bind source lines to an enclosing stored
symbol. Neither is an admitted external provider declaration representation.
The design does not substitute a per-service copy or implement that future
plane in this lane.

## Local schema input contract

The product reads only an operator-created JSON export from
`tofu providers schema -json` or `terraform providers schema -json`. Neither
init command writes a schema document: it installs provider executables and
records selections in the lockfile. Getting the schema means executing those
providers over their plugin protocol. Without an operator-named export, keep
today's reading with no error. The product never runs that export command.

Proposed binding surface for review: one explicitly operator-selected, authored
JSON manifest, version 1, naming each export, its bound root and lockfile. It is
an input, not a product-generated memo, new implicit config search or hidden
cache. No default discovery and no binding means no schema enrichment. The
reader's proposed input type has this shape (placeholders, not measurements):

```json
{
  "version": 1,
  "roots": [{
    "root": "environments/prod",
    "lockfile": "environments/prod/.terraform.lock.hcl",
    "lockfile_sha256": "<sha256>",
    "export": "workspace/iac/prod.schema.json",
    "export_sha256": "<sha256>",
    "tool": {"name": "tofu", "version": "1.13.0"},
    "providers": [{
      "source": "registry.opentofu.org/hashicorp/aws",
      "version": "<locked-version>"
    }]
  }]
}
```

Manifest paths are relative to the service root. Both in-root and out-of-root
schema storage belong to the dependency-plane decision below, not different
representations. An out-of-root export may be named for calibration, but the
product adapter's path/anchor policy must be accepted before it admits one.
`ReadTerraformSchemaExport` is the new bounded reader proposed in
`internal/depplane`; this revision creates no manifest or implementation.

Verify the current lockfile's digest, source addresses and selected versions,
the export digest and supported format major before matching. A resource schema's
`version` is not the provider version. The schema JSON itself has no provider
version or lockfile identity. Tool version, root-of-capture and export-to-lockfile
association are operator assertions, not facts the JSON proves. Hash matching
detects subsequent changes, not a falsely captured association. Staleness checks
cannot establish CLI `dev_overrides`, a mirror's different build at the same
version, or a hand-edited binding. Count unverified provenance; never describe it
as a verified executable-to-export match without independent capture evidence.

Local provider names/aliases resolve through `required_providers`, explicit
resource provider selection and module providers mappings, never by prefix.
Preserve registry authority in source identity. For a shared child file, discover
all statically reaching bound roots through literal local module sources. Enrich
only if every reaching root selects the same provider address/version and
compatible schema content for the relevant provider. Different versions, export
content or mappings are counted ambiguous, with no first-root choice. Unbound
reaching roots yield incomplete context, not an inferred agreement. Stop cycles
and respect the existing traversal budget. The common reusable-module-at-root
plus multiple `examples/*` roots must be a fixture, not a later special case.

Accept only named regular local inputs under the accepted path policy. Resolve
symlinks; reject scope escapes, URLs, duplicate keys, unsupported major versions
and conflicting bindings. Never search home/cache paths, read state or plan files,
inherit credentials or fetch missing inputs. Failure removes enrichment only,
with a bounded reason. Unknown compatible metadata may be ignored; unknown schema
namespaces are counted unsupported rather than silently omitted.

The [OpenTofu schema format](https://opentofu.org/docs/cli/commands/providers/schema/)
distinguishes provider schemas, nested blocks, attributes and schema versions.
The manifest delivery is a proposed authored-input contract for controller review,
not permission to add a public flag, command or configuration key in this revision.

## Resolution and honest persistence

Parse a bounded immutable schema during ingest, not at query time. Select the
resource or data-source schema by provider identity plus declared type. Walk
static attribute traversals through attributes, nested blocks, object members and
collection element types. Count dynamic indexes, unknown tails, incompatible
types, absent function signatures and ambiguous provider bindings separately.
`aws_instance.web.private_ip` requires the declaration of `aws_instance.web` and
that selected provider's `private_ip`; `data.T.N.attr` uses the data-source schema.
A schema-known computed field is a type fact, never a known runtime value.
Provider function calls require native parameter/return metadata; no execution.

The planned new `EnrichTerraformSchemaRefs` in `internal/fusion` consumes reader
bindings plus declarations already admitted by the dependency adapter. IAC-12
alone owns declaration admission/persistence; IAC-8 owns configuration-reference
edges into those admitted targets, never a second declaration store. Schema
declarations belong in ADR-050's dependency plane, an ecosystem adapter whose lockfile is `.terraform.lock.hcl` and whose
input is this explicit export, following the operator-supplied OSV-file precedent
in `system-graph.md` section 8.4. SG-36/SG-37 are not yet delivered. Existing
`typed_edges` and SCIP import are not an external dependency API store.

No schema declaration persists in this lane before the controller accepts L4
policy and the dependency representation. IAC-7 also waits for IAC-16; IAC-8 waits
for SG-37 and the explicitly reopened IAC-12. These are manual authority
conditions as well as work eligibility:
`needs` alone is not a verification gate. The preferred design has no per-service
copy, even for an in-root export. An interim duplicate representation would need
separate explicit controller acceptance of duplication, migration and store-cost
measurements; it is not selected or authorized here. Do not pre-build SG-37 or
add its tables in the IaC lane under the table freeze.

When that adapter is accepted, persist only the referenced declaration closure
and its resource/data type nodes. Dedupe by provider address, selected version
and schema-export content digest across roots/services; retain the enclosing
export digest as provenance. Different content at one version remains distinct
and can make a shared child ambiguous. Do not insert all exported declarations,
emit dangling endpoints, invent source locations or relabel unresolved external
edges typed. Replacement/removal invalidates the affected digest's references.

Anchoring is an admission requirement. A one-line export gives every declaration
line 1; that is a counted anchor limitation, not a usable declaration citation.
The adapter must serve a JSON-pointer plus byte-range locator through an accepted
source contract, or consume an operator-supplied formatted export with useful
real lines. Until one is supported without inventing columns/locations, those
targets are ineligible for persisted typed proof. Calibration may retain bounded
pointer/range summaries and hashes; the product creates no pretty-print memo.

### Schema namespaces

Each exported namespace gets a disposition; absent maps are not evidence of
support. These are proposed grading categories, not claimed measurements:

| Namespace | First binding pass disposition |
|---|---|
| `provider` | Native schema declarations; provider-block fields graded only when the native candidate attests them |
| `resource_schemas`, `data_source_schemas` | Proposed graded static field/collection bindings, per native oracle scope |
| `functions` or separate native function metadata | Grade only reported names/signatures; unreported signatures counted unsupported |
| `ephemeral_resource_schemas`, including `ephemeral.T.N.attr` | Counted unsupported in the first pass, never silently omitted |
| `resource_identity_schemas`, `list_resource_schemas`, `action_schemas` | Counted unsupported when present; not-applicable only when that pinned format lacks the namespace |
| Other export schema maps | Counted unsupported, named in the assessment before any scope expansion |

## Bounds and failure behavior

Proposed calibration limits: 128 MiB per export, 256 MiB total input bytes,
16 exports per ingest, JSON depth 64, 500,000 scanned declarations in total per
ingest, and 256 path segments per reference. Streaming checks precede subtree
allocation; retain only matching declarations and needed ancestors. No compressed
artifact is a product input. Honor cancellation and tenant admission.

Proposed persistence budgets for the referenced closure are 10,000 rows and
16 MiB of actual stored bytes per distinct provider artifact, with at most
20,000 rows and 32 MiB newly admitted in one ingest. Measure store bytes per
provider and per service, row counts, deduplication across 16 roots, and peak
memory before the controller accepts any cap. These are unmeasured proposals.
The dependency plane currently plans an 8 MiB source-read and 2 MiB per-file cap:
a full AWS export may not fit. Those product caps remain binding until an explicit
per-ecosystem cap decision; the larger calibration proposals do not amend them.

The refusal unit is the whole export, including every provider it contains.
Byte, nesting, declaration, row or stored-byte cap+1 refuses all enrichment from
that export and publishes none of its partial closure. Other independently
valid exports and syntax reading continue. Stage the closure before a bounded
atomic publish through the accepted dependency representation; if it cannot
ensure whole-export refusal, persistence stays deferred. Report counters and
bounded reasons, never a partial provider presented as complete. No new cache,
memo or background worker is introduced by this lane.

## Provider-schema feasibility evidence

The IAC-2 [design/probe assessment](../assessments/iac-provider-schema-calibration.md)
records controlled offline captures for AWS 6.67.0, AzureRM 5.11.0 and Google
8.8.1. Native configuration display enumerates all four authored traversals per
provider; native validation rejects each individually renamed field with an
attributable diagnostic. These controls establish feasibility, not an L4 grade
or full-public-subject coverage. Native function, ephemeral-resource and
resource-identity metadata is present and remains ungraded.

Measured full exports are 19,517,134, 3,204,662 and 9,782,236 bytes respectively.
All exceed SG-37's planned 2 MiB file cap; AWS and Google also exceed its 8 MiB
package cap. The selected three-attribute JSON proxies are only 739, 699 and
833 bytes, but neither store bytes nor a usable declaration-anchor measurement.
All exports have one physical line. This evidence reinforces the existing
IAC-12 gate: settle source admission, anchors and ecosystem budgets through the
dependency plane before implementing IAC-7/IAC-8. It does not authorize raising
caps or selecting an interim per-service declaration copy. IAC-2 remains open.

## Calibration and acceptance

1. Freeze subjects, tool/provider pins, artifact checksums, metric definitions and
   exclusions before reading. Keep holdouts outside canonical shards. Use the
   tracked-set manifest recipe before/after oracles, retaining local receipts.
2. Pin and locally export schemas for `hashicorp/aws`, `hashicorp/azurerm` and
   `hashicorp/google`. No plan/apply, credentials or cloud endpoint. Distinguish
   registry archive checksums, executable checksums and schema-export digests.
   Measure prospective/actual store bytes per provider, referenced rows and
   dedupe across roots; no persistence is authorized just to obtain a number.
3. Independently enumerate native configuration references and native schema
   declarations. A schema export is tier A for the declarations it actually
   reports; a hand-written traversal walker does not become tier A because it
   consumes that export. Grade the two existing native candidates below
   separately before an L4 claim. Their binding scope is not yet graded here,
   not unavailable. IAC-2 records each candidate's disposition and residual gaps.
4. Grade resources, data sources, nested collections, aliases and provider
   functions separately, with the namespace dispositions above and per-subject
   numerators/denominators and native limits disclosed. Missing signatures, dynamic paths, ambiguity and unresolved
   references cost coverage; never remove hard cases to improve a ratio.
5. Perturb extractor output without changing raw oracle truth; change a known
   input field and prove the oracle changes. Test source replacement, removed
   bindings, multi-root ambiguity, provenance blind spots, no-schema byte
   equivalence, no network/exec, cancellation, path escapes, duplicate keys,
   malformed types, cap and cap+1, fan-out and nesting.
6. Start with failing tests. Run affected tests with `-race`, the language gate
   subset, round-trip checks and all moved doc-xref readings. Median-of-three
   throughput runs start only at allowed load and under nice. A parser test or
   schema acceptance result alone proves no L4 reading.
7. Only after design approval and an I-R0 slot: implement and regrade. Update
   provenance; the integrator edits the queue and generated files, and the
   controller proves plan claims. No lane flips a plan box.

## Native oracle candidates and X-R4 search

Search date: 2026-10-26. This is a source-contract search, not a new L4 runtime
reading. The committed Terraform reading records OpenTofu 1.13.0; the runner
actually detects its version, so the next measurement must pin/checksum the
chosen binary and each mirror provider rather than treating the runner as a pin.
`calibration/oracles/terraform/main.go` already invokes offline-mirror init,
`show -json -config` and `graph -type=plan`; its `references` helper reads native
expression reference arrays. It does not invoke validate. Calibration may execute
explicitly selected local measuring tools in a scrubbed, network-blocked mirror
environment. The product never executes those tools.

| X-R4 route | Candidate and source checked | Scope, tier and current disposition |
|---|---|---|
| Own toolchain | OpenTofu 1.13.0, existing oracle's `show -json -config` expression `references`; [show contract](https://opentofu.org/docs/cli/commands/show/) | Tier A enumeration of native traversal strings, including attribute tails and module outputs; already used for address-level truth, not yet graded for L4 field binding. Preserve full native strings before the existing collector normalizes addresses. Enumeration alone does not prove field validity or alias-selected provider identity. |
| Own toolchain | Same pinned mirror plus `tofu validate -json`; [validate contract](https://opentofu.org/docs/v1.9/cli/commands/validate/) | Tier A static attribute existence/type diagnostics. Candidate untried for this grade: collect per-reference rename/ablation diagnostics across the full native population. Validity alone does not identify the provider chosen by an alias. Combine native configuration provider mapping with lockfile/export provenance; count unsupported ambiguity. No plan/apply. |
| SCIP/LSIF | [SCIP implementation list](https://github.com/scip-code/scip#tools-using-scip), [Sourcegraph indexers](https://sourcegraph.com/docs/code-navigation/writing-an-indexer), [LSIF list](https://lsif.dev/); public searches `scip-terraform` and `scip-hcl` | No released Terraform/HCL producer identified in these searches. The LSIF site is archived and its embedded table rows were absent from the fetched text, an access gap. This is a bounded search miss, not a universal absence claim. A new custom SCIP writer would still require independent native binding truth. |
| Language server | HashiCorp terraform-ls 0.39.0, [versioned schema contract](https://github.com/hashicorp/terraform-ls/blob/v0.39.0/docs/schema.md) | Official provider-schema validation/completions are a tier-A candidate, untried here. Bundled schemas can mismatch versions; exact installed schemas require running the native schema command. Multi-root selection is documented as best effort, so ambiguous roots must remain misses. |
| Package registry/parser | HCL v2.24.0 from go.mod; [versioned parser API](https://pkg.go.dev/github.com/hashicorp/hcl/v2@v2.24.0/hclsyntax) and the native schema-export contract | ParseConfig/Variables enumerate syntax but do not select provider schemas or prove typed bindings. Shared with the product, so not independent native truth. Exported native declarations are tier A for their own scope; joining them with a custom walker does not raise that walker's tier. No new package is installed. |
| Runtime tracing | OpenTofu 1.13.0 `TF_LOG=trace`; [trace controls](https://opentofu.org/docs/v1.7/cli/config/environment-variables/) | Native executed/logged events only, not a demonstrated exhaustive static binding map. Untried, not a refusal; raw logs can include source/identity data and stay local. Grade the static candidates first. No cloud program, preview/up or apply is allowed. |

IAC-2's Native binding oracle disposition must give each of the two native
candidates its own numerators/denominators, tier, alias/provider-identity limits,
mutation outcome and residual scope. Change a known field to force Unsupported
attribute, and change extractor output without changing native truth. Missing
provider/function metadata stays counted. A future ceiling requires that measured
candidate disposition, not the phrase "oracle not established". Function metadata
and newer namespaces get their own counted scope rather than inheriting an
attribute-validity grade.

## Neighbor lanes within this program

Crossplane uses pinned provider CRDs only as calibration inputs. Check composed
objects against the matching group/kind/version schema. `fromFieldPath` must be
interpreted by patch direction: FromComposite reads the XRD schema; ToComposite
reads the managed resource schema; environment patches require their own source.
Optional/missing fields and runtime transforms do not prove a static binding.
Function input validation, reference evaluation, group-aware instance joins and
Function runtime evaluation remain distinct gaps. IAC-19 supplies the missing
calibration/remaining-ceiling assessment, while GAP1-34 covers the provider inputs
in the eventual reading. Upjet-generated AWS/Azure/GCP providers can reuse pinned
Terraform schemas as source inputs; bridged Pulumi AWS/GCP can too, while Azure
Native is a separate schema lineage. This is a reuse candidate: match exact
upstream versions/transforms before treating any two artifacts as equivalent.
Official patch semantics:
<https://docs.crossplane.io/latest/guides/function-patch-and-transform/>.

Kustomize requires larger real overlays with multi-base, components, generators
and cross-directory patches. Use the existing build-ablation oracle. It currently
excludes Components as standalone roots; do not claim component coverage from an
overlay's successful build. Measure component contents through their real parent
contexts before proposing an oracle extension. Keep unsupported filenames and
remote bases visible. Apply D19 per metric and report both thin and missing data.

Local Pulumi probe numbers are not durable design evidence. IAC-4 must first
commit a reproducible, sanitized assessment and receipts before any claim relies
on them. The generic external-target path is the dependency plane's npm `.d.ts`
reader (system-graph.md section 8.2, SG-36/SG-37), not a duplicate Pulumi-only API
store. IAC-14 also revives if SG-37 lands and a provider-specific gap persists.

Pulumi TypeScript/Python programs first use their ordinary language support with
installed SDKs. Probe all three clouds without running the programs. A compiler
binding and an unresolved external Fathom edge are different outcomes. Propose a
Pulumi-specific schema layer only if a measured provider-specific gap remains
after separating generic external-dependency limitations. YAML stays at its own
committed reading. IAC-20 supplies oracle-first triage for the other three
formats, not a restatement of their ceilings: try or explicitly dispose of
`terragrunt stack generate`/native block dumps, offline `cdk synth` with installed
dependencies, and bento's required Packer plugins. For each record tried, refused
or needs-owner-approval with the concrete scope and reason. Plugin approval
remains a separate owner boundary; a provider-schema request does not grant it.

## Evidence hygiene and authority

X-R5 applies to every future committed reading, receipt, assessment and data
file: keep counts and hashes, never subject contributor identities, ownership
metadata or copyright-holder text. Raw exports/logs/manifests stay local unless
a bounded sanitized representation is explicitly selected. Inspect new files
for at-signs, Author and name/email shapes before READY, alongside X-R2/X-R3.

The controller owns any implementation Accepted line. Design approval of PR #59
and release of the earlier IaC slot do not accept the L4 policy, declaration
representation or source caps. The current authorization remains design and
probe only; no implementation acceptance is asserted here.

## Decision models

None applies. Provider identity, schema membership, field paths, source admission,
limits and evidence grades are deterministic facts. Provider `none` is the whole
path and opting out means supplying no schema binding. No calibrated confidence
band can authorize a typed edge or cure missing native oracle evidence.
