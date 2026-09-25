# P12: client-owned rich editing in a Go-hosted page

The [ownership contract](../design/client-editor-island.md) now has a working private
specimen and scoped browser evidence. This advances G1/G4 and the client-heavy strategy;
it does **not** complete P12, convert the editor to Go, add a public widget API or raise
the certified-target count above zero.

## Implementation

The hardened MatDash React/Tiptap editor remains the editing implementation. Its
toolbar, preview, CSS and nine source icons are preserved. An explicit private delta
accepts plain initial text, contextual labels/hints and an editor-ready callback.
The first-party adapter provides typed mount/update/snapshot/dispose, one live owner
per element and instance ID, bounded UTF-8 inputs and detached snapshots. Context
cannot replace the document. Disposal releases the instance and rejects stale handles.

Go serves a byte-pinned asset allowlist and synthetic context endpoint on loopback.
It is not an editing RPC or save backend. Context updates leave the editor subtree
and document under React/Tiptap ownership. Status changes once when a local draft
changes, not once per keystroke/revision. Paid bodies remain private. No original,
consumer, dependency manifest, global configuration or signed-in profile changed.
No install, commit or publication. Impeccable's extraction guidance kept this a private
pilot instead of prematurely promoting a public widget from one use case.

## Evidence

| Gate | Observed result | Limit |
|---|---|---|
| Parent |1,269 source files/13,940,069 bytes reverified; selected source/icons/CSS/image/fonts pinned | hardened parent, not untouched original |
| Protocol | seven Node test groups pass: shape/version, Unicode/byte bounds, no coercion/accessors, context-only updates | not an untrusted-code sandbox |
| Go host | five synthetic test functions with refusal subcases pass under race | not production hosting/auth |
| Build | TypeScript passes; two outputs match,17 files/1,036,807 bytes | reused dependencies, not clean-install proof |
| Live host | ten HTTP probes pass: exact CSP, GET/HEAD, Host/Origin/fetch-metadata, methods/routes | actual binary/source/startup bound in trial |
| Browser |136 task/global assertions across1440/390px light/dark plus no-script |64 are16 existing editor assertions repeated in four views |
| Assets |66 response checks across eight unique requested paths | repeated observations, not66 independent capabilities |
| Captures |16 full-page PNGs with geometry/hashes | representative initial/code/image views inspected; no pixel-equivalence claim |
| Cleanup | owned Go child exits0 after SIGTERM, no forced kill | targeted owned PID only; no other-preview inventory |

Tasks cover formatting, headings/lists, code languages, undo/redo, link/image refusals,
empty/long content and reload reset. Added checks cover context preserving node/focus/
selection/draft/revision/history; invalid updates/duplicate IDs/disconnected mounts;
snapshot detachment; plain-text initialization; two-instance isolation; actual context
HTTP; disconnected typing without requests; quiet status; focused disposal, stale
handles, no late callbacks and remount. No unexpected network destination, runtime
error or browser-console error was observed.

## Cost screening

Predeclared budgets: JS500KiB gzip, CSS100KiB gzip, first-view fonts350KiB.
Observed: **220,689 JS bytes gzip;32,545 CSS bytes gzip;24,836 first-view font bytes**.
All retained font subsets total134,116 bytes. Source base CSS has unused selectors;
this is not production CSS pruning or complete original-app asset closure.

Apple M4/32GiB, Darwin25.6.0, Chrome152.0.7977.84:30 Playwright keyboard inputs per view,
input-listener to second-animation-frame p95 **31.6–32.3ms**, below100ms screening.
The final document was74 characters. This is a small-document paint-latency **proxy**,
not physical paint, large-document, heap, long-task, mobile-device or IME proof.
The retained receipt's word "native" denotes Playwright browser-input automation only;
the current harness corrects that description without changing task assertions.
Toolchain: Node26.3.0, TypeScript5.5.4, Vite5.4.21, React/ReactDOM18.3.1,
Tiptap2.27.3, Lowlight2.9.0, Iconify5.2.1, Go1.26.3, Playwright1.63.0.
Vite raw chunk-size and ignored Iconify `use client` warnings remain visible.

## Retained identities and corrections

