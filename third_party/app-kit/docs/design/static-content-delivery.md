# Static reading delivery

[ADR-0071](../adr/0071-deliver-a-bounded-static-reading-reference.md) adds an
experimental build-only export of the Gallery's first-party reading/FAQ composition.
Visitors need HTML and CSS only: no Go server, JavaScript, hydration or remote fonts.
This is a complete bounded example, not a generic site generator or certified profile.

Agent discovery: `search_capabilities(query="static content")` routes through the
existing `content.docs` primitive to this separate example. Read this whole guide
with `get_kit_decision("static-content-delivery")` or `kit://static-content-delivery`.
The primitive's Go-web profile and maturity do not certify this export or its host.

## Build and inspect

From app-kit, compile CSS with the existing pinned toolchain, then export to a new
absolute directory. Replace these example paths with an existing private parent;
neither output path should contain a symlink. The exporter never overwrites a directory.

```sh
go run ./cmd/appkit css -i apps/gallery/cmd/static-content/input.css -o /absolute/private/app.css --minify
go run ./apps/gallery/cmd/static-content \
  -css /absolute/private/app.css -out /absolute/private/reading \
  -base-path /reading/
```

The CSS command writes its selected output; do not point it at an unrelated existing
file. The exporter requires a regular non-symlink trusted CSS input of 1–131072 bytes.
It preserves those bytes; it does not sanitize arbitrary CSS or promise that a custom
stylesheet contains the required utilities. Inspect every selected theme after changes.
The provided input disables automatic repository scanning and scans only the reading
templates and their component sources, keeping unrelated Gallery styles out of delivery.
Parse/argument failures exit 2; accepted-invocation file/export/output failures exit 1.

The directory contains `index.html`, `profile.html` and `404.html`, each with explicit
`-light.html` and `-dark.html` variants; system pages follow the browser preference.
One `app.css` serves all pages. `manifest.json` lists each other file's exact bytes and
SHA-256 plus the selected hosting prefix. Identical inputs produce identical files.
The manifest is written last. An I/O failure may leave partial output: retain it for
diagnosis and choose another fresh destination; do not treat it as a complete build.

Only this example's first-party content ships. The FAQ uses native disclosures and
real article/fragment destinations. Theme links preserve the page. The shared body
templates also serve Gallery, but the export excludes Gallery navigation, catalog
links, state forms and other server assumptions. Licensed templates remain references,
not hidden static-runtime dependencies.

## Configure the static host

Mount the directory at the exact prefix supplied to `-base-path` (default `/`).
Ordinary page links are relative. Error-page asset and recovery links use the prefix
because the same 404 document may be served at an arbitrarily deep missing URL.
Re-export when changing the mount point; moving the files is not enough for 404 recovery.

Serve `.html` as `text/html` and `.css` as `text/css`. Configure unknown routes to use
the appropriate 404 document **with HTTP 404**, not 200. Directly opening `404.html`
only checks its appearance, not the host's missing-route behavior. Unknown routes
default to system appearance unless the host explicitly selects another error variant;
there is no cookie/client store to remember a prior theme selection.

Generated meta CSP denies scripts and nonlocal styles. Hosts still own HTTP security
headers, TLS, cache/revalidation policy and status handling. Filenames are not content
hashed: do not give mutable `app.css` an unversioned immutable cache lifetime. Serve an
immutable versioned directory or use revalidation. The private exporter creates 0600
files in a 0700 directory; deliberately configure destination permissions for the host,
not by broadening the source workspace. No hosting or permission changes are automatic.

## Adopt and contribute

Use the reading model, semantic tokens, native ToC/FAQ patterns and task checks as a
reference for an app-owned content build. Use a proven publishing tool when a real
consumer needs a CMS, localization or a large content pipeline; the kit does not supply
one. Declare canonical URLs, metadata, sitemap/search and asset policy for that product.
Do not ship a contact/account form without its actual backend and failure behavior.

Follow [the JIT workflow](just-in-time-conversion.md): select a real reading task,
inspect the original and chosen target, retain typed configuration, and test content,
navigation, theme, keyboard/narrow/zoom states, error recovery and byte budgets.
Return a broadly reusable fix with its reproducer through the
[contribution loop](contribution-loop.md); keep product content and publishing policy
app-owned. Record applicable gates in `core.static-content`, without promoting the
whole catalog from one example's result.

The [maintained local fixture](../assessments/fixtures/static-export/README.md) tests
actual exported bytes using a read-only generic static host. It is neither Gallery
runtime proof nor an authorized consumer/deployment change. Full accessibility,
broader performance and selected-consumer acceptance remain separate gates.

The [separate cache-cold replay](../assessments/2026-09-22-static-cold-replay.md) now
produces identical binaries and all eleven site files from two empty Go caches with
network, source checkout and shared caches denied. It uses an exact private source
bundle and small frozen file-proxy closure; the earlier inventory/checker failures
remain retained. This is selected local Go1.26.3/macOS-arm64/CGO-disabled build proof,
not independent-host or consumer-hosting acceptance. Review a new closure when source
or tools change; existing browser receipts retain only their original scope.
