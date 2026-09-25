# Resume-as-Code: stored assessment evidence

Owner-selected G3/G4 follow-up to the [Pipeline/People uplift](2026-09-14-resume-pipeline-uplift.md).
This completes one bounded read-only detail surface, not RAC G12 or app-kit G1–G6.

## User task and design

Geoff reviews a role on his laptop before tailoring; phone use is reading and review.
The selected Synthwave identity remains: near-black/purple surfaces, neon-like ink,
existing semantic roles and native shell. No new palette, framework or dependency.
The ledger preserves requirement order, keeps verdicts/refusals visible, and discloses
citations and provenance through native keyboard-operable details. It is bounded to
896px, with a single summary column at phone/tablet widths.

Tailwind Plus `Application UI.Data Display.Description Lists.Left-aligned`
(HTML, Tailwind 4, dark; corpus version `2026-02-02-230729`) informed label/value
hierarchy. Its source was inspected through the licensed MCP, not copied or vendored
into the consumer. The implementation composes the existing vendored
`dashboard.RenderWidget`; no new first-party component or conversion target is claimed.

## What changed

- RAC `cmd/ops/assessment_detail.go`: pure copied projection; canonical
  `tracker.NormalizeMatch`, `UnknownRate`, N=5 floor and `Author.Validate`.
- `cmd/serve/assessment.go` plus `/app/{id}` integration: unknown/sample counts,
  normalized verdicts, refusal notes, escaped citation/reference/hash text, dates
  and recorded provenance. Identifiers never become fabricated evidence links.
- Blank requirement text retains the raw sample but refuses a percentage. Invalid
  author shape is named outside disclosure; stored fields remain inspectable.
- `FitAssessment` has no assessment-level source class. The view renders the
  canonical `unscored` refusal instead of inferring from model confidence or
  per-requirement provenance. Triage/CLI scoring and stored schemas are unchanged.
- Tailwind source and incremental task inputs now include the renderer; generated
  CSS and adoption selection hash were refreshed. Small disclosure text uses `ink`,
  not the lower-contrast `accent`; focus retains the accent outline.

ADR-018 governs score/evidence semantics; ADR-021 already resolves its dependency/type
revisit. ADR-009/014 retain the existing stack and deep links. Calibration remains F6.
RAC G12 now explicitly retains source-bounded score and rejected-lead rationale work.

## Evidence

| Check | Result | Boundary |
|---|---|---|
| Handler regression before integration | Five scenarios fail on missing section | Genuine behavioral red, not a compile failure |
| Final baseline browser | 42 missing-section failures / 44 checks | Seven states × three widths × two preferences |
| First integrated browser | 58 failures / 746 checks | 22 geometry, 36 sampled contrast failures |
| Final browser | 746/746 pass | Same logical checks, seven stored-data scenarios |
| Scoped race tests | Pass, workspace and `GOWORK=off` | Whole ops/metrics/tracker/serve packages |
| Scoped vet | Pass | Same four packages |
| CSS scan/cache regression | Pass after demonstrated failures | Source entry, task input, emitted wrap rule |
| App-kit MCP package tests | Pass | Updated CSS guidance and exact tool/resource document routing |
| Consumer adoption check | 14/14 pass | Selected bytes/declarations only, not runtime or release proof |
| Full RAC lint | Fails existing `TestKitVendorFresh` | Agentic-kit lesson-transfer source drift; no repin |

States: absent assessment; legacy empty matches; 1/4 unknown (floor); 1/5 (20%);
two refused positive claims plus one unknown (3/5, 60%); hostile/long recorded strings;
blank requirement and contradictory author. Raw rows are neither silently dropped nor
called complete coverage of the job description. Reference existence/freshness is not checked.

