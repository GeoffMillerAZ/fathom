# MatDash reference reconstruction — 2026-09-13

## Result and scope

The complete local MatDash `main` project now has an explicitly repaired, privately
replayable build. Strict dependency resolution, a full TypeScript check and the Vite
production build pass. A fresh workspace installed from the local cache with npm offline
mode and reproduced **all 366 output files byte-for-byte**, totaling 14,627,979 bytes.
This is controlled build evidence, not browser, visual, interaction or performance proof.

The selected [pilot](2026-09-13-conversion-pilot.md) cases remain P11 Dashboard2 and P12
the editor hosted by the original add/edit product pages. The full layout, providers,
navigation, source pages, data and assets were retained; no smaller substitute dashboard
or stripped-down editor was built to pass. No conversion or kit target was certified.

## Original preserved; repair separately identified

The supplied `data/matdash/packages/main/` contains 1,245 regular files / 12,392,681 bytes,
no lockfile, and no observed symlinks or special files. Its sorted path/hash/size inventory
has SHA-256 `0f22a03dea837acca1a196eb87a58327fb38c58d15fa57014b995e4a27611d42`.
A locally created TAR preserves that input. It is **not an official vendor ZIP**, a
verified vendor release or a new entry in the Tailwind archive catalog.

The original directory and the untouched first temporary copy remain byte-identical.
The repair changes exactly four existing files; 1,241 remain identical. A new lockfile
is separate. P11 retains its frozen source hash. P12 differs only by its declared
Lowlight import repair, so its derivative must not replace the frozen original identity.
CSS, Tailwind configuration, layout, chart data and visual classes were not edited.

Both source snapshot and five-member repair overlay are stored separately under ignored
`data/design-assets/reference-assets/matdash/`, named by their TAR SHA-256. They have
owner-only file permissions. [Receipt](fixtures/matdash-native/receipt.json) records
their identities, members and before/after source hashes. Back these private files up;
Git is not their recovery mechanism. No purchased source body is in the assessment fixtures.

## Actual failures and deliberate repairs

The supplied `.npmrc` sets `legacy-peer-deps=true`. The first lock-only resolution
explicitly restored strict checks and failed: React 19 conflicted with CASL's declared
peer range. Registry inspection found further React 19 exclusions in the supplied DnD,
Quill, map and Redux versions. A bypassed peer tree is not compatible-source evidence.

| Repair, confined to the reference | Reason and preserved boundary |
|---|---|
| React/DOM `18.3.1`, matching existing React 18 type ranges | Explicit departure from the source's React 19 declaration; fits the older supplied component contracts. Not an app-kit default, React 19 upgrade or new-app recommendation. |
| `react-apexcharts` `1.4.1`, retaining ApexCharts `3.48.0` | The range now reaches a wrapper requiring ApexCharts 4+. Keep the original chart engine, callbacks and styling. |
| `mui-tiptap` `1.13.0` | A newer in-range release selected Tiptap 3 peers against the project's Tiptap 2 core. Keep the original declared generation. |
| Remove unused `react-spring` umbrella | Strict resolution exposed incompatible 3D/native peers. Source/config inspection found no imports; no animation implementation was removed. |
| Declare Lowlight `2.9.0` and four previously hoisted imports | Compiler-AST inspection found direct uses of `@dnd-kit/utilities`, `clsx`, `react-icons`, `tailwind-merge`; exact installed versions are now declared. |
| Named Lowlight `lib/core.js` import | The old default/subpath did not match the actual ESM package. Retain core-only behavior; do not add the common bundle's 37 grammars. |
| Nullable selection in `BasicComobobox.tsx` | Full-project TypeScript reported TS2322 for a handler rejecting `Person \| null`. State/handler now admit null and the display callback returns an empty string. No type assertion or compiler flag bypass. |
| `.npmrc` disables legacy peers and enables strict peers | Normal replay retains the checks rather than depending on a one-off command override. |

