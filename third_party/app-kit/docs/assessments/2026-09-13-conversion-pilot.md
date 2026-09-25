# Conversion pilot: frozen source cases — 2026-09-13

Status: **source selection frozen; syntax/compiled-DOM pilot run recorded below**.
This is the 12-case input denominator for [ADR-0023](../adr/0023-profile-specific-conversion-and-client-compute.md)
and the [conversion pipeline](../design/conversion-pipeline.md), not twelve successful conversions.
At source freeze, browser access was blocked; that inventory did not run vendor projects.
The later [paid-reference browser audit](2026-09-13-paid-reference-browser.md) captures
P10/P11/P12 with scoped tasks and real failures. Full visual/behavior/accessibility and
performance gates remain **unverified**; no converted target is certified.

## Inventory and source identity

Validated private catalog revision:
`7c7c0ab69b86349f1bb8d1df865f7cf8eb4910542c6fe8340252c6593facaab6`.
ZIP roots are under ignored `data/design-assets/archives/<archive-sha256>.zip`.
Archive IDs include their complete SHA-256, not a mutable product name.

| Alias | Exact archive ID | Original filename |
|---|---|---|
| M | `marketing@8863da80a161b3dd0f3ee2991657edbc29855ff8dc23915dda1f520fd6859675` | `marketing-v4.zip` |
| A | `application-ui@b26cbf6e9529c2219452de32b1a9c49ca12d7bc63bca5e48cd286088521ba0e8` | `application-ui-v4.zip` |
| C | `catalyst@94acb41796a8a97fdef44bf5b2d18f42919e43300cbca90077775f6b36dfa529` | `catalyst-ui-kit.zip` |

`MD` below means an existing local file relative to the app-kit repository, **not**
an imported archive or an immutable snapshot. Its frozen hash detects subsequent changes;
it does not establish a backup or reproducible dependency closure.

## Exact twelve-case manifest

Paths for M/A/C are archive-relative regular members. MD paths are repository-relative.
The SHA column always identifies the exact primary file bytes, including existing line endings.
Supporting files below belong to their case; they do not enlarge the twelve-case denominator.

| Case ID | Family | Origin | Exact root member/path | Member/file SHA-256 |
|---|---|---|---|---|
| P01-hero | Static hero | M | `html/sections/heroes/09-split-with-image.html` | `356c19f1095d82529cb54014e3b94a67b7531489bb26b96b5fba8686d94beba9` |
| P02-pricing | Pricing | M | `html/sections/pricing/05-single-price-with-details.html` | `f545f273689b9a4312e5d0eeb66c15dc21d7d318e102e37c39b40f494e017e0f` |
| P03-faq | Content/FAQ accordion | M | `html/sections/faq-sections/02-centered-accordion.html` | `9114b0b981055638eb28fede17eff7bc49c91f40fc32ce758514bfdb7d19f937` |
| P04-table | Responsive table/list | A | `html/lists/tables/07-with-stacked-columns-on-mobile.html` | `5d00e2755dcf060cceafa4c2dba4f15452719e54f5cd40646ab3155c89dc8288` |
| P05-form | Form error/validation presentation | A | `html/forms/input-groups/03-input-with-validation-error.html` | `c35ba209333b7a4897c93a8c696b7fe306f4797625bc42906090e6edf864cc19` |
| P06-tabs | Page-navigation tabs | A | `html/navigation/tabs/01-tabs-with-underline.html` | `01345180cee4744b57e50a3ceca9461566fdfd7ab5d8f629113654bac052ca31` |
| P07-menu | Dropdown menu | A | `html/elements/dropdowns/01-simple.html` | `10ed82d0c337ad246f1ab8687d73cc0a7dc5d834bb7b3ec46a01bc7c5d5f144a` |
| P08-nested-dialog-menu | Dialog with nested menu | A | `html/overlays/drawers/07-user-profile-example.html` | `1fbde54a0be874618cc295bf3b837fee4d89f38a1543dd9a6e2374bbd1d13415` |
| P09-mobile-nav | Mobile navigation | A | `html/navigation/navbars/04-simple-with-menu-button-on-left.html` | `79cdbf1cd163a13cb77cc96c4cf8046a088ad8f18757658408372d437400b9d7` |
| P10-catalyst-settings | Catalyst settings page | C | `catalyst-ui-kit/demo/typescript/src/app/(app)/settings/page.tsx` | `1ca7f4e14738e917390815dc38d48047588eb324090d337f5c718b9d2ec6fa21` |
| P11-matdash-analytics | MatDash analytical composition | MD | `data/matdash/packages/main/src/views/dashboard/Dashboard2.tsx` | `36c37212b4e5571367618d4e0d8fc9d1dfa35c75266228170f8eac071112b277` |
| P12-client-editor | Client-owned rich editor | MD | `data/matdash/packages/main/src/views/forms/from-tiptap/TiptapEdit.tsx` | `569ffc17dd27859dfe0df9a3fbf85e673e6552a547d142d840f37a30447fc9ac` |

