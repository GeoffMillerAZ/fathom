# Catalyst source-native reference rehearsal — 2026-09-13

## Result

Frozen pilot P10 now has a successfully built source-native Catalyst settings reference
in a private workspace, served by a deliberately read-only loopback launcher. A second
fresh workspace installed the locked dependencies from the first cache with npm offline
mode, built successfully and passed the same HTTP checks. **No visual baseline is captured.**
This advances [G1/G4](../intent/scope.md), not a verified conversion or consumer migration.

Use the original alongside the [Gallery shell](../design/application-shell.md) to review
hierarchy, description/control columns, spacing, controls and narrow-screen navigation.
Their field schemas and save behavior differ; this is not a claim of task equivalence.
The original remains a valid React-profile choice, not a failed Go conversion.

## Frozen inputs and observed working-copy changes

Archive: `catalyst@94acb41796a8a97fdef44bf5b2d18f42919e43300cbca90077775f6b36dfa529`.
Only `catalyst-ui-kit/demo/typescript/` was materialized: 75 regular files,
2,696,269 bytes and 18 assets. The complete subtree retains shared layout/navigation,
components, fixture data, public assets, configuration, lockfile, README and license.
The other demo/component trees were not copied. The archive's empty `.git/` directory
is vendor directory metadata, not a new repository initialized by this rehearsal.

Original Downloads and stored ZIP hashes remain identical. All 75 materialized input
files matched the archive before building. Afterward, 74 still matched; Next generated
an extra routes-type import and updated a comment in `next-env.d.ts`. Both builds made
the same declaration-file change. No component, CSS, package, lock or tsconfig was edited.
Generated `.next` and installed `node_modules` stay in the temporary workspaces.
The [receipt](fixtures/catalyst-native/receipt.json) and [replay inventory](fixtures/catalyst-native/replay-evidence.json)
record hashes, commands, source changes and outputs. No paid source is copied into these fixtures.

Observed toolchain: macOS 26.6.2/arm64, Node 26.3.0, npm 11.16.0; locked Next 16.2.6,
React/React DOM 19.2.6, Headless UI 2.2.10, motion 12.38.0 and Tailwind 4.3.3.
The supplied manifest does not pin Node/npm. All 456 dependency records in lockfile v3
have exact versions, registry tarball URLs and integrity values. The installed platform
closure contains 383 packages; installed versions and lock metadata match those records.
That is not an independent byte audit of every installed package or a security assessment.

## Preparing another private reference

Run from app-kit. Verify the selected archive with existing `appkit data assets-get`
before extraction. Create a fresh private temporary directory with `mktemp -d`; extract
only the reviewed TypeScript prefix from that exact ZIP, never all purchased projects.
Keep the license and original lockfile. Do not extract over an existing worktree.

The exact historical npm/build commands and cache paths are in the receipts. Their
important constraints are `npm ci`, `--ignore-scripts`, `--no-audit`, `--no-fund`,
separate empty user/global config paths, a task-local cache and a minimal environment.
The initial build failed because my sanitized environment omitted PATH; retaining
`/opt/homebrew/bin:/usr/bin:/bin` fixed that setup error without a source patch.
Build with `NEXT_TELEMETRY_DISABLED=1`; use the reviewed installed Next CLI, not an
unversioned `npx` download. Sanitizing environment variables is not an OS sandbox.

The first prepared root is `/tmp/appkit-catalyst-reference.hY6KVV/catalyst-ui-kit/demo/typescript`.
The replay root is `/tmp/appkit-catalyst-replay.WLXXFC/catalyst-ui-kit/demo/typescript`.
These are disposable local paths, not a distribution or permanent asset-backup location.
For an already prepared, reviewed P10 root, run the repo-authored fixture explicitly:

```sh
APPKIT_CATALYST_REFERENCE_ROOT=/absolute/prepared/catalyst-project NEXT_TELEMETRY_DISABLED=1 node --test docs/assessments/fixtures/catalyst-native/reference-server.test.mjs
NEXT_TELEMETRY_DISABLED=1 node docs/assessments/fixtures/catalyst-native/reference-server.mjs /absolute/prepared/catalyst-project
```

The test refuses missing selection or mismatched package/lock/P10/layout identities
before loading Next; both refusal canaries were exercised. It does not install anything.
The launcher requires an explicit reviewed absolute project; it is a rehearsal helper,
not an automatic executor, source-integrity verifier, MCP tool or reusable runtime API.
No Node dependency is added to kit-core or to Go/portable consumers.

## Read-only delivery evidence

The launcher binds one ephemeral `127.0.0.1` listener and gives Next its actual port.
Next's port-zero default and automatic upgrade registration were checked in the installed
implementation; the outer server rejects upgrades and closes both the listener and
Next runtime on shutdown. There is no separate unguarded Next TCP server.
Only GET/HEAD pass. Loopback peer, exact Host and supplied Origin/Fetch Metadata are
checked; ambiguous authority headers and read bodies are refused. This is not user auth.
Its framing/form CSP is not a production script policy or proof of browser security.

The fixture suite passed **42 tests, zero failures/skips, on each built workspace**:

- `/settings` returns the original title, organization fields and fixture values;
  local CSS and one script named by its HTML return nonempty successful responses.
- HEAD/query requests work; POST returns 405 with no invented save result. The vendor
  form has no save handler, so a successful rendering must not imply working persistence.
- Foreign/ambiguous authority, non-loopback forwarded-authority claims, bodies and
  unsupported methods/targets are refused; an unknown route returns 404.
- After ordinary requests prime Next's upgrade handler, a websocket attempt closes
  without handshake bytes. Closing the fixture makes another connection fail.

The first independent launcher process, PID 37273, served
`http://127.0.0.1:53300/settings` with HTTP 200 and 86,371 uncompressed HTML bytes.
That URL belongs to the observed local process, not a stable Gallery route. The HTTP
suite does not fetch external fonts, hydrate React, click controls or measure rendering.
It is opt-in evidence, not added to the ordinary corpus-free `task check` gate.

## Replay result and remaining fidelity gates

Offline npm installation succeeded using the previously populated cache; it does not
establish a separately archived dependency supply or offline browser operation.
Of 12 raw output comparisons, seven matched and five differed. CSS, settings server JS
and selected route/client manifests matched; BUILD_ID, settings HTML/RSC and build/
prerender manifests differed. No output was normalized or baseline replaced to make
these equal. A byte-reproducible native build has **not** been demonstrated.

The original layout loads `https://rsms.me/inter/inter.css`; no font binaries are in
the archive. The stylesheet/font revisions remain unpinned. Freeze those assets and
conditions before a controlled visual comparison. Do not silently substitute a font
or label an online fallback an offline pass. [Pilot gates](../design/conversion-pipeline.md)
still require 390/1440 widths, dark/light, zoom, keyboard/pointer/focus, hydration and
reviewed source/adaptation differences. Chrome remained unavailable after revalidation.
All 12 pilot cases remain without captured source-native visual baselines; no target
was certified and no existing original or conversion was hidden or replaced.

MatDash P11/P12 remain a separate source-native preparation task. The selected main
package lacks a lockfile; its dashboard uses fixtures but imports external fonts/map
data and has unresolved icon-resource closure. The editor has an undeclared direct
`lowlight` import. Fixture registration passes unmatched Axios requests through, so it
is not network isolation. Do not install another package's lock or advertise a MatDash
preview until that selected project's dependency/assets and runtime checks pass.
