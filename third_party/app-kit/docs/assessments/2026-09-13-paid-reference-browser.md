# Paid references: actual browser audit — 2026-09-13

## Result and boundary

**The selected full paid designs are now inspectable, but are not finished application recipes.**
P10 Catalyst settings and P11/P12 MatDash dashboard/editor have four captured views each:
390/1440 CSS-pixel widths, light/dark, 1000px viewport height, device scale 1.
This is **3/12 pilot cases with scoped browser captures**, not three completed pilot
baselines or converted targets. The required task/state/zoom/contrast/performance matrix
is incomplete; **zero certified conversions** remains the correct count.

The sources are the separately [font-bound Catalyst](2026-09-13-catalyst-font-reference.md)
and [repaired, asset-bound MatDash](2026-09-13-matdash-asset-reference.md) references.
Their archived/local originals are unchanged. No design was replaced with a smaller
generic page, and no client-owned rich editor was rewritten as a server template.
The original source and the separate [hardened Catalyst adaptation](2026-09-13-catalyst-hardened-reference.md) keep different identities; this original audit remains unchanged evidence.

The owner-approved isolated harness used installed Chrome 152.0.7977.84 and existing
Playwright 1.63.0 on macOS, with fresh contexts, reduced motion, no signed-in profile,
service workers or downloads. Only allowlisted loopback GET/HEAD routes are permitted;
WebSockets, outside requests and writes are refused. No dependencies were installed,
no portal session accessed, and no real save, account or business data used.

## Audit scope and priorities

These are known commercial reference designs, not an AI-generation-origin test. Their
coherent typography, space, surfaces and identity are assets to preserve. Pink/purple,
rounded cards or generous spacing are not by themselves defects. P12's nested editor
container and test Output panel need an explicit product role when adopted.

The five audit dimensions below are provisional, not compliance scores. No aggregate
health score is assigned where the necessary measurement has not been performed.

| Dimension | Scoped assessment | Evidence / limit |
|---|---|---|
| Accessibility | Partial | Catalyst country keyboard selection/focus works; ten MatDash toolbar controls have no accessible text names. Full tab order, screen reader and contrast not assessed. |
| Performance | Not scored | Actual local resource observations and chart readiness, not production compression, latency, task-time or update budgets. |
| Responsive design | Partial | Catalyst and Dashboard2 avoid page-width overflow; editor expands a 390px page to 565px. Zoom and other widths untested. |
| Theming | Positive, bounded | Both source themes render with their intended loaded fonts and distinct surfaces. Whole-token mapping and measured contrast remain open. |
| Anti-patterns | Contextual | Nested editor/test-output presentation and decorative demo analytics need product adaptation; no justification for stripping source identity. |

Confirmed adoption gaps: **four P1 issues, two P2 issues; no P0 or P3 claimed**.
These priorities apply before promoting a working app-kit recipe, not a claim that a
vendor demo promised an app backend. The analytics data contract below is an additional
adoption requirement, not an allegation that supplied demo figures are real finances.

## Findings and reusable acceptance requirements

### P1 · PRB-1: Country changes retain an invisible, invalid region

P10's `src/app/(app)/settings/address.tsx` controls country separately from the region
listbox. Space opens Country; ArrowDown then Enter selects Mexico, closes the listbox
and returns focus. The displayed Region becomes empty, but `FormData` retains Ontario.
An app could accept a value the user cannot see and which is not in the selected country's
options. A recipe must clear or deliberately replace dependent state, validate membership
at the authoritative boundary, and keep visible and serialized state consistent.
Suggested follow-up: `impeccable harden`; preserve the original as the failing reference.

### P1 · PRB-2: Reset does not reset the compound form

P10's native reset button (`settings/page.tsx:89`) resets name and bio, while controlled
country remains Mexico and the hidden region remains Ontario. The user sees a reset
form whose state is not the original. An adaptation must define one reset snapshot for
native and controlled inputs together; test serialized values as well as visible labels.
Suggested follow-up: `impeccable harden`; no app persistence claim is inferred.

### P2 · PRB-3: Advertised bio length is not enforced before save

`settings/page.tsx:38` advertises 240 characters; line 41 supplies no maximum. Filling
241 ASCII characters leaves length 241, `maxLength = -1`, and native validity true.
Supply consistent UI and authoritative validation, with an explicit length convention,
error association and correction path. This probe intentionally does not submit the form
or claim anything about a backend. Suggested follow-up: `impeccable harden`.

