# Conjure real-handler browser results

Executed 2026-09-22, America/Phoenix, under the prospectively fixed
[two-lane protocol](BROWSER.md). Conjure baseline remains
`e9d0f2e5d54a3ffd3aca9c548642849e0df274de` plus the preserved status-read repair.
The synthetic fixture is additional test source; no production, dependency, vendor,
CSS or Datastar change was needed during this browser slice.

## Outcome

| Attempt | Observed outcome |
|---|---|
| [run-01](run-01.json) | **Failed:** 23 assertions passed, then mobile keyboard predicate rejected a correctly focused native `summary`. The captured focus outline and source establish an oracle omission, not a product focus repair. Desktop sequence completed. |
| [run-02](run-02.json) | **36/36 targeted assertions passed**, after adding only `summary` to the control selector. Same fixture binary, source and assets; Chrome `153.0.8010.53`. |

Both lanes (1440x1000 light / 390x1000 dark) exercise actual five-second polling:
first failure → three clean units → failed refresh retaining all six exact prior
fragments with `13:04:05` last-good time → recovery to five → empty → orphan.
The fresh-connection test waits for actual `datastar-fetch` patch events before
checking unavailable state; initial HTML or an opened socket alone cannot pass it.
Id uniqueness, escaped diagnostics, stylesheet/theme, polite status semantics,
one visible keyboard control and document-width checks pass. No external requests,
page script errors or injected dialogs were observed. This is not full keyboard,
screen-reader, contrast, responsiveness, visual-fidelity or human usability acceptance.

An independent small-model source review checked fixture isolation and found no
blocker. Its browser-oracle review caught the initial-HTML false-positive risk and
cleanup paths before execution; both were repaired first. The later selector
correction changes no product behavior and does not erase run-01.

The fixture exited normally in 45.89s / 64.43s. Both reports record zero active
streams at cleanup; `lsof` found no listener on owned ports 54867/55485 afterwards.
These durations are fixture lifetimes, **not page-performance measurements**.
Focused Go race tests pass (`internal/webui` 2.001s, `internal/ui` 1.953s); `go vet`
passes. Other user previews/processes were not stopped.

## Visual findings still open

Inspected full-page first-failure, stale and recovered screenshots confirm the
error text/timestamp remain readable and the recovered count changes. They also
show existing defects outside this status-read repair:

1. Fixed read-only and connection badges overlap each other (both lanes), and can
   obscure mobile content. Reproduce in the unchanged read-only synthetic fixture;
   resolve badge placement and add a geometry/non-overlap oracle in a separately
   bounded shell change.
2. The connection chip says **Stale while dashboard updates succeed**. Source
   suggests an event-contract mismatch: `internal/ui/shell.templ` listens for
   `datastar-sse-message`, while the unchanged v1.0.2 asset emits `datastar-fetch`.
   That is a source-supported diagnosis, not a repaired/tested connection policy.
   A follow-up needs live-message, disconnected, retry and timeout controls, without
   conflating transport activity with data freshness or upgrading dependencies.

Doctor/Runs/SnapshotSize validity is still independent unfinished work. Do not
promote full DASH-1, G3/G4, production performance or human A/B from these results.
No Conjure code is extracted into app-kit; ADR-010's quarantine remains binding.
Fathom/RAC, operational commands, real data and consumer commits remain untouched.

## Retained identities

Local artifact root: `/Volumes/m2ext/appkit-evidence/appkit-conjure-browser.yxVa9t/`.
`run-01/` retains the failed report/focus screenshot; `run-02/` retains six full-page
screenshots. Both exact JSON reports are also preserved beside this document.
Intermittent external-volume read refusals recovered on a single bounded retry;
no permissions or global configuration were changed.

| Input/output | SHA-256 |
|---|---|
| Fixture test source | `da715c90c1a0be7f040fd85b130daff64988170b2d214b4199af69947bc07b39` |
| Compiled `webui.test` | `edf5af4e1a10392ba9899c6c084baa5c013cac0e07422d63e1562191adc3b2d6` |
| Browser harness, run-01 | `f2b530fd5535020eeab44b63737aa76f30561c92216e205232ef9f308a7d9506` |
| Browser harness, run-02 | `ec95644ecb1eafd8982e2e1050dcb88b1e2b319b4ee71dd9168a379aa8476cb4` |
| `dashboard.go` | `c21484257660ca73b7dd0a6274051c5d8ae0e484f9e3b3c3e377b8e89aea5d8f` |
| `dashboard.templ` | `1a7803d7743cf1b6f96148896023aaae69f4c0e297961b2ba85b723bd76d463b` |
| Generated dashboard | `118ca4abece7e578e5e23c196c5d8fc58decd4cc23a2bf49feada928627cdf3b` |
| Embedded CSS | `8d6dcbe030ad6cbb6aac1849da59a4c7215ee7f6346ff0e6e2b155a73c2cc17d` |
| Embedded Datastar | `dc1a02369774df324bea65cc7c12aace29d62cee9f53484ef0bcd10a07eb9a85` |
| run-01 report | `36cedd4a94b5a3282fdf117130280feaa1ebeae8989f136264cae317fc1efa87` |
| run-02 report | `b7033e77832393573c2b510b3706f247d751a57982fce90ca437cc297c228d4f` |
| run-02 desktop first failure | `2354b9f9d04bf8ecf9a5254274842ff2286a44d03d76d0dafb9672513aa63fa0` |
| run-02 desktop stale | `6dec436152539849fe682176d462bff4def05510bbba0c1b1d196ed1b228081e` |
| run-02 desktop recovered | `6d5c9dca07c205b520c3f25355f809502c309a0a596c37769136880148b78c26` |
| run-02 mobile first failure | `14265f66b85e9ebe55fe88b454eb79bf4a68456a42f89dbb8f24319236900bb8` |
| run-02 mobile stale | `b30b279cdb0fb85999ec354c38e69f22c134cd392f80c730b5cb5053d0c5e335` |
| run-02 mobile recovered | `2ab6dd90b12dcde2300ba823b96f74fa4e8530decac5d0549907a470802fd706` |

To reconstruct run-01's exact harness, remove only `summary,` from the keyboard
selector in `browser-smoke.mjs` and verify its recorded digest. Never replace its
failed report with a later success. Consumer patch ordering is in [README](README.md).
