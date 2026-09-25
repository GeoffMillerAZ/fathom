# ADR-0064: Incubate responsive settings sections without form authority

- **Status:** Accepted
- **Type:** architecture
- **Date:** 2026-09-16
- **Decision drivers:** repeated Profile/Settings layout, usable long labels, app-owned state

## Context

Gallery repeats five Catalyst-informed description/control sections across Profile and
Settings. The same responsive grid, spacing and heading relationship are maintained
inside a private template. Existing form primitives supply labeled controls, not this
composition. The repetition is real; two comparable consumer integrations are not yet
established. ADR-0017's permanent-promotion bar therefore remains unmet.

## Decision

Expose experimental `form.SettingsSection` and `form.RenderSettingsSections` under
ADR-0019's bounded incubation rule. Each ordered section associates an escaped h3 with
reviewed content, stacks below sm, and places description and controls side by side
above it. Semantic roles, separators and long-label wrapping are shared. The caller
supplies an h2 parent and a document-unique heading ID; this renderer validates IDs
within one call and rejects invalid input before emitting any markup.

Use existing controls through reviewed HTML slots. The app retains labels, field IDs,
forms, submit actions, authorization, validation, secrets, persistence, draft recovery,
concurrency and focus management. A section is not a fieldset or a label substitute.
No JavaScript, form wrapper, session abstraction, schema engine or dependency is added.
Consumers scan `appkit/ui/form/*.go` with their Tailwind build and include theme CSS.

Gallery replaces its five repeated sections with this renderer, retaining original
copy, controls and private state branches. Appearance stays independently composed.
Catalog `shell.settings-sections` advertises only the experimental Go-web presentation,
with no consumer or backend claim. Existing form and shell APIs remain unchanged.
The new exported API is a compatibility obligation despite experimental maturity.

## Proof and promotion boundary

The named app fixture is Gallery Profile/Settings: preserve native save, validation,
conflict, permission, sign-in and recovery behavior with scripts on and off. A separate
Go-rendered component fixture tests long unbroken labels, heading relationships,
keyboard input/select access and narrow/wide light/dark geometry without scripts.
That virtual fixture proves layout, not an application route or save backend.
Both probes retain executed source and artifacts; full accessibility, physical input,
other engines and real-consumer acceptance remain separate gates.

Permanent promotion requires two relevant consumers replacing their original layout
forks with reviewed upgrades and preserved domain behavior. This addition does not
complete G2 or promote the Gallery's account controller.

## Alternatives considered

- Keep all sections private: avoids an API but keeps five layout/semantic forks.
- Export the whole form workflow: premature authority sharing across incompatible apps.
- General form-schema engine: no demonstrated need; revisit for repeated unmet forms.

## Revisit when

- A second consumer establishes or contradicts the description/control boundary.
- Heading hierarchy, long text, focus or script-free saving regresses in a consumer.
- New section types require runtime/state ownership rather than presentation.
- If only Gallery uses it for two releases, demote it with migration notes; remove an
  abstraction that adds no reuse benefit instead of expanding it to manufacture demand.
