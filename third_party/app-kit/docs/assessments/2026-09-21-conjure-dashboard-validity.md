# Conjure: bounded dashboard validity pilot

## Selection and boundary

The owner reports concurrent Fathom work and offers Conjure instead. Fathom's
reviewed packet in `8b311b8` is preserved, not applied; its failed native ZIP task
stays failed. RAC's separate active CSV integration awaits its scope answer.
Conjure's existing Status-read/SSE workflow is the selected R4 task under ADR-0072,
not a whole-app redesign, dependency upgrade or replacement for G3's two selected
consumer outcomes. Human A/B remains explicitly owner-deferred.

Conjure started clean on `poc`,
`e9d0f2e5d54a3ffd3aca9c548642849e0df274de`. Its intent, UI-A specs and ADR-010/016
govern: one Go/templ loopback app, authenticated reads, explicit write consent,
canonical engine types and token roles. No Node product dependency, Wails,
post-cutover engine feature, real machine data, global configuration or extraction.

## Delivered locally

- Failed Status reads no longer become empty successful structs; `(nil, nil)` also
  becomes unavailable. Successful empty configuration remains a distinct state.
- Each SSE connection retains its last-good fragments on a later failure, with a
  visible successful-read time. Recovery clears the banner; a new stream starts
  independently. There is no process-wide status cache or new data contract.
- Whole id-bearing fragments use outer patches, avoiding nested duplicate ids.
- Orphan-only status cannot claim full convergence. Diagnostics remain escaped,
  wrap long strings and carry polite announcement semantics.
- Conjure's `docs/app-kit.md`, paired AGENTS/CLAUDE routing, PRODUCT entry point,
  changelog and INC-7 preserve adoption/contribution guidance. The product entry
  point derives from existing decisions, not a new visual brief or authority.

Source is applied in Conjure's working tree, **not committed or pushed there**.
The [portable patch](fixtures/adoption-analytics-2026-09-21/conjure/README.md) preserves
all 13 originally selected paths in this private kit, including generated source and
tests. The later four-path browser increment adds one test fixture and updates three
handoff documents; applying both in order to the exact private baseline index yields
zero diff against the active result. The increment also reverse-checks cleanly.
No Conjure dependency, vendor, CSS, Datastar, mutation route or installed binary changed.

## Evidence

| Check | Observed result / limit |
|---|---|
| Synthetic validity suite | Five new test functions cover nil/error, first failure, success, stale failure, recovery, a second outage and initial authenticated HTTP/SSE rendering; pass under `-race` |
| Renderer regressions | Unavailable/empty/clean/orphan-only states and escaped long diagnostic text pass; reviewed degraded-banner golden updated |
| Package regression | `go test -race ./internal/webui ./internal/ui -count=1` passes (2.285s / 1.390s in retained terminal observation); `go vet` for both packages passes |
| Generator | Manifest-matched templ v0.3.1020; repeated single-file generation yields SHA-256 `118ca4abece7e578e5e23c196c5d8fc58decd4cc23a2bf49feada928627cdf3b` |
| Review | Independent source review found no blocking regression; its unbounded SSE-test wait finding was repaired with a deadline, pipe cleanup and handler join, then race-tested |
| Intent structure | Read-only doctor found an 82-line router; trimmed to 80. Other applicable structural checks passed. Installed-host version drift remains a warning, not a host-integration pass |
| Real-handler browser follow-up (Sep 22) | [36/36 targeted assertions](fixtures/adoption-analytics-2026-09-21/conjure/browser-results.md), Chrome153, 1440/light and 390/dark, actual Datastar polling; repeated updates, first/stale/recovered/empty/orphan states, fresh-stream isolation and zero remaining streams. Initial failed oracle run retained; no production changes during browser slice |

Earlier test attempts remain part of the result: initial new tests could not compile
before the stream-state seam; a pre-canceled request produced no events and a
unit-id expectation was wrong for the converged view. Corrected fixtures use live
synthetic request cancellation and the rendered count. These are harness corrections,
not evidence of prior product success. The original banner golden failed before its
reviewed announcement/wrapping update. macOS briefly denied source/module-cache
reads; one bounded retry recovered access, with no permission/configuration change.

## Still open, and smallest next task

Doctor/Runs/SnapshotSize errors still lose their own validity information; they must
be modeled and tested independently before claiming dashboard-wide truthfulness.
The bounded browser follow-up supplies real rendered workflow evidence, not broad
visual, screen-reader, human usability, timing/performance or full-convergence
acceptance. Existing UI-A plan rows stay unchanged.

Next: the browser found overlapping read-only/connection badges and a stale connection
chip despite successful updates. The [receipt](fixtures/adoption-analytics-2026-09-21/conjure/browser-results.md)
retains repro/diagnostic pointers and bounded next controls; neither is silently waived.
Then address independent source validity and review an immutable kit/vendor/CSS upgrade
before adopting the already-specified recent-run sparkline. No new metric calculation
is needed; use canonical `engine.RunsInfo`.

Contribution disposition: [analytics consumer checks](../design/analytics-journey.md#consumer-adoption-checks)
gain failure/recovery and patch-mode lessons. No Conjure code is promoted into a kit
API: its 90-day solo-managed quarantine plus named second-consumer gate still applies.
This bounded handoff earns no G3/G4 promotion or artificial weighted-completion credit.
