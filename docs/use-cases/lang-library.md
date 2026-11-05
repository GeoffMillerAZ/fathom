# The Language Library

Read a source file or a configuration file into symbols, references and declared facts from your own Go program or shell script, without a store, a daemon, an index, git or a network. The same extraction `fathom ingest` runs, with the level fathom measured for each dialect attached to every answer.

---

## When to use this

- You are writing a tool (a policy checker, a linter, a documentation generator) that needs to know what a Terraform, Go, Python, Dockerfile, YAML or Markdown file defines and references, and you do not want to run fathom's daemon to get it.
- You want to know how well fathom reads a language before you trust it: every `Result` and every dialect carries the measured level (L1 to L4, `none` or `unmeasured`).
- You have a format fathom does not read and want to add it as a data pack, or as a Go extractor, and prove it against goldens.

It is not a query engine. Nothing is stored, nothing is joined across files, and a symbol in one file is not resolved against another. For that, use the indexed tools in [coding-context.md](coding-context.md).

---

## Surfaces

| Name | Kind | What it gives you |
|---|---|---|
| `pkg/lang` | Go package | `lang.New`, `Reader.Extract` (one `Result` per file: dialect, symbols, references, unresolved references, facts, level, notes), `Reader.Dialects`, `WithDialects`, `WithExtractor`, `WithLimits`, `EnableCompilationCache` |
| `pkg/lang/cue` | Go package | `cue.Option()` links CUE and Timoni extraction. Without it `pkg/lang` links no `cuelang.org/go` and a `.cue` file reads `Skipped: not-linked` |
| `pkg/lang/langtest` | Go package | `CheckPack`: a pack's validation, required kinds and goldens, for pack authors |
| `cmd/fathom-lang` | Binary (stdlib flags only) | `extract` and `dialects`, printing the same facts as JSON for tools not written in Go |

---

## From Go

```go
r, err := lang.New()
if err != nil { /* a bad option or pack: the error says which */ }
defer r.Close()

res, err := r.Extract(ctx, lang.File{Path: "infra/main.tf", Src: src, Siblings: paths})
// res.Dialect, res.Skipped, res.Symbols, res.References, res.Unresolved, res.Facts, res.Level, res.Notes
```

- **Bytes and a path.** `Path` is repository-relative with forward slashes. `Siblings` (the other paths in the same repository) is optional: some classifications read it (an Ansible role, a Helm chart, a Timoni module), and `Notes` says so when leaving it out may have changed the answer.
- **Skipped, with a reason.** A file no dialect reads is not an error. `Skipped` is one of `binary`, `oversize`, `generated`, `ambiguous`, `unclassified` or `not-linked`, and a skipped file has no facts.
- **`Canonical` and `Kind`.** `Symbol.Canonical` is a closed cross-dialect vocabulary that only grows, and is compatibility surface. `Symbol.Kind` is each dialect's own name (for example `TerraformResource`) and changes as a language improves: do not branch on a `Kind` across dialects.
- **Concurrency and determinism.** A `Reader` is safe for concurrent use. Equal inputs give byte-equal JSON.
- **No side effects.** The library opens no socket and writes no file. The one opt-in is `lang.EnableCompilationCache(dir)`, which keeps the compiled grammars (machine code) in `dir`; it refuses a directory that a group or other user can write, and errors on a second directory.
- **Limits.** Files over 2 MiB are skipped. `WithLimits` may lower the limit, never raise it.

The runnable versions are the godoc examples on `Reader.Extract`, `Reader.Dialects` and `WithExtractor`.

### Packs and extractors

`lang.WithDialects(fsys)` adds a dialect pack (`dialects/<id>/dialect.json` with the queries and doc specs it names) to a `Reader`. It is merged with the built-in dialects, held to the same rules, compiled at `New` and refused whole with a `*lang.PackError` (a `Pack*` reason) when any rule breaks. `lang.WithExtractor(match, e)` registers a Go extractor under the same routing rules. A pack or extractor dialect reads `unmeasured` until it ships a reading. Run `langtest.CheckPack` in the pack's own tests.

---

## From a shell

```
fathom-lang extract [-root DIR] [-siblings FILE|-] [-walk] [-cache DIR] [-pack DIR] PATH...
fathom-lang dialects [-pack DIR]
```

`extract` prints `{"files":[{"path":...,"result":...}]}`, `dialects` prints `{"dialects":[...]}`. Both are JSON with the field names of `lang.Result` and `lang.DialectInfo`. The output is compatibility surface for tools not written in Go: fields are added, never renamed, and a change is a reviewed edit of the goldens under `cmd/fathom-lang/testdata/golden`. Exit 0 on success, 1 on an error, 2 on misuse.

```
$ fathom-lang extract -root infra main.tf | jq '.files[0].result | {dialect, level, symbols: [.symbols[].name]}'
{ "dialect": "terraform", "level": "L3", "symbols": ["aws_s3_bucket.logs", "var.name"] }
```

Release archives carry `fathom-lang` beside `fathom`, and `task deploy` installs it next to the daemon binary.

---

## Setup & notes

- **Import cost.** An outside module that imports `pkg/lang` links at most 80 non-standard packages and builds a stripped binary under 30 MiB (reading: `docs/bench/readings/lang-library/m1-closure.md`). `TestLangClosure` fails if the door reaches fathom's store, fusion, connectors, MCP server, `net/http` or CUE, and `TestOutsideModuleBuilds` builds a real outside module offline against this tree.
- **One module.** `pkg/lang` lives in fathom's module, so a consumer takes fathom's Go version and dependency pins. A nested module is kept open (design `docs/design/lang-library.md` section 7) and will be cut if a real consumer reports a version conflict.
- **API changes are deliberate.** `TestLangAPISnapshot` holds `go doc -all` for the three packages. After an intended change run `go test ./pkg/lang -run TestLangAPISnapshot -args -update-api` and read the diff of `pkg/lang/testdata/api`.
- **What it does not do.** Resource attribute reads, catalogs and policy checks are separate work (ADR-091, ADR-092) and are not in this guide yet.

---

## Maintained

This doc covers: `pkg/lang` (`lang.New`, `Reader.Extract`, `Reader.Dialects`, `WithDialects`, `WithExtractor`, `WithLimits`, `EnableCompilationCache`), `pkg/lang/cue`, `pkg/lang/langtest`, `cmd/fathom-lang` (its extract and dialects commands, the `-root`, `-siblings`, `-walk`, `-cache` and `-pack` flags and the JSON goldens), and the same extraction as `fathom ingest`. Update it when these change.
