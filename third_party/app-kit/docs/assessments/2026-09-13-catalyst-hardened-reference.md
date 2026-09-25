# Catalyst settings: hardened private derivative — 2026-09-13

## Result and boundary

A separately identified React derivative preserves the complete Catalyst settings design
while implementing dependent-field validation, compound reset and draft-preserving bio
limits. It is **local validation only**, not a persistence/authentication backend, Go
conversion or certified catalog target. Original purchased and font-bound sources stay
unchanged; the [original browser audit](2026-09-13-paid-reference-browser.md) remains red.

Identity: `catalyst-settings-hardened-v3`. The [pinned manifest](fixtures/catalyst-hardened/source-manifest.json)
covers 114 base files and 116 derivative files across selected source/config/public roots.
Exactly three files are replaced and two added; 111 records remain identical, including
the original fonts, styles, package manifest and lockfile. No dependencies were installed.
Generated `.next`, installed `node_modules`, VCS/cache paths and unselected root extras
are outside that source closure; do not call this a dependency-tree attestation.

## Behavior contract and intentional design changes

| Concern | Hardened behavior |
|---|---|
| Country → Region | Keep a region only if it belongs to the new country; otherwise clear it, show “Choose a region” and an associated error. Never choose an address for the user. |
| Validation | Compare actual submitted values with canonical country membership and visible state; do not trust serialized option lists. Native/bio errors receive focus before Region. |
| Reset | Restore one native/controlled snapshot, checkbox, labels, serialized values, errors, validity and status without replacing the focused Reset button. |
| Biography | Maximum 240 Unicode code points (`Array.from`), not UTF-16 units or graphemes; combining marks count separately. Retain the entire over-limit draft and offer correction. |
| Accessible errors | Preserve Headless UI error descriptions; forward `invalid` to the actual listbox trigger's `aria-invalid`. Clear it when corrected. |
| Error-state layout | Keep the postal input's surface at its own height when an adjacent Region error expands the grid row. |
| Local-only action | “Validate changes”; permanent “Nothing is saved” notice and explicit validation-only success. Example field descriptions promise no publication, billing or save. |
| Hydration failure | Server-rendered native fieldset stays disabled until the client coordinator mounts; no-JavaScript users see an explanation. A real app still needs its own fallback policy. |

The source page becomes a metadata wrapper around a client-owned `SettingsForm`.
`Address` receives controlled values; `settings-state.ts` supplies pure typed validation.
The shared private `listbox.tsx` forwards the invalid signal without changing its classes
or the default valid markup. This is a five-file private patch, not a new kit-core API.

The full source composition, control styling, typography, themes and spacing are retained.
Intentional differences are clearer demo copy, a bio counter/errors, dependent-value
placeholder/errors, validation status and the native fieldset wrapper. These are behavior
reference changes, not a palette flattening or replacement with a smaller generic form.
Client checks are UX feedback, never authoritative validation for an app backend.

## Evidence earned

- Eight pure policy tests pass: ASCII/astral/combining/ZWJ boundaries, non-string
  rejection, draft preservation and exact country/region membership. Node's existing
  typeless-package warning remains visible; no package metadata was changed to hide it.
- Eleven source-closure tests pass, including changed/missing/extra selected files,
  file/directory symlinks and missing selection. Canaries use disposable copies;
  both evidence roots are reverified afterward. Missing private inputs fail, not skip.
- The final derivative production build passes on Node 26.3.0 / Next 16.2.6 with the
  existing locked dependency selection. Source replay independently matches all 116
  records. The selected source/config/public closure remains unchanged by the build.
- The independent replay production build also passes. All 22 shared-path static
  assets match byte-for-byte; three manifest paths contain different generated build
  IDs. The exact static-tree comparison remains **false**, not normalized into a whole
  reproducibility claim. Server output and installed dependency closure are not compared.
- Final Chrome 152.0.7977.84 / Playwright 1.63.0 run: **48/48 assertions**, four default
  views (390/1440, light/dark) and two separate 390px error-state captures. All 149 observed
  static responses match: 80 selected build, 48 pinned public, 21 pinned font responses.
  Another 84 dynamic responses have status-only evidence; all 233 responses are 200.
  There are zero blocked/outside/write requests, page errors or console warnings/errors.
  These are tested requests/states, not whole-application egress or accessibility proof.
