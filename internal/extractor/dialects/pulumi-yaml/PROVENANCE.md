# pulumi-yaml — provenance and divergences

LB-Q-pulumi-yaml, design `docs/design/language-breadth.md` §4, §13 (wave iac). A doc-engine
dialect: `Pulumi.yaml` and `Pulumi.yml` by name (fusion fast-path keys), a stack file
`Pulumi.<stack>.yaml` by the manifest's glob through fusion's `appFrameworkDispatch`.

## What it models

- The **project** (`PulumiProject`, `package`, named by `name`, signed by its runtime), each
  **config** key (`PulumiConfig`, `variable_decl`; a stack file's namespaced keys too), each
  **variable** (`PulumiVariable`), each **resource** with a `type` (`PulumiResource`, signed by
  it) and each **output** (`PulumiOutput`).
- **References** (L2): an interpolation `${name}`, `${name.prop}`, `${name[0]}` in any string of
  a resource, a variable or an output (`options.dependsOn` and `fn::` calls included; `$${` is a
  literal), to the resource, variable or config key it names. `${pulumi.stack}` names nothing of
  the file.
- **main** (L3 declared): a project's `main:` is a `repo-path` to its program directory.

## What is not modelled, and why

- `main` names a directory of program files: the doc-xref pass joins only doc-engine files, so
  the edge cannot be stored. Programs in other runtimes ride their own languages.

## Calibration oracle

`calibration/oracles/pulumi-yaml/pulumi_yaml_oracle.js` (bench-only): a reference parse with the
`yaml` package (ISC) and Pulumi YAML's documented program model (tier C): `pulumi convert --from
yaml --language pcl` type-checks every resource against its provider's schema, so it needs the
provider plugins, and `pulumi preview` a backend and credentials (LB-Q-pulumi-yaml-rest).
Subjects: `calibration/corpus.d/pulumi-yaml.yaml`.
