# vimscript — provenance

- Grammar: `tree-sitter-grammars/tree-sitter-vim v0.8.1` (`3092fcd99eb87bbd0fc434aa03650ba58bd5b43b`, MIT, Thomas Vigouroux 2020; licence
  checked at the pin: the top-level `LICENSE` only, `queries/` and `src/` carry none), ABI 15, external scanner, built into
  `internal/extractor/tsengine/grammars/vim.wasm.gz` (1,177,166 bytes raw, 212,656 gzipped, 487 peak module pages on vim-fugitive).
  `src/parser.c` (4.7 MB) ships at the tag, so the lock says `"generate": false`; the scanner compiles with one
  `-Wreturn-type` warning (`lex_string`'s unreachable `default: assert(0)`), which does not affect the blob. Upstream `master` is two
  commits past the tag (a regenerated parser and a CI change); its head reads the same 1 of 1 and 2 of 6 files with an error.
- **The dialect is refused (design §14.2, `LB-Q-vimscript-rest`).** 3 of the 7 calibration files (42.9%) parse with an ERROR node, over the
  5% line: vim-plug's `plug.vim` (the whole file is one ERROR: `s:update_python` embeds a Python heredoc, `execute py_exe "<< EOF"`),
  vim-fugitive's `autoload/fugitive.vim` (a bare `-` ex command at line 4674) and `plugin/fugitive.vim` (the unary-plus string
  `+"<range>"` at line 564). The dialect still finds 565 of the 568 definitions ctags reports (sym_recall .9947); the 3 it misses are the
  ones inside those regions (`s:update_python`, and the commands `G` and `Git`). `refusedDialects` (`internal/fusion/pipeline.go`) keeps
  `.vim` and the `vimrc` basenames away from the dialect and `.vim` stays disclosed in `unsupported_languages`;
  `TestDialectGoldens/vimscript` and `TestIngest_VimscriptIsRefusedAndDisclosed` keep it built and refused. Re-open it when a
  tree-sitter-vim commit parses both subjects with at most 5% of files in error, then dispatch it (an `ExtractorEpoch` bump).
- `tags.scm` is original: the grammar ships `folds`, `highlights` and `injections` only, and neither aider's nor nvim-treesitter's
  collections carry a Vim script tags file. It tags `function_definition` (`VimFunction`, `function`/`func`/`fu`, any name the declaration
  writes: `Foo`, `s:Foo`, `foo#bar#Baz`, `Dict.method`), `command_statement` (`VimCommand`, the `command_name`) and `augroup_statement`
  (`VimAugroup`; `augroup END` closes a group and `augroup! Name` deletes one, neither defines). A function nested in a function or an
  `if` block is a definition, as ctags has it. `@body` is the first node of the body (a function with no body signs its first line).
  `def ... enddef` (Vim9 script) is not parsed by this grammar release (`unknown_builtin_statement`), so it is not extracted; ctags tags it
  as a function, and neither calibration subject uses it.
- Names: the comparison mode is `identity`, decided against `bare`: ctags names a script-local function `s:Foo` and the harness compares
  the oracle's name whole, so a dialect name reduced to its last segment (`Foo`) would never pair with it. Whole names also suit the
  language: `call s:Foo()` and `function! s:Foo()` are one key. `names.strip_prefix` drops `g:` and `<SID>` from a definition and from a
  callee, because ctags does (`function! g:Global()` is `Global`, `function! <SID>Foo()` is `Foo`; `g:` is the global scope a bare name
  already has). Neither subject writes a `g:` or `<SID>` function, so that decision is checked by the manifest test and
  `testdata/functions.vim` only.
- Calls: `call f()`, `f()` in an expression, `s:f()`, `a#b#f()`, `Dict.method()`, `a:obj.child.method()` and `obj->Method()`, each named as
  written. A callee that is a call itself (the grammar reads `Foo() . Bar()` as `Foo().Bar()`, so the pattern accepts a field access only
  on a name), a lambda or a subscript has none. A call belongs to the smallest function around it; a call in a mapping, command or
  autocmd replacement is a string in this grammar and is not extracted.
- Not extracted: variables (`let g:x`, `const s:X`; ctags tags 93 on the subjects, excluded from the kind intersection as
  `ctags:variable`), mappings (21, `ctags:map`) and `n` filename tags. They are outside the L1 kinds the queue names (functions, commands,
  autocmd groups).
- Classification: `.vim` and the basenames `vimrc`, `.vimrc`, `_vimrc`, `gvimrc`, `.gvimrc`, the ones universal-ctags 6.2.1 maps
  (`vimrc [._]vimrc gvimrc [._]gvimrc *.vim *.vba`); `.vba` (vimball) is not claimed.
- `testdata/`: six fixtures written in this repository (functions, commands, augroups, calls, a plugin file, one with a syntax error);
  goldens written by hand from each fixture before any query existed (`TestDialectGoldens/vimscript` red), then the queries were written
  against `indexbench lang-probe`; they passed on the first run.

## Measurement (docs/bench/readings/lang/vimscript.json)

vim-plug `0.14.0` (1 file) and vim-fugitive `v3.7` (6 files), universal-ctags 6.2.1 (tier B): 568 oracle definitions and 565 dialect
definitions in the kind intersection (function, command, augroup), 565 matched: sym_recall .9947, sym_precision 1, kind_agreement 1,
range_agreement 1 (546 of 546 with an oracle end), signature_coverage 1 (547 callables), 2.34 MB/CPU-s, 0 faults, 487 peak module
pages: L1 excellent, measured, and refused above. The subjects are small (11,841 lines); the licence of vim-fugitive is "the same terms as
Vim itself", which is not OSI-approved, and is flagged for review.
