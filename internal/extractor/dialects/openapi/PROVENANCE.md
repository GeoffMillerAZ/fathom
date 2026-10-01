# openapi — provenance and divergences

LB-Q-openapi, design `docs/design/language-breadth.md` §4, §11, §13. A doc-engine dialect and the port of
`openapiextract.go` (retired: lang-parity read the dialect at or above the incumbent on every metric, and
the dialect reads L3). The decode is `gopkg.in/yaml.v3` through the doc engine, for `.json` documents too
(YAML reads JSON). No `tags.scm`, no upstream grammar.

## How a file is claimed: by content, never by name

The manifest claims no basename, glob or extension. A YAML document is claimed by fusion's classifier on a
top-level `openapi:` or `swagger:` key (`openapiTopRe`, unchanged from the incumbent), and a JSON one by
`resolveExtractor` when `doc.json`'s `detect.content_sniff` matches and the file decodes with a document
entity at its root (`openAPIJSON`: a JSON file that only has a nested `"openapi"` key, a schema, an
example or a package manifest keeps the JSON extractor). `fusion.Pipeline.ContentDialect` is the same
decision, which the harness measures on, and `ReingestFile` makes it too (a YAML document had been read as
generic YAML by a one-file refresh).

## What it models

- **The document** (`OpenAPIDocument`, canonical `package`): one entity per file that has an `openapi` or
  `swagger` key and an `info`, named by the file (`$file`), at its root mapping's first line, signed by its
  version. A file with an `openapi:` key and no `info` (Redocly's `redocly.yaml`, which the classifier's
  regexp claims because its `\s*\S` crosses the newline of `openapi:` followed by a mapping) yields nothing,
  as the incumbent's read of it did. It is what a `$ref` with a file part resolves to.
- **Operations** (`OpenAPIOperation`, `route`): a `get`, `put`, `post`, `delete`, `options`, `head`, `patch`
  or `trace` mapping under a path item of `paths` or `webhooks`, named `METHOD /path` (the incumbent's
  name), signed by its `operationId`, else its `summary`.
- **Schemas** (`OpenAPISchema`, `type_alias`): every entry of `components.schemas` (3.x) and `definitions`
  (2.0), whatever it holds (a 3.1 boolean schema is one).
- **Parameters** (`OpenAPIParameter`, `variable_decl`), **responses** (`OpenAPIResponse`, `message`),
  **request bodies** (`OpenAPIRequestBody`, `message`), **headers** (`OpenAPIHeader`, `key`) and **security
  schemes** (`OpenAPISecurityScheme`, `policy`): the mapping entries of `components.parameters`,
  `components.responses`, `components.requestBodies`, `components.headers`, `components.securitySchemes`,
  and 2.0's root `parameters`, `responses` and `securityDefinitions`.
- **References** (`Calls` on an operation or a component entry, L2, intra-file, each only when it names an
  entity the document declares: `declared_only`): a local `$ref` under it at any depth, read as a JSON
  Pointer. The first prefix that begins it gives the target kind (`#/components/schemas/`, `.../parameters/`,
  `.../responses/`, `.../requestBodies/`, `.../headers/`, `.../securitySchemes/`, and 2.0's `#/definitions/`,
  `#/parameters/`, `#/responses/`, `#/securityDefinitions/`) and the first segment after it, unescaped (`~1`,
  `~0`, percent-decoding), the name: `#/components/schemas/Pet/properties/name` is a reference to `Pet`.
- **References between documents** (L3, `fusion.ResolveDocXrefs`, file-tier edges from the referring
  document's root entity to the referenced document's): a `$ref` anywhere in a document whose value has a
  file part (the text before the `#`), resolved by `repo-path` from the document's directory. A URL and a
  fragment-only reference resolve to nothing.

## Engine additions (LB-F90), each opt-in

`$ref` as a selector key (`$..$ref`); `$parent` (the key one step up); the `@upper` and `@file` value
transforms; the `fmt:` template (`fmt:{$key @upper} {$parent}` is `GET /pets`), which is the one place the
engine composes strings (design §4 said "no templating"; an operation's name is a method and a path, two
keys, and no alternation reaches it); a reference's `pointer` list and a `from` of several kinds joined by
`|` (one rule serves the seven kinds that can carry a `$ref`). `schema/dialect/doc.cue` accepts them, and
every other dialect's goldens are unchanged.

## What is not modelled, and why

- **A fragment file** (a split description's `paths/pets.yaml`, `schemas/Pet.yaml`) carries no `openapi:` or
  `swagger:` key and is not claimed by content, so the operations in it are not read and a `$ref` that lands
  on it names no entity (design §1.4: the harness scores only edges between files the dialect owns, and
  counts the rest: OAI's petstore-separate 8, Redocly's starter 9, Azure's examples 157). LB-Q-openapi-rest
  names the ceiling and its trigger.
- **A document written with an indented root** (OAI's `examples/v2.0/yaml/petstore-minimal.yaml`: `---` and
  then every key two spaces in) is not claimed: the classifier's signal is column 0. Detection stays
  `openapiTopRe`.
- **A `$ref` outside every entity** (a path item's shared `parameters`, a root `security` entry): an
  operation's node does not contain its path item's. Counted by the oracle, in no truth.
- **Path items, callbacks, links, examples, `components.pathItems`**, an `operationRef`, and a reference
  into `#/paths/...`.
- **Files over the ingest's 2 MiB read cap** (stripe/openapi's spec3, 5 to 6 MB; kubernetes' `api__v1`,
  2.1 MB): ingest reads none of them, so they are measured on no reading.

## Divergences from the incumbent

The incumbent read YAML only and emitted `OpenAPIOperation` and `OpenAPISchema`. Every name, kind, tier and
signature it emitted is kept; the dialect adds the document, parameter, response, request-body, header and
security-scheme entities, the intra-file references and the JSON documents. An empty document
(`openapi: 3.0.0` alone) yields nothing, as before.

## Calibration oracle

`calibration/oracles/openapi/openapi_oracle.mjs` (bench-only): `@apidevtools/swagger-parser` 13.1.0 (tier A:
`parse` for the documents, `resolve` and `$refs.exists` for every reference), `yaml` 2.9.1 for key lines
(an independent YAML 1.2 reader with offsets, in place of yq), cross-checked by `@redocly/cli` 2.57.0
`bundle` (no disagreement on any subject). Subjects: `calibration/corpus.d/openapi.yaml`. Reading:
`docs/bench/readings/lang/openapi.json`: sym and ref recall and precision 1 over 3,372 definitions and 3,805
reference pairs, xref recall and precision 1 over 25 pairs (Azure's `$ref`s between Swagger documents, the
only subject whose cross-file references land on documents: a thin L3 truth).
