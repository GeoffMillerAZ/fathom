# ADR-0036: Share filesystem assurance, not adapter semantics

- **Status:** Accepted
- **Type:** architecture
- **Date:** 2026-09-14
- **Extends:** ADR-0034; its P02 CLI, artifact and six-file contract remain unchanged
- **Decision drivers:** second concrete adapter, private provenance, bounded reuse

## Context

P04 now has separately tested source-styled and enhanced typed output. Like P02,
it needs explicit no-clobber delivery and source-replayed checks. ADR-0034's second
adapter trigger has fired. The duplication is filesystem assurance, not a shared
component model, renderer, artifact schema or arbitrary export/scaffolding framework.

## Decision

Share only unexported fixed-file writing/checking and secure-open helpers inside
`internal/designadapt`. Source-specific entrypoints verify their frozen source and
current replay before I/O, build their own inventories and retain distinct drift
sentinels. Inventory names are unique basenames; `artifact.json` is written last.
No caller chooses filenames or provides a manifest as the source of expected bytes.

P04's `assets-adapt-directory` retains its exact JSON default and existing `-enhanced`
envelope. Add mutually exclusive `-out NEW_DIRECTORY` and `-check DIRECTORY`, silent
on success. Base delivery has five files: template, model, defaults, usage guidance
and artifact. Enhanced delivery has eight, adding companion template/model/CSS and
using the existing nested-base artifact. `-enhanced` selects that exact inventory;
base and enhanced deliveries never silently cross-check or upgrade one another.

Preserve ADR-0034's existing real parent, exclusive 0700/0600 creation, retained
incomplete writes and no overwrite/repair/delete contract. Check exact names/bytes/
modes using bounded single-link regular-file reads with no-follow/nonblocking and
identity checks on macOS/Linux. Other checks remain unsupported. Exit 0 is matching
or exported, 1 observed delivery drift, 2 invalid request/source or could-not-check.
The owner controls parent/output during execution; this is not hostile same-user
concurrency isolation, ACL auditing or crash-atomic publication.

Consumers compile a copy in a separate private app-owned build directory. Delivery
does not install tools, run compilers, bind routes, supply auth/backend behavior,
change catalog state or introduce an app-kit runtime dependency. Keep licensed
inputs and generated outputs private. Source-styled/enhanced choices remain explicit.

## Alternatives considered

- Duplicate secure I/O in P04: simpler separation but doubles security fixes and tests.
- Generic public exporter or adapter registry: no present need; fixed inventories
  already express both deliveries without a new schema or plugin mechanism.
- Merge artifacts/models: would invalidate P02 and erase independently replayed P04
  base/enhancement identities without an integration benefit.

## Consequences

Both adapters inherit reviewed filesystem repairs, while semantics stay independent.
Existing P02 tests and exact artifact/output checks guard against refactor drift.
Delivery/compile proof does not certify target quality, real app adoption or G1–G6
completion. Original sources, runtime alternatives and deferred platforms stay intact.

## Revisit when

- Another adapter needs non-flat assets, mutable upgrades or different access rules.
- A shared helper changes either adapter's established output or error classification.
- A named consumer needs atomic publication, Windows checks or managed integration.