Private root: `data/design-assets/p12-client-island.UM4cXe/`.
Current build: `build-nrRmtg/`; trial: `trial-ES1QEj/`; browser: `browser-zpjubg/`.

- Parent manifest: `2c3d53eea0ffb3247aa1b7247c8869fd3fa9200ea6f0e48c8eede627eb6f4007`.
- Output manifest: `10e038a49012cd262c89fb257c7afc7a7225335a8815530dbd60d20df640cc04`.
- Host source: `0cf600dd7cee4fd51b6f59b83a1bf3a891ac1616ed13d9bf4eeb709266fceaff`.
- Binary: `5519c635c63afcbae929e0a8193d5e1c4aa4bb8f83207f9a1d005d3684e58ebd`.
- Trial receipt: `01645671749af157c388d45e57a41bf3c2936f42012d42212a8a4528df2adfa6`.
- Browser receipt: `b5fe82f4e786ba7898985610801bda434efdee050a88ef5765fe677adb29b072`.

Earlier `build-nkbRqV`/`build-W4qUuq` remain. The first browser's192 passing observations
lacked live-host binding and no-script network instrumentation; it is not relabeled
final proof. Independent review prompted those controls, static-asset pinning,
duplicate live-ID refusal, allocation-bounded validation, meaningful completion checks
and separated counts. Final status-copy revision has a separate build identity.
[Replay instructions](fixtures/client-editor/README.md) describe retained inputs and
harness/config snapshots. Race snapshots: `/Volumes/m2ext/appkit-evidence/appkit-p12-host.YzfCdD4F`
and `/Volumes/m2ext/appkit-evidence/appkit-p12-host.haEtYcnG` (latter has image-only data CSP).
Within the private root, `proofs-HDYFxX/receipt.json` separately records the full parent
inventory and fresh race execution; its exact verifier, manifest, Go source/tests and
`race.log` are retained. Seven protocol groups are recorded in
`build-nrRmtg/protocol.log`, not inferred from a build receipt's gate label.

## Representative-document follow-up

The unchanged `build-nrRmtg` editor now also passes the prospective65,535-byte,
256-paragraph workload at1440/390px light/dark. `trial-baNcZp` binds the same host
and asset identities; `browser-2cgawj` records181 task/global assertions,66 asset
observations across eight URLs,20 captures and no failures. Its44 scale assertions
cover exact editor/preview text, scrolling,30 edits, context preservation, pre-update
undo/redo and ten mount/edit/dispose cycles per view. Ten live-host probes pass;
owned shutdown exits0 without forced termination.

Input-to-second-frame p95 was31.0–31.2ms, with no observed input-window long task
above50ms. Mount-to-two-frame time was16.5–32.2ms (diagnostic, no declared gate).
After explicit GC, each view retained12 additional DOM nodes, zero additional event
listeners and804,904–815,364 additional heap bytes. These deltas are recorded, not
called leak-free: the page includes other test-held references and allocator caches.
The two inspected large-document captures show readable local editor scrolling in
desktop light/narrow dark; they are viewport task views, not the full long preview.

The failed `trial-viH1j0`/`browser-jRYx8a` remain intact. Context changed the editor's
accessible name, invalidating the old role locator; zero matches falsely looked like
undo success and redo timed out. The corrected harness uses a stable editor locator
with explicit initial/renamed accessibility and live-editor prerequisites. No editor
code, workload, timing threshold or expected history behavior changed.

- Trial SHA-256: `a0f080d4f4644352b57e1cb3dfd8fdddd742396fa66cca8c3a23820f020f0976`.
- Browser SHA-256: `87e9b4185c7ff63e947503af38bb69aa4132a5bbf1414fe46a8d89412efba3cd`.

## Open acceptance

IME/clipboard/assistive technology, actual200% zoom, broader document/memory workloads,
general Datastar integration, imported-rich-content safety, source-fidelity comparison,
clean dependency closure and real-consumer adoption remain open. Disconnected typing
is not persisted offline work. No save/sync backend, sanitizer, collaboration framework,
Wasm runtime or new public catalog component is implied. Originals remain available.
The separate [actual Datastar outer-morph probe](2026-09-16-client-editor-datastar.md)
now passes its bounded preservation/failure/cancellation tasks, not these full gates.
