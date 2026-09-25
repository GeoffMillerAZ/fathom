# Paid design assets — 2026-09-13

## Conclusion and evidence boundary

The owner supplied the main current Tailwind Plus source downloads: 13 named
templates/kits, Catalyst, and three Tailwind 4 UI-block bundles. All 17 ZIPs in
`/Users/geoff/Downloads/tailwindcssplus-templates` pass CRC checks. This is not a
claim that every online configuration, document, dependency or preview is archived.
The original downloads were inspected without extracting or executing their code.
This audit preceded import; the implementation follow-up below records its later completion.

App-kit is the owner's private personal infrastructure. Preserve vendor provenance
and access restrictions; private use is not a claim that third-party assets have
unrestricted rights. Do not place purchased source in public/distributable outputs.

Evidence: authenticated Chrome portal inspection, bounded local archive inspection,
the existing component verifier, source/index audit. No consumer pixel regression
was attributed to a specific adapter without a before/after comparison.

## Coverage

| Product | Present | Important boundary |
|---|---|---|
| Named templates | All 13 currently listed on the [portal](https://tailwindcss.com/plus/templates) | Full projects are not equivalent to the UI-block Page Examples |
| [Catalyst](https://tailwindcss.com/plus/ui-kit) | 27 core components in both JavaScript and TypeScript; both Next.js demos, manifests, locks, images, README and license files | Dependencies still need deliberate installation; source presence is not a running preview |
| [Oatmeal](https://tailwindcss.com/plus/kits/oatmeal) | 13 root pages, 38 sections, 16 elements, 112 icons and demo assets | Olive/Instrument configuration only; not the other palette/font combinations; font URLs, not font binaries |
| UI-block bundles | Application UI, Marketing and Ecommerce Tailwind 4 ZIPs | Do not replace the existing multi-version corpus without comparing variant coverage |

Oatmeal offers four palette choices and three font combinations. Capture their
configuration differences when useful; twelve duplicated project trees are not a
necessary storage model. The missing combinations do not mean its base components
or pages are absent.

## What the original scrape did well

The raw dump records 657 components: Application UI 364, Marketing 179, Ecommerce
114. The current portal displays the same category totals. All 10,452 captured
HTML/React/Vue and Tailwind 3/4 code variants passed the existing byte-identity
verification against the index. Six Logo Cloud variant omissions are already absent
from the raw capture, not an indexer loss; live availability was not checked.

The connected TailwindPlus MCP identifies its snapshot as February 2, 2026. That is
an age observation, not proof that component names or source are now missing.
Matching category totals likewise do not prove byte-for-byte current source.

Direct comparison of the new UI-block ZIPs resolves part of that uncertainty:
their 1,971 files cover 657 IDs in HTML/React/Vue. Of these, 1,941 exactly match
captured Tailwind 4 system-mode source (Ecommerce: none); 24 files change eight
existing components, and six supply the previously absent React/system Logo Cloud
variants. The ZIPs contain source only, not supporting documentation/assets/runtime.
Import these as an additional versioned source, retaining the scrape's Tailwind 3,
separate light/dark variants and previews. The snippet index has not been refreshed;
the archives are now a separately queryable source (implementation follow-up below).

## Where purchased value was being lost

1. The existing corpus did not include Catalyst or the separately named templates.
2. Stored previews preserve HTML, but depend on externally hosted scripts, fonts and
   portal-relative assets. They are not reproducible offline specimens. Some refer
   to `@tailwindplus/elements@insiders`, not an immutable runtime version.
3. Search covers names/taxonomy/variant axes and a bounded class list, not complete
   project dependencies, documentation, assets or design rationale.
4. The theming helper collapses different shades, can map white text to faint ink,
   erase base/hover distinction and discard dark color variants without context.
   Raw retrieval remains faithful; mechanical adaptation is not fidelity-preserving.
5. MatDash has eight local package trees and separate documentation. Its 762-record
   main-package index omits supporting contexts, usage examples, styles, assets,
   manifests and alternate-layout configuration. More files exist than MCP exposes.

## Additional material worth capturing

- Use official downloads for source and dependency manifests. Retain original ZIPs,
  checksums and versioned extraction; keep source data private and separately backed up.
- Ingest official documentation directly. Elements already provides
  [LLM-readable Markdown](https://tailwindcss.com/plus/ui-blocks/documentation/llms.txt),
  so a custom HTML scraper is unnecessary for that material.
- Record selected complete-page references at wide/narrow widths and available
  light/dark states, including opened controls and keyboard behavior. Pair captures
  with source revision, viewport, fonts, dependencies and known limitations.
- Resolve and pin the actual interaction runtime. [Elements](https://tailwindcss.com/plus/ui-blocks/documentation/elements)
  supplies framework-independent HTML interactions; test its fit with Go/templ,
  dynamic updates and the adopted security policy before claiming compatibility.
- Track vendor changelogs and source hashes with explicit refreshes. Preserve useful
  older variants rather than silently overwriting the only known working snapshot.

The [assets documentation](https://tailwindcss.com/plus/ui-blocks/documentation/assets)
also offers a discontinued Figma file, last updated July 14, 2021. It is optional
historical reference, not a missing modern design system or an adoption prerequisite.

## Recommended next move, not shipped behavior

Extend the existing private catalog/MCP and Gallery with purchased-project discovery,
documentation, dependency context and reference previews. Keep capture targeted and
incremental; do not introduce another server merely for each vendor or a second raw
component corpus without a demonstrated gap.

Separate **visual-reference selection** from **implementation selection**: a native
primitive may be the right renderer while a complete Catalyst or MatDash page sets
the composition standard. Preserve hierarchy, typography, spacing, surface depth,
interaction and dark-mode distinctions during adaptation. Review the source reference
beside the adapted result; more snippets alone cannot establish better UI/UX.

Existing authority: [ADR-0020](../adr/0020-shared-catalog-and-adoption-evidence.md),
[experience system](../design/experience-system.md), invariants I1/I8/I12/I13/I16.
No runtime migration, global registration or automatic authenticated crawler is
authorized or claimed by this recommendation alone.

## Implementation follow-up — same day

Imported all 17 archives into ignored `data/design-assets/`: 129,050,689 source bytes,
4,304 inventoried files and 27 project manifests. Catalog revision:
`7c7c0ab69b86349f1bb8d1df865f7cf8eb4910542c6fe8340252c6593facaab6`.
Independent SHA checks passed for all original Downloads files and all stored ZIPs;
reimport returned the same catalog revision. No source was extracted or executed.

`appkit data assets-*` and four design-asset MCP tools now provide bounded discovery,
project/dependency context and verified text reads. Real stdio protocol fixtures pass;
this is not proof that an existing Claude/Codex host has restarted onto the new binary.
The [reference guide](../design/private-references.md) is served as `design-references`.
Theming now preserves input and gives review notes; its MCP preservation test passes.
Gallery commercial project browsing was added in the continuation below. Runnable
references and visual comparisons remain work; source availability is not UI quality proof.

The opt-in `TestDesignAssetsLivePurchaseProbe` then passed against this real cache:
17/17 archives verified, dependency metadata matched original manifests, and one
README/source per archive returned byte-identically through the actual stdio server.
53 tool calls completed in a 14.37-second end-to-end probe (including local comparison
work). Observed maxima: discovery 121 ms; exact metadata 438 ms; file listing 387 ms;
source read 369 ms. Largest response was 4,935 bytes. This is one local macOS run,
not a fleet latency guarantee, host-registration proof or exhaustive source-read test.

## Gallery continuation — same day

`/references` now offers archive search, project dependencies, paged file filters and
escaped original text from the same library. HTML and JSON routes are read-only and
require loopback peer/Host plus same-origin access; tests reject forged host/origin,
remote peers, corrupted stores, ambiguous queries, unsafe source and oversized reads.
Inline text above 32 KiB gets an explicit complete-JSON/CLI path, not truncated source.

Full maintainer checks, Gallery/MCP race tests and the document identity checks passed.
The live Gallery returned Catalyst's exact shared ID/revision; its dark-theme README
page returned HTTP 200, 26,005 bytes in 0.362 seconds in one local request. A cross-origin
probe returned 403. These are HTTP observations, not rendered visual or keyboard proof.
Chrome remained unavailable after approved fresh-window recovery; browser review is pending.
