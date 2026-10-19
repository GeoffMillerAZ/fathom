# json-schema — provenance and divergences

LB-Q-json-schema, design `docs/design/language-breadth.md` §4, §11, §13. A new doc-engine dialect (no
incumbent, no upstream grammar). The decode is `encoding/json` through the doc engine (`"decode": "json"`).

## How a file is claimed

The manifest claims no name. `fusion.jsonSchemaDispatch` (`jsonschemadispatch.go`, asked after
`appFrameworkDispatch` in `resolveExtractor` and `ReingestFile`) claims a `.json` that decodes to a root
mapping and either has a root `$schema` naming `http(s)://json-schema.org/` (the spec's `detect.content_sniff`
prefilters, the root entity's signature is the check, so a nested `$schema` never claims) or a base name ending
`.schema.json`. Every earlier verdict stands: a basename (`package.json`, `cdk.json`), CloudFormation, OpenAPI,
Grafana, ARM. An instance that points `$schema` at schemastore.org is not a schema. `fusion.JSONSchemaContentDialect`
is the same decision for the harness.

## What it models

- **The document** (`JSONSchemaDocument`, `package`): one per claimed file, named by the file, signed by its `$schema`.
  A `$ref` with a file part resolves (`repo-path`) to it.
- **Definitions** (`JSONSchemaDef`, `type_alias`): every entry of the root `$defs` and `definitions`, whatever it
  holds (a boolean schema is one), signed by its `type` when a string.
- **Properties** (`JSONSchemaProperty`, `field`): every entry of the root `properties`.
- **References**: a `$ref` under a definition or property to `#/$defs/<n>`, `#/definitions/<n>` or `#/properties/<n>`
  naming an entry the document declares is a reference to it (first pointer segment, unescaped `~1 ~0` and percent
  encoding). A `$ref` with a file part, anywhere, is a cross-file edge at file granularity.

## Divergences (observations)

- A `$ref` outside every definition and root property (a root `allOf`) has no source: O1351.
- A pointer is named by its first segment, so a deeper path that does not resolve still draws the edge: O1350.
- Nested `$defs` and `properties` below the root are not symbols; `$id`, `$anchor`, `$dynamicRef` and URL references
  are not resolved (the oracle's resolver does not fetch them either).
- Row prose, detection and the epoch: O1352.

## Oracle

`calibration/oracles/json-schema/json_schema_oracle.mjs`, tier A: `@apidevtools/json-schema-ref-parser` 16.0.5 (MIT).
Subjects `calibration/corpus.d/json-schema.yaml`.
