# Revision-bound Gallery quality review

## Prospective task

Baseline `189012c`. A guarded capability page links to quality work without the
catalog identity. After a rebuild, following that link can silently show work joined
to different capability metadata. The quality-inventory revision does not detect a
catalog-only change. MCP already uses the shared reader's independent catalog guard;
Gallery currently rejects that argument. Its use of `URL.Query()` also discards
query decoding errors instead of rejecting the entire malformed selection.

ADR-0020's Gallery/MCP contract-divergence trigger applies. Reuse the existing reader,
not another registry or API. I7/I8/I15/I18 and ADR-0024/0065 preserve read-only review,
distinct metadata/evidence identities and app-owned adoption decisions. No new ADR
is needed to extend the existing precondition to its other presentation surface.

## Fixed acceptance

- Optional `expected_catalog_revision` on HTML/JSON quality list and detail uses
  `ListForCatalog`/`GetForCatalog`. Omitted/empty keeps latest-snapshot access with
  unchanged JSON bodies. A valid stale guard returns409 without work-item content;
  malformed, repeated or invalidly encoded input returns400. Keep independent
  inventory-guard behavior, unknown-item404 and read-only method handling.
- Capability-to-quality links bind the reviewed catalog revision. Quality detail
  links bind catalog and inventory revisions. Native filter submissions, Back and
  theme links retain the applicable catalog guard; Back omits the detail inventory
  guard. Deliberate filter reset preserves catalog identity rather than accepting drift.
- Stale catalog recovery goes to current capability search, preserving a valid theme
  and using the selected capability ID as search text where valid. It never silently
  resets directly into current quality work. Explain that app/pins are unchanged.
  Error responses are not cached. Invalid input still offers a usable recovery path.
- HTTP regressions cover exact JSON parity, strict parsing, independent refusals,
  guarded native links/forms and successful explicit recovery. Preserve failing
  baseline tests before production edits. No catalog/quality hash or metadata change.
- Freeze an actual-handler fixture and browser driver. At1440/light and390/dark with
  application JavaScript disabled, use native keyboard navigation from capability to
  work detail/Back, change scope through the native form, and recover from a stale
  link via fresh capability review. Check exact completed document/CSS responses,
  focus, no overflow, no unexpected requests and bounded process cleanup; inspect
  stale/recovered captures. This is not full accessibility or consumer certification.
- Run scoped races/vet and a real isolated CSS rebuild. Retain source, tools, commands,
  failure logs and executed fixture identities. No consumer, installed host, dependency,
  daemon or global-configuration mutation. Bound this packet to40 minutes from11:10UTC;
  preserve incomplete evidence rather than weakening the acceptance contract.

Private evidence: `/Volumes/m2ext/appkit-evidence/appkit-quality-review.HM1Aj9gC`.

## Review refinements before browser acquisition

The first candidate passes the five initial HTTP regression groups. Independent
review identified two untested acceptance edges: an unrelated invalid filter should
reset quality filters while retaining a valid catalog guard (empty still means
omitted), and unsupported methods should also receive the promised no-store header.
Add explicit regressions against that candidate before correcting either behavior.
Malformed/stale or duplicate guards still require capability review; strict decoding
refuses the request while recovery may retain separately valid theme/capability fields.

Browser harness review precedes execution: action navigation must be observed without
a compensating `goto`, native form traversal must reach the actual submit button,
and completed document/CSS bytes must match the retained handler output. Query-order
normalization is allowed, but not removal of duplicate or changed query parameters.
Keep all runtime assertions and initial failed runs; no oracle relaxation is admitted.

## Results

`red` retains five failing regression groups against the unchanged handlers.
`green` passes those groups and the scoped race suites. The separate `review-red`
candidate fails the newly added empty/current guard recovery cases and method-response
header assertions; those failures remain retained. The final implementation resets
unrelated invalid filters without dropping a valid guard, treats empty like omitted,
and adds explicit read-only405 fallbacks with `Allow: GET, HEAD` and no-store.
Malformed/stale/duplicate catalog guards still require fresh capability review.

`final` passes Gallery, quality-plan and MCP race suites (1,187 test/subtest events;
12 opt-in/environment skips), Gallery vet, the real isolated CSS compiler and a
cgo-free actual-handler fixture build. CSS remains byte-identical. Skips cover the
archive's missing-Git binary guard, ordinary opt-in installed/private-source/evidence
checks and unrelated live/browser/CSS fixtures; they are not runtime passes.
The selected browser task and real CSS build are executed separately below.

The first browser acquisition stops after26 checks: native `End` does not change
the scope select on this Chrome/macOS combination, so the expected all-scope
navigation never occurs. Process cleanup passes. The separately frozen reviewed
driver uses native `a` type-ahead; it does not assign the selected value or weaken
the expected all-scope destination. Both failed and corrected driver bytes are retained.

`green-browser-reviewed` passes116 checks across1440/light and390/dark using Chrome
153.0.8010.53, Playwright1.62.1 and Node26.3.0. Native Tab/Enter, scope selection and
form submission carry the guards through quality detail/Back; stale409 offers fresh
catalog review, whose current capability returns to the selected work. Completed
document/CSS bodies match exactly, with no application scripts, unexpected requests,
page overflow, forced cleanup or unresolved child. All four stale/recovered captures
were inspected: long hashes wrap and recovery actions remain readable and visible.

The final fixture binary SHA-256 is
`c8c3d3978b655097c26b02c118e41315e9d46a449fb1f8abffc763c8f3ff4f54`.
Its response packet is byte-identical to the browser-tested candidate:
`a6bd4276fcc78e5f13f02a5388028336a9c697c9b05123dc38d5178dfd03be31`.
This binds browser review to exact handler output; it is not a live-server lifecycle
or consumer-adoption claim. The private root retains archives, overlays, executable,
runner/browser/helper identities, commands, full logs and failures. Go builds disable
VCS stamping; hashes identify private source snapshots, not clean released binaries.

Independent review found no remaining concrete defect in the corrected handler.
Final checkout checks pass the Git binary guard, served-document parity/current
guidance/decision index,111 relative links, root intent/changelog budgets, paired
agent files and whitespace. Executed Go/fixture/browser/helper bytes match the
retained snapshots. These targeted checks are not a whole-repository or fleet audit.
Rebuild Gallery to receive this repair. Catalog/quality source, metadata pins,
dependencies, installed MCPs and consumer repositories are unchanged. No standard,
target certification, broader accessibility/performance or G1/G4 acceptance is promoted.
