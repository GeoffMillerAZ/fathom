# ADR-0040: Keep Elements translation explicit and replayable

- **Status:** Accepted
- **Type:** architecture
- **Date:** 2026-09-14
- **Extends:** ADR-0023; does not supersede the legacy syntax rule or storage contract

## Context

Four frozen HTML specimens now have a pinned, reviewed Elements runtime baseline.
The legacy translator deliberately refuses custom elements, and its receipt identity
hashes its source/schema. Widening that implementation would change existing replay
identities and hide the difference between static markup and runtime-dependent UI.
The owner wants faithful automated conversions, not replacement interaction engines.

## Decision

Add an opt-in `html-elements-to-templ-v1` rule and `elements-syntax` receipt family.
The ordinary `html-to-templ-v1` source, schema, defaults and saved receipts stay intact.
Shared private validation helpers may be reused without mutating their global grammar;
the new rule fingerprint includes their legacy digest and its own implementation.
No universal intermediate representation or placeholder-tag rewrite is introduced.

The initial grammar admits disclosure, dropdown/menu and dialog/panel Elements tags,
literal placement attributes and bounded native HTML/SVG. It preserves original
structure, text, comments, classes and states. Scripts, inline events/styles, behavior
expressions and unsupported custom elements remain refusals, with no partial output.
Command references must resolve unambiguously to the expected local target kind.

Each receipt declares Elements 1.0.22 with exact ESM/tarball hashes and an explicitly
host-provided module. Translation neither installs that commercial runtime nor supplies
routes, form actions, authorization or data. Output remains private constant-only templ.
`verification: not-verified` cannot represent behavior, accessibility or target approval.

Expose `appkit data assets-translate-elements` as a separate read-only JSON command.
Its 0/1/2 exits retain syntax-success/unsupported/could-not-check distinctions; `-save`
is not accepted. Existing `syntax@` storage, list/get tools and Gallery conversion
routes do not consume this family. Discoverability may use source-free experimental
CLI catalog metadata, not a verified web-component record.

Verify the four frozen sources and compiled outputs under identical fixtures, CSS,
font, runtime, viewport and theme. Preserve source failures. Test exact replay and
full compiled DOM before/after formatting, then equivalent interaction/remount tasks.
Browser proof remains scoped; no Datastar, native-host or consumer support is inferred.

## Consequences and alternatives

- A second explicit rule costs a small family-specific parser/envelope surface. Share
  existing safe helpers now; consolidation must retain old replay behavior and identity.
- Silently widening v1 loses the runtime and historical-receipt boundary, so is rejected.
- Rewriting disclosures/menus in Datastar duplicates behavior without an earned benefit.
- A separate persistent catalog is deferred until a named retrieval/upgrade task needs
  it. No speculative multi-family store or automatic promotion is built now.

## Revisit when

- An admitted family needs a new Element, command, attribute or runtime pin.
- Original/compiled DOM, interaction, CSP or lifecycle parity fails.
- A named consumer needs persisted Elements receipts or typed/repeated instances.
- A shared-helper change would invalidate historical legacy or Elements replay.
