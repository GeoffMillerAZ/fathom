# P02 typed private adapter probe

This is a source-specific experiment under ADR-0023, not a certified catalog target.
The generator lives in `internal/designadapt`; the purchased body and all generated
derivatives remain private. Do not build inside `data/design-assets`.

## Contract fixed before browser checks

- Source: the exact frozen P02 archive/member/hash in the conversion pilot.
- Preserve all classes, icons, element/attribute order and surrounding text whitespace.
  Bind named plain-text fields; reuse the identical feature-item structure for 1..12
  bounded items. The default copy is private. Its URL starts empty, never a fake action.
- `NewPricing(Pricing)` validates before output and snapshots the feature slice.
  One native same-window link accepts a local absolute path or explicit HTTPS URL.
  No raw HTML, script, billing/backend, authentication, computed prices or token mapping.
- Fixture destination `/pricing/access` returns an explicit local demonstration page.
  No purchase, external navigation or provider call. Baseline comparison changes only
  the original placeholder href to that same route; the retained source is unchanged.
- Reuse the prepared P02 CSS/font, verifying the old complete document hashes first.
  Those are chosen test conditions, not recovered portal font/CSS provenance.
- Test light/dark, 390/1440 CSS-pixel viewports, baseline and long labels, JS on/off,
  keyboard/pointer navigation, visible focus and no horizontal page overflow.
- Exact screenshot bytes for baseline original/target under identical local conditions;
  zero hidden masks or updated goldens. Separately inspect target images and differences.
- Screening budgets: no app JS; complete document including embedded font <=1 MiB;
  link target >=44 CSS pixels; normal/large text contrast >=4.5/3; no external requests.
  Record failures inherited from the source separately from conversion regressions.
- No claim for actual 200% browser zoom, screen readers, touch hardware, adverse network,
  production task-time, clean consumer installation or integration until separately run.

## Replay

The faithful probe found exact baseline pixels but a 36px source link and long-price
overflow. Preserve that red receipt. An independently reviewed **separate enhanced
variant** may add one `data-appkit-pricing` wrapper and a companion scoped CSS file:
minimum 44x44 link, flex wrapping and `overflow-wrap:anywhere` containment. No colors,
vendor classes, font scale or focus rules change. `NewPricingEnhanced` reuses the
validated snapshot from `NewPricing`; callers explicitly include its companion CSS.
Enhanced checks use actual separately compiled HTML and the same absolute budgets.
Its screenshots are reviewed as a deliberate visual delta, not pixel equality with
the unchanged original. Faithful original/target pixel checks and their source-quality
failures remain separately reported. Neither variant receives general certification.

```sh
go test ./internal/designadapt -count=1
APPKIT_PROBE_CONVERSION_KIT_ROOT=/absolute/app-kit go test ./internal/designadapt -run '^TestAdaptPricingLive$' -count=1 -v
```

The optional `APPKIT_PROBE_PRICING_OUTPUT` must name an existing, empty, real directory
under the system temporary directory. The explicit live test retains eight files:
source, artifact JSON, defaults, faithful/enhanced baseline/long HTML and companion CSS.
Ordinary tests use only first-party synthetic markup and skip paid-source reads.

Browser command (existing isolated Playwright only, no install or signed-in profile):

```sh
node docs/assessments/fixtures/pricing-adapter/browser.mjs /absolute/app-kit /absolute/private-temp-output
node docs/assessments/fixtures/pricing-adapter/browser-enhanced.mjs /absolute/app-kit /absolute/private-temp-output
```

The harness starts an ephemeral loopback-only fixture server, blocks other browser
requests, writes a private temporary report/screenshots and closes its browser/server.
The default `appkit data assets-adapt-pricing [-root PRIVATE_DIRECTORY]` still emits
private derivative JSON. For repeatable delivery, opt into `-out NEW_DIRECTORY` under
an existing real parent. It exclusively creates six 0600 files in a 0700 directory:
`pricing.templ`, `pricing.go`, `pricing.css`, `defaults.json`, `README.md`, `artifact.json`.
No overwrite, install, compilation or catalog change. Partial I/O leaves incomplete
output for inspection; choose a new path, never assume the final JSON proves success.

`-check DIRECTORY` regenerates expectations from the independently verified original.
On macOS/Linux, it rejects missing/extra/modified/special/linked files and mode drift.
Exit 0 = match; 1 = observed drift; 2 = invalid request, unsupported or could-not-check.
Export/check success is silent. Their flags are mutually exclusive; JSON remains default.

Treat the export as immutable input. Copy it into a **private**, app-owned build package
named `converted` before compiling with pinned `go tool templ` v0.3.1020; generated
compiler files must not be added to the checked export. The app owns its module/go.sum,
Tailwind 4 scan inputs and font. app-kit is not a component runtime dependency.
Call `NewPricing(Pricing{...})`, supply a real URL, handle validation before response
output, and render at build time or through Go HTTP. Preserve source license obligations.
For the enhanced variant include `pricing.css` **after** source CSS and explicitly call
`NewPricingEnhanced`. Its wrapper scopes these rules; no global overrides. Faithful
mode retains the source's 36px link and hostile-copy overflow. No checkout/backend is supplied.
MCP guide: `get_kit_decision("pricing-adapter")`, also available as a kit resource.

Passing this bounded probe does not alter the syntax-attempt catalog or grant any
target badge. Full target receipts/discovery and consumer proof remain separate work.

## Clean delivery rehearsal (macOS)

`delivery-proof.mjs` uses the adjacent pinned consumer module/sums and synthetic
`consumer_test.go.txt`. Build an appkit binary to an owned private temporary path,
then run `node delivery-proof.mjs /absolute/app-kit /absolute/appkit-binary /absolute/go-binary /absolute/public-module-zip-cache`.
It creates private temporary outputs, two cold-cache consumers and an owned loopback
denial canary, which it closes. No dependency installs, signed-in profiles or app data.
It requires the retained earlier P02 HTML oracle and all selected public dependency ZIPs;
there is no network fallback. Read [evidence and limitations](../../2026-09-14-pricing-private-delivery.md).