### P1 · PRB-4: Editor toolbar overflows the narrow page

P12 `src/views/forms/from-tiptap/TiptapEdit.tsx:62` uses a non-wrapping flex toolbar;
`Tiptap.css` supplies no wrapping or local overflow policy. At 390px the whole page is
565px wide in both themes. Its toolbar has 234px client width, 482px scroll width and
20px gaps; the last button reaches x=564.72. The Customizer is closed beyond x=390,
without a backdrop or running transition. A long toolbar must wrap into usable groups or have deliberate,
keyboard-accessible local navigation; hiding the document overflow is not a repair.
Keep editor selection/undo stable and verify every control remains reachable.
Suggested follow-up: `impeccable adapt`, then keyboard and narrow-screen regression checks.

### P1 · PRB-5: Ten toolbar controls have no accessible name

P12 `TiptapEdit.tsx:63` through its toolbar uses hidden SVG icons without button labels;
only H1/H2/H3 have text. The browser confirms ten empty names. Add names, explicit button
types and applicable pressed/disabled state, then test the accessibility tree and keyboard
commands. Existing icon aliases also need semantic review; do not infer action from a
picture alone. Suggested follow-up: `impeccable harden`. This is not a full WCAG audit.

### P2 · PRB-6: Duplicate editor extension registration warns at runtime

`TiptapEdit.tsx:20` includes StarterKit and line 24 adds CodeBlockLowlight; both register
`codeBlock`. Chrome emits the duplicate-extension warning on editor mounts. The probe
retains this as a failing check, not an ignored console entry. Choose one code-block
implementation and test language registration, document round-trip and undo behavior.
Suggested follow-up: `impeccable harden`; no observed data-loss claim is made.

### Analytics adoption requirement: visual composition is not metric semantics

P11's `RevenueForcastChart.tsx` supplies two Jan–Sep arrays around a −5…5 scale without
declared units; its Total/Profit/Earnings values are separate literals. Preserve the
complete composition as the visual reference, but author units, period, derivation,
uncertainty, accessible values and chart/table/export agreement before using it to make
decisions. Do not import decorative demo numbers into Fathom or another consumer's
analytics contract. Sparse/missing/error fixtures remain required by G3.

## Positive evidence to preserve

- Catalyst's whole settings hierarchy remains legible in both themes and selected widths;
  Inter actually loads, with 400/600 used in the narrow view and 400/500/600 in the wide view.
  Country options, selection, dismissal and focus-return work with the documented keys.
- MatDash retains its complete Dashboard2 composition, seven charts, imagery, Manrope,
  grouped cards and theme distinctions. All 18 forecast bars settle in every selected view.
  Page-width containment passes for the dashboard; dense labels still need closer review.
- The actual client editor supports typing, keyboard bold, undo and redo with agreement
  between editor content and supplied Output. A fresh mount returns to the demo's initial
  state. That is not persistence, HTML-safety, IME, clipboard or teardown certification.
- Observed asset bytes are checked against the selected prepared build/pins, and broken
  laid-out images, unexpected requests, errors, warnings and missing phases stay visible.

## Reproduce without installing or executing a new project

Use the existing reviewed guarded servers and explicitly prepared private roots. These
commands launch only a fresh browser test, never dependencies, servers or a signed-in profile:

```sh
APPKIT_CATALYST_BROWSER_ORIGIN=http://127.0.0.1:PORT APPKIT_CATALYST_FONT_REFERENCE_ROOT=/absolute/prepared/font-bound-project APPKIT_PLAYWRIGHT_MODULE=/absolute/existing/playwright/index.mjs node docs/assessments/fixtures/catalyst-native/browser-reference.mjs
APPKIT_MATDASH_BROWSER_ORIGIN=http://127.0.0.1:PORT APPKIT_PLAYWRIGHT_MODULE=/absolute/existing/playwright/index.mjs node docs/assessments/fixtures/matdash-native/browser-reference.mjs
```

The [Catalyst probe](fixtures/catalyst-native/browser-reference.mjs) verifies selected
source/font identities and records static-body agreement with its selected build.
Dynamic Next HTML/prefetch responses have status observations only, not historical byte
pins; a partial prefetch stream can remain open and must not be awaited as a static asset.
The [MatDash probe](fixtures/matdash-native/browser-reference.mjs) compares observed
responses with the pinned asset-build manifest. Neither is whole-app egress or process
attestation. Their exact assertion manifests prevent omitted tasks earning a green result.
Known source gaps intentionally make these adoption-audit runs exit 1.

