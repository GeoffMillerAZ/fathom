# Catalyst font-bound reference — 2026-09-13

## Result and boundary

A separate P10 Catalyst project now builds with a locally pinned copy of its original
Inter stylesheet and complete font-file closure. The archived source and the earlier
[source-native reference](2026-09-13-catalyst-native-reference.md) remain unchanged.
This removes a mutable font-delivery input from the prepared comparison, not a renderer,
font-family or visual-design change. Font preparation itself is **not a visual baseline**;
later scoped browser evidence is linked below.

The source-native React profile remains a valid choice alongside a future verified
Go adaptation. Neither font preparation nor compilation certifies a conversion. No
Gallery/MCP route executes this project; no Node/font dependency is added to kit-core.

## Exact asset snapshot

The selected root layout originally loads the [official Inter stylesheet](https://rsms.me/inter/).
Its bytes were fetched from `https://rsms.me/inter/inter.css`, SHA-256
`46d01c7807f64a24c1b2853b756ef15f3a2facdf4a9f066eaf5d39c0c9935441`.
The file contains 40 font-face declarations referring to 38 unique WOFF2 files:
18 static Inter, 18 static InterDisplay and two variable files. Legacy family aliases
reuse the variable files. Every face retains `font-display: swap`.

Critically, Catalyst's unchanged theme selects **Inter**, with feature `cv11`, rather
than InterVariable. Keeping only two variable files or renaming the family would change
the source baseline. This rehearsal retains the complete stylesheet without rewriting
its declarations, feature aliases or relative `font-files/*.woff2?v=4.1` URLs.
The query version is not an integrity guarantee; each body has its own SHA-256.

The [asset manifest](fixtures/catalyst-native/font-assets.json) records exact source
URLs, sizes and hashes. The CSS, 38 fonts and [upstream license](https://raw.githubusercontent.com/rsms/inter/v4.1/LICENSE.txt)
total 40 files / 4,930,333 bytes; fonts alone total 4,914,456 bytes. All font downloads
returned HTTP 200 and `font/woff2`, without redirects. Each passed byte-length/hash and
WOFF2 magic/declared-length checks. Those are not font decoding or glyph-rendering tests.
An optional local `fc-scan` attempt returned exit 1 without metadata; it earns no proof.

Downloads used bounded HTTPS requests to the selected upstream origin, four at a time,
with 30-second deadlines, 2 MiB per-font and 20 MiB aggregate ceilings. The linked
license has a separate 64 KiB ceiling. No browser credentials or authenticated portal
content was sent to these endpoints. Upstream reporting/cache headers are not copied
into the local server; only the reviewed asset bodies and license are retained.

The durable private copy is under ignored
`data/design-assets/reference-assets/inter/<stylesheet-sha256>/`, separate from ZIP
archives and their catalog. It includes the manifest and license. Back it up separately;
Git does not protect ignored assets. Missing/corrupt bytes require explicit recovery,
not silent redownload, fallback-font substitution or replacement of a comparison baseline.

## Controlled project binding

The fresh prepared root is
`/tmp/appkit-catalyst-fonts.0EFBii/project/catalyst-ui-kit/demo/typescript`.
It comes from the same exact archive and TypeScript prefix as the earlier reference.
Its public directory contains the 40-file bundle at
`/__appkit_reference_fonts/<stylesheet-sha256>/`.

The sole authored source change replaces the external stylesheet link with that local
hashed URL and removes its now-unused external preconnect. It leaves components, theme,
package manifest and lockfile alone. Next also regenerates `next-env.d.ts`, as before.
An independent comparison checked all 75 archived regular members: 73 still match,
and only those two files differ. Generated UI CSS remains byte-identical to the earlier
native build. The [receipt](fixtures/catalyst-native/font-receipt.json) records identities,
commands and outputs; the original build receipts are not rewritten.
Do not replace the original layout hash in the earlier fixture to admit this variant.
The original and font-bound project identities must remain distinguishable.

The third isolated project installed 383 packages using `npm ci --offline` from the
previous task-local cache, with install scripts, audit, funding and update notices off.
It then passed the same minimal-environment production build on Node 26.3.0 / Next
16.2.6. Successful cache replay does not establish an independently archived dependency
supply, full egress isolation or offline browser operation.

## Recheck explicitly

Keep all private assets and installed dependencies outside tracked first-party source.
Prepare only the reviewed project in a fresh directory; never extract over a worktree.
Copy the exact asset bundle, make the declared layout binding, and build before testing.
The following commands do not install or download anything:

```sh
node docs/assessments/fixtures/catalyst-native/font-reference.mjs /absolute/prepared/font-bound-project
APPKIT_CATALYST_FONT_REFERENCE_ROOT=/absolute/prepared/font-bound-project NEXT_TELEMETRY_DISABLED=1 node --test docs/assessments/fixtures/catalyst-native/font-reference.test.mjs
```

The verifier and HTTP evidence are deliberately separate from browser fidelity. The
existing guarded launcher is reused unchanged, with an owned ephemeral listener that
the test closes. Do not treat this opt-in fixture as a general vendor-project executor.

## Delivery checks earned

The new opt-in suite passes **25 tests, zero failures/skips**. It pins the verifier's
metadata before parsing paths, checks five selected source identities (package, lock,
P10 page, bound layout and theme), and requires the exact 40-file public bundle.
It rejects missing/changed CSS or fonts, invalid WOFF2 headers, changed selected source,
extra files, symlinks and altered metadata. Canaries modify only their own temporary
copies. Missing project selection is a failure, not a skipped private-corpus success.
A deterministic file-to-FIFO race initially reproduced a blocked open in a child;
nonblocking descriptor opening fixes it, and the bounded regression now passes.

The HTTP test checks `/settings`, all 19 distinct script/link/img resource URLs found
in its fixed server-rendered HTML, and all 40 bundle files. Each font URL retains the
original query string and returns the expected size/hash/MIME; CSS and license bytes
also match. No remote font URL remains in that HTML, and no upstream reporting headers
are served. Parser canaries exclude navigation links, comments and inline-script text;
this is not a browser-observed request graph or proof of all possible application egress.
The owned listener closes and refuses a later connection. The unchanged original
reference suite also passes its 42 tests; original-source recovery remains independent.

## Remaining gate

At font preparation, still required: browser font loading/fallback checks, hydration, glyph appearance,
keyboard/pointer/focus, light/dark at 390/1440 widths, zoom and reviewed comparison
against the intended Go composition. CSS/resource delivery alone does not prove these.
At that point all twelve pilot cases had **zero captured source-native visual baselines**.
The Chrome integration remained unavailable; no alternate browser was used without
approval. MatDash's separate dependency, font, icon and external-data closure remains
unfinished. No consumer was migrated, original hidden or compatibility badge awarded.

## Later browser follow-up

The owner-approved isolated [browser audit](2026-09-13-paid-reference-browser.md) now
captures P10 at 390/1440 widths in both themes, observes actual Inter font loading and
country keyboard/focus behavior, and retains failing bio/dependent-region/reset checks.
Its repo-owned optional probe uses the prepared font-bound project, not the signed-in
portal. Complete accessibility, zoom, save/recovery and converted-target gates remain open.
