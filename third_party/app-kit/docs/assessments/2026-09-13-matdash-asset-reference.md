# MatDash asset-bound reference — 2026-09-13

## Result

A separate copy of the [repaired native reference](2026-09-13-matdash-native-reference.md)
now carries both declared Google font families, the dashboard map and the source's
named Iconify data locally. Full TypeScript and Vite builds pass. A fresh workspace,
installed offline from the existing cache, reproduced all **383 output files byte-for-byte**:
15,247,074 bytes, without normalization. No original or converted target was certified.

This prepares a stable source-native comparison. It does not establish browser rendering,
application-wide offline operation, accessibility, interaction quality or performance.
The twelve-case pilot still has **0/12 captured source-native visual baselines**.

## What was retained

| Asset | Pinned input and preserved contract |
|---|---|
| Manrope | Exact source Google CSS request, variable weights 200–800, six WOFF2 unicode subsets; all six font-face rules retained. |
| Plus Jakarta Sans | The second font link in the original HTML, weights 400/500/600/700; four unique WOFF2 files and all sixteen font-face rules retained, even though the theme selects Manrope. |
| Map | Source `us-atlas@3` resolved to 3.0.1; range and exact-version responses were byte-identical. Local JSON replaces only its URL, not the projection, geometry or markers. |
| Icons | 309 literal names from fourteen collections, requested from the original Iconify provider; seventeen response batches, no missing names. Original bodies, dimensions and aliases retained. |
| Notices | Font OFL texts, map ISC notice and per-collection icon attribution/license/source links accompany the private copy. No additional redistribution rights are claimed. |

The source API CSS responses are retained before URL rewriting. The ten font binaries
pass byte/hash and WOFF2 header/length checks, not a font-decoder or rendered-glyph test.
Only font URLs change inside the two local stylesheets; their rules, weights, ordering
and unicode ranges remain intact. The editor's explicit Arial/Courier New fallbacks
remain platform fonts, not newly bundled or cross-platform-proven assets.

## Same application design and renderer

The original tree remains 1,245 files with its previous source-tree hash. This copy
adds four source changes beyond the earlier dependency/type repair: local font links
in `index.html`, removal of the remote Manrope import from `globals.css`, a local map
URL, and an entrypoint preload import. The CSS edit also adds a final newline; changed
lines may use LF while untouched source lines retain CRLF. Those byte deltas are recorded.
New data/loader files are separately identified. P11 and the already-repaired P12 are
unchanged from the prior native reference. No class, layout or chart-data rewrite occurred.

[Build metadata](fixtures/matdash-native/asset-build.json) proves six application
stylesheets remain byte-identical. The seventh matches after removing exactly its
leading remote Manrope import. This is a narrow text comparison, not a visual diff.
Both font stylesheets are delivered separately through local links.

The preload uses the same installed Iconify 5.2.1 module before mounting. It changes
neither renderer/import paths nor provider/unknown-name fallback behavior. It checks
every expected name; `addCollection` returning true is insufficient on its own.
The first type check caught heterogeneous JSON inference at the collection boundary.
Generated data now has checked `IconifyJSON[]` contextual typing, with no cast, `any`,
compiler-setting change or excluded source used to silence that error.

## Icon evidence and limits

The TypeScript AST inventory parsed 994 source/config files: 309 distinct literal names,
609 occurrences, fourteen collections and 35 dynamic `Icon` expressions. It includes
unselected routes and may include unused literals. Selected shell expressions forward
names from static data, but this is not a whole-program dataflow or browser request proof.

An isolated Node audit loaded only the pinned Iconify/React library, not the MatDash
application, DOM, component or hook. With fetch replaced by a failing counter, all
309 expected names resolved with **zero fetch calls**. Nine requested names are aliases;
their nine canonical parents are also loaded, giving 318 registry entries. Every
observed alias preserves the parent's resolved data. A separate synthetic canary tests
default dimensions and transformed aliases. Repeated lookup evidence is byte-identical.

Restricted SVG-body checks found seven tags, 24 attributes and one valid local mask
reference, without scripts, event/style attributes or remote references. This is not a
general sanitizer or a security certification. Existing `alert-outline` and `Vector`
classes remain potential CSS-collision cases for browser inspection. No glyph geometry,
accessibility name, displayed size or mount/update behavior was verified in a browser.

## Reproduction and private storage

[Receipt](fixtures/matdash-native/asset-receipt.json) identifies the original source TAR,
earlier repair TAR, new asset overlay, captured upstream inputs, manifests, toolchain,
commands and checks. The two new ignored, owner-only TARs under
`data/design-assets/reference-assets/matdash/` retain the overlay and acquisition evidence.
They are local snapshots, not official vendor releases or new Tailwind archive entries.
Back them up separately; Git does not protect this private data.

Extract source, repair and asset overlay in that order into a fresh owned directory.
Keep the exact existing lock and strict npm configuration. Replay used `npm ci --offline`
with lifecycle hooks disabled, empty user/global configs and the already populated
task-local cache, followed by the pinned TypeScript and Vite CLIs. The cache is not
independently archived; its absence is not a successful offline-build check.

The [asset launcher](fixtures/matdash-native/asset-reference.mjs) uses the same bounded,
read-only static server as the prior reference, with a different hard-coded manifest pin.
Neither HTTP nor CLI can select arbitrary metadata. The original launcher retains its
original pin and behavior; its expanded regression suite passes 57 checks.

```sh
node docs/assessments/fixtures/matdash-native/asset-reference.mjs /absolute/prepared/main
```

The launcher reports its ephemeral loopback URL. Ctrl-C closes only its own listener.
The [opt-in asset suite](fixtures/matdash-native/asset-reference.test.mjs) requires an
explicit prepared root. Its 14 checks and the shared 57 checks pass **71/71**, with zero
skips; an independent asset-suite run also passes. HTML responses do not prove React mounted.

## Gates still open

At asset preparation Chrome was unavailable; no other browser was substituted. Font decoding, dashboard
and editor tasks, keyboard/pointer/focus, narrow/wide, theme/zoom and actual data results
still need inspection. Tiptap's duplicate code-block registration remains unresolved.
User-supplied image URLs, unknown icon names and unmatched mock API requests can still
use the network. No whole-application network isolation or offline-editing claim follows.

Preloading full-demo literal icons adds payload; the output totals include unselected
routes and are not per-route transfer or latency measurements. Vite's large-chunk warning
remains enabled. Measure costs before promoting a consumer recipe or conversion.
Impeccable guided preservation of the complete source composition; ADR-0022/0023 keep
its original identity, repaired variant and eventual conversion evidence distinct.

## Later browser follow-up

The approved isolated [browser audit](2026-09-13-paid-reference-browser.md) now captures
P11/P12 at 390/1440 widths in both themes, observes loaded Manrope and settled charts,
and exercises actual editor typing/bold/undo/redo. Narrow editor overflow, unnamed
toolbar controls and duplicate-extension warnings remain failing checks. The full task,
accessibility, zoom, data and performance matrix is not complete; no target is certified.
