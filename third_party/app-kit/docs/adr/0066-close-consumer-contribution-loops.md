# ADR-0066: Close contribution loops at consumer handoff

- **Status:** Accepted
- **Type:** process
- **Date:** 2026-09-16
- **Extends:** ADR-0017, ADR-0018 and ADR-0020

## Context

The owner wants each app effort to benefit the others and its intent/agent guidance
to remain accurate after adoption. Existing guidance says return improvements but
does not define a consistent assessment point, handoff record or incomplete-delivery
disposition. Fathom demonstrates that an app-owned composition fix can yield a shared
regression lesson without a new permanent component API.

## Decision

Use [the contribution loop](../design/contribution-loop.md) at discovery, repeated
workarounds and material UI/CLI handoff. Record task, evidence, ownership, disposition,
next owner/action and observable revisit condition. A shared defect earns reproduction
immediately; permanent promotion keeps existing layer-specific bars, including two
consumers for L3b/L4. No contribution quota.

Keep one consumer-owned contract reached by identical canonical agent routers. Review
intent, decisions, product context, proof/build commands and local host registration
on upgrades; edit only changed truths. Do not duplicate policy into new skills/config
or expand host permissions as incidental maintenance.

Distinguish returned knowledge/code, originating-app integration and second-app
propagation. Preserve domain ownership, identity, sanitized fixtures, paid-source
provenance and exact reviewed upgrades. Cross-repo writes remain scoped; prepare a
handoff when destination writes are unauthorized instead of changing it automatically.

## Consequences and alternatives

One concise record per material candidate or scoped no-candidate review, using existing
docs/issues. Guidance is not an enforced agent gate. Existing MCP serves the document;
no new server/runtime dependency. Automatic extraction and fleet upgrades are rejected:
they cannot establish fit, authority or promotion evidence. Sharing every feature
moves domain policy into the kit and creates abstractions ahead of demand.

## Revisit when

- The same workaround appears in two apps without a linked kit disposition.
- A promoted component retains the original fork after its upgrade is accepted.
- Two handoffs lose candidates despite this record; evaluate a deterministic local
  check with failure proof before claiming enforcement.
- A project cannot retrieve canonical guidance in its selected host; repair that
  scoped registration rather than adding a competing policy copy.
