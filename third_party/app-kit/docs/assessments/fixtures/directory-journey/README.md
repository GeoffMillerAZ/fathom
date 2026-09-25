# P04 synthetic directory journey

Prospective task contract under ADR-0023, extending the
[existing private table fixture](../table-adapter/README.md). Implementation and browser
acceptance are not implied by this contract; [dated results](../../2026-09-16-directory-journey.md)
record scoped proof and retained failures. The existing `Directory` and
`NewDirectoryEnhanced` APIs, original archive and previous failed receipts stay unchanged.
This fixture is not a shared directory backend, consumer adoption or target certification.

## Task and ownership

At a desk or on a phone, find synthetic colleagues by role, inspect one record without
losing the search, and export all matching observed records. Preserve the purchased
table's stacked narrow layout, palette, typography and explicit usability companion.
Add ordinary native GET controls and semantic surrounding text, not a new visual theme.
No account editing, persistence, authentication service or application JavaScript.

The fixture owns its fixed records, filtering, pages, detail routes and CSV projection.
Render list rows through a fresh private export of `NewDirectoryEnhanced`, not copied
table HTML or a test-only approximation. Adapter constructors still validate their input.
The Add action becomes an explicitly labeled "Reset directory" link; row actions are
"View" with app-owned detail URLs. Neither pretends that a user can add or edit accounts.

## Fixed semantics, before implementation

- Twelve synthetic rows, stable IDs `p01`–`p12`, in ID order. All email domains are
  `example.invalid`. Include a quoted/comma-bearing name and a formula-leading name.
  Freeze and hash the literal dataset before the browser run; do not change it to pass.
- Native GET `/directory`: `q` is a trimmed case-insensitive substring across name,
  title, email and role; at most 120 UTF-8 bytes, no controls or invalid UTF-8.
  `page` is a positive decimal integer; three rows per page. Filter before pagination.
  No matches on page1 is an honest empty result; an out-of-range page is a400 recovery.
- `theme=light|dark` selects the existing schemes, default light. Fixture-only
  `state=complete|partial|unavailable` defaults complete. Duplicate/unknown/invalid
  query fields fail400; they cannot silently broaden a selection.
- Complete uses all12 rows. Partial uses the first6 observed rows and explicitly says
  that total coverage is unknown. Never extrapolate12 or claim all known records exist.
  Unavailable fails503 without a zero count, table or successful CSV export.
- `/directory/{id}` requires membership in the current observed filtered selection,
  returns the exact record, and retains query/page/theme/state in its Back link.
  Unknown or excluded IDs fail404; no stale detail from a different selection.
- `/directory.csv` exports **all matching observed rows across pages**, in the same
  ID order, not only the visible page. The UI labels that scope explicitly.
  Columns: `dataset_revision,coverage,query,id,name,title,email,role`.
  Every row carries selection/revision/coverage; CSV quoting preserves commas/quotes.
  Prefix formula-leading text cells with an apostrophe as an explicit spreadsheet
  safety encoding. Keep displayed source text unchanged; test this mapping explicitly.
- GET/HEAD only, bounded responses, loopback Host/peer, same-origin request guards,
  restrictive CSP, no arbitrary static paths, no real records, external requests or writes.
  Missing renderer/assets refuse startup; rendering errors must not leak partial rows.

## Acceptance

Independently authored expected IDs/order/counts exercise complete, filtered second
page, detail/Back, all-match CSV, partial, empty, unavailable and malformed selection.
Negative tests plant dropped/reordered rows, stale detail and CSV disagreement in the
observation checker; a good generator alone is not its own oracle.

At390/1440 CSS pixels in light/dark, with scripts disabled, perform the native keyboard
journey and compare visible rows, detail and decoded CSV against the frozen expectations.
Check retained selection, meaningful error/recovery, visible focus, 44px standalone
controls, no horizontal page overflow and readable long/quoted/formula-leading text.
Record HTTP/CSS identities, response sizes and local task timing separately. Existing
1MiB HTML bound and zero application JS apply; no new latency budget may be invented
after measuring. No physical touch, screen-reader, full performance or zoom claim.

Use only the existing offline Go/templ/Tailwind and approved isolated Chrome toolchain.
Snapshot executed fixtures, generated private source, dataset, CSS and binaries before
browser work; retain all failed runs. Originals stay private and unchanged. A synthetic
app-owned workflow is not actual consumer integration or proof of conversion payback.

## Explicit local reproduction

Set `GOCACHE` and `GOMODCACHE` to reviewed absolute caches; builds use cached modules
only (`GOPROXY=off`, `GOSUMDB=off`, local toolchain, readonly module files). The real
home is preserved only for read-only discovery of existing pinned Tailwind. No installs.
From the kit root, run:

```sh
node docs/assessments/fixtures/directory-journey/build.mjs /absolute/app-kit /absolute/private-parent
```

The returned fresh directory contains a build receipt and two matching warm binaries.
Set `APPKIT_PLAYWRIGHT_MODULE` to the approved isolated Playwright entry, then run
`node docs/assessments/fixtures/directory-journey/browser.mjs /absolute/build-root`.
This owns ephemeral loopback/Chrome processes, retains snapshots/failure receipts and
cleans up only those processes. Pinned browser/tool upgrades need deliberate revalidation,
not automatic installation or a relaxed check. Pure tests: `go test -race
./docs/assessments/fixtures/directory-journey` and the `oracle_test.mjs` / `csv_test.mjs`
Node test files. Pure tests are not browser proof.
