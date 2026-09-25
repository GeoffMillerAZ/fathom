# ADR-0063: Incubate shared account destinations and feedback, not an account framework

- **Status:** Accepted
- **Type:** architecture
- **Date:** 2026-09-16
- **Decision drivers:** G2 reuse, repeated navigation destinations, explicit app authority

## Context

The Gallery shell already composes six routes through one RenderShell call. Another
page-options wrapper would not eliminate duplication. Account destinations, however,
were repeated across its sidebar, account disclosure, quick navigation and help links.
The disclosure omitted About and used different target treatment from the sidebar.
Feedback semantics were embedded in a Gallery template keyed directly to HTTP status.

RAC and Fathom currently prove navigation reuse, not a common account-save backend.
ADR-0017's two-consumer bar for permanent L3b component promotion is not met.
ADR-0019 permits a bounded owner-directed incubation with evidence and a kill condition.

## Decision

Expose experimental `dashboard.AccountDestinations` and `dashboard.RenderNotice`
alongside the existing frame and navigation. The destination model produces a fresh,
ordered group, omits unsupported entries and preserves caller labels, paths and match
policy. RenderNav still owns escaping, disabled unsafe URLs and current-link selection.
The same app-authored group feeds Gallery's sidebar and account menu. Custom task and
quick links keep fixed Gallery routes: raw model fields are not validated URLs, and
template escaping does not implement RenderNav's disabled-item treatment.

Notice kinds are explicit status, error or conflict presentation. Plain text is escaped;
unknown kinds receive attention treatment. The app determines the outcome, title and
body. This renderer does not infer a successful save, permission or retry from HTTP.

Keep identity, route authorization, state branching, schema, validation, CSRF, persistence,
draft retention, concurrency, appearance mounting and diagnostic redaction app-owned.
No new shell wrapper, framework, runtime, dependency or client script is introduced.
Existing Shell/RenderNav contracts and composition paths remain unchanged.

This is an additive exported compatibility surface, even while experimental. Existing
callers are not changed; future incompatible edits require migration notes. The catalog
retains experimental maturity and names downstream acceptance gaps, not production auth.

## Proof and promotion boundary

The named fixture is Gallery `/shell`: compare all six routes' account destinations,
notice outcomes, mobile/theme/keyboard targets and existing native save/recovery tasks.
Compiled public examples and omission/escaping/unsafe-link canaries prove the narrow API.
One model replaces duplicated sidebar/account-menu destinations, not their route policy.

Permanent promotion still needs two relevant consumer integrations with preserved
domain behavior, a11y contracts and reviewed upgrade evidence. This change is neither
that promotion nor completion of G2; read-only apps need not gain account mutations.

## Alternatives considered

- Another universal ApplicationShell options wrapper: duplicates the existing frame
  without sharing a repeated behavior. Revisit only for repeated unmet composition needs.
- Export Gallery's session/form controller: couples consumers to synthetic authority and
  non-secret draft policy; incompatible with sensitive or read-only app workflows.
- Documentation alone: cannot keep several navigation surfaces on the same model.

## Revisit when

- Two consumer integrations establish or contradict this presentation boundary.
- A permitted destination differs unintentionally between navigation surfaces.
- A notice misrepresents an app-owned outcome or breaks keyboard/recovery behavior.
- If only Gallery uses the recipe for two releases, demote it to Gallery with migration
  notes; if an integration adds no reuse benefit, remove the unnecessary abstraction.
