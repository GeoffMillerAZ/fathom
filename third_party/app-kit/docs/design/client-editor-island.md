# P12: a client-owned editor inside a Go page

Status: admitted bounded pilot under [ADR-0023](../adr/0023-profile-specific-conversion-and-client-compute.md),
not a public component API, converted editor, source-fidelity claim or certified profile.
The [frozen P12](../assessments/2026-09-13-conversion-pilot.md) and
[hardened MatDash reference](../assessments/2026-09-14-matdash-hardened-reference.md)
remain independent originals/references. This specimen changes hosting and lifecycle,
not the decision to use React/Tiptap for local editing.
The [scoped implementation and proof](../assessments/2026-09-16-client-editor-island.md)
pass ownership/tasks and cost screening; full P12 acceptance remains open.

## Discover and rehearse

Agents can retrieve this exact guide with `get_kit_decision("client-editor-island")`
or `kit://client-editor-island`; neither requires a licensed corpus. Follow the
[private fixture instructions](../assessments/fixtures/client-editor/README.md) for
the source/native-toolchain prerequisites and explicit build/browser commands.
This is guidance, not a catalog capability or a command that installs/prepares paid
source. No public widget entry point or consumer-ready editor is advertised.

## Task and ownership

Geoff edits a product description on a laptop in light or dim surroundings, while
the surrounding application updates its context. Preserve the existing light/dark
editor, toolbar, preview and keyboard behavior. Typing and formatting stay local;
Go serves the page, immutable assets and synthetic context, not an editing RPC.

One React root owns the editor subtree. Host code may update the surrounding page and
use the declared context operation; it must not replace or morph editor descendants.
No universal island framework, settings backend, persistence, collaboration, offline
reconciliation or new kit-core/client dependency follows from this one reference.
Paid component bodies and their derivatives stay in a separately identified private
directory; first-party host/contract/tests contain no paid markup.

## Version-one lifecycle contract

- `mount(element, initial, onChange)` reserves one connected empty element and creates
  one client instance. Duplicate mounts fail before changing the live instance.
  Initial text is plain text, not imported HTML; the editor still produces rich content.
- `initial` names protocol version1, a stable instance ID, accessible label, hint and
  initial text. Validate types, unknown fields, ID syntax and bounded text before DOM work.
  Labels/hints are text, never HTML. A second instance has separate IDs and state.
  Live instance IDs are unique within this adapter; disposal releases the ID.
- `update(context)` changes only versioned label/hint context for the same instance.
  It may not supply document/initial text or silently remount. Preserve the editor node,
  focus, selection, draft revision and undo/redo. Refuse invalid updates atomically.
- `snapshot()` returns a detached typed document/text/revision snapshot once ready.
  Host use of this content remains untrusted; this API is not HTML sanitization.
- `dispose()` is idempotent, unmounts React and destroys the editor. Later updates
  refuse; no change callback is delivered after disposal. The element may mount a new
  explicitly initialized instance. Detachment alone must not become implicit persistence.
- A page reload starts the explicit initial document. The host visibly states that
  there is no save backend. Local editing while disconnected is not offline durability.

Bound initial text to64KiB UTF-8, labels/hints to512bytes, and instance IDs to48 ASCII
letters/digits/hyphens starting with a letter. These are specimen protocol limits,
not universal document-size or internationalization rules. No arbitrary imported
HTML/JSON, remote document replacement or silent truncation is admitted.

## Gates fixed before the candidate

Use the existing pinned offline React/Tiptap/Vite toolchain and installed isolated
Playwright/Chrome; no install, global configuration or consumer write. Preserve exact
parent identities and all candidate source/build/harness bytes before browser execution.
No task expectation or image baseline changes merely to make this candidate pass.

1. Source: exact parent editor/styles, explicit private delta, no original mutation.
2. Build: TypeScript checks; repeat the same private entry build with identical outputs.
   Copied/resolved dependencies are not proof of a fresh dependency installation.
3. Host: Go loopback only, bounded byte-pinned static allowlist, GET/HEAD only, origin/
   Host refusal, no arbitrary file serving or state writes; meaningful no-script notice.
4. Browser: existing editor task checks where applicable,390/1440px light/dark,
   keyboard and pointer; context update while editing preserves node/focus/selection/
   draft/history; invalid update and duplicate mount leave the instance unchanged.
5. Lifecycle: two-instance isolation; dispose while focused; stale-handle refusal;
   same-element remount; document events after disposal cannot produce callbacks.
