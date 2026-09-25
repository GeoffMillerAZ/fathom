# App-owned detail links in the analytics recipe

## Prospective task and boundary

Baseline `116d55d28e18d62d483f71c0f37cff8e87a10b24`; owner-funded R1/R4/R5 work.
A person reads a selected analytical row, opens its app-owned detail, returns and
exports the unchanged dataset. The public recipe currently forces an adopter to
duplicate the table to link its row labels. Gallery's existing service-detail route
is the immediate need witness; this does not authorize consumer repository writes.

Add optional `Row.DetailHref`, empty by default, with atomic validation before
either renderer or CSV output. Admit only bounded root-relative in-app URLs;
reject external/protocol-relative targets, malformed escaping, controls and
backslash ambiguity. Escape URL and label independently. Missing/zero/signed and
table-only fallback rows retain their app-supplied link and complete meaning.
The app owns destination existence, authorization, selection context and recovery.
CSV remains the same ten-column presentation export and never includes the URL.
Zero-link callers retain byte-identical markup; positional Go literals need a
named-field migration. No new route engine, dependency, script, promotion or server.

I4/I7/I8/I11/I12/I14/I17/I18 apply. ADR-0069's missing-critical-task trigger fires;
ADR-0074's richer-navigation trigger is resolved by a data-only presentation seam,
not a chart/router engine. ADR-0075 records the compatibility and authority boundary.

## Acceptance and stopping budget

- Freeze tests/source before RED and GREEN. Test URL admission/refusal, escaped
  label/href, both renderers, missing/zero/signed/fallback cases, atomic output and
  byte-identical CSV. Compare unlinked original/candidate markup separately.
- Gallery's exact public-renderer specimen supplies its existing detail URLs;
  filters/theme survive the destination. Existing paired workflow/export stay intact.
- Preserve all selection cases and schema/metadata/refusal regressions. Discover
  the seam through the existing catalog/MCP; no competing capability or server.
- One bounded no-script native-browser acquisition: selected wide/light and
  narrow/dark views, keyboard link activation, exact destination/data, browser Back,
  unchanged export and visible focus/44px link target/long-label wrapping. Retain
  complete document/CSS inputs and inspect captures. A harness correction gets one
  separately frozen retry; application failures remain failures.
- Gallery HTML stays within its existing 32KiB cap; no new client asset or runtime.
  This is not a new performance or full accessibility acceptance claim.
- Up to 60 minutes implementation/acquisition, then retain a coherent result and
  exact remaining work. Human A/B, real adoption and G1/G3/G4 acceptance stay open.

Evidence root: `/Volumes/m2ext/appkit-evidence/appkit-analytics-detail.tvOEWmrg`.

## Results

Implemented under ADR-0075. Gallery clones the selected rows and supplies only
destinations present in its existing journey. Both renderers use the same native
label link; app routing/authorization and the ten-column CSV remain unchanged.
Catalog/MCP/Gallery selection adds two profile-bound cases, 79 total, under the
existing experimental capability. No installed server or consumer was upgraded.

### Executed evidence

- `red/` freezes the absent-rendering/validation failures before implementation;
  `green/` passes the unchanged core tests. `regressions/` retains three failures
  from a catalog-preview test still expecting the former plain-label specimen.
  Its reviewed migration checks the linked public output and independent native
  destination, while still comparing CSV against the original unlinked dataset.
- Independent review found C1 Unicode controls were not excluded by the first
  validator. `red-c1/` freezes literal/encoded counterexamples. The final validator
  uses `unicode.IsControl` before and after one decoding pass; encoded spaces and
  valid non-control Unicode remain accepted. Explicit signed-fallback zero/missing
  cases were added before that run, not inferred from another row's result.
- `regressions-reviewed/` passes race tests for analytics, viz, Gallery, catalog
  and MCP with no source drift. The broader HTML/CSV agreement and profile-refusal
  suites remain intact. This is not a full root-suite or consumer execution claim.
