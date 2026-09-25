# Gallery standards and work inventory

G1/G4 implementation of ADR-0020/0069. Gallery now projects the same embedded
`internal/qualityplan` snapshot as the existing MCP quality tools. There is no second
inventory, additional server, runtime evidence scanner or quality-score calculation.

## Behavior and boundaries

- `/quality`: native GET filters for core, recorded on-demand or all work; an optional
  exact capability ID joins the existing first-party catalog. Catalog pages link here.
- `/quality/{id}`: the task, authored standard/gate statuses, reasons, next steps and
  recorded consumer integration. List links bind the snapshot revision; stale
  selection refuses rather than silently opening a newer plan.
- `/api/quality` and `/api/quality/{id}` share the MCP reader's JSON representation.
  Unknown capabilities, unsupported/duplicate parameters and stale detail requests
  receive explicit errors; known capabilities without work get an honest empty state.
- Existing themes, native navigation/forms and escaped text are retained. References
  are plain records, never endpoints serving local evidence files or licensed assets.
- Rebuild Gallery/CSS to adopt. No running preview, installed MCP host, consumer pin,
  inventory quality status or target certification is upgraded by this change.

## Verification

Gallery package tests passed after the initial implementation. Scoped quality tests
then passed JSON parity across scopes/items, stale/invalid requests, escaped references
and optional metadata/native disclosures. The MCP/Gallery test compares all35
capability joins, four scope inputs and all six work-item details. A private build
embeds fresh CSS through a Go overlay without replacing the dirty working-tree CSS.
This is not a clean-checkout dependency-closure or consumer-delivery proof.

The repaired candidate passes480 focused browser checks with zero failures. The
[maintained probe](fixtures/gallery-quality/README.md) uses isolated local Chrome,
JavaScript disabled, reduced motion, light/dark and320/1440 CSS-pixel widths. It
follows no authored evidence path or external site. Native filter/open/back/disclosure
tasks, status text, query recovery,44px standalone controls and overflow pass;
38 HTML pages and eight list/detail captures are retained. Four captures received
agent visual review: readable hierarchy, clean narrow wrapping and restrained wide
density; no further defects observed within that scope. This is not human acceptance,
full accessibility, browser parity, performance or component certification.

## Retained attempts

`/Volumes/m2ext/appkit-evidence/appkit-gallery-quality.9mQI1e2I/` preserves the first build, withdrawn
before browser testing because native evidence/metadata disclosures were missing.
The replacement `/Volumes/m2ext/appkit-evidence/appkit-gallery-quality.wEVGEXIb/` retains build/test
logs, private CSS/overlay/binary and selected source. Those source bytes were copied
after execution only after matching recorded hashes; timing is explicit in
`receipt-followup.json`. The command ledger was recorded after execution, not a
pre-frozen executed script or full source closure.

Within that replacement root, `gallery-quality-qCb0bA/` retains the first browser
attempt (26 checks, three failures): selectors wrongly treated heading IDs as section
containers. `gallery-quality-rbAHHp/` retains the corrected-selector run (480 checks,
eight failures). Four expose missing visible keyboard focus on the detail back link;
four use a stale known-empty fixture (`content.docs` now has recorded work). The
repair keeps the focus requirement and selects an actually unassigned known
capability for the same empty-state assertion. Neither failed attempt is a pass.

The focus repair and narrow synthetic regressions are retained in
`/Volumes/m2ext/appkit-evidence/appkit-gallery-quality.spQ1NymK/`. This build retained selected source,
configuration, overlay and its command script before execution. The passing browser
run is its `gallery-quality-ghOUY7/` directory. All100 requests were admitted local
GETs, with no rejected request; Gallery and Chrome152.0.7977.84 both exited0 without
a signal. The expected and served CSS hashes match. No running preview was replaced.

| Final artifact | SHA-256 |
|---|---|
| Gallery binary | `001225811396ef0dd2b525cf9d5ab17f4274f7fb0c0036e777ca94888c2b6631` |
| Embedded/served CSS | `0bb39edec5eb03cfe1a79ddf9ca1559399b56b3312e550b5782b115423b19ff8` |
| Build/test receipt | `1da65973e5012fa8359064d913d0f5f914ef26ed313058dfec2a590c2824089b` |
| Browser harness | `f59645f095711aadf88b4f40e5a7d97b065a4b7e32915d95d1509ccde68c8a84` |

Scoped documentation checks are recorded under
`/Volumes/m2ext/appkit-evidence/appkit-quality-gallery-docs.YqwiCiIo/{docs-check,mcp-check}/`:
links/budgets/paired intent guidance and source-MCP document identity. Their scope
does not establish installed host loading, a full intent audit or a released pin.

## Model allocation

Terra owns bounded implementation and build/test preparation; Sol owns the isolated
browser task and representative visual review. Deterministic checks establish JSON
parity and query behavior. Main-thread integration does not duplicate routine visual
evaluation. This records allocation, not measured token savings.
