# Private Go-hosted client editor pilot

P12 exercises [the ownership contract](../../../design/client-editor-island.md).
This is a source-native React/Tiptap island served by Go, not a rich-editor rewrite,
public widget API, production scaffold or certified conversion.

## Inputs and private boundary

The paid editor/styles and generated icon bodies stay under a mode0700 private
candidate. First-party adapter, protocol, host and tests live here. The independently
preserved MatDash original and hardened parent remain unchanged. The selected local
parent is `/private/tmp/appkit-matdash-hardened.3Chjkt`; source-manifest identity is
`2c3d53eea0ffb3247aa1b7247c8869fd3fa9200ea6f0e48c8eede627eb6f4007`.
Do not reconstruct a missing parent by installing dependencies implicitly.

The private candidate has `source/TiptapEdit.tsx` (explicit lifecycle/context delta)
and byte-identical `source/Tiptap.css`. Historical candidates are not overwritten.
`build.mjs` verifies selected parent source and static asset pins, snapshots all
selected inputs, reuses the existing isolated dependency directory, checks TypeScript
and protocol cases, and produces two compared builds in a fresh private directory.
The whole dependency tree is not content-pinned by the installed-lock hash.

```sh
node docs/assessments/fixtures/client-editor/build.mjs \
  /private/tmp/appkit-matdash-hardened.3Chjkt \
  /Volumes/m2ext/code/geoffmilleraz/app-kit/data/design-assets/p12-client-island.UM4cXe
```

Each output manifest allowlists JS/CSS/fonts and the selected synthetic image.
The source base stylesheet is retained, including unused selectors. This is not CSS
tree-shaking or complete original-app asset closure. Browser checks cover used URLs.
Vite's raw chunk-size and Iconify `use client` warnings remain visible; declared
compressed budgets are checked separately and a budget failure exits nonzero.

## Run and inspect

Use the new absolute `build-*` directory printed by the build:

```sh
node docs/assessments/fixtures/client-editor/run-browser.mjs ABSOLUTE_BUILD_DIRECTORY
```

The runner retains its exact harness/helper/host source, compiles with existing offline
Go caches, launches that binary on an ephemeral loopback origin, records startup and
live request guards, runs isolated installed Chrome, then stops only its owned host.
It never uses a signed-in browser profile or requests the Tailwind portal. Retained
receipts/screenshots stay private. Follow the reported `trial-*` and `browser-*` paths.

The browser reuses the existing 16 editor-task assertions per view, then verifies
context ownership, atomic refusal, snapshot detachment, two-instance isolation,
disconnected typing, quiet status, disposal and explicit remounting. It tests1440/390px
light/dark and a no-script view. Task assertions, repeated asset observations and unique
asset identities are separate counts. Performance is a30-key Playwright, small-document,
input-to-two-animation-frame proxy, not physical paint, long-document or IME proof.

The scale helper separately exercises a frozen65,535-byte/256-paragraph document,
30 edits, pre-context formatting history and ten mount/edit/dispose cycles per view.
It retains input long-task observations and explicit-GC counters; neither is a general
performance or leak certificate. Scale captures show the viewport task region only.

## Optional pinned Datastar probe

Append `--datastar` to the build command to copy the exact Gallery1.0.3 runtime,
adjacent MIT license and first-party proof module into a new private candidate. The
host refuses partial reserved asset sets or runtime/license pin drift. Ordinary
builds retain their existing HTML/routes; no runtime is added to kit-core.

The browser then exercises the real exported GET action against `/morph-fragment`,
an outer morph retaining a paired keyed `data-ignore-morph` editor slot. It checks
surrounding changes, editor state/history and local typing, plus separately labeled
intercepted503/cancelled-response failures. This is not declarative-expression,
arbitrary ancestor removal, SSE/reconnect or server-side editing proof. The existing
script policy is unchanged; no inline scripts or `unsafe-eval` are enabled.

## Actual browser zoom

Append `--zoom=200` to `run-browser.mjs` (not the build command). This creates a
separate retained trial with two fresh private Chrome profiles, sets real browser
zoom and verifies1440px outer/720px inner geometry, DPR2 and visual scale1. Both
themes run the same editor, ownership, scale and optional Datastar tasks. The
ordinary four-view run remains a separate regression requirement; no-script stays
an explicitly100% check. No signed-in profile or global settings are changed.

The runner retains and identity-binds both shared zoom helpers. Full captures verify
complete DIP bounds and decoded PNG rows; large-document images are explicitly
viewport-only. Profiles and all receipts remain under the private browser directory.
[Measured results and limits](../../2026-09-16-client-editor-zoom.md).

## Boundaries still open

No backend save, imported rich document, sanitizer, collaborative/offline durability,
general Datastar lifecycle integration, full IME/clipboard/assistive-technology validation, real
consumer adoption or blanket performance/accessibility certification is supplied.
Snapshot documents remain untrusted when a host consumes them. Consumer authority
must never be inferred from client-side labels or instance identifiers.

Public compilation of this fixture intentionally lacks the private `TiptapEdit` module
and React toolchain. Kit-core/portable CSS consumers acquire no Node dependency.
