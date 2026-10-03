# rust-analyzer scip — tier-A Rust index (definitions, calls, def_mapping, typed_recall, file edges)

Installed without sudo into a scratch dir (nothing global; rustup, cargo's registry and the build target all live
there and are deleted after measuring):

    export RUSTUP_HOME=<scratch>/rustup CARGO_HOME=<scratch>/cargo CARGO_TARGET_DIR=<scratch>/target PATH=<scratch>/cargo/bin:$PATH
    curl -sSf https://static.rust-lang.org/rustup/dist/aarch64-apple-darwin/rustup-init -o <scratch>/rustup-init && chmod +x <scratch>/rustup-init
    <scratch>/rustup-init -y --profile minimal --no-modify-path --default-toolchain stable -c rust-src   # rustc/cargo 1.98.1
    rustup component add rust-analyzer                                                                  # 1.98.1 (48a229ce 2026-09-01)

`rust-src` is what lets rust-analyzer resolve the standard library, so a call on an iterator adapter's closure
argument still reaches the project's own function. The `minimal` profile is about 560 MB unpacked.

Run (2026-09-28, both clean; `cargo metadata` fetches each subject's dependencies into `CARGO_HOME`, 120 MB for the
two):

    echo '{"cargo":{"features":"all"}}' > <scratch>/ra-all.json
    rust-analyzer scip workspace/calibration/rust/ripgrep --config-path <scratch>/ra-all.json --output workspace/tmp/scip/rust-ripgrep-all.scip   # 6 s
    rust-analyzer scip workspace/calibration/rust/clap    --config-path <scratch>/ra-all.json --output workspace/tmp/scip/rust-clap-all.scip      # 35 s
    indexbench lang-oracle -dialect rust -subject <clone> -subject-name <owner/repo> -scip <index> -scip-defs -out docs/bench/readings/lang/rust.json

`cargo.features = "all"` matters: with the default features rust-analyzer does not analyze feature-gated modules,
and every definition in them is a dialect-only miss (clap: 262 documents against 299 with all features). Code under `#[cfg(windows)]` and the `tests/derive_ui` compile-fail files are still outside it — the
remaining dialect-only definitions (48 on ripgrep, 212 on clap after LB-F82), all real definitions in the source.

What the index carries: `SymbolInformation.kind` on every symbol (Struct, Enum, Trait, Function, Method,
StaticMethod, TraitMethod, Macro, Module, Constant, StaticVariable, TypeAlias, AssociatedType, Field, EnumMember,
plus locals: Variable, Parameter, SelfParameter, TypeParameter), an `enclosing_range` on every definition (15,892 of
15,892 on clap), one definition per source item at its name, and references to macros by the macro's name. Four
things about its symbols the harness reads for Rust (`internal/langbench/scip.go`):

- An impl block's members are `impl#[Type]name().` or `impl#[Type][Trait]name().`: `]` closes a descriptor, so the
  leaf is the name and the parent is a type (`scipBareName`, `scipDescriptorKind`, `internal/scip`'s
  `descriptorLeafName`).
- It expands macros, so it defines items no source line spells: globset's `syntax!(literal1, ..)` defines a test
  function `literal1`, 687 of them across the two subjects. `SCIPDefs` marks a definition whose position has no
  declaration keyword before it `Generated`, and `ScoreL1` leaves it out and counts it
  (`excluded_by_kind: scip:macro-generated`). No syntactic extractor can find one.
- A file module (`mod x;`) has one definition, its whole file (a multi-line range at `0:0`), and the `crate/` root
  has one too. `SCIPDefs` drops every multi-line range and recovers a file module's declaration from the reference
  written after `mod`.
- It resolves an operator to its trait method at the operator's own range — `?` to `branch`, `a == b` to `eq`,
  `!x` to `not`, `*x` to `deref`, `a[i]` to `index`, `a + b` to `add`, 5,887 references across the two subjects —
  and references a macro invocation by the macro's name. `SCIPOptionsFor("rust")` counts the macro as a call and
  leaves out a callable reference whose source range is not the callee's name (`scip_calls_unwritten`); L2 is call
  sites, and none is written for an operator. L4 is touched by one Rust-only rule (wave lx, `internal/langbench/scip_rust.go`): a definition no source line writes, and the references inside it, leave `def_mapping` and `typed_recall` (counted, below); every other SCIP
  definition and reference counts.

