# cue — the CUE dialect's oracle (LB-Q-cue, tier A)

The cue binary is the oracle (design §18.3: an oracle independent of the extractor; never fathom's
code path, never `cue/load` in process). This program runs it as separate processes and writes
`indexbench lang-oracle`'s tool-oracle JSON:

```
go build -o <scratch>/cue-oracle .
<scratch>/cue-oracle -subject <checkout> -cue <cue v0.16.1> -cache <CUE_CACHE_DIR outside the subject> \
  -files-out <files.txt> > workspace/tmp/oracle/cue-<subject>.json
indexbench lang-oracle -dialect cue -subject <checkout> -tool-oracle workspace/tmp/oracle/cue-<subject>.json \
  -only "$(paste -sd, <files.txt>)" -out docs/bench/readings/lang/cue.json
```

`cue` v0.16.1 is the version fathom's go.mod pins: `GOBIN=<scratch>/bin go install
cuelang.org/go/cmd/cue@v0.16.1`. A module with registry dependencies (`deps` in its module.cue)
fetches them into `-cache` (network).

| Step | The binary | What it gives |
|---|---|---|
| Gate | `cue vet -c=false <package>` per package instance (its import path, at its module root; a module-less file by its path); `cue help cmd` in the directory of `_tool.cue` files (vet leaves them out; it lists their commands only when the package loads with them); `cue export --out json ./cue.mod/module.cue` for a module file; `cue def <package>` beside the gate, counted | `-files-out`: the files of the packages it loads (a rejected package is named on stderr and not measured) |
| Definitions | `cue lsp serve` (the binary's language server), `textDocument/documentSymbol` per file, one session per module | every field and definition at its range, named by the label path; `kind` `definition` (a `#` label) or `field`, and `pattern` or `dynamic` for a declaration with no static name (no dialect kind maps them, so they are excluded by kind). A module file's fields come from `cue export` (the binary's word on each key) at the line this program's own lexical scan finds its label (the server lists no symbol for a module file) |
| Calls | `textDocument/definition` at every identifier a lexical scan of the file proposes (`lex.go`: comments and plain string text skipped, interpolations `\(` and `\#(` scanned, labels, aliases, let and loop variables, keywords and predeclared names not asked) | (file, innermost enclosing field with a static name, each declaration the server answers): a reference to a field or definition. A declaration that is no field (an import, a let, an alias's own name, a loop variable, a package clause) is none; a reference with no enclosing field is counted on stderr |
| Cross references | the same answers whose declaration is in another file of the subject; `textDocument/definition` at an import's path (at its alias when it has one) gives the package clauses of the files it loads | `cross_references` (entity granularity: the import entity, named by the path, to `package/<name>`; a field to the declaration) and `imports` (file, line, file it loads). The standard library and modules outside the subject are counted on stderr |

Names are compared as the harness compares them (§1.4): a definition's name is the label path's
last segment with a definition's `#` dropped (BareName's rule); a call's names are full label paths,
reduced by the harness; a cross reference's names are full (they compare whole).

Limits, disclosed in the reading: the scan proposes positions only, so a reference the scan misses is
missing from the truth (none known: the tests in `main_test.go` pin the string forms); the server
answers what CUE's evaluation settles, so a reference the language server cannot place (a field of a
list element reached by index after a comprehension) is in no side's truth.
