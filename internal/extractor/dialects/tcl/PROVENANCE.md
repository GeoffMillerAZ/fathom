# tcl — provenance

- Grammar: `tree-sitter-grammars/tree-sitter-tcl`, commit `850a72ab6436e06645b33b11cfa60cbdb04b1f01` (upstream `main`'s head on
  2026-09-29; the repository has no release tag, so the lock's `tag` is the commit, as kotlin's and swift's are), MIT (Lewis Russell
  2022; licence read at the pin: the top-level `LICENSE` only, `queries/` and `src/` carry none), ABI 15, external scanner, built
  into `internal/extractor/tsengine/grammars/tcl.wasm.gz` (203,872 bytes raw, 74,046 gzipped, 468 peak module pages on tcllib).
  `src/parser.c` (454 KB) ships at the commit, so the lock says `"generate": false`. Pin and digests in
  `internal/extractor/tsengine/grammars/grammars.lock.json`.
- **The dialect is refused (design §14.2, `LB-Q-tcl-rest`).** 516 of the 827 calibration files (62.4%: tcllib 438 of 704, Tk 78 of 123;
  plus one tcllib file, the 1.3 MB `fumagic/filetypes.tcl`, on which the parser traps) parse with an ERROR node, over the 5% line. The
  grammar reads every braced word as a script, so what Tcl writes inside braces that is not a command sequence stops it: an array
  element (`$psglyphs($hexcode)`, `set state($k) v`; the first error of 207 files is a `(`), a `!flag` word (`instate !disabled`, 22), a
  stray `;` (11) or a `set $` word (10). Over every `.tcl` file of the two clones (tcllib 796, Tk 123) the head reads 62.2% and 63.4%; two
  older commits read 62.7% and 64.2% (98015ebe, 2024-10-04) and 64.7% and 69.1% (4ac1e729, 2024-09-05), so the head is the best pin
  measured (the only branch ahead of `main`, `native-dsl-translation` 0ff435e5, builds the identical blob). `refusedDialects` (`internal/fusion/pipeline.go`) keeps `.tcl`, `.tk`
  and `.tm` files away from the dialect and `.tcl` stays disclosed in `unsupported_languages`; `TestDialectGoldens/tcl` and
  `TestIngest_TclIsRefusedAndDisclosed` keep it built and refused. Re-open it when a tree-sitter-tcl commit parses both subjects with at
  most 5% of files in error, then re-measure and dispatch (an `ExtractorEpoch` bump).
- `tags.scm` is original: the grammar ships `folds`, `highlights` and `indents` only, and neither aider's nor nvim-treesitter's
  collections carry a Tcl tags file. It tags `namespace eval NAME { ... }` (`TclNamespace`, a container), `proc NAME ARGS BODY`
  (`TclProc`, wherever the grammar parses one: a namespace body, an `if` block, another procedure), and in `oo::class create NAME { ... }`
  the class (`TclClass`, a container) and each direct `method NAME ARGS BODY` (`TclMethod`). `constructor` and `destructor` are not
  extracted (ctags has no kind for either). A name built at run time (`proc $x`, `namespace eval $ns`), a braced or quoted proc name and a
  class named at run time have no simple word to record and are skipped. `@body` is the body word, so a signature stops at the arguments.
- Names: `qualify.scope_sep` and `container_sep` are `::`, so a leading `::` is dropped and a relative name qualifies by its container:
  `namespace eval ::app { proc run ... }` is `app::run`, `proc ::app::run` at the top of a file is `app::run` too, and a call
  `::app::util::trim` is `app::util::trim`. Known limit: an absolute `proc ::x::y` written inside `namespace eval a` is read as
  `a::x::y`. The comparison mode is `bare` (the harness compares last segments); see the measurement below for why that is the only
  mode that fits ctags's procedure names and what it costs on namespaces.
- Calls: every `command` whose name is a plain word (`helper`, `::ns::cmd`, `string`, `puts`). `variable`, `global` and `upvar`
  declare names and `method` defines one, so none is a call. `set`, `if`, `foreach`, `while`, `try`, `catch`, `expr` and `namespace` are
  their own nodes in this grammar, not commands. A callee spelled with a variable or a substitution (`$obj method`) has no name. A call
  belongs to the smallest procedure or method around it. Two limits the goldens record: the grammar also reads a braced argument list
  (`method feed {animal}`, `{prefix ""}`) and a `switch` arm label as a command, so those words come out as calls of the method or
  procedure around them.
- Classification: `.tcl`, `.tk` and `.tm`. The universal-ctags 6.2.1 Tcl map is `*.tcl *.tk *.wish *.exp`; the manifest's `ctags.args`
  adds `--map-Tcl=+.tm` so the oracle reads the same files fathom does. `.tm` is also TeXmacs's extension (a TeXmacs document would parse
  with errors and yield nothing); `.wish` and `.exp` (Expect) are not claimed. No `tclsh`/`wish` shebang is claimed: `lang-oracle`'s file
  set is by extension, so an extensionless script would be extracted in production and never measured.
- `testdata/`: five fixtures written in this repository (procs, namespaces, calls, TclOO, one with a syntax error); goldens written by
  hand from each fixture before any query existed (`TestDialectGoldens/tcl` red), then the queries were written against
  `indexbench lang-probe`.

## Measurement (docs/bench/readings/lang/tcl.json)

tcllib `tcllib-2-0` (`modules/`, 704 files) and Tk `core-9-0-4` (`library/`, 123 files), universal-ctags 6.2.1 with `Tcl` and `TclOO`
(tier B), 8,882 oracle definitions and 9,589 dialect definitions in the kind intersection (namespace, function, class, method), 7,147
matched: sym_recall .8047, sym_precision .7453, kind_agreement 1, range_agreement .9703 (6,631 of 6,834 with an oracle end),
signature_coverage 1 (8,727 callables), 3.00 MB/CPU-s, 1 fault, 468 peak module pages. L1 is unmet (recall under .85, precision under
.90, faults over 0.1% of files). Where the difference comes from, both subjects together:

- 759 of the 763 oracle-only namespaces and classes (711 + 52) and 789 dialect-only ones (725 + 64) are one disagreement: ctags names
  `namespace eval ::struct::list` and `oo::class create ::tcl::chan::core` with the whole path written (`::struct::list`), and the harness
  reduces the dialect's name to its last segment (`BareName`, `list`) but compares the oracle's name as it is (`ScoreL1` calls
  `MatchName` on the dialect's names only). A ctags procedure name is already its last segment, so `bare` is the only mode that matches
  procedures (`identity` would fail 6,122 of 8,364 proc lines, all those written `proc ::a::b`). Applying `MatchName` to the oracle's
  names as well would pair them (a scratch run of that one-line change, not a reading and not committed: 8,253 matched, sym_recall .9292,
  sym_precision .8607, range_agreement .9715); the remaining shortfall is the next two bullets.
- 483 oracle-only procedures have a bare-name separator in their name (`.PP`, `!`, `<<<`, `J1/2`) and 418 are plain names in files whose
  ERROR recovery lost them (`ftp.tcl`, `crc16.tcl`, `sha256.tcl`, ...).
- 1,527 dialect-only procedures are real definitions universal-ctags does not tag: it enters `namespace eval` and `oo::class` bodies
  only, so a procedure inside an `if` block (`if {[package vsatisfies ...]} { proc ::asn::asnGetByte ... }`, asn.tcl), another
  procedure or a `catch` is missing from the oracle.
