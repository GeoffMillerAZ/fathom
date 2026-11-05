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
// res.Dialect, res.Skipped, res.Symbols, res.References, res.Unresolved, res.Facts, res.Level, res.Incomplete, res.Notes
```

- **Bytes and a path.** `Path` is repository-relative with forward slashes. `Siblings` (the other paths in the same repository) is optional: some classifications read it (an Ansible role, a Helm chart, a Timoni module), and `Notes` says so when leaving it out may have changed the answer.
- **Skipped, with a reason.** A file no dialect reads is not an error. `Skipped` is one of `binary`, `oversize`, `generated`, `ambiguous`, `unclassified` or `not-linked`, and a skipped file has no facts.
- **Incomplete.** `res.Incomplete` is true when an engine could not fully read the file, and `Notes` gives a one-line reason (a decode failure, or the budget a very large file spent, such as the grammar module's 64 MiB memory limit). A policy tool that must not pass a file it could not read should refuse on it. The tree-sitter dialects recover from a syntax error without reporting it, so `false` means no engine reported a failure, not that every construct was read.
- **Credentials.** A `Result` does not carry one. The userinfo of a URL is dropped wherever it lands (a module source, a dependency repository, a kustomize resource, a reference target), and a string literal bound to a credential-named parameter or variable (`password`, `api_key`, `token`), a credential flag (`--password`, `curl -u user:pass`) and an Authorization header are masked as `***` in a `Signature`. It is a list of patterns and not a proof, so a credential in another shape is not promised withheld.
- **Limits that refuse.** A file nested past 10,000 levels (brackets, but also a run of prefix operators or field labels, and a chain of selectors or calls) and a TOML document of more than 40,000 keys read `Incomplete` with the reason, because the parsers behind them overflow the stack or run in quadratic time, and neither can be interrupted. A panic in an extractor is one file's `Incomplete`, not the batch's end.
- **References.** `Reference.To` is the target as the dialect writes it, and `ToKey` is the `Key` of the one symbol of the same file it names, empty otherwise (another file, an ambiguous name). `Edge` is the relation the dialect declared (`ref`, `depends_on`, `uses`, `references`), `call` for a call, and `reference` for a document dialect that declares none. `Line` is where it is written, 0 when the engine records none (a call read by the Go or a tree-sitter engine).
- **`Canonical` and `Kind`.** `Symbol.Canonical` is a closed cross-dialect vocabulary that only grows, and is compatibility surface. `Symbol.Kind` is each dialect's own name (for example `TerraformResource`) and changes as a language improves: do not branch on a `Kind` across dialects.
- **Concurrency and determinism.** A `Reader` is safe for concurrent use. Equal inputs give byte-equal JSON.
- **No side effects.** The library opens no socket and writes no file. The one opt-in is `lang.EnableCompilationCache(dir)`, which keeps the compiled grammars (machine code) in `dir`; it refuses a directory that a group or other user can write, and errors on a second directory.
- **Limits.** Files over 2 MiB are skipped. `WithLimits` may lower the limit, never raise it.
- **Context.** `Extract` checks `ctx` on entry and between a file's stages (after dispatch and after the engine returns), never inside a parse: a deadline shorter than one parse is reported when that parse ends, not at once. The engine's own 10 second parse and query deadline is the one wall-clock input to a `Result`: a file that runs past it reads `Incomplete` with the deadline named, so a slower machine can read fewer files fully than a faster one.
- **Dialect and Level.** `Dialect` is "" only for a skipped file. The JSON and TOML files no dialect claims are read by `generic-json` and `generic-toml`, which `Dialects` lists as unmeasured. `Level` is "" on a skipped file.

The runnable versions are the godoc examples on `Reader.Extract`, `Reader.Dialects` and `WithExtractor`.

### Packs and extractors

`lang.WithDialects(fsys)` adds a dialect pack (`dialects/<id>/dialect.json` with the queries and doc specs it names) to a `Reader`. It is merged with the built-in dialects, held to the same rules, compiled at `New` and refused whole with a `*lang.PackError` (a `Pack*` reason) when any rule breaks. `lang.WithExtractor(match, e)` registers a Go extractor under the same routing rules. A pack or extractor dialect always reads `unmeasured` in this version: no mechanism loads a reading for it. A pack may not take a file whose extension a built-in dialect owns (a glob of `**/*` still reads `.tf` and `.go` files with their own dialects), and is refused whole for a glob that is not a pattern, an empty tags query, a tags capture of a kind the manifest does not declare, an extension variant whose own query does not compile, and any file in a dialect directory the manifest does not name. Run `langtest.CheckPack` in the pack's own tests.

---

## From a shell

```
fathom-lang extract [-root DIR] [-siblings FILE|-] [-walk] [-cache DIR] [-pack DIR] PATH...
fathom-lang dialects [-pack DIR]
```

`extract` prints `{"schema_version":1,"files":[{"path":...,"error":...,"result":...}]}`, `dialects` prints `{"schema_version":1,"dialects":[...]}`. Both are JSON with the field names of `lang.Result` and `lang.DialectInfo`. PATH may be absolute or relative to `-root`. A file that cannot be read has an `error` and a null `result`, and the other files are still read. The output is compatibility surface for tools not written in Go: fields are added, never renamed, `schema_version` changes only with a break, and a change is a reviewed edit of the goldens under `cmd/fathom-lang/testdata/golden`. Exit 0 when every file was read, 1 when a file could not be read or the run failed, 2 on misuse (a bad flag, no PATH, an unknown command). The command links no CUE: a `.cue` file reads `Skipped: not-linked`, and a Go program adds CUE with `pkg/lang/cue`.

```
$ fathom-lang extract -root infra main.tf | jq '.files[0].result | {dialect, level, symbols: [.symbols[].name]}'
{ "dialect": "terraform", "level": "L3", "symbols": ["aws_s3_bucket.logs", "var.name"] }
```

Release archives carry `fathom-lang` beside `fathom`, and `task deploy` installs it next to the daemon binary.

---

## Setup & notes

- **Import cost.** An outside module that imports `pkg/lang` links at most 80 non-standard packages (79 for the library itself, whose own package is the 80th) and builds a stripped binary under 30 MiB (reading: `docs/bench/readings/lang-library/m1-closure.md`). `TestLangClosure` fails if the door reaches fathom's store, fusion, connectors, MCP server, `net/http` or CUE, and `TestOutsideModuleBuilds` builds a real outside module offline against this tree.
- **One module.** `pkg/lang` lives in fathom's module, so a consumer takes fathom's Go version and dependency pins. A nested module is kept open (design `docs/design/lang-library.md` section 7) and will be cut if a real consumer reports a version conflict.
- **API changes are deliberate.** `TestLangAPISnapshot` holds `go doc -all` for the three packages. After an intended change run `go test ./pkg/lang -run TestLangAPISnapshot -args -update-api` and read the diff of `pkg/lang/testdata/api`.
- **What it does not do.** Resource attribute reads, catalogs and policy checks are separate work (ADR-091, ADR-092) and are not in this guide yet.

---

## Maintained

This doc covers: `pkg/lang` (`lang.New`, `Reader.Extract`, `Reader.Dialects`, `WithDialects`, `WithExtractor`, `WithLimits`, `EnableCompilationCache`), `pkg/lang/cue`, `pkg/lang/langtest`, `cmd/fathom-lang` (its extract and dialects commands, the `-root`, `-siblings`, `-walk`, `-cache` and `-pack` flags and the JSON goldens), and the same extraction as `fathom ingest`. Update it when these change.
