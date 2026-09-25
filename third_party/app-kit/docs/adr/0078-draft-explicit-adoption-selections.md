# ADR-0078: Draft explicit adoption selections without installing them

- **Status:** Accepted
- **Type:** architecture
- **Date:** 2026-09-23
- **Extends:** ADR-0024/0065; existing commands and manifest schemas are unchanged

## Context

First-time adopters must hand-assemble JSON and placeholder hashes before `pins`
can help: it requires an existing valid manifest. `inspect` observes declarations
and `router` prints instructions; neither authors a selection. This need is within
the owner's resumed adoption work, not permission to infer an app's requirements.

## Decision

Add CLI-only `appkit adoption draft`. Require explicit root, consumer, profile and
human-contract path. Repeated layer/capability/artifact flags express selections;
optional module-file/version and agents flags declare further checks. No module
version, local-replacement allowance, router requirement or capability is inferred.
At least one capability, artifact, module or agent requirement is necessary.

Validate through the existing CUE-authored JSON schema and the same catalog-fit
logic as `check`. Bind selected entries with existing canonical metadata pins;
an optional expected whole-catalog revision refuses stale discovery. Hash only
explicit artifact paths through the existing confined reader and shared budgets.
Refuse duplicates, invalid UTF-8, unsafe paths, unavailable files and cancellation.
Do not read an existing adoption manifest, module, contract or instruction file
unless the caller explicitly lists that file as an artifact. No commands run.

Emit one deterministic indented version-2 selection to stdout, with no progress
decorations. Zero means the draft was produced, not adoption passed. Usage,
authoring, read and output failures return2 and produce no selection before the
output write; a broken output writer may receive partial bytes. Existing check's
0/1/2 behavior and report bytes are preserved. Arrays remain arrays when empty.

The operator reviews selected metadata, requirements and observed hashes, saves
the draft deliberately, prepares the human contract/routers and then runs `check`.
Draft is not an update command: never regenerate over an existing contract to
approve unexplained drift. Sequential artifact reads are not an atomic snapshot.
Metadata/file agreement does not prove origin, licensing, host use or runtime quality.

No new MCP consumer-filesystem surface, installer, dependency, network use, host
registration or profile/runtime generator is introduced. Printed choices are not
approved consumer changes. Existing v1/v2 readers need no schema migration.

## Alternatives rejected

- Hand-author all pins/hashes: preserves control but unnecessary error-prone work.
- Infer choices or rewrite manifests in place: hides consumer authority and drift.
- Artifact-only draft: leaves Go/module and agent-only users hand-authoring fields
  already supported by the checker; explicit flags need no inference or execution.

## Revisit when

- Draft and check disagree about schema or metadata fit for identical inputs.
- An adopter treats drafting as automatic upgrade approval or runtime proof.
- Two named adopters need a reviewed multi-file authoring transaction or profile
  generation that this read-only manifest command cannot express.
