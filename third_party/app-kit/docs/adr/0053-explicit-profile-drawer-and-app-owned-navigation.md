# ADR-0053: Bind profile drawers to explicit app-owned presentation and navigation

- **Status:** Accepted
- **Type:** architecture
- **Date:** 2026-09-15
- **Extends:** ADR-0023, ADR-0036, ADR-0040 and ADR-0049

## Context

Frozen P08 contains a profile drawer with a nested Elements menu, demo identity,
external photo and placeholder actions. Constant compilation preserves the design
but cannot safely infer a presence provider, messaging system or clipboard action.
Typed binding fires ADR-0040's review condition; nesting requires the focus review
called for by ADR-0049. The existing twelve-case pilot already includes this task.

## Decision

Retain the unchanged `DrawerReference()` beside a source-specific `NewDrawer(Drawer)`.
Generate both deterministically from the verified private original, never pasted
vendor source. Preserve source styling and icon geometry. The typed variant adds
document-scoped IDs and dialog naming, non-submit triggers, plain content, explicit
image semantics and conditional app-owned navigation. This is not an account backend.

Require an app-selected ID prefix and approved image URL. Decorative images have
empty alt; meaningful images require nonblank alt. The app owns document-wide ID
uniqueness, asset rights/delivery and truthful identity/presence data. Defaults leave
IDs/images unbound, actions absent and online state unclaimed. Empty action lists
form a valid read-only drawer rather than inaccessible or fake actions.

Primary/secondary destinations and nested menu links use native GET navigation.
They do not send a message, place a call or copy a profile link. The app selects
labels that describe actual destinations and never performs sensitive mutations
through GET. No POST, clipboard API, auth service or event bus is inferred.

Snapshot bounded inputs and escape plain text. Definition-list details have explicit
text, paragraph or canonical calendar-date semantics. Elements 1.0.22 owns modal
focus, nested menu selection, Escape and dismissal; no second state machine or
Datastar ownership is added. Essential tasks need an app-owned runtime-failure path.

Use ADR-0036's immutable private five-file export/check with source-replayed
expectations. Do not install CSS, fonts, photos, runtime or dependencies. Compile a
copy in an app-owned module. Keep browser/visual/standalone/consumer proof separate;
default artifacts and metadata remain not-verified experimental CLI references.

## Alternatives considered

- Infer behavior from demo labels: would invent app authority and misleading actions.
- Reuse P07's POST model for every action: no P08 mutation contract justifies it.
  Reconsider for a named consumer with explicit authorization and recovery semantics.
- Replace Elements with a new drawer/menu state machine: no demonstrated advantage;
  first measure and repair a concrete failure while preserving the original.

## Consequences

Apps gain readable typed source without app-kit runtime coupling. The cost is an
explicit image/action contract, pinned asset/runtime closure and family-specific
proof. Missing source imagery remains a fidelity gap, never silently substituted.
No existing adapter API/output changes or permanent reusable-layer promotion occur.

## Revisit when

- Nested focus, Escape order, long content, zoom or runtime failure blocks the task.
- A consumer needs mutation, clipboard, presence freshness or persisted/morphed state.
- Source/runtime drift invalidates replay or required image rights/bytes are unavailable.
- Two consumers demonstrate a shared composition boundary beyond this private pilot.
