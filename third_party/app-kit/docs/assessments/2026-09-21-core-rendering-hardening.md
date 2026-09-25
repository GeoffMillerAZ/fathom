# Bounded core rendering hardening

## Resume contract

- **Packet:** R1, owner-funded maintenance batch requested on 2026-09-21.
- **Selected work:** `core.analytics` / `analytics.comparison` and
  `core.static-content` / `content.docs`, Go-rendered web and static HTML.
- **Need:** malformed analytical text must not acquire different meanings in
  HTML and CSV; layout admission must not overflow; a document link must target
  the caller's literal heading ID, including percent signs and Unicode.
- **Inputs:** clean source `0fe6292129a7df9138d52cecd9109f921bf066e7` before this
  batch; the existing stale generated-file stash is unrelated and retained.
- **Authority:** app-kit implementation, synthetic local checks and documentation.
  No consumer writes, dependencies, paid-source changes, global configuration,
  installation, release, deployment or native-host experiments.
- **Budget:** three narrowly scoped fixes, at most 90 minutes after selection,
  two bounded browser acquisitions (one baseline and one candidate), then a
  verified handoff or explicit residual gap. Do not restart the G1–G6 campaign.
- **Owner / evidence:** app-kit maintainer; initial private evidence directory
  `/private/tmp/appkit-core-rendering.QX8bLn`, with retained source/test snapshots
  and logs copied to repository-local ignored artifacts before cleanup.
- **Contribution:** return fixes and regressions to the shared packages. Existing
  adopters may select a reviewed pin later; no adoption or promotion is inferred.

## Frozen acceptance

1. Analytical text rejects invalid UTF-8 and NUL before any HTML or CSV output.
   Existing 2,048-byte limits, valid Unicode, text escaping, numerical meaning,
   zero/missing distinctions and spreadsheet formula protection stay unchanged.
2. `RenderBars` rejects label-column widths outside its existing layout budget
   without integer addition overflow. Defaults, legitimate explicit dimensions,
   low-level `viz.Bars`, and the documented table-only fallback stay unchanged.
   This batch does not invent a new minimum row-height contract.
3. Boxed/panel/rail outlines and heading permalinks encode URL fragments separately
   from HTML attributes. Parsing a link recovers the supplied literal ID; native
   browser activation reaches that heading without slugifying or renaming it.
   Ordinary ASCII IDs and empty-permalink behavior retain their existing output.
4. Focused regression suites pass, with pre-repair failures retained. The small
   browser task uses synthetic headings, script-disabled content, keyboard
   activation and exact target assertions. No external assets or real data.

Applicable standards: typed-config, native-interaction, honest-states and
safe-composition. Exact replay and source identity support reproducible-delivery;
this is not a new broad performance, visual, assistive-technology or consumer gate.
All affected core inventory statuses remain partial unless their entire declared
acceptance is independently met. Historical receipts and failed gates remain intact.

## Results

Implemented the three scoped boundaries. Analytical text and dimension regressions
fail against retained pre-repair source and pass after repair. Full analytics/docs
race tests pass. The pre-existing CSV NUL/formula canary now asserts atomic refusal;
all other spreadsheet neutralization cases stay intact. Malformed text refusal is
an admission tightening: callers must repair or reject it before constructing data.
Review also pinned empty-ToC `href="#"` compatibility and acceptance of `RowH: 1`.

Both **baseline and candidate** passed all 24 native-keyboard fragment cases in
Chrome 153.0.8010.50, Playwright 1.62.1 and Node 26.3.0. Cases cover four renderers,
literal percent/encoded-looking IDs with decoy headings, Unicode and punctuation.
The original byte output fails canonical URL round-trip tests, but this browser
already resolves these literal IDs correctly. This is encoding-contract hardening,
**not** evidence of a reproduced browser navigation defect. No visual, theme,
assistive-technology, consumer or performance certification is claimed.

The script-disabled fixture server and temporary browser process closed; their ports
refused new connections. The runner, fixture bodies, executable identities, receipts
and screenshots remain in the private evidence directory above, copied to
`workspace/artifacts/core-rendering/2026-09-21/`. No browser dependency was installed:
an offline acquisition failed and the existing product runtime was used explicitly.
The CUE inventory links this scoped evidence without upgrading any partial status.
