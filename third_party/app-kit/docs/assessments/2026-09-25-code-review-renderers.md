# Code excerpt and unified diff (FATHOM-KIT-04/05)

## Packet

Originating consumer: Fathom's observatory console (vision 35,
`docs/design/observatory-console.md` §5 in the Fathom repository). Its refactor page
needs "a code excerpt per partial item with its unexecuted changed lines highlighted";
its review surfaces need a line-level diff beside the field-level `hitl.DiffView`.
Capability search found no existing code-excerpt or text-diff entry, and the licensed
component corpus returned no code-viewer or diff composition, so both are original.

Scope: two experimental L3b primitives, `content.code-excerpt` (`appkit/ui/code`) and
`work.unified-diff` (`hitl.RenderUnifiedDiff` over `hitl.DiffText`), plus the private
`appkit/ui/internal/linetext` helper. No syntax highlighting, JavaScript, word-level
diff, side-by-side view or patch application. I4/I5/I7/I8/I12 apply; no token, runtime,
dependency, consumer or installed-host change. Fathom integration is not part of this
packet and has not happened.

## Critical states

Excerpt: normal, changed, not executed, highlighted (each with marker, spoken label and
counted legend), targeted line, stated gap between non-consecutive lines, revealed
hidden/bidi characters, long lines scrolling without wrapping, empty excerpt, refused
input (blank label, non-increasing numbers, unknown state, unsafe link, over
`MaxLines`). Diff: added, removed, context, collapsed and expanded unchanged runs,
omitted ranges between precomputed hunks, no final newline, no changes, edit-budget
fallback, revealed characters, refused input (both inputs, bad counts/numbers,
overlap, oversized text or hunks).

## Results at `94e6815`

Unit and structure (`go test -race`): golden HTML for both renderers; state labels,
anchors, permalinks, legend counts, gaps, escaping, bidi/zero-width/control/invalid-byte
reveal, atomic refusals, determinism, no inline style, no literal colors or `dark:`
variants, no `ink-faint` text, list structure without tables. `DiffText` rebuilds both
texts and keeps exactly LCS-many lines on 400 seeded random cases; hunk headers match
`git diff --no-index` at `-U3`, `-U0` and `-U6`, including new and deleted files.
Default-palette contrast is measured in both schemes: ink and ink-muted on every row
treatment ≥ 4.5:1, state rules and the focus frame ≥ 3:1. The light `caution` role
failed 3:1 as a rule (2.35–2.45:1), so not-executed uses the coverage convention
`danger` instead; that failure is why the test exists.

Browser (Gallery binary SHA-256 `9b32edd3a39ca89bb4df4aab955e775dee42d55508649e0bacea362b586e68bc`,
built from `94e6815` with Go 1.26.3 darwin/arm64 and committed CSS SHA-256
`4f849363e755112a083ff2ebfdf1488e9914ef273757e7495d15864f60b09f11`; Chrome 153 via the
Playwright MCP in an isolated context; harness
[`fixtures/code-review/browser-matrix.js`](fixtures/code-review/browser-matrix.js),
SHA-256 `9fe1b4087db3d8bc0f1588dd1dd6e42b2c4e14ed2220e75009bf77cf942fdca6`;
synthetic page data only):

| Task | Viewport / theme / input | Expected | Observed |
|---|---|---|---|
| Tab from the jump link | 1440×1000 light, keyboard | caption link → excerpt region → line 40 link, each with a visible 2px focus ring | as expected, `:focus-visible` on all three |
| Enter on a line number | same | URL becomes `#review-L40`; that row shows the focus-colored frame 64px from the top | as expected |
| Click "jump to line 49" | same, pointer | not-executed line 49 targeted and framed | as expected; frame is an overlay above the pinned gutter |
| ArrowRight ×10 in the long-line region | same, keyboard | region scrolls; gutter stays pinned; rows never wrap | scrollLeft 0 → 400 → 0, gutter offset 2px → 0px, every row 20px tall |
| Tab to the collapsed run, Enter, Space | 1440×1000 dark, keyboard | 44px summary with focus ring; opens with verb "Hide", closes with "Show" | as expected; two hidden lines shown when open |
| Load the page | 390×844 light and dark | no page overflow; code regions scroll inside themselves; 44px caption link and summary; Agentic nav current | as expected in both themes |
| Accessibility tree | 1440 light | line state and diff labels exposed as text | "Line 48" link + "Not executed:" + code; "Added, proposed line 6:"; hunk "Changed region: current 3–11, proposed 3–12"; legend list with counts |

No page errors or console errors were recorded in the isolated context. Full-page
captures at 1440 light/dark and 390 light/dark, and the targeted frame at 390, were
inspected; they are working files, not retained evidence.

## Findings and repairs during the packet

- A `:target` outline on the row was painted beneath the sticky number gutter, so the
  frame vanished on the left. Replaced by a `::after` overlay (z-10).
- A new Gallery header link pushed `/analytics/signed` past its 20KiB full-document
  budget (20,818 bytes). The budget was kept; the specimen moved to the Agentic
  subpage `/agentic/code`, linked from `/agentic`, with Agentic marked current.
- At 390px the second collapsed-run summary text extends past the region edge and is
  read by scrolling the region; it is not truncated.

## Review repairs at `5bf90cd`

An independent review of `94e6815`/`42d696d` raised seven findings. Six were
reproduced and repaired test-first; one proposed remedy was not adopted.

- **Hidden characters (high).** `linetext.Hidden` used a hand-picked list: 153 Unicode
  format (Cf) code points, including the tag block used for ASCII smuggling, U+2028,
  U+2029 and the default-ignorable fillers passed unrevealed, so a `// comment` could
  hide a JavaScript statement after U+2028 and an added line with tag characters drew
  identically to the removed one. `Hidden` now reveals by class (Cc, Cf, Zl, Zp,
  `Other_Default_Ignorable_Code_Point`, variation selectors; Tab excepted), and a test
  enumerates each class. Variation selectors include an emoji's U+FE0F, which a review
  surface now shows as a code point. Classification follows Go's Unicode 15.0 tables.
