# Governed catalog workflows

Status: accepted implementation specification, 2026-10-21. Decision: 0023.
Work order: [catalog-kit-uplift](../plans/catalog-kit-uplift.md).

## Product and boundary

A catalog is a versioned program of discoverable, attributable definitions.
Humans need to find an entry, understand its origin, and safely tailor it. Agents
need the same operations with bounded input, deterministic JSON, explicit empty
results, and refusal before partial output. Integrity never establishes semantic
truth, source authenticity, authorization, or completeness of a review.

Compose the existing catalog and decision foundations. Keep the root module
stdlib-only. Preserve the existing catalog Build/hash contracts. Consumers retain
typed domain payloads, final CUE validation, monotonic floors, approvals, evidence
collection, provider selection, and effects. The common tree is a metadata view,
not a replacement for Review Kit or Env Kit schemas.

## Delivered contracts

**[CK-S1] Composition.** Already-loaded layers form an explicit dependency DAG.
Duplicate identities, missing dependencies, cycles, unrelated collisions, and
implicit conflicting replacements refuse. Defaults bound layers, nodes/patches,
depth and input/output bytes. Input permutations preserve effective resolution.

**[CK-S2] Tailoring.** Only receiver-owned exact field grants permit reasoned,
digest-checked patches. Explicit false, zero and empty values remain expressible.
Unknown, duplicate, stale and unused grants refuse. Contributions cannot grant
themselves authority. Semantic floors must be checked by the domain after tailoring.

**[CK-S3] Profiles.** A separately authored named profile selects root layers and
their transitive dependencies. Roots must be nonempty, unique and present. The
entire loaded graph must have valid dependency structure, node slugs, pins and
resource bounds, including unselected declarations. Contextual identity, scalar
conflicts, override preconditions and grants are checked only for the selected
composition: omitted stable keys/sources can be valid inherited contributions.
Policy is
unchanged by selection, so an unused grant remains an error. Names are descriptive:
profiles selecting identical content and policy yield the same effective digest.

**[CK-S4] Identity.** A bundle binds source declarations, precedence, policy, effective
nodes, and patch history. Semantic digests include synopsis, order and hidden state.
Consumers compare with independently trusted pins. Strict JSON rejects unknown and
duplicate fields, null authored values, malformed Unicode, trailing input, and caps.

**[CK-S5] Decision preparation.** A criterion binding pins its stable node, semantic
node digest, definition version, and SHA-256 of the complete current domain
definition bytes. The consumer supplies those exact bytes separately. A mismatch
refuses even if the question and version are unchanged. Prepared state contains
the exact definition and captured witnesses. Every required witness must be present,
nonempty, unique and hash-matched; unexpected witnesses refuse. Preflight sizes
before constructing packets and enforce the complete encoded request cap.

**[CK-S6] Reuse.** The prepared digest binds definition, question, evidence and bundle.
Reuse identity additionally binds requested/resolved model, provider, adapter and
outcome-policy versions. `prepare -identity identity.json` exposes the prepared
request and its key without a provider call. These are identity helpers, never permission to reuse
expired evidence or bypass consent, revocation, calibration or required review.

**[CK-S7] Discovery and diagnosis.** `catalogctl` provides help, resolve/profile,
scan-go, explain, query, search, doctor, diff and prepare. Search reuses foundation
BM25 and deterministic tie-breaking, returns an explicit count and query, and hides
hidden entries unless explicitly requested. Doctor validates an existing bundle or
resolves authored input, reports checked counts and digest, and refuses malformed
input. Explain includes exact source pins and the policy digest as well as the node
and patch history. Stdout is stable data, diagnostics use stderr. Usage errors and
I/O inability exit 2; checked-invalid input exits 1; success and help exit 0.

**[CK-S8] Projections.** JSON/NDJSON rows are deterministic derived data. Optional
Parquet lives outside the Go dependency graph and preserves prior output on refusal.
The bundle remains the audit record. The Go inventory projection reconciles exact
package membership against the live collector, including intermediate groups.

## Domain integration and acceptance

Review Kit preserves all DefinitionSource line spans/content hashes and CatalogOrigin
fields in its own stable-key sidecars. Project only discovery metadata. Compare
effective identity coverage and provenance before and after projection, and execute
BuildStandard's severity, tier, freshness, concept and finding refusal cases.

Env Kit continues to load through CUE Unify/Validate. Project the resulting catalog
for discovery; do not replace its loader with override semantics. Execute LoadWith
extensions and contradiction refusals. Reference pilots use isolated consumer
snapshots at recorded revisions and never modify the developer's consumer checkout.
A pilot proves a bounded integration, not production migration or measured savings.

Acceptance requires adversarial library/CLI tests, CUE fixtures, full `task validate`,
actual configured downstream checks with revisions and skips, and Fathom proof of
the plan's code claims. Preserve archival branches/worktrees/stashes until their
disposition is recorded. Never merge a weaker historical draft over hardened main.

## Coverage across catalog domains

| Need | Current mechanism | Consumer responsibility / extension trigger |
|---|---|---|
| Organization, team, project variants | layers, explicit grants, named profiles | final policy/floor validation |
| Deep taxonomy and orthogonal variants | arbitrary tree shape with operational depth bound; facets | domain naming and payload schema |
| Rules, recipes, tools, model inventories | stable keys, source pins, exact definitions | typed domain schema and collectors |
| Search, drift, documentation, analytics | existing ranker, semantic diff, deterministic projections | wire drift into actual publication gate |
| Ownership, applicability, evidence requirements | consumer-owned typed definitions and sidecars | validate before resolution/adoption |
| Required/optional checks and parent outcomes | existing domain evaluation/policy contracts | no compensating averages or hidden discharge |
| Aliases, moves, typed cross-links, retirement | stable keys and existing moved-node drift | add shared lifecycle only with a named link/history failure |
| Exceptions, approval, expiry, revocation | existing authority systems | exact target/version and failure tests |
| Signed distribution and atomic activation | receiver pins and local resolution | add transport only with named distribution/trust need |
| Multimodal and model quality | existing optional decision port | calibrated adapter/workload, explicit budgets and opt-out |

## Decisions and review

No universal metadata bag, result grade, mandatory model lane, server, registry, or
new storage engine. No speculative relation/lifecycle protocol. The user-sponsored
uplift admits profiles and discovery now; future adapters stay outside this delivery.
Decision 0023 retains its revisit triggers. Decision 0018's provider/confidence
contract is preserved. The actual domain-definition gap is closed here without a
provider migration. App-kit CLI guidance is consulted, with a recorded divergence:
its Go packages require external dependencies forbidden by I1, so this command uses
stdlib flags and plain JSON while following its output and exit-code contracts.