It records no `impl` definitions (960 `RustImpl` symbols are outside the L1 kind intersection, disclosed under
`excluded_by_kind`), reports fields, enum variants and associated types as definitions the dialect does not extract
(`def_mapping` .692), and gives locals a definition (21% of the 151,764 references target one, and no extractor maps
a local, which bounds `typed_recall` at about two thirds whatever an extractor emits).

Re-run 2026-10-01 (wave la re-measure, fathom d8ed9a29): Homebrew `rust-analyzer 0.0.0 (03fcb77246 2026-09-27)`
and `cargo`/`rustc 1.98.1 (48a229ce 2026-09-01)` instead of rustup (the formula's sysroot carries `rust-src`, so the
standard library still resolves), no install into a scratch directory. The recipe above ran unchanged with
`CARGO_HOME=<toolcache>/cargo` (246 MB after both subjects) and `CARGO_TARGET_DIR` on the external volume (303 MB):
ripgrep 12.4 s (7.4 MB index, 18 `Duplicate symbol` warnings), clap 27.4 s (11 MB, 712). `lang-oracle -scip -scip-defs`
ran against each real subject root and re-read every L1, L2 and L4 metric and count identically (sym_recall .9986,
sym_precision .9784, call_recall .9593, call_precision .9846, def_mapping .692, typed_recall .5222), so the Homebrew
build is a drop-in for the rustup one and no met level moved. What is new is L3 `import_resolution` (computed from the
index's references on `use` and `mod` lines since LB-F40), .4725: 635 of 1,344 in-repo imports resolve to the file the
index names (ripgrep 115 of 251, clap 520 of 1,093), 543 resolve to none and 166 only to another file; 427
import-line references are left out because their symbol is defined in more than one file. typed_recall predates F-A
(item scip: the `ImportIndex` line-span fallback and local references out of `typed_recall`) and is re-run after that
merge.

Re-run 2026-10-01 (wave lb remeasure, fathom fdd2bb59, same Homebrew `rust-analyzer 0.0.0 (03fcb77246 2026-09-27)`): ripgrep
12.0 s, clap 35.7 s, the recipe unchanged. typed_recall reads .671 (53,089 of 79,123 in-index edges) now that LB-F55 takes the
22,443 references to a local out of its denominator; L1, L2 and L3 read as at d8ed9a29.

Re-run 2026-10-01 (wave lc, fathom b750ed19, the same Homebrew `rust-analyzer 0.0.0 (03fcb77246 2026-09-27)` and
`cargo`/`rustc 1.98.1`): `cargo.features all`, `CARGO_HOME` and `CARGO_TARGET_DIR` outside the subjects (target
dropped after), both subjects cloned shallow at their pinned tags, ripgrep and clap indexed in the same seconds as
before. LB-F82 (fields, enum variants and associated consts and types in rustextract.go and the dialect) moved
def_mapping .692 -> .8988 and typed_recall .671 -> .8212; the SCIP definitions that remain unmapped are the 687
macro-expanded ones and about 400 file modules (a `Module` definition at line 1 of each file, `crate/` for a root).
rust-analyzer defines no field for a crate's own tuple struct (only terminal_size's `Width#0.` appears, an
external crate), so neither extractor emits one.

Residue of import_resolution (.4732, 830 of 1,754 in-repo bindings; classification and counts in the reading's
notes). LB-F83 (nested use trees) closed the extractor-side cause; every miss left is the file resolver's or the
scoring's, handed to the integrator because `internal/fusion/{resolvestrategy.go, fileresolve.go}` are item refs'
alone:

- O287 (482 + 60 unresolved): `resolveCrateMod` reads a spec with no `::` as a `mod x;` child module. A use whose
  path root is a workspace crate (`use globset::{Glob}`, spec `globset`) or exactly `crate`/`self`/`super`
  (`use crate::{a, b}`) therefore resolves to nothing. Minimal fixture: a Cargo workspace `crates/a/Cargo.toml`
  (`name = "a"`), `crates/a/src/lib.rs` (`pub struct S;`), `crates/b/src/lib.rs` (`use a::{S};`): the spec `a`
  must resolve to `crates/a/src/lib.rs`, and in `crates/a/src/x.rs` `use crate::{S};` to `crates/a/src/lib.rs`.
  The extractor cannot tell them apart from `mod x;` (both bare), so the resolver needs to know the spec came from a
  `use` (an ImportSpec field) or `mod x;` should be spelled `self::x`.