- **Spoofable names (medium).** Caption, link text, diff label and side names went
  through plain escaping, so `admin` + U+202E + `og.txt` rendered as `admintxt.go`. They now
  get the same reveal inside `<bdi>`; accessible names carry the notation as text.
- **Precomputed hunks (medium).** The gap between hunks said "Unchanged current X–Y not
  included" from old-side numbers only. The renderer never received those lines, and a
  supplied subset of hunks can skip a real change, so the gap is now stated per side
  ("Lines 4–14 not included", "Not included: current 2, proposed 2–9"). A "no newline"
  mark is refused except on the last supplied line of its side. The suggested refusal
  of hunk sets whose old-to-new offset drifts was **not adopted**: subsets are a
  documented use, and an omitted hunk that adds or removes lines legitimately shifts the
  offset (the seeded property test found odd-indexed `DiffText` subsets that rule would
  refuse). Every `DiffText` hunk set and subset on the 400 seeded cases, and every subset
  of the TOML fixture's hunks, is accepted.
- **Budget note (low).** `UnifiedDiff.Replaced` carries `LineDiff.Replaced` with
  precomputed hunks, so a non-minimal diff keeps its note; setting it with texts is
  refused.
- **Target size (medium).** Excerpt rows were 20px, so every line-number permalink was a
  48×20px target at a 20px pitch, below WCAG 2.2 2.5.8. The T5 row above measured only
  the caption link and summary. Rows are now `leading-6`: each permalink is 48×24px at a
  24px pitch. That meets 2.5.8 but not the kit's 44px floor for standalone controls,
  which dense code rows cannot reach; the catalog records it as a divergence. Diff rows
  stay 20px: they hold no targets.
- **Catalog (low).** `consumers` is `[]` for both entries (the Gallery had shown Fathom
  as an observed consumer); the originating request stays in gaps. Task phrases now
  include "unexecuted changed lines highlighted", "source code viewer", "line-level
  diff", "diff viewer" and "unified text diff renderer", each with a selection case;
  `appkit adoption search` returns the matching entry for all five, and
  "syntax highlighting" still returns nothing.

Browser at `5bf90cd`: Gallery binary SHA-256
`93555bc1abadc0e081eb9a784ffa526fc04c1f3e4e476d7ff677c65f67dec1c7`
(`go build -trimpath ./apps/gallery/cmd/gallery`, Go 1.26.3 darwin/arm64; its
`vcs.modified=true` reflects only this uncommitted assessment and harness), served CSS
SHA-256 `dd980f8dca11dccac0bddfaf653ab8cb3cfd87972b1b3eda76ca238ad5ae3874` (the rebuild
adds only `.leading-6`), harness SHA-256
`a382ed015da233fde25cb74d7bec5d08929896044e76af5492f97536ff9d5196`, Chrome 153 in an
isolated context:

| Task | Viewport / theme / input | Observed |
|---|---|---|
| T1–T2 permalinks | 1440 light, keyboard and pointer | unchanged from `94e6815`: focus rings, `#review-L40`, framed targets 64px from the top |
| T3 long lines | 1440 light, keyboard | scrollLeft 0 → 400 → 0, gutter 2px → 0px; excerpt rows 24px, diff rows 20px |
| T4 disclosure | 1440 dark, keyboard | 44px summary; Enter opens ("Hide", 2 lines), Space closes |
| T5 narrow | 390 light and dark | no page overflow; caption link and summary 44px; all 24 permalinks 48×24px, minimum pitch 24px |
| T6 accessibility text | 1440 light | as before: "Line 48" link + "Not executed:"; "Added, proposed line 6:"; hunk "Changed region: current 3–11, proposed 3–12" |
| T7 reveal and gaps | 1440 light | U+202E, U+2066, U+2069, U+2028, U+E0061, U+E0064 each drawn as a bordered code point; omitted ranges "Lines 1–5 not included", "Line 7 not included", "Not included: main 9–18, branch 10–19" |

One earlier run in this pass reported a "Failed to load resource: 404" console error
with no URL. The Gallery answers `/favicon.ico` with 404 and the harness's favicon
filter read only the message text, which omits the URL; the harness now reads the
message location, and every later run, including the recorded one, shows no console
errors. A synthetic page rendering the review's own repro inputs through the repaired
renderers with the same CSS showed, at 1440 and 390 in both themes: the ls.js
added row as `// disabled:U+2028sendTokens()` with a visible notation; tag-character
lines of different widths (109.92px removed, 227.08px added); captions
`deploy/prodU+202Elmot.sh`, `fixU+202Eniam` and `internal/auth/adminU+202Eog.txt`; and
48×24px permalinks at a 24px pitch. That page and the captures were working files.

## Limits

Chrome only; no screen reader, zoom, forced-colors, print, reduced-motion (no motion is
used) or other-engine run. Characters assigned after Unicode 15.0 are not classified.
With precomputed hunks the renderer cannot know whether it has a subset or where the
texts end: counts cover the supplied hunks and lines after the last one go unmentioned. Other first-party palettes were not measured. Every line
number is a tab stop; `MaxLines`/`MaxDiffLines` and the 1000-edit budget bound input,
but no render time, payload or memory budget was measured. Collapsed runs carry their
text in the HTML. Lifecycle is not exercised (server-rendered, no script). This is not
a consumer integration, a second-app propagation or a promotion; both entries remain
experimental until Fathom integrates them and a second consumer exists.