## Target assumptions and behavior scope

These are **initial applicability assumptions**, not earned catalog profiles or reviewed
exclusions. S means build-time HTML/static-web; G means Go/templ ordinary HTTP/go-web;
E means preserve a deliberately pinned Elements browser runtime; C means retain the
source-native client implementation. Datastar is evaluated only for a named projection
or lifecycle need; none of these controls intrinsically requires a server round-trip.
Static delivery with E is not a zero-JavaScript implementation.

| Case | Source traits grounded in the selected file | Candidate targets and required proof | Original fallback |
|---|---|---|---|
| P01 | Split text/image composition, external logo/photo assets, CTA links, explicit dark variants; no script tag | S/G; preserve image sizing, typography, spacing and dark states; replace placeholder destinations with declared fixture routes | Exact HTML plus its CSS/assets |
| P02 | Single-price feature/detail composition, CTA link and dark variants; no runtime script | S/G; keep price/content hierarchy and long-label wrapping; no subscription or billing backend is supplied | Exact HTML plus CSS |
| P03 | Six command-triggered `el-disclosure` FAQ panels and dark variants | S+E/G+E; keyboard/pointer disclosure, initial/open states and update lifecycle; any native-details substitution needs a separately reviewed behavior mapping | Exact HTML with Elements |
| P04 | Table with columns stacked on mobile, action button/links and dark variants; no script | S/G for the displayed table; selected sort/filter/page/detail/export tasks need explicit fixture logic, not an invented vendor data service | Exact HTML table plus CSS |
| P05 | Email input, label, `aria-invalid=true`, linked error description and dark variants | G for a reviewed validation workflow; S can reproduce the error specimen only. Valid/pending/server-error/retry behavior and authoritative validation must be supplied explicitly | Exact error-state HTML; not a validation engine |
| P06 | Desktop navigation links, `aria-current=page`, responsive native select and dark variants; no script or tabpanel | S/G page navigation with real URLs and an explicit narrow-select action. This is **not** an ARIA tablist/tabpanel state machine; no panel keyboard contract is inferred | Exact navigation-tab HTML; placeholder links remain incomplete |
| P07 | `el-dropdown`/`el-menu`, menu actions, form submit item and dark variants | S+E/G+E; focus entry/return, keyboard movement, Escape, outside click and owned action handling | Exact HTML with Elements; actions still need an app destination |
| P08 | An actual `el-dropdown`/`el-menu` occurs **inside** the native `dialog` wrapped by `el-dialog`; drawer panel, imagery and dark variants | S+E/G+E; nested Escape ordering, menu/dialog focus return, dismissal, clipping/stacking and mount/remove/remount | Exact original nested drawer/menu, not a fabricated two-source composition |
| P09 | Mobile `el-disclosure`, command trigger, account `el-dropdown`/`el-menu`, images and dark variants | S+E/G+E; narrow/wide visibility, keyboard focus, current route and disclosure/menu lifecycle; pin assets | Exact HTML with Elements and source imagery |
| P10 | Settings form composed from Catalyst Input/Select/Checkbox/Button/Heading/Text components plus local Address; reset/submit controls | G is a reviewed composition-port candidate; C is the native baseline. App save/validation/authorization are not supplied by importing the page; static output alone is not the settings workflow | Complete archived Next/React demo with its own dependencies |
| P11 | Dashboard2 composes forecast/financial summaries, activities, geography, schedules and a product table; imported widgets use React/ApexCharts/Flowbite and images | G composition with reviewed chart/data adapters or owned client widgets; C native baseline. Freeze numerical fixtures/units/windows before claiming analytic equivalence | Existing MatDash main React/Vite project, not a dependency-free page |
| P12 | Tiptap `useEditor`, `EditorContent`, local React document state updated with `getHTML`, link/image/code/underline extensions and local CSS | C subtree/full client app; G may host the subtree behind a typed mount/update/dispose boundary. A server-template editor rewrite is not the proposed target. Test typing, IME, selection, undo, clipboard, HTML safety and teardown | Exact existing Tiptap React implementation with its dependency closure |