## Retained final receipts

Private copies live under ignored `data/design-assets/browser-evidence/2026-09-13-paid.SNruYV/`.
`catalyst/` and `matdash/` each contain the original receipt and screenshots; the receipt's
absolute capture paths still name the temporary run, while the copied basenames retain
the recorded hashes. Back this directory up separately; Git does not preserve paid images.

| Run | Checks | Browser evidence | Receipt SHA-256 |
|---|---|---|---|
| Catalyst `QnyB1A` | 20/23 pass; 3 intended form failures, exit 1 | 4 views; 127 matching static bodies, 82 dynamic status observations; no errors/warnings/blocked requests | `3701866ff663032fb74377b59640c6c3cac34e539617a7ba049dd9979d8f8347` |
| MatDash `0Qxg93` | 51/55 pass; 4 intended assertions fail, exit 1 | 8 views, 4 additional viewport captures and editor task image; 89/89 pinned responses; no execution/body errors or blocked requests; 3 duplicate-extension warnings retained | `663118bc0fd986398de3775ee64230c75e66499ad66e74630a58e182df92dc4f` |

Thus 78 assertions yield 71 passes and seven failures covering six distinct adoption
issues; overflow fails once per theme. These are not coverage percentages for the catalog.
The final probe SHA-256 values are respectively
`98b69f49b3187d5e04836d0dec4fd624bab86214043ec13c0f6750e1cbad8604` and
`a6707afdf680ab4db1abb88feb7ebf98a676e82f010a7a43637a21ac1b711287`.
Both final runs use `/tmp/appkit-browser-review.xBrDCp/node_modules/playwright/index.mjs`,
Playwright 1.63.0, package JSON SHA-256
`8bc2ddebe8d9b051b9ecb87d62c6c63877e59d08617f32c3215681ca8bb70998`.
Entrypoint/package identities are not full transitive-toolchain attestation.

The P11 and P12 initial wide views observed respectively 3,356,507 and 2,539,896 encoded
resource-body bytes. Local static delivery was uncompressed (decoded bytes equal encoded);
these figures exclude the navigation document and are not production transfer, latency,
memory or compressed-budget measurements. No performance grade follows from them.

Early probe iterations exposed oracle faults: awaiting open Next prefetch bodies, losing
a static body while navigating, demanding an unused narrow-view font weight, requiring FX
allocation after synchronous theme updates and selecting both forecast/subscription bars.
They do not count as vendor defects. The final Catalyst run uses a fresh context per task
or captured view; its selected static reads remain bounded and checked. An intermediate
agent run used a different installed Playwright version; it is not the authoritative run.
Earlier temporary evidence is not promoted or substituted for these final identities.

Both probes pass Node syntax checks. Focused MCP document/Fathom tests pass. A fresh
MCP process serves `design-references` through tool/resource with exact local bytes:
9,526 bytes, SHA-256 `de4e0a98056454b41d50345d10902d7970cbe2ce86de097cbfe4eb615e92d241`.
The scoped intent checker passes 79 documents / 471 relative links / 33 ADRs / six goals /
nine futures, preserving 17 legacy over-budget ADRs. This is not a whole-repo runtime gate,
host reload, real consumer adoption or completion of G1–G6.

## Capture discipline and next gate

Network idle and reduced motion did not settle MatDash's JavaScript chart animation.
The final probe requires seven observable SVG.js chart roots, idle existing FX queues,
rendered geometry, exactly 18 nonzero forecast bars and at least 600ms continuous geometry
stability. Synchronous theme rerenders may allocate no FX; that alone is not a failure.
Forecast selection is scoped to its named card: a generic class also matched 14 subscription
bars. Earlier oracle timeouts are retained, not relabeled as source UI failures.

Keep full-page overflow captures distinct from the 390px viewport: an enlarged full-page
image can expose normally off-canvas Customizer content beyond that viewport. A screenshot
showing it is not proof the drawer stayed open. Capture close-state and viewport evidence.

Next: a separately identified hardened settings/editor adaptation with before/after checks,
then the remaining pilot families and complete acceptance matrix. Audit first, harden/adapt
the selected derivative, rerun the audit, and finish with `impeccable polish`. Do not replace
the frozen source, lower assertions to pass, increment certified-target counts, or promote
source screenshots into app-owned style/analytics/consumer delivery proof.
