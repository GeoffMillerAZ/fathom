# vhdl — provenance

- Grammar: `jpt13653903/tree-sitter-vhdl v2.0.3` (MIT), commit `a7cf493d07dbcf52ca287596451ae3fb8b12099f`, ABI 15, external
  scanner, `src/parser.c` committed, grammar at the repository root (`tree_sitter_vhdl`), built into
  `internal/extractor/tsengine/grammars/vhdl.wasm.gz` (1,314,192 bytes raw, 240,100 bytes gzip). The grammar is "simplified
  by design": it parses to a coarse tree (an identifier that names a standard library type, function or constant becomes a
  `library_type`, `library_function` or `library_constant_*` node) rather than the full VHDL-2008 syntax. Pin and digests in
  `internal/extractor/tsengine/grammars/grammars.lock.json`. Licence read at the pinned commit: the top-level `License.md`
  (MIT, John-Philip Taylor 2024); the repository ships no licence file under `queries/` or `src/`.
- `tags.scm`: the grammar ships no `tags.scm` (its `queries/Neovim` and `queries/Helix` hold highlight, fold, indent, context
  and text-object queries), so the file is drafted from the node shapes those queries name (the `entity:`, `architecture:`,
  `package:`, `function:`, `procedure:`, `component:` and `type:` identifier fields, `label_declaration`,
  `subprogram_head`) and grown against the grammar's tree with `indexbench lang-probe`. It follows universal-ctags' VHDL
  parser.
- Kinds and naming. `VhdlEntity` (an interface: canonical `interface`), `VhdlArchitecture` (named by its own identifier, as
  ctags names it: `rtl`, not the entity's), `VhdlPackage` and `VhdlPackageBody` (a package and its body, one file each or
  both in one), `VhdlProcess` (a labelled process only; an unlabelled one has no name), `VhdlFunction` and `VhdlProcedure`
  (a body) with `VhdlFunctionSpec` and `VhdlProcedureSpec` (a declaration, a "prototype" to ctags), `VhdlType`,
  `VhdlSubtype`, `VhdlRecordField` (an element of a record type, one per name; ctags calls the kind `record`) and
  `VhdlComponent`. A package, entity or architecture qualifies what it contains (`rtl.tick`, `counter_pkg.next_count`); a
  subprogram nested in another subprogram or a process, a type, a record field and a component keep their bare name.
- Signature and range. A body's signature stops at its `is` (`subprogram_head`), a declaration's at its `;`, a process'
  before its declarative part or its `begin`: `tick : process (clk, rst)`. Every definition carries an `EndLine`; ctags
  writes one for an entity, an architecture, a package, a process, a subprogram body and a record type, so
  `range_agreement` is measured.
- Not extracted, on purpose: ports, generics, signals, variables, constants and aliases (ctags tags them, the dialect leaves
  them out and the reading counts them under `excluded_by_kind`), an unlabelled process, a component instantiation, a
  generate statement, a configuration or context declaration, a protected type's methods as members (they are
  declarations inside a type, so they keep their bare names), `library` and `use` clauses (L3).
- Calls (L2, not measured for this row): a `procedure_call_statement` named by its callee, and a `name(...)` inside an
  expression. VHDL's syntax cannot tell a function call from an index or a conversion, so `a(i)` is recorded as a call to
  `a`; a name that resolves to no callable draws no edge. The target of an assignment (`a(i) <= x`) is not an expression
  and is left out.
- Oracle (calibration/oracles/README.md): universal-ctags 6.2.1, tier B, run with `--kinds-VHDL=+Cd` (component and
  prototype are off by default). It writes an end line for an entity, an architecture, a package, a process, a subprogram
  body and a record type, so L1 can grade excellent; it agrees with the dialect on 99.5% of the definitions in the kind
  intersection.
- Syntax errors: 5 of 399 calibration files (1.25%) parse with an ERROR node (VUnit's `fifo.vhd`, `tb_axi_stream.vhd`,
  `custom_types.vhd`, `com_messenger.vhd` and `AlertLogPkg.vhd`); the definitions around the error are still found
  (`testdata/broken.vhd`).
