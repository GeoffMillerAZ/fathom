# compile-commands — provenance

LB-Q-compile-commands, design `docs/design/language-breadth.md` §4 and §13. A new document-engine tool format (no
incumbent, no grammar): a Clang JSON compilation database (https://clang.llvm.org/docs/JSONCompilationDatabase.html)
was claimed by the generic JSON extractor, which reads it as `JSONKey`s and nothing else.

- Engine: the doc engine, with one new decoder, `decode: "compile-commands"` (`internal/extractor/docdecode_compdb.go`,
  plan row LB-F127): JSON decoded as `decodeJSONDoc` does, then three keys added to every entry from its flags, because
  `command` is one string (or `arguments` one array) and a selector cannot read a flag out of it: `includes`, `defines`,
  `out`. The flags are read the way `internal/fusion.includeRoots` (`resolvestrategy.go`) reads the same file: the
  `arguments` array, else the `command` split on whitespace, a surrounding quote pair trimmed; `-I` joined or separate,
  `-iquote`/`-isystem`/`-idirafter` separate only. `-D` is read like `-I` (joined or separate; a joined macro keeps
  its own quotes), the object is the `-o` argument, else the entry's `output`. `TestCompileCommandsFlagGrammarIsIncludeRoots`
  holds the two readers together; a joined `-isystem/dir` (valid for clang) is read by neither, and the oracle reads it
  (O376 for item cinc).
- Entities: one `CompileUnit` (canonical `step`) per entry, named by its `file` as written, at the entry's `{` line,
  ending at its `}`; signed by the entry's `output`. Two entries for one file are two units.
- References: each unit's include directories, macros and output, each written the flag and its argument together
  (`-I/usr/include`, `-DNDEBUG`, `-oa.o`) so that no reference reads as the name of a symbol elsewhere in the index.
- Not modelled: `directory` (a path on the machine that generated the database), `compile_commands.json` found only
  under a build tree (excluded by `.fathomignore`'s CT-8 default: ingest reads a root database), `*.rsp` response files
  and the `command` of a wrapper (`ccache cc ...`) whose flags follow the compiler.
- `testdata/`: fixtures written in this repository, each golden written by hand from reading the fixture before the
  decoder was final (`command-form`, `arguments-form`, `repeats`, `output-flag`, `empty`, `odd-entries`, `broken`).
- **Measurement (docs/bench/readings/lang/compile-commands.json, calibration/corpus.d/compile-commands.yaml):** see the
  reading's `disposition`.
