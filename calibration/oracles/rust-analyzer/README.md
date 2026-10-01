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
remaining dialect-only definitions (30 on ripgrep, 140 on clap), all real definitions in the source.

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
  sites, and none is written for an operator. L4 is not touched: `def_mapping` and `typed_recall` count every SCIP
  definition and reference.

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
