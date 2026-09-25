# ADR-0031: Preview and export app-owned themes without rewriting source

- **Status:** Accepted
- **Type:** architecture
- **Date:** 2026-09-13
- **Extends:** ADR-0029

## Context

Named presets and the Gallery selector do not complete the owner's app-specific
preview, reset, validation and export loop. Existing token loading evaluates CUE;
it does not preserve authoring expressions/comments as an editable syntax tree.
Token shape validation also does not make arbitrary CSS safe for web submission.
No two-consumer promotion trigger has fired for a general editor framework.

## Decision

Incubate the present loop in `apps/styleguide`, independently of Gallery and paid
corpora. An app supplies immutable trusted source CUE, an explicit kit identity,
approved font stacks, nonempty selected contrast checks, stylesheets and trusted
side-effect-free synthetic specimen callbacks. Reuse the existing token schema,
CSS emitter and contrast measurement. Do not add runtime preferences or file writes.

Keep browser edits bounded: existing token paths only, opaque hex color overrides,
finite bounded px/rem radii and approved font IDs. Unchanged authored values remain
valid inputs; this is not a generic CSS/CUE sanitizer or an upload executor.
Unknown/nonfinite/malformed measured colors cannot pass a contrast check.

Prepare a deterministic concrete JSON-as-CUE candidate and sorted token diff.
Keep raw original bytes separately. Export a deterministic ZIP with original CUE,
candidate CUE/CSS, diff and receipt only after the current review digest matches and
all configured checks pass. The digest binds original bytes, candidate/CSS/diff,
source label, kit identity, dark strategy and selected check definitions/results.
This is candidate acknowledgement, not authenticated human approval or accessibility
certification. The concrete candidate is explicitly not a lossless source patch.

The initial handler is local-development only: exact configured loopback origin/peer,
bounded native URL-encoded forms, same-origin POST, no cookies or persistent drafts.
Apps own any authentication wrapper. Reset discards form edits. Preview documents use
opaque-origin sandboxing, no scripts/forms/font/image requests and app-owned CSS plus
candidate variables. Allow reviewed SVG style attributes, not arbitrary user CSS.
Callbacks are trusted Go code; bounded output is not isolation of their I/O or CPU.

Gallery opts in with a source file at startup and reuses its existing renderers.
Its local executable hash identifies the exact unpublished implementation, not a
released module pin. Other apps supply their actual selected identity and fixtures.
Keep ordinary Gallery setup, licensed comparisons and deployed apps independent.

## Alternatives considered

- Unify a new value into concrete authored CUE: conflicting values do not implement
  overrides, and flattening silently discards authoring structure.
- Arbitrary browser CUE/CSS editing: unnecessary execution/resource surface today.
- Add frame same-origin permission for fonts: weakens isolation; a named local-font
  delivery contract must earn a later extension instead.
- Treat a screenshot or selected contrast checks as promotion: misses real app tasks.

## Revisit when

- Two real app guides duplicate the same missing contract and meet the promotion bar.
- A named task requires remote access, local font assets, active interactions or persistence.
- A source/candidate/style change escapes the acknowledgement or export checks.
- Browser review finds clipped controls, inaccessible errors or ineffective token changes.
