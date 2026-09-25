# ADR-0041: Keep faithful FAQ reference and native disclosure explicit

- **Status:** Accepted
- **Type:** ux
- **Date:** 2026-09-14
- **Extends:** ADR-0023, ADR-0036 and ADR-0040; changes no existing syntax rule

## Context

The frozen P03 FAQ now has source/compiled runtime and visual evidence. It still
contains demo text, fixed IDs and small controls. Its runtime-withheld control cannot
toggle an answer. The owner's help/about and reusable-content goals need bounded app
content, multiple independent instances and a deliberate static-site choice.

## Decision

Add a private source-specific typed FAQ adapter, not a general accordion framework.
Accept only the frozen P03 archive/member/hash and use the unchanged Elements syntax
rule as the first check. Generated content and defaults remain private.

`NewFAQReference` retains source DOM, classes and Elements behavior; extracted defaults
must reproduce the original. Its host owns the exact Elements 1.0.22 runtime from
ADR-0040. `NewFAQDetails` uses native independent `details`/`summary` disclosures for
help/content pages, without a client module. Neither silently substitutes for the other.

Both take bounded plain heading/question/answer strings, stable item keys, explicit
initial open states and a caller-chosen ID prefix. Snapshot input; reject malformed
identity, duplicate keys and invalid content before output. The host reserves distinct
prefixes across instances and owns content policy. No raw HTML, automatic ID registry,
app backend or persisted disclosure state is introduced.

Native markup replaces definition-list/button/custom-panel semantics while retaining
purchased visual roles and explicit open icons. Add 44px targets, visible keyboard
focus and long-content wrapping. These are reviewed usability changes, never normalized
into fidelity comparisons. Multiple panels may stay open; no exclusive accordion.

Reuse private immutable export/check assurance with exact source replay before I/O.
Keep runtime requirements per variant, original-linked metadata and `not-verified`
receipts. No installation, public vendor integration, certified catalog target,
permanent shared component or consumer mutation is implied.

## Evidence and alternatives

Test original/default exact DOM and controlled screenshot parity separately from
native keyboard/pointer/JavaScript-disabled tasks, repeated-instance isolation, long
content, caller mutation, safe text, cancellation and writer errors. Compile exported
copies without a sibling checkout. Preserve source failures and unsupported platforms.

- Elements everywhere preserves behavior but makes static help depend on a module.
- Native-only erases the faithful reference and its independently useful runtime choice.
- A new Datastar disclosure engine duplicates local browser behavior without benefit.
- Automatic fallback/ID allocation hides runtime failure or document ownership.

## Revisit when

- A real consumer needs rich answers, deep-linked panels, exclusive opening, persisted
  state or dynamic morphing beyond this bounded content contract.
- Native keyboard/focus/open-state behavior or purchased styling fails the frozen task.
- Distinct instances interfere, invalid identity passes, or source replay changes.
- Two consumers justify a permanent shared component rather than a private adapter.
