# ADR-0034: Deliver private adapters through explicit source-specific exports

- **Status:** Accepted
- **Type:** architecture
- **Date:** 2026-09-14
- **Decision drivers:** private provenance, repeatable consumer inputs, no-clobber writes

## Context

ADR-0023 separates literal conversion, reviewed adaptations and target verification.
The P02 typed adapter produces reusable private sources but its original JSON-only
interface requires manual extraction. The public portable vendor channel must not
silently acquire licensed derivatives. A delivery match cannot certify an experience.

## Decision

`assets-adapt-pricing` keeps its read-only JSON default. Explicit mutually exclusive
`-out NEW_DIRECTORY` and `-check DIRECTORY` deliver and compare the frozen P02 source.
Both first require independent original archive access/integrity and current replay.

Export creates one absent directory under an existing real parent, with exclusive
0700 directory/0600 regular-file creation. It writes template, model, companion CSS,
private defaults, first-party usage guidance and the existing CUE-governed artifact.
No new lock or verification schema duplicates the artifact. Its JSON is written last.
Existing destinations are refused, never replaced, repaired or deleted. Failed I/O
leaves incomplete private output for operator inspection; this is not crash-atomic
publication. The owner controls the parent and output during execution; this is not
a filesystem sandbox against a hostile same-user concurrent writer or ACL audit.

Checks regenerate expectations from the original, never trust delivered hashes alone,
and compare all six files, exact 0700/0600 modes and bounded regular single-link files.
Missing, extra, modified, symbolic/hard-linked or special files fail. macOS/Linux use
non-following, nonblocking reads and file-identity checks; other platform checks report
unsupported. CLI exit 0 means exported/matching, 1 means observed drift, 2 means invalid
request or could-not-check. Export/check success is silent; failures omit private bodies.

The delivery is immutable input: compile a copy in an app-owned build directory, with
app-owned Go/templ/Tailwind/font pins, checksums, routes and data. No dependency install,
compiler invocation, application/backend scaffolding or catalog write occurs in export.
No app-kit runtime dependency is introduced. `NewPricing` and `NewPricingEnhanced` remain
explicit independent choices. Generated sources/compiled outputs remain private.

## Alternatives considered

- Public `appkit vendor` integration: would mix portable kit source and licensed assets.
- Generic exporter/scaffolder: defer until a second adapter proves a shared contract.
- Stage then rename: attractive publication boundary, but portable rename can replace
  an existing empty directory. Exclusive creation gives the intended no-clobber rule.
- Manual extraction only: remains compatible, but does not check delivered inventory.

## Consequences

Immutable input and build output are separate directories; consumers must retain
provenance and own their build. Delivery replay and a sibling-free synthetic compile
are bounded evidence, not a real app task, visual pass, catalog target or release.
No G1–G6 outcome or deferred consumer/platform is completed or promoted by this choice.

## Revisit when

- A second typed adapter has the same delivery needs, justifying shared machinery.
- A named consumer needs in-place edits/upgrades, Windows checks or atomic publication.
- A missing, modified or linked delivery falsely passes, or private output escapes its
  intended access boundary.