6. Ownership: host context requests are explicit; local typing/formatting makes no
   request. No outside requests or unexpected console/page errors. No Datastar
   integration claim without an actual independently measured morphing test.
7. Screening budgets: selected entry JS <=500KiB gzip; CSS <=100KiB gzip; first-view
   selected fonts <=350KiB; local input-to-next-paint p95 <=100ms for a named synthetic
   document and device. Measure these separately from build time and whole-demo bytes.
   Full IME/clipboard, native-device, screen-reader, zoom and memory gates stay open
   unless exercised explicitly; synthetic dispatch is not native input proof.

Existing MatDash browser findings and nondeterministic historical builds remain visible.
A passed lifecycle sample is not complete P12 acceptance or permission to promote a
general L3b/L4 component. [Runtime selection](runtime-selection.md) owns the profile choice.

## Prospective representative-document probe

Before tuning, freeze a synthetic256-paragraph ASCII document:255bytes per paragraph
plus255 separating LF bytes,65,535 UTF-8 bytes total. This approaches the initial-text
protocol limit without turning that limit into a recommended document size. Record
the fixture hash and exact paragraph text; no imported HTML or real user content.
Frozen fixture SHA-256: `443d1a3aa090fda81133c5821064dbed9dca14ac6742483d8b1fff1e5ade41ab`.

At1440/390px light/dark, check complete editor/preview agreement, local scrolling,
30 Playwright keyboard edits at the document end, context update without losing node/
focus/selection/revision, and undo/redo. Preserve the100ms p95 two-frame proxy screen;
measure input long tasks separately against the existing50ms task-blocking threshold.
Record mount-to-two-frame time without inventing a threshold after seeing it. Keep
actual paint/IME/mobile-hardware/assistive-technology proof separate.

Exercise ten mount/edit/dispose cycles on the same element with distinct drafts;
all callbacks stop and detached handles refuse. Record Chrome DOM/listener and JS-heap
counters after explicit GC before/after as diagnostics, not a universal leak certificate.
Viewport captures name the editor/task region and full document geometry; an enormous
preview is not hidden or falsely represented as a complete captured document.

## Datastar ownership probe

The [scoped actual-runtime proof](../assessments/2026-09-16-client-editor-datastar.md)
now passes this prospectively declared probe; it does not complete P12 acceptance.

Opt in at private build preparation; ordinary builds retain their current HTML and
routes. Copy the exact Gallery Datastar1.0.3 bundle and adjacent license, verify its
published local pin, and manifest-bind the runtime and first-party proof module.
Keep the existing CSP. Use the runtime's exported `actions.get` with an empty form,
no query/body, zero retries and explicit cancellation; this does not prove declarative
expression compilation, SSE/reconnect, remote hosting or another Datastar version.

Go returns one synthetic GET/HEAD HTML fragment with `Datastar-Mode: outer` and
`Datastar-Selector: #host-fragment`. Both old and incoming fragments contain the same
keyed `#editor-slot[data-ignore-morph]`; surrounding context changes, client descendants
do not. This marker is not an authorization or lifecycle boundary: direct replacement,
removal and an ancestor that omits the slot are outside this preservation contract.
Intentional removal still requires explicit adapter disposal before DOM replacement.

At1440/390px light/dark, require the actual HTTP request and exact response/runtime
identities, changed surrounding text, unchanged slot/editor node, focus, selection,
draft/revision and callback count, pre-morph undo/redo and continued local typing.
The runtime must not add network requests to typing. Verify error/aborted requests
leave the draft intact, dispose/remount still works, and no unexpected runtime/CSP
errors occur. Retain all earlier builds and failed probes; no certification follows.

## Actual browser zoom probe

An additional `--zoom=200` browser trial uses two fresh private Chrome profiles,
light/dark, with the browser's zoom setting at200%, not CSS zoom or device emulation.
Require outer1440px, inner720px, devicePixelRatio2 and visualViewport.scale1 before
and after tasks. Run the same editor, ownership, large-document and optional actual
Datastar tasks. The ordinary four-view and no-script evidence remains independent.
Retain exact helper bytes before execution. Full-document captures use verified CDP
DIP bounds and complete PNG data; the long-document task uses an explicitly labeled
1440x1000 viewport capture with retained document geometry, never a purported full
preview. Keep original failures and existing task/cost expectations unchanged.
