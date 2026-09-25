# MatDash: private hardened editor reference

Date: 2026-09-14. Separate source-native React derivative under [ADR-0022](../adr/0022-private-design-references-and-fidelity.md) and
[ADR-0023](../adr/0023-profile-specific-conversion-and-client-compute.md).
The original, strict-dependency repair and [asset-bound reference](2026-09-13-matdash-asset-reference.md)
remain unchanged. The [original browser audit](2026-09-13-paid-reference-browser.md)
retains its failures; no conversion or consumer maturity is promoted here.

## Changed behavior

- Preserve MatDash's composition, fonts, light/dark surfaces and all 13 editor controls.
  Name every control; show pressed/disabled/focus states; use native buttons in named
  groups, 44-pixel targets and wrapping groups instead of overflowing the narrow page.
- Label the editable textbox and rendered-preview region. Initialize both from the
  same document; disclose that reload clears the unsaved local draft.
- Give code blocks one extension owner. New blocks default to plain text; a native
  selector offers plain text, JavaScript, TypeScript, JSON and Go. Marks unavailable
  in code are disabled; long code scrolls locally and prose wraps.
- Preserve the selection during mouse toolbar activation. Normalize whole-document
  selections to a text range only for formatting, retaining ordinary Select All for
  document replacement. Keep list switching available when Tiptap 2's dry-run check
  incorrectly rejects the supported operation.
- Link cancel/set/remove, image cancel/insert and undo/redo keep preview agreement.
  The image control accepts only same-origin HTTP(S), refuses credentials, and asks
  for explicit alternative text (empty only for a deliberately decorative image).

These controls are not an editor security boundary: pasted/imported content and
clicked preview links need app-owned policy. There is no save, upload, sanitation,
authorization, collaborative editing or offline reconciliation backend.

## Private source and replay

Only `TiptapEdit.tsx` and `Tiptap.css` change. Complete selected source/config/public/
reference-input inventories contain 1,269 files; package/lockfile and 1,267 other
files are unchanged. Source bodies, base TAR, patch, logs and screenshots stay in
Git-ignored `data/design-assets/matdash-hardened-v4.N7dfOm/`; back it up separately.

First-party metadata and opt-in checks are in [the fixture](fixtures/matdash-hardened/reference.mjs).
Source identity covers explicit roots, excludes dependencies/generated output, and
rejects missing/extra/changed files and symlinks. The static server independently
checks 24 source pins and the complete 383-file build before binding loopback.
Tests join the pins to the full source inventory and recompute the two-file delta.
No vendor code executes in that server; selected JavaScript executes in the browser.

| Evidence identity | SHA-256 |
|---|---|
| Source manifest | `2c3d53eea0ffb3247aa1b7247c8869fd3fa9200ea6f0e48c8eede627eb6f4007` |
| Build metadata | `efa4bba0c8fb5da36eded564a717a034c47ac2ca22796a5797ef14be0f20cf54` |
| Private patch | `0d7920bf18c775efda6cd4ac0e69172688f0e825658b920f6f2cf88414d572b5` |
| Browser receipt | `3499011855a7e86d28745d5f237884fcfe399d41cdd0c2534b8be11eed646dcd` |
| Replay receipt (byte comparison fails) | `0ec1320bc4533b995d1de06c4b237d252558077550df2dc0b1722ef9a0bbc4d5` |
| Private tested build TAR | `bd8270d469b89205cab36eb1ee68c1c5173bba9790d280eb02cb2d9dcbe2d096` |

Final policy/source/delivery tests: **11/11 pass**, no skips; all 383 asset identities
and 133 routes are checked. Final browser run: **112/112 pass** in Chrome
152.0.7977.84, using the existing pinned Playwright 1.63.0 entry/package. Eight P11/P12
views at 1440/390 pixels and light/dark schemes, plus eight editor-state and four
narrow viewport captures, retain 20 hash-verified PNGs. All 174 observed responses
match pinned bytes; zero outside/write requests, page errors or console warnings.

The task probe covers initial/empty/reloaded drafts, all 13 button names/44px targets,
Tab/Space/focus including history and the conditional language selector, formatting/
list roundtrips, code languages, local scrolling, link cancel/set/remove, image alt/
cancel/undo/redo and four refused URL classes. Seven chart geometries settle before
each P11 capture. Root inspected all eight editor-state images and the dark P11
desktop/narrow captures; no editor overflow or missing dark hero/illustration observed.
Independent review of all eight main views found no substantive visible regression.

