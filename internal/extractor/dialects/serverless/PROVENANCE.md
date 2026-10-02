# serverless — provenance and divergences

LB-Q-serverless, design `docs/design/language-breadth.md` §4, §13 (wave iac). A doc-engine
dialect claimed by name (`serverless.yml`, `serverless.yaml`; a fusion fast-path key, read before
any content signal, so ReingestFile decides alike). YAML with CloudFormation's short-form tags
(`yaml_tags: cloudformation`), as the Framework reads them. `serverless.ts` and `serverless.js`
are programs, not this dialect (LB-Q-serverless-rest).

## What it models

- The **service** (`ServerlessService`, `package`; `service` or `service.name`), the
  **provider** (`ServerlessProvider`, `section`) and each of its keys (`ServerlessProviderKey`,
  `key`), each **function** (`ServerlessFunction`, `function`, signed by its handler), each
  **event**: an `http`, `httpApi` or `websocket` one is a `ServerlessRoute` (`route`) named
  `METHOD path` (or as written), any other a `ServerlessTrigger` (`rule`) named by its type;
  `resources.Resources` entries (`ServerlessResource`, CloudFormation-shaped), `resources.Outputs`
  entries, each **custom** key (`ServerlessCustom`, `variable_decl`) and each **plugin**
  (`ServerlessPlugin`, `import`). A list of functions or resources reads as the map form.
- **References** (L2): a `self:custom.x` or `self:provider.x` source, in a `${...}` or as a
  fallback after a comma (`${opt:stage, self:provider.stage}`), to that custom or provider key
  (the doc engine's `pattern`, the innermost entity holding it); a CloudFormation `Ref`,
  `Fn::GetAtt` (and a resource's `DependsOn`) naming a resources entry, from every entity whose
  value holds it.
- **Includes** (L3 declared): `${file(./x.yml)}` is a `repo-path`; it resolves when the file is a
  doc-engine document (a CloudFormation resources file).

## What is not modelled, and why

- A handler's source file (`src/h.main` to `src/h.js` by runtime): the doc-xref pass joins only
  doc-engine files, so the edge cannot be stored (LB-Q-serverless-rest).
- `${self:service}` (the root's name is the service's, not `service`), `${opt:}`, `${env:}`,
  `${ssm:}`, `${cf:}`, `${sls:}` (no declaration of the file).

## Calibration oracle

`calibration/oracles/serverless/sls_oracle.js` (bench-only): the Serverless Framework v3.40.0
(MIT) — its own reader, `serverless print --format json` and its variable parser — with the
`yaml` package (ISC) for lines (tier A). Subjects: `calibration/corpus.d/serverless.yaml`.
