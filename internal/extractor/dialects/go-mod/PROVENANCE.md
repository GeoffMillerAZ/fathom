# go-mod — provenance and divergences

LB-Q-go-mod, design `docs/design/language-breadth.md` §4, §13. A native-engine dialect (no
`tags.scm`, no doc spec): `internal/extractor/gomoddialect.go` reads a `go.mod` and a `go.work`
with `golang.org/x/mod/modfile` (BSD-3-Clause; a direct dependency now, it was indirect), the parser
the go command itself uses, and gives the document-path engine's cross-file pass its `DocResult`
(`extractor.DocResulter`) so `fusion.ResolveDocXrefs` draws the module graph a repository declares.
The incumbent, `gomodextract.go`, was a line scanner that emitted one `ModuleDep` per require and
nothing else; it was retired in the commit that switched dispatch to this dialect. This file records
what the dialect models, what it deliberately does not and why, and where the calibration oracle
comes from.

## What it models

- **The module** (`GoModule`, canonical `module`): the `module` directive, named by the module path,
  signed `go <version>` when a `go` directive is written, at the directive's line. It carries every
  required module's path as a call (sorted, once each), and is the file's root entity: what a
  file-level reference resolves to.
- **A requirement** (`ModuleDep`, canonical `package`): one per `require`, in a block or on one
  line, named by the module path and signed by the version, at its own line. This is the incumbent's
  kind, name, signature and tier, kept exactly (`TestGoModDialectKeepsIncumbentContracts`).
- **A replacement** (`GoModReplace`, canonical `key`): one per `replace` of a go.mod or a go.work,
  named `replace/<old path>` (`replace/<old path>@<old version>` when the left-hand side names a
  version), signed by its right-hand side (`../lib`, `github.com/acme/errors v0.9.2`). The prefix is
  why a module that is both required and replaced keeps two symbol keys: `core.SymbolKey` carries no
  kind, and one file's entities are keyed by name alone (the same collision Cargo's `bin/<name>`
  avoids).
- **A workspace** (`GoWorkspace`, canonical `namespace`): a go.work's own entity, named `workspace`,
  spanning its first directive to its last (the `go`, `toolchain`, `godebug`, `use` and `replace`
  lines), signed `go <version>`. It is the go.work's root entity.
- **A use** (`GoWorkUse`, canonical `key`): one per `use` of a go.work, named `use/<directory as
  written>` (`use/./staging/src/k8s.io/api`, `use/.`), at its own line.
- **Cross-file references** (`fusion.ResolveDocXrefs`, file-tier edges, entity to entity):
  - `depends_on` (`module-source`, origin the module): each requirement's module path to the one
    other go.mod that declares a module of that name.
  - `replaced_by` (`repo-path`, origin the replace): a replace whose right-hand side is a directory
    (`./x`, `../x`, `/abs`, a drive path), to that directory's go.mod, relative to the file's own
    directory. A directory that is absolute or climbs out of the repository resolves to nothing (the
    reference is recorded with an empty target, as the doc engine records one).
  - `workspace_member` (`repo-path`, origin the use): a go.work `use` to that directory's go.mod.
  A target is the go.mod file's path, never the directory: a directory that holds both a go.mod and a
  go.work (a workspace root) is two doc files, and the resolver's "the one doc file in a directory"
  rule would drop it.

The tier is `pre-scan`, the incumbent's: nothing here is a type-checked or evaluated fact, and a
promotion to `verified` (the modfile parse is a real one) is a separate decision the incumbent's
contract test would have to change with.

## Reading tolerance

`modfile.Parse` refuses a file with any bad directive, and `ParseLax`, which is meant for
dependencies' go.mod files, drops every `replace`. The extractor therefore parses strictly and, when
the parser rejects lines, blanks each rejected line (line numbers kept) and reads again, up to 32
rounds: a require whose path and version disagree (`example.com/a/v3 v1.0.0`), a directive from a
newer Go, a merge marker, a stray parenthesis costs that line and not the file, as the incumbent's
line scanner did. A block left open at the end of the file is closed. A file the parser still rejects
after that reads as nothing, no nodes and no error (the doc engine's convention for malformed input).

## What it deliberately does not model

- **Which registry a name means.** A requirement resolves by module path alone, as Cargo's and
  Maven's do. Go resolves a path to the registry unless a `replace` or a workspace redirects it, so a
  requirement that shares a path with a module of the repository, in a go.mod that neither replaces
  it nor sits in a workspace with it, is drawn as an edge the tool does not draw. The reading has
  one: `hack/tools/golangci-lint` requires the published `k8s.io/apimachinery` and kubernetes carries
  the staged one (xref_precision .9949). The rule is the doc engine's `module-source`, not something
  a dialect adds.
- **An inactive replace.** A replace to a local directory is recorded whether or not anything
  requires the replaced module; the tool lists a replacement only for a module in the build list
  (`hack/tools/instrumentation`'s `replace k8s.io/kubernetes => ../../..`, the reading's other
  false edge).
- **A path two go.mod files declare.** The resolver's unique-root rule resolves such a name to
  nothing; cli/cli's `.github/codeql/tests/...` module declares the root module's own path.
- **`// indirect`.** The incumbent recorded no flag and its contract fixes the `ModuleDep`
  signature as the bare version, so a direct and an indirect requirement read alike.
- **`exclude`, `retract`, `toolchain`, `godebug`, `tool` and `ignore`** (go.mod), **`toolchain` and
  `godebug`** (go.work). None is a module, a dependency edge or a location; the go.work's `toolchain`
  and `godebug` lines only widen the workspace entity's span.
- **`go.sum`, `go.work.sum` and `vendor/modules.txt`.** Not manifests of what a module declares.
- **A version the parser canonicalizes** (`v1.0` reads `v1.0.0`; modfile cannot hand back the
  written spelling). Real requirements are canonical.

## Where the oracle comes from

`calibration/oracles/go-mod` (its own module, bench-only, tier A): the go command's `go mod edit
-json` and `go work edit -json` for what each file declares, `go list -e -m -json all` (module mode
with `GOWORK=off -mod=mod`, and workspace mode where a go.work reaches the module) for what a
declaration resolves to. The parse is the package the dialect reads with, so it attests what the
dialect models and not the parser; its positions come from an independent line scan and its
evaluation from the tool. Calibration subjects are sparse checkouts of the go.mod family of
kubernetes/kubernetes and cli/cli (`calibration/corpus.d/go-mod.yaml`, `sparse:` patterns). The
reading is `docs/bench/readings/lang/go-mod.json`.

Fixtures (`testdata/`, hand-written goldens, each read as the file named in its golden's `path`):
`basic.mod` (every directive the dialect reads, block and single forms, a replace to a local
directory, a registry fork, a version-qualified left-hand side, a path climbing out of the
repository, exclude and retract), `single.mod`, `quoted.mod` (quoted paths, `+incompatible`, a
pseudo-version, a comment inside a block), `syntax-error.mod`, `workspace.work` and `nested.work`.
