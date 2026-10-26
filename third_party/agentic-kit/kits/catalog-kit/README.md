# Catalog Kit

An experimental, offline workflow over the existing catalog and decision foundations.
CUE owns authored shape; Go resolves composition and records the result. No provider,
credentials, network access, registry or storage service is enabled by this kit.

Use [the assessment](../../docs/assessments/2026-10-20-catalog-kit.md) for the repository
inventory, research, placement decision, capability matrix and migration sequence.

```text
cmd/
  catalogctl/                 resolve/profile, scan-go, explain, query, search, doctor, diff, prepare
  internal/catalog/bundle.go  existing Go inventory integration
kits/catalog-kit/
  schema.cue                 authored contracts
  resolve.go                 composition, integrity and provenance
  decision.go                evidence-bound request preparation
  decode.go                  strict input decoding
  export.go                  semantic diff and analytics rows
  scripts/                   optional Parquet adapter and tests
  skills/                    adopt, declare, doctor, policy
  testdata/                  CUE fixtures
docs/
  assessments/2026-10-20-catalog-kit.md
  plans/catalog-kit-uplift.md
  evidence/catalog-kit/2026-10-20/
```

## A working path

Run from the agentic-kit repository root with the declared Go and CUE toolchains:

```sh
cue vet ./kits/catalog-kit/...
cue export ./kits/catalog-kit/testdata -e spec > /tmp/catalog-spec.json
cue export ./kits/catalog-kit/testdata -e policy > /tmp/catalog-policy.json
go run ./cmd/catalogctl resolve -spec /tmp/catalog-spec.json -policy /tmp/catalog-policy.json > /tmp/catalog-bundle.json
go run ./cmd/catalogctl explain -bundle /tmp/catalog-bundle.json -id review/evidence
go run ./cmd/catalogctl query -bundle /tmp/catalog-bundle.json -prefix review
go run ./cmd/catalogctl search -bundle /tmp/catalog-bundle.json -text evidence
go run ./cmd/catalogctl doctor -bundle /tmp/catalog-bundle.json
go run ./cmd/catalogctl resolve -spec /tmp/catalog-spec.json -policy /tmp/catalog-policy.json -format ndjson > /tmp/catalog-rows.ndjson
go run ./cmd/catalogctl diff -before /tmp/catalog-bundle.json -after /tmp/catalog-bundle.json
```

The first real-source integration is the existing Go package inventory. It reuses
`cmd/internal/catalog.Collect`, resolves every package plus intermediate groups, and
tests exact package-facet coverage against the live scan:

```sh
go run ./cmd/catalogctl scan-go -root . -revision <recorded-source-revision> > /tmp/go-catalog-bundle.json
go run ./cmd/catalogctl query -bundle /tmp/go-catalog-bundle.json -facet kind=package
go run ./cmd/catalogctl scan-go -root . -format ndjson > /tmp/go-catalog-rows.ndjson
```

This is a tested source projection of this repository's catalog, not a production
Review Kit or Env Kit migration. The adapter follows this repository's kebab-case
path identity convention; unsupported path segments and ambiguous packages sharing
one path refuse rather than silently normalize or drop entries. The existing Markdown
generator and published-doc drift gate keep their compatibility format.

For an adopter outside this repository, pin the kit module revision and import its
CUE schema through the adopter's existing CUE dependency process. Do not paste a
second schema into the adopter. A validated CUE export and the Go resolver are both
required; the JSON parser's checks do not replace domain-specific CUE constraints.

`catalogctl help` lists commands and flags. Successful help and operations exit 0,
checked-invalid input exits 1, and usage or file I/O failures exit 2. Reports are plain
JSON on stdout, diagnostics on stderr. Search exposes scores and an explicit count,
with no-match as an empty result; hidden entries require `-include-hidden`. Explain
also reports the full source-pin set and receiver policy digest.

## Composition contract

`Spec.layers` is the selected collection of already-loaded subcatalogs. `requires`
declares precedence, including general, organization, team and project layers. The
resolver does not infer scope from directory names, discover remote sources or fetch
imports. A source's optional `pin` is `LayerDigest` of its JSON declaration excluding
the pin. Pin trust comes from the receiver's reviewed configuration.