- O288 (93 + 17 + 11 unresolved): module roots that are not `lib.rs`/`main.rs`/`mod.rs`. (a) A `[[bin]] path =
  "crates/core/main.rs"` target in the workspace-root manifest with no Cargo.toml of its own: `crateSrcDir` finds
  the root manifest and takes `src/`, which does not exist (ripgrep, 93 bindings); read `[[bin]]`/`[lib]` `path`
  and `[lib] name` when naming a crate root. (b) `use super::{A}` inside an inline `mod tests { }` means the file's
  own module, not its parent directory's (17). (c) `mod util;` in `tests/tests.rs` is `tests/util.rs`: an
  integration-test, example or bench root file is a crate root, so its child modules sit beside it, not in
  `tests/tests/` (11, and 19 `crate::util` paths from those files resolve to `src/lib.rs`). Minimal fixture:
  `tests/tests.rs` with `mod util;`, `tests/util.rs` empty: the spec `util` must resolve to `tests/util.rs`.
- O289 (210 resolved elsewhere, 51 scoring): `pub use` re-exports. The resolver names the re-exporting module's file
  (a facade crate's `src/lib.rs`, a `mod.rs`) and the index names the file that defines the item: `use grep::pcre2;`,
  `pub use clap::ValueHint;` (clap's lib.rs `pub use clap_builder::*;`), `use super::ArgAction;` where
  `builder/mod.rs` has `pub use self::action::ArgAction;`. This is the integrator follow-up on item refs' generic
  export channel: a named import should follow the target file's own re-exports to the defining file. Minimal
  fixture: `a/src/lib.rs` `mod inner; pub use inner::T;`, `a/src/inner.rs` `pub struct T;`, `b` with
  `use a::T;`: the named import `T` must reach `a/src/inner.rs`. The 51 are not resolution errors: a statement
  `use {bstr::BString, grep::printer::{ColorSpecs}}` pairs one oracle set (the in-repo files of its in-repo
  members) with every binding of the statement, so the external-crate binding reads as unresolved.

Re-run 2026-10-02 (wave lx item rust, fathom 3963477f): the same Homebrew `rust-analyzer 0.0.0 (03fcb77246 2026-09-27)` and
`cargo`/`rustc 1.98.1`, `cargo.features = "all"`, `CARGO_HOME` and `CARGO_TARGET_DIR` on the external volume, ripgrep 14.1.1 and clap
v4.6.1 cloned shallow at their tags (ripgrep 8.7 s, 7.4 MB; clap 24.2 s, 11 MB), the recipe above unchanged. Four things this wave
adds to what the reading reads:

- **Macro-generated definitions leave `def_mapping` and `typed_recall`.** `rgtest!(r16, ..)` defines `fn r16` (ripgrep's tests), clap's
  test macros and lazy_static's struct define items whose range sits on the invocation with no declaration keyword before the
  name. `SCIPOptions.RustMacroDefs` (on in `SCIPOptionsFor("rust")` with a source reader, `internal/langbench/scip_rust.go`, LB-F249)
  counts such a definition under `scip_defs_macro_generated` (680) instead of `scip_defs`, and a reference written inside one under
  `scip_edges_macro_generated` (4,812) instead of `scip_edges`: def_mapping .8988 -> .9588, typed_recall .8033 -> .8638. A file
  module (a whole-file Module definition, 404) is not touched: whether it gets a symbol or its references are read at file
  granularity is the owner's question (O542).
- **Parameters.** rust-analyzer writes every parameter, `self` and type parameter as `local N` with kind Parameter, SelfParameter or
  TypeParameter (14,533 references), the shape S1 (LB-F239) counts as `scip_edges_param_target`; it stopped a parameter read minting a
  function-to-function typed edge, so typed_recall read .8033 where .8212 had been inflated.
- **Duplicate symbols.** Two test targets of one package that each hold a `utils` module (clap's tests/builder and tests/derive) get one
  SCIP symbol; `SCIPResult.FileEdges` takes the first document as the definer, so a reference in the other target is an edge to the
  wrong file (O539). Both extractors' file_edge_precision is understated by it, and lang-parity refuses the dispatch by one edge.
- **L3 residue.** After the references query read `#[macro_use] mod x;` as a glob re-export and the walker emitted `macro_rules!`
  symbols, import_resolution reads .9771 (22 bindings left: `value_parser!`, a `#[macro_export]` macro written in a child module;
  O540) and the 1,491 undrawn file edges split 849 module references, 292 typed-receiver members, 139 macros and 211 fully
  qualified paths (O541).
