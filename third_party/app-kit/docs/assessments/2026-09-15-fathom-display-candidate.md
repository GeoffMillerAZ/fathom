# Fathom freshness and local containment candidate

Date: 2026-09-15 (local). Historical G3 preparation, **not adoption evidence**.
The candidate was unapplied here; the owner subsequently authorized and
[adopted the exact repair on September16](2026-09-16-fathom-display-adoption.md).
Retain the original RED/candidate receipts below; later adoption does not change
what those earlier receipts prove.

## Change and governing contract

The [unapplied patch](fixtures/fathom-handler/display-quality.patch) changes two
app-owned renderers using the existing pinned `stat`, `viz` and dashboard APIs:

- Nonempty, complete, known freshness alone permits positive `all fresh`.
- Empty/unknown coverage is absent with a reason; partial coverage keeps a named
  lower-bound stale count, known/total coverage and the first raw unknown reason.
- A registry failure invalidates even populated rows. The model carries no independent
  provenance for those rows, so they cannot supply a trustworthy lower bound.
- Key/value contents can shrink and wrap full diagnostics; absent-source prose is
  left-aligned without changing numeric alignment or trusted-HTML caller boundaries.
- Grade distribution uses intrinsic256px geometry, retaining60px label space and the
  existing11px text, rather than CSS scaling. The actual handler fixture labels its
  grade `idle`, not an assumed A–F-only dataset. Width below320 and zoom remain unproven.

Fathom I38/ADR-027 and app-kit I4/I7/I11/I17 govern. Existing aggregate-coverage
guidance applies to qualitative health summaries too. No new API, runtime, backend,
dependency, future promotion or ADR is needed. App-kit MCP was not exposed in this
session; local kit/catalog and consumer source supplied the selection context.
The shared `fathom` index still points at a scratch clone; canonical source was checked
directly after index-based location. No service reroot or wiring change was attempted.

## Reproduction and identities

Private preparation root: `/Volumes/m2ext/appkit-fathom-display.YRhqkaUe`.
`candidate/` contains the exact current CSS-source closure, builder and small pinned
mirror. Only the two Go files differ. `scripts/ui-css.sh` built and checked identical
CSS and token output offline; no stylesheet, theme or mirror update is necessary.

| Artifact | SHA256 |
|---|---|
| Original `observatory_render.go` | `309c86dd3df035a0a22a3271359498daa9e28837d90cbe4cbbdd26b25f50b34c` |
| Final render candidate | `c17dfae367276856579902d1184a56f2aa53a0a533b65d52d5d272870b99290b` |
| Original `observatory_series.go` | `6530bce155e003a0384a79e03ccd07da232c04f2f8aee938caeced4c5c8ee886` |
| Final series candidate | `7bf0e15b0e1503f2539bb781e303c7028967d2361580eb63ac7a31ed3c78d41e` |
| Original and generated `app.css` | `0e0b5e5408c5e2484f0f2adcdba6a7c9491214d3528051ef7b43300dc52acaa1` |
| Unapplied patch | `37dfeced14c40068c463342bb11274fd2b83b560b948fa31c12783cce239f4ea` |

The [maintained runner](fixtures/fathom-handler/README.md#unapplied-display-candidate)
refuses different base/candidate bytes and existing virtual test files. It saves the
executed browser/config/Go fixture bytes before compilation; Go overlays and browser
imports use those snapshots. An explicitly selected private warm compiler cache avoids
repeated cold builds. This is not release, cold-build or complete dependency-closure proof.
`git apply --check` accepts the patch against the canonical working tree; no apply occurred.

## Retained results

All run directories below are children of the private preparation root.

1. `appkit-fathom-handler-USPRb5/run-report.json`: original renderer fails11 of13
   freshness cases and the long-diagnostic wrapping test; the two known-state controls
   and required Fleet/Service/Impact page tests pass. Executed fixture/harness snapshots
   are retained. No browser rerun was needed to rediscover the previously recorded
   [fourteen actual-handler display failures](2026-09-15-fathom-handler-read-failure.md#freshness-and-local-containment-refusal).
2. `appkit-fathom-handler-idFWYS/run-report.json`: intermediate candidate passes364
   browser checks. Visual review found wrapped error prose still right-aligned; this
   run predates the left-reading-edge requirement and320px extension, not a final pass.
3. `appkit-fathom-handler-S91snJ/run-report.json`: final candidate passes all13 freshness
   cases, long/escaped diagnostic wrapping and the three required page tests with race
   detection. Real mounted handlers pass88 HTTP checks and412 browser checks, with
   zero failures, blocked requests or browser errors. Command-package selected run7.732s.

Final run SHA: `52fa2c12866757c792392c2be4eb6f6f7c174f5f5804a6c61c9375114bc0ac1b`.
Browser report SHAs: registry `e0be0cca636dc8246d03abbd35be94b4b5584c9a1d6d50a8e3a9333df34ed27b`;
pathless `22807a7a5cbecb5b60eee995798fee144b29882f0a24126d1aff1303828290d6`.
The report binds five harness/config/fixture snapshots, candidate copies and selected
canonical hashes; all source, dirty-inventory, absent-virtual-file and snapshot checks pass.

Eight native-keyboard home/fleet/service/back journeys run at390/1440 by1000 in
light/dark Chrome. Four additional resize observations reach320px and return to390;
they are not four extra complete journeys. Twelve full-page captures have matching
recorded hashes; main visually inspects final narrow-dark320 and wide-light1440.
Font measurements preserve11px labels and full diagnostic glyph containment. Thirteen
runner refusal tests pass; these are scoped canaries, not filesystem-race assurance.

The broader `^Test(Observatory|GradeDistribution|RenderGrade)` race regression passes
in1.837s over the final frozen overlay; command/output are retained in
`observatory-regression-result.json` at the private root. MCP tests pass6.047s.
Scoped structural checks pass ten root-document budgets,281 relative file links and
the58-line Unreleased block; this is not full intent-doctor or anchor/host validation.

## Evidence retention correction and remaining acceptance

An initial direct subagent RED invocation edited its live fixture afterward without
retaining the executed bytes. Its reported hash alone cannot establish replayability;
that run is transcript-only and excluded from the proof above. INC-43 recurrence is
closed locally by prospective runner snapshots and the retained baseline/candidate runs,
not by reconstructing or relabeling the lost revision.

Approval remains required before applying the patch, adding the synthetic tests and
making the promised Fathom changelog note. Then recheck base hashes, preserve paused
diffs, run consumer tests/CSS checks and current-source browser proof. Larger analytical
hierarchy, populated-grade coverage, real-user decision tasks, actual zoom, assistive
technology and complete G3/G4 acceptance remain open. Long diagnostics are now readable
and complete, not evidence that the whole dashboard is effortless. No deployment,
release, physical-input, production-performance or native-host claim follows.
