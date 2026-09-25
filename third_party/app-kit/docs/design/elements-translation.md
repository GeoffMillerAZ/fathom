# Explicit Elements HTML-to-templ translation

[ADR-0040](../adr/0040-explicit-elements-syntax-family.md) adds an opt-in runtime-bound
syntax family alongside the [legacy translator](html-translation.md). It does not
widen the old grammar, change its rule digest, or migrate saved `syntax@` attempts.

## Use

Select an exact private archive/member with `assets-list` and `assets-files`, then:

```sh
go run ./cmd/appkit data assets-translate-elements \
  -id ARCHIVE_ID -path EXACT_MEMBER.html -root PRIVATE_DIRECTORY
```

The command verifies the original archive member and emits one deterministic private
JSON receipt. It writes no files, accepts no `-save`, fetches/installs nothing and
does not compile output or load runtime code. Automation needing precise exit codes
should invoke a built `appkit` binary: 0 translated syntax, 1 unsupported with JSON
and no partial artifact, 2 invalid request/original failure/I/O failure.

The separate `elements-syntax` envelope names `html-elements-to-templ-v1`, source,
rule/toolchain identities, output/source mappings and the runtime requirement.
Every result remains `not-verified`. A successful command is not a runnable app,
behavior pass, accessibility approval or converted-target certification.

## Runtime and grammar boundary

Output is constant-only `package converted`, `Specimen()`, `snippet.templ`. It retains
original classes, literal attributes, comments and text. The grammar adds only
`el-disclosure`, `el-dropdown`, `el-menu`, `el-dialog` and `el-dialog-panel` to the
existing bounded native HTML/basic-SVG vocabulary, with literal menu placement.
Commands require unambiguous local targets of the expected kind. Inline scripts,
events including `oncommand`, styles, expression namespaces, malformed/tree-repaired
markup and unknown custom elements remain unsupported.

The host must deliberately provide the commercial Elements module:

- Package/version: `@tailwindplus/elements` 1.0.22.
- ESM SHA-256: `120ff5b81c8b3bb7e6de13d8b83a92da984c4312aa017b7392c9cb876cd65013`.
- Tarball SHA-256: `fc684f17d960fb60dadfdf9a0b49ba8fd295af4b8fd1715a26cd20b832ea245b`.

No module loader, CSS, font or image is automatically installed. Review app-owned CSP
and source licenses, keep derivatives private, and compile in an app-owned private
module. Supply source-appropriate CSS/assets and test actual routes/forms/authorization.
Hardcoded specimen IDs need deliberate repeated-instance handling; this is not yet
a typed multi-instance component constructor. Datastar morph/lifecycle is a separate
contract, not implied by rendering templ on a Go server.

## Replay, discovery and proof

`TranslateElements` and `VerifyElementsReplay` are internal tooling entrypoints;
apps use the CLI, not imports of kit internals. CUE constrains the separate envelope;
replay rejects changed/rehashed output, runtime, source, diagnostics or toolchain.
The rule fingerprints its implementation plus the unchanged legacy validation helpers.

First-party catalog ID `reference.elements-translator` describes experimental CLI
tooling. Gallery and MCP expose its same metadata without private archives. It does
not advertise `go-web`/`static-web` compatibility for generated components. Existing
conversion list/get tools and Gallery `/conversions` remain legacy-family-only;
there is no persistent Elements store or automatic promotion.

The [source-native reference](../assessments/2026-09-14-elements-source-reference.md)
pins the four pilot originals and their runtime. Separate [original/compiled proof](../assessments/2026-09-14-elements-conversion.md)
records four compiler/DOM replays, 842 scoped Chrome assertions and 34 byte-identical
screenshot pairs. The fixture independently binds each lane to its fragment/wrapper;
rehashing an aliased manifest cannot make it pass. This does not cover other inputs,
Datastar updates, app-owned actions or consumer adoption. Source small controls,
unpinned imagery and full accessibility remain visible gaps.
