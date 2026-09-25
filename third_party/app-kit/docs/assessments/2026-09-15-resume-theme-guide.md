# Resume as Code: local theme-guide adoption

Owner-authorized G1/G4 integration, not completion of either outcome. RAC's
ADR-028 owns the consumer boundary; [kit contract](../design/app-owned-styleguide.md)
remains experimental. No third-party upgrade, commit, deployment, global settings,
live registries or default theme edit. Existing work was preserved.
This is the initial ten-check delivery; the [badge/200% follow-up](2026-09-15-resume-theme-contrast-and-zoom.md)
adds two required checks and refreshed standalone/browser evidence without recoloring the app.

## Delivered behavior

`racli theme-guide [--port N]` prints an exact loopback URL, defaults to a free port,
and stops on cancellation/SIGTERM. It bypasses root config/logger initialization.
The command works outside the checkout with an explicitly nonexistent config path.
It starts neither the ordinary server nor provider/poller/data handlers.

The kit editor consumes nine fixed-clock, fresh in-memory specimens: Today,
Pipeline analytical regions and People, each populated/empty/incomplete. These call
actual app-owned renderers and the same shared shell. Navigation anchors are inert;
native disclosures remain usable. The opaque preview runs no scripts or submissions.
This is not a full Pipeline board or live application preview.

Preview/source/reset/diff and acknowledged export never write preferences. Original
CUE survives verbatim in the five-file ZIP. The initial configuration exposed four failing
checks: action and faint table text, each in both branches. Those failures remain
visible. A repaired candidate must
pass all ten selected checks to export. Whole-page accessibility is not implied.
Literal brand SVG fills, gradients, glows, scrollbar and some radii remain exceptions.

## Selected delivery

RAC HEAD `7a6c56a26dd10a7cf80128d23ef13bab90377ae8` and kit HEAD
`63b3a4f01168b184f91d1e701414445820e24fdc` are context, not working-tree identities.
RAC `cmd/serve/themeguide/sources.json` binds exactly 13 selected kit/CUE files;
only three first-party package paths were added to vendor/modules.txt. Third-party
versions and go.mod/go.sum were not changed by this integration.

CSS imports the vendored Synthwave theme, disables automatic scanning, and names
the current renderer/JS/selected component closure. The local Tailwind v4.3.0
darwin/arm64 executable is SHA-verified before execution. `task web:css:check`
always compiles beside app.css, compares and refuses stale/missing output without
rewriting it. Normal regeneration replaces atomically after compilation succeeds.

`prepare-build.mjs` retains the selected code/embed/CSS/license closure, refuses
symlinks and rechecks source hashes. Its final snapshot has 2,065 files/21,863,208
bytes. Outside sibling checkouts, initial empty build/module caches plus
`GOWORK=off GOPROXY=off GOSUMDB=off GOTOOLCHAIN=local` produced two identical
`go build -mod=vendor -trimpath -buildvcs=false` binaries using Go 1.26.3.
This is selected working-tree delivery, not a clean committed/released checkout.
No OS-level network/filesystem sandbox is claimed by these environment settings.

Removing only the snapshot's embedded token schema makes the actual build fail
`pattern schema.cue: no matching files found`. Restoration leaves all 2,065 file
hashes unchanged. A first shell canary attempt exited before mutation and is not
counted as refusal proof. The independent snapshot also passes the real CSS check.

## Verification

- Scoped race tests for the guide command, nine specimens, trust/no-app-route
  boundary, selected hashes, source/reset/export, CSS, navigation and existing
  Today/Pipeline/People renderers pass. No full RAC/lint certification is claimed.
- `TestCSSBuildCheckFreshness`, explicitly given the pinned compiler, passes
  missing/fresh/stale/repair/missing-input checks and temporary-file cleanup. It is
  an opt-in integration test; an environment that skips it earns no compiler proof.
- The maintained browser probe passes 105/105 with actual Chrome 152.0.7977.84 and
  page JavaScript disabled, first on the working-tree binary and again on the
  independently built binary. Fresh profiles, nine states, 390/1440px, both token
  branches, native invalid/repair/export and stale 409 refusal are exercised.
