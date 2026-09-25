# CSS setup: declared sources, not accidental checkout contents

G4 / INC-9 follow-up. Fathom's [stat/build rehearsal](2026-09-14-stat-reflow.md)
showed that implicit Tailwind scanning can produce different CSS from the same selected
sources when unrelated checkout contents differ. The served MCP setup example still
used an implicit import. Its existing tests proved generation flags, source-directory
preflight and protocol parity, but did not compile that exact CSS example.

## Repair and proof contract

`cssSetupInputCSS` now owns the primary example returned by `get_kit_css_setup`.
The repair uses `@import "tailwindcss" source(none)` plus explicit app and
resolved kit source selections. The optional motion example follows the same rule.
Consumers using other markup languages must explicitly select their own paths; source
closure is not permission to omit a renderer or restore ambient scanning to hide it.

The executable source-closure test compiles the exact served example with the
checksum-pinned standalone compiler. Two isolated synthetic consumer roots must emit
identical bytes despite unrelated utility-bearing content in one root. App and selected
kit classes must exist; changing an included source must change its output. An omitted
selected source must be detected by required-selector checks even if the compiler
returns zero. A guard-disabled variant must expose the contamination.

## Observed results

The old implicit import fails the new test: `notes/sample.html` adds
`.min-w-\[1937px\]` and changes the output. The repaired example passes the unchanged
test. The omitted-source case calls the same required-selector oracle as the valid
baseline and reports exactly `.min-h-\[41rem\]` missing despite compiler exit 0.

| Actual compiler case | Bytes | CSS SHA256 |
|---|---:|---|
| Clean root / unrelated-file root, closed scanning | 8,333 each | `3fa50fe03e37a3236e224333bc3215ee288d1780b9c54874b721a2d8579ed596` |
| Added allowed Go source | 8,358 | `2824b68f2e1254ef6615627ca6157da5ce74f811c22d180f0f548acd149dab99` |
| Selected-module fragment removed | 8,324 | `202961b4323c7ac2a76da21d6bd576419f20959ec6761dd237688be6f0f0a031` |
| Automatic scanning restored / original contaminated baseline | 8,368 | `6c36a45904c5f7cbacad362dfc09ceab1ec1270cb9e52289d14b3cc2180dcefc` |

The cached macOS arm64 Tailwind v4.3.0 executable is verified by `tailwind.Resolve`
against the CUE pin on this run. Normal execution may download on a cache miss;
`-short` explicitly skips compiler proof and is not a substitute for this result.
No network-denial or empty-module-cache provisioning claim follows.

Verified: root `GOPROXY=off go test ./...`, focused CSS-setup race tests, MCP package
vet, real generation/source-preflight failure tests and exact in-memory MCP text/JSON
parity pass. The new proof is [TestCSSSetupGuideSourceClosure](../../cmd/appkit-mcp/setup_scan_test.go),
file SHA256 `a873974b5fba33edebeaed5a6c3d247966097c02e0b3604c62103b32477404aa`.
Fixtures use private temporary directories and are removed by Go's test cleanup;
the source and printed hashes make the selected experiment replayable:

```sh
GOPROXY=off go test -race ./cmd/appkit-mcp -run '^TestCSSSetupGuide' -count=1 -v
```

## Scope and migration

This changes guidance and its executable regression, not a consumer's build files,
tokens, dependencies, mirror or deployment. There is no MCP name/schema or public Go
API change, new runtime, catalog promotion or completed G4 claim. Existing servers need
a reviewed rebuild/restart to receive this compiled guide; no global configuration or
running user-owned server is changed here.

For an existing app, inventory every consumed app/module source before disabling
implicit scanning. Compare required selectors, served bytes and representative browser
tasks against the chosen baseline; do not bless a smaller output merely because it
compiled. Match the pinned generator to selected font/radius aliases and preserve the
app-owned palette. Build a generator in its own pinned module context when the CLI's
indexing dependencies would otherwise spill into the consumer module.

This fixture establishes source-closure behavior for its selected Go/templ layout, not
all Tailwind syntax, malicious filesystem isolation, live host adoption, graphical
fidelity or arbitrary clean-consumer distribution. Fathom's actual stat/emitter pin
upgrade remains subject to the outstanding scope approval.