- Visual review preserves source hierarchy/typography in the four default views and
  checks both final narrow error views. Postal surface/input heights are now 44/44px.
  Native validation bubbles can appear in error captures; they were not suppressed.
  Explicit dirty-snapshot assertions precede Reset; a matching final snapshot alone
  cannot earn reset coverage for a field that was never changed.

The original country predicate expected an immediately selected valid region. This
derivative deliberately clears incompatible state and requires a user choice, so its
new oracle proves clear/error/focus/block/correct behavior. The original predicate and
failure are not relabeled as a pass. This is an explicit behavior-contract change.
The first hardened run also caught missing Region `aria-invalid`: five checks failed
despite correct error association. The fix adds the shared trigger forwarding; the
strict predicate is retained, with a separate correction-state assertion.
Visual review then caught a stretched postal-code surface beside the new Region error:
104px wrapper around a 44px input in both narrow themes. Two new geometry assertions
fail on v2; v3 applies `self-start` to that input wrapper. No overflow clipping or native
validation-popup suppression is used to make screenshots pass.

## Private replay and inspection

Final private packet: `data/design-assets/catalyst-hardened-v3.yitt7g/`.
It contains the exact font-bound base archive and five-file `settings.patch`, separate
from the purchase archive catalog. Earlier v1/v2 packets and failed browser evidence
remain separate; never overwrite them to manufacture a clean baseline. Back up ignored
assets separately: Git does not protect them. No licensed source body belongs in a
first-party distribution or unrelated upstream contribution.

| Identity | SHA-256 |
|---|---|
| Selected source manifest | `f8de11343773dbb8dbeb53bb713d684fee23fcd36604ddff921b07da57450ee2` |
| Private base archive | `c25a31c34605f16c71f36c242b1d845ad9355b4c12885aea833d579958238c77` |
| Private v3 patch | `c846a5017fd861ec48189a73a230b37696b81658e0222a376605df8272596082` |
| Source/build replay receipt | `fc6dff2624ca3341b76b431d7d150b50300caf08aa0df6b8bf6fdc388ad12830` |
| Final browser receipt | `d7b6709592dc11344a9c2c4a5e90e4f6f38dc73c46d17adfdd0cc7ecabfd8444` |
| Final browser probe | `53963548b23129ccc72b1584a389c47d6cf067a6f46af5b67839458a9bdbeb88` |

`browser-final/` contains the final receipt, exact probe and six hash-verified PNG copies;
receipt paths retain their original temporary capture locations. `candidate-build.json`,
`replay-build.json` and `replay-receipt.json` record commands and bounded comparisons.
The v1 invalid-state failure is retained in `catalyst-hardened.n0rDgr/browser-failed-v1/`;
the v2 geometry failure is retained in `catalyst-hardened-v2.hYGH5y/browser-layout-failure/`.

For replay, verify these archive/patch hashes, extract only this known base into a new
empty private directory, verify the base, then `git apply --check` and apply the exact
patch from inside that directory. Verify the derivative before building. Reuse an
explicitly selected dependency installation or follow the separately authorized
[source-native preparation](2026-09-13-catalyst-native-reference.md); no automatic install,
download or portal login is part of this fixture.

```sh
node docs/assessments/fixtures/catalyst-hardened/reference.mjs --verify-base /absolute/private/base
node docs/assessments/fixtures/catalyst-hardened/reference.mjs --verify /absolute/private/derivative
APPKIT_CATALYST_HARDENED_ROOT=/absolute/private/derivative APPKIT_CATALYST_FONT_REFERENCE_ROOT=/absolute/private/base node --test docs/assessments/fixtures/catalyst-hardened/reference.test.mjs docs/assessments/fixtures/catalyst-hardened/settings-state.test.mjs
NEXT_TELEMETRY_DISABLED=1 node docs/assessments/fixtures/catalyst-hardened/reference.mjs --serve /absolute/private/built-derivative
```

