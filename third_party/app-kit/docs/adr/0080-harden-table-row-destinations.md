# ADR-0080: Keep managed table row destinations safe and keyboard-reachable

- **Status:** Accepted
- **Type:** security
- **Date:** 2026-09-23
- **Extends:** ADR-0079's destination-shape policy to kit-managed table row links

## Context

The shared table casts unvalidated `Row.Href` to `templ.SafeURL` and renders a small
inline Open action. A plain string therefore bypasses URL sanitization, while cell
padding is mistaken for link target space. Form buttons already enforce a bounded
web-reference policy; duplicating it would invite future drift.

## Decision

Share the existing form-button URL-shape predicate in an internal UI package, not
a public routing/authorization API. Preserve its behavior and form rendering bytes.
Managed table links accept unchanged relative references and absolute HTTP(S) with
a hostname. Reject leading/trailing whitespace, C0/DEL controls, raw backslashes,
protocol-relative references, malformed escapes/URLs and non-web or opaque schemes.
This is shape validation, not trust, same-origin enforcement or route authorization.

A refused nonempty `Row.Href` produces a noninteractive span with role="link",
aria-disabled="true" and visible “Open unavailable”. It has no href, tabindex,
handler or button semantics. Enabled links retain native anchors and gain44px
minimum width/height plus explicit semantic focus styling. No JavaScript is added.
Empty Href, `RowAction`, `HTMLCells`, `Attrs` and app-owned activation stay unchanged;
trusted slots remain the caller's reviewed boundary, not sanitized by this repair.

This tightens accepted destinations and changes managed-link HTML/layout. Adopters
rebuild templ output and scanned CSS, review snapshots/table density, and verify
their Open/detail/return task. Non-web handlers require reviewed app-owned actions;
never strip a rejected scheme to bypass refusal. No consumer upgrade is automatic.

## Alternatives rejected

- Rely on HTML escaping or a cast: neither rejects executable URL schemes.
- Keep an href with aria-disabled: native activation still navigates.
- Public universal URL policy: route matching and trust have different owners.
- Separate copied predicates: two current callers justify internal reuse without
  changing the accepted policy or introducing an extensible URL framework.

## Evidence boundary

[Frozen tests and browser tasks](../assessments/2026-09-23-table-link-safety.md) cover
the selected row actions. Full table accessibility, sorting/selection, consumer
adoption and core standards remain separate. A repaired link is not a certified table.

## Revisit when

- A named consumer needs a refused scheme or legitimate destination shape.
- Accepted browser URL interpretation disagrees with the predicate.
- Refused links activate or managed targets/focus fail a composed table task.
- Another control needs a different policy; do not broaden this predicate silently.
