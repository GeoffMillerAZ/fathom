# P08 profile drawer with explicit app-owned navigation

Status: experimental private typed adapter. [ADR-0053](../adr/0053-explicit-profile-drawer-and-app-owned-navigation.md)
defines its base; [ADR-0054](../adr/0054-explicit-drawer-focus-delivery.md) defines the
explicit CSS-only repair. [Evidence](../assessments/2026-09-15-drawer-typed-adapter.md)
separates compiler/replay and browser observations from outstanding target gates.
Nothing here supplies an account backend or certifies a web component.
The recorded screening passes 229 Chrome checks, including typed nested Escape,
selected long-text bounds and final-detail scrolling. A separate actual200% run
passes task/reflow checks but fails visible-focus screening, including unpainted
nested menu links. The selected repair passes 360 normal and 182 actual200% checks.
Missing imagery and full accessibility/lifecycle/consumer gates remain limitations.

## Task and scene

A person inspecting a profile from an app list opens its details, reads identity
and context, then chooses an app-owned destination. Test daytime desktop and evening
laptop use, with the purchased light/dark variants and a narrow viewport. The nested
menu must dismiss before its parent drawer, with focus returned at each step.
This bounded source adapter does not prescribe modal UI for every profile workflow.

## Export and integration

`appkit data assets-adapt-drawer -root PRIVATE_DIRECTORY` emits private JSON.
Add mutually exclusive `-out NEW_PRIVATE_DIRECTORY` or `-check PRIVATE_DIRECTORY`.
Export creates five immutable files: `drawer.templ`, `drawer.go`, `defaults.json`,
`USAGE.md`, `artifact.json`. Success is silent; exit 1 means observed drift,
2 invalid request/source or could-not-check. Existing output is never overwritten.
The destination parent must exist. Secure checks are scoped to macOS/Linux.

Compile a copy in a private app-owned module with templ v0.3.1020. Generated Go
imports only templ and the standard library, not app-kit. The app supplies a pinned
Tailwind 4 build scanning `drawer.templ`, its font/image closure and exact Elements
1.0.22 runtime identified in the artifact. No dependency or asset is installed.
Keep generated derivatives private; source access and license obligations persist.

| Entry point | Meaning |
|---|---|
| `DrawerReference()` | Constant original, including external photo and placeholder actions; a design specimen |
| `NewDrawer(Drawer)` | Validated, snapshotted, escaped app-owned presentation with optional native navigation |

No enhanced stylesheet or implicit P07 menu repair is loaded by default. Source styling
stays unchanged apart from the documented typed/ID/naming/action adaptations.

## Explicit focus repair

`assets-adapt-drawer -repair` selects a separate nested artifact and six-file
export/check: the unchanged three source/default files, `drawer_focus.css`, variant
`USAGE.md` and `artifact.json`. Without `-repair`, JSON and five-file delivery remain
unchanged. A base check rejects repair output and vice versa; no automatic migration.

Select both integration seams: load `drawer_focus.css` after source CSS, then wrap
only the chosen `NewDrawer` output in an app-owned `<div data-appkit-drawer-focus>`.
Do not wrap `DrawerReference()` or unrelated content. No extra constructor, JavaScript
mount/disposer, role or focus target is introduced. Elements still owns interaction.

Selected controls gain 44px minima, wrapping and inset 2px focus outlines. The menu
trigger outline uses source `--color-gray-900/100` tokens and its
`prefers-color-scheme: dark` media condition, not an invented `.dark` selector.
Text/icon colors and the already-passing close control retain source styling.
Review the app's final CSS ordering, theme/backdrop and clipping; source-token contrast
is not universal proof for arbitrary theme overrides. This repair cannot supply
runtime-failure recovery, touch/screen-reader proof or dynamic lifecycle ownership.

## Model and ownership

`Drawer` binds `IDPrefix`, `OpenLabel`, `Heading`, `CloseLabel`, `Name`, optional
`Handle`/`OnlineLabel`, `MenuLabel`, `ImageURL`, `ImageAlt`, `ImageDecorative`, optional
`Primary`/`Secondary *DrawerLink`, `MenuLinks []DrawerLink` and `Details []DrawerDetail`.
Defaults deliberately leave instance ID and image URL empty, omit actions, and do
not claim online presence. Handle constructor errors before writing a response.

- ID prefixes are 1–64 ASCII identifier characters starting with a letter. The app
  must keep them unique across the document; the constructor cannot inspect it.
- Required labels are bounded nonblank UTF-8 without controls. Optional handle and
  online label are omitted when empty. Presence and freshness remain app-owned.
- Images require an explicit local/HTTPS URL. Decorative means empty alt; otherwise
  provide meaningful alt. Source-photo URLs are provenance, not bundled assets.
- Each link supplies label and local/HTTPS URL. URLs reject protocol-relative/userinfo,
  malformed escapes, controls and backslashes. Primary/secondary and up to twelve
  menu entries navigate; no message, call, clipboard or mutation is implemented.
- Up to twelve details use `Label`, `Text`, `Kind` and `Date`. Kind is `text`,
  `paragraph` or `date`; date rows require a real canonical YYYY-MM-DD, otherwise
  Date is empty. Text is bounded to 4096 bytes; labels to 256 bytes.
- All pointers/slices are copied. Empty actions yield a valid read-only drawer.
  Navigation must lead to an honestly labeled task, never a sensitive GET mutation.

Elements owns local modal/menu state, focus, selection and Escape. The app owns
runtime/CSP delivery, routes, authorization and any downstream result/recovery.
No second renderer may morph the owned subtree without separate lifecycle proof.
Provide essential profile/destination access elsewhere when runtime loading fails.

## Proof and remaining work

The converter accepts only the frozen P08 identity, validates binding positions,
retains exact source-reference DOM, and checks deterministic replay before delivery.
Default tests use authored synthetic markup; live tests require explicit private
source access. Hashes and observed browser failures live in the linked assessment.

Original-photo fidelity, touch, screen readers, complete visible focus/
contrast, runtime failure, dynamic lifecycle, performance, standalone CSS/asset delivery
and real consumer adoption are not inferred from compiler or selected Chrome tasks.
Two cold isolated consumer render builds now reproduce code, typed/read-only HTML
and the test binary without app-kit source or network access; see the linked evidence.
Legacy 439/657 conversion coverage and the zero-certified-target count are unchanged.
