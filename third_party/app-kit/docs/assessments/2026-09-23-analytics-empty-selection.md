# Analytics empty selection and recovery

## Selected task and prospective acceptance

R1/R4 owner-funded core work, baseline `4406d70`. The current Gallery analytics
fixture cannot exercise a successful query with no matching services. Existing
48 nonempty combinations and shared empty-dataset tests are not this recovery task.
The full offline baseline Go suite passed before this change; retained source/log:
`/Volumes/m2ext/appkit-evidence/appkit-full-regression.IhmsRq8p`.

Add explicit synthetic `category=empty`, labelled as an empty fixture group, to
the existing comparison and public-renderer forms. It is not a new source condition,
query engine or shared API. Current and reference datasets retain their context but
have no rows; never derive a failure total, difference or reliability interpretation.
Distinguish no matches, measured zero and missing observations. Other filters/themes
and source conditions stay independently visible.

Offer native “Show all service groups” recovery that changes only category. Hide the
empty comparison/table scaffold on the paired overview; keep the exact public renderer
on its specimen page with app-owned empty-selection explanation and recovery. Do not
offer “Load the complete fixture” as if it clears a group filter. Nonempty behavior
and the existing48-combination CSV digest must remain unchanged.

Keep the33-column comparison and27-column spreadsheet exports header-only for empty
selections; explain that they contain no row/context values. The existing renderer
CSV retains current-period context. Label that scope explicitly when the overview
has a previous-period selection; do not imply that it exports both empty windows.
Filtered-out service detail stays404 with a selection-preserving recovery link.

I4/I7/I8/I11/I12/I17/I18, ADR-0069 and the analytics journey govern. The missing-task
revisit condition is addressed within the existing experimental Gallery composition.
No permanent promotion, new ADR, consumer/config/dependency change or installed-host
refresh. Existing human A/B, consumer and broader quality gates remain open.

Acceptance: all16 empty period/comparison/source-condition combinations; light/dark
HTTP structure and GET/HEAD; native recovery/Back/export in two fixed browser cells
(1440×1000 light September/previous/complete;390×844 dark August/none/partial).
No scripts; exact source-collected HTML/CSS, bounded fresh Chrome ownership, no external
requests, preserved selection, visible keyboard focus, no page overflow, reviewed
captures. Existing normal report/CSV tests remain unchanged. HTML cap32KiB per page.
Freeze executed source and harness before each acquisition. Budget: one RED, one
GREEN plus scoped regressions, one browser acquisition and at most one diagnosed
harness correction with its failure retained. Stop after this task; no theme matrix,
native-select permutations, memory research or consumer upgrade is admitted.

## Results and retained corrections

The feature passes its bounded task. Two new Go tests cover64 subcases:16 selections
in both themes on each overview/renderer surface, including GET/HEAD, empty detail404,
recovery destinations and three CSV formats. The first new test harness had an HTML
import/unused-variable compile failure, then needed the real CSV reader's blank-line
semantics and the shared renderer's intentional empty frame respected. Original and
corrected test bytes/logs remain in the private packet; those failures are not passes.

The first broader Gallery race run failed only the existing filter-options oracle:
it still required exactly three groups. Its original48 nonempty selections remain;
the options assertion explicitly admits the new fourth group. The144-export digest
and its independently checked role columns are unchanged. Final full Gallery race
passes44.586s; shared analytics, MCP and qualityplan races pass1.308s/37.546s/14.629s.
Gallery vet and diff whitespace checks pass. Pinned Tailwind4.3.0 regeneration is
byte-identical: CSS46,191B, SHA256
`f5da664c9007f1bcf3e7da4e4025b1c781722ceb8a5494be696b894b142a0f2d`.

The first browser acquisition passes154 checks, zero failures/unexpected page requests,
four workflow cells (overview and renderer in each preselected viewport/theme), and
eight native downloads. Node26.3.0, Playwright1.62.1 and installed Chrome153.0.8010.53
are input-pinned; Chrome uses a fresh sandboxed headless profile with application
scripts disabled. Source-collected `gallery.New()` responses are fulfilled locally,
not a live server, consumer deployment or clean standalone build. All17 collected
responses have expected status;10 observed HTML bodies and their completed CSS match
their retained hashes. HTML maximum27,675B remains under the preselected32KiB cap.

Native Tab/Enter reaches the44px recovery targets with visible focus; group recovery
and browser Back preserve the remaining selection and theme. Two expected detail404
console messages are explicitly matched to their URL/message/response, not ignored
generically. Downloads prove exact header-only33/27-column exports and context-only
renderer exports without fabricated rows. Input hashes are unchanged after execution;
the owned Chrome child exits/closes with complete capture and no forced termination.
Eight empty/recovered captures are retained. Empty views have clear explanation and
recovery, no comparison scaffold, and readable wrapping; the renderer specimen keeps
its actual empty-table message and intentional inner table scrolling.

The assessment harness lives in [fixtures/analytics-empty](fixtures/analytics-empty/).
Pre-acquisition review corrected export labels, empty-table placeholder expectations,
CSV blank-line handling, bounded navigation/font checks and expected404 classification.
No browser rerun, application workaround or acceptance weakening was needed.

## Identity and limits

Private packet: `/Volumes/m2ext/appkit-evidence/appkit-analytics-empty.Fm2oll5x`.
It retains baseline/source overlays, failed and corrected test bytes, logs, the exact
collector/binary, all17 responses, the browser/helper/manifest and eight captures.

| Artifact | SHA256 |
|---|---|
| Build source overlay | `91de478061ab5ec0ee766bf54253894f3c811ae3c7ef0a9ae038ef6ad8d852c7` |
| Collector binary | `ea2e62c403af071c4d8071edcf16b8754a2f3dfb2034d8fe47a451e13d34fd8a` |
| Collected responses | `4eff4288175198a880abfd6f8d3d1a21899b649f71423a06b2853b3d552504cd` |
| Browser driver | `d4202aa1304b924b9164171114758d52f1ec67dd16ef5f608764fff5421f47fe` |
| Input manifest | `d71fe9a6dd0857dcb9994c1fd2a4c9289510e9467d44ccbc308d20f67ae10337` |
| Browser report | `0dcd5b16f082689727d4223ab0fa53460b69599be97fa1360f118e9472297715` |

No shared API/schema, dependency, consumer, installed-tool or catalog-pin change.
This closes one Gallery recovery gap, not G3/G4/core certification, human A/B,
screen-reader/all-browser/zoom coverage, memory or performance acceptance. The native
select lane remains parked. Future adopters should preserve the distinction between a
successful empty selection and a failed source; never replace unreadable input with
an empty slice, manufacture a zero row, or call a state-only reset filter recovery.