Final archive → patch → source verification and both TypeScript/production builds
pass, with unchanged selected source. **Build-byte reproducibility fails**: replay
has 383 files / 15,389,733 bytes versus the pinned build's 15,389,163. Main-bundle
minified identifiers differ and generated import hashes cascade: 157 files match,
225 generated paths change, and `index.html` differs. Cause/equivalence
is not established. The strict replay command exits 1 and retains its receipt.
An earlier candidate's exact build replay is not substituted for this revision.
Do not promote or overwrite the pinned browser-tested build with the differing replay.
The private packet retains `tested-dist.tar` and `restore-tested.mjs`. Recovery checks
all three archive/patch pins, reconstructs a fresh source/build root without dependencies,
then passes full source/build verification and a loopback route check. This earned
artifact recovery, not build reproducibility; the distinct failing replay stays intact.

Follow-up controlled builds from the same candidate root, exact Node/Vite entrypoint
and allowlisted environment reproduced both outcomes: first matched replay exactly
(9,743 transform-hook calls; 15,389,733 bytes), second matched the pinned build exactly
(9,689 calls; 15,389,163 bytes). Thus same-root output is nondeterministic; root or
invocation differences are not required. Collision-aware normalization paired all 383
assets and left one structurally different main entry (+189 tokens, +138 AST nodes)
without additional function or control-branch counts. Vite reports transform-hook calls,
not distinct modules. Earlier missing-Node probes never reached Vite; equivalence remains unproven.

The Axios/mock-adapter minimal fixture did not reproduce the issue: two auto/debug
and two strict builds each matched their own pair. Two full-candidate builds with
only `build.commonjsOptions.strictRequires=true` then matched exactly: 383 files,
15,406,919 bytes, 9,486 transform calls; selected source/config/dependencies unchanged.
The output differs from both historical auto builds. A separate
[strict-candidate browser assessment](2026-09-14-matdash-strict-build.md) now passes the
same 112 behavioral checks; cause isolation, cross-root/clean-install and full pixel
equivalence remain unproven. No original tested-build pins changed. Diagnostic packet:
`data/design-assets/matdash-cjs-experiment.W0DyAu/manifest.json`, SHA-256
`be06f245abd50919a4100a55c0e265df17fcfea4c4053fece63a9ff670c4a8ef`.
Earlier v2/v3 pointer/selection/code-mark and probe-edit failures remain retained with their red audit.

## Reproduce the bounded checks

Select an independently prepared private project, not the purchased original:

```sh
node docs/assessments/fixtures/matdash-hardened/reference.mjs --verify /absolute/private/project
node docs/assessments/fixtures/matdash-hardened/reference.mjs --serve /absolute/private/project
```

The second command prints its owned ephemeral origin. The tests require explicit
`APPKIT_MATDASH_HARDENED_ROOT` and `APPKIT_MATDASH_BASE_ROOT`; absence fails, not skips:

```sh
node --test docs/assessments/fixtures/matdash-hardened/reference.test.mjs
```

The browser probe additionally requires that origin in `APPKIT_MATDASH_HARDENED_ORIGIN`
and the approved existing 1.63.0 module in `APPKIT_PLAYWRIGHT_MODULE`:

```sh
node docs/assessments/fixtures/matdash-hardened/browser-reference.mjs
```

The probe uses fresh Chrome contexts, reduced motion, blocked service workers and
downloads, only allowlisted loopback GET/HEAD, response-byte pins and retained console/
outside/write failures. It checks an exact assertion-name set; missing work is red.
Dependencies are copied from the selected existing tree, not installed; source replay
does not prove a clean dependency reinstall or full transitive dependency identity.

## Limits and adoption guidance

The paid full application remains heavy. The final build is 15,389,163 bytes, versus
15,247,074 for the asset-bound base: +142,089 bytes. Lowlight's common entrypoint
bundles 37 grammars while this UI offers five choices; no package or lockfile was
changed. A production extraction needs explicit grammar,
route-payload and input-latency budgets, not wholesale adoption of this demo bundle.
The original large-chunk build warning is retained. Unknown pasted language attributes
can still reach Lowlight's auto-detection path; the selector is not a schema validator.
Observed uncompressed resource bodies were 3,356,507 bytes for P11 and 2,681,985 for
P12; these are loopback reference observations, not selected performance budgets.

Browser task checks do not establish full visual fidelity, screen-reader behavior,
genuine zoom, IME/clipboard/mobile-device parity, input latency or consumer adoption.
P11 synthetic charts are a regression specimen, not app-owned financial semantics.
Reusable acceptance lessons belong in [experience recipes](../design/experience-system.md);
paid React bodies remain a private source-native option, not a distributable Go port.

Configuration hygiene: [Fathom maintenance](../design/fathom-maintenance.md); not a new host-adoption claim here.
