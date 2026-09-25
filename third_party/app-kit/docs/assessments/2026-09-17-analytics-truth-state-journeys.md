# Analytics truth-state comparison journeys

This extends the [existing browser probe](fixtures/gallery-analytics/README.md), not
the production renderer, public API or consumer configuration. G3 requires the same
truth across selection, explanation, chart, table, detail and exported evidence.
Prior browser lanes remain intact; the new matrix selects September2026/API services
against August2026 using native keyboard controls, disclosure, detail/back and downloads.

## Fixed oracle

| State | Current Gateway/Search | Previous Gateway/Search | Comparison |
|---|---|---|---|
| complete | 4 / 1 | 6 / 3 | both2 fewer; total5 versus9 |
| missing | missing / missing | 6 / 3 | no current total, numeric plot or difference |
| stale | 4 / 1 | 6 / 3 | same numbers; stale source explicitly disclosed |
| partial | at least3 / at least1 | 6 / 3 | lower bound4; no full-month comparison |

Expectations are authored from the fixed Go fixture contract, not learned from browser
output. Exact visible narratives, state labels, per-row values/deltas, SVG label/value
sequences, freshness and returned query selection are checked. Source disclosure is
opened with native keys. Missing emits no SVG; partial plots current observations only.
Gateway detail is tested; this is not both services' detail coverage.

Each mode adds four fresh contexts at1440 outer pixels: complete/stale light and
missing/partial dark. Ordinary mode is1440 CSS pixels; actual Chrome200% is720 CSS
pixels, selected through Appearance settings and checked against browser geometry.
These four states are not crossed with every viewport/theme. Existing partial-only
390/768/1440 light/dark and200% lanes remain separate regression coverage.

## First frozen acquisition

Private packet: `/Volumes/m2ext/appkit-evidence/appkit-theme-cancel.F2rbr0oq/analytics-states-AHYPJF/`.
Owner receipt SHA `cd9139260a47d7d1912c309f478d4b8453bf8afcb7f31e35138036dfb8d55a69`.
Reviewed input manifest SHA `c1d973247b0019232128fef2b675fd4fa0e40cd8f6cea7c396817b993c4f3921`.
Harness SHA `f8d8b2682434fca785f2c3d20c7f7000196176ff41deac64cc780e992a78ebca`.

| Lane | Checks | New state journeys | Captures / CSVs | Browser receipt SHA |
|---|---|---|---|---|
| ordinary | 1,691 pass /0 fail | 4/4 | 20 /14 | `451cb0342ab2a26e8baaff1b74062f23664d8897168ec31f7edd58157e615712` |
| actual200% | 1,156 pass /0 fail | 4/4 | 12 /10 | `00b40321c681e3ede6d3a098f3cecea8d81b4b2f78b71e01ce86cfa2f56a2243` |

All56 retained PNG/CSV artifacts rehash correctly. Counts include repeated assertions,
not independent requirements. CSVs have exact33-column headers and checked numerical,
selection, window, freshness, completeness, missingness and meaning fields. Independent
review found that this first oracle did not constrain every provenance/contract cell;
it therefore does not prove all33 cell values. That stronger gate is separate below.

## Source, runtime and ownership

`quality-analytics-states-readable/` passes race tests for Gallery, quality inventory,
MCP and the controlled fixture, then builds the synthetic server. Its receipt SHA is
`b89a406612de502f41aa42c4bf854d293cd3f0e8e18315b4f478a4dd82d23ac9`.
The earlier `quality-analytics-states/` attempt encountered intermittent volume EPERM;
its retained source remains, but result capture/receipt failed and acceptance is unproven.

The owner binds the selected build, harness/helpers, Node, Go, Playwright entry/package
and Chrome launcher before executing retained harness copies. Versions: Chrome152.0.7977.84,
Node26.3.0, Go1.26.3, Playwright1.63.0 on macOS arm64/M4/32GiB. All15 existing capture/
setup helper tests pass. Source/tool/build identities remain unchanged through execution.
No new dependencies, signed-in profile, real app data, consumer edits or deployment.

Both browser receipts have zero unexpected errors or blocked application requests.
The harness closes its contexts; the owner proves its direct Node/server child cleanup
and Gallery-port refusal. This is not independent browser-descendant inventory or
OS-wide network containment. Five fresh DOM-ready samples per mode yield local p95
9.9ms/8.4ms against the unchanged250ms ceiling, not a speedup, human task time or INP claim.

## Full-cell follow-up

The separate all33-cell acquisition is retained at `analytics-states-k4Foiw/` under
the same private parent. Its owner receipt SHA is
`1e7653eb6ec0a2d3a593470ae4426ec0468264c9d65d27ed53319e4978edabf2`;
reviewed manifest `b63dcf7d882514d03bd5c53714d07e5ad121e4082cb378dd9fa3aa7db42c2bb6`;
harness `1db07a5a5e6b1279053f8d9bd1c249d99737f41e8efc26b1c5beec3587059bee`.
It preserves the build, budgets and existing assertions, adds full contract/provenance
cell expectations for both periods and Gateway detail, and records started failed
phases separately from later unexecuted phases. Both modes again pass4/4 state journeys,
1,691/1,156 checks and15 helper tests; all56 artifacts rehash correctly. This later
proof does not upgrade the first oracle or prove visible SVG title/ARIA semantics.
Browser receipt SHAs: ordinary
`b49b3bb13a0a0d1c9a6ac0c9cfaad19d86b7da6ac93f3956a1daa49d03b0af4d`;
actual200% `dad7f38eb6dc3452ded7cf57d51387b12a00c63772b8907e3977087238b1b8f2`.
Local DOM-ready p95 is17.1ms/9.6ms in this acquisition; timing boundaries/limits above apply.

## Remaining acceptance

Full consumer journeys, every state/theme/viewport combination, screen readers, human
baseline/improved task outcomes and consumer-scale performance remain open. This bounded
synthetic evidence does not complete G3 or promote a public workflow API.