The original [wrapper metadata](https://registry.npmjs.org/react-apexcharts/1.4.1)
and [newer wrapper metadata](https://registry.npmjs.org/react-apexcharts/1.9.0) corroborate
the chart constraint. Installed package bytes, not names alone, established Lowlight's
named export and Tiptap's accepted highlighting interface. Package provenance and exact
versions are in the lock; successful resolution is not a vulnerability assessment.

## Build and replay evidence

Toolchain: macOS 26.6.2/arm64, Node 26.3.0, npm 11.16.0, TypeScript 5.5.4, Vite 5.4.21.
The lock has 1,084 dependency records with exact versions, registry HTTPS locations and
SHA512 integrity values. Installation added 1,011 platform-selected packages with scripts
disabled. A complete `npm ls --all` check returned exit 0 and no reported problems;
installed versions and installed-lock metadata matched. Individual installed package
bodies were not independently audited. Native modules whose hooks were skipped must
not be advertised as runtime-verified merely because the build did not need them.

The supplied full `tsc` and Vite stages both ran; strict/unused compiler settings remain
unchanged. The first full type check failed on the nullable combobox, and the declared
repair made it pass. No source routes or compiler errors were excluded. The fresh replay
also passed both stages, with no dependency downloads during `npm ci --offline`.

The raw sorted output path/hash/size inventory is
`02edbaaa65f1a3c0a6ed314afe02022f102e3c02b06b1281a1483f5330639efc`
in both builds. No byte normalization or baseline substitution was used. This proves
the observed two-build replay under these inputs, not deterministic output on every OS,
independent dependency-backup completeness or offline operation in a browser.

Vite still warns about chunks over 500 kB. The demo includes a roughly 1.22 MB minified
entry chunk, chart runtime and code-example viewers. These are artifact sizes, not a
measured route request graph or user-perceived latency. Keep that cost visible when
adopting a complete composition; do not raise warning limits to call it optimized.

## Reproduce the selected private build

Verify the exact source TAR, overlay TAR and their member inventories against the
receipt before extraction. Extract source, then the five reviewed overlay members, into
a **fresh** task-owned directory. Never apply this overlay to the supplied original,
another MatDash package or a consumer. Keep the new lockfile and strict `.npmrc`.

Use `npm ci --offline --ignore-scripts --legacy-peer-deps=false --strict-peer-deps` with
the already populated task-local cache, empty user/global configurations and a minimal
environment. The exact successful commands and roots are in the receipt. Cache absence
is a prerequisite failure, not permission for an unrecorded network resolution.
Then run the pinned installed TypeScript and Vite CLIs. No unversioned `npx` or global
installation is required; nothing here adds Node dependencies to portable/Go kit layers.

## Guarded static reference

The [local helper](fixtures/matdash-native/reference-server.mjs) pins the reviewed
metadata before parsing it, verifies 17 selected source files and the exact 366-file
build, and serves cached verified bytes from one ephemeral loopback listener. It never
loads the vendor runtime on the server. Exact authority/origin checks, GET/HEAD-only
requests, no-store responses and frame/form restrictions bound this private preview.
It is not authentication, script/outbound-network isolation or an OS sandbox.

The opt-in [suite](fixtures/matdash-native/reference-server.test.mjs) initially passed
42 checks; its expanded shared-helper suite passes **57/57**, with zero skips. It covers
all asset bytes/hash/MIME, 133 declared client-route shell
responses, request refusal, missing/corrupt/symlink/FIFO inputs, cached-byte isolation
and actual CLI SIGINT/SIGTERM shutdown. Unknown routes/assets/APIs remain 404.
Serving a route's HTML shell is not evidence that React mounted or the route rendered.
The receipt pins the helper, tests and metadata and records the explicit replay command.

From the checkout, pass the absolute verified build root explicitly:

```sh
node docs/assessments/fixtures/matdash-native/reference-server.mjs /absolute/prepared/main
```

The helper prints the selected local dashboard URL; Ctrl-C closes its own listener.
This is an opt-in private fixture, not automatic execution from Gallery or MCP.
A separate [asset-bound variant](2026-09-13-matdash-asset-reference.md) now pins fonts,
map and named icon data without changing this earlier source/build identity.

## Remaining runtime and design gates

The build does not mount React or prove that a route works. Fonts still load from Google,
the geography widget still references external us-atlas data, and named Iconify assets
need a pinned closure. Axios mocks pass unmatched requests through; they are not network
isolation. No live consumer data, secrets or production backend was used by this rehearsal.

The original editor registers both StarterKit's code block and CodeBlockLowlight; core
Lowlight has no grammars registered. Duplicate-extension behavior, highlighting, undo,
IME, selection, clipboard, HTML safety and teardown need actual runtime checks. The
import repair does not silently fix or claim those behaviors. Saving is not a real
application backend simply because a demo button exists.

Chrome remained disconnected. No alternate browser was substituted. Preserve the
12-case denominator: **0/12 captured source-native visual baselines**, including this
repaired reference. Still required: the actual dashboard/editor task, expected data,
keyboard/pointer/focus, narrow/wide, theme/zoom, payload/update budgets and comparison
against an adaptation. ADR-0022's reconstruction trigger is handled by a separate
source/repair/replay record, not by changing originals or weakening ADR-0023's proof bar.
