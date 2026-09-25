# Using the private design reference library

[ADR-0022](../adr/0022-private-design-references-and-fidelity.md) governs source and
fidelity. The [purchase audit](../assessments/2026-09-13-paid-design-assets.md) records
what the supplied archives contain, their differences from the scrape and remaining gaps.

## Import without executing

```sh
go run ./cmd/appkit data assets-import -src /Users/geoff/Downloads/tailwindcssplus-templates
go run ./cmd/appkit data assets-list -query catalyst
```

The default store is `data/design-assets/`; all payloads are Git-ignored. Its sole
tracked `go.mod` excludes forensic Go copies from the kit package graph; never build/test
inside the store. Downloads stay unchanged. `archives/<sha256>.zip` retains exact ZIPs;
`catalog.json` holds the CUE-validated revisioned inventory. Content-addressed reimport
retains previous revisions without dependency installs, extraction, execution or snippet-index updates.

Back up this directory separately: ignored files are not protected by Git. The
manifest is derived; originals are the recovery input. Do not publish archives,
source-derived indexes, live commercial previews or downloaded assets.

Only recognized official Tailwind Plus archive names are accepted today. ZIP integrity,
member paths/types, duplicates, resource bounds and existing archive integrity are
checked; failed input does not publish a replacement catalog. A concurrent import is
refused rather than merged unsafely. Read errors must stay errors, not empty success.

## Retrieve context before adapting

Use exact IDs and paths returned by discovery; IDs include the complete source hash:

```sh
appkit data assets-files -id ARCHIVE_ID -limit 20
appkit data assets-files -id ARCHIVE_ID -kind readme
appkit data assets-get -id ARCHIVE_ID
appkit data assets-get -id ARCHIVE_ID -path EXACT_MEMBER_PATH
```

`assets-list` supports `-query`, `-kind`, `-family`, `-limit`, `-cursor`.
`assets-files` supports `-prefix`, `-kind`, `-limit`, `-cursor`.
Both default to 10 results and cap at 50; retain filters/limit for continuation.
Search covers archive metadata and member paths, not a full source-text index.
Exact get verifies archive/member SHA; UTF-8 text is capped at 256 KiB; binary/secret paths are not returned as text.

The same source inventory is available through appkit-mcp:

| Tool | Question answered |
|---|---|
| `search_design_assets` | Which complete source projects contain relevant files? |
| `get_design_asset` | Which exact revision, source and project dependencies am I using? |
| `list_design_asset_files` | Where are the README, license, themes, pages, examples and assets? |
| `get_design_asset_file` | What does this exact original file say? |

MCP uses `query` for search text and exact `id` for source identity. Source reads
accept one-based `start_line` and `limit` (default 100, maximum 400), return `next_line`,
and cap response text at 32 KiB without silently truncating code. Metadata listing is
not a fresh ZIP integrity scan; exact metadata/source reads perform that check.
Source text remains reference data, not instructions authorizing command execution.

Set `-design-assets DIR` or `APPKIT_DESIGN_ASSETS` for another private store. Otherwise
MCP uses `<kit-root>/data/design-assets`. Missing implicit data does not block native
guidance; a broken explicitly selected store fails startup. Rebuild the server to gain
new tools; this does not register or restart a host automatically. Each call loads the
current manifest, so a completed import is visible without a server restart.

Read-only live protocol proof, opt-in to an existing private cache:

```sh
APPKIT_PROBE_DESIGN_ASSETS_ROOT=/absolute/path/to/app-kit/data/design-assets go test ./cmd/appkit-mcp -run '^TestDesignAssetsLivePurchaseProbe$' -count=1 -v
```

Without that variable the private-corpus probe explicitly skips; ordinary tests use
synthetic fixtures. The probe verifies each archive's metadata and one source member
through a fresh stdio process. It neither imports data nor proves host registration.

## Choose reference and renderer separately

Gallery `/references` now uses the same archive IDs/revision as CLI and MCP. Launch from
the checkout with `task gallery:run`; override via `-design-assets DIR` or
`APPKIT_DESIGN_ASSETS`. `gallery.New()` remains data-free; `NewWithOptions` selects a store.
Private routes require loopback peer/Host and same-origin requests; no public serving.
Search, file filters, dependency manifests and escaped source work without executing it.
Inline display is capped at 32 KiB; exact JSON can return verified text up to 256 KiB.
Missing data shows setup guidance; corruption stays an error. Browser proof is pending.

