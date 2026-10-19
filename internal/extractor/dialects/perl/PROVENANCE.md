# perl — provenance

- Grammar: `tree-sitter-perl/tree-sitter-perl` 2.0.0 (MIT), pinned at the `release` branch commit
  `4586b3f1fc6f8b80cca6ceb23ba863ae986eeae8` (the v2.0.0 tag, `50904961`, ships no `src/parser.c`; the release branch adds it),
  ABI 15, external scanner, grammar at the repository root (`tree_sitter_perl`), built into
  `internal/extractor/tsengine/grammars/perl.wasm.gz` (2,443,271 bytes raw, 452,171 bytes gzip). Pin and digests in
  `internal/extractor/tsengine/grammars/grammars.lock.json`. Licence read at the pinned commit: the top-level `LICENSE` (MIT).
- `tags.scm`: written for this dialect from the grammar's node names. The grammar ships no `queries/tags.scm` and no
  third-party Perl tags query was consulted. A package, a `class` and a sub or `method` are named as written; the block forms
  (`package Foo { ... }`, `class`) contain their subs so these are qualified by them, the statement form `package Foo;`
  spans only itself. A function call is a named `function_call_expression` or `ambiguous_function_call_expression`
  (`f(1)`, `Pkg::f(1)`, `f 1`), a method call a `method_call_expression`; a callee that is no name (`&$code(1)`,
  `$obj->$name`) and a function named like a Perl builtin or keyword (perlfunc 5.34's list plus try/catch/finally/defer/isa,
  with or without `CORE::`) are left out, a method of that name is kept. The same list is in
  `calibration/oracles/ppi-calls/ppi_calls.pl`.
- Naming: a sub is named as written (`Pkg::name` for a qualified one), so the manifest compares names whole
  (`name_match: identity`): a package is `Mojo::Asset` where the bare rule would cut `Asset`.
- Dispatch: `.pm`, the `perl` and `cperl` shebangs, and a content sniff for `.pl` and `.t` (`.pl` is Prolog's too): a
  `#!...perl` line, `use strict|warnings|Test::|Mojo|v5`, a `package NAME` or a `sub NAME`.
- Not extracted, on purpose: anonymous subs, typeglob assignments (`*name = sub`), `use constant`, variables, labels, formats,
  forward declarations (`sub name;`), `BEGIN` and other phasers as definitions (their calls belong to the enclosing sub or to
  no sub).
- Known gaps: a package statement is a one-line definition, so a sub of `package Foo;` is not prefixed `Foo::`; `async sub`
  (Mojo::Base `-async_await`) parses and is extracted though ctags misses it; the grammar's first-line `use v5.38` class
  feature is extracted as `PerlClass` and `PerlMethod`.
- Oracles (calibration/oracles/README.md): L1 against universal-ctags 6.2.1 (tier B; it writes no end line for Perl, so
  `range_agreement` is not measured and L1 stays good), L2 against PPI (`calibration/oracles/ppi-calls`, an independent Perl
  parser library, tier C).