- Visible font-sans, rounded-lg and bg-accent consumers change to monospace,
  16px and rgb(73,44,153), then source/reset restores 8px and rgb(124,77,255).
  The named all-stage table disclosure opens with Enter; visible inert navigation
  is skipped by Tab and cannot navigate when clicked. No unexpected requests/errors.
- Actual adoption checker passes 31 static checks, including 20 artifact hashes.
  Its initial sole failure was catalog drift. Current `shell.layout`/`work.approval`
  were prospectively reviewed for the same go-web/L3b contract and source consumers;
  no old-catalog delta is claimed. IDs, layers and module declaration are unchanged.

## Failed comparisons remain failed

An early 36-pair comparison was invalid: the consumer wrapper denied CSS fetched
from the opaque preview, so both panes were unstyled. RAC INC-15/ADR-028 and
`TestThemeGuideAccessBoundaryAndNoApplicationRoutes` record the repair: only the
two immutable nonsecret CSS resources accept opaque metadata, only GET/HEAD, with
loopback/Host checks retained. Dynamic forms still reject null/cross-origin requests.
Browser probes now require actual CSS delivery and computed application.

With that repair, all 36 geometry/style pairs match. Exact default-renderer pixels
match 35/36, then 33/36 on replay with different failing cases. An unchanged-baseline
control matches 34/36, establishing capture variability, not its precise cause.
A requested software-rendering control still matches only 35/36. No tolerance,
image mask, altered baseline or green visual certification was substituted.
Full visual equivalence remains inconclusive; source identity is preserved in code.

## Replay and evidence identities

Fixture scripts: `fixtures/resume-theme-guide/{prepare-build,browser,css-comparison}.mjs`.
Local artifact root: `/tmp/appkit-rac-guide.vABdjd`; retained snapshots are
`standalone` and `standalone-final`. Browser paths: `browser-hardened-3` and
`browser-standalone-final`; failed/rehearsal directories are retained separately.

| Artifact | SHA-256 |
|---|---|
| Selected 13-file source manifest | `d7c9ac3e3b595ac140bfd388e44639e09d82262b0bffc5ce03954a691877d571` |
| Embedded CSS, 29,221 bytes | `95d5cfbe22c6a2fee60dc416a863a66563364f9762aaef78081517257b37b2d4` |
| Tailwind standalone compiler | `56b4bbc62dbdc4614a78930d9c6986423a2ec63e4e640144a59a5d95c914322e` |
| Final build-closure manifest | `84ee5021ff8f47e3baffac02eab3764f5a17414ea8809417d320fe734540cd13` |
| Both standalone binaries | `258caa6af58234a3fa4aed0a2b00ba8e111d85b309f7478b5b7f80cc23b3275d` |
| Hardened browser script | `b9ba2a925cd5c49241ecfc872fd5bd9dcfd2b8fe22f6530cafd9befe2606e2b0` |
| Standalone browser receipt | `af49c162448f1ad49095a348467af5c7438cecaaead06f8930feab96dedd4ab4` |
| Standalone exported ZIP, 16,688 bytes | `7682d604c7191e437e02a9f4e0ea2e9abd087be50a6d8a3a3ee16ea4c369b291` |
| Retained app-kit adoption checker | `a9569eb3bd644c2fcbf1eb3e6a65411f6f4f138214bc3d519eb3eb8bae22a9b8` |
| Final static adoption receipt | `470d14a260323ec70d2aff3368aa739d2f2fd3a35507f64efb7ae02ee480c2b5` |

Try from RAC: `devbox run -- task web:css:check`, then
`devbox run -- go run -mod=vendor ./cmd theme-guide`. A built binary needs neither
checkout nor compiler to run the guide. Developer toolchain auto-download issues
can be avoided with the already-installed Go 1.26.3 and the offline environment above.

Remaining: deterministic visual comparison, complete keyboard/zoom/screen-reader
and measured performance acceptance, actual external-font delivery, tracked release
and a second guide consumer before shared-editor promotion. G1/G4 stay active.
