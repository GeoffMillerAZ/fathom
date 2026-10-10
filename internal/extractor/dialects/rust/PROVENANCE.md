# rust — provenance

- Grammar: `tree-sitter/tree-sitter-rust v0.24.2` (MIT), commit `77a3747266f4d621d0757825e6b11edcbf991ca5`
  (the tag the incumbent CGO binding pins), ABI 15, external scanner, grammar at the repository root
  (`tree_sitter_rust`), built into `internal/extractor/tsengine/grammars/rust.wasm.gz` (841,090 bytes raw,
  204,421 bytes gzip). Pin and digests in `internal/extractor/tsengine/grammars/grammars.lock.json` (that
  directory's `PROVENANCE.md` regenerates its own tables from it). Licence read at the pinned commit: the
  top-level `LICENSE` (MIT); the repository ships no licence file under `queries/` or `src/`.
- `tags.scm`: started from tree-sitter-rust's own `queries/tags.scm` (MIT) and rewritten around the incumbent
  `rustextract.go`'s kinds. Upstream tags a struct, enum, union and type alias as `@definition.class`, a trait as
  `@definition.interface`, any function in a `declaration_list` as `@definition.method`, an impl block only as a
  `@reference.implementation`, has no const, static or `@body`, and tags a call only for a plain name, a field call
  and a macro. This file keeps the incumbent's own `SourceKind` strings (`RustFn`, `RustStruct`, `RustEnum`,
  `RustTrait`, `RustImpl`, `RustMod`), makes an impl block a definition named by the head type it implements (so
  its methods qualify to that type as `Type.method`, the incumbent's contract), and adds `RustUnion`,
  `RustTypeAlias`, `RustConst`, `RustStatic` and `RustMacro` (a `macro_rules!` definition).
- Naming, kept from the incumbent (§11 step 3, port parity): an impl method is `Type.method`; a function in an
  inline `mod` keeps its bare name (a module is not a qualifying container); an impl block is a `RustImpl`
  symbol named by its type, so a struct and its impl blocks share a name. Different from it: the head type's own
  name is used where the incumbent kept the `type:` field's source text (`impl<T> Foo<T>` is `Foo`, not `Foo<T>`;
  `impl net::tcp::Conn` is `Conn`, `impl Trait for &str` is `str`); a type with no name of its own (a slice,
  tuple, `dyn Trait`) still keeps its text (`[u8]`); a trait's methods, default and required, are `Trait.method`
  (the incumbent never entered a trait body); a nested function, a bodiless `extern` function and a function-local
  `const`/`static` are definitions. Every callable carries a `Signature` (its source up to the body, whitespace
  collapsed; a bodiless signature ends at its `;`) and an `EndLine`.
