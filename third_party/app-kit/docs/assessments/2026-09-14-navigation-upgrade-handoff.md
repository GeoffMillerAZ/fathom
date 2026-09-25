# Authorized navigation/focus delivery handoff

Owner authorization: isolate and commit only the kit navigation/focus change, refresh
Resume-as-Code's vendored kit and Fathom's pinned mirror, replace duplicate navigation,
and run synthetic browser/standalone-build tests. No push, tag, deployment, global
configuration, real-data or business-logic changes. Preserve unrelated work and staging.
Kit commits: `c2b3a90` (typed navigation/focus) and `63b3a4f` (browser-returned target,
active-label contrast and Menu focus repair). RAC's scoped browser/standalone upgrade
is verified in [the delivery receipt](2026-09-14-navigation-delivery.md). Fathom's
explicit owner handoff and scoped verification are now recorded there too. This is
not whole-workflow or released two-consumer promotion proof.

## Selected kit boundary

- New `appkit/ui/dashboard/nav.go` and `nav_test.go`: bounded route selection,
  semantic links, one current item, long-label wrapping and focus styling.
- Only the focusable-main change in `shell.go` and matching `shell_test.go` assertions.
- No Go dependency changes. Exclude unrelated widget/table/catalog/token/theme changes.
- Isolate the relevant release/API guidance too: the current navigation paragraph in
  `docs/ui-components.md` is under the chrome heading and needs dashboard placement.
  Do not stage the entire dirty CHANGELOG or documentation working tree.

Before committing, refresh HEAD/index/worktree identities and prove the selected patch
in an isolated checkout. Tests against the large live worktree cannot establish that
the minimal commit is self-contained. Never overwrite the existing index or stash.
The six-file candidate was isolated against `d228711` and committed as `c2b3a90`.
It passed offline `task check`, full Go tests, dashboard race tests/vet and independent
source review; regeneration introduced no additional deltas. The exact tested patch
SHA-256 is `f683d755a53cf29050f8bad08c2225961e61bfeb442d9f2b4882259292cea085`.
Only the four selected source files and focused API/changelog notes were committed;
the existing stash and unrelated working-tree edits survived, with an empty final index.
A working-tree symbol is not an archived dependency revision.
The six-file follow-up `63b3a4f01168b184f91d1e701414445820e24fdc` independently passed
offline `task check` and dashboard race tests before commit; only scoped nav/focus
sources/tests/API notes were staged. It is the exact revision delivered to RAC.

## Resume as Code

Repository: `/Volumes/m2ext/code/geoffmilleraz/resume_as_code`.
All Go/build/test commands use its `devbox run --` convention.

Retain `cmd/serve/shell.go`'s `navPages` as the app-owned route/icon/palette source;
project it into `dashboard.NavItem` and call `RenderNav` with the active route. Trust
only fixed application icon constants. Preserve route order, identity and auth behavior.
Update `shell_test.go`, the vendored dashboard navigation/shell, app CSS and checksum,
and narrow delivery evidence. The existing Tailwind input scans the vendor tree, but
Taskfile `web:css` source tracking must include both renderer files so changes rebuild.
No module/version/dependency change is expected from this stdlib-only renderer.

Its adoption documentation/manifests contain prior uncommitted work. Do not silently
include that whole earlier slice in a navigation-only commit. Update only owned fields
and preserve their other content. Require standalone vendor/build freshness and actual
synthetic skip/current-route, keyboard, narrow/wide and theme checks; local replace or
workspace success alone is not delivery proof. Inspect current overlap before editing.
Generated CSS/checksum already contain predecessor UI work: never partial-stage them
or claim they form a navigation-only consumer commit. The present authorization requires
an isolated **kit** commit, not committing unrelated consumer predecessors; a scoped
working-tree consumer upgrade must record that broader parent-state dependency honestly.

## Fathom: explicit owner handoff completed

Repository: `/Volumes/m2ext/code/geoffmilleraz/sextant`. The owner confirmed the other
Claude Code session was paused by its weekly limit and explicitly handed off the
navigation change, requesting a changelog explanation. Overlapping renderer, CSS,
adoption and test work was preserved. `CHANGELOG.md` now names the handoff and mirror
revision. No consumer commit, staging, deployment or global configuration change.

The mirror advanced from `d228711…` to exact kit `63b3a4f…` via the scoped archive
script, not the dirty kit tree. The following recipe was applied and checked:

1. Re-read Fathom's intent, active edits and mirror stamp; coordinate with its owner.
2. Use scoped `bash scripts/sync-vendor.sh app-kit`, not an all-kit sync.
3. Replace `obsNavigation` with a thin seven-route `dashboard.RenderNav` adapter.
4. Enable `Shell.SkipLink` targeting `observatory-main`; remove only the duplicate
   nested focus target/manual skip workaround, preserving main content and routes.
5. Update affected shell tests, generated CSS, app-kit adoption hashes (including
   `nav.go`) and mirror evidence. Preserve unrelated active UI/token/business work.
6. Run `task vendor-check`, focused race tests with workspace and `GOWORK=off`, a
   standalone build, then synthetic light/dark, narrow/wide, keyboard/current-nav tests.

The source snapshot builds and passes focused race/vet checks; the 1,207-check
synthetic browser matrix passes. Tracked-files-only builds still need the new mirrored
navigation files included in the eventual consumer commit. The manifest now passes
all 16 static checks against an exact pinned checker after explicit prospective
review of its current `shell.layout` selection; old catalog bytes were unavailable,
so no old/new delta comparison is claimed. Only the catalog revision changed; the
checker and all artifact requirements remain strict. The delivery receipt records it.
At handoff, `list_services` maps service `fathom` to a scratch `pushclone`, not this
checkout. Direct local source checks were used; no shared service re-root occurred
(app-kit observation O15). Do not treat that index as checkout evidence.

## Operational caution

Intermittent macOS EPERM reads occurred on `/Volumes/m2ext` during this inspection;
later reads and tests recovered without permission/global changes. Require stable
repository access before index/commit operations. The kit commit completed during
stable access; RAC now has exact committed nav/shell bytes and a scoped adapter/source
tracking change. Final 2,094/2,094 browser checks, repeated identical standalone
builds and synthetic race checks pass; interrupted filesystem attempts remain separate.
Further intermittent RAC read failures were recorded rather than bypassed.
Conjure remains optional future F2.
