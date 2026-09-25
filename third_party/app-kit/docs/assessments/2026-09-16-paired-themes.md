# Three paired first-party themes — 2026-09-16

Copper, Lagoon and Orchard are implemented as opt-in CUE-authored identities with
separate light/dark mappings, embedded Go CSS delivery, Gallery selection and
`appkit vendor --theme` support. Existing presets and defaults remain unchanged.
These are first-party role palettes, not converted commercial designs. No fonts,
network assets, dependencies or consumer changes are required by these stylesheets.

## Contract and scoped checks

`TestFirstPartyThemesRegenerateAndKeepSemanticContrast` compares emitted and embedded
CSS and rejects external asset declarations. In both schemes it checks ink,
muted/faint text against all four surfaces at 4.5:1; accent/status/focus/strong
boundaries at 3:1; and control labels plus soft-state text at 4.5:1. These are
selected role pairs, not blanket accessibility or every composed-state approval.
Handler tests cover GET/HEAD, exact embedded bytes, ETag/304 and non-read refusal.
Vendor and Gallery tests exercise the existing named-theme routes and selection.
`task gen:paired-themes` regenerates all three; `task check` includes their drift check.

Root review found omitted faint-text and strong-boundary coverage in the first
candidate. Added assertions exposed reported ratios of 3.96–4.43:1 and 1.97–2.53:1.
Only those roles in the new palettes changed; the thresholds and existing palettes
did not. Original CUE bodies/raw failing stdout survive only in the tool transcript,
not an immutable external snapshot. Earlier retained CSS/browser results do not
replace those missing inputs. This is an INC-43 recurrence, not replayable RED proof.

## Exact final browser replay

Private root: `/Volumes/m2ext/appkit-evidence/appkit-theme-browser.SELO4r/`.
Before replay, `final-inputs/` froze current CUE/CSS, contrast tests, workbench source,
browser harness and private Gallery binary; before/after comparisons matched.
The unchanged harness ran against that candidate in a fresh local Chrome profile,
not the owner's signed-in profile or Tailwind portal.

`output-frozen-replay/report.json` SHA-256:
`7b6dbd573b743ddf6323818aca3ac310b7ce92355b0c4bf163a90dce334ea289`.
It contains 61/61 passing checks, 12 captures, no blocked requests and no browser
errors. Each family/scheme runs at 390 and 1440 CSS pixels: selected stylesheet and
scheme, semantic values, horizontal overflow and visible programmatic focus are checked.
The probe calls `focus()`; it does not establish keyboard traversal or activation.
This is one component-preview surface; it does not cover every app guide or specimen.

| Final stylesheet | Bytes | SHA-256 |
|---|---:|---|
| Copper | 6,741 | `ac781c8f037d4880268de94dedbbbeb2934e12f5e9014512bbbfb4975d9018af` |
| Lagoon | 6,801 | `5455354795d6e9f356ee5fa599de297680705ee3c49d9b9c656fbac7001debfc` |
| Orchard | 6,802 | `e8fd61fe933f6afa7865cdf273b91410bb73fa6504f9c0300fab24f0914a5b87` |

The root review independently matched current CUE/CSS/test bytes, all 12 capture
hashes and the report hash. Inspected final Copper-light/Orchard-light desktop and
Lagoon-dark narrow captures: hierarchy, controls and visible text are contained and
readable in those views. The narrow capture ends before the chart specimens; it is
not evidence that every chart, color encoding or below-fold state was visually reviewed.

## Remaining acceptance

Whole-screen assistive-technology/forced-color/print/actual-zoom checks, every form
and analytical state, measured task/performance budgets, and a versioned consumer
upgrade remain separate. The readiness inventory retains partial gates rather than
turning this palette slice into a completed themes/style-guide starter area.
No full `task check`, released pin, installed-host or whole-app certification is claimed.