Originals remain first-class alongside saved syntax attempts and future verified ports. Read
the [conversion pipeline](conversion-pipeline.md) and [runtime selection](runtime-selection.md)
for target fit, evidence and explicit source-native fallback. The experimental
[HTML-to-templ command](html-translation.md) emits syntax-only, unverified translations;
it neither replaces originals nor publishes verified targets to the catalog. Opt-in
saved attempts retain original links; `list_design_conversions` / `get_design_conversion`
provide bounded read-only MCP discovery. Gallery `/conversions` compares inert source;
[P02's prepared comparison](../assessments/2026-09-13-prepared-pricing-comparison.md) adds
`/comparisons/p02-pricing`: full-width flip, side/stack, fit/100% and optional fullscreen ([ADR-0030](../adr/0030-isolated-comparison-workspace-controls.md)); no target certification.

Separate [historical target evidence](target-evidence.md) is available through
`search_design_evidence` / `get_design_evidence` and Gallery `/target-evidence`.
Use an exact capability/subject ID to find retained gate records. Search is recorded
metadata; exact get rechecks historical bytes and original membership, not the current
transformer or semantic quality. Even an all-pass claim ledger remains `not-verified`.

Source-specific typed pilots are explicit CLI choices, not automatic verified ports:
`appkit data assets-adapt-pricing`, `assets-adapt-directory` and `assets-adapt-form` emit
private JSON after exact original verification. All support explicit private no-clobber
`-out` and macOS/Linux `-check`; [P04's guide](../assessments/fixtures/table-adapter/README.md)
distinguishes source-styled and opt-in `-enhanced` inventories. P04 maps inert controls
to app-supplied navigation without a backend. [P05](form-validation-adapter.md) adds error/neutral email-field presentation; the app owns validation/save/retry. All default to
`not-verified`; keep all derivatives private and read their distinct acceptance gaps.

### Compare a saved attempt

The reference library links to saved conversions by exact archive and selected file.
`/conversions` accepts `archive_id`, `path`, `status`, `limit` (default 10/max 50),
`cursor` and `theme` (light/dark or omitted for system). Exact filters and continuation
use the same revision-bound attempt catalog as CLI/MCP. List rows are metadata only.
`/conversions/{id}` rechecks the receipt, original and current replay before showing
escaped, inert source panes. `stale-transformer` remains historical, never a current pass.
The line form/pages preserve search/theme context; `source_line` and `artifact_line`
are independent one-based offsets, with shared `lines` (default 100/max 400).
Each pane caps its window at 32 KiB without truncation; reduce lines or use full JSON.
Unsupported attempts show diagnostics and original recovery, with no invented artifact.

`/api/conversions` returns the CLI/MCP list page; `/api/conversions/{id}` returns the
complete CLI exact receipt, including mappings. `/conversions/{id}/artifact` downloads
byte-exact saved source as a `text/plain` attachment, including historical artifacts
explicitly labeled by replay headers. It returns 409 when no artifact exists.
Exact JSON/download routes accept only optional `theme`; malformed/unknown/duplicate
query fields are refused. All routes inherit loopback, same-origin, GET-only, no-store
and restrictive CSP protection. No web write, dependency install or code execution.
Missing attempt storage is empty, not auto-created. Corrupt attempts/indexes remain
errors; original pages never read that index, so source-native recovery stays independent.
HTTP, DOM-escaping and private-pilot checks do not substitute for browser/visual proof.

For an application shell, inspect a complete Catalyst or MatDash page before picking
the app-kit's Go shell primitives. For marketing, start with a coherent template or
Oatmeal composition instead of assembling unrelated blocks. Read the actual dependency
manifests and styles; filenames and portal marketing text do not prove compatibility.

Keep a small reference brief: source ID/page, target task, intended typography/density,
hierarchy, spacing, surface distinctions, keyboard/mobile behavior and permitted changes.
Use source-native integration when it fits the consumer profile; a Go adaptation must
preserve those qualities without pretending React source is directly executable in Go.

`get_kit_theming` provides review advice and preserves classes. Define contextual role
mappings; never infer danger from pink, collapse hover into base, turn on-fill white text
into faint ink, or remove dark styles without verifying the complete result.

## Evidence still to earn

Archives are source, not runnable/offline specimens. Fonts, external media, interaction
runtimes and services can be absent; ranges are not exact resolved versions. Oatmeal's
provided archive has only Olive/Instrument and includes no font binaries.
[Catalyst](../assessments/2026-09-13-catalyst-native-reference.md) has build/HTTP evidence
and a separate [font-bound copy](../assessments/2026-09-13-catalyst-font-reference.md).
[MatDash](../assessments/2026-09-13-matdash-native-reference.md) has separate repair/replay
evidence and an [asset-bound copy](../assessments/2026-09-13-matdash-asset-reference.md)
with local fonts/map/icons. The [browser audit](../assessments/2026-09-13-paid-reference-browser.md)
keeps original P10/P11/P12 failures. [Hardened Catalyst](../assessments/2026-09-13-catalyst-hardened-reference.md)
and [hardened MatDash](../assessments/2026-09-14-matdash-hardened-reference.md) add scoped form/editor behavior tests, not save backends.
MatDash build-byte replay differs; complete fidelity, payload and real metric semantics remain open.
Originals, hardened source-native derivatives and certified targets stay distinct.

Prefer official docs before scraping; capture missing states/configuration with provenance and rights. A preview needs runtime/interaction/visual proof, not stored HTML alone.