Browser: isolated Playwright harness with installed Chrome `152.0.7977.84`, fresh
headless profile, macOS arm64/Apple M4; 320/768/1440 × 1000 CSS pixels, DPR1,
reduced motion. Both light/dark system preferences retain the same selected Synthwave
palette; this does **not** prove two distinct palettes. Native keyboard open/close,
visible focus, ≥44px disclosures, Pipeline/back, hostile text and exact requests pass.
Fifteen screenshots include collapsed, expanded and full-page-context views. Reviewed
final desktop context, phone collapsed ledger and tablet hostile expanded ledger.

Budgets were set before measurement: HTML ≤160KiB, CSS ≤128KiB, DOM ≤2000 nodes,
DCL ≤1500ms, sampled text contrast ≥4.5:1. Final maxima: HTML 26,154 bytes across
served preview routes, CSS 28,562 bytes, 356 DOM nodes, DCL 28ms; minimum sampled
contrast 5.9986:1. These are small synthetic/local samples, not scale or latency SLOs.
Contrast uses browser-rendered opaque sRGB samples; it is not an accessibility audit.

## Reproduce safely

In RAC, supply `APPKIT_PREVIEW_DIR` as an existing empty child of the system temporary
directory, then run via devbox in a fresh process:

```sh
devbox run -- task web:css
devbox run -- go test ./cmd/serve -run '^TestExportAppkitAssessmentPreview$' -count=1
devbox run -- go test -race ./cmd/ops ./cmd/metrics ./cmd/tracker ./cmd/serve -count=1
```

The exporter asserts the unconfigured relative corpus path, uses temporary roots and
people stores, never starts ordinary `racli serve`, strips scripts/forms, and adds CSP.
Only exact fixture detail and existing synthetic overview routes are admitted.
GET/HEAD checks describe the export wrapper, not native handler HEAD semantics.

Run [browser.mjs](fixtures/resume-assessment/browser.mjs) with the export directory
and a separate existing empty system-temp output directory. It serves exact in-memory
route bytes on loopback, blocks other origins/methods/queries/WebSockets, forbids forms,
and closes its servers/browser. No signed-in profile, credentials or real records.

Local system-temp artifacts (under `/var/folders/2r/2zrkv4fd549bnlc5vwfzd4b00000gn/T/`):

| Artifact | Directory |
|---|---|
| Original route export | `appkit-assessment-baseline.PIpHN8ab9G` |
| Final-harness baseline | `appkit-assessment-browser-baseline-final.9qlYhMIlYo` |
| Integrated failing browser | `appkit-assessment-browser-after.MpZzNvpDBO` |
| Final source export | `appkit-assessment-final.KQIQbNoHDT` |
| Final report / 15 images | `appkit-assessment-browser-final.fa0ZqQ34aY` |

Final harness SHA256: `11d1f94b25bcd74d7926d01225e31b0fe91340a8f585988df32d68a323a3ccc5`.
Final report: `2de541c21773baf7a3b9968262c536f9d8905f927d3cd48a61ec781a4a195306`.
Baseline report: `76a5b796b996ecdeec2c5d021e37af171b8dcb439020bed76ac42f7c887dce28`.
Final CSS: `550547eafe3cd834db9d7b18dccf517c7f0b85270c241d330a3b2fb24e2bf94c`.
Renderer: `8aacd1f63d3be83c9f58f36d990abf62d5c8a96fff44b772d88abe8000b76012`.
Projection: `b592ffe2437ab9739589843d58ab00beccc5dd258764b15945419b28b48f21f0`.

## Return and limits

Returned normalization/refusal semantics to [experience-system](../design/experience-system.md),
and scanner/incremental-input/contrast hazards to [adoption](../design/adoption.md)
and `get_kit_css_setup`. These existing guidance routes reach MCP; tests are not a
claim that already-running consumer servers or fresh agent hosts received the update.
No generic scoring API, licensed-source export, schema/vendor migration or deployment.

Remaining: source class and rejected-lead rationale delivery, wider route polish,
real assessment/provider and fact-resolution tasks, scale/accessibility evidence,
released dependency/second-consumer propagation and consumer host proof. Existing
posting-link/action surfaces outside this ledger were not certified. Windows, CA/GGB
and optional Conjure remain deferred under their recorded owner decisions.