- Members (LB-F82): a struct's or union's named field, an enum's variant, a struct-like variant's field and an
  associated const or type of an impl or trait are definitions, qualified `Owner.name` like a method (`Config.name`,
  `Shape.Rect`, `Source.LIMIT`, `Config.Item`; kinds RustField, RustVariant, RustAssocConst, RustAssocType,
  `internal/core/kinds_rust.go`; canonical field, enum_member, field, field as rust-analyzer's scoped kinds map).
  Struct, union and enum are containers; a variant is a callable-qualified member, so a struct-like variant's field is
  bare (`w`) here and `Rect.w` in the CGO walker (a variant is not a container, the engine does not chain). A tuple
  field is not extracted by either: rust-analyzer's index defines none for a crate's own tuple struct. This moved
  `def_mapping` .692 -> .8988 (the reading). Still not extracted: local variables and parameters, and what a macro
  expands to (687 definitions across the two subjects, disclosed).
- Calls: the callee is the called name; a scoped path (`Foo::new()`, `crate::a::b()`, `std::mem::swap()`) is
  captured whole and the manifest's `qualify.scope_sep: "::"` spells it `Foo.new`, the key of the method it names;
  a path with a generic segment (`Vec::<u8>::new()`, `<T as Trait>::f()`) keeps its last name. A capitalised callee
  (`Some(x)`, `Wrapper(y)`, `Shape::Circle(1.0)`) is a tuple-struct or enum-variant constructor, not a function
  (rustc's naming lints make the case reliable) and is not a call. A macro invocation (`println!`, `vec!`, a
  project's own macro) is a call by the macro's name, as the incumbent's contract records (`greet` calls `helper` and
  `println`). A macro's arguments are a flat `token_tree`, so a call written inside `assert_eq!(f(x), ..)` or
  `format!("{}", v.len())` is an identifier followed by a parenthesised token tree; the last pattern of the file
  reads that shape. Its known false reading: tree-sitter anchors skip anonymous tokens, so `a, (b, c)` and
  `n * (m + 1)` inside a macro read as a call of `a` and of `n`. No shape here reaches an operator: `a == b`, `?`, `!x`,
  `*x` and `a[i]` resolve to trait methods in SCIP but are written as no call (see the reading).
- `imports.scm`: written in this repository (upstream has none; the incumbent never resolved an import).
  `mod x;` is a whole import of the child module `x`; `use a::b::c;` is two specs, a named import of `c` from `a::b`
  and a whole import of `a::b::c` bound to `c` (a path's last segment may be an item or a module; the strategy
  resolves the deepest module file the path names, so the wrong reading resolves nothing extra); `as` binds the alias;
  `use a::b::*;` is a wildcard; a group `use a::{b, c as d, self};` is a named import of the prefix per identifier
  member. `imports.strategy: crate-mod` (`internal/fusion/resolvestrategy.go`'s `resolveCrateMod`, already
  tested) resolves `crate::`, `self::`, `super::`, a workspace member's crate name and a child module. Not
  resolved through nested groups (LB-F83): a group's list is a scope record (`@import.scope` with its path as
  `@import.prefix`, no spec of its own) and every member inside takes the prefixes of all the lists around it,
  outermost first (`use a::{b::{c, d}, e}` is `a::b` for c and d, `a` for e; the engine composes them,
  `tsimports.go` composeImportPrefix). The residue is the resolver's, listed in the reading's notes.
- `references.scm` (LB-F74, wave lc): written in this repository; no upstream query adapted (type names, a scoped
  type `m::Bar`, a scoped path `Foo::new` and `X::CONST`, kept when a `use` binds the head or, under a glob `use`,
  when it starts upper case). A `pub use` re-export is not captured yet (O289).
- `testdata/`: fixtures written in this repository. `shapes.rs` every kind (nested modules, unit, tuple and generic
  structs, an enum, a union, a trait with required and default methods and associated items, inherent, trait,
  reference, slice and path impls, a multi-line signature, async, const and unsafe extern functions, an `extern`
  block, a nested function, a local const and struct, a `#[cfg(test)]` module); `calls.rs` every call shape (plain,
  scoped, `Self::`, method, chained, turbofish, `<T as Trait>::`, closures, `.await`, macros and calls inside a
  macro's arguments) and what is not a call; `generics.rs` lifetimes, trait objects, generic and trait impls in a
  module and in a function; `imports.rs` each `use` and `mod` form; `broken.rs` a syntax error the parser must
  survive (`good` and `survivor` extract either side of it). The `shapes.rs`, `calls.rs` and `generics.rs`
  symbol lists were written by hand from reading the fixtures before the query was run against them and matched
  the engine's output entry for entry (41, 16 and 25 symbols; `shapes.rs` gained its local `SCRATCH` const and `generics.rs`
  its slice and double-reference impls, five symbols, by hand when those shapes were added); `imports.rs`'s specs were derived the same way; `broken.rs` was generated and
  reviewed line by line.
