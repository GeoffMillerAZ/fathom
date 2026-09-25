# Static export browser fixture

This prospective fixture screens a frozen Gallery static export through a generic,
read-only loopback HTTP server mounted at `/reading/`. It does not start Gallery and
does not prove a deployed static host, production caching, broad performance,
accessibility conformance, or consumer acceptance.

## Frozen inputs

The caller supplies four absolute paths:

```text
node probe.mjs EVIDENCE_PARENT SITE_ROOT RUNTIME_JSON CONTRACT_JSON
```

`SITE_ROOT` must contain exactly the eleven files named by `contract.cue`. Its
`manifest.json` must bind the other ten files by byte count and SHA-256, declare
schema version 1, `/reading/`, and exclude only itself. Normal document links remain
relative; the selected 404 documents use absolute `/reading/` recovery links.

`RUNTIME_JSON` has this pinned shape:

```json
{
  "schema_version": "static-export-runtime-v1",
  "playwright": {
    "path": "/absolute/path/to/playwright/index.mjs",
    "sha256": "64 lowercase hex characters",
    "package_json": "/absolute/path/to/playwright/package.json",
    "package_sha256": "64 lowercase hex characters",
    "version": "1.62.1"
  },
  "chrome": {
    "path": "/Applications/Google Chrome.app/Contents/MacOS/Google Chrome",
    "sha256": "64 lowercase hex characters",
    "version": "the exact browser.version() value"
  }
}
```

The launcher validates the contract, manifest, complete site closure, and tool
identities before creating a fresh evidence directory. It snapshots the harness,
tests, contract, runtime configuration, and every site byte before launching the
frozen harness. The repository `prepare.mjs` owns repeatable export/build preparation:

```text
node prepare.mjs EXISTING_PRIVATE_PARENT GO_CACHE MODULE_CACHE
```

Its static-specific CSS input is `apps/gallery/cmd/static-content/input.css`; the
prepared closure records the verified cached Tailwind executable, file-only module
proxy, repeated binaries/sites, and generated contract/runtime inputs. It does not
overlay Gallery CSS. This fixture never derives expected answers from live page content.

## Browser contract

Six JavaScript-disabled, service-worker-blocked, cache-disabled contexts cover 390px
and 1440px at system, light, and dark themes. System emulates a dark OS preference.
Each navigation admits only owned-origin GETs for the selected documents and
`app.css`; response bodies must match the frozen site byte-for-byte. The journey uses
native Tab/Enter and pointer input to exercise skip and fragment focus, ToC links,
independent FAQ disclosures, the FAQ article destination and return, browser Back and
Forward restoration, both resource fragments, page-preserving theme navigation, and
deep-path 404 recovery. Local links must match the contract's same-directory or exact
404-prefix grammar; schemes, queries, escapes, and traversal are refused. It records exact
status, MIME type, payload, timing, focus, size, overflow, theme, and screenshots.

The numeric ceilings are an unthrottled local payload/timing screen only. Every
failure is retained in `receipt.json`. Success additionally requires the owned Chrome
process to exit normally, the local server to close, and its listener to refuse with
`ECONNREFUSED`.

## Preflight only

These checks do not launch a browser:

```sh
go tool cue export docs/assessments/fixtures/static-export/contract.cue
node --check docs/assessments/fixtures/static-export/probe.mjs
node --test docs/assessments/fixtures/static-export/probe_test.mjs
```

Do not run the browser command until the complete prepared closure has been reviewed
and frozen. A failed acquisition is evidence and must not be overwritten or retried
without a reviewed cause and an explicitly bounded repair.
