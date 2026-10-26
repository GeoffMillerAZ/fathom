# IaC provider schemas: local evidence before typed claims

Design proposal for issue #55, 2026-10-26. Implementation is blocked by I-R0
and design review. This document changes no reading or product behavior.

Owner request: "let's add the major terraform providers and pulumi providers and
crossplane providers if applicable ... the providers for aws, azure and gcp, no?"

The unit of support is the configuration that uses a provider. Provider schemas
are evidence inputs, not languages or new board entries. Terraform/OpenTofu L3
comes first (I-R1). L4 is a separate measured result, never inferred from loading
a schema. Kustomize joins this lane under I-R4. ADR-048 currently limits tool formats
to L1-L3; Terraform is catalogued as tooling. The owner-requested L4 design
therefore needs an explicit grading-policy amendment before any L4 queue or
reading claim, even if the implementation proves native typed bindings.

## Binding intent and fired triggers

- G15 and ADR-048 require a committed, oracle-graded reading for a support level.
  I54 forbids a level from registration alone. D19 applies each L3 metric's own
  denominator floor of 20. Kustomize's historical 13/13 is not measured under it.
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
  Function execution still needs containers and remains excluded by the brief.
- Kustomize's directory-preference trigger has already been addressed in code:
  `oneDocFile` prefers the referring dialect, and
  `TestKustomizeBaseDirectoryNeedsTheOneFileRule` includes Kubernetes manifests.
  The provenance's future-tense warning is stale. Verify it, then correct the
  provenance only after I-R0 opens. Larger real component/overlay evidence is
  still needed.
- F25 (infrastructure migration conservation) remains deferred: nobody requested
  a state migration. Keep provider and schema identity in evidence; build no
  migration engine or future dependency database for this lane.

## Present behavior and the first structural gap

`HCLExtractor.ExtractDoc` in `internal/extractor/hclextract.go` labels definitions
by Terraform address and directory. Its `childDir` map exists for one file only.
A module-output reference is emitted only when that file also declares the module
call. A sibling `outputs.tf` keeps the call reference but loses the output target
and child-directory provenance before `ResolveDocXrefs` sees it. The recorded
reading finds 2,147/3,528 cross-file pairs (.6086 recall), missing 1,381 such reads.
The current regression corpus explicitly expects only the sibling's call edge.

The future fix must preserve unresolved module/output intent at extraction and
join it at fusion using the uniquely declared call in the caller's directory,
then its static local source, then the output in the child directory. Do not
scrape a signature, broaden the directory scope, pick the first duplicate, or
open sibling files in a per-file extractor. Remote, dynamic, escaping, recursive
and ambiguous module paths stay unresolved and counted. Existing same-file
behavior remains. This is ENGINE work: inventory all doc-xref readings (including
nested holdout/tuned readings), regrade every moved reading, and report epoch
requirements before integration. No implementation is included here.

## Local schema input contract

The first supported artifact should be an operator-created JSON export from
`tofu providers schema -json` or `terraform providers schema -json`, bound to the
configuration it describes. An initialized provider installation is not itself a
schema document. Reading a provider binary cannot recover its schema; invoking
it would execute repository-selected code. If init left no readable export,
retain today's syntax/file reading with no hard error.

A binding needs the tool/version, export format version, canonical provider
source address, selected provider version from the lockfile, configuration/module
root, lockfile digest and export digest. The schema's resource `version` is a
schema version, not a provider version. An unbound export cannot prove that the
installed provider matches it. Provider aliases and local names resolve through
`required_providers` and each resource's explicit provider selection, never by
prefix alone. Preserve the Terraform/OpenTofu registry authority in identity;
normalizing a shorthand must not silently equate two different registries.

Accept only named regular local files. Resolve symlinks and reject scope escapes,
URLs, duplicate JSON keys, unsupported major versions and conflicting bindings.
Unknown minor-version fields may be ignored. Never search arbitrary home/cache
paths, read state or plan files, inherit credentials, or fetch missing schemas.
A missing/invalid/oversized/mismatched artifact removes schema enrichment only.
Record a bounded reason and keep the original reading. Provide an opt-out through
the eventual input binding; no binding is the default. The binding's public
surface is a review decision, not an invented configuration key in this design.

The OpenTofu format reference documents resources, data sources, nested blocks
and attributes, and distinguishes schema version from provider version:
<https://opentofu.org/docs/cli/commands/providers/schema/>. Exported function
metadata varies by tool/version. A function with no native signature evidence
remains unmeasured rather than being guessed from provider source code.

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

There is a representation constraint, not just a parser task. Existing
`typed_edges` stores symbol endpoints and edge kind; SCIP import maps those onto
already extracted symbols. The dependency API index in `docs/design/system-graph.md`
§6/§8 is planned, not a delivered external-symbol store. Do not emit a dangling
schema target or label an unresolved external edge typed.

Recommended first slice: an explicitly named schema export inside the service
root is an additional read-only source, with real source rows and content hashes.
Its schema declarations become ordinary source-anchored observations in the
existing tables, identified by provider address/version, resource-or-data kind,
type and field path. Preserve JSON source lines (including one-line exports),
use collision-free canonical symbol keys and deterministic signatures, and keep
schema nodes distinct from configured resource instances. Only schema-backed
relations whose two endpoints have been admitted may persist, using an additive
relation kind that does not masquerade as a call or a file-only edge. Authorize
that source explicitly even if ignored; this is not general ignore bypass.
Missing, replaced or removed schema sources must invalidate their observations
and relations through existing ingest replacement, not append stale evidence.

