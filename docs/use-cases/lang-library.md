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
- **Skipped, with a reason.** A file no dialect reads is not an error. `Skipped` is one of `binary`, `oversize`, `generated`, `ambiguous`, `unclassified`, `not-linked` or `busy` (see Reading hostile input), and a skipped file has no facts.
- **Incomplete.** `res.Incomplete` is true when an engine could not fully read the file, and `Notes` gives a one-line reason (a decode failure, or the budget a very large file spent, such as the grammar module's 64 MiB memory limit). A policy tool that must not pass a file it could not read should refuse on it. The tree-sitter dialects recover from a syntax error without reporting it, so `false` means no engine reported a failure, not that every construct was read.
- **Credentials.** The door is not a secret scanner. It follows one rule about which of the file's own text lands in a `Result`, and the rule fails closed. A `Signature` from a code dialect on a list a test holds (Go, Java, Python, JavaScript, TypeScript, C, C++, CUDA, C#, Rust, Kotlin, Scala, Swift and Dart, each of whose string and char literal forms opens with a quote character) is kept up to its first quote character (`'`, `"` or a backtick) and reads `***` from there on, and in JavaScript, TypeScript and Scala from its first `<` if that comes first (JSX text and XML literals open with `<`, so generics are over-masked; regex and comment bodies are not covered), whatever a quote is (a comment's apostrophe or a C++ digit separator cuts too, and a Rust lifetime stays); a value written before the first quote stays. A `Signature` from any other code dialect (Ruby, PHP, Elixir and the rest) is empty. A `Signature` from a dialect that is no code (a manifest, a pipeline, a template) is empty unless the symbol's `Kind` is one whose `Signature` is a keyword or a declared name, and then it is cut at its first quote the same way, so a script's text is never returned. A `Fact` value under a credential-looking key (password, secret, token, apikey, credential, private_key) reads `***`, as does a literal a fact value binds with `=`, `:=` or `:`; other `Fact` values are returned as written. The userinfo of a URL, with or without its scheme, is dropped wherever it lands, without changing the URL's host. `Notes` carry a reason and a line, never the file's text. A name, a reference target and any value the rule does not name are returned as written, and an image reference or a module source is read from `Reference.To`.
- **Limits that refuse.** A parser that recurses on nesting is handed no file nested past 10,000 levels, and a few parsers refuse a shape that cost them hours (a template that melds past a budget, a TOML document of 40,000 keys or a table header of 64 segments, a proto file with a bracket open at its end). Those files read `Incomplete` with the reason. No other shape is promised refused, or bounded in time or memory, in process: see Reading hostile input. A panic in an extractor is one file's `Incomplete`, not the batch's end.
- **References.** `Reference.To` is the target as the dialect writes it (with a URL's userinfo dropped), and `ToKey` is the `Key` of the one symbol of the same file it names, empty otherwise (another file, an ambiguous name). `Edge` is the relation the dialect declared (`ref`, `depends_on`, `uses`, `references`), `call` for a call, and `reference` for a document dialect that declares none. `Line` is where it is written, 0 when the engine records none (a call read by the Go or a tree-sitter engine).
- **`Canonical` and `Kind`.** `Symbol.Canonical` is a closed cross-dialect vocabulary that only grows, and is compatibility surface. `Symbol.Kind` is each dialect's own name (for example `TerraformResource`) and changes as a language improves: do not branch on a `Kind` across dialects.
- **Concurrency and determinism.** A `Reader` is safe for concurrent use. Equal inputs give byte-equal JSON.
- **No side effects.** The library opens no socket and writes no file. The one opt-in is `lang.EnableCompilationCache(dir)`, which keeps the compiled grammars (machine code) in `dir`; it refuses a directory that a group or other user can write, and errors on a second directory.
- **Limits.** Files over 2 MiB are skipped. `WithLimits` may lower the limit, never raise it.
- **Context and the deadline.** `Extract` checks `ctx` on entry and between a file's stages, and returns by `lang.DefaultReadDeadline` (30 seconds, lowered by `WithLimits`). A context that ends mid-parse returns its error at once and abandons the read. The deadline, the engine's own 10 second parse and query deadline and a grammar module's memory limit are the wall-clock and memory inputs to a `Result`: a slower machine can read fewer files fully than a faster one.
- **Dialect and Level.** `Dialect` is "" only for a skipped file. The JSON and TOML files no dialect claims are read by `generic-json` and `generic-toml`, which `Dialects` lists as unmeasured. `Level` is "" on a skipped file.

The runnable versions are the godoc examples on `Reader.Extract`, `Reader.Dialects` and `WithExtractor`.

### Packs and extractors

`lang.WithDialects(fsys)` adds a dialect pack (`dialects/<id>/dialect.json` with the queries and doc specs it names) to a `Reader`. It is merged with the built-in dialects, held to the same rules, compiled at `New` and refused whole with a `*lang.PackError` (a `Pack*` reason) when any rule breaks. `lang.WithExtractor(match, e)` registers a Go extractor under the same routing rules. A pack or extractor dialect always reads `unmeasured` in this version: no mechanism loads a reading for it. A pack may not take a file whose extension a built-in dialect owns (a glob of `**/*` still reads `.tf` and `.go` files with their own dialects), and is refused whole for a glob that is not a pattern, an empty tags query, a tags capture of a kind the manifest does not declare, an extension variant whose own query does not compile, and any file in a dialect directory the manifest does not name. Run `langtest.CheckPack` in the pack's own tests.

---

## From a shell

```
fathom-lang extract [-root DIR] [-siblings FILE|-] [-walk] [-cache DIR] [-pack DIR]
                    [-deadline DURATION] [-max-memory SIZE] [-in-process] PATH...
fathom-lang dialects [-pack DIR]
```

`extract` prints `{"schema_version":1,"files":[{"path":...,"error":...,"result":...}]}`, `dialects` prints `{"schema_version":1,"dialects":[...]}`. Both are JSON with the field names of `lang.Result` and `lang.DialectInfo`. PATH may be absolute or relative to `-root`. A file that cannot be read has an `error` and a null `result`, and the other files are still read. `extract` reads in a child process it supervises (see Reading hostile input). The output is compatibility surface for tools not written in Go: fields are added, never renamed, `schema_version` changes only with a break, and a change is a reviewed edit of the goldens under `cmd/fathom-lang/testdata/golden`. Exit 0 when every file was read, 1 when a file could not be read or the run failed, 2 on misuse (a bad flag, no PATH, an unknown command). The command links no CUE: a `.cue` file reads `Skipped: not-linked`, and a Go program adds CUE with `pkg/lang/cue`.

```
$ fathom-lang extract -root infra main.tf | jq '.files[0].result | {dialect, level, symbols: [.symbols[].name]}'
{ "dialect": "terraform", "level": "L3", "symbols": ["aws_s3_bucket.logs", "var.name"] }
```

Release archives carry `fathom-lang` beside `fathom`, and `task deploy` installs it next to the daemon binary.

---

## Setup & notes

- **Import cost.** An outside module that imports `pkg/lang` links 81 non-standard packages today (80 for the library itself, its own package among them, and the consumer's `main`; LL-M1's recorded bar of 80 is one under and awaits an owner decision, design section 8) and builds a stripped binary under 30 MiB (reading: `docs/bench/readings/lang-library/m1-closure.md`). `TestLangClosure` fails if the door reaches fathom's store, fusion, connectors, MCP server, `net/http` or CUE, and `TestOutsideModuleBuilds` builds a real outside module offline against this tree.
- **One module.** `pkg/lang` lives in fathom's module, so a consumer takes fathom's Go version and dependency pins. A nested module is kept open (design `docs/design/lang-library.md` section 7) and will be cut if a real consumer reports a version conflict.
- **API changes are deliberate.** `TestLangAPISnapshot` holds `go doc -all` for the three packages. After an intended change run `go test ./pkg/lang -run TestLangAPISnapshot -args -update-api` and read the diff of `pkg/lang/testdata/api`.
- **What it does not do.** Resource attribute reads, catalogs and policy checks are separate work (ADR-091, ADR-092) and are not in this guide yet.

---

## Reading hostile input

A parser can hang, allocate without bound or overflow the stack on a file of a few kilobytes, and guards have not ended that: each round of review closed the shapes it named and found new ones. So the library promises two boundaries.

- **In process, a deadline.** `Reader.Extract` returns by `lang.DefaultReadDeadline` (30 seconds; `WithLimits(lang.Limits{ReadDeadline: d})` lowers it). A read that overruns reads `Incomplete` with a `read-deadline:` note and none of its facts. The parser is abandoned, not stopped: it keeps its CPU and memory until it returns. After `GOMAXPROCS` abandoned reads are still running, `Extract` starts nothing and returns `Skipped` `busy`, and recovers when they finish. A fatal stack overflow, a parser that never returns and a read that allocates without bound still take the calling process down or starve it.
- **Across a process, a kill.** `fathom-lang extract` is the way to read input you do not trust. It reads in a child process it supervises, and kills and restarts the child when a file takes longer than the deadline plus a margin, when the child dies for any reason, or when it holds more than `-max-memory` (4 GiB by default).

A file the supervisor killed does not stop the batch. Its entry has no error and an `Incomplete` result with one note that says why, and the files around it read as they would have:

```json
{
  "path": "evil.tf",
  "error": "",
  "result": {
    "dialect": "", "language": "", "skipped": "",
    "symbols": [], "references": [], "unresolved": [], "facts": [],
    "level": "", "incomplete": true,
    "notes": ["read-killed: the reader process ended while it read this file (exit status 2: fatal error: stack overflow), so none of its facts are returned"]
  }
}
```

Treat `Incomplete` as "not read". The exit status stays 0 for such a file, as for any `Incomplete` result: read the JSON. `-deadline` lowers the per-file deadline, `-max-memory 512MiB` lowers the memory limit, and `-in-process` turns the supervisor off. A library caller reads in process, so to get the same protection run `Reader.Extract` in a process of your own that you can kill. Output is byte-equal in both modes for a batch that faults nowhere.

## Maintained

This doc covers: `pkg/lang` (`lang.New`, `Reader.Extract`, `Reader.Dialects`, `WithDialects`, `WithExtractor`, `WithLimits` and the read deadline, `EnableCompilationCache`), `pkg/lang/cue`, `pkg/lang/langtest`, `cmd/fathom-lang` (its extract and dialects commands, the `-root`, `-siblings`, `-walk`, `-cache`, `-pack`, `-deadline`, `-max-memory` and `-in-process` flags, its supervised child and the JSON goldens), and the same extraction as `fathom ingest`. Update it when these change.