All nine HTML selections contain explicit `dark:` variants. Preserving their source
bytes is not proof that generated CSS, contrast, focus or runtime behavior is correct.
P05 and P06 deliberately expose limits in source semantics; they may not be counted as
complete validation or ARIA-panel coverage. Expanding those cases requires a reviewed
task contract and baseline, not silently adding semantics during a repair.

## Runtime and supporting-input pins

UI-block provenance is the named **Tailwind 4** official HTML bundle. The bundles have
no package manifests, compiled CSS or vendored interaction runtime. An exact Tailwind
patch version, font set and build configuration are not established by the filename.
P03/P07/P08/P09 contain commented runtime setup instructions with the URL
`https://cdn.jsdelivr.net/npm/@tailwindplus/elements@1`; this is a mutable major selector,
**not** a reproducible version pin or an executing loader. It was recorded, not fetched or executed.
Remote Tailwind Plus logo/Unsplash image URLs also remain unpinned; availability,
asset rights, intrinsic sizing and offline reproduction need later verification.

Catalyst's selected TypeScript demo declares `catalyst-demo` version `0.1.0`, Next
`16.2.6`, React/React DOM `19.2.6`, Headless UI `^2.2.10`, motion `^12.38.0` and
Tailwind `^4.3.3`. These are source declarations, not successful installation or runtime
results. The entire project is preserved by archive C; relevant member pins are:

| Supporting input (archive C) | SHA-256 |
|---|---|
| `catalyst-ui-kit/demo/typescript/src/app/(app)/settings/address.tsx` | `4c0bc2c0b9827c91d6cebe9dc3d4c3bd8c63d62154bafdded843a636ecb28b0f` |
| `catalyst-ui-kit/demo/typescript/package.json` | `04f5b915bdd1b1ef041291a7a30b4a2af1e0edb43156ab876b2f27ae72c56c53` |
| `catalyst-ui-kit/demo/typescript/package-lock.json` | `3a9469d59cb2c862c35735c5153f43b7ddf27c9f30c867cf64838df1f9b66b3c` |

Both MatDash roots belong to `data/matdash/packages/main/package.json`, which declares
`matdash-react-vite-ts` version **`0.0.0`**. This is the local package version, not a
verified MatDash vendor-release number. Declared ranges/pins include React `^19.0.0`,
Vite `^5.4.11`, Tailwind **`^3.4.16`**, Flowbite React `^0.10.2`, ApexCharts `3.48.0`,
React ApexCharts `^1.4.1`, and Tiptap React/StarterKit/extensions generally `^2.10.3`.
The editor imports `lowlight/lib/core`; direct/resolved availability is unverified.
No npm/yarn/pnpm/bun lockfile was found under this selected package in the literal
filesystem inventory. Do not reinterpret this tree as Tailwind 4 or a resolved install.

| Supporting local input | SHA-256 |
|---|---|
| `data/matdash/packages/main/package.json` | `47be5e2fca24a169207168aef342c89f3a2997f0984d1538260dc123140b97eb` |
| `data/matdash/packages/main/src/components/dashboards/Dashboard2/RevenueForcastChart.tsx` | `e557c33dd19fdfad4d794bd327c0f2b84ab7abf4777a2697dcfeb324056aacd9` |
| `data/matdash/packages/main/src/components/dashboards/Dashboard1/RevenueByProduct.tsx` | `abbd47dc8fdf1c3d1e2ecbc88a2a6af0c62de349f6cce789a6a1baa90483e996` |
| `data/matdash/packages/main/src/views/forms/from-tiptap/Tiptap.css` | `984fa91622a6727da3b00a29e3509d5418253263600229904a4a598b8ba4d1d8` |