Before implementation, prove that existing source admission, replacement,
querying and edge semantics support this slice without widening the store or
inventing source locations. If they cannot, stop at a written representation
ceiling and return to the controller. An out-of-root export may be inspected
locally for calibration but cannot yield persisted typed targets under current
root-relative anchors. Supporting that needs the separately reviewed dependency
plane; do not copy it into a hidden memo or store an absolute path. This narrow
first slice keeps the table freeze and future dependency identities open.

## Bounds and failure behavior

Proposed preregistered limits, to be tested against all three providers before
acceptance: 128 MiB per export, 256 MiB total input bytes, 16 exports, JSON depth
64, 500,000 schema declarations and 256 path segments per reference. Use a
streaming token reader with byte/depth/count checks before allocating a subtree.
No compressed artifact is a product input. Honor ingest cancellation and the
existing responsible-tenant admission budget. Refuse the whole offending export
at cap+1; do not present a truncated provider as complete. Keep other valid
providers and syntax extraction. Report read/parse/match counts and reason totals
with paths only at the existing bounded disclosure surface. No new cache or
worker. The first local probes must establish whether these limits admit the
pinned AWS/Azure/GCP exports; the numbers are proposals, not measured capacity.

## Calibration and acceptance

1. Freeze subjects, tool/provider pins, artifact checksums, metric definitions and
   exclusions before reading. Keep holdouts outside canonical shards. Use the
   tracked-set manifest recipe before/after oracles, retaining local receipts.
2. Pin and locally export schemas for `hashicorp/aws`, `hashicorp/azurerm` and
   `hashicorp/google`. No plan/apply, credentials or cloud endpoint. Distinguish
   registry archive checksums, executable checksums and schema-export digests.
3. Independently enumerate native configuration references and native schema
   declarations. A schema export is tier A for the declarations it actually
   reports; a hand-written traversal walker does not become tier A because it
   consumes that export. Resolve the independent native field-binding oracle
   before making any L4 claim. Untried candidates stay untried.
4. Grade resources, data sources, nested collections, aliases and provider
   functions separately, with per-subject numerators/denominators and native
   limits disclosed. Missing signatures, dynamic paths, ambiguity and unresolved
   references cost coverage; never remove hard cases to improve a ratio.
5. Perturb extractor output without changing raw oracle truth; change a known
   input field and prove the oracle changes. Test source replacement, removed
   bindings, no-schema byte equivalence, no network/exec, cancellation, path
   escapes, duplicate keys, malformed types, cap and cap+1, fan-out and nesting.
6. Start with failing tests. Run affected tests with `-race`, the language gate
   subset, round-trip checks and all moved doc-xref readings. Median-of-three
   throughput runs start only at allowed load and under nice. A parser test or
   schema acceptance result alone proves no L4 reading.
7. Only after design approval and an I-R0 slot: implement, regrade, change the
   item's queue/provenance, and have the controller regenerate the board and
   prove plan claims. No lane flips a plan box.

## Neighbor lanes within this program

Crossplane uses pinned provider CRDs only as calibration inputs. Check composed
objects against the matching group/kind/version schema. `fromFieldPath` must be
interpreted by patch direction: FromComposite reads the XRD schema; ToComposite
reads the managed resource schema; environment patches require their own source.
Optional/missing fields and runtime transforms do not prove a static binding.
Function input validation, reference evaluation, group-aware instance joins and
container-dependent render remain distinct gaps. Official patch semantics:
<https://docs.crossplane.io/latest/guides/function-patch-and-transform/>.

Kustomize requires larger real overlays with multi-base, components, generators
and cross-directory patches. Use the existing build-ablation oracle. It currently
excludes Components as standalone roots; do not claim component coverage from an
overlay's successful build. Measure component contents through their real parent
contexts before proposing an oracle extension. Keep unsupported filenames and
remote bases visible. Apply D19 per metric and report both thin and missing data.

A local TypeScript probe (TypeScript 5.9.3, scip-typescript 0.4.0; SDKs
AWS 7.0.0, Azure Native 3.0.0, GCP 9.0.0) resolves all three resource
constructors natively. Fathom imports all three as unresolved external targets,
with zero direct constructor edges. This is the existing generic external-target
boundary, not evidence for a Pulumi-specific schema layer. Probe artifacts and
full reproduction remain local under I-R0; no language grade changes.

Pulumi TypeScript/Python programs first use their ordinary language support with
installed SDKs. Probe all three clouds without running the programs. A compiler
binding and an unresolved external Fathom edge are different outcomes. Propose a
Pulumi-specific schema layer only if a measured provider-specific gap remains
after separating generic external-dependency limitations. YAML stays at its own
committed reading. Terragrunt, HCL tool configs and CDK retain their recorded
ceilings until the corresponding trigger is demonstrated, not merely restated.

## Decision models

None applies. Provider identity, schema membership, field paths, source admission,
limits and evidence grades are deterministic facts. Provider `none` is the whole
path and opting out means supplying no schema binding. No calibrated confidence
band can authorize a typed edge or cure missing native oracle evidence.
