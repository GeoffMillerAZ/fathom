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

Subjects (calibration/corpus.d/wgsl.yaml, 2026-10-25): gfx-rs/wgpu v30.0.1 (40f4a34e; 268 files, 1.98 MB, `naga/tests/out/` left out
by the sparse pattern; wgsl-parse rejects or times out on 10: 3.7%) and webgpu/webgpu-samples main (e040ec1a; 76 files; 1 rejected).
The reading is measured with `lang-oracle -only` excluding `naga/tests/naga/deep-template.wgsl` alone (owner ruling 2026-10-24: a
restriction to files wgsl-parse accepts is allowed while its rejects are at most 5% of a subject's files, each named in disclose):
tree-sitter's GLR parse of that file does not finish inside the engine's 10 s budget, which is one fault in 344 files against the
0.1% floor, so it cannot stay in the measured set; the other nine rejects stay in, and their 39 definitions are the reading's
extra definitions. `task lang:loop` has no `-only` pass-through, so the reading came from two `lang-oracle` runs (one per subject,
the same `-out`) replayed with `task lang:loop READING=... WRITE=1 -- wgsl`.

Totals on that scope: 2,287 oracle definitions all matched (kind, range and signature agreement 1.0), 39 extra fathom definitions
in rejected files (sym_precision .9832), 2,573 call pairs all found (call_recall 1.0, call_precision 1.0).
