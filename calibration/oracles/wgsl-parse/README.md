# wgsl-parse-oracle — tier-B WGSL L1/L2 oracle from an independent parser

ADR-048 amendment D2: universal-ctags 6.2.1 has no WGSL parser. `wgsl-parse` (0.6.0, MIT OR Apache-2.0, feature `naga-ext`; MSRV
1.97.1), a Rust crate that shares no code with tree-sitter, parses WGSL into a typed syntax tree whose global declarations carry
a byte span (starting at the first attribute). The tool here walks every `*.wgsl` file under a subject and writes the neutral
JSON `indexbench lang-oracle -tool-oracle` reads (`tool`, `version`, `tier` "B", `definitions`, `calls`, `disclose`). Bench-only;
never a product dependency. The tier is a claim for review, not a harness fact: an independent syntactic parser, so L1 can be
excellent and L2 at most good (the tier-B cap, as for JavaParser, Roslyn, php-parser and luaparse).

Installed without sudo, nothing global (rustup, cargo's registry and the build target all live under the scratch dir):

    export RUSTUP_HOME=<tools>/rust/rustup CARGO_HOME=<tools>/rust/cargo CARGO_TARGET_DIR=<tools>/rust/target PATH=<tools>/rust/cargo/bin:$PATH
    curl -sSf https://static.rust-lang.org/rustup/dist/x86_64-unknown-linux-gnu/rustup-init -o <tools>/rust/rustup-init && chmod +x <tools>/rust/rustup-init
    <tools>/rust/rustup-init -y --profile minimal --no-modify-path --default-toolchain stable      # rustc/cargo 1.99.1
    (cd calibration/oracles/wgsl-parse && cargo build --release)
    <tools>/rust/target/release/wgsl-parse-oracle <clone> out.json

The loop's runner file (`task lang:loop ORACLE_CONFIG=<runners.json> -- wgsl`), keyed by the queue's oracle name:

    {"wgsl-parse": {"command": "<tools>/rust/target/release/wgsl-parse-oracle", "args": ["{subject}", "{out}"], "format": "tool"}}

What it counts, and does not:

- **Definitions** (`kind`): `fn`, `struct`, `alias`, `var`, `const`, `override` — every global declaration. `line` is the line of
  the declaration's name (the word after the keyword, found past any attributes and comments, since `Ident` has no span; fathom
  reports the name's row), `end` the last line of the span (a variable's or constant's `;` included). `let` at module scope and
  `const_assert` are not definitions. The manifest maps the kinds (`dialect.json` `oracle_kinds.tool`; `const` and `override` are
  both constants).
- **Calls**: every `FunctionCall` expression and statement, builtins and type constructors included (`vec4<f32>(...)`, `array<T,
  N>(...)`), the callee named without its template arguments, deduplicated per (from, to). `from` is the global declaration that
  holds the call: a function (its body and attributes), or a `var`, `const` or `override` (its type and initializer).
- **Rejects**: a file wgsl-parse does not accept, or does not finish within 20 s, yields no definitions and is named in `disclose`
  with the count. Its definitions still count against `sym_precision` for fathom (the harness has no per-file scope).
- **Time and stack**: the parse of each file runs on its own 1 GiB-stack thread under the 20 s limit: wgpu's
  `naga/tests/naga/deep-template.wgsl` (a template nested thousands deep, a parser stress input) takes wgsl-parse exponential
  time, and a deep file overflows the default stack.

Subjects (calibration/corpus.d/wgsl.yaml, 2026-10-25): gfx-rs/wgpu v30.0.1 (40f4a34e; 267 files, `naga/tests/out/` and
`naga/tests/naga/deep-template.wgsl` left out by the sparse list; wgsl-parse rejects or times out on 9) and
webgpu/webgpu-samples main (e040ec1a; 76 files; 1 rejected). The reading is measured with no `-only`: every file in both
subjects counts, the nine wgpu rejects and the one samples reject stay in, and their definitions are the reading's extra definitions.

`naga/tests/naga/deep-template.wgsl` (a template nested thousands deep, 25 KB) is not in the subject, by owner ruling 2026-10-25
(design 1.5 precedent: a file past what the engine can parse leaves the subjects, disclosed as a product limit). It is a negative
fixture, not code real WGSL resembles: wgpu's `recursion_depth_template` test (naga/tests/naga/wgsl_errors.rs:998) feeds it to naga's
own front end and expects the error "Parser recursion limit exceeded". tree-sitter's GLR parse of it takes 6.1 to 8.1 s on the
4-core measuring VM (one earlier run exceeded the 10 s guest budget, a fault), and with it in the set wgpu's throughput is 0.31 MB/s
against the 1 MB/s floor, so the level would not be met. The oracle prints this limit in `disclose` whenever the subject has
`naga/tests/naga`. A template-nesting ingest guard is a separate follow-up.

Totals on that scope: 2,287 oracle definitions all matched (kind, range and signature agreement 1.0), 39 extra fathom definitions
in rejected files (sym_precision .9832), 2,573 call pairs all found (call_recall 1.0, call_precision 1.0).