- **Measurement (docs/bench/readings/lang/rust.json, calibration/corpus.d/rust.yaml):** a rust-analyzer 1.98.1 SCIP
  index (tier A, `calibration/oracles/rust-analyzer`; `cargo.features = "all"`) for every level, on
  BurntSushi/ripgrep 14.1.1 and clap-rs/clap v4.6.1 (427 files, 4.2 MB). `level_met` is L2, both L1 and L2
  excellent: sym_recall .9986, sym_precision .9784, kind_agreement 1, range_agreement 1, signature_coverage 1,
  call_recall .9593, call_precision .9846; 2.2 MB/CPU-s, 0 faults, 461 peak module pages. Four rules of the
  instruments are Rust's own and are counted in the reading: a definition rust-analyzer finds inside a macro's
  expansion (globset's `syntax!(literal1, ..)`; 687 of the 8,414 definitions in the L1 kind intersection) is written nowhere, so it is left out of L1
  (`scip:macro-generated`); a file module's one definition is its whole file, so its declaration is recovered from
  the reference after `mod` (`mod x;`); a macro invocation is a call (rust-analyzer references it by the macro's
  name); a callable reference whose source range is not the callee's name — an operator resolved to its trait
  method: 5,887 references — is not a written call site and is left out of the L2 truth
  (`scip_calls_unwritten`). L2 against the raw truth (operators counted, macros not) reads .864 recall and .873
  precision on ripgrep's default-feature index, L2 good. `impl` blocks (960) have no SCIP definition and are outside the
  L1 kind intersection; an unscored universal-ctags 6.2.1 cross-check over the same files (it writes no end line
  for Rust, so it can only grade good) reads sym_recall .9927, sym_precision .9775, kind_agreement 1, impl blocks
  and modules included (8,647 of 8,711 oracle definitions matched). `docs/plans/language-breadth.md`'s
  `LB-Q-rust-rest` records what stays open.
- **Merge gate (§11 step 2):** the target (L4) is not met, so the CGO incumbent (`rustextract.go`) stays fusion's
  live `.rs` dispatch (`internal/fusion/pipeline.go`); the dialect is written, tested and measured but not switched
  in. `lang-parity` against it: sym_recall .8934 -> .9927, sym_precision .9638 -> .9775, signature_coverage 0 ->
  1, call_recall .8172 -> .9593, call_precision .9339 -> .9846, no kind lost; kinds gained: RustConst,
  RustMacro, RustStatic, RustTypeAlias (RustUnion when a subject has one). It is registered for imports only:
  `fileResolveLangFor(".rs")` selects it, so an ingest's file-resolved pass resolves `mod` and `use` statements
  through crate-mod (the pass reads symbols from the store and imports from the dialect). The CGO walker emits the
  same members (LB-F82) and is what ingest dispatches; the measured numbers are the dialect's. A switch flips
  `pipeline.go`'s line, deletes `rustextract.go` and its test, and bumps `ExtractorEpoch`: impl symbols are named by
  their head type instead of the type field's source text and trait methods, unions, aliases, consts, statics and
  macros are new symbols.
- **Dispatch (ADR-048 amendment 2026-10-01, D1), 2026-10-02:** lang-parity against `rustextract.go` on the reading's
  own rust-analyzer indexes (`-scip-defs`, `docs/bench/readings/lang/parity/rust.json`) refuses the switch on
  sym_precision alone, 0.9746 < 0.9756; every other L1 and L2 metric is level or ahead. Of the 38 definitions only the
  dialect leaves unpaired, 16 are this file's own duplicates: a `const` or `type` inside an inline `mod { }` is
  captured both as a module item (`RustConst`, `RustTypeAlias`) and as an associated item (`RustAssocConst`,
  `RustAssocType`), because a `mod` body is a `declaration_list` like an `impl` body; capturing it once lifts the
  dialect to 0.9761 (9,964 of 10,208) and the parity reads MERGE. The other 22 are real items under a cfg
  rust-analyzer does not analyze (21) and a raw identifier the harness spells without `r#` on one side (1)
  (O456). `rustextract.go` stays fusion's live `.rs` dispatch until lang-parity reads MERGE.
- **Dispatched, and the walker retired, 2026-10-10 (wave lf3, K25 rulings 3 and 4, LB-F388, LB-F390):** with the
  harness resolving a symbol defined in more than one document to every one (ruling 3, O539) lang-parity reads MERGE
  at L4 against `rustextract.go` (`docs/bench/readings/lang/parity/rust.json`: sym_recall .8966 -> .9984,
  file_edge_precision .9971 -> .9972, def_mapping .8938 -> .9984, typed_recall .9092 -> .9966, no kind lost), so
  `internal/fusion/register.go` serves `.rs` from this dialect and `rustextract.go` and its test are deleted: the
  walker's own contracts are the `contracts.rs` and `empty.rs` goldens here. Extraction costs 2.68-2.84 MB/CPU-s on
  ripgrep and clap (4.24 MB) against the walker's 6.87-6.92, above the 1 MB/s floor (K25 ruling 2). The reading reads
  every level excellent once a module path is a file edge (ruling 4, `internal/fusion/rustcrate.go`
  `rustModuleEdges`) and a module is a namespace at L4 (K18). Every `.rs` file's extraction changes (the integrator's
  `ExtractorEpoch` bump covers it).