The launcher reports its owned ephemeral loopback origin; open `/settings`. It reuses
the unchanged same-origin GET/HEAD-only guard, not a general vendor-project executor.
Its source-native header identifies the React runtime, not untouched purchased source;
the launch record and manifest identify this hardened derivative. No existing Gallery
route or host/MCP registration is changed to execute it automatically.
Current v3 inspection: `http://127.0.0.1:55516/settings`, checked HTTP 200. This is a
temporary owned process, not a stable Gallery URL; relaunch explicitly when it stops.

The optional [browser probe](fixtures/catalyst-hardened/browser-reference.mjs) requires
explicit `APPKIT_CATALYST_HARDENED_ROOT`, `APPKIT_CATALYST_HARDENED_BROWSER_ORIGIN` and
the existing approved `APPKIT_PLAYWRIGHT_MODULE`. Fresh Chrome contexts permit only
allowlisted loopback GET/HEAD, block WebSockets and record attempted writes/outside
requests as failures. Observed static responses must match pinned public assets or
the selected build. Dynamic Next HTML/RSC responses receive status checks only.

## Guidance verification and root intent-doctor

The reference guide and common-form acceptance guidance are updated together; scope and
changelog keep source-native repair separate from conversion/consumer certification.
Fresh MCP tool/resource reads match `design-references` exactly (9,564 bytes, SHA-256
`70469bff10b55799b7a83d4434113e0094cbd16b4463e1540a8630606e87e182`); guidance routes there.
Focused decision/guidance/intent/Fathom MCP tests pass. No host was reconfigured.

Scoped root check: **21 PASS, 2 SKIP**. This is structural wiring, not semantic intent
truth or a claim that either host loaded instructions. The existing checker covers 80
documents, 479 relative links, 33 ADRs, six goals and nine futures with zero failures;
the additional manual/path/hash checks below complete this root-only doctor review.
Seventeen inherited long ADRs remain explicitly identified history, not new budget passes.

| # | Result | Check / evidence |
|---|---|---|
| 1 | PASS | Root router exists. |
| 2 | PASS | Router remains within 80 lines. |
| 3 | PASS | All root living docs remain within their budgets. |
| 4 | PASS | Required root docs exist and route from README. |
| 5 | PASS | 33 local ADRs match the index; no missing or duplicate local record. |
| 6 | PASS | Every ADR has the documented Status and Revisit when shape; legacy length debt retained. |
| 7 | PASS | Goal/future/ADR trigger registry is nonempty. |
| 8 | PASS | Six goals have exit tests; nine futures have promotion/drop/confidence/horizon fields. |
| 9 | PASS | Scope contains the compatibility surface and entries. |
| 10 | PASS | Ten sensitive surfaces have nonempty evidence requirements. |
| 11 | PASS | Unreleased starts with Release impact. |
| 12 | PASS | Unreleased remains within 60 lines. |
| 13 | PASS | Autonomy has may/ask/never tiers and the branch model. |
| 14 | PASS | All 17 incident records have an Outcome. |
| 15 | PASS | Root CLAUDE.md and AGENTS.md are byte-identical. |
| 16 | PASS | Eight Claude personal intent skills resolve to agentic-kit; no project override found. |
| 17 | PASS | The same eight Codex personal skills resolve to identical bodies; no project override found. |
| 18 | PASS | Installed Claude 2.1.270 and Codex 0.154.0 match the router. |
| 19 | PASS | Scoped relative links resolve. |
| 20 | PASS | Router concrete paths resolve; generated lockfile clarified as `<consumer>/kit.lock`. |
| 21 | PASS | One owned router; charter inherits root and all 11 evolution cells use the fixed vocabulary. Two older cells normalized without scope changes. |
| 22 | SKIP | No owned nested intent router, so no nested stanzas to compare. |
| 23 | SKIP | No owned component intent documents; root inheritance is explicit. |

Next useful action: address MatDash's demonstrated editor gaps with the same preserved
source → separate patch → failing-before/passing-after → visual-review discipline.

## Remaining gates

No pilot baseline or converted-target count increases from this source-native repair.
Full screen-reader/contrast/zoom, broader viewport and task/performance matrices,
real persistence/authorization/recovery, consumer integration and Go-target fidelity
remain open. MatDash's three editor gaps remain separate work. G1–G6 are not complete.
