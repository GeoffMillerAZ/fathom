# powershell - provenance

- Grammar: `airbus-cert/tree-sitter-powershell v0.26.5` (MIT, verified at the
  pin: the repository's top-level `LICENSE`; there is no licence file under
  `queries/` or `src/`), commit `d398441825243b00e317e87e1829b9d6a3e54ce0`,
  ABI 15, with a scanner. The release ships `src/parser.c`, so the lock row
  does not set `"generate"`. Built into
  `internal/extractor/tsengine/grammars/powershell.wasm.gz` (841,031 bytes raw,
  165,241 bytes gzip). Pin and digests are in
  `internal/extractor/tsengine/grammars/grammars.lock.json`
  (`internal/extractor/tsengine/grammars/PROVENANCE.md` regenerates its own
  tables from it). v0.26.5 is the latest tag; upstream's default branch is
  three commits ahead (fold queries and comment test cases only, checked
  2026-09-28), so there is no newer grammar to move to.
- `tags.scm`: written in this repository (LB-Q-powershell,
  docs/design/language-breadth.md 13). The grammar ships only
  `queries/highlights.scm`; Aider-AI/aider has no PowerShell tags query;
  nvim-treesitter's `runtime/queries/powershell/locals.scm` @ `728e031f`
  (Apache-2.0) was read for node shapes and nothing was copied. Every shape
  comes from `indexbench lang-probe -dialect powershell <fixture>` (13 step
  12), never from `grammar.js`/`node-types.json`.
- **Kinds emitted**: `PowerShellFunction` (a `function`, `filter` or `workflow`
  statement, at any nesting depth), `PowerShellClass`, `PowerShellEnum`,
  `PowerShellEnumMember`, `PowerShellMethod` (class methods and constructors
  alike; a constructor is named after its class). A function's signature runs
  from its first token through its `param()` block (the queue note's "param
  blocks as signatures"); a class method's runs through its parameter list.
  Calls (L2): a command invocation (`Get-Item -Path x`, also through `&` and
  `.` with a bare-word name), a member invocation (`$obj.Method($a)`,
  `[Type]::Method($a)`), and a member invocation in command-argument position
  (`Format-Type $Value.GetType()`, which the grammar splits from its
  argument list).
- **Not emitted** (a `required_kinds` hole is refused by the harness, so these
  are simply outside the manifest): class properties and script/global
  variables. A PowerShell variable is a single token that includes its `$`
  sigil, and the tags engine takes `@name` text verbatim with no transform
  predicate (design 3.2), so a property would be named `$Name` and never
  match an oracle's `Name`. An enum member keeps its bare name: the engine
  qualifies only containers and callables (`qualifyTSDefs`). Pester `Describe`
  / `It` blocks are commands, so they are call edges, not definitions.
- **Call filter** (`#not-match?` on the callee): drops a name that starts
  with `$`, an integer range read as one command (`1..$n`, `1..$x.Count`),
  and the statement keywords `else`, `elseif`, `catch`, `finally` that
  error recovery or `$x = if (...) { }` newline `else { }` leaves as
  commands. Before the filter, 43 of Pester's 1,971 call pairs (against the
  parser's own AST) were `else`, `elseif` or `catch`.
- **Grammar variance (disclosed, verified with lang-probe)**:
  - Two consecutive empty-script-block defaults (`$OnBlock = {}, $OnTest = {}`
    in a `param()`) and a bare `{}` statement desync the parser; the rest of
    the file after that function is lost (Pester `Fold-Container`, `Fold-Run`,
    `Exit-CoverageAnalysis`).
  - `param(...)` followed by a statement on the same line
    (`function F { param($x) Get-Item }`) fails the same way; a newline after
    the param block parses cleanly. The `functions.ps1` fixture keeps its
    `workflow` one-liner free of this shape on purpose (a golden must not pin
    a grammar bug).
  - A command whose name starts with a keyword and a hyphen (`Do-Work`) reads
    as the `do` keyword plus an ERROR.
  - `using namespace X` parses as a command named `using` (top level, so it
    attaches to no definition).
  - A typographic quote that leaves a string unterminated turns the rest of
    the file into an ERROR (PSScriptAnalyzer's `NotGlobal`).
  - A file ingest reads as binary is skipped before any extractor runs:
    UTF-16 (Windows PowerShell ISE's default) is 15 of PSScriptAnalyzer's 244
    PowerShell files. A product gap, not a grammar one.
- `testdata/`: fixtures written in this repository. `functions.ps1` - every
  function shape (param block with attributes on several lines, an inline
  parameter list, `filter`, `workflow`, an empty body, a nested function, a
  scope-prefixed name kept verbatim) with plain, static and instance member
  calls. `classes.ps1` - an enum, a class and a subclass with constructor,
  static and instance methods, a multi-line method signature (properties are
  in the source and, by design, not in the golden). `module.psm1` - a module
  file: script state, functions calling each other, `&` and dot-sourcing,
  `[List[string]]::new()`. `manifest.psd1` - a data file, no definitions.
  `calls.ps1` - the call shapes and the misreads the filter exists for (a
  member call as a command argument, `& Get-ChildItem`, `. Import-Helper`,
  `& $context.Runner` (no callee), `1..$items.Count`, `else` after an `if`
  expression). `broken.ps1` - an unrecoverable statement between two good
  functions (all three still extract). Goldens were written by hand from
  reading each fixture before the queries existed, except `broken.ps1`'s,
  which is the extractor's output reviewed against the fixture (what a parser
  recovers from an error is not predictable by reading). Three predictions
  were corrected on first contact, none by weakening an assertion: an enum
  member is unqualified (the engine's rule, above), a one-line `workflow {
  param($x) ... }` in a fixture hit the same-line grammar failure (the fixture
  changed, the failure is disclosed), and `calls.ps1`'s line numbers were
  miscounted by hand. Every fixture was also run through the oracle:
  definitions and calls agree except the calls inside `broken.ps1`'s failed
  function.
- **Measurement** (`docs/bench/readings/lang/powershell.json`,
  `calibration/corpus.d/powershell.yaml`): PowerShell's own parser
  (`calibration/oracles/powershell-ast`, tier B) on `PowerShell/PSScriptAnalyzer`
  1.25.0 (subpath `Tests`) and `pester/Pester` 6.2.0. L1 excellent
  (sym_recall .9961, sym_precision 1.0, kind_agreement 1.0, range_agreement
  .9971, signature_coverage 1.0), L2 good (call_recall .9971, call_precision
  1.0; tier B caps L2 at good). universal-ctags 6.2.1 was measured first and
  is why the dump is the L1 oracle (D2): no end lines, no methods or
  properties, class text tagged inside here-strings, thousands of variable
  tags; see the oracle's README.
- **Merge gate (11 step 2)**: not applicable, there is no incumbent. The
  generic dialect registration in `fusion.New` dispatches `.ps1 .psm1 .psd1`
  to this dialect; `ExtractorEpoch` 18 re-extracts files previously
  fingerprinted as unsupported.