The supporting-input list is not a complete runnable project closure. Freeze remaining
imports, CSS, fonts, images, toolchain and runtime hashes before capturing a baseline.
Originals remain available independently of any conversion result; neither this
manifest nor a later successful port changes vendor provenance or private-use restrictions.

## Evidence collected and next gate

Read-only `appkit data assets-list`/`assets-files` resolved catalog IDs and metadata;
`assets-get` verified the selected archive/member reads. Bounded structural summaries
checked source tags/attributes/imports without publishing source bodies. `shasum -a 256`
checked the selected existing MatDash files; manifests were read as data. No vendor
code was extracted to disk, installed, executed, uploaded or modified.

The selection stage established **12/12 primary source pointers with byte identities**,
not source authenticity, dependency reproducibility or target compatibility. At freeze,
there were zero conversions; the subsequent syntax-only experiment is recorded below.
At that stage there were **0/12 captured source-native visual baselines** and no passing
behavior/fidelity/performance gate. Later observations below must retain their own scope.
The later [P10 native rehearsal](2026-09-13-catalyst-native-reference.md) built Catalyst
twice and passed read-only HTTP checks; it does not change that visual-proof denominator.
A separate [font-bound reference](2026-09-13-catalyst-font-reference.md) pins the complete
Inter asset closure; it changes neither the frozen source identity nor that denominator.
The later [P11/P12 MatDash reconstruction](2026-09-13-matdash-native-reference.md) preserves
these originals and records a separate repaired build/static delivery, still not visual proof.
Its [asset-bound variant](2026-09-13-matdash-asset-reference.md) adds local font/map/icon
inputs and build replay; the frozen source inputs remain unchanged.
The [browser follow-up](2026-09-13-paid-reference-browser.md) now has scoped source-native
captures for **3/12 cases**, but **0/12 complete pilot baselines** and **zero certified targets**.
Original form/editor gaps are explicit adoption requirements, not silently repaired inputs.

Before implementing behavior adapters or claiming target compatibility, review each
target's applicability and freeze its fixture,
source-native baseline, runtime/assets, required gates and budgets from
[runtime selection](../design/runtime-selection.md). Retain missing/error/long-label,
390/1440-width, dark/light, keyboard/zoom and recovery cases in the denominator.
For P05/P06, settle the explicit validation/navigation task without claiming missing
vendor behavior. A change to these roots, their hashes or applicability must be recorded
as a new manifest revision; it cannot quietly replace a failing case.

## First syntax and compiled-DOM run

Executed `TestFrozenConversionPilotLive` on 2026-09-13 with the explicit private checkout
environment variable in [the translation guide](../design/html-translation.md).
All twelve case-to-source bindings and original hashes matched. Five generated templ
components compiled and rendered with the pinned toolchain, both before and after
formatting. A namespace-aware HTML5 tree comparison retained elements, text, attributes
and comments. These are constant-only renderings, not running vendor React/JS projects.

| Frozen cases | Observed outcome | Remaining boundary |
|---|---|---|
| P01, P02, P04, P05, P06 | 5 syntax translations; 5 compile/render DOM comparisons passed | CSS/assets, visual fidelity, real routes/data and task behavior unverified |
| P03, P07, P08, P09 | 4 explicit unsupported results; no partial output | Elements/custom runtime and interaction adapters required |
| P10 | Non-HTML source explicitly refused | Catalyst source-native profile or reviewed composition adapter |
| P11, P12 | Local TSX hashes matched; not sent through the archive/HTML translator | MatDash/client-native ownership; no fabricated archive provenance |

