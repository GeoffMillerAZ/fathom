# hcl-tools — provenance and divergences

LB-Q-hcl-tools (wave iac), design `docs/design/language-breadth.md` §13, engine `native`,
grammar_source `native:github.com/hashicorp/hcl/v2` (MPL-2.0, a library dependency, unmodified).
`internal/extractor/hcltoolsdialect.go` walks the `hclsyntax` AST the terraform dialect already reads
(`hclwalk.go` is the collection it shares with the terragrunt dialect); one extractor serves three
tools, told apart by the file's name.

## Why native, not the tags engine

As for terragrunt: the catalogue grammar differs (`grammar-differs:tree-sitter-grammars/tree-sitter-hcl`;
none was pinned), `hclsyntax` is already vendored, and a tags query cannot read a Nomad task's
`config.image` as an image-ref cross-file reference.

## What it models

- **Packer** (`*.pkr.hcl`, `*.pkrvars.hcl`): `source "t" "n"` (`source.t.n`; `resource`), `build`
  (`build`, or `build.<name>` by its `name` argument; `target`), `variable "n"` (`var.n`;
  `variable_decl`), `locals` (`local.k`), `data "t" "n"` (`data.t.n`), and in a build each
  `provisioner "t"` and `post-processor "t"` (also inside `post-processors`), named
  `<build>.provisioner.t` and `<build>.post-processor.t` (`step`). A `.pkrvars.hcl` file's top-level
  keys (`key`). A repeated name gets `#2`. The `packer {}` block is not an entity.
  L2: a build's `sources = [...]` (quoted `"source.t.n"` strings or bare traversals) and `source
  "t.n"` blocks call `source.t.n`; `var.`, `local.`, `data.` and `source.` traversals call theirs.
- **Nomad** (`*.nomad`, `*.nomad.hcl`, a `.hcl` with a top-level `job "name" {` in its first KiB):
  `job "j"` (`j`; `job`), `group "g"` (`j.g`; `section`), `task "t"` (`j.g.t`; `task`),
  `service` (`<parent>.service.<name>`, `service` when it has no literal name; `service`),
  `variable` and `locals`. L2: `var.` and `local.` traversals, from the innermost entity. A task's
  `config { image = "x" }` with a literal image is an `image` cross-file reference (strategy
  `image-ref`), resolved through the image-tag index (`fusion/imageref.go`) to the Dockerfile that
  builds it; an image that is a variable or an interpolation is left out.
- **docker-bake** (`docker-bake.hcl`, `docker-bake.override.hcl`, `docker-bake*.hcl`): `target "t"`
  (`t`; `target`), `group "g"` (`group.g`; `stage`), `variable "v"` (`var.v`), `function "f"`
  (`func.f`; `function`). L2: a group's `targets` (a target `t` or a group `group.g`), a target's
  `inherits` and `contexts = { x = "target:t" }` (`t`), `target.t.attr` (`t`), a variable named
  bare (`var.v`) and a call of a declared function (`func.f`). The image tags a target builds are
  not read here: the image-ref index reads them through `fusion.BakeBuilds`, once.

## Classification

`docker-bake.hcl` and `docker-bake.override.hcl` by basename, `.nomad` by extension, `**/*.pkr.hcl`,
`**/*.pkrvars.hcl`, `**/*.nomad.hcl` and `**/docker-bake*.hcl` by glob, a `.hcl` by sniff (`job "x" {`
at the start of a line). Vault and Consul configuration is not claimed: an unclaimed `.hcl` is
counted ambiguous, never extracted.

## Limits (LB-Q-hcl-tools-rest)

- No tier-A oracle for Packer or Nomad (neither binary is installed): their readings are tier C and
  the grade caps at good.
- Vault and Consul configuration files are not read.
- A bake file split over several `-f` files is read file by file; `inherits` and `targets` naming a
  target of another file name nothing here.
- A matrix target is one entity (buildx lists each expansion).
- A Nomad jobspec in HCL1 syntax that `hclsyntax` rejects yields what the parser recovers.

## Calibration

`calibration/oracles/hcl-tools/` (python-hcl2, tier C; `docker buildx bake --list` as a cross-check
of the bake names). Subjects: `calibration/corpus.d/hcl-tools.yaml`.
