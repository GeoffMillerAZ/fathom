# ADR-0029: Separate visual identity from scheme and application behavior

- **Status:** Accepted
- **Type:** product
- **Date:** 2026-09-13
- **Extends:** ADR-0017 and ADR-0020

## Context

The owner wants the personal site's design to improve later apps, starting with an
optional theme, and wants app-specific living style guides and safe theme previews.
The owner clarified that the older white-neon/pink/purple Synthwave site is primary;
the newer gray/indigo dashboard should also be retained. The committed older CSS and
untracked Next migration are different references. App-kit's existing Synthwave remains
a third identity. Current-source hashes identify uncommitted inputs; a HEAD alone does
not. No whole-site or third-party component ownership is inferred.

The existing token schema already models color, fonts and radii. Color utilities use
runtime variables, but font/radius literals previously became build-time constants.
Gallery scheme links do not select a named identity. Theme adoption should not require
a renderer migration, copied application pages or a new configuration framework.

## Decision

Add source-traceable `geoff-synthwave` and `geoff-indigo` dark-only presets at L0/L1,
independently of existing Synthwave. Preserve the primary reference's white-neon title
effect as separate opt-in static CSS, never mandatory blur or motion on data/body text.
Document semantic accessibility adaptations and kit-assigned status roles separately
from source values. A declared font family is not a downloaded or verified font.
No personal content, photographs, website dependencies or paid component bodies travel
with the preset. Broader website patterns remain candidates, not promoted components.

Keep named identity distinct from light/dark/system scheme. Use the existing CUE
schema and generator; extend runtime variable indirection to fonts and radii without
introducing a general variant-axis schema. Preserve utility names and default resolved
values. Consumers regenerate selected artifacts and rebuild utilities to gain the new
bindings; no automatic upgrade or consumer mutation follows.

Add explicit named-theme selection to portable vendoring and a Gallery theme workbench
over actual kit specimens. Selection must be bounded and preserve the chosen preview
context. Previewing does not save application preferences or change reference originals.
Theme source, available schemes, font dependencies and evidence gaps stay visible.

App-owned living style guides become a G1/G4 outcome: a selected kit revision and
app token source, real app recipes/states, explicit overrides, and repeatable preview,
reset, validation and reviewed export. Runtime apps do not depend on the guide or MCP.
Start with existing specimen/rendering and artifact contracts. Do not build a universal
visual editor, arbitrary CSS executor or copied fork of every app's component catalog.

## Alternatives considered

- Vendor the entire website: preserves source but brings unrelated identity, content,
  dependencies and uncertain asset rights. Wins only for a separately scoped reference.
- Rename Synthwave to Geoff: silently changes an existing consumer's visual identity.
- Add generic density/layout/motion axes now: no established shared schema or measured
  need. App-owned CSS can express these independently until a concrete contract earns it.
- Recolor components at render time: duplicates palette logic and undermines shared roles.

## Revisit when

- The owner identifies a different deployed/source revision as the intended reference.
- A named-theme switch leaves a used font, radius, color or semantic state unchanged.
- A theme fails contrast or task checks in a real app; correct the narrow role mapping.
- Two app guides duplicate the same missing preview/export contract.
- A website composition has two actual consumers and meets ADR-0017's component bar.
