# Resume-as-Code: honest Pipeline and People analytics

Owner-authorized G2/G3/G4 consumer work, not completion of those goals. This extends
[Today's uplift](2026-09-13-resume-today-uplift.md) while retaining RAC's Synthwave
identity, shared stat/widget/viz primitives, server rendering and app-owned semantics.
No dependencies, vendored source, ranking/actions, live records or deployments changed.

## Failures and repair

The old Pipeline rendered `Avg fit 0` without assessments, or averaged preliminary
ranker scores with assessments. It also omitted archived from its terminal subtraction.
RAC ADR-018 already rejects the mixed-score average; this repairs that contract.

`ops.SummarizePipeline` now counts known active/closed/unclassified stages and actual
stored assessment presence, including score zero. The UI labels presence, not score or
evidence quality. The shared-border band sits outside widget surfaces. Current-stage
charts explain excluded closed records and offer a complete native stage table.

Channel attribution/counts remain `ops.ChannelYield`'s responsibility. The new
`OutcomeWithoutApplied` reports outcomes lacking applied-date/history evidence.
An outcome from one application divided by five different sent applications can look
like a credible 20%; `numerator <= denominator` is not a cohort check. Both rates are
unavailable for a channel with this gap; counts remain visible. Valid samples retain
`metrics.Rate.RenderFloored`'s N=5 floor, with exact counts/denominators and definitions.
Missing history is not reconstructed from current status, clamped or silently repaired.

Pipeline and People both use the same channel component in a bounded reading column.
Native disclosures separate counts from rate/source detail; coverage warnings remain
outside the disclosure. Empty, terminal-only, no-sent and incomplete states differ.
These summaries describe recorded events, not calibrated probabilities or causal yield.

Tailwind Plus `Application UI.Data Display.Stats.With shared borders`
(HTML, TW4, dark, 2026-02-02-230729) informed the direct summary grouping. Its invented
period/trends and gray palette were not adopted. The source was read via licensed MCP;
no purchased source or external runtime was copied into RAC.

## Verification

| Check | Observed result | Limit |
|---|---|---|
| New Pipeline tests over the preserved old source via Go overlay | Behavioral failures, exit 1; not a compile failure | Old mixed score, archived count, missing context and disjoint rate reproduced |
| Pure ops tests | All 10 stages, zero/nonzero assessment presence, legacy score, invalid stage, cohort gaps and deep non-mutation pass | Synthetic registry only |
| Server/ops/metrics race tests | Pass: 5.768s / 3.801s / 2.472s, uncached final run | Not live operator actions or all RAC packages |
| Scoped chart/UI lint and vet | Pass | Full lint fails separately on stale agentic-kit vendor source |
| First browser pass | 247/259; 12 narrow table failures retained | No outer page overflow still allowed clipped cells/SVG values |
| Final Chrome pass | 301/301; 42 after cases plus 12 baseline observations; 108 HTTP 200 responses | 320/768/1440px, both system preferences use the same Synthwave palette |
| Rendered geometry and native inputs | In-bounds SVG text/cells, chart text at least 14.81px; 44px disclosure targets; Enter opens stage/rate details | Not full keyboard, screen-reader, zoom or contrast certification |
| Actual adoption checker | Old intermediate CSS pin fails; final reviewed pin passes all 14 static checks | Not module-resolution, release, host-loading or clean-checkout proof |

Final browser: Chrome 152.0.7977.84, isolated Playwright 1.63.0, fresh context,
no page errors, unexpected page requests, workers or downloads observed. It serves an
in-memory GET/HEAD allowlist on ephemeral loopback origins and closes its servers.
Source exports isolate registry/people/firms/root, remove scripts/forms, and deny
submissions via CSP. They invoke direct handlers, never ordinary `racli serve`.

The browser failure produced a second repair: a smaller viewBox, proportional bounded
SVG sizing and readable text, shorter table headings with explicit definitions, and a
single-column People placement. Final phone/tablet/desktop screenshots were inspected.
The strengthened probe tests actual labels/cells, not merely page scroll width.
The initial failing probe source was not retained; its hash and result remain in its
receipt. The final source and both sets of screenshots/results are retained.

CSS: 27,089 → 27,709 bytes (+620); final SHA-256
`0ef2de4a1d953d8600e3b0d0c0fb1abede4b5fb4237cd245311c43435c50b5ee`.
RAC's adoption manifest matches the exact browser-tested CSS. This byte delta is not
a payload/render/input-latency budget or benchmark.

## Durable private evidence and replay

Packet: `data/design-assets/resume-pipeline-verified.UYrwCu/manifest.json`.
SHA-256: `55bfaac5f4132fb9a9c389c08870015c9e3a6a2e6f34abdba07e1bd556600c85`.
All 82 files / 3,525,020 bytes verified against manifest sizes and hashes: before/after
synthetic exports, selected sources, red/green test/adoption logs, browser receipts and
18 screenshots. This is not the complete dependency/build closure or a release.
The earlier packet with an intermediate CSS pin remains separate, not silently blessed.

Final browser receipt SHA-256:
`2f8a7c41e64476f9e81ee3414de135987e25f9e6d6ef9c1665991a8c69571b10`.
Reproduction and safety contract: [probe README](fixtures/resume-pipeline/README.md).
The report explicitly records artifact hashes, browser identity, cases and limitations.

Actual stdio MCP retrieval also matches current `experience-system` and `adoption`
guides through tool, resource bytes/metadata and guidance routing. SHA-256 respectively:
`bbddbda08218f14112e750d20750e40252b783ab654f8d157734b6ee135c7f1a` (11,056 bytes),
`2d3f25f0d7275d07dd530e622e6b30e8e153dbbd39a5524d83091f464cbe401b` (10,137 bytes).
This is local protocol evidence, not a newly loaded consumer agent session.

## Returned lessons and remaining work

[Experience guidance](../design/experience-system.md) now requires domain-owned
per-record cohort checks, separation of unlike score sources, and rendered text/cell
bounds. MCP serves that same guidance; no new business-metrics backend or primitive
was promoted from one consumer. RAC records INC-13 and its adoption contract/release
notes; existing agent routers already point to that contract and were preserved.

The full RAC lint gate remains red: agentic-kit moved `8b252304978b` → `2e1b1bbfd627`
and `kits/lesson-transfer/lesson.go` differs from vendor. No dependency update or gate
bypass was attempted. Fathom's shell propagation separately awaits a verified app-kit
commit and authorized mirror refresh; its in-app skip-link workaround remains.

Follow-up found legacy Overview and strategy CLI still rendering unguarded rates.
`ops.ChannelRate` now owns cohort refusal before the existing sample floor, so all
three readers inherit it. Raw `Num`/`Den`, stored schemas, CLI verbs/columns and API
fields remain; the internal Go field type changes. Synthetic tests preserve valid
80%/40% and low-N output, reject disjoint 20% and exceeding 120%, and check non-mutation.
New exports of all five states are byte-identical to the prior browser-tested exports;
this is artifact comparison, not another browser pass or legacy Overview visual review.
Independent review explicitly checked read-only computation and the existing lock split.
The final uncached race run passes ops/metrics/serve/CLI (3.330/1.822/5.632/2.075s);
vet, scoped UI lint and 14 static adoption checks pass. Re-running full lint still
fails only the documented vendor-freshness check. Archived-source Go-overlay tests
fail behaviorally for the three readers; they neither overwrite the worktree nor
weaken the production guard to manufacture a passing result.
Follow-up packet: `data/design-assets/resume-channel-guard.DtxDe3/manifest.json`,
SHA-256 `11e8206151230d2559d257002e2676429438d461530b0fcae8db2bb608546c4e`.
All 46 payloads / 517,280 bytes independently verified: selected before/current source,
red/green logs, 20 exact-match fresh exports and comparison provenance; not a build closure.
Overview's older mixed-fit/active-count summary remains outside this rate-only repair.
Open: real confirmation/error/retry tasks, aging/board/wider-route quality, full focus,
contrast/zoom/assistive tech, task-time/error and performance budgets, released delivery,
consumer-host loading and second-app propagation. Conjure remains optional future F2;
CA/GGB and Windows remain deferred. No G1–G6 box was checked off.
