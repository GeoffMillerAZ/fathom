# Resume-as-Code Today: first consumer uplift

Owner selected Resume-as-Code and Fathom as active UI consumers; Conjure is optional
later work. This receipt covers Today's composition only, not a complete UI redesign.
Consumer base: `7a6c56a26dd10a7cf80128d23ef13bab90377ae8`, plus the working-tree changes
in `cmd/serve/today.go`, layout tests, preview helper, generated CSS and adoption docs.
Existing host-policy changes were preserved. No dependency, vendor or business-data edit.

## Problem and chosen adaptation

Today wrapped each `hitl.ApprovalCard` in a `dashboard.Widget`. The one-column grid
became two columns at `sm`, then one again at `lg`; the 768px browser baseline showed
long titles and controls squeezed into narrow, doubly bordered columns.

The retained synthwave identity comes from RAC ADR-009/014. Tailwind Plus
`Application UI.Lists.Stacked Lists.Narrow with actions` (HTML/TW4/dark, retrieved via
the licensed MCP) informed bounded reading width and action grouping. Its avatars,
remote assets, truncation and palette were not adopted. The kit ApprovalCard retains
the richer risk, consequence preview and app-owned actions; no paid source was copied.

Now `renderTodayQueue` gives a date/count heading, ordered single-column items and
empty-state destinations. Each kit card is directly inside one list item. The queue
engine, preview renderer, allowed dispositions and scripts are unchanged.

## Evidence

| Check | Result | Boundary |
|---|---|---|
| New Today hierarchy test against old implementation | Failed on six absent queue structures | Actual behavioral TDD red, not a compile-only failure |
| Today composition tests | Pass: direct adjacency, order, exact card output, escaping, date, empty/single states | Static rendering contract |
| `devbox run -- go test -race ./cmd/serve -count=1` | Pass | Fixture handler/runtime coverage, not live operator data |
| `devbox run -- task web:css` | Pass, 27,089 bytes versus baseline 25,962 | Compiled and embedded CSS, not a payload performance benchmark |
| Chrome 152.0.7977.84, isolated Playwright 1.63.0 | 114/114 checks; zero page errors or blocked requests | 390/768/1440px; both system schemes retain the same chosen synthwave theme |
| Visual inspection | Before/after tablet, after phone and desktop inspected | Actual screenshots, not pixel-equivalence certification |

Browser assertions cover HTTP, no outer overflow, inert exports, exact fixture card
counts, named heading, direct list composition, vertical order, width bound, native
keyboard snooze disclosure and empty-state destinations. Nine screenshots retained.
Mutating actions, clipboard, error/retry, contrast and full keyboard task completion
were not browser-tested. This is not accessibility or full consumer certification.

## Isolation and reproducibility

Consumer `cmd/serve/appkit_preview_test.go` exports only fictional stores through direct
Today/Pipeline handlers. It isolates firms/people/root, starts no poller or provider,
strips scripts and forms, and adds restrictive CSP including `form-action 'none'`.
The local server serves an explicit in-memory GET/HEAD route map, not a repo directory.
Before used committed Today/CSS via a Go overlay; after used current source. Neither
export altered production files or used signed-in browser state.

Artifacts (temporary, not release evidence):

- Before root: `/var/folders/2r/2zrkv4fd549bnlc5vwfzd4b00000gn/T/tmp.alOuEyps2W`.
- After root: `/var/folders/2r/2zrkv4fd549bnlc5vwfzd4b00000gn/T/tmp.wEwQ9hZUZQ`.
- Harness/report/screenshots: `/tmp/appkit-browser-review.xBrDCp/rac-smoke.mjs`,
  `rac-smoke.json`, `rac-screenshots/`.
- Harness SHA-256: `e383eaebff9e2274e03b8e55d3a3fd57e8535a1285417ea4cd826ff0c33f9255`.
- Report SHA-256: `296484a0856143616d5708b438e6e353d8a9f8bef0c768b1fa1a6ded4f78f975`.
- Before populated Today HTML: `e173614243e0a66c98091578936b2c66ef438ea0213aec7070d9b58ed8a31313`.
- After populated Today HTML: `f325c7cfe6477731559718ddc60b5ad8f4e638d785ab2048f92d626c564af7c3`.

## Returned to the kit

`RenderWidgetGrid(Cols: 1)` now emits one column at every breakpoint for default-span
widgets. `TestRenderWidgetGridOneColumnStaysStacked` failed before and passes after;
dashboard race tests pass. Default/other column settings remain unchanged. Regenerated
Gallery CSS contains the selected class. A separate real-Go-output + Gallery-CSS browser
fixture passes nine geometry checks at 390/768/1440px; its 768px screenshot was inspected.
Harness/report: `widget-smoke.mjs` / `widget-smoke.json` beside the files above, SHA-256
`4e3189c9e4c8440388164a7e2ac0395785ef61f7d645369ef22ce7fb98117d7c` /
`7f42e7ff8a5e5fba348624c7734fad6d44b9b14909575c3e913f4ff106a4f2ff`.

Shared `work.approval` catalog guidance and the experience guide now describe the
non-nested ordered composition. No new queue abstraction was promoted from one app.
RAC's first static selection binds its existing vendored Shell/ApprovalCard and rebuilt
CSS; it does not claim to contain the new grid fix, which its Today page no longer uses.
Other consumers need an explicit source and CSS upgrade. Second-consumer propagation,
delivery pinning and fresh consumer-host proof remain open; G1–G6 are not complete.
