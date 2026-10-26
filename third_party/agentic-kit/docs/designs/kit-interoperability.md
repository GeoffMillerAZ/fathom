# Independent kits, explicit composition

Owner request (2026-10-21): improve kit interoperability, independence, capability
and developer experience; integrate valuable contributions and prune old workspace
state. Decision: 0024. This is maintenance of shipped workflows, not a new platform.

## Boundaries

I1/I5/I13/I18 remain binding: stdlib-only Go, CUE data contracts, downward imports,
and consumer-owned processes. Decisions 0005/0006/0018/0023 retain domain authority,
evidence scope and optional providers. Review Kit owns floors and definition types;
Env Kit owns CUE unification. Shared metadata does not replace either domain model.
No provider call, registry, universal lifecycle protocol or consumer migration is
admitted. Catalog Kit remains experimental. Existing release promises remain intact.

The 0023 independent-release trigger has not fired. The audit does demonstrate a
present stable-key adapter mismatch and weak composition pilots, so its shared-
contract review applies. Keep the existing kit and foundations; add no new module.

## Acceptance

### KI-S1: scaffolding preserves existing work

`kit-new` must refuse every existing intended output, including directories,
symlinks and dangling symlinks, before writing any generated file. Render templates
before publication. Exclusive creation prevents a race from truncating a newly
appeared file. Failure triggers best-effort identity-checked cleanup of invocation-owned outputs.
Unknown identities, changed entries and cleanup failures report exact retained paths;
cleanup never deliberately unlinks an unknown identity. Exclusive publication protects
existing bytes. Cleanup is not atomic against an uncooperative concurrent writer,
and directory-creation failure can leave empty ancestors. This is not a cross-process
filesystem transaction or sandbox. Tests exercise the actual scaffolding entry point,
collision, metadata/write/close failure and incomplete-cleanup reporting paths.

Rollback keeps rooted file and workspace handles open through cleanup so closed-file
identity metadata cannot authorize removal after an inode is reused. File removal
also requires unchanged mode, exact size and the actual written bytes. A read pin is
required even for successful generation: a umask or ACL permitting creation/write
but denying read makes generation fail with an exact retained-path diagnostic.
Do not broaden permissions to work around that refusal. Same-byte rewrites and
concurrent namespace changes remain outside the best-effort cleanup guarantee.

### KI-S2: stable identity is an interoperable selection contract

`catalogkit.LookupStableKey(Bundle, string)` verifies the complete bundle and
returns a detached catalog.Node for exactly one nonempty exact key. Empty/invalid
UTF-8, absent keys and corrupt/ambiguous bundles refuse with no node. Hidden nodes
remain discoverable; PrepareDecision continues to refuse them. Callers explicitly
construct bindings using the returned ID and NodeDigest. Lookup grants no authority,
does not rewrite a stale binding and does not loosen exact definition/evidence pins.
Renaming a path retains lookup identity but invalidates an old path-bound packet.

### KI-S3: diffs preserve logical identity

Compare matches nodes by stable key. A retained key at a new path is a move, not
an unrelated removal/addition. Difference gains deterministic non-null `moved`
records with stable_key, before_id and after_id. Existing added/removed/changed
fields remain. Changed lists the current IDs whose non-location semantic fields
changed; a pure move is only moved. A replaced key at the same ID is removed plus
added. Hidden/presentation/source changes remain visible. Invalid inputs produce
no partial diff. All collections sort deterministically.

### KI-S4: discovery includes identities

Foundation BuildDocs indexes node IDs and stable keys as well as existing content.
Lexical ranking stays deterministic and injectable; it is not exact lookup or
semantic relevance certification. Catalogctl search retains hidden-node policy.
Exact Go consumer selection uses LookupStableKey rather than a fuzzy search result.

### KI-S5: CLI commands reject ignored configuration

Catalogctl rejects explicitly supplied flags that its selected command does not
consume, including explicit zero/false values. Invalid output formats, missing
explanation IDs, negative packet caps and invalid search limits also refuse before I/O. Rejection is a usage error (exit 2)
before file reads, with empty stdout and a useful command/flag diagnostic. Existing
valid invocations, JSON output, exit 0/1/2 and broken-writer behavior remain intact.
No terminal UI or new CLI dependency is introduced.

### KI-S6: domain pilots prove composition, identity and replay

Both Review and Env projections derive slugs from stable keys, never list position.
Insertion and permutation cannot rename unrelated nodes. Typed definitions, source
spans/hashes/origins and loaded Entry sidecars remain domain-owned and reconciled.
Review exercises at least two domain-validated layers, matching the common DAG's
precedence/provenance to the effective domain result. Env composes baseline and
extension metadata while LoadWith remains the authority for typed refinement and
contradiction refusal. Common metadata alone never proves domain validity.
The runner archives its immutable consumer revision if that object exists locally,
regardless of current checkout HEAD or dirty state; missing pins refuse. It never
writes the consumer tree. Tests prove these runner properties with disposable Git
fixtures and replay actual domain loaders at the recorded pin.

### KI-S7: independent adoption is executable

A small example runs from a separate temporary Go module with GOWORK off and
network module resolution disabled. It composes exported CUE fixtures, decodes and
resolves a catalog/profile, queries stable identity, binds a definition/evidence
packet and derives a reuse key without importing a provider, harness or sibling
checkout. A changed definition refuses. It uses public kit/foundation imports and
an explicit local replacement of the kit under test, never internal packages.
No third tracked nested module is created. Document prerequisites and exact replay.
This is executable integration evidence, not measured human setup or adoption.

### KI-S8: publication and cleanup are recoverable

Every selected change gets meaningful focused tests, independent review, full
`task validate`, strict plan proof and configured downstream checks before main.
Audit local/remote branches, worktrees, stashes and dangling contributions again.
Merge useful deltas; preserve non-equivalent drafts in verified backup artifacts
before pruning. Keep active/unresolved work. Record exact removed refs/paths and
backup digests. Verify remote main and the clean primary tree after publication.
Do not delete consumer work, private inputs or unrelated caches.

## Evidence and decisions

The implementation work order is `docs/plans/kit-interoperability.md`; its code
claims need strict Fathom proof plus runtime checks. External-module and domain
pilot receipts separately carry integration proof. Resource-gated/stale code
intelligence is not a clean proof: O11 records the observed host-headroom refusal.

## Decision models

None. Identity, flag use, file preservation and domain reconciliation are
deterministic. No model can waive a failure or authorize a write.