- `build-reviewed/` retains exact baseline/candidate source archives, resolved Go
  binary/version, offline environment, binaries and output bytes. Unlinked signed
  specimens are byte-identical in both themes, including normal and fallback tables.
  Empty-link byte preservation is also structural: the old label output is the
  unchanged branch; the historical specimens are not every possible input.
- CSS rebuilt through the pinned Tailwind tool and was byte-identical: all selected
  classes already belong to Gallery's existing scan. Consumers must still rebuild
  their own selected CSS; Gallery's closure does not prove their scanner coverage.
- `browser-reviewed/` passes 56 checks in four no-script cells: 1440×1000/light and
  390×844/dark, each with an actual-handler recipe and a long-label signed fallback.
  Native Tab/Enter opens the selected Gateway detail, browser Back restores the
  recipe URL, and native export retains exact labels, numbers, units and missingness.
  The wide September rows are 4/1/0; narrow August/API missing values stay missing.
  Links have visible 2px focus outlines, 44px minimum heights and full wrapped text.
  Recipe HTML is 21,499/18,419 bytes, within the predeclared 32KiB cap. Exact document
  and CSS responses finish before assertions; unexpected requests are refused.
  Chrome 153.0.8010.53, Playwright 1.62.1 and Node 26.3.0 are manifest-bound. Cleanup
  observes exit and complete output drain without forced termination.
- The first browser acquisition also passed 56 checks; image review caught stale
  introductory copy. A separately frozen final build/run includes the corrected
  row-detail/Back instruction and Unicode-control fix, using the unchanged harness.

The browser serves exact responses collected through `gallery.New()` from a private
synthetic binary. It proves that returned UI task, not a live deployment, browser
server latency, real data, production authorization or independent app adoption.

### Visual review and remaining work

Four final captures retain clear underlined labels and visible focus in both themes;
400-character labels wrap without ellipsis. Native table scrolling remains necessary
on narrow screens. The missing-state recipe's offscreen reason column becomes very
narrow and makes rows excessively tall. This is a remaining table-density defect,
not a passed craft gate or evidence that links caused it; compare original and
candidate at the same missing-state selection before choosing its repair.
Human A/B, assistive-technology/full contrast checks, actual zoom, comprehensive
performance, consumer delivery and G1/G3/G4/core acceptance remain open.

### Retained identities and replay

All paths below are relative to the evidence root above; JSON records bind exact
executed commands, source overlays/archives, input versions and results.

| Artifact | SHA-256 |
|---|---|
| `red/result.json` | `532b0a30f969abfde68ab41e9d29d076c35f95e1f7422273d825d66ec80e0349` |
| `red-c1/result.json` | `1de9618b9636ccb9aaa23e27c523ed9b756cd0a9cc58695d9760aaf70f1984b7` |
| `regressions/result.json` | `e9b49ba4d71b7d885af4340304cff6182072ce58149d77c8b56c2d6d7cf1c5c4` |
| `regressions-reviewed/result.json` | `88b7f0f10ebfa84c04c2558b33f63fe9e2d88e4f53304637d0d96ced9122a582` |
| `build-reviewed/result.json` | `2182f5256879c3c6cc15db47269ca8978d692660bd3440b3e8fc5c94ee989150` |
| `build-reviewed/responses.json` | `be0b2f0e161d67db29ee4d1b17eb6aa6583bed296052af3f57be0a796a07d0a7` |
| `browser-reviewed-inputs/manifest.json` | `47b529d1261482c011dcdd4b576c8d63fd57c564ac4866cc18b55ee8c14eb21b` |
| `browser-reviewed/report.json` | `d2d3cc669f2c7ff33650b2db3a905f0c7d127c30e2430e22d51dc12d751551df` |

Run the retained `browser-reviewed-inputs/docs/assessments/fixtures/analytics-detail/browser.mjs`
with the exact Node manifest path, `browser-reviewed-inputs/manifest.json` and a new
absolute output directory. It refuses changed input hashes or browser/runtime versions.
The committed [collector](fixtures/analytics-detail/fixture.go.txt) and
[browser task](fixtures/analytics-detail/browser.mjs) describe a fresh acquisition;
do not re-label a new source or installed-browser version as the historical run.
