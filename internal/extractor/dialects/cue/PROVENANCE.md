# cue — provenance and divergences

LB-Q-cue, design `docs/design/language-breadth.md` §4, §13, §18.3 (T6). A native-engine dialect (no
`tags.scm`, no doc spec): `internal/extractor/cueextract.go` reads a `.cue` file with the official
parser, `cuelang.org/go/cue/parser` (Apache-2.0, a direct dependency at v0.16.1), which also binds
each identifier to the declaration it names in the file (`astutil.Resolve`), and gives the
document-path engine's cross-file pass (`fusion.ResolveDocXrefs`) its `DocResult`
(`extractor.DocResulter`, `cuedoc.go`), resolved by the cue-import and cue-package strategies
(`internal/fusion/cuexref.go`). The extractor was registered before the manifest (`register.go`
maps `.cue` to it) and is upgraded in place, not ported.

## What it models

- **A definition** (`CUEDefinition`, canonical `type_alias`) and **a field** (`CUEField`, canonical
  `field`): every field a struct literal declares, wherever the literal is written — a field's value,
  a conjunction, disjunction or default operand, a list element or element type, a comprehension's
  source or body (`for`, `if`, `let`, `try`, a fallback), a let's value, an embedding, a selector's
  base (`(#A & {f: 1}).out`) — at any depth up to `cueMaxDepth` (256; the cap was six, which real
  configuration exceeds: a CRD's OpenAPI schema imported into CUE nests past twenty). A label
  starting with `#` or `_#` is a definition. The name is the path of the labels of the fields that
  enclose it (`#Kit.ports.port`, `alt.d1`, `w.y` for a field of a let's struct), the naming the cue
  binary's own language server uses for its document symbols; an alias is not part of a label
  (`X=name:` is `name`), a quoted label is its content. A field with no static name (a pattern
  constraint `[string]: v`, an interpolated or parenthesized label) is no entity; the fields inside
  its value are, under its label as written (`labels.[string].value`). Range: the field's first to
  last line.
- **A package** (`CUEPackage`, canonical `package`): the package clause, named `package/<name>` and
  signed `package <name>`, the file's root entity — what an import resolves to. The prefix keeps its
  symbol key apart from a top-level field of the package's own name (`package dashboards` beside
  `dashboards: {...}` is common in real modules; `core.SymbolKey` carries no kind).
- **An import** (`CUEImport`, canonical `import`): one per import declaration whose path is a string
  literal, named by the path (`example.com/m/schema`, `strings`); `import 42` is none.
- **References** (each field's `Calls`, compared as call pairs at L2): every identifier and selector in
  a field's value that names a field or definition, the field's own nested fields' values excluded
  (they carry their own) and a pattern's or dynamic field's included. A name the parser binds in the
  file is the declaration's `core.SymbolKey` (every declaration of that name in its scope, as CUE
  unifies them); a selector into a declaration whose value the file writes as a struct (through
  conjunctions, disjunctions, defaults, parentheses, aliases, value aliases `V={...}` and lets) is
  that nested declaration's key; a selector into a value that unifies with a declaration the file
  does not write is that name with the label appended (`#Conv.result`, `vend.#V.x`); a selector on a
  path of the package value the file's struct lacks is the path (`net.config`, which another file
  may declare); a selector on an import (not the standard library) is `seg.Name…`; a name the file
  does not declare is the bare name (`#Kit`, `shared`). Let, alias and comprehension variables, the
  standard library and predeclared identifiers (`string`, `len`, `matchN`, …) are no reference.
- **Cross-file references** (`fusion.ResolveDocXrefs`, file-tier edges, entity to entity):
  - `imports` (cue-import, origin the import entity): the package entity of each file of the
    package the import loads. An import path inside its module (the `module` field of the nearest
    `cue.mod/module.cue`, `@vN` dropped; the extractor signs that field with it) names the directory
    at that path under the module root; any other path is looked up in `cue.mod/pkg`, `cue.mod/gen`
    and `cue.mod/usr`. The package is that directory's files, and its ancestors' up to the module
    root, whose clause names it (the path's last element, or its `:name` qualifier).
  - `references` (cue-import, origin the referring field): a selector through an import, to the
    declaration of that name in the package.
  - `references` (cue-package, origin the referring field): a name the file does not declare, and a
    path of the package value it does declare (CUE unifies every file's declarations), to the
    declarations of that name in the package's other files (its directory and ancestors).
  The standard library and a registry module outside the repository resolve to nothing.

The tier is `verified`, the incumbent's: a real parse. Nothing here is an evaluated fact: a reference
CUE settles only by evaluating (a field of a definition another package embeds, `repo.writeWorkflows`
reaching `base`) is not resolved across files; `internal/fusion/typedcue.go` (cue/load, top-level
fields, a repository-root `cue.mod` only) remains the typed pass.

## The dialect gate

The manifest claims `.cue`, so ingest's dialect gate now applies to CUE files: a generated file
(`// Code generated cue get go. DO NOT EDIT.`, a `cue.mod/gen` tree written by `cue get go` or
`timoni mod vendor`) is skipped, as every dialect's generated files are; before the manifest it was
read and marked generated. An import into such a tree resolves to no file at ingest.

## Calibration oracle

`calibration/oracles/cue` (tier A, its README): the cue binary v0.16.1, the version `go.mod` pins,
run as separate processes — `cue vet -c=false` per package (the gate; `cue help cmd` for `_tool.cue`
files, which vet leaves out; `cue def` beside it), `cue lsp serve` (the binary's language server:
document symbols for the definitions, `textDocument/definition` at every identifier a lexical scan
proposes for the references and cross-file references, at each import for the files it loads), and
`cue export` for the module file's fields. It never loads a CUE library and never calls fathom code.

## Fixtures

`testdata/module` is a small module (a vendored package under `cue.mod/pkg`, a package importing a
sibling, files referring to each other, every place CUE declares a field); `testdata/broken.cue` a
mid-edit save. `testdata/goldens.json` holds the extractor's output for each, and
`testdata/module.oracle.json` the cue binary's answer for the module (`cue-oracle`), which the
dialect matches exactly on definitions, ranges, kinds and references
(`TestCUEDialectAgreesWithTheCueBinary`). Reserved example names only (ADR-011).