New nodes require a `stable_key` and `source`. Siblings, path slugs and stable keys must
be unambiguous. Tree structure and facet axes remain separate. Default bounds are 256
layers, 100,000 input nodes plus patches, 256 hierarchy levels, and 16 MiB for authored
or resolved JSON. Exceeding a bound refuses the complete operation. The data model does
not fix a domain taxonomy depth; the resolver intentionally has an operational bound.

Repeated IDs must descend from every earlier contributing layer. Facets union with
sorted, deduplicated values. A scalar assertion may fill a previously absent value or
agree with an inherited value. Replacing a conflicting value requires `overrides`:

```json
{
  "id": "review/evidence",
  "expected_digest": "<NodeDigest from explain>",
  "reason": "Use the project presentation order",
  "set": {"order": 0, "hidden": false}
}
```

The receiver must independently grant exactly those fields for that layer and ID in
`Policy.grants`. Unknown, duplicate or unused grants refuse, as do stale digests,
undeclared dependencies, ambiguous winners and unreasoned patches. Grant keys never
use wildcards. Omitted patch fields differ from explicit `0`, `false`, `""` and `{}`.
Identity and source cannot be patched. Facet replacement is explicit; ordinary facet
contributions remain additive.

This is an explicit Go transformation of separately validated records. It is **not
CUE unification**. After tailoring, a domain adapter must revalidate the result against
its final CUE constraints and semantic floors. For example, Review Kit's evaluation
identity, expiry, severity and lane restrictions continue to belong to Review Kit.
A mechanical grant does not authorize weakening those restrictions.

`Bundle` records source declaration digests, resolved order, the receiver policy
digest and each patch's before/after digests, fields, layer and reason. `NodeDigest`
covers hidden state, presentation order and synopsis as well as identity and content;
it does not redefine the older `catalog.Node.Hash` compatibility contract. `VerifyBundle`
detects changes to the resolved bundle, including provenance. It does not authenticate
the producer. Compare against a separately trusted expected bundle digest at adoption.

A receiver-owned `Profile{name, roots}` selects named roots plus their transitive
dependencies. Use `DecodeProfile`/`ResolveProfile`, or `catalogctl resolve -profile
profile.json` alongside the spec/policy. Every loaded declaration must remain
structurally valid and bounded (graph, node slugs, pins and resources), including
unselected layers. Contextual identity, conflicts and override permissions are
validated only in the selected composition. Alternative layers may
conflict when combined but remain valid outside the selected closure. Policy is used
unchanged, so grants for unselected layers refuse as unused. The profile name does
not change the digest when effective content and policy are identical.

## Decision-ready criteria

Keep the domain criterion definition in its existing CUE schema. A `Binding` sidecar
names its catalog node, exact `NodeDigest`, definition version, SHA-256
`definition_digest` of its complete current domain definition bytes, one existing
`decision.Question` and required evidence IDs. The question ID must equal the catalog
node's stable key. Choice options require distinct authored criteria and an explicit
`unknown` option. Predicate and score interpretations need a caller-owned abstention
policy; the kit does not provide universal thresholds.

`Evidence` supplies the captured text, source, revision and SHA-256 of that exact text.
The collector is responsible for allowed scope, line locations, source authenticity,
redaction and completeness. `PrepareDecision` refuses stale or hidden criteria;
missing, duplicate, extra, empty or corrupted evidence; and oversized packets. It
assembles one atomic question, quotes the criterion and evidence as data, and seals the
complete request. This text wrapper is advisory defense in depth, not an isolation
boundary. A binding does not prove the evidence is sufficient for its semantic claim.

Pass the exact current definition bytes separately to `PrepareDecision`; preparation
refuses a stale definition even if metadata, question and version labels are unchanged.
The exact definition is included in the sealed request. Validate its typed domain
schema before preparation. Raw-size preflight bounds payload construction, then the
complete encoded request is checked against the byte cap.