The complete run took 8.79 seconds on the session's macOS/arm64 host with cached
dependencies (Go 1.26.3, templ v0.3.1020, x/net v0.54.0). This is test elapsed time,
**not** an app performance benchmark. Generated private test files were temporary;
original archives and local MatDash files were not modified. All translation envelopes
retain `verification: not-verified`; no catalog target was certified or published.
The checks have canaries for changed case bindings, stale/rehashed artifacts, dropped
dark classes, executable templ constructs, and unsupported nodes with partial output.
No oracle threshold, fixture identity or unsupported target was weakened to pass.

The built CLI separately demonstrated actual process exits 0 (hero translation),
1 (dropdown unsupported with JSON and no artifact), and 2 (missing required arguments).
The observed rule SHA after independent-review fixes was
`6f4e1407e64c3d00d6277ca8e45436e24078ae013163268421807af829422f6b`.
`task check`, focused race tests for designconvert/CLI/MCP, offline module-tidy diff,
and intent link/budget/stanza checks passed. These checks do not close browser proof.

## Elements source-native follow-up — 2026-09-14

Subsequent [P03 typed FAQ proof](2026-09-14-faq-typed-adapter.md) adds independently
selected faithful Elements/native JS-free variants with app-owned content and scoped
browser/standalone evidence. It does not change the frozen input or the historical
default-translator refusals below; broader pilot/family certification remains open.

P03/P07/P08/P09 now have a separate [pinned runtime reference](2026-09-14-elements-source-reference.md):
all four original source hashes match, a private 18-file bundle plus manifest replays
byte-for-byte, and 404 scoped Chrome checks pass across two widths and light/dark.
The runtime-withheld control fails the same disclosure assertion. P08/P09 imagery
remains unpinned/blocked; original small controls and missing real actions remain
visible findings. These are source-native interactions, not four conversions or
complete pilot baselines. The default translator still refuses them, and historical
syntax counts/receipts are unchanged. The assessment freezes the next opt-in rule
boundary without changing original selection or widening the old rule.

## Elements converted-output follow-up — 2026-09-14

The separate [Elements rule and paired assessment](2026-09-14-elements-conversion.md)
now convert those same four originals: exact compiler/DOM equality before/after templ
formatting, 48-file deterministic preparation replay, 842 scoped browser assertions
and 34 byte-identical source/compiled screenshot pairs. Independent lane-binding
checks reject rehashed aliases to the source. The read-only CLI and source-free
experimental catalog record expose the tool, not certified web components. Historical
default-rule refusals above, saved attempts, original source gaps and zero certified
targets remain unchanged; this is not broad custom-element or Datastar proof.

## P11 bounded composition inventory — 2026-09-14

A read-only scout rechecked the frozen Dashboard2 SHA above and inspected six of its
13 immediate panels: GradientBox, Subscriptions, UsersBox, RevenueForcastChart,
AnnualProfit and WeeklySchedule. The root uses a 12-column responsive grid; these
children combine a visual welcome card, headline metrics, bar/area/range-bar charts
and summary rows. Five sampled panels depend on React ApexCharts; their series and
labels are literal fixtures, including `ChartData: any`, not typed domain contracts.
No app handlers, inputs, fetch or state were observed in those six files. This is
bounded source inspection, not a claim about every panel or ApexCharts internals.

The sampled composition supplies valuable visual structure but no complete governed
period/filter → explanation → detail → export journey. Source-native hover behavior
and the [strict-build browser receipt](2026-09-14-matdash-strict-build.md) remain useful
reference evidence, not proof of that missing task. Before a Go composition port,
freeze a coherent synthetic metric/period/unit contract and explicit interaction
mapping; reuse existing kit analytics primitives only where they fit. Do not infer
business meaning from demo labels, flatten chart behavior into static tags, or call
an authored task extension a faithful source conversion. No paid source was modified,
copied into kit code or newly executed by this scout; P11 remains unconverted.

The subsequent [task contract](../design/matdash-analytics-adapter.md) and first-party
CUE fixture define explicit receipts/refunds, periods, channel selection and comparison
refusals. The original forecast's signed values have no declared monetary derivation;
the product table's month selector is unbound while its client tabs reorder literals.
Those are adoption requirements, not evidence that an authored workflow is source-faithful.
