# terragrunt — provenance and divergences

LB-Q-terragrunt (wave iac), design `docs/design/language-breadth.md` §13, engine `native`,
grammar_source `native:github.com/hashicorp/hcl/v2` (MPL-2.0, a library dependency, unmodified).
`internal/extractor/terragruntdialect.go` walks the `hclsyntax` AST the terraform dialect already
reads; `hclwalk.go` is the entity and reference collection it shares with the hcl-tools dialect.

## Why native, not the tags engine

The queue entry named the tags engine and tree-sitter-grammars/tree-sitter-hcl (MIT). The catalogue
grammar differs: `grammars.json` flagged `grammar-differs:tree-sitter-grammars/tree-sitter-hcl` (the
language pack pins MichaHoffmann/tree-sitter-hcl, Apache-2.0), and no grammar was ever built or
pinned. `hclsyntax` is already vendored, evaluates the same expression language Terragrunt reads, and
a tags query cannot emit an include, dependency or terraform.source cross-file reference (its L3 is
`imports.scm` through `resolvestrategy.go`). The queue entry and the two grammar rows are retargeted.

## What it models

- **The file** (`TerragruntConfig`, canonical `package`): its base name; every file's root entity,
  what an include or a dependency of it resolves to.
- `include "l"` (`include.l`, an unlabelled one `include`; `import`), `dependency "n"`
  (`dependency.n`; `target`), `dependencies` (`target`), `terraform` (`module`), `remote_state` and
  `generate "g"` (`generate.g`; `resource`), `feature "f"` (`variable_decl`).
- Each `locals` attribute (`local.k`; `variable`) and each `inputs` key (`input.k`; `key`): the keys
  of an object literal, directly or as an argument of `merge()`; an `inputs` of any other shape is one
  entity, `inputs`. A repeated name gets `#2`.
- Not entities: `exclude`, `errors`, `engine`, `unit`, `stack`, the hook blocks inside `terraform`,
  and every other top-level attribute.

## References

- **L2 (Calls)**: `local.x`, `dependency.n` (read as `dependency.n.outputs.x`), `include.n` (read as
  `include.n.locals.x`) and `feature.n` traversals, from the entity (block, local or input key) they
  are read in; a heredoc template's interpolations included.
- **L3, paths** (`repo-path`): each path expression is read by its static value. The forms read:
  a literal, a local of the same file whose own value is static, `get_terragrunt_dir()`,
  `dirname()`, a template joining them, and `find_in_parent_folders("name")` /
  `find_in_parent_folders()` (default `terragrunt.hcl`), which is the nearest ancestor of the file's
  directory's parent that has the file. One file cannot see which ancestors have it, so every
  ancestor's candidate is emitted and `fusion.ResolveDocXrefs` keeps those that are files (as the
  nginx dialect does for an include): a repository with the file at two ancestors gets both edges,
  Terragrunt only the nearer. `include.path` is an `includes` edge; `dependency.config_path` and
  `dependencies.paths` a `dependency` edge to `<dir>/terragrunt.hcl` (the value itself when it ends
  in `.hcl`).
- **L3, terraform.source** (`label-selector`): a local source (`.`, `./…`, `../…`, `?query` and a
  `//subdir` below it read as the module directory inside) is an edge from `terraform` to every
  definition of the module directory (scope `terraform:<dir>`, label `terraform.module`, which the
  terraform dialect now puts on every definition beside `terraform.address`); and, from each `inputs`
  key, a `module-input` edge to the module's `var.<key>`.
- **Counted, not guessed** (`TerragruntUnresolvedRefs`): a function none of the above reads
  (`function:get_env`, `function:get_parent_terragrunt_dir`, …), a reference (`reference:dependency`,
  `reference:include`), `no-ancestor`, `outside-repo`, `absolute-path`, and a source that names
  nothing of the repository (`external:git` for `git::`, `git@`, `github.com/`; `external:registry`
  for `tfr:` and `ns/name/provider`; `external:remote` for a URL; `external:other`).

## The unit rule

Only a file named `terragrunt.hcl` resolves a relative path. In a file an include brings in
(`root.hcl`, `_common/app.hcl`) Terragrunt reads `config_path`, a source, `get_terragrunt_dir()` and
`find_in_parent_folders()` from the including unit's directory, which one file cannot see, so every
path there is `included-file` unresolved (an external source is still classified).

## Classification

`terragrunt.hcl` and `root.hcl` by basename; any other `.hcl` by content sniff (a top-level
`include`, `dependency`, `generate` or `feature` block, `dependencies`, `remote_state`, `locals` or
`terraform {`, or `inputs =`, in the first KiB). A `.hcl` no sniff claims (a Vault or Consul
configuration, a `.terraform.lock.hcl`, a file that opens with more than a KiB of comments) is
counted ambiguous, never extracted.

## Limits (LB-Q-terragrunt-rest)

- Nearest-ancestor: see above; the doc-xref resolver cannot pick the nearest of several candidates.
- `read_terragrunt_config`, `read_tfvars_file`, `sops_decrypt_file`: no read edge (the tool reports
  the files a unit reads but not which entity reads them).
- `config_path = include.x.…` and `get_parent_terragrunt_dir()`: unresolved.
- A stack file (`terragrunt.stack.hcl`) and `unit`/`stack` blocks are not read.

## Calibration

`calibration/oracles/terragrunt/` (python-hcl2 for definitions and references, tier C; Terragrunt
1.1.6 `render --json --with-metadata` and `find --json --include` for the cross-file references,
tier A). Subjects: `calibration/corpus.d/terragrunt.yaml`.