```sh
go run ./cmd/catalogctl prepare -bundle /tmp/catalog-bundle.json -binding binding.json -definition criterion.txt -evidence captured-evidence.json > /tmp/prepared-decision.json
```

The command prepares a request; it never sends it. A consumer may pass `Prepared.Request`
to the existing `foundations/decision` port using its consent, budget, capability and
calibration checks. The existing TypeSafe and OpenAI adapters own their native wire
formats. Provider none remains a supported path.

`DecisionCacheKey` binds the sealed request to provider, requested and resolved model,
adapter version and outcome-policy version. Changes to question wording, evidence,
catalog, definition version or provider identity invalidate the key. It is a key
helper, not a cache or authorization to reuse expired/revoked evidence. Add
`-identity identity.json` to `catalogctl prepare` to emit both the prepared request
and its exact reuse key; incomplete identity refuses before output. Refusal,
malformed output and timeout remain separate outcomes under the decision port. A
question answered successfully does not close its parent rule or authorize an effect.

## Analytics and Parquet

`Rows` and `ExportNDJSON` produce ID-sorted projections, including hidden entries, with
stable keys, facets and source lineage. The full bundle is the audit record. A projection
does not preserve every policy/source detail needed to reconstruct a resolution.

The optional Python adapter needs Apache PyArrow in its own environment:

```sh
python kits/catalog-kit/scripts/export_parquet.py /tmp/catalog-rows.ndjson /tmp/catalog-rows.parquet --bundle-digest <resolved-bundle-digest>
python -m unittest discover -s kits/catalog-kit/scripts -p '*_test.py'
```

It writes typed columns, nested facets/layers, projection version, the supplied bundle
digest and the actual NDJSON hash. The supplied digest is a lineage label, not proof
that the input rows were independently verified. Validation precedes replacement of a
prior output. Pin the adapter dependency in the adopter's environment. Logical row and
metadata determinism is tested; Parquet byte identity across library versions is not
promised. Parquet is useful for large cross-catalog queries and evaluation telemetry;
small authoring catalogs should stay in CUE.

## Skills and verification

Read the relevant skill directly; no global agent configuration is required:

- [catalog-adopt](skills/catalog-adopt/SKILL.md): inventory and migrate a real catalog.
- [catalog-declare](skills/catalog-declare/SKILL.md): author entries and atomic bindings.
- [catalog-doctor](skills/catalog-doctor/SKILL.md): diagnose resolution, drift and evidence.
- [catalog-policy](skills/catalog-policy/SKILL.md): review overrides and semantic floors.

Run `go test -race ./kits/catalog-kit ./cmd/catalogctl` and the CUE checks above for local
verification. The repository's `task validate` and affected consumer checks remain the
integration gate. These instructions and skills are advisory; the resolver's refusal
tests establish only the specified offline behavior. Live model quality and production
consumer adoption require their own recorded evidence.

The [accepted specification](../../docs/designs/catalog-workflow.md) and
[replayable consumer pilots](pilots/README.md) define current scope and bounded
Review Kit/Env Kit integration evidence.

## Stable identity and independent adoption

Use `LookupStableKey(bundle, key)` for exact selection before constructing a binding.
It verifies bundle integrity and canonical catalog structure, then returns a detached
node. It can discover hidden nodes; decision preparation still refuses them. A path
move retains the stable key but never refreshes an old path/digest-bound decision.

`Compare` joins by stable key. Its `moved` array records stable_key/before_id/after_id;
pure moves are not remove/add pairs, while non-location changes appear in `changed`
using the current ID. Replacing a key at the same path is removed plus added.
All result collections are deterministic and non-null. Identity terms participate
in lexical search; use exact lookup rather than relevance ranking for bindings.

Catalogctl accepts only each command's documented flags. Unused/conflicting flags
(including explicit false/zero values) and invalid command values fail with exit 2
before input I/O and no JSON output.

The [external-module example](../../examples/catalog-consumer/README.md) validates
CUE and runs without a provider, harness, sibling repository or network module
resolution. [Composition pilots](pilots/README.md) replay their immutable consumer
commit from any checkout HEAD and retain domain-owned validation and sidecars.
