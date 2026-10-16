# Language breadth and depth — engineering design and the worker recipe

Status: accepted 2026-09-27 (ADR-048). Vision: `docs/vision/37-language-breadth.md`.
Plan: `docs/plans/language-breadth.md` (LB-F… foundation, LB-M… measurements, LB-Q-…
one row per queue item). Queue: `docs/plans/language-breadth.queue.json`. Tracker:
`docs/languages.md`. Constraints carried in from elsewhere: ADR-021 §4 (wazero, never
CGO, for new languages), ADR-041 decision 5 (typed edges by SCIP import), ADR-047 and
`docs/design/responsible-tenant.md` §7.2 (RT-107: module reuse, 64 MiB module cap),
invariants I2, I23–I25, I47–I51, principles P2, P19, P20, P22, P23.

The design has seven parts. §13 is the recipe a worker follows for one queue item; it is
written to be read on its own.

| Part | Owns | Where |
|---|---|---|
| A. Levels and measurement (§1, §10) | what a level means; the harness; the readings | `internal/langbench`, `cmd/indexbench lang-oracle` |
| B. Dialects as data (§2, §5) | the manifest, classification, kinds | `internal/extractor/dialects/<id>/`, `internal/connector` |
| C. The tags-query engine (§3) | syntax languages, L1–L2, imports for L3 | `internal/extractor` (`TSExtractor`), `internal/extractor/tsengine` |
| D. The document-path engine (§4) | structured tool formats | `internal/extractor` (`DocExtractor`) |
| E. L3 resolution and L4 import (§6, §7) | file-resolved and typed edges | `internal/fusion/fileresolve.go`, `internal/scip` |
| F. Grammars, packs, licences, tenancy (§8, §9) | build, lock, distribution, memory | `tsengine/grammars/`, `internal/extractor/tsengine` |
| G. Retirement, tracker, loop (§11, §12, §14, §15) | bespoke walkers out; honest tracker; the worker loop | `docs/languages.md`, the queue |

A note on names: the bench pyramid's "L0" (the free index-oracle layer, ADR-032) is not a
support level. Support levels are L1–L4; this document never says L0.

---

## 1. Support levels, precisely

### 1.1 What each level emits

| Level | The extractor must emit | Stored as |
|---|---|---|
| L1 | every definition of the dialect's declared kinds: `Name` (qualified by containment), `Kind` (a `SourceKind` string from the manifest), `Package`, `Line` (the definition node's first line), `EndLine` (its last line), `Signature` for callables (source from the definition's start to its body's start, whitespace collapsed, ≤ 200 characters) | `core.SymbolNode` |
| L2 | L1, plus each definition's call sites by callee name (the `@name` inside a `@reference.call`, else the call expression's text, last dotted segment kept) | `SymbolNode.Calls` |
| L3 | L2, plus every import statement as an `ImportSpec`, and a module-resolution strategy that turns in-repo specs into files; `ResolveFileEdges` then writes `file-resolved` edges at tier pre-scan | `file_imports`, typed_edges under the file tier |
| L4 | L3, plus typed edges from a native pass or an imported SCIP index, mapped to fathom symbols by position (O115) | typed_edges |

Tool formats: L1 entities, L2 intra-file references, L3 cross-file or cross-repository
references (as `service_ref`/`module_ref`-shaped edges, the existing k8s/compose
convention). L4 is `not-applicable` for tool formats and is never claimed. A route, an
upstream or a mesh rule that names the host it sends traffic to is an L3 cross-file
reference too: a `route_ref` edge from the route entity to the Kubernetes Service or compose
service declaring that host, the runtime topology's first edges (§4, "Route references");
an intra-format name (an nginx upstream, an Envoy cluster) stays L2.

### 1.2 Thresholds

The vision's table (§2) is normative. Restated as the harness computes them:

| Metric | Definition | L1 exc. | L1 good | L2 exc. | L2 good | L3 exc. | L3 good | L4 exc. | L4 good |
|---|---|---|---|---|---|---|---|---|---|
| `sym_recall` | matched ÷ oracle definitions in the kind intersection | ≥ .95 | ≥ .85 | — | — | — | — | — | — |
| `sym_precision` | matched ÷ fathom definitions in the kind intersection | ≥ .95 | ≥ .90 | — | — | — | — | — | — |
| `kind_agreement` | matched pairs with equal canonical kind ÷ matched | ≥ .95 | ≥ .85 | | | | | | |
| `range_agreement` | matched pairs with \|EndLine − oracle end\| ≤ 1 ÷ matched pairs where the oracle has an end | ≥ .90 | ≥ .75 | | | | | | |
| `signature_coverage` | callables with a non-empty `Signature` ÷ callables | ≥ .90 | ≥ .70 | | | | | | |
| `call_recall` | oracle (enclosing symbol, callee name) pairs found ÷ oracle pairs | | | ≥ .90 | ≥ .75 | | | | |
| `call_precision` | fathom pairs present in the oracle ÷ fathom pairs | | | ≥ .90 | ≥ .85 | | | | |
| `import_resolution` | in-repo import specs resolved to a file ÷ in-repo specs (oracle: SCIP's cross-file references' files) | | | | | ≥ .95 | ≥ .85 | | |
| `file_edge_precision` / `file_edge_recall` | file-resolved edges vs SCIP cross-file edges at fathom granularity; with no SCIP index, vs an import dump's (file, imported file) pairs at the dump's tier (LB-F57: statements, not references, §10); Go's are its typed pass's edges that name their target (`EdgeDirect`, `EdgeFuncValue`; LB-F58, §1.7) | | | | | ≥ .95 / ≥ .50 | ≥ .90 / ≥ .25 | | |
| `def_mapping` | SCIP definitions of mappable kinds mapped to a fathom symbol ÷ those definitions; a namespace's definition (a Go package, K18, §1.4) is out of both, counted (`scip_defs_namespace`) | | | | | | | ≥ .95 | ≥ .85 |
| `typed_recall` | SCIP reference edges present as typed edges at fathom granularity ÷ SCIP edges; a reference to a local (LB-F55), to a parameter or type parameter (S1, LB-F239, §1.4) or to a namespace (a Go package, K18, §1.4) is out of both, counted (`scip_edges_param_target`, `scip_edges_namespace_target`) | | | | | | | ≥ .95 | ≥ .85 |

**Not measured is not failed.** When the only available L1 oracle cannot report a field —
universal-ctags writes no end line for HTML, TypeScript, JavaScript, Rust, Kotlin, Lua or
PHP — the harness records `range_agreement` under the reading's `not_measured` (it is the
only metric that may be) with a value of `null`, never a failure. The level is then met at
most at grade **good**, never excellent, and the reading's notes say why. The rule is
inferred from the data, not declared: an L1 oracle that matched definitions but gave an end
line for none of them. A tier-A oracle that reports ends lifts the cap by measuring it.

Always required, every level: `faults ≤ 0.1%` of files with every faulting file named;
`throughput ≥ 1 MB/s/core`, judged on CPU time (bytes per CPU-second of the extraction
process, `throughput_basis: "cpu"`; D1, §1.5); `max_file_ms ≤ 2000`; `peak_module_pages ≤ 1024`.

Tool formats: L1 `sym_recall` and `sym_precision` ≥ .98 (good ≥ .95); L2 reference
recall and precision ≥ .95 (good ≥ .90); L3 precision ≥ .95, recall ≥ .80 (good .90/.60).

### 1.3 Oracle tiers and what they can prove

| Tier | Oracles | May prove |
|---|---|---|
| A | SCIP index from the language's maintained indexer; a native type checker; the tool's own evaluation (`make -pn`, `bazel query`, `terraform graph -type=plan`, `helm template`, `kustomize build`, `docker compose config`, `gitlab-ci-local --list`, the format's reference parser) | any level, excellent |
| B | universal-ctags (`--output-format=json --fields=+nKzSe --extras=-{anonymous}`) run as a subprocess | L1 excellent |
| C | the grammar's upstream `queries/tags.scm` run by the tree-sitter CLI; an independent parser library; a third-party definitions query run by fathom's engine at a pinned revision (`lang-oracle -tags-query`, D5 extension, §1.5) | at most good, any level it covers |
| D | grep proxy (`baseline.sh`); a sample labelled by the extractor's own author | "tolerated" only |

A reading records the tier per metric. The harness computes `level_met` from the metrics
and tiers alone; nobody types it.

A runtime import trace is the language's own evaluation, and so tier A, only for what it
attests (LB-F256, `calibration/oracles/ruby-require-trace`). Ruby loads each subject file in
its own process, under the subject's own bundle and the load path its own test task gives, and
records the file Ruby's lookup names for every `require`, `require_relative`, `load` and
`autoload` that runs. A static dump is tier B because the operator chooses its load path; the
trace uses the program's. A statement that never runs (inside a method, or in a file that
raises before it) has no truth, so the trace claims tier A only when the statements it ran are
at least 90% of the subject's in-repo statements with a literal path. A statement it did not run
counts as in-repo when the same lookup from the driver's load path names a subject file. Below
90% the trace's file is tier B, as the dump is. The trace writes only literal statements, the
ones an extractor of import statements can see. It counts a computed `require` it ran, and
never writes one.

### 1.4 Matching rules

- **Definitions**: key (path, name, line). A fathom definition matches an oracle definition
  in the same file with the same name and |Δline| ≤ 2 (decorators and attributes move
  a definition's first line); nearest wins, one-to-one. Qualified names are compared on the
  last segment; containment is checked separately (`scope_agreement`, reported, not gated).
  A dialect whose identifiers carry punctuation — CSS selectors (`.a`, `#b`, `:hover`), npm
  names (`@babel/core`), script names (`codesandbox:build`), export paths (`./utils`) —
  declares `"name_match": "identity"` in its manifest and its names are compared whole
  (default `"bare"`, the last-segment rule). The same mode governs reference pairs.
- **Kind intersection**: the manifest's `oracle_kinds` maps each oracle kind (ctags kind
  letters/names, SCIP descriptor suffixes) to a canonical kind. Definitions whose kind is
  outside the intersection on either side are excluded from both counts and reported as
  `excluded_by_kind`. A dialect may not exclude a kind the vision's L1 list names for the
  language (the manifest's `required_kinds`, which are its own kind keys, checked through
  their canonical kinds — a reading records them canonical) — the harness refuses the reading.
- **Calls**: truth pairs are (enclosing definition, callee bare name) derived from SCIP
  reference occurrences whose symbol is callable (SCIP method descriptor `().`), inside a
  definition that maps to a fathom symbol.
- **Calls, amendment 2026-10-02 (wave le, item tsjs; LB-F175, O440): oracle-blind sites.**
  A fathom pair the truth lacks whose every call site written for it (`name(` inside the
  caller's lines) has no occurrence of the index at all at the callee's range is out of both
  counts, counted (`calls_oracle_blind`) and disclosed in the reading's notes, behind the
  per-dialect flag `SCIPOptions.OracleBlindSites` (on for typescript and javascript only). A
  pair with no written site found, or with one site the index did write an occurrence at,
  stays. *Reason:* scip-typescript writes no occurrence where it resolves no symbol, at
  `super(...)` and at a member of a receiver it types as `any` (an untyped JavaScript
  parameter, `this` inside a function assigned to a prototype), so such a call is one the
  oracle never judged, not one it judged absent; LB-F116's compiler-unjudged rule makes the
  same cut for C/C++ with the compiler as witness, this one with the index itself. The
  oracle's environment is fixed first (its dependencies installed, the subject's own
  compiler options, O441), so the rule removes what no environment resolves: 429 of
  TypeScript's 11,327 pairs and 422 of JavaScript's 1,441 at 43c90bb4 (1,833 of nest's alone
  on the 2026-09-28 index, which resolved no `@nestjs/*` import). It never drops a call from
  the store, and never touches recall. Item pycost (Python) may reuse it for scip-python
  through the integrator; any other dialect opts in only with its own counted reading.
  *Revisit when:* a JavaScript or TypeScript calls oracle that types every receiver (a
  checkJs/JSDoc-typed subject, or a tier-B call oracle) is available, then the rule is
  measured off against it.
  *Fired at birth for JavaScript* (wave le audit, 2026-10-02): its reading leaves out 422 of 1,441
  pairs (29%; express 242 of 614). O442 reads that share as the oracle's (scip-typescript
  types few receivers in untyped CommonJS), not the subject's, and JavaScript claims no L2
  level from the rule (LB-Q-javascript-rest stays open on its counted ceiling); its reading
  is to be re-measured with the rule off when the first trigger's oracle exists.
  *The 10% cap is a rule, not a trigger* (owner-architect decision (a), ADR-048 amendment
  2026-10-02): the oracle-blind rule is accepted, and a reading whose `calls_oracle_blind`
  exceeds 10% of its fathom pairs claims no level from the rule: its L2 is graded as if the
  rule were off, and the reading says so. The harness computes the cap (item js, LB-F203), and
  every exclusion stays counted in the reading. Until then the share that once fired the
  trigger (JavaScript's 29%) is applied by hand, as above.
- **Typed edges, amendment 2026-10-02 (wave lx, item python; S1, LB-F239, O525): parameter
  targets.** A SCIP reference whose target is a parameter or type parameter the index defines
  and no extractor maps is out of `typed_recall`'s numerator and denominator, counted
  (`scip_edges_param_target`, `internal/scip` `Report.Skipped["parameter-target"]`) and never
  an edge. A parameter is a `(name)` or `[name]` descriptor (scip-python, scip-typescript,
  scip-php, scip-dart's named parameters) or a `local N` whose `SymbolInformation.kind` is
  Parameter, SelfParameter or TypeParameter (rust-analyzer, scip-dart), and such a
  definition never joins a fathom symbol by position (O115's self-loop guard, now covering the
  `local N` shape too). A reference inside an unmapped parameter's `enclosing_range` (scip-dart's
  type annotation or field formal, rust-analyzer's type-parameter bounds) is attributed to the
  function that declares it, as LB-F55's local initializers are; scip-python 0.6.6 sets no
  `enclosing_range` on a parameter (0 of 6,588 on fastapi and flask). *Reason:* fathom's
  granularity is the definition and a parameter is finer (O115); `def_mapping` already leaves
  parameters and type parameters out (`scipBareName`), and LB-F55 made the same cut for locals.
  `typed_recall` measured them as typed edges no extractor could ever mint, which bounded it far
  below the bar for every language whose receivers are parameters (Python's `self`). *Revisit
  when:* fathom mints a symbol for a parameter (a signature-level symbol, or a parameter
  query surface), then they re-enter both counts.
- **Go members, measured and killed, 2026-10-10 (wave lf2, item go; K17, O606, O673, O674).** The owner's member
  pass was built and measured: `goextract` named a top-level struct's fields `Owner.name` (`GoField`, canonical
  `field`, an embedded field by its type) and an interface's own methods `Iface.Method` (`GoMethod`), and the go/types
  pass stored each field reference as a typed edge to the field (`field-read`, `field-write`, never a call: callgraph's
  callees views and Go's L2 pairs left them out). Go read L4 excellent (def_mapping .9632, typed_recall .9957; L1 to L3
  excellent, L2 unchanged; field references joined the named-target file-edge set, file_edge_recall .7034 to .8758). K17's
  kill gate (fathom's self-ingest, median of 3: wall +15%, store +20%) refused it: every struct's fields cost +30% wall and
  +34% store, an exported struct's fields only +26% and +22% (L4 good, .8773 / .9369, with the fields kept outside
  `oracle_kinds` since ctags tags an unexported struct's fields). The pass is reverted; the branch history keeps it, and
  O674 lists what it had to change beyond `goextract` (the syntax pass's name-only binding, the method-set implementation
  readers, the L0 oracle, the rename verb, `TypedEdgeEpoch`). *Revisit when:* the owner picks a cheaper member pass from
  O673's levers (no field reference from a `_test.go` file, 43% of them; centrality over call edges only; field symbols
  without reference edges; interface methods alone), re-measured under the same gate.
- **Namespaces, amendment 2026-10-10 (wave lf2, item go; K18's Go half, LB-F344, O607, O675).** A pure
  namespace (a Go package) is never a fathom symbol: it is file granularity, graded at L3 by file edges.
  scip-go defines a package at every file's package clause; those definitions leave `def_mapping`'s
  numerator and denominator, counted (`scip_defs_namespace`: 728 on cli/cli, 35 on cobra), and a reference to
  one leaves `typed_recall`'s, counted (`scip_edges_namespace_target`, `internal/scip`
  `Report.Skipped["namespace-target"]`: 14,101 and 61) — the S1 shape of the parameter targets above,
  producer-gated to scip-go, never an uncounted exclusion. A reference whose enclosing definition is itself
  unmapped stays an unmapped endpoint of its `from` class (the import judges the `from` side first). A Java
  package, a C# namespace and a Rust `mod` take the same rule when their L4 waves run; a file whose top level
  executes code gets a minted `module` symbol instead (K18's other half, §2 and §3). *Revisit when:* fathom
  mints a namespace symbol (a package-level query surface), then namespaces re-enter both counts.
- **Scope**: generated files (`// Code generated`, `@generated`, minified), vendored trees
  and test fixtures inside the subject are excluded by the same rules fathom's connector
  applies, so the oracle is run over exactly the file list fathom would read (the harness
  passes it `-L <filelist>`).

### 1.5 Owner decisions, round 2 (2026-09-28)

Wave 1b showed the bottleneck had become the measuring instruments, not the dialects. The
owner's decisions, binding on the harness and every reading after them:

- **D1 — throughput is CPU time.** The §1.2 floor is bytes per CPU-second of the
  extraction process (user + system, `getrusage` around exactly the timed extractions),
  not wall clock, so a reading taken while other workers run is not failed by contention.
  The reading records `throughput_basis: "cpu"` and discloses `wall_throughput_mb_s`; a
  subject measured before D1 (no `extract_cpu_ms`) puts the whole reading back on the
  wall-clock basis, never a mix. C's wave-1b reading (0.62 MB/s wall) is re-graded by
  re-measurement.
- **D2 — a stronger oracle where ctags is deficient.** When the tier-B oracle is
  demonstrably deficient for a language (a verified mis-tag, not a disagreement), the
  dialect is measured against a stronger one: TypeScript/JavaScript against scip-typescript
  (tier A: `lang-oracle -scip <index> -scip-defs` scores L1 from the index's definitions —
  `langbench.SCIPDefs`: kind from `SymbolInformation.kind`, else the hover text's
  declaration keyword, else the descriptor suffix; `@scoped` when the parent is a type;
  the manifest's `oracle_kinds.scip` maps them); Java against scip-java, or where it cannot
  run on arm64 a JavaParser symbol dump (tier B, JVM, bench-only); C# against scip-dotnet
  (tier A) if it installs, else a Roslyn syntax dump (tier B). Dumps speak the neutral
  oracle file (`-tool-oracle`, repeatable: `definitions`, `calls`, `references`, each file
  with its own `tier`), mapped through `oracle_kinds.tool`. SQL restricts its calibration
  subjects to the families its grammar supports (ANSI, SQLite, PostgreSQL) with
  `-only <globs>`; the reading names the restriction and the unsupported families (T-SQL,
  MySQL, Oracle, Db2, Vertica) are a disclosed gap. Node, a JVM and the .NET SDK are bench
  instruments only (installed under the measurement scratch dir), never product
  dependencies. An instrument that cannot be installed is recorded with why, and the
  reading keeps ctags with the deficiency disclosed — L1 capped at good.
- **D2, Kotlin (LB-Q-kotlin, 2026-09-28).** universal-ctags writes no end line for Kotlin and lists every
  local `val`/`var`, primary-constructor parameter and lambda as a definition (60,761 definitions to the
  dialect's 16,335 on okhttp and Exposed), so Kotlin is measured against a PSI dump of the Kotlin
  compiler's own parser (`calibration/oracles/kotlin-psi`, tier B like JavaParser and Roslyn) — its
  definitions, ranges and call pairs, a call belonging to the smallest enclosing function or
  constructor as it does in the dialect. scip-java's Kotlin index was tried and cannot stand in: under
  Kotlin 2.2.0 (its plugin refuses 2.3.20) it wrote 58,698 occurrences with no range at all, and
  okhttp's build needs an Android SDK. The grammar pin moved from fwcd 0.3.8 to commit `1852ea17`
  because 0.3.8 left 16.6% of the subject files with parse errors (over §14.2's 5% line) and 1852ea17
  leaves 1.1%.
- **D2, Dart (LB-Q-dart, 2026-09-28).** universal-ctags has no Dart parser, so Dart is measured against
  scip-dart (tier A, `calibration/oracles/scip-dart`) for definitions (`-scip-defs`), calls, definition mapping
  and file edges, one index per pure-Dart package. The instrument reads its shapes as they are written
  (`internal/langbench`, scoped to `scip-dart` symbols and to `SCIPOptionsFor("dart")`): a private member (a
  leading `_`) is a `local N` symbol whose kind still says what it is, so it is a definition and the owner of
  the calls inside it; an unnamed constructor (`Foo#<constructor>().`) is named as its class and an accessor
  (`<get>x`) by x; a file and an `import ... as p` prefix are no definitions; the producer's kind outranks a
  `get`/`const`/`operator` written before the name; and a callable reference is a call site only where its name
  is written followed by `(` or by type arguments and `(` (a tear-off, an operator and an initializer's
  `super(...)` are counted as `scip_calls_unwritten` and disclosed). The tags engine gained an `@end` capture
  (§3.3) because tree-sitter-dart puts a callable's body in a sibling of its signature.
- **D2, Swift (LB-Q-swift, 2026-09-29).** universal-ctags has no Swift parser and SourceKit-LSP's index store is not
  SCIP, so Swift is measured against the compiler's own parser (`swiftc -frontend -parse-as-library -dump-parse`, Apple
  Swift 6.4, tier B like JavaParser, Roslyn and the Kotlin PSI dump; `calibration/oracles/swift-parse`) for definitions
  and call pairs. `-dump-parse` prints nothing of an `#if` block, so the oracle blanks the directive lines and parses every
  branch, as the grammar does; it rebuilds the tree from the dump's indentation because the compiler leaves a
  parenthesis open after a function type; and a `case` pattern the parser writes as a call is not a call. The grammar
  pin moved from the 0.7.3 tag to alex-pinkus commit 35245fb (built with `generate: true`): 0.7.3 left 12.2% of swift-nio's
  files with a parse error against 6.4% on the commit, and the commit fixes the 32-bit `OP_SYMBOL_SUPPRESSOR` shift the
  tag has on wasm32. Both subjects together still read 5.6% (36 of 642 files), Swift 6 syntax the grammar lacks, which
  is over §14.2's refusal line and is recorded, not refused: the dialect finds 21,939 of 21,997 definitions.
  *Amended 2026-09-29 (integration review): "recorded, not refused" was the worker's call, and §14.2
  does not allow it — a parse error rate over 5% of files is a refusal and the language stays disclosed.
  Swift is refused (`LB-Q-swift-rest`): the dialect stays built, tested and measured, fusion does not
  dispatch it (`refusedDialects`), and the tracker, milestones and support matrix claim no level for it
  until a tree-sitter-swift commit parses both subjects under the line.*
- **D2, PowerShell, PHP and Lua (LB-Q-powershell, LB-Q-php, LB-Q-lua; integration review,
  2026-09-29).** universal-ctags is verified deficient for all three: it writes no end line for any
  of their tags (`range_agreement` cannot be measured), and it tags no PowerShell class method,
  constructor or property, has no PHP enum kind and names a PHP namespace by its full path, and
  tags Lua text as a function by pattern (228 on the subjects). Each is measured against a parser's
  syntax dump at tier B, like JavaParser, whose place in D2 is the precedent for an independent
  parser library: PowerShell's own (`System.Management.Automation.Language.Parser`,
  `calibration/oracles/powershell-ast`), nikic/php-parser 5.7.0 (`calibration/oracles/php-parser`)
  and luaparse 0.3.1 (`calibration/oracles/luaparse`, which shares no code with tree-sitter). Lua's
  definition scope (a function stored in a statically named slot) is the dialect author's rule,
  stated in the oracle's README; ctags, reading by pattern, agrees on 4,191 of the dialect's 4,265
  definitions, so the rule is not the dialect's alone.

- **D2, Ada (LB-Q-ada, 2026-09-29).** universal-ctags 6.2.1 is the only Ada oracle installed and it is tier B with two
  disclosed limits: it writes no end line for Ada (`range_agreement` is not measured, so L1 is met at most at good, as
  §1.2 says of the other end-less ctags parsers) and it loses sync in real bodies (after a `declare` block, at a
  `private package` header, at `function ... is abstract;`), so 896 of the subjects' 1,064 dialect-only definitions
  come after the last structure tag it writes in their file. The instrument was changed in one place: ctags writes a
  child unit's name whole (`GNATCOLL.Arg_Lists`) where §1.4 compares the dialect's name on its last segment, so
  `oracleMatchName` cuts an oracle name that is a dotted identifier path to its last identifier (a name with any other
  punctuation is left whole, so `name_match: "identity"` and TestNameMatchIdentity are untouched). Without it the same
  run reads sym_precision .8909 and level_met none, because 573 child units match on neither side. *Revisit when a
  language whose oracle writes a dotted name for something the dialect qualifies differently makes the rule too
  broad, or when an Ada tier-A oracle (Libadalang, GNAT's own tools) is installed.*

- **D3 — oracle-deficiency disclosure.** A reading may carry an adjudicated set of
  dialect-only definitions the L1 oracle is verified to miss (`lang-oracle -adjudicate
  <file>`: classes, each with a `why`, an optional structural `pattern` where `{name}`
  stands for the name and a required `scope`, and the claimed `path`/`line`/`name`/`class`
  entries). A miss is valid only when the oracle failed to report a real definition of a
  scored kind — never a dialect's over-extraction the oracle deliberately excludes
  (locals, callback helpers, object-literal keys): the only scope is `module`, whose
  definition's line starts at column 0 with no emitted definition enclosing it (an
  audit of the first sets found 69% of sampled TypeScript and 29% of JavaScript
  admissions were locals or object-literal keys). An entry is
  admitted only by `langbench.Adjudicate`'s deterministic check — fathom emitted that name
  at exactly that line inside the kind intersection, the oracle matched nothing to it, and
  the definition's source text contains the name (and the class pattern) — never by a
  model's judgment. Admitted entries leave `sym_precision`'s denominator (never recall's
  numerator), are listed under their subject's `adjudicated`, and the reading carries an
  `adjudicated:` note; refused claims are counted and named in the notes.
- **D5 — an L2 calls oracle for every language dialect.** Where no SCIP producer exists
  (bash, SQL …), call pairs come from the grammar's upstream `queries/tags.scm` run by an
  independent runtime (py-tree-sitter or the tree-sitter CLI): each `@reference.call`'s
  `@name` paired with its innermost enclosing `@definition.*`'s `@name`, emitted as the
  neutral file's `calls` at tier C (L2 good at most). Where SCIP gives references it stays
  tier A; a producer that sets no `enclosing_range` (scip-clang 0.4.0) no longer zeroes the
  calls: a definition mapped to a fathom definition takes that definition's line span, and
  the reading counts how many did.
- **D5 extension (2026-09-29) — a third-party definitions query is a tier-C L1 oracle**
  (ADR-048's amendment of that date; LB-F46, LB-F52). Accepted when (a) it is not the source
  the dialect was drafted from; (b) the tier-C cap applies; (c) it is fetched at measurement
  time by pinned revision and never vendored; (d) its licence is recorded in the reading. A
  licence declared consistently across a grammar's package manifests counts when there is no
  LICENSE file; AGPL grammars are refused. `lang-oracle -tags-query <scm> -tags-query-rev
  <40-hex> -tags-query-source "<owner/repo> <path>" -tags-query-licence <SPDX>` (§10) runs the
  query through fathom's engine on the dialect's pinned grammar over the files the manifest
  owns and scores its definitions as the neutral oracle file at tier C, kinds mapped through
  `oracle_kinds.tool`. Capture conventions, detected or set with `-tags-query-format`: `tags`
  (upstream, helix, aider, Sourcegraph: `@definition.<kind>` on the definition node plus
  `@name`; the end line is the definition node's), `locals` (nvim-treesitter:
  `@local.definition[.<kind>]`, or legacy `@definition.<kind>`, on the name node; no end line,
  so `range_agreement` is not measured) and `outline` (Zed: `@item` plus `@name`; the kind is
  the `@item` node's type). Refused: a query a `; source:` header of the dialect's own queries
  names (repository compared case-insensitively), a revision that is not 40 lowercase hex, a
  query file inside the work tree outside `workspace/`, an empty or AGPL licence, and a
  predicate the engine does not support (named, never stripped). The reading's oracle run
  records `source`, `revision` and `licence`. Definitions only; D5's calls path is unchanged.
- **D7 — C#'s `using X; Y.Z();` resolves.** An unaliased `using X;` is a wildcard import
  (the C# idiom: every type of namespace X in scope unqualified); an aliased
  `using A = X;` keeps its prefix binding. Under namespace-dir a wildcard's namespace is
  found first by the files whose symbols declare it (C# namespaces need not mirror
  directories), then by the directory rule; and a dotted call `Y.Z()` whose head `Y` a
  wildcard brought into scope resolves to the member `Y.Z` when `Y`'s file defines it,
  else to `Y` (the one-segment depth the whole-module path keeps) — for every strategy,
  so Java's `import a.*;` then `Y.z()` resolves the same way. The Dockerfile half of D7
  (a local image tag to the Dockerfile that builds it) is recorded with D4 (§4).
- **D6 — a queue-item dependency is met at L1.** A queue item that depends on another queue
  item (cpp on c; scss and less on css) is eligible once the parent has a committed reading
  at L1 or better, not only once the parent's row is `[x]` (its row may stay open for L4
  for months). Foundation rows still need `[x]`/`[-]`. `langbench.QueueEligibility` and
  `indexbench lang-next` compute it (§13.0 step 2). *Amended 2026-09-29 (G2-6): a `[-]`
  row no longer satisfies a dependency — a foundation row needs `[x]`, and a refused
  dependency (a queue item's too, whatever its reading) means re-planning; see §13.0 step 2.*

**Round-2 re-measurement (2026-09-28), with the instruments above** — the readings in
`docs/bench/readings/lang/` carry the numbers and the dispositions:

| Item | Wave 1b | Round 2 | L1 oracle | L2 oracle |
|---|---|---|---|---|
| c | none (0.62 MB/s wall) | L1 good | ctags (B) | scip-clang (A, D5 fallback): precision short |
| typescript | none | L1 good | scip-typescript (A) + D3 (module scope) | scip-typescript: precision short |
| javascript | none | none (sym_precision .84) | scip-typescript (A) + D3 (module scope) | scip-typescript: not adequate for untyped JS |
| java | none | L2 good | JavaParser dump (B) | JavaParser dump (B) |
| csharp | none | L1 excellent | Roslyn dump (B) | scip-dotnet (A): precision short |
| sql | none | L2 good | ctags (B), restricted families | sqlglot FK pairs (C) |
| bash | L1 good | L2 good | ctags (B) | aider bash-tags.scm on py-tree-sitter (C) |
| github-actions | L2 good | L3 good | actionlint walk (C) | same, `cross_references` (D4) |
| kubernetes | L2 good | L3 good | yaml.v3 walk (C) | same, label-selector joins (D4) |
| dockerfile | L2 excellent | L2 excellent (re-graded) | buildkit parser (A) | L3 waits on LB-X-docker-image-ref |

Two re-measurement findings changed the dialects themselves: TypeScript missed abstract
classes and every bodiless signature (fixed, `ExtractorEpoch` 17), and SQL invented a
column for each PostgreSQL `ATTACH PARTITION` (fixed). One is a product limit, disclosed:
a SQL data dump past about 400 KB builds a tree larger than the 64 MiB module cap and
faults (chinook-database left the subjects for it).

---

### 1.6 Go's L2 graded set (wave le, 2026-10-02; E and D shipped in wave lg)

**Decision.** Go's L2 grades the calls the product serves, not goextract's syntactic `Calls`: option **C**,
`callgraph.BuildBoth(nodes, typedEdges)` (the syntactic `Calls` that resolve to a node of the subject, plus the
typed pass's `EdgeDirect`, `EdgeIfaceDispatch` and `EdgeFuncValue` edges; which surface reads it is O475 and
the wave-lg amendment below), written as the same `(path, caller bare name, callee bare name)` pairs (§1.4), and scored
against the **servable truth**: the scip-go call pairs whose callee symbol the index itself defines in the subject
(`langbench.SCIPResult.CallInIndex`). A truth pair on a standard-library or dependency callee is a call the
service has no node for and the product cannot serve; it leaves the truth, and each subject's note counts it
(13,674 of 28,582 pairs over cobra and cli/cli).

**Why.** L2 is a claim about what a caller of the index can rely on; the product answers "who calls X" from the
served graph, and the typed pass (go/types, independent of scip-go) is that graph's resolver for Go. Grading the
syntactic list alone measured a set no tool reads, and it hid two things the served graph shows: a syntax-only
edge binds a dependency's call to the one in-service node of that bare name (`assert.Equal` to a service's
`Equal`, `mu.Lock()` to a mock's `Lock`: 1,706 pairs, 789 of them `Equal`), and a type reference is an edge
(1,533). The graded numbers are lower than option A's, because they are the product's.

**Dispatch and unresolved.** A dispatch edge to each of N implementers is N edges, but written as pairs it is one
pair per distinct (caller, method name): scip-go's single interface-method term (O261) and the N edges meet at the
name (6,541 dispatch edges collapse to 1,371 pairs). An `EdgeUnresolved` edge names no callee (1,665) and gives no
pair; an `EdgeInstantiates` edge is a fact about a caller, not a call (10,128); a call to a symbol the subject has
no node for is left out of the truth, as above. Each class is counted in the reading's notes, none hidden.

**Every option, both subjects together** (cobra 40b5bc14 and cli/cli 45ffa3c6; pairs found, then precision and
recall against the servable truth of 14,908 pairs; the last column grades the set against the full truth of
28,582, dependency calls included, as the readings did before this wave):

| Option | Found | Precision | Recall | Against the full truth |
|---|---|---|---|---|
| A: goextract's syntactic `Calls` | 34,292 | n/a: it holds the dependency calls the servable truth lacks (.4259) | .9797 | .8205 / .9844 (the 2026-10-01 reading) |
| A resolved: the syntactic `Calls` that reach a node (`callgraph.Build`) | 14,603 | .7480 | .7327 | .8526 / .4356 |
| B: the typed edges alone | 13,957 | .9805 | .9180 | .9934 / .4851 |
| C before wave lg: served (`BuildBoth` at 90f0a01f), graded in wave le | 17,774 | .7794 | .9292 | .8754 / .5444 |
| D: C without the syntax-only edges to a type (LB-Q-go-rest's trigger) | 15,968 | .8655 | .9271 | .9720 / .5430 |
| E: C without the syntax-only edges into a method from a file the typed pass resolved | 16,162 | .8524 | .9241 | .8656 / .4895 |
| F: D and E together | 14,356 | .9574 | .9219 | .9719 / .4881 |
| **C since wave lg: served (`BuildBoth` with E and D in the graph), graded** | 14,357 | **.9573** | **.9219** | .9719 / .4882 |

Option B is the typed pass read alone; it reads excellent, and it is the default answer of `blast_radius` and
`find_callers` for a service with typed edges (O475), but not what `symbol_context`, `blast_radius` with
`include_heuristic`, verify and review read, because those also serve the syntactic edges. Before wave lg the
graded set C read **L2 unmet: precision .7794** (good is .85,
excellent .90), recall .9292 (excellent). The 3,921 pairs C holds that the truth lacks are 1,706 dependency calls
bound by name to an in-service node (a syntax-only edge: `callgraph.resolve`'s unambiguous bare-name rule), 1,533
type references (kept by design, below) and 682 others; the 1,055 truth pairs it lacks are 872 calls through a
func-typed field, variable or parameter (the typed pass records them `EdgeUnresolved`) and 183 on a callee that
has a node and no reaching edge (an interface method called where no in-service type implements the interface,
`http.Client`'s `Do` behind a local interface, is the sampled shape).

**Option D, evaluated and not shipped in wave le** (shipped in wave lg without a table, below). D reads .8655 / .9271 at L2 (good). Its product side was measured in a
scratch copy of the call graph that drops the syntax-only edges to a type and adds a method-to-type edge (a type
is reached when one of its methods is): BC-7, OC-1 and OC-13 verify again (the earlier failure was the type
reference leaving without anything to replace it), and so does every row of the 90 plans (2,664 rows, 358
failing before and after, no row changed). Not shipped, for a reason the rows do not show: the 1,806 caller-to-type
edges D removes (17,774 less 15,968) are the consumers `blast_radius`, `callers` and `rename_impact` report for a
type, and a type's consumers are what breaks when it changes; they need the reference channel that is behind the
table freeze until RT-M1 reads PASS, and the change is in `internal/analysis/callgraph`, outside this item.
**This departs from LB-Q-go-rest's written trigger**, which named D (or a reference channel) as the thing to wait
for before re-measuring L2: L2 is now re-measured under the served set without waiting, and the trigger is
rewritten to E and D below.

**Revisit when** (each re-measures with `lang-oracle`, which carries every option in its notes, so no harness
change is needed): (1) `callgraph` stops binding a syntax-only method edge from a file the typed pass resolved
(E; `ProductReach.walk` already discounts it): expected .8524, good; (2) a reference channel apart from `Calls`
lands (RT-M1 PASS lifts the table freeze), which lets D ship: D and E together read .9574 / .9219, excellent;
(3) the typed pass or the index gives a call through a func-typed field a callee (872 pairs of recall). If the
reading's graded set is ever changed back to A, say why here. *(1) and (2) fired in wave lg; (2) without the
table: the reference channel D waited for needs no store row, below.*

**Amendment (wave lg, 2026-10-02): E and D are the product; owner-architect decision (b).** The owner-architect
accepted this section's decision (ADR-048 amendment 2026-10-02, decision (b)): Go's L2 grades the calls the
product serves. Wave lg made the graded set the product, in `internal/analysis/callgraph` itself, so every reader
of `BuildBoth`, of `Build` plus `AddTypedEdges`, and of a package-patched graph (`ReplacePackage`, so
`CachedCallGraph.InvalidatePackage` and `memo.PatchPackage` stay equal to a fresh build) gets it:

- **E (LB-F199).** `AddTypedEdges` ends by dropping every syntax-only edge into a Go method whose caller sits in a
  file the typed pass resolved (`dropTypedFileGuesses`): the typed pass compiled every call site in that file and
  answered each with a typed edge, an unresolved marker, or nothing for a callee outside the service, so the
  syntax pass's bare-name binding there is a guess (`bytes.Equal` or a `sync.Mutex`'s `Lock` bound to the
  service's one `Equal` or `Lock`). `core.Edge` carries no call site, so the caller's file stands for the site: a
  build-tag-excluded file, a cgo file the pass did not compile, or a package that failed to load is not in
  `typedFiles` and keeps its syntax edges; a compiled file whose code yields no typed edge at all is not either,
  and keeps its guesses (the residue the next trigger names). `ProductReach.walk`, which applied E to product
  reach alone since EW-15, answers as before.
- **D (LB-F200).** A syntax-only edge whose callee node is a Go type or interface is a reference, labelled
  `core.EdgeTypeRef` ("type-ref", heuristic in `core.EdgeConfidenceOf`), classified from the callee node's kind,
  which the graph fixes at build, so it costs no map entry and no store row. `CallersDetail`, `CalleesDetail`
  and the L2 pairs leave it out; `ReferrersDetail` and `ReferencesDetail` return it, and `DependentsDetail` and
  `DependenciesDetail` return calls and references together, each labelled. The reach walkers (`TestImpact`,
  `TestImpactDetail`, `Reachable`, `CallPath`, `RenameImpact`, `DeadCode`, `ForwardReach`, `ProductReach`) walk
  the adjacency, references included, so verify's test_reaches, edge_wired and product reach read as before.
  The 1,806 caller-to-type edges stay a type's consumers: `blast_radius` keeps them in the radius and names them
  (`references`, and `edge_kind: "type-ref"` on the by_depth row), `symbol_context` lists them under
  `referenced_by` (and a symbol's types under `references`), and the review engine's changed-symbol callers,
  the layer diff, findings neighborhoods, the console's neighborhood and the preflight blast radius read the
  dependents views. No table, cache or background job: the table freeze holds.
- **Surfaces (O475).** `symbol_context`'s callers, callees and related tests read GraphBoth since wave lg (they
  read the syntax graph alone, option "A resolved"); the default `blast_radius` and `find_callers` answer for a
  service with typed edges stays the resolved graph (option B); `find_callers` with `include_heuristic` still
  lists the store's raw `Calls` matches.
- **Measured** (`lang-oracle`, cobra 40b5bc14 and cli/cli 45ffa3c6, scip-go 0.2.7): the served set C reads
  **.9573 / .9219, L2 excellent**; level_met L2, L3 unmet on file_edge_precision .5625. C is F less one pair:
  cli/cli's `TestNormalizeReference` is defined under one key in `artifact_posix_test.go` and
  `artifact_windows_test.go`, the graph holds one node per key, and that node is the build-excluded Windows
  file's, so E keeps its edge while F, filtered per file, drops the POSIX copy's. The reading's notes carry A,
  A resolved, B, the pre-lg C (built explicitly), D, E, F and C.
- **Verify-neutral** (O478): `fathom verify --strict` over the 90 plans on a fresh scratch ingest before (90f0a01f)
  and after reads the same 2,694 rows with the same criteria (355 failing), plus the wave's four rows passing:
  the reach walkers still walk a reference, and no claimed row's test reached its symbol only through E's class.

*Revisit when:* (3) above; `core.Edge` gains a call site (E then applies per site, and a compiled file that
yields no typed edge keeps no guess); a served surface reads a graph other than O475's map names (the map and
this grading are re-recorded); or `find_callers`' heuristic rows move off the store's raw `Calls`.

### 1.7 Go's L3 and L4 (wave lf1, item go, 2026-10-03)

Go read L3 unmet (file_edge_precision .5625) and L4 unmet (def_mapping .4867, typed_recall .5637) before this
wave. `-unmatched` now carries Go's file edges, each fathom-only pair with the kinds of typed edge that drew it, and
the reading's notes count every class below, so each rule is a count and a disclosure, never a silent cut. No served
edge class left the product: the typed pass, `goextract` and the served graph are unchanged (no `ExtractorEpoch`
need), and L2 reads as before (.9573 / .9219).

**L3, fathom side (LB-F297, LB-F298).** A file edge is a reference the source writes to a definition in another file
(§1.2, as every other language draws it). Go's file edges are the typed pass's edges, and the pass also stores two
kinds no source line writes: an `EdgeIfaceDispatch` edge to *each* implementer of an interface method (the call
names the interface's method, which lives in the interface's file) and an `EdgeInstantiates` fact (a value of a
type the caller holds, written or inferred). On cli/cli the 3,175 false file pairs were 2,580 drawn only by dispatch
edges, 578 only by instantiation facts, 8 by both, 4 by a direct call and 5 by a function value (the 1,630 pairs from a `_test.go` file and the 48
between files of one directory are the same kinds; build tags, generated code, cgo and vendored code drew none). Go's file-edge set is the edges that name
their target at a site: `EdgeDirect` and `EdgeFuncValue`. The file edges only the other two kinds draw (cli/cli:
4,097 pairs of 7,136, 880 of them in the truth; cobra 13 of 132, 8 in the truth) are left out of `file_edge_precision` and
counted in each subject's notes, with the pairs by kind-set (`goFileEdgeClasses`). *Reason:* the dispatch fan-out and
the held type are product facts about what may run and what is held, served by the call graph; they are not
file-granularity references, and scoring them as such measured a different relation than every other language's
file edge. *Cost, counted:* recall loses the 880 pairs the instantiation facts drew that scip-go also has (a type
named in a declaration), read .7034 afterwards.

**L3, truth side (LB-F298).** scip-go defines a package symbol at the package clause of *every* file of the
package, so a reference to the package (an import spec, the `b` of `b.Foo`) names no one file; the index's first
document of the package stood in for it. A reference to a symbol the index defines in more than one document draws no
file edge (the same rule that keeps scip-typescript's ambiguous module file out of the import truth, `defDoc`),
gated to scip-go and counted (`scip_file_edges_multidef`: 1,106 file edges on cli/cli, 11 on cobra, edges only
such a reference drew).

**L4, the subject (LB-F299).** `scip-go index ./...` also indexes the testmain `go test` writes into the Go build
cache for each test package: a document whose path leaves the subject (`../../cache/ab/…-d`), holding an `init()`,
a `main()` and the calls into the package under test. It is no file the subject holds (§1.4: the oracle is judged
over the files fathom reads); the documents are dropped before any count and counted (`scip_go_docs_outside`: 204
on cli/cli, 2 on cobra). scip-go also writes a function literal's parameters in a package initializer, and some
function-scoped locals, as a term of a meta descriptor (`Options:c.`) instead of `local N`: neither the descriptor
rule nor the local rule reached it; it is a parameter (S1, `GoScopedTerm`), out of both counts, counted
(`scip_edges_param_target`: 21 references). Every definition and unmapped endpoint left is counted by descriptor
class in the notes (`scip_go_defs_*`, `scip_go_unmapped_*`).

**L4, what stays unmet, and why it is an owner decision (LB-F300).** On cli/cli 4,236 of the 4,967 definitions with no
fathom symbol are struct fields (and the interface methods scip-go writes the same way) and 728 are the package
symbols; 28,084 and 14,101 of the 42,232 unmapped endpoints of `typed_recall` are references to a field and to a
package. Neither ADR-048 nor §1.4 makes fields-as-symbols a cross-language rule (Rust members, Python attributes and
fields and Java and C# members were each added per language, LB-F82, LB-F241, LB-F242), so `goextract` is unchanged
and the Go field decision is the owner's (O606); the per-file package symbol is the question Python's module
definitions left open (O527; O607). Arithmetic over the counts, never graded: def_mapping reads .8837 without the field
class and .9995 without fields and packages; typed_recall .7931 and .9991. Grading `typed_recall` against fathom's
own go/types edges instead was measured and refused: the native pass is a call graph, and of the 25,299 unique
(from, to) pairs the import stores between mapped definitions it draws 15,337 (.6062; 3,779 of them as an
instantiation fact, a held type), because it stores no reference to a type or a variable (O608). That coverage is
disclosed beside the graded figure, which stays the SCIP import's (§1.2).

*Revisit when:* the owner decides Go field symbols (a member pass as for Python, `Owner.name`, a `field` kind outside
`oracle_kinds`; needs an `ExtractorEpoch` bump) or the per-file package/module symbol (O527), then re-measure;
the typed pass stores a type or variable reference as an edge (then `EdgeInstantiates` and dispatch are no longer
the only kinds to weigh here and the native coverage can be graded); or `core.Edge` carries the interface of a
dispatch edge (the dispatch pairs then map to the interface method's file, which scip-go writes, and recall gains
them).
*Fired 2026-10-10* (K17 and K18, owner-architect, 2026-10-09): a Go package is a counted namespace (§1.4's
amendment of that date: def_mapping .5891, typed_recall .6713, L4 still unmet on the field class alone); the member pass
was built, read L4 excellent and was killed by its cost gate (§1.4, O673), so the field class stays the owner's call.

### 1.8 Owner rulings K22, K23 and K26 to K29, and the web rulings W1 to W4 (ADR-067, 2026-10-10, committed before any holdout reading, K19)

Three owner rulings of 2026-10-09 and 2026-10-10 were recorded only in an operator note. They are committed here so
every record that cites them resolves, with the three cost and reading rulings of 2026-10-10 that follow K26:

- **K22 (owner, 2026-10-09).** Fathom's automatic passes never fetch or run a toolchain a repository names: every
  automatic `go list` and packages.Load runs with `GOTOOLCHAIN=local` (LB-F341), and cgo flag safety rests on Go's
  own allowlist. Explicit user verbs (`doctor --stranger`, `receipt verify --run-tests`) may run repository code
  because the user chose to. Oracles run in the measuring harness only.
- **K23 (owner, 2026-10-09).** Language order follows the queue's first-ranked languages, and the generated dispatch
  column of `docs/languages.md` (F.46) was its first item. The ranking is the queue's, not any adopter's private
  list (ADR-011).
- **K26 (owner, 2026-10-10, amending K17's cost gate).** A change over +20% store or +15% wall ships only with an
  owner ruling recorded beside its metric gain. Absent that ruling, it is reverted. The TS and JS module symbol
  (O748, +33% and +37% store) is the first ruling under it.
- K27 (owner, 2026-10-10): a cost gate reads against a baseline that did the work; when the baseline's output for the measured scope was empty (a skipped or failed pass), the cost is read on a repository where the pass already worked and the other is disclosed as previously skipped, never counted as a regression (first case: the nested-module typed pass, EW-59/EW-60, +17% wall on fathom, accepted).
- K28 (owner, 2026-10-10): the cost gate sums per wave and per language against the last ruled baseline, so several changes under the bar cannot add up past it unruled; TS/JS cumulative store at 9e21be2b against e186fbc3 (nest +51.5%, zod +62.2%) is ratified beside its gain (typed_recall .14 -> .94, TypeScript L4 excellent on a holdout).
- K29 (owner, 2026-10-10): a harness rule change re-reads every committed reading it touches in the same wave, or marks each one stale with a row naming the rule (K25 ruling 3 changed file-edge truth under readings that were never re-read, O790, O806).

The web rulings are named W, so they cannot be read as ADR-056's program rules R1 to R8:

- **W1.** HTML's L2 is same-document ID references: an attribute the HTML standard types as an ID reference or a
  list of them (`for`, `aria-labelledby`, `aria-describedby`, `aria-controls`, `aria-owns`,
  `aria-activedescendant`, `headers`, `list`, `form`, `popovertarget`) and `href="#id"`, from the referring element
  to the element carrying the id. Oracle: parse5's tree with the attribute list taken from the standard's own index,
  tier A. An unmatched reference counts against recall. A duplicate id is excluded and counted.
- **W2.** lightningcss (built on Mozilla's cssparser) is tier A for what a browser parser attests: rule and at-rule
  definitions, their ranges, custom property definitions and the `@import` graph (the precedent of section 1.3's
  runtime trace, tier A only for what it attests). It attests no cascade, so `var()` and `animation-name`
  references grade as name references within the import closure, and the reading says so.
- **W3** (O482, owner 2026-10-10): the oracle-blind cap applies per subject, so a large typed subject cannot lift
  untyped code past its own grade; a JavaScript L2 or L4 claim names each untyped subject's own precision.
- **W4.** A format's own compiler or linter is its reference parser, tier A as section 1.3 already says, for what it
  attests: `@vue/compiler-sfc`, `svelte/compiler` `parse`, `@astrojs/compiler`, `@angular/compiler`
  `parseTemplate`, and the Jenkins declarative linter (run against a local controller in the measuring harness
  only, never by fathom). A tags query is tier C for these formats and caps them at good.

**W1's pair key and scope (K19, wave lf5, LB-F441; committed with the holdout
`calibration/corpus.d/html-holdout.yaml` before any reading under it).** A W1 pair is keyed by the
document: (file, the file's module symbol `<module>`, the id), never by the referring element's
enclosing definition. The HTML standard resolves an ID reference against the element's tree, the
first element in tree order within that tree whose ID is the value, so the referring element's
ancestors take no part in it, and the referring element itself is seldom a definition (a `label`,
an `input` with no id). Keying by an enclosing element would name an arbitrary ancestor, an
element with an id that happens to wrap the reference, as the reference's owner. The parse5
oracle writes every pair from `<module>`, the dialect gives every reference to the module symbol
(`calls.owner "module"`), and the pair is counted once per document (a set, §1.4). Two scope
rules, both html-only: HTML's pairs are made by the html dialect's own kinds, so a `<script>`'s
JavaScript calls are JavaScript's L2 and leave HTML's, counted (`html_injected_call_pairs`); and a
reference to an id two or more elements of the document carry leaves both sides, counted (W1).
Ids are compared as written (`name_match "identity"`): an id may hold `.`, `:` or `/`, which the
bare rule would cut. L2 is graded under §1.2's `call_recall` and `call_precision` thresholds in
this pair unit, and the reading says so. L3 is lychee `--offline`'s resolved local link targets
(tier A for what it attests, §1.3) scored as the dialect's `import_resolution` and, with no SCIP
index, its file-edge truth (LB-F57); a target outside the measured `.html` and `.htm` files (a
script, a stylesheet, an image) is out of scope, counted, never a miss (§1.4 Scope).

**Owner ruling M2: a JSX element is a call (K19, wave lf6, LB-F481; committed with the TSX holdout
`calibration/corpus.d/typescript.yaml` before any reading under it).** `<Foo .../>` compiles to
`jsx(Foo, props)` (`React.createElement(Foo, props)` under the classic runtime) and the renderer
invokes Foo, so the component graph is the call graph agents need, and the dialects already record an
element as a call to its tag (LB-F377). The scip-typescript call truth, until now a reference
written `name(` (LB-F174), also counts a reference occurrence at an opening or self-closing
element's tag that names a component as a call pair to the name written there, in a document
TypeScript parses with JSX (its JSX language variant: `.tsx`, `.jsx`, `.js`, `.mjs`, `.cjs`; never
`.ts`, `.mts` or `.cts`, where `<Foo>x` is a type assertion). TypeScript's own rule decides what is
a component: an identifier tag starting a to z, or holding `-` or `:`, is an intrinsic element (a
string, no value reference, so no pair); a capitalized or dotted tag is a value reference. A member
tag `<Foo.Bar>` calls its last segment, Bar (Foo is read, as `foo.bar()` calls bar), which is the
name the dialects record (the bare last segment, §1.1 L2). The opening tag alone counts: a closing
`</Foo>` is the same element and never a second pair, a self-closing element counts once, and a
fragment `<>` names nothing. What is written decides: the tag follows a `<` that no identifier, `)`
or `]` stands against (that `<` opens type arguments, `useState<Foo>(`, or is a comparison), and
is followed by its own type arguments, then `>`, `/>`, or a space and an attribute or a spread; the
end of the line closes a tag only when no operand stands before the `<` (`ok = i < items.length` in
a file without semicolons is a comparison). The pairs are counted (`scip_jsx_element_calls`) and
disclosed in each subject's notes, and an element is a written call site for the oracle-blind rule
(LB-F175), so a member tag on a receiver the producer cannot type is unjudged, as at `name(`. A
component bound to a wrapper call (`const X = React.forwardRef(...)`), which scip-typescript kinds
a variable, is not ruled here: D3's wrapper class stays the owner's (LB-Q-typescript-rest).

**M3, C's linkage rule for scip-clang's file-edge truth (owner ruling on O863, K19, LB-F487;
committed with the holdout `calibration/corpus.d/c.yaml` `holdout:` and its criterion before any
index of it).** A reference to a file-local function or object, an identifier a file-scope
declaration writes `static` (internal linkage, C11 6.2.2p3), resolves to the definition in the
referring document's own translation unit: the referring document itself, which draws no file
edge, or a header its `#include` lines reach by the build's preprocessor oracle (`imports`). It
never resolves to another unit's same-named copy, because within each unit an identifier with
internal linkage denotes that unit's own entity (6.2.2p2). scip-clang 0.4.0 names every copy
alike, so K25 ruling 3 drew each libuv test file's own `close_cb` to the 72 other files defining
one. An external-linkage symbol keeps ruling 3 (every definer), and so does a reference whose
includes reach no copy when no definer is external (a header naming a static its includers
define). Linkage is the build's own compiler's AST (`calibration/oracles/clang-ast-defs`
`internal_linkage`, tier A), not the written `static`: the AST carries 6.2.2p4 and p5 (a
definition written without `static` after a `static` prototype), a `static` a macro writes, and
each declarator of `static int a, b;`, none of which a token before the name shows. Only the
file-edge truth changes: the pairs left out that nothing else draws are counted
(`scip_file_edges_internal_linkage`) and disclosed beside `scip_file_edges_every_definer`;
def_mapping and typed_recall do not read it. Scope: dialect c alone (`SCIPOptionsFor`'s c branch),
so cpp.json and every other reading are untouched by construction, and K29 re-reads c.json and
the C holdout. C++ has the shape too (a namespace-scope `static`, a member of an unnamed
namespace), but its internal linkage also takes in namespace-scope `const` objects, so it needs
its own ruling and holdout. Criterion, pre-registered: on the holdout, file_edge_recall >= .50 and
file_edge_precision >= .95 (§1.2's L3 excellent bars: the two metrics the rule feeds, as for
LB-F473 and LB-F475).

**Shell (wave lf6, item shell; LB-F454, O792, O793).** zsh routes to no grammar: on ohmyzsh at a pin the best
maintained zsh-capable grammar leaves an ERROR or MISSING node in 15.9% of `.zsh` and zsh-shebang files and
tree-sitter-bash in 66.4%, against §14.2's 5% (`docs/bench/readings/lang/zsh-grammars.md`), so `.zsh`, `.ksh` and their
shebangs stay unread and disclosed until a grammar passes on the same corpus. An extensionless script the shebang
rule dispatches (D4) is a file of its dialect's language in the file-resolve pass, as importer and as target: the pass
reads the shebang itself, since the resolver's language table is keyed by extension. A shell script's import paths
are read against a current directory no static reading knows, so for Bash the file-symbol strategy's order stands
(`./` and `../` against the script's directory, any other path against the script's directory then the repository
root) and its oracle names the operator's choice (the script's directory, then the subject root), tier B; for
PowerShell the bases are the language's (`$PSScriptRoot` and `using module` the script's directory, a manifest's
modules the manifest's), tier A for those, and a current-location path is counted and in no truth. Both rules mint a
module symbol for each script's top level (K18). Bash's ships under owner ruling S1 (wave lf7, O969), L3 good its
ceiling (tier B); PowerShell's, its module keys fixed under S2 (O915) and read once on a fresh holdout, ships under owner
ruling S5 (wave lf8, O970, O1053), L3 excellent on its tuning subjects and level_met L3 on the holdout (tier A).

**Owner rulings S1 and S2 (2026-10-11, wave lf7, item shell).** S1 accepts Bash L3 under K26 beside its gain (L3
not-attempted to good; +38.0% store and +21.9% wall against e7bbd99e, +36.2% and +21.8% against c39875f3), with one lever:
a script's top-level call (a call its module symbol makes) is stored only when its callee is a definition of the
repository, the script's, a sourced file's or any other the service defines, since an external command (`echo`, `grep`)
carries no navigation value; the filter runs where every definition is known, at the end of ingest's extraction stream
(LB-F520; +33.4% store and +18.0% wall). S2 fixes O915 by a package strategy: `package.strategy` "file" sets a
definition's package to its file's base name with the extension (`X.psd1`, `Button.vue`, `+page.svelte`), so a dialect
whose files share a stem in one directory keys a module symbol per file (LB-F519), then reads PowerShell on a fresh
holdout and gates its cost; PowerShell's +33.5% store held it for a ruling (O970), and owner ruling S5 (2026-10-15)
ships it under K26 on S1's reasoning, S1's lever widened to it (+27.7% store against 99aa205a, O1053). A stem two dialects share
(`foo.component.ts` beside `foo.component.html`, `index.ts` beside `index.html`, C++ `foo.hpp` beside `foo.cpp`) keeps
one key until a ruling on re-keying existing dialects, which moves every key of each (K29 on every reading, O968).

**Owner ruling S3: TypeScript's module-level bindings and anonymous type-literal members (K19, wave lf7, LB-F486,
LB-F511; committed with the TSX holdout `calibration/corpus.d/typescript.yaml` before any index of it).** (a) A `let` or
`var` at module scope or exported, in a `.ts` or `.tsx` file, and each name a destructuring one binds, is a definition,
JSConst, as ruling M1 made JavaScript's (LB-F470): scip-typescript defines each as a global term of the file, and a
let-style codebase binds its components that way (headlessui's 220 `export let Button = forwardRefWithAs(ButtonFn) as
...` bindings were O901's unmapped definitions). (b) A member of an anonymous type literal leaves def_mapping's
denominator, counted (`scip_defs_type_literal`) and disclosed per subject. scip-typescript's descriptor grammar names
every type literal by its file's counter, `typeLiteral<n>:`, under the node that holds it
(`ButtonProps#Props:typeLiteral1:disabled.` for a type argument's `{ disabled?: boolean }`,
``ButtonFn().(`{ value }`)typeLiteral7:value.`` for a destructured parameter's annotation), and TypeScript names no such
type (its checker's symbol for one is the anonymous `__type`): the members are D2's object-literal properties on the
type side, structural and declared by no named declaration. The innermost literal decides. A type alias's own body
(`type Own = { open: boolean }`, `Own#typeLiteral0:open.`; the only place a type literal stands directly under a type
descriptor, since a class, an interface or an enum holds none of its own) is the declared body of a named type, written
interchangeably with `interface Own { … }`, and the dialect names its members (LB-F377's field pattern), so they stay;
a literal nested in it (`Own#typeLiteral0:nested.typeLiteral1:deep.`) is anonymous. A member the dialect names
anyway (a method signature, which it names in any object type) leaves the numerator alike, counted
(`scip_defs_type_literal_named`). typed_recall is unchanged, its references to such members kept (D2's other half
stays the owner's, O747). Scope: dialect typescript alone. A JavaScript document writes a type literal only inside JSDoc
(`@typedef {{ a: number }}`, `@param {{ x: number }}`), which scip-typescript 0.4.0's indexer never walks for a
definition (measured on a fixture: the members' references are local symbols), so a JavaScript reading's measured
documents hold none, and the `.ts` and `.d.ts` documents beside JavaScript are outside its scope already (ruling L5);
the JavaScript readings are untouched by construction. The holdout, chosen before any index: palantir/blueprint
@blueprintjs/core@6.21.0 (c5a3873b, Apache-2.0), its source directory `packages/core/src` (299 files, 218 `.tsx`)
under the package's own compiler options; radix-ui/primitives, first in order, has no package directory of 200 files
(the shard's comment gives the vetting). Criterion, pre-registered: the holdout's `level_met` is L4.

**Kubernetes L2 and L3 graded by Kubernetes' own label selectors (K19, wave lf8, item k8s, LB-F456; committed
alone before kubernetes.json is re-read).** Verified at row start against `k8s.io/apimachinery` v0.37.0 (the
version the oracle module's Prometheus pin already requires): `labels.ValidatedSelectorFromSet` is the selector a
Service's `spec.selector` map is (exactly its pairs, the API server's label validation applied; a nil or empty set
is `Everything()`), `metav1.LabelSelectorAsSelector` the one a `matchLabels` and `matchExpressions` selector is
(`In`, `NotIn`, `Exists`, `DoesNotExist`; nil is `Nothing()`, empty `Everything()`), and `Selector.Matches` decides
whether a label set is selected. That is tier A for one decision, whether a selector selects a label set (§1.3: tier
A only for what it attests, the runtime trace's precedent). It attests no namespace an applier gives a manifest, no
object a cluster holds and no name reference. The rule, family by family:

- *L3, a Service's selector joins: tier A.* A core v1 Service selects every object whose pod labels its
  `spec.selector` matches under `labels.ValidatedSelectorFromSet` and `Matches`. The pod labels are the manifest's
  own: a Pod's `metadata.labels` and any object's pod template labels (`spec.template.metadata.labels`, a CronJob's
  `spec.jobTemplate.spec.template.metadata.labels`), each document read by apimachinery's own decode
  (`utilyaml.NewYAMLReader`, `unstructured.Unstructured`), a label map with a non-string value no label set,
  counted. A Service with no selector or an empty one selects nothing (the Service API: "If empty or not present,
  the service is assumed to have an external process managing its endpoints"), and one the label validation refuses
  selects nothing, counted. *The namespace rule:* a Service selects only objects of its own namespace, compared as
  written, an absent `metadata.namespace` read as `default` (the namespace `kubectl apply` gives an unnamespaced
  object under an empty kubeconfig, `KUBECONFIG=/dev/null`, the oracle's own environment, and the `scope_default` of
  fathom's `label-selector` resolver). A new oracle file (`calibration/oracles/kubernetes/selectors.go`,
  `-selectors-out`) writes these pairs at tier A, and the yaml.v3 walk's own selector pairs leave its tier-C file, so
  no pair is scored twice; the walk's agreement is disclosed, never scored.
- *L3, the route pairs: tier C.* The pairs LB-F99 and LB-F315 score into the same xref family (Ingress and Gateway
  API backends, Istio destination hosts, subsets and gateways, ServiceEntry hosts, Probe targets, and the side
  pass's ServiceMonitor, PodMonitor and AuthorizationPolicy selectors) join by DNS and object-name rules and, for the
  side pass, in the nearest scope holding a match, which no Kubernetes library evaluates offline: a monitor's or a
  policy's label match is apimachinery's, its `namespaceSelector` and nearest scope are not. They stay the walk's,
  tier C, and istioctl analyze's cross-check of Istio's stays disclosed and unscored. The xref family grades at the
  weaker of its tiers (`langbench.WeakerTier`).
- *L2, the name references: tier C.* `serviceAccountName`, `envFrom` `configMapRef` and `secretRef`,
  `configMapKeyRef`, `secretKeyRef` and a volume's `configMap` and `secret` name an object no offline Kubernetes tool
  resolves: the API server's ServiceAccount admission and the kubelet resolve them in a cluster, and `kubectl create
  --dry-run=client` needs API discovery and fails offline. They stay the walk's, tier C.
- *The bar.* L3 reads excellent only if every graded family is tier A and meets §1.2's tool-format bars (precision
  .95, recall .80). Otherwise the reading carries a note beginning `L3 ceiling: ` that names the family and why,
  written by the oracle as a `disclose` note (the GitHub Actions reading's precedent, LB-F457), never by a hand edit
  of the reading. kubernetes.json is re-read once on its six committed subjects (`calibration/corpus.d/kubernetes.yaml`),
  the harness reproducing its committed counts first. K29's scope is kubernetes.json alone: the output of
  `labeljoins.go` and `mesh.go` stays byte-identical, so the readings TL-8 and TL-16 read are untouched.

**W4 for Vue and Svelte (K19, wave lf7, item vuesvelte; LB-F534, LB-F535; committed with
`calibration/corpus.d/vue.yaml` and `svelte.yaml` before any reading under them).** A single-file component
is one module: its `<script>` blocks are TypeScript or JavaScript the dialect injects, and the file hosts
them (F2, `injection_host`): their imports are the file's and their top-level calls its module symbol's,
the component (`module_symbol`, keyed by the file's base name with its extension, F1, so `Button.vue` and
`button.ts` beside it, or `+page.svelte` and `+page.ts`, are two modules). The oracles are the frameworks'
own compilers, tier A for what they attest (W4): `@vue/compiler-sfc` (`calibration/oracles/vue-compiler-sfc`)
and `svelte/compiler` `parse` (`calibration/oracles/svelte-compiler`). Subjects: element-plus 2.14.7 and
vuejs/vitepress v1.6.4 (vuejs/docs is CC BY 4.0, no OSI licence, §13.3), sveltejs/kit 2.70.3 and
shadcn-svelte 1.7.0. The rules, each from the framework's semantics:

- *L1, kinds and denominator.* The definitions are the top-level declarations of each script block as the
  compiler's own parse reads them: Vue's `compileScript` `scriptAst` and `scriptSetupAst`, the trees its
  `bindingMetadata` is computed from (the compiler's `@babel/parser` with its plugins when `compileScript`
  throws), Svelte's `instance` and `module` programs. A function declaration and a `const` initialized by an
  arrow or function expression are `function`, a class `class`, any other `const` declarator (each name of a
  destructuring) `constant`, a `let` or `var` `variable`, a TypeScript interface, type alias or enum their own
  kinds. The file's definitions on fathom's side are its injected script's, with their TypeScript and
  JavaScript kinds (the vue and svelte manifests name those kinds; `sharedSourceKindGroups`), and its module
  symbol, outside every oracle kind. An import binding (in `bindingMetadata`, no definition of the file) is
  out of both counts; `variable` is outside the kind intersection because the TypeScript and JavaScript
  dialects define no `let` or `var` (an exported `let` is a `JSConst`), counted (`tool:variable`): Svelte 5's
  `$state` and `$props` are `let`s, the residual a reading names. *Read 2026-10-11 (svelte.json, LB-F540): the
  premise holds for TypeScript alone; the JavaScript dialect names any lexical declaration, a `let` too, a
  `JSConst`, so kit's 266 JavaScript `let`s are definitions the compiler's kind keeps outside the
  intersection, and Svelte's L1 is unmet (sym_precision .8395). The rule is not changed after its reading
  (K19); LB-Q-svelte-rest carries the trigger.*
- *L2, the pair unit.* One pair per (file, `<module>`, component name) for each template element the
  compiler itself types a component, owner ruling M2's principle (§1.8: an element is a call when it names a
  component, an intrinsic element is a string and none). Vue: an `ElementNode` whose `tagType` is
  `COMPONENT` in `descriptor.template.ast` (compiler-core `isComponent` under compiler-dom's options: a tag
  none of `@vue/shared`'s HTML, SVG and MathML lists names, not `<slot>`; Vue's own `Transition`,
  `KeepAlive`, `Teleport`, `Suspense` included), named as `resolveComponent` resolves it,
  `capitalize(camelize(tag))` (`el-input` names `ElInput`; the dialect's `names.pascal_case`). Svelte: a
  `Component` node (a capitalized or dotted tag). A dotted tag pairs by its last segment, as M2 rules for
  `<Foo.Bar>`. A dynamic component (`<component :is>`, `<svelte:component this>`) and `<svelte:self>` name
  no component of their own: out of both sides, counted. The pair is keyed by the document, as W1's: the
  template is the component's render function, one per file, so the component, never an enclosing element,
  renders each element; a component rendered twice in a file is one pair (a set, §1.4).
- *Scope.* An injected `<style>`'s CSS or SCSS is CSS's own L1 and L2 (its kinds are outside the
  intersection; its pairs are not the module symbol's). A script's calls are TypeScript's or JavaScript's own
  L2, graded by their own readings, and out of the component pair set: fathom's side keeps the module
  symbol's pairs alone (html's `htmlOwnCallers` precedent), and a name a script block calls that no template
  component of the file names leaves both sides, counted (`call_exclusions`, reason `script-call`), since F2
  gives a script's top-level calls to the module symbol. A call in a template expression (an interpolation,
  a directive's value, Svelte's `{expr}`) is not extracted and in no truth. An SFC custom block (`<i18n>`,
  `<docs>`) is never rendered: the vue dialect names it a block, and no markup inside it is a call.
- *L3.* The unit is an import statement (a static `import`, or a re-export with a source) of a script block
  whose specifier is relative, resolved by Vite's rule, the bundler `create-vue` and SvelteKit run: the path
  as written, a TypeScript block's `.js` spec as the `.ts` it is emitted from, Vite's default
  `resolve.extensions` (`.mjs .js .mts .ts .jsx .tsx .json`), then a directory's index; never an implicit
  `.vue` or `.svelte` suffix, which Vite's default list holds neither of (so the dialects read relative
  specs by relative-path's TS/JS rule, which appends none either). An alias specifier (`@/`, `~/`, `#`,
  `$lib`, `$app`) is bundler configuration: out of scope, counted; a package is external. `import_resolution`
  counts the imports whose target is a measured file (`.vue` for Vue, `.svelte` for Svelte); a `.ts` or `.js`
  target is outside the measured files, out of scope and counted, as HTML's scripts are, and the file-edge
  truth is the imports' (file, file) pairs between measured files (LB-F57), tier A.

**Astro components and Angular templates against their compilers (K19, wave lf7, item angularastro, LB-F543;
committed with `calibration/corpus.d/astro.yaml` and `angular.yaml` before any reading under them; owner rulings S4 and
W4).** *Angular* has no template grammar that parses its templates: dlvandenberg/tree-sitter-angular v0.9.2, the only
maintained licensed one, leaves an ERROR or MISSING node in 13 of the 72 templates the Angular compiler reads on the two
pinned subjects (18.1%; a unary minus in a conditional branch, a non-null assertion, a structural directive's `let`
microsyntax, a `@for` header's trailing semicolon; `docs/bench/readings/lang/angular-grammar.md`), over §14.2's 5%. No
Angular dialect is built, so no Angular rule is ruled here: a template stays the html dialect's, and LB-Q-angular-rest
is refused with the measurement and its trigger. *Astro* is ruled as follows. A `.astro` file (the compiler's own
input; no other dialect claims the extension) is one component: the astro dialect keys it by its base name with the
extension (package strategy `file`, F1), so `Card.astro` never shares a key with a sibling `Card.ts`, and it hosts its
injected ranges (F2): the frontmatter and each `<script>` whose `type` is absent or names JavaScript (`module`,
`text/javascript`, `application/javascript`) as TypeScript, each `<style>` as CSS. The reference parser is
`@astrojs/compiler` 4.0.0 `parse` (W4, tier A for what it attests: the frontmatter's text and lines, every element and
the type it gives it, `component`, `fragment`, `custom-element`, `element` or `expression`). Astro compiles the frontmatter
as the component's module script, which is TypeScript, so what is declared and called in it, and in a script, is
attested by TypeScript's own parser (`typescript` 5.9.3 `createSourceFile`, tier A for what it attests) over the range the
compiler delimits, at host lines. *L1* is those ranges' definitions, on the TypeScript dialect's kinds (a hosted range
keeps its own kinds): a class, a function (a top-level `const` bound to an arrow or function expression included), a
method, an interface, a type alias, an enum, and each name a top-level `const` binds (constant); a field and a prop stay
outside L1 as in typescript.json, counted; the component itself is the file's module symbol, outside L1 as every module
symbol is (K18). The reading scores the dialect's own extractor under the astro manifest's kinds plus these
(`calibration/oracles/astro-compiler`). *L2*'s pair is (file, caller, callee), a set (§1.4), the callee by the bare
rule. A component tag is a call of the component's module symbol `<module>`, wherever the template writes it (an
element inside an expression included), because the component renders it, and M2's principle holds: the compiler's own
`component` node decides what is a component, a tag whose name starts A to Z or holds a `.`, `Fragment` excepted (the
compiler's `fragment`); a custom element (a `-`), an HTML element and a fragment name nothing, and a member tag calls
its last segment (`<UI.Card>` calls Card, M2). A tag inside a `<script>`, a `<style>` or another raw-text element is text
to both parsers. A range's calls (a call or `new` expression, a tagged template, `super(...)`, `import(...)`) are owned
as the TypeScript dialect owns them: by the innermost enclosing function or class declaration, method or top-level
`const` (a destructuring `const`'s initializer is the module's), a decorator's by what it decorates; a call no such
definition encloses is the module symbol's (K18, F2: a hosted range's top-level calls are the host's). Out of both
sides, counted: a template expression's JavaScript (`{formatDate(d)}`), which is not injected, since the elements an
interpolation holds cut its text into no one parsable range (astro_template_expressions); and a `<style>`'s CSS pairs,
CSS's L2 under W2, left out of fathom's side (astro_injected_css_pairs). *L3* is every import declaration TypeScript's
parser reads in a range (`import`, `import type`, a side-effect import, `export ... from`) whose specifier is relative
(`./`, `../`), resolved against the importer's directory as Vite resolves a relative path: the file named, else the
spec plus `.ts`, `.tsx`, `.js`, `.jsx`, `.mjs`, `.mts` or `.json`, else its `index` file (Vite adds no `.astro`, so an
`.astro` target is written with its extension). It is scored as import_resolution and, with no SCIP index, as the
file-edge truth (LB-F57). A target that is no measured `.astro` file (a `.ts` module, a stylesheet, an image, a content
entry) is out of scope, counted (import_oracle_out_of_scope); a bare specifier (a package, `astro:content`) names no
file of the subject; and a tsconfig `paths` alias (`~/`, `@/`) is unjudged, the oracle reading no configuration: its
imports and the file pairs only they make leave both sides, counted (astro_alias_imports). Each of the three levels is
graded under §1.2's thresholds in these units, and the reading says so.

**Jenkins pipelines' L3 (K19, wave lf8, item cifmt, LB-Q-jenkinsfile; committed with the L3 subjects of
`calibration/corpus.d/jenkinsfile.yaml` before any reading under it).** The unit is (file, file), a set per subject: the
pipeline file a step is written in and the file of the repository the step reaches. Two steps reach one. (a) `load` with
a literal path (`load 'p'`, `load("p")` with no interpolation, `load(path: 'p')`) evaluates that Groovy file into the
pipeline, read relative to the current directory, which is the workspace a checkout gives (the repository root) under
each enclosing `dir('x')`: it reaches the file the path names. (b) A step call whose name is a `vars/<name>.groovy` of the
repository that declares `call` reaches that file (LB-F375's join); a name two `vars/` directories declare reaches
neither, counted. `@Library` is in-repo only: a library is reached through (b) alone, and only when the repository holds
its `vars/` file; an `@Library` annotation or a `library` step names a library a controller configures and draws no edge
of its own, and a step the pipeline calls from a library the repository does not hold reaches nothing on either side
(neither the compiler's AST nor the dialect can tell it from a plugin's step). A shell step's first command (LB-F374) is
a second family nothing evaluates, and a `build job:` names no file (LB-F376): both are out of the unit, counted. The
origin is the innermost definition holding the step (a stage's `steps`, the `script` block within it, a function); a
step no definition holds is out of both sides, counted, as is a `load` in a `vars/` file, which reads the calling job's
workspace, never the library's repository. Scope: a target is a file of the subject the jenkinsfile dialect reads (a
`Jenkinsfile`, a `*.jenkinsfile`, a `vars/*.groovy`), whether or not `-only` measures it, since only those carry the
definitions an edge reaches; a loaded file of any other name (a `.groovy` elsewhere, unclaimed while LB-Q-groovy-rest
stands) is out of scope, counted (`jenkins_load_unread`), as are a target the subject does not hold
(`jenkins_load_missing`) and a computed path (a GString with `${...}`, a variable, a path beneath a `dir()` whose path
is computed: `jenkins_load_computed`). The referring side is the claim's: the declarative pipelines `-only` measures; a
scripted pipeline's or a `vars/` file's edges are measured beside the reading, tolerated, never claimed. The oracle is
the Groovy compiler's AST at CONVERSION (`calibration/oracles/jenkinsfile`), whose walk writes these
`cross_references`, tier C: the walk, not the compiler, decides what a step reaches. The declarative linter (W4, tier A)
attests a pipeline's syntax, not the files its steps load, so no tier-A oracle covers the family, and tier C caps L3 at
good. L3 is graded under §1.2's tool-format thresholds in this unit (good: precision >= .90, recall >= .60), and the
reading says so. Criterion, pre-registered: jenkinsfile.json reads `level_met` L3.

## 2. Dialects as data

### 2.1 Layout

```
internal/extractor/dialects/<id>/
  dialect.json          the manifest (§2.2), validated by schema/dialect/dialect.cue
  tags.scm              definitions and references (tags engine)
  imports.scm           optional: import captures for L3
  injections.scm        optional: embedded-language ranges (Vue, Svelte, Astro, HTML, Markdown fences)
  doc.json              doc engine only: selectors (§4)
  PROVENANCE.md         where each query came from: repo, commit, licence, what changed
  testdata/
    <case>.<ext>        small fixtures, written by the worker, each ≤ 200 lines
    <case>.golden.json  the extractor's expected output for that fixture
```

All dialect directories are embedded (`//go:embed dialects`). The registry is built from the
manifests at init: `fusion.New` registers every manifest's extensions and basenames, the
connector's classification consults the same manifests, and the unsupported-language table
is generated from the queue (every queued language discloses until it is registered).
Adding a dialect therefore touches only its own directory, the tracker (generated), the
calibration shard and its reading — which keeps parallel workers from colliding.

### 2.2 The manifest

```json
{
  "id": "swift",
  "display": "Swift",
  "kind": "language",
  "engine": "tags",
  "lang_tag": "swift",
  "match": {
    "extensions": [".swift"],
    "basenames": [],
    "globs": [],
    "shebangs": [],
    "sniff": null
  },
  "grammar": { "blob": "swift", "entry_point": "tree_sitter_swift" },
  "variants": [],
  "queries": { "tags": "tags.scm", "imports": "imports.scm" },
  "kinds": {
    "class":     { "source_kind": "SwiftClass",     "canonical": "class",     "container": true },
    "struct":    { "source_kind": "SwiftStruct",    "canonical": "struct",    "container": true },
    "protocol":  { "source_kind": "SwiftProtocol",  "canonical": "interface", "container": true },
    "extension": { "source_kind": "SwiftExtension", "canonical": "class",     "container": true, "qualify_as_target": true },
    "enum":      { "source_kind": "SwiftEnum",      "canonical": "enum",      "container": true },
    "function":  { "source_kind": "SwiftFunc",      "canonical": "function",  "callable": true },
    "method":    { "source_kind": "SwiftMethod",    "canonical": "method",    "callable": true },
    "property":  { "source_kind": "SwiftProperty",  "canonical": "property" }
  },
  "required_kinds": ["class", "struct", "protocol", "enum", "function", "method"],
  "oracle_kinds": {
    "ctags": { "class": "class", "struct": "struct", "protocol": "interface", "enum": "enum",
               "func": "function", "method": "method", "property": "property" },
    "scip":  { "Type": "class", "Method": "method", "Term": "property" }
  },
  "qualify": { "chain_containers": true, "container_sep": ".", "method_sep": "." },
  "package": { "strategy": "dir" },
  "imports": { "strategy": "swift-module" },
  "signature": { "max_chars": 200 },
  "target_level": "L3",
  "queue_id": "swift"
}
```

Rules the schema and `TestDialectManifests` enforce:

- `id` is unique, lowercase, `[a-z0-9-]+`, and equals the directory name and the queue id.
- Every `source_kind` is globally unique across dialects, except that one dialect may reuse
  an existing `core.Kind*` constant's string for a language that already has it (Python keeps
  `PythonClass`). Existing languages keep their existing strings when they migrate — a
  changed string needs an `ExtractorEpoch` bump and a CHANGELOG line.
- `canonical` is from the closed list in §2.3.
- Every extension and basename is claimed by exactly one dialect, except where `match.sniff`
  splits an ambiguous extension (§5).
- `target_level` matches the queue; the manifest never carries a claimed level.
- `qualify.scope_sep` (optional; C++'s `"::"`): a definition name or callee spelled with the
  language's own scope operator is rewritten to `container_sep` and its template argument lists
  are dropped, so an out-of-class `Widget::render` and an inline `Widget.render` are one key
  (LB-Q-cpp; `normalizeScopedName`). A dialect without it takes `@name` as captured.
- `names` (optional; `strip_prefix`, `trim_quotes`): rewrites every captured `@name`, a
  definition's and a callee's, before qualification. `strip_prefix` drops the first listed prefix
  the captured text starts with (`.Values.` leaves the values key an accessor reads; a quoted
  string that starts the same way is untouched), then `trim_quotes` drops one matching pair of
  surrounding quotes (a Go-template `define "chart.name"` is the symbol `chart.name`), then
  `scope_sep` applies. A dialect without it takes `@name` as captured (LB-Q-helm).
- `module_symbol` (optional; K18, ADR-048 amendment 2026-10-10, LB-F346): the key of a kind the
  tags engine mints once per file whose top level executes code, the file's module symbol (§3,
  item 8). The kind is the manifest's own, canonical `module`, neither `container` nor
  `callable`, and outside `oracle_kinds` (no L1 oracle tags a file as a definition); the engine
  is `tags` or `tags+doc`; and `package.strategy` is `dir`, so the file's own name keys each
  module symbol apart. The schema (`#Dialect`, negatives 16c..16h) and `checkModuleSymbol` at
  load hold the same rules. Python turns it on (lf2); a pure namespace (a Go or Java package, a
  C# namespace, a Rust `mod`) names none: it is file granularity.

### 2.3 Canonical kinds (closed)

`module namespace package class struct interface trait enum enum_member union type_alias
function method constructor field property constant variable macro import test`
— for languages — and `target job stage task step resource data_source output variable_decl
rule policy service rpc message table view column selector mixin element section label
route key` — for tool formats. `extractor.CanonicalKind(SourceKind)` answers from the
registry; no store change.

---

## 3. The tags-query engine (part C)

`TSExtractor` already is the generic engine; these are its upgrades, each a foundation row.

1. **Registry from manifests** (LB-F1). `tsRegistry` is built from `dialect.json`; the five
   existing languages move into `dialects/{javascript,typescript,python,ruby,kotlin}/`
   with byte-identical extraction (golden parity over their fixtures and the calibration
   subjects, `TestTagsDialectParity`).
2. **Predicates** (LB-F2). The engine reads each pattern's predicate steps at compile
   (`ts_query_predicates_for_pattern`, exported from the blob) and evaluates `#eq?`,
   `#not-eq?`, `#match?`, `#not-match?`, `#any-of?`, `#not-any-of?` host-side over the
   captured text, dropping matches that fail. `#set!`, `#strip!`, `#select-adjacent!` and
   `#offset!` are accepted and ignored (they associate documentation, which fathom does not
   extract). Any other predicate fails compilation, and `TestDialectQueriesCompile` names it.
   This is what lets upstream and aider `tags.scm` files be used nearly verbatim.
3. **Ranges and signatures** (LB-F3). `EndLine` from the `@definition.*` node's end row;
   `Signature` from the definition node's start to the start of its `@body` capture (a
   fathom extension to the convention; a query without `@body` gets the definition's first
   line), whitespace collapsed, truncated to `signature.max_chars` with a trailing `…`.
   A grammar that puts a callable's body in a *sibling* of its signature (tree-sitter-dart's
   `function_signature` + `function_body`) has no node spanning both; the match captures the
   sibling as `@end` (usually also as `@body`) and the definition then reaches its end, so its
   `EndLine`, the calls its body makes and the definitions nested in it follow (LB-Q-dart).
   `@end` never shortens a definition.
4. **Name-captured references** (LB-F4). The standard convention puts `@name` inside
   `@reference.call`; the engine takes the callee from `@name` when present and falls back
   to the whole expression's text (today's behaviour, kept for the five existing queries).
   `@reference.class`, `@reference.implementation` and `@reference.type` are parsed and
   counted in the reading (`type_refs`) but not stored until a consumer exists.
5. **Package capture** (LB-F5). `@scope.package` (e.g. Java's `package` clause, C#
   namespaces, Kotlin and PHP packages) sets `SymbolNode.Package` when the manifest says
   `"package": {"strategy": "capture"}`; `"file"` sets it to the file's base name with its
   extension, for a dialect whose files share a stem in one directory (LB-F519, wave lf7:
   `X.psd1` beside `X.psm1`, `Button.vue`); otherwise the directory rule (`pkgFromPath`,
   the stem) stands. An injected range takes its host's package whatever the strategy.
6. **Injections** (LB-F6). `injections.scm` captures `@injection.content` with a static
   `#set! injection.language "<dialect id>"` (the one `#set!` the engine reads). The engine
   re-runs the injected dialect over that byte range, offsetting lines, one level deep only,
   and qualifies nothing across the boundary. This covers `<script>`/`<style>` in HTML, Vue,
   Svelte and Astro, and fenced code in Markdown (off by default for Markdown: documentation
   code is not the repository's code).
7. **Imports** (existing SU-11 vocabulary, generalized in LB-F7): `@import.stmt`,
   `@import.module`, `@import.name`, `@import.alias`, `@import.wildcard`, plus the
   `@import.from*`/`@import.rel*` captures the Python and JS queries already use.
8. **Module symbol** (K18, LB-F347, wave lf2; `internal/extractor/tsextract_module.go`). A
   manifest naming `module_symbol` (§2.2) gets one more definition per file: `<module>`
   (`core.ModuleSymbolName`, Python's own name for a module's top-level code, which no
   identifier spells), of that kind, from line 1 to the file's last line, unsigned. It owns the
   calls no definition owns: one no definition's node encloses and no call attachment
   (`@calls.from`/`@calls.to`) gives to a definition, so a module-level statement's, an
   `if __name__ == "__main__":` block's and a module-level loop's calls are its; a call in a
   function, a decorator's, an assignment's and one in a class body outside any member stay
   where they were. The pass runs after `attachTSCalls`, for the file itself and never for an
   injected range, and a dialect without the switch skips it: its output is byte-identical
   (`TestModuleSymbolOnlyWhereTheManifestNamesIt`, conan on python's grammar included). The
   file resolver reads definitions by line, so the module symbol is what the file's top-level
   references and imports are made from, and an import naming a module (`import a.b`, the
   module a `from` import reads, a submodule it names) is a file-resolved edge to that
   module's symbol (`fusion.moduleImportEdges`); it is no top-level name of its file, so a
   bare use of a whole-module binding still reads the target's sole definition. `internal/scip`
   maps a scip-python module definition (`<path>/__init__:`, `scip.ModuleDefinition`) to the
   document's module symbol and nothing else, never by position or by its `__init__` leaf
   (O527), and the harness names it the same way, so def_mapping maps it and a reference to
   a module is a typed edge to it.

Queries stay predicate-light and structural. The engine's match-scan budget
(`fathom_query_cursor_exec_bounded`, 10 s, the host backstop at 2×) applies unchanged.

**A second dialect on one blob, and a path claim (wave c3, item jenkins).** The jenkinsfile dialect
reads Jenkins pipelines on the groovy blob the gradle dialect pinned (LB-F93): one grammar, two
manifests, each with its own `tags.scm`, so a tool format written in a language fathom refuses
(Groovy sources, LB-Q-groovy-rest) still reads where the grammar parses it. Its kill line, more than
5% of the corpus files in error, fired (47.1%, 0 of 23 declarative pipelines), so the reading is
scoped with `-only` to declarative pipelines and the rest is tolerated, read and measured beside the
claim. A shared library's `vars/<name>.groovy` is claimed by the manifest glob `**/vars/*.groovy`,
never by the extension: `registerTagsGlobs` sends a tags glob's extension to the classifier, which
claims a file only when the glob matches. Two readings of this grammar are silent, no ERROR node
marks them, so `files_with_error` does not count them: a command call after a comment reads as its
parts side by side, and a top-level `pipeline { }` must end the file; the queries match those shapes
(LB-F373, O728). Cross-file work stays in fusion: a doc-xref hook (`jenkinsrun.go`) reads shell steps
with a small Groovy lexer and resolves them through the GitHub Actions token rule (`ghaRunRefs`), so
one tier-C rule serves both CI formats (LB-F374), and a step call joins its `vars/` variable
(LB-F375).

**React (wave c3, LB-F377..LB-F380, LB-Q-react).** React's constructs are kinds of the typescript and javascript
dialects, not a dialect of their own: one function node (a declaration, or a module-scope arrow or function
binding) is split by predicates over its name and body text into a function, a component (a PascalCase name whose
body writes `</` or `/>`, which every JSX expression has) or a hook (a `use[A-Z0-9]` name whose body calls a
hook); a PascalCase binding to memo, forwardRef, lazy, a styled template or a PascalCase member is a component; a
class whose heritage extends Component or PureComponent a class component; a member of a `*Props` interface or
type alias a prop, named after it (`@member`, the type alias now a container); and a PascalCase or member JSX tag
a call. The text predicates stand in for JSX nodes because the typescript and tsx entry points share one query
and the typescript grammar has none (a JSX call is a wildcard node with a `<` token and a name field). The kinds
keep canonical function, class, function and field outside `oracle_kinds`, so the TS/JS readings grade every
existing definition as before (typescript.json re-measured: L1 to L3 unchanged). `indexbench lang-react` grades
the React kinds alone (`docs/bench/readings/lang/react.json`, L3 good, capped by react-docgen-typescript's
tier C), and React Router's routes reach the route seam through `internal/analysis/routes/reactrouter.go`
(`docs/bench/readings/routes/react-router.json`). The patterns cost extraction CPU where module-level functions
are many (zod +39% per file, nest unchanged; O732); `#requires-text!` guards made it worse, not better.

## 4. The document-path engine (part D)

Tool formats whose syntax is YAML, JSON, TOML or XML are not a parsing problem; they are a
schema problem. The existing bespoke extractors (GitHub Actions, compose, Kubernetes,
Kustomize, Helm values, OpenAPI) already decode and walk; `DocExtractor` generalizes the
walk into data (LB-F8, P20: generalized from six).

`doc.json`:

```json
{
  "decode": "yaml",
  "multi_document": true,
  "detect": { "basenames": [".gitlab-ci.yml"], "globs": [".gitlab/ci/**/*.yml"],
              "require_keys": [], "content_sniff": null },
  "entities": [
    { "select": "$.*", "where": { "is_map": true, "not_key_in": ["stages","variables","default","include","workflow"], "key_not_prefix": "." },
      "kind": "job", "source_kind": "GitLabCIJob", "name": "$key" },
    { "select": "$.*", "where": { "is_map": true, "key_prefix": "." },
      "kind": "job", "source_kind": "GitLabCITemplate", "name": "$key" }
  ],
  "references": [
    { "from": "job", "select": "$.needs[*]", "value": "$ | .job", "to_kind": "job", "edge": "needs" },
    { "from": "job", "select": "$.extends", "value": "$", "to_kind": "job", "edge": "extends" }
  ],
  "cross_file": [
    { "select": "$.include[*]", "value": "$.local | $", "resolve": "repo-path", "edge": "includes" }
  ]
}
```

Selectors are a deliberately small JSONPath subset (`$`, `.key`, `.*`, `[*]`, `[N]`, `..key`,
`$key`, `$path`, `$file`, alternation `|`), implemented in Go over the decoded tree — no expression
language, no templating. `[N]` is a sequence's item N (from 0; nothing for anything else) and `..key`
is recursive descent: the value of every entry named `key` at any depth under the node, through maps
and sequences, in document order (LB-Q-cloudformation: a `Ref` wherever a resource's properties nest
it; `..Fn::GetAtt[0]` is the resource a GetAtt names). A reference may carry `declared_only` (record a
value only when it names an entity of its `to_kind` declared in the same document: a CloudFormation
Ref to a pseudo parameter, `AWS::Region`, is not one of the template's entities). A YAML spec may name
`yaml_tags: "cloudformation"`, the one tag scheme: a short-form intrinsic reads as its long form
(`!Ref X` is `{Ref: X}`, `!GetAtt A.B` is `{Fn::GetAtt: [A, B]}`, as is a long form written `A.B`,
any other `!Name v` is `{Fn::Name: v}`, cfn-lint's rule); without it a local tag is dropped, as it
always was (GitLab's `!reference` reads as a plain list). `detect.content_sniff` is a regular
expression over the whole file for a dialect no basename, glob or extension claims: fusion's
yaml-family classifier and its `.json` route send a file to the dialect when it matches, and the
harness measures exactly those files. `$path` is the dotted keys from the selection's start down to the node (one entity per
depth names a nested map's keys as a template reads them, `image.repository`); `$file` is the base
name of the file being read, extension included, the name of a document that carries none (an
MSBuild project is named by its file, LB-Q-msbuild; the extension keeps `Directory.Build.props` and
`Directory.Build.targets` two symbols); a `repo-path` reference reads `\` as the separator it is in
a Windows-authored file (`..\lib\lib.csproj`, MSBuild's ProjectReference); an entity or a
cross-file reference may carry `files` (the basenames it reads, so one spec serves a format's
several documents: Helm's Chart.yaml and values.yaml), and an entity `include_null` (keep a key
written with no value, which selectors otherwise drop). A `tags+doc` dialect (Helm) splits its
files across the two engines by this spec's `detect`: a basename or glob it names is a document,
every other file the manifest owns goes to the tags engine (`extractor.TagsDocExtractor`). Formats whose meaning needs evaluation (Helm templates, Jsonnet, Starlark,
Nix, CUE) are not doc-engine formats: they are syntax dialects on the tags engine, and
where a tool can evaluate them it is the tier-A oracle. A doc-engine tool format reaches
L3 through `cross_file` selectors resolved by strategy (`repo-path`, `repo-root`,
`image-ref`, `module-source`, `service-name`, `label-selector`, `route-host`).

**Value predicates (K20, wave c2, LB-F337, 2026-10-09).** A `where` filter tests values, not only
keys and types, and a value expression splits a string; each is opt-in, universal (K15: the engine
names no dialect) and absent from every spec that does not write it, so those extract byte for byte
as before. `not_has_key` is the key-absent selector: none of the listed keys is an entry of the node
(a node that is not a map has none), so a GitHub Actions step with `run` and no `id` is the positional
entity and an id'd one is its id's alone (O631). `matches` maps a selector (relative to the node, `$`
the node itself) to an RE2 that some scalar the selector reaches must match whole: the engine anchors
it `^(?:...)$` and compiles it once when the spec loads, and refuses one that does not compile there,
never at extraction; a GitHub Actions pin's SHA versus tag is two such predicates over `uses` (LB-F338),
and a Compose list element is claimed by `NAME=.*`. The `kv:<KEY>` transform, applied left to right
with `@upper` and `@file` and before `cut:`, reads a string as comma-separated `key=value` pairs and
gives KEY's value, nothing when no pair names it: `service.name=x,deployment.environment=y` gives x for
`service.name`, a list element `NAME=value` gives value for `NAME`, and a comma separates two pairs only
when an `=` follows it, so a value may hold one. An entity's `refuse_templates` drops a name decided at
deploy time: one carrying `${`, `{{` or `$(`, or read from a scalar written with a YAML local tag
(`!Ref`, `!Sub`; the decoder keeps a dropped tag on the node for this test). It is on by data for every
telemetry identity a deploy-time tool interpolates (compose, serverless, cloudformation,
otel-collector; Prometheus does not interpolate `job_name`, so its identity keeps the literal), TL-5's
never-guessed rule held at extraction rather than only at the join. An entity's `unique` keeps one
entity per name and source kind in a file, its first occurrence (a pin named by the action it pins; a
compose identity written twice). A pattern reference still never walks into another entity's node, so
an identity entity refused at its name gives its function back the references the name carries.

**Cross-file references reach the store (LB-F30, owner decision D4, 2026-09-28).** One
resolver, `fusion.ResolveDocXrefs`, serves ingest and the harness: `repo-path`, `repo-root`
and a relative `module-source` name a file, the one doc file directly in a named directory,
or every such file in the directories a glob names (`packages/*`), and resolve to that
file's first root entity (an entity selected at `$`); a non-relative `module-source` or a
`service-name` resolves to the one entity of that name in another file; `label-selector`
joins every entity whose declared `labels` (a top-level `labels` list: `from`, `select`,
`scope`, `scope_default`) include the selector map, in the same scope. `repo-root` resolves
`./x` from the repository root (GitHub Actions' `uses:` convention), every other strategy
from the referring file's directory. A `cross_file` entry may carry a `where` filter on its
origin (the Service-only selector), and an entity a `qualify` list (`[".kind",
".metadata.namespace"]` names a Kubernetes object `Deployment/prod/web`, the incumbent's
naming, so a Service and a Deployment sharing a name keep distinct keys). **Edge shape:** a
file-tier typed edge (`core.EdgeFileResolved`, tier pre-scan, never presented as typed) from
the referring entity's symbol to the referenced entity's, stored under the typed_edges key
`doc-xref` and replaced wholesale by an ingest pass gated like the file resolver's — the
same table and label `ResolveFileEdges` writes. A reference leaving the repository (a
registry package, a remote action, an image) resolves to nothing here: crossing
repositories is the `service_ref`/`module_ref` layer's job (§1.1). A reference resolving to
its own origin's key is dropped. The harness scores exactly the resolver's output
(`xref_precision`/`xref_recall`, §10), so the bench never scores an edge ingest drops.

**Route references (wave px, LB-F96 to LB-F100, 2026-10-01).** A route target is a
`DocCrossFileRef{From: <route, upstream or cluster entity>, Edge: "route_ref", Strategy:
"route-host", Raw: <target as written: URL, host[:port], unix socket>, Target: Raw}`; a
doc spec declares it as `cross_file {"resolve": "route-host", "edge": "route_ref"}`, a
native extractor sets the fields in Go. `fusion.ResolveDocXrefs` has no `route-host` case;
`fusion.ResolveRouteRefs` (`routeref.go`) serves ingest and the harness (`indexbench
lang-routes`). It strips a scheme, userinfo, port, path and Istio's `namespace/dnsName`
prefix and reads the host by the Kubernetes and Istio DNS rules into the ADR-050 `deploy:`
key (`syslink.ParseContractKey`): `name` is the Service in the referring Kubernetes object's
namespace (from its `Kind/<ns>/<name>` entity name, `default` for `Kind/<name>`), `name.ns`,
`name.ns.svc` and `name.ns.svc.cluster.local` the Service `ns/name` — `deploy:k8s/<ns>/<svc>`;
anything else `deploy:host/<hostname>`, which a compose service provides by its key (a
non-Kubernetes referrer's bare `name` may be either). Providers are a Kubernetes Service entity
(`Service/<ns>/<name>`, `Service/<name>`) and a compose service from whatever extractor ingest
dispatches. The nearest declarations of the key are joined — in the referring file, else its
directory, else the repository — each of them when they provide one key (an all-in-one
manifest beside per-service files declares one runtime object; ADR-050's link is a key match,
one per provider node), none when they provide two (`ambiguous`). Unresolved yields no edge
and is counted by reason: `external`, `ip-literal` (localhost too), `variable` (`$x`, `${X}`,
`$(X)`, `{{ }}`), `unix-socket`, `ambiguous`, `undeclared` (a cluster form no provider
declares); a join to the referring entity itself is dropped. The ingest pass (`enrichRouteRefEdges`,
gated like doc-xref) reads every path a doc or native dialect's manifest claims by extension,
basename or glob, so a dialect added later joins with no edit; while Kubernetes manifests
still dispatch to `k8sextract.go` (no DocResulter) it reads their `route-host` references
through the measured kubernetes dialect and joins them only to the incumbent's stored nodes
(the same `Kind/<ns>/<name>` naming). mongobetween, which has no config file, is read from the
launch arguments of the container or compose service that runs it: each host of a positional
`address=uri`'s seed list is one reference from that workload. **Interim divergence from
ADR-050 (Revisit when: SG-2 lands):** the `contract_keys`/`system_links` tables are frozen until
RT-M1, so an intra-repository route edge is stored the doc-xref way — a file-tier typed edge
(`core.EdgeFileResolved`, pre-scan, heuristic confidence) under the typed_edges key
`route-ref`, replaced wholesale — and `RouteRef.Key` carries the `deploy:` key it matched, so
SG-21 and SG-10 can later write the identical pair into `system_links`. By ADR-050's ladder
these are `literal` links and never gate (I57): they feed impact (`impact.BlastRadiusWithTypedEdges`,
the call graph; `blast_radius` with `include_heuristic` on a service that has a typed pass).
The typed Go pass's reconcile and the one-file refresh name the `doc-xref` and `route-ref`
keys (O335: before, a Go-only change purged both passes' edges until a doc file changed). The
harness scores the oracle's `route_references` into the xref family with the
`cross_references` (`calibration/oracles/README.md`); the kubernetes oracle's DNS rules are
tier C, so a route pair is proven at most good until a runtime resolution oracle (compose
DNS, a kind cluster) reads them.

**Policy references (wave pc, LB-F134 to LB-F137, 2026-10-01).** A policy names what it
governs; every policy producer emits the target as `DocCrossFileRef{From: <policy, rule,
exception or constraint entity>, Edge: "policy_ref", Strategy: <resource-kind | label-selector
| resource-type | aws-arn | service-name>, Raw: <as written>, Target: <normalized, or Raw>,
Scope, Labels}` — a native or Go-side producer sets the fields in Go, as the Kyverno and
Gatekeeper dialects do (`fusion.policyDocExtractor`: the doc spec cannot normalize a kind or
map a plural). `resource-kind`: Target `Kind` (any group) or `group/Kind` (Kyverno's
`apps/v1/Deployment` is `apps/Deployment`, `Pod/exec` is `Pod`; Gatekeeper's `apiGroups ×
kinds`; a CEL policy's plural resource mapped through the built-in API, a custom resource's
plural Target `""`), `"*"` kept; Scope a comma list of namespace patterns (`""` any), Labels a
selector narrowing the kinds of its own block. `label-selector`: a selector with no kind, Raw
and Target the canonical `k=v,k2=v2`, Labels the map. `resource-type`: Target the declared type
(`AWS::S3::Bucket`, `aws_s3_bucket`), `""` for an alias the producer cannot map. `aws-arn`: Raw
the ARN. `service-name`: Target the entity name. `fusion.ResolvePolicyRefs` (`policyref.go`)
serves ingest and the harness (`indexbench lang-policies`); `ResolveDocXrefs` skips the edge.
Targets: a Kubernetes object is the node ingest stores for a manifest (k8sextract's
`Kind/<ns>/<name>`, its group from the stored apiVersion) with its own `metadata.labels` read
beside it (the kubernetes dialect's labels are a workload's pod-template labels, a Service
selector's join, so the policy pass reads the object's own); a declared type is a Terraform
resource's type or another dialect's canonical `resource` whose Signature is its type (a
CloudFormation Type; an ARM or Bicep dialect joins with no edit); a physical name is a literal
CloudFormation name property or Terraform name attribute, which the pass declares as a
`DocLabelSet` in scope `aws:physical-name`. The join keeps the nearest targets — the policy's
file, else its directory's subtree, else the repository — and refuses more than 256 (a policy
library's fixtures, a ClusterPolicy over every Pod of a monorepo). Unresolved yields no edge and
is counted by reason: `wildcard`, `variable` (`${..}`, `{{ }}`, `[parameters(..)]`), `external`
(a literal ARN whose physical name the repository does not declare), `undeclared`, `unmapped`,
`ambiguous` (a service-name declared twice), `fan-out`. **Interim divergence from ADR-050 and F61
(Revisit when: RT-M1 reads PASS and SG-2 lands):** F61 names `policy_ref` and forbids a new edge
table before the table freeze lifts, so a policy edge is stored the route-ref way — a file-tier
typed edge (`core.EdgeFileResolved`, pre-scan) under the typed_edges key `policy-ref`, replaced
wholesale by a pass gated like route-ref, named in the Go pass's reconcile and the one-file
refresh (O335). The edges feed impact, never a verdict. The harness scores an oracle's
`policy_references` (`{path, from, to_path, to, raw, strategy}`) into the xref family with its
`cross_references`, `policy_unresolved` by reason, and an engine's agreement as
`policy_crosscheck` (disclosed, never scored); a content-claimed policy dialect is owned by the
predicate ingest claims it with (`fusion.PolicyDialect`), not classify.go's `ContentDialect`.

**PromQL references (wave c1, LB-F316, LB-F317, 2026-10-09).** A Prometheus rule, a Grafana panel
target or a PrometheusRule spec carries its references inside a PromQL string. `internal/promql` is a
pure-Go selector scanner (standard library only): `Selectors(expr)` returns every vector selector an
expression writes, its metric name (the identifier, a quoted name alone in the braces, or an equality
`__name__` matcher) and its matchers (`=`, `!=`, `=~`, `!~`) with the written span, and never parses
a tree or evaluates. Strings with their escapes, comments, offset and `@`, subquery ranges, functions,
aggregations and their `by`/`without` lists, `on`/`ignoring`/`group_left`/`group_right` lists and
colon recording-rule names are not selectors; a lexical defect is an error, never a guess. Its answer
is checked against the Prometheus parser's own `ExtractSelectors` over the corpus by a test in the
nested, test-only module `calibration/oracles/prometheus` (ADR-020, ADR-025: no main-module
dependency). `promql.IdentityLabels()` is the universal default identifying set, embedded data (K15):
`job`, `service`, `app`, `destination_service`, `destination_workload`, `service_name` and
`namespace`. A doc spec opts in per reference with `"promql": {}`: an intra-file reference reads the
selectors' metric names (with `declared_only`, a `record:` name another rule's expr uses: L2); a
cross-file reference, `route-host` only, reads each selector's **equality** matcher on an identifying
label as one route value (`"promql": {"labels": [...]}` replaces the set), scoped by the selector's
own `namespace` equality matcher, else the reference's `scope` (a PrometheusRule's
`metadata.namespace`), and written in the DNS form `fusion.ResolveRouteRefs` reads (`checkout.shop`
for `job="checkout"` in namespace shop; a dotted value such as `destination_service="reviews.default.svc.cluster.local"`
as written), the matchers it came from kept in `Labels`. The namespace label scopes and is never a
value; an empty value (`job=""`, a series without the label) names nothing. A regex or negative
matcher on an identifying label, or a regex namespace that would scope one, and an expression that
does not scan are never guessed: each is a `DocResult.Unresolved` entry, by reason
(`regex-matcher`, `negative-matcher`, `unparsed`). The kubernetes PrometheusRule entry (item mesh)
and the prometheus dialect (LB-Q-prometheus) are its first readers; MetricsQL and vmalert are not
read (`LB-Q-prometheus-rest`).

**Observability documents (wave c3, LB-F361 to LB-F367, 2026-10-10; ADR-065).** An Alertmanager configuration
and a Grafana dashboard act on Prometheus rules, so their dialects (`alertmanager`, `grafana-dashboard`, T7) join
those rules, through the one PromQL scanner and never through a `deploy:` label join (K19: the Stage A kill stays
fired). The engine gains, each opt-in and universal: `$ipath` (a node named by its keys and sequence indexes,
`route.routes[0].routes[1]`, so a recursive route tree names every node); an entity's `signature_fields` (its
fields, `key=value`) and `signature_keys` (keys it writes, by name only: a receiver's `slack_configs`, never its
settings); `matchers` (an entity's label matchers, an Alertmanager matcher list or a legacy `match`/`match_re` map
with its operator, read by `promql.ParseMatchers`, the selector scanner's own brace grammar with Alertmanager's
unquoted value, a malformed one unresolved; checked against Alertmanager's own parser); `facts` (scalars an entity
writes, kept as data a join reads: a route's receiver and `continue`, a rule's and its group's labels); the
`entity-name` (every entity of `to_kind` and that name, in any file) and `repo-tree` (every file under a directory
whose root is of `to_kind`) cross-file strategies; a `promql` reference on `entity-name` reading metric names, a
templated one refused (`template-variable`); promql's `grafana` mode (a target's query read only for a
Prometheus-typed datasource, the target's, else the panel's, else every datasource variable and `${DS_*}` input of
the dashboard; Grafana's macros substituted first; a variable outside a string marking a name no metric has; Loki,
Tempo and expression queries counted `datasource-not-prometheus`, an undecidable one `datasource-unknown`); and
`sensitive` (an entity in a file whose values may be secrets, whose source no surface serves as a snippet). One
resolver, `fusion.ResolveObsXrefs`, serves the doc-xref pass's one hook and the harness (`indexbench lang-obs`), its
edges doc-xref file-tier edges, the edge kind a data label: a route's or inhibit rule's `alertname` equality matcher
reaches every alerting rule of that name; every alerting rule is routed through every configuration by
Alertmanager's own semantics (depth first, the first matching child unless `continue`, the receiver inherited,
anchored RE2) under its name and static labels, `routes_to` when they decide every matcher on the way and
`may_route_to` when one reads a label the rule leaves to runtime (never a guess presented as certain); a panel's
metric reaches the recording rule that records it (never emitter code: Stage B is gated), a `/d/<uid>` link the
dashboard, a datasource uid the provisioned datasource, a provider's `options.path` (relative, or a container path a
compose bind mount maps) every dashboard under it. A dashboard JSON is claimed by its content only
(`fusion.grafanaDashboardJSON`); provisioning and Alertmanager YAML through late-YAML rows asked before prometheus. A
document a Kubernetes object embeds (a ConfigMap's dashboard or provisioning value, a GrafanaDashboard's
`spec.json`, a Secret's `alertmanager.yaml`) is read through its dialect as a document of its own beside the object,
its entities qualified by the key, at the manifest's lines (`fusion/k8sembed.go`). No secret-bearing field is
selected: TestIngest_AlertmanagerSecretsNeverStored plants a sentinel in every field Alertmanager's config package
types a secret (by reflection) and Grafana's, and finds it in no store table and no `symbol_context` answer.
Readings: alertmanager L3 excellent (amtool v0.34.1, tier A), grafana-dashboard L3 good (a tier-C walk, every level
capped at good). Ceilings: `LB-Q-alertmanager-rest`, `LB-Q-grafana-dashboard-rest`.

**Identity joins and Lambda handlers (wave c3, item cw, LB-F368..LB-F371, 2026-10-10).** A
CloudWatch alarm and the metric filter that feeds it, and a metric filter and the Lambda function
whose log group it reads, are joined by an identity, not a name: a new cross-file resolve
strategy, `identity`, carries an identity signature in TL-5's `key=value` form as its value
(`aws.cloudwatch.metric=Shop/Orders/Errors`, `aws.lambda.function_name=orders-api`, built with
`fmt:`), and `fusion.ResolveIdentityXrefs` draws an edge to every entity signed with exactly that
identity — the referring file's own included, the nearest declarations only (the file, else the
deepest enclosing directory holding one, the repository last; more than 64 is no join), never the
referring entity — so the join crosses files and dialects with no dialect's names in the resolver
(a CloudFormation alarm on a Terraform filter's `TerraformMetric`). ResolveDocXrefs leaves the
strategy to it, as it leaves `aws-arn` to the policy pass. What a Lambda function runs is code,
not data (`fusion.ResolveLambdaHandlers`, lambdahandler.go): SAM's `Globals.Function` defaults,
each runtime's handler rule (python's module path, node's file and export or an esbuild entry
point, java's `Class::method`, ruby's file and method) and a Terraform function's archive_file
are no selector; the function's entity is the origin of a file-tier edge to the handler's symbol,
only the named file extracted (as gharun.go does), stored with the doc-xref edges by one line of
the pass. A handler file's change re-runs the pass (`isDocXrefInput`), and the one-file refresh
invalidates its state (`passKeysFor` reads the same predicate), so ReingestFile then an ingest
stores what a fresh ingest does. The harness scores both edge families with ResolveDocXrefs'
(`indexbench lang-oracle`'s xref family; an oracle entry with edge `handler` names a code file the
dialect does not read and stays in scope when the subject holds it), so the bench scores the
edges ingest stores. EMF and PutMetricData emitters (Stage B) and a telemetry fetch (ADR-058)
are not read.

## 5. Classification

`connector.classify` consults the manifests in this order (LB-F9): exact basename, path
glob, extension, shebang (first line, for extensionless scripts), then — only for an
extension more than one dialect claims — `match.sniff` (a regex over the first 1 KiB,
evaluated in manifest `sniff.priority` order). The ambiguous extensions this program
must settle, and their sniffs:

| Extension | Candidates | Sniff rule, in order |
|---|---|---|
| `.m` | Objective-C, MATLAB | `@interface\|@implementation\|#import` → objc; `^\s*function\b\|^\s*%` → matlab; else objc when a sibling `.h` exists, else unclassified |
| `.h` | C, C++, Objective-C | existing CT-3 router, extended with `@interface` → objc |
| `.v` | Verilog, Coq, V | `\bmodule\b.*;` → verilog; `\b(Theorem\|Lemma\|Proof)\b` → coq (not queued, unclassified); `\bfn\s+\w+\(` → unclassified |
| `.pl` | Perl, Prolog | `^#!.*perl\|\buse\s+(strict\|warnings)\b\|\bsub\s+\w+` → perl; `:-` → prolog |
| `.fs` | F#, GLSL fragment | `\b(let\|module\|open)\b` → fsharp; `#version\|gl_FragColor` → glsl |
| `.pp` | Puppet, Pascal | `\b(class\|define\|node)\s+[\w:]+\s*[({]` → puppet (not queued); `\bprogram\b\|\bunit\b` → pascal |
| `.inc`, `.cls`, `.t` | several | not claimed by extension; basename/glob only |

github-linguist (MIT) is where the drafts of these rules come from, not where they are
decided. `indexbench lang-linguist -linguist <checkout> -rev <40-hex>` imports its
extensions, filenames and interpreters for the queue's items, and its heuristic rules for
the extensions they claim, as data at a pinned revision
(`docs/plans/language-breadth.linguist.json`, LB-F48, LB-F49). A tooling item takes
filenames only, never a language's extension list, and a linguist language two items share
(Starlark: bazel and tiltfile; Ruby, Groovy, HCL) carries an explicit filenames allowlist.
Every heuristic pattern is compiled as `ClassifyDialect` compiles a sniff, `(?m)` before the
pattern under Go's RE2; one that does not compile is dropped and counted, never approximated.
Linguist's rules are the candidate rules for the sniff table above: a worker compares them
with the table, writes the sniff, and proves it on the fixtures. Nothing in the data
changes what the connector classifies.

The count of files classified by sniff, and of ambiguous files left unclassified, is
reported per ingest and in `list_services` (disclosed, never silent — ADR-021 §1).

Classification also applies two new gates before any engine sees a file (LB-F10): the
**minified/generated sniff** (average line length over the first 64 KiB > 500 characters,
or a `@generated`/`Code generated … DO NOT EDIT` marker in the first 2 KiB → fingerprinted,
counted as `skipped_generated`, not extracted) and the existing 2 MiB and binary gates.

**Custom-resource dispatch (iac wave, LB-F149; Ansible, LB-F150).** A doc dialect that claims no name, extension or
glob is reached by content, in one of two ways. Argo, Flux and Crossplane objects open as Kubernetes objects (a
top-level `apiVersion` and `kind`), so `fusion.crDispatch` (a table of dialect ids, each naming its API groups in
`detect.content_sniff`) preempts the Kubernetes verdict, and only that verdict: a file is claimed when EVERY
non-empty document of it passes ONE dialect's sniff, a Helm chart's template, a CRD and a basename claim keep their
verdicts, and a file with such documents beside others is *mixed*, stays Kubernetes whole and is counted
(`fusion.CRMixed`). The doc engine reads a file as a unit, so a per-document split would need the engine. Ansible is
`classifyYAML`'s last signal (`ansibleDetects`): by path (`roles/<r>/...`, a standalone role's directories by the
sibling index, `tasks/` and `handlers/` sequences, `group_vars/`, `host_vars/`, a YAML inventory) and by content (a
top-level sequence of plays), so every earlier verdict is unchanged. Both are the decision `ReingestFile` repeats
(reading the chart and role roots from the disk, as ingest reads them from Discover). The harness measures them
through `Pipeline.ContentDialect`, which must name each such ToolKind (O405).

**Late YAML dialects (wave c1, LB-F309; LB-Q-prometheus).** A YAML dialect may claim a file by its manifest's
basenames and globs OR by its doc spec's `detect.content_sniff`: `classifyYAML`'s last signal
(`fusion.lateYAMLDetects`) claims a file no earlier signal claimed for the first late dialect (prometheus, then
otel-collector) whose name or sniff holds and which decodes to a root, never a Kubernetes object or a workflow. The
harness measures such a dialect on its named files and on the files `Pipeline.ContentDialect` names it for
(`ownedSourceRefs`, O620). Route candidacy stays path-only until item mesh's routeref change (O622).

## 6. L3: import resolution strategies

`ResolveFileEdges` today resolves TypeScript/JavaScript and Python. L3 generalizes it by a
closed set of strategies (LB-F11), each a Go function over (importer path, module spec,
repo file set) → target files:

| Strategy | Languages | Rule |
|---|---|---|
| `relative-path` | JS/TS, CSS/SCSS/LESS `@import`, HTML `src`/`href`, Markdown links | existing `resolveTSModule` rule plus configured extensions and index files |
| `dotted-module` | Python, Elixir (`Mix` apps), Erlang, Haskell, OCaml, Julia | dotted name → path under discovered source roots (`src/`, `lib/`, the repo root) |
| `namespace-dir` | Java, Kotlin, Scala, C#, PHP (PSR-4 via `composer.json`), Dart (`package:` + `pubspec.yaml`), Groovy | package/namespace → directory; PHP and Dart read their manifest's autoload/name |
| `include-path` | C, C++, Objective-C, CUDA, GLSL | quoted includes relative to the file, then the roots of the file's own `compile_commands.json` entry (`-iquote`, `-I`, `-isystem`, `-idirafter`; then every entry's), then sibling `include/` dirs, then (a quoted include only, counted) an ancestor directory of the includer |
| `crate-mod` | Rust | `mod x;` → `x.rs` / `x/mod.rs`; `use crate::a::b` via the crate root; workspace members from `Cargo.toml` |
| `go-package` | Go (reference only; Go is native L4) | — |
| `swift-module` | Swift | `import M` → a SwiftPM target named M (`Package.swift` targets' `path` or `Sources/M`) |
| `file-symbol` | Ruby (`require_relative`), Lua (`require`), Bash (`source`), R (`source`), Perl (`use`/`require`), Nix, Jsonnet, Starlark (`load`) | path or dotted module → file, per-language suffix rules in the manifest |

A manifest names its strategy and its parameters — the closed set `extractor.ImportParams`
decodes: `source_roots`, `suffixes`, `index_files`, `separator`, `relative` (a bare spec is
relative to the importer first), `case: "snake"`, `labels: "bazel"` and `type_wildcard:
"members"` (a wildcard naming a type brings its members into scope: C#'s `using static`); an
omitted one takes the strategy's default, which for `dotted-module` is the pre-LB-F11 Python
rule and for `relative-path` the TS/JS rule: Node's resolution of a relative spec (`.` and
`..` and a trailing `/` name a directory, whose package.json `main` — for a TypeScript
importer `typings`, `types`, then `main` — precedes its index file) with TypeScript's
node16/bundler extension mapping (`./x.js` names `x.ts`/`x.tsx`/`x.d.ts`, `.mjs` names `.mts`,
`.cjs` names `.cts`; LB-F43, LB-F44). Under `namespace-dir` a spec naming a type the
universe's symbols declare (package, separator, name: a C# `using A = N.T;`, `using static
N.T;`, a nested `N.Outer.Inner`) resolves to the files declaring it, wherever they sit; a
`using static` member binds a bare call only when no type enclosing the caller declares that name
and no second `using static` type does, as C#'s simple-name lookup reads it (LB-F45;
`internal/fusion/resolvestrategy.go` and `fileresolve.go` document each strategy's reads).
Under `include-path`, `file-symbol` and `swift-module` a whole-module import brings every
top-level name of its target into scope, as does any `@import.wildcard` (C#'s unaliased
`using`, D7); a name two targets define is left unresolved, and a dotted call through a
wildcard-imported name resolves to that name's member when its file defines one.
Under `include-path` (wave cx item cinc, LB-F121, LB-F122) the compile databases read are the
conventional in-repo ones (the repo root, `build/`, `builddir/`, `_build/`, `cmake-build-*/`,
`out/build/*/`), from the disk when the connector lists none (a build directory is gitignored):
per translation unit its flags in the compiler's search order, relative to the entry's
directory, the unit's own roots before the union. A call that binds no definition in the
caller's file or in the closure of its includes binds, when a header of that closure declares
the name (a declaration scan of the header's text: a prototype is no symbol), the one source
file that defines it with external linkage: a `static` definition is no candidate, the files a
database compiles are the only candidates when it names any, and two candidates bind nothing.
A header the router sends to the other language (CT-3's sniff reads C++ keywords in comments)
is read for its includes and prototypes and binds no definition. Every fallback, ambiguity and
exclusion is counted in `FileResolveReport`.
A new strategy is a foundation-level change (an Opus row), never something a worker adds.
Edges keep the existing label: `file-resolved`, tier pre-scan, never presented as typed.

References and re-exports (F-D, wave lc: LB-F74 to LB-F78). A dialect's `references.scm`
(manifest `queries.references`, compiled and run apart from its tags query, so no symbol or
call changes) names what a file references outside a call's own text (`@ref.name`, or
`@ref.head` with `@ref.member`; `@ref.not` marks a node no reference reads, a member's name)
and what it re-exports (`@export.*`: `export * from`, `export { A as B } from`, Dart's
`export 'x.dart' show/hide`, a default export). The extractor keeps a reference only when an
import binds its head or, where unnamed names are in scope (a wildcard import, an
include/require, a package scope), it starts with an upper case letter, and the file's
`file_imports` row carries its imports, references and re-exports together (no new table).
The resolver draws a reference from the innermost definition whose lines enclose it, through
the bindings a call takes (a named import, a whole-module prefix, a wildcard name), never
within its own file; a name a module re-exports binds the file defining it (a Python
package's own imports are its re-exports); a whole-module binding used bare binds the
module's default export, then its definition of the bound name, then its sole one. Language
scope rules ride manifest parameters: a namespace-dir dialect whose package is a clause sees
its own package's types unqualified (`package_scope: "nested"`: every enclosing namespace's
too, C#), `constants: "lexical"` looks a Ruby constant up through the modules around it,
`module_roots: "buf"` roots a protobuf import at buf.yaml's `modules[].path` and buf.work.yaml's
`directories`, and `type_paths: "package-dir"` reads a dotted protobuf type as package then
type, the package naming the defining file's directory. A namespace-dir named import whose
path names no file binds the top-level name, or the class member, its path less the last
segment names (Kotlin's `import a.b.fn`, Java's `import static a.b.Foo.bar`). A bare-named
member a dialect emits inside a type (a C# property, a Java field) is never a top-level name,
and a type two files declare (a C# partial class) binds nothing: which file a reference means
has no answer at file granularity. A reference written outside every definition (an import
line, module-level code, a re-export line) has no definition to draw from: the measured
ceiling on JavaScript and TypeScript recall (O275).

## 7. L4: SCIP import per language

The mechanism exists (`fathom ingest --scip`, auto-detected `index.scip` and
`.fathom/scip/*.scip`, position-based mapping since O115). What L4 needs per language is a
measured run. Producers, as of 2026-09:

| Language(s) | Producer (licence) | State | Notes |
|---|---|---|---|
| Java, Kotlin, Scala | scip-java (Apache-2.0) v0.13.x | active | Kotlin via scip-java's own path; the standalone scip-kotlin plugin is archived — do not use |
| TypeScript, JavaScript | scip-typescript (Apache-2.0) 0.4.0 | active | measured (CV-M7) |
| Python | scip-python (MIT) 0.6.6 | active | measured (CV-M7) |
| Rust | rust-analyzer `scip` (MIT OR Apache-2.0) | very active | |
| C, C++, CUDA | scip-clang (Apache-2.0) | beta | needs `compile_commands.json` |
| C#, VB.NET | scip-dotnet (Apache-2.0) v0.2.14 | active | needs a buildable solution |
| Ruby | scip-ruby (Apache-2.0) v0.5.0 | active | Sorbet-based; good only on Sorbet-adopting code — disclose |
| PHP | scip-php (MIT) | small, active | |
| Dart | scip-dart (Apache-2.0), community continuation | active | |
| Go | native (`go/types`); scip-go (Apache-2.0) is the oracle | — | |

Running an indexer is an operator or bench action, never the daemon's: the worker runs it
in scratch, bounded (§13 step 9), and commits only the reading. A language without a
maintained producer caps at L3 (ADR-041's revisit trigger names the case where that
becomes a problem: such a language becoming a primary target).

## 8. Grammars: build, lock, licence, distribution (part F)

### 8.1 The lock

`internal/extractor/tsengine/grammars/grammars.lock.json` (LB-F12) replaces the tag
variables in `build.sh` and the hand-kept tables in `PROVENANCE.md`:

```json
{
  "core": { "repo": "tree-sitter/tree-sitter", "tag": "v0.26.11",
            "commit": "64402de2857cc197ecc4ca3bc144ea91fda7e72e", "licence": "MIT" },
  "toolchain": { "zig": "0.16.0", "wasm_opt": "129" },
  "grammars": [
    { "blob": "swift", "repo": "alex-pinkus/tree-sitter-swift", "tag": "0.7.1",
      "commit": "<40 hex>", "tarball_sha256": "<64 hex>", "licence": "MIT",
      "entry_points": ["tree_sitter_swift"], "subdirs": [], "scanner": true,
      "generate": false, "abi": 14,
      "blob_sha256": "<64 hex, of the uncompressed .wasm>", "blob_bytes": 0, "gz_bytes": 0 }
  ]
}
```

`build.sh <blob>` builds one grammar from the lock (fetch tarball, check `tarball_sha256`,
compile with the existing zig recipe plus the predicate export, `wasm-opt -Oz`, gzip -9),
and prints the three numbers to paste back. `subdirs` lists the grammar directories under the
repository root when there is more than one (tree-sitter-typescript: `typescript`, `tsx`);
empty means the root. `commit` is the commit the tag points at (`git get-tar-commit-id` of the
tarball), never an annotated tag object's id. The lock is decoded strictly: an unknown field
is an error. `"predicate_export": true` adds `ts_query_predicates_for_pattern` to a blob's
exports; the five original blobs keep `false`, because the engine reads predicates from the
query source (LB-F2) and a rebuild of them from the lock is then byte-identical (measured:
`build.sh typescript` reproduced the committed module's sha256). A grammar whose release ships only
`grammar.js` sets `"generate": true` and the recipe runs a pinned `tree-sitter-cli` via
`npx --yes tree-sitter-cli@<pinned> generate` in scratch (never a global install).
`PROVENANCE.md` becomes a generated rendering of the lock plus the prose that is there today.

### 8.2 The licence allowlist

Allowed for grammars, queries and fixtures: MIT, Apache-2.0, BSD-2-Clause, BSD-3-Clause,
ISC, Unlicense, CC0-1.0, Zlib, 0BSD; MPL-2.0 only for files taken unmodified. Refused: GPL
(any), LGPL, AGPL, SSPL, EUPL, PolyForm (any), BUSL, FSL, Commons Clause, "no licence" and
"licence unclear". The check reads the grammar repository's top-level `LICENSE`, `LICENCE`
or `COPYING` at the pinned commit **and** any licence file under `queries/` and `src/`; a
mismatch refuses. A top-level licence file decides, identified by its text. Where there is
none, a licence **declared** by the package manifests counts when they agree: `package.json`
`license`, `Cargo.toml` `package.license` and `tree-sitter.json` `metadata.license`, every
one present naming the same allowlisted SPDX id, recorded with basis `declared`
(`kennyhml/tree-sitter-abap` has no LICENSE file and declares MIT in all three). Manifests
that disagree, or none, leave the grammar unlicensed and refused. **AGPL is refused by name**,
wherever it is read (the ADR-048 amendment of 2026-09-29, written with item tagsq): a queue
grammar under AGPL is not pinned whatever the catalogue offers instead
(`foxyseta/tree-sitter-prolog` is AGPL-3.0; the catalogue's `Rukiza/tree-sitter-prolog` is ISC,
and re-pinning to it is a queue decision). `indexbench lang-pin -tree <checkout|tarball>`
runs the check (`GrammarLicenceVerdict`, LB-F51) and exits 1 when refused. The verdicts
for the two queue grammars above are recorded in `docs/plans/language-breadth.grammars.json`.
`TestGrammarLock` fails when a blob's sha256 or size differs from the lock, when a blob
exists without a lock row or a lock row without a blob, and when a licence is not on the
allowlist. Every borrowed `.scm` file starts with a header:

```
; source: <repo> <path> @ <commit> (<SPDX>)
; changes: <one line: what fathom changed and why>
```

Fixtures are written by the worker or copied (≤ 200 lines) from a calibration subject whose
licence is on the allowlist, with the same header.

### 8.3 Distribution

- **Embedded, compressed, lazy** (LB-F13). Blobs are committed as `<blob>.wasm.gz`
  (deflate, stdlib `compress/gzip` — no new dependency), embedded, decompressed on first
  use of that grammar, and handed to wazero's compiler. Measured today: the five blobs are
  7,234,475 bytes raw and 1,483,414 bytes gzip -9 (4.9×). `TestEmbeddedGrammarBudget`
  fails when the embedded `.wasm.gz` total exceeds 40 MiB.
- **Compilation cache.** wazero's `NewCompilationCacheWithDir` under the user cache
  (`~/.fathom/cache/wazero/<wazero version>/`) keeps compiled machine code across restarts;
  `fathom gc` ages it out (ADR-047 decision 13), and it is off for unmanaged daemons.
- **Dialect packs** (LB-F14). A pack is a directory or `.tar.gz` holding one dialect
  directory plus its `.wasm.gz` and a `pack.json` (the lock row, the manifest's sha256, the
  queries' sha256). `fathom dialect install <path>` copies it to
  `~/.fathom/dialects/<id>/<blob_sha256>/`; the daemon loads it at start only when its
  `blob_sha256` is pinned in the binary's lock (`"distribution": "pack"` rows) or listed in
  the user-scope `[dialects] trust = ["<sha256>", …]`. A repository's `.fathom/config.toml`
  can neither install nor trust a pack (law 13: a clone cannot raise what a machine runs).
  `fathom dialect list` shows embedded, installed, trusted and refused packs; `list_services`
  names an unpinned pack's dialect as `tolerated (unpinned pack)`. No verb fetches over the
  network until a public release host exists (F2); until then an operator copies packs in.
- **When a dialect moves to a pack**: only when the embedded budget would be exceeded, the
  lowest-demand embedded dialects (by the queue's demand score) move first, recorded in the
  lock and the CHANGELOG.

## 9. Tenancy (the guard every dialect lives inside)

| Mechanism | Bound | Row |
|---|---|---|
| Lazy grammar load | a grammar is decompressed and compiled only when a file of its dialect is extracted | LB-F13 |
| Process-wide parser pool | ≤ 6 live parser modules (LRU across all dialects, `tsengine.AcquireParser`, keyed by extractor and variant so compiled queries stay with their owner); a module idle 2 minutes is closed; a leased module is never closed under its holder (past the cap the surplus closes at release); `tsengine.LiveModules()` stays the instrument, `PoolPeak()` the proof's | LB-F15 |
| Compiled-code accounting | each compiled grammar is a `grammar_code` memo group entry charged at its lock row's `blob_bytes` (wazero does not report compiled size; measured on arm64, wazero v1.12.0, the code is 0.46×–1.84× the module) against the memo registry's byte budget (I51); shed evicts least-recently-used compiled grammars, refuse drops all and closes idle modules; an evicted grammar's code is closed once no parser is being made from it | LB-F15 |
| Module memory | RT-107's `MemoryLimitPages = 1024` (64 MiB) per module, unchanged | existing |
| Per-file gates | 2 MiB cap, binary sniff, minified/generated sniff, known-fault fingerprint | existing + LB-F10 |
| Guest budgets | parse and query-scan deadline 10 s, host backstop 20 s | existing |
| Ingest | extraction stays inside the 64 MiB in-flight window and the one heavy slot (I50) | existing |
| The proof | `TestPolyglotIngestHoldsModulePool` ingests a fixture with every registered dialect and asserts live modules ≤ 6, modules closed after `Close`, and compiled bytes charged; LB-M3 reads the operator daemon's `phys_footprint` before and after a polyglot ingest | LB-F15, LB-M3 |

A dialect whose corpus run shows a per-module peak above 48 MiB (75% of the cap) or a
compiled size above 16 MiB is flagged in its reading and needs an Opus review before it
merges.

## 10. The measurement harness (part A)

`indexbench lang-oracle` (LB-F16) — in-process, no daemon, no store:

```
go run ./cmd/indexbench lang-oracle \
  -dialect swift \
  -subject workspace/calibration/swift/Alamofire \
  -ctags workspace/tools/ctags \
  [-scip workspace/tmp/scip/swift-alamofire.scip] \
  [-scip-defs]                                 # L1 from the SCIP index, tier A (D2)
  [-tool-oracle workspace/tmp/oracle/<id>-<subject>.json]...   # repeatable (D2, D5)
  [-tags-query workspace/tmp/oracle/<id>-tags.scm \    # third-party definitions query: L1, tier C (D5 extension)
   -tags-query-rev <40-hex> -tags-query-source "<owner/repo> <path>" -tags-query-licence <SPDX> \
   [-tags-query-format tags|locals|outline] [-tags-query-out <json>]]   # out: workspace/tmp/oracle/<id>-<subject>-tagsq.json
  [-only 'dir/**/*.sql,other.sql']               # restrict the subject (D2)
  [-adjudicate calibration/adjudications/<id>.json] \  # verified oracle misses (D3)
  -out docs/bench/readings/lang/swift.json   # merges subjects: one reading per dialect
```

It lists the subject's files through the connector's own discovery (so exclusions match) —
for C++ also the `.h` headers the header router's sniff sends to C++, since no dialect claims
`.h` (§5) — runs the dialect's extractor on each, runs the oracle(s) on the same file list, matches per
§1.4, and writes the reading. `indexbench lang-probe -dialect <id> <file>` prints the
S-expression of a small file (capped at 200 lines of output) and every capture the
dialect's queries make on it — the tool a worker writes queries against, instead of reading
`grammar.js` or `node-types.json`.

The reading (`internal/langbench.Reading`, JSON, indented, stable key order):

```json
{
  "dialect": "swift",
  "kind": "language",
  "target_level": "L3",
  "level_met": "L2",
  "grade": { "L1": "excellent", "L2": "good", "L3": "unmet", "L4": "not-attempted" },
  "measured_at": "2026-10-02",
  "fathom_commit": "<sha>",
  "grammar": { "blob": "swift", "commit": "<sha>", "blob_sha256": "<sha>" },
  "subjects": [ { "name": "Alamofire/Alamofire", "sha": "<sha>", "files": 98, "bytes": 1650000 } ],
  "oracles": [ { "tier": "B", "tool": "universal-ctags", "version": "6.2.0" } ],
  "metrics": { "sym_recall": 0.97, "sym_precision": 0.96, "kind_agreement": 0.98,
               "range_agreement": 0.93, "signature_coverage": 0.95,
               "call_recall": null, "call_precision": null },
  "tenancy": { "faults": 0, "faulting_files": [], "throughput_mb_s": 4.1,
               "max_file_ms": 212, "peak_module_pages": 310, "compiled_bytes": 9123456 },
  "excluded_by_kind": { "ctags:variable": 1204 },
  "notes": [ "L2 oracle tier C only: no maintained SCIP producer for Swift" ],
  "disposition": null
}
```

A tool-native oracle file may carry `cross_references` (`[{path, from, to_path, to}]`; `to`
omitted compares at file granularity): the harness scores them against
`fusion.ResolveDocXrefs` over the dialect's files as `xref_precision`/`xref_recall` at the
file's tier (LB-F30, §4). Bench-only oracles that write this shape live under
`calibration/oracles/` (tier C: github-actions, kubernetes). A tool format on the tags engine
(protobuf, then cmake and graphql) has no doc-engine references: its cross-file references are
the file resolver's edges over its import strategy (`fusion.ResolveFileEdges`, the pass ingest
stores file-resolved edges from), each read as (file, referring definition, referenced file), and
the oracle's are compared at file granularity, `to` dropped and `from` read by the dialect's name
matching (LB-F56, O167). The reading says so in a note.

**File edges without a SCIP index (LB-F57).** With no `-scip`, a neutral oracle file's `imports`
(below) are also the file-edge truth: each (path, to_path) pair with both files measured and
path ≠ to_path is a truth edge for `langbench.ScoreFileEdges` against `fusion.ResolveFileEdges`, in
the `file_edges` family at the file's tier (tier B caps L3 at good, §1.3). The truth is then what
each file's import statements name, not the files its references reach (a SCIP index's
cross-file edges): a file reached without an import of its own (one another file required, an
autoloaded constant) is outside it and a resolved edge to it counts against precision, while a
file imported and never referenced is in it. The reading's note discloses this. A SCIP index,
when given, stays the one file-edge truth.

**Go (LB-F58).** Go is native (`goextract.go`): no tags dialect, no file resolver
(`extractor.FileResolveLangs` skips its go-package strategy). What ingest stores for Go across
files is the typed go/packages pass's edges (`fusion.EnrichTypedEdges`); the harness runs that
pass in process into an in-memory store, scoped to the measured files, and scores its edges
against a SCIP index's file edges. `import_resolution` takes each import spec of the measured
files, at its own lines, resolved by Go's own loader (go/packages, under the typed pass's
environment) to the measured files of the package its path names, against the index's
references on the import lines; a standard-library or dependency path resolves to nothing.

**Files with a parse error (LB-F59, O203).** Each subject of a tags dialect records
`tenancy.files_with_error`: the measured files (the connector's set the extraction reads) whose
tree on the dialect's own pinned grammar holds an ERROR or MISSING node, found with the
extractor's own error query `[(ERROR) (MISSING)] @error` (lang-probe's S-expression renders named
nodes only, so a MISSING `;` never shows in it). The reading's `tenancy.files_with_error` is the
sum over the subjects that report one, beside `faults`; a doc or native engine parses no
tree-sitter grammar and reports none, and the field is optional, so a reading measured before it
is unchanged. §14.2's parse-error refusal (more than 5% of files) is read off it. Cross-checked
on a protobuf fixture against the tree-sitter 0.27 CLI built from the lock's tarball of
tree-sitter-proto 0.6.0 (sha256 and commit as locked): the same 2 of 4 files, one MISSING `;`
and one ERROR.

`level_met` is the highest level whose metrics all pass at excellent or good with an
admissible tier; `grade` says which. The file is written with
`json.MarshalIndent(reading, "", "  ")` so `"level_met": "L3"` is always one line — the plan
rows' lexical oracle matches that line. Readings are small (a few KB) and committed; raw
oracle output stays under `workspace/`.

`TestLanguagesTrackerMatchesRegistry` gains a third assertion (LB-F17): a tracker row's
level is never higher than its reading's `level_met`, and a registered dialect without a
reading is listed as `tolerated`.

**Producer-specific rules (LB-Q-rust, 2026-09-28).** A SCIP producer's own vocabulary is read
by the harness where it would otherwise score a definition or a call no source text spells:
rust-analyzer's `impl#[Type]name().` members (`]` closes a descriptor), its macro-expanded
definitions (a definition with no declaration keyword before it at its reported position is
`Generated`, left out of L1 and counted as `<oracle>:macro-generated`), its file-module definition
(a multi-line whole-file range, recovered at the `mod x;` reference), its macro references (a call
by the macro's name) and its operator references (`?`, `==`, `!x`, `*x`, `a[i]` resolve to trait
methods at the operator's range: no call is written, so a callable reference whose range is not the
callee's name leaves the L2 truth, counted as `scip_calls_unwritten`). `langbench.SCIPOptionsFor`
selects the call-site rules per dialect; every other dialect keeps the zero options.

**Import resolution (LB-F40, 2026-09-29).** `import_resolution` is computed for every language dialect
whose manifest names an import strategy (§6). Fathom's side is `fusion.ResolveImportSpecs`: each file's
import specs (`TSExtractor.ExtractImportSpans`: the specs `ExtractImports` returns, each with the line
span of the statement binding it) resolved by the strategy over the dialect's measured files to the
file(s) the file resolver binds (a named import: the file defining the name, else the
`Module<sep>Name` submodule, else the module's files; a whole-module import: the module's files; a
wildcard: the files declaring the namespace, else the module's). The unit is an *import*: the name one
statement binds, its alternative readings merged (rust's `use a::b::c` is read both as a named import
of `c` and as the whole module `a::b::c`: one import, resolved when either reading is). The oracle is
either the SCIP index (tier A) — every reference in a document to a non-local symbol another document
defines, a module or file symbol included (`mod x;`, a `use` path, a Dart import URI), paired with the
import statement whose lines hold it — or a neutral oracle file's `imports` (`[{path, line, to_path,
module}]`, `module` optional: a language-native import dump at the file's tier;
`calibration/oracles/ruby-require` and `calibration/oracles/clang-includes` are tier B). An import is
**in-repo** when the oracle resolves its statement to a measured file, and **agrees** when fathom's
strategy resolved it to one of those files: `import_resolution` = agreeing ÷ in-repo imports, the §1.2
bar (excellent ≥ .95, good ≥ .85). The reading's counts and its one `import_resolution:` note also give
what is not gated: in-repo imports left unresolved or resolved only elsewhere; (file, statement,
target) edge precision and recall; imports fathom resolved where the oracle names no file
(`imports_unattested`: an external import resolved into the repository, or one the oracle cannot see —
a namespace import under scip-dotnet, which defines no namespace anywhere); oracle imports on no
statement fathom extracted (a dump entry, or a SCIP occurrence the producer gave the Import role);
targets outside the measured files; and references left out because the index defines their symbol in
more than one document (scip-typescript names every nest sample's `src/app.module.ts` alike: which file
is meant would be the index's guess, never truth). One imports oracle per subject. The first runs, on
the SCIP indexes kept from round 2 (scratch, not committed): axios 232 of 232, nest 3,271 of 3,271,
express .34 (`require('../..')`, a directory spec, is unresolved), zod 0 of 504 (every spec is an ESM
`./x.js` naming `x.ts`), Polly 0 of 15 and Newtonsoft.Json 0 of 313 (an aliased `using` of a type
whose file is not named after it) — strategy gaps the metric exists to show (O145). The first tier-B dump runs (2026-09-29, ruby 2.6.10 and Apple clang 21.0.0, the pinned calibration subjects): Ruby 144 of 144 (sinatra, jekyll; `ruby-require` over the load path `lib`) and C++ 218 of 218 (fmt, nlohmann/json; `clang-includes`), each with a large unattested count that is the oracle's blind spot, not fathom's error (a `lib`-only load path; an include the compiler does not enter, O146). Neither language reaches L3: the grade also needs `file_edge_precision` and `file_edge_recall`, which until LB-F57 only a SCIP index gave (§14.1): C++ reads .5309 and .1114 against .90 and .25, and Ruby's scip-ruby index sees typed receivers only (.8 and .0124, disclosed, not graded; the binary is absent on the measuring machine). Since LB-F57 the dumps' own (file, imported file) pairs are the file-edge truth when no SCIP index is given (above); neither reading is re-measured with it yet.

## 11. Retiring heuristics and bespoke walkers

The nine CGO walkers and three regex extractors each have a queue item whose target is at
least their current output. The protocol (LB-F18, and each port item):

1. Write the dialect (tags or doc engine) alongside the incumbent; both registered, the
   incumbent still dispatched.
2. `indexbench lang-parity -dialect <id> -incumbent <extractor>` runs both on the calibration
   subjects and reports, per metric, new minus old against the same oracle. The port may
   merge when every L1 metric is ≥ the incumbent's and the target level is met.
   `-incumbent` names the retired file's stem (`dockerextract`, `sqlextract`, …; empty
   measures what fusion dispatches today); `-subject [name=]dir`, `-scip [name=]path` and
   `-tool-oracle [name=]path` repeat per subject; the dialect runs through its own engine
   (tags or doc) while the incumbent is still dispatched. The incumbent's side is scored with
   the dialect's oracle mapping plus its LB-M1 spec's kinds. Exit 0 = merge, 1 = refused
   (a gated L1 metric below the incumbent's, or measured by the incumbent and not the
   dialect, or the target unmet), 2 = usage; the kinds diff is printed for step 3, and
   `-out` writes the report as JSON.
3. Switch dispatch to the dialect, delete the incumbent and its CGO grammar import, bump
   `ExtractorEpoch`, record the kinds diff (identical `SourceKind` strings are required; a
   kind the incumbent emitted and the dialect does not is a regression unless the row's
   reason names why).
4. When the last CGO walker is gone: `go.mod` loses `github.com/tree-sitter/go-tree-sitter`
   and the per-language CGO modules (LB-F19, a `gone: module` row), `internal/AGENTS.md`'s
   CGO sentence is corrected, and ADR-013's status line says so. Whether `CGO_ENABLED=0`
   builds are then possible is measured, not assumed (F1). Done 2026-10-10 (wave h4, K25
   ruling 1): the last importer, `internal/extractor/treesitter/`, which nothing imported,
   left (LB-F395) and `go mod tidy` dropped go-tree-sitter, five grammar bindings and
   mattn/go-pointer (LB-F19). Measured: linux/amd64 and linux/arm64 build with
   `CGO_ENABLED=0` as static binaries; darwin keeps cgo only for cmd/fathom's two memory
   readers. ADR-066 supersedes ADR-013 and records the deltas; CS-9 guards the result.

## 12. The tracker

`docs/languages.md` becomes generated (LB-F17): `go run ./cmd/indexbench lang-tracker >
docs/languages.md` renders one row per dialect from the manifests, the readings and the
queue — dialect, kind, engine, grammar (repo@tag, licence), target, level met with grade,
oracle tier, typed-edge producer, reading date, next. The hand-written preamble and the
typed-edge prose above the table are kept as a template. `TestLanguagesTrackerIsGenerated`
fails when the committed file differs from the rendering. Workers never hand-edit the table.
The excellence board (§18) is part of the rendering, so `TestLanguagesTrackerIsGenerated`
(in `task test`, under `task validate`) already fails a stale board.

## 13. THE WORKER RECIPE — one queue item, start to finish

You are a worker executing exactly one entry of `docs/plans/language-breadth.queue.json`.
You have this section, your queue entry, and the repository. Do not read the rest of the
design unless a step sends you there. Do not read `grammar.js`, `parser.c`,
`node-types.json` or other generated grammar files — use `lang-probe`. Stay inside your
worktree and your scratch directory.

### 13.0 Before you start (5 minutes)

1. Read your entry: `id`, `name`, `kind`, `target_level`, `engine`, `grammar_source`,
   `licence`, `oracle`, `calibration_subjects`, `depends_on`, `port_of` (when present).
2. Check `depends_on`: every id must be `[x]` or `[-]` in `docs/plans/language-breadth.md`
   (`fathom plans status`) — except a dependency on another queue item (`LB-Q-<id>`: cpp on
   c, scss/less on css), which is satisfied once that item has a committed reading at L1 or
   better, even while its own row stays open for a higher target (D6, §1.5).
   `go run ./cmd/indexbench lang-next -all` prints exactly this. If blocked, stop and
   report `blocked: <ids>`.
   *Amended 2026-09-29 (G2-6, `docs/plans/sdd-reconciler-v0.md`): deferred is not done.
   A `depends_on` row is met only when it is `[x]`; a `[-]` row no longer is. A deferred
   one (or one with no disposition) keeps the item blocked until it revives; a refused
   one (any disposition but deferred) will not be built, so the item needs re-planning,
   not a wait: `lang-next -all` prints `needs re-planning: <ids>` and the console's What's
   next lists it apart. D6 still holds for a queue-item dependency whose own row is open
   or deferred (its committed reading at L1 or better satisfies it), never for a refused
   one. An item refused itself — its row, or its `-rest` sibling while the row stays open
   with the contract, as Swift's — is never offered either: `lang-next -all` prints
   `needs re-planning: <refusing row>` for it and What's next lists it apart, as the tracker's
   Next column says `re-plan`. `langbench.QueueEligibility`, `lang-next` and What's next share
   this rule with `fathom plans next`. If a dependency is refused, stop and report `replan: <ids>`.*
3. Work in a fresh worktree on branch `lang/<id>` from the integration branch, with
   `GOWORK=off`, your own scratch dir `<scratch>/lang-<id>/` (never shared), and
   `GOFLAGS=-p=2`. `<scratch>` is one directory outside any Go module, the same for every
   worker on the machine; the brief names it. Never start a daemon, never touch
   `127.0.0.1:7575` or `~/.fathom`. If a read fails with "Operation not permitted" on
   macOS, the terminal has lost its file-access grant: restore it and retry.
4. Take the measurement lock before any heavy step (clone, indexer, harness):
   `mkdir <scratch>/lang-measure.lock` (retry every 60 s; remove it when the step
   ends). One heavy measurement runs machine-wide at a time.

### 13.1 Pin the grammar (tags-engine items only; skip for doc-engine items)

5. Confirm the licence at the pinned commit: fetch the repository at the tag you will pin
   (a checkout or the GitHub tarball) and run `indexbench lang-pin -tree <checkout|tarball>`
   (§8.2): a top-level `LICENSE` decides; with none, a licence declared consistently in
   `package.json`, `Cargo.toml` and `tree-sitter.json` counts (basis `declared`); a licence
   file under `queries/` or `src/` that disagrees refuses. If it exits 1 (off the allowlist,
   AGPL, unlicensed), stop: disposition `refused`, reason `licence <SPDX>`.
6. Start the row from the catalogue: `indexbench lang-pin -id <id>` prints the draft lock row
   (repo, commit as the tag, ABI, `generate`, licence, entry point, subdirs) from
   `docs/plans/language-breadth.grammars.json`, which pre-fills the queue's unlocked tags items
   from tree-sitter-language-pack's catalogue (MIT) at a pinned revision. Its flags tell you
   what to check: `grammar-differs:<queue source>` (the catalogue names another repository than
   the queue), `abi-outside:<n>`, `not-in-catalogue`; `refused:<SPDX>` and `unlicensed` exit 1.
   The ABI is the catalogue's number, 14 unless it says otherwise; a grammar that ships a
   `src/parser.c` has its own, so read `LANGUAGE_VERSION` there. Set `scanner` from
   `src/scanner.c`. An item that names two grammars (gradle, scheme-racket) is pre-filled with
   its first; pin the second by hand. Then resolve the tag to a commit (`git ls-remote`),
   download the tarball, compute its sha256, and add the lock row (§8.1). Prefer a release tag
   that ships `src/parser.c`; if the grammar only ships `grammar.js`, set `"generate": true`
   and pin the CLI version.
7. Build: `internal/extractor/tsengine/grammars/build.sh <blob>`. Paste `blob_sha256`,
   `blob_bytes`, `gz_bytes` into the lock. If the gzip size exceeds 3 MiB, note it in your
   report (it spends the embedded budget).
8. If the grammar is already in the lock (a variant or a port), skip 5–7.

### 13.2 Write the dialect (red first)

9. Create `internal/extractor/dialects/<id>/dialect.json` from the manifest template (§2.2):
   kinds from the language's real vocabulary, `required_kinds` from your entry's
   `required_kinds` (or, if absent, every definition kind the language has), `oracle_kinds`
   for the oracle(s) in your entry, strategies from §6 (`imports.strategy` only when the
   target is L3 or L4). Start the `match` block from
   `go run ./cmd/indexbench lang-linguist -match <id>` (linguist's extensions, filenames as
   basenames, interpreters as shebangs, and a sniff for an extension another queue item or
   dialect claims); it warns about each claimed extension, and the sniff and its priority
   stay yours to settle (§5).
10. Write 3–6 fixtures in `testdata/`, each ≤ 200 lines, covering every required kind,
    nesting (a method in a class in a namespace), a callable with a multi-line signature,
    calls (plain, method, qualified), imports (when L3), and one syntax error the parser must
    survive. Write each `.golden.json` **by hand, from reading the fixture**, before any
    query exists. Run `go test ./internal/extractor -run TestDialectGoldens/<id>` and see it
    fail (red). Commit nothing yet.
11. Draft `tags.scm`: start from, in order, the grammar's own `queries/tags.scm`, aider's
    `aider/queries/**/<lang>-tags.scm`, then nvim-treesitter's `locals.scm` for shapes —
    each only if its licence is on the allowlist; never GitNexus. Put the provenance header
    (§8.2) on the file. Fetch a `locals.scm` at a pinned commit and draft it with
    `go run ./cmd/indexbench lang-draft -locals <locals.scm> -source "<repo> <path> @
    <40-hex commit> (<SPDX>)" -out tags.scm [-imports-out imports.scm] [-dialect <id>]`
    (LB-F47): each pattern with one `@local.definition[.kind]` becomes `@name` with
    `@definition.<kind>` on its outer node; scopes, references, variables, parameters and
    fields are dropped; imports go to the imports draft; a predicate the engine does not run
    is flagged `; UNSUPPORTED predicate …` above its pattern, and a `#lua-match?` becomes
    `#match?` only when its Lua pattern maps to RE2. MPL-2.0 is refused, because a draft
    modifies the file. Tune the draft with `lang-probe` (step 12). It is a starting point,
    never an oracle: the header it writes names nvim-treesitter, so `lang-oracle -tags-query`
    refuses that file as the same dialect's oracle (LB-F52). Add `@body` captures for
    signatures, `@scope.package` if the language declares packages, and `@name` inside every
    `@reference.call`.
12. Iterate with `go run ./cmd/indexbench lang-probe -dialect <id> <fixture>` until
    `TestDialectGoldens/<id>` passes. Keep queries structural; use only the supported
    predicates (§3.2). If `TestDialectQueriesCompile` names an unsupported predicate,
    rewrite the pattern — do not extend the engine.
13. L3 only: write `imports.scm` with the `@import.*` captures and fixtures that import
    across two fixture files; `TestDialectImports/<id>` must resolve them with the manifest's
    strategy. If no existing strategy fits, stop at L2 and report `needs-strategy: <why>`
    (an Opus row adds strategies, you do not).
14. Doc-engine items: write `doc.json` (§4) instead of steps 11–13, and fixtures drawn from
    the format's own documentation examples (licence-checked) or written by you.
15. Ports (`port_of` set): keep every `SourceKind` string the incumbent emits; run the
    incumbent's existing tests against your dialect too.

### 13.3 Calibration subjects

16. For each entry in `calibration_subjects` without a shard in `calibration/corpus.d/`,
    create `calibration/corpus.d/<id>.yaml` in the corpus schema: repo URL, a release tag,
    the commit from `git ls-remote`, the licence (must be OSI-approved; the subject is only
    read, never vendored), `subpath` when the repository is larger than 50 MB or holds more
    than 5,000 files of the language, and 3–6 idioms the subject exercises.
17. Clone with `task calibrate-clone -- <id>` (shallow, into `workspace/calibration/`).
    Check the clone's size; if over 500 MB, pick a `subpath` or another subject.

### 13.4 Measure

18. Oracle B: universal-ctags from `workspace/tools/ctags` (a pinned release; if absent,
    install the pinned build there per `calibration/README.md` — never a global install
    without asking). Check `ctags --list-languages` for your language. If ctags does not
    know it, your L1 oracle is tier C, and L1 can only be `good`: a third-party definitions
    query (the grammar's upstream `tags.scm`, a helix, aider or Sourcegraph `tags.scm`, an
    nvim-treesitter `locals.scm` or a Zed `outline.scm`) that your dialect was not drafted
    from, fetched by commit into `workspace/tmp/oracle/` (for example `curl` of
    `raw.githubusercontent.com/<repo>/<commit>/<path>`) and run with `lang-oracle -tags-query`
    (§10; D5 extension, §1.5). Record its licence; an AGPL grammar's query is refused.
19. Oracle A (L2–L4 targets, when your entry names a SCIP producer): install the pinned
    indexer inside your scratch dir (npm `--prefix`, `cargo install --root`, a release
    binary; JVM `-Xmx2g`; `nice -n 10`), run it on the subject with a 20-minute timeout,
    and keep `index.scip` under `workspace/tmp/scip/`. If it needs a build the subject cannot
    do offline in 20 minutes, record `oracle A unavailable: <why>` and continue with tier C
    for L2; L3/L4 then cannot be excellent. For tool formats, run the tool-native oracle
    named in your entry into `workspace/tmp/oracle/`.
20. Run the harness (§10) for each subject into `docs/bench/readings/lang/<id>.json`.
21. Read the reading's `level_met` and `grade`. Iterate queries while a metric misses the
    excellent threshold for a reason the queries can fix (missing kind, wrong containment,
    missed call shape). Each iteration adds or fixes a fixture first. Stop iterating after
    three rounds without movement.

### 13.5 Register and document

22. Run `go test ./internal/extractor ./internal/connector ./internal/fusion -run
    'Dialect|Tracker|Grammar|Classify|Unsupported'` (with `-race`), then
    `go run ./cmd/indexbench lang-tracker > docs/languages.md`.
23. Plan row: in `docs/plans/language-breadth.md`, your row `LB-Q-<id>` already carries its
    contract (a lexical oracle on your reading's `level_met`). Do not edit the contract.
    Mark it `[x]` only if `fathom verify --service fathom --plan docs/plans/language-breadth.md`
    would pass it: your reading exists and its `level_met` meets the row's pattern. If you
    reached less than the target, see §14.2 — do not change the pattern.
24. CHANGELOG: one line under `[Unreleased]` → `### Added` (new language) or `### Changed`
    (a port): "`<Name>` extraction at <level> (<grade>), measured against <oracle> on
    <subjects>". A port that changes output for already-indexed files also bumps
    `fusion.ExtractorEpoch` in the same commit.
25. Delete your clones unless the next queued item shares a subject
    (`task calibrate-clean -- <id>`); delete your scratch dir.

### 13.6 Commit and report

26. One commit (two for a port: the dialect, then the incumbent's removal), conventional
    message `feat(lang): <name> at <level> (<grade>)`, with a `Verified:` line naming the
    tests you ran, the reading's `level_met`, and the oracle tiers. No `Co-Authored-By` or
    session trailers. Do not push; the coordinator merges.
27. Report, in at most 12 lines: id, level met and grade per level, the metrics that missed
    excellent and why, oracle tiers, grammar blob size (gz), peak module pages, compiled
    bytes, faults, anything flagged for Opus review (§9's thresholds, `needs-strategy`,
    licence doubt), and disk you left behind (should be none).

## 14. Definition of done, and when a level is out of reach

### 14.1 Done per level

| Level | Done when |
|---|---|
| L1 | goldens pass; every `required_kind` emitted with range and (callables) signature; reading `level_met ≥ L1` (the plan row proves the level at good or excellent; the grade is shown by the tracker and counted by the milestones, and excellent needs tier A or B) |
| L2 | L1 done; call pairs measured against tier A (excellent/good) or tier C (good) |
| L3 | L2 done; `imports.scm` + a strategy (Go: its loader and typed pass, LB-F58); `import_resolution` and file-edge metrics measured against SCIP, or both against a language-native import dump at its tier (§10, LB-F40, LB-F57: the dump's statements, not references, are the file-edge truth); a tool format's `xref_precision`/`xref_recall` against its oracle's `cross_references`, from the doc engine's references or a tags-engine format's file-resolved edges (LB-F30, LB-F56) |
| L4 | L3 done; a SCIP producer run on ≥ 1 subject, imported with `fathom ingest --scip` into a scratch store, `def_mapping` and `typed_recall` measured; the tracker names the producer and version |
| any | tenancy numbers inside §1.2's floor; tracker regenerated; CHANGELOG line; no disk left behind |

### 14.2 When the target is not reachable

A worker never weakens a threshold, a pattern, a golden or a `required_kinds` list to make
a level pass. Instead:

- **Partial**: the level below the target is met. The row stays `[ ]`; the worker adds a
  sibling row `- [-] <name> beyond <level met> … {id: LB-Q-<id>-rest, disposition: deferred,
  reason: "<metric> <value> vs <threshold>: <cause>", trigger: "<what would change it>"}` and
  the reading's `disposition` names the same cause. The coordinator may re-target the queue
  entry (a new, lower target is an Opus decision, recorded in the entry's `retargeted` field
  with the reason).
- **Refused**: licence not on the allowlist, no grammar that parses real code (parse error
  rate > 5% of files), or the language cannot be classified without guessing →
  `disposition: refused` with the reason; the language stays disclosed as unsupported.
- **Deferred**: blocked on an oracle (no ctags support, no producer) or a strategy
  (`needs-strategy`) → `disposition: deferred` with the trigger that unblocks it.

A disposition is a result. A fake claim is an incident.

## 15. The loop (how the queue is consumed)

- **The queue file.** `docs/plans/language-breadth.queue.json` is a top-level JSON array in
  priority order. Each entry: `id`, `name`, `kind` (`language`|`tooling`), `target_level`,
  `grammar_source` (`owner/repo`, `native:<package>`, or `pure-go:<decoder>`), `licence`
  (of the grammar or parser; "verify at pin" means recipe step 5 decides), `oracle` (the
  intended oracles with their tiers; "unverified" means check `ctags --list-languages`),
  `calibration_subjects` (proposed; the worker pins them, and may substitute one only when
  it fails §13.3's licence or size rule, saying why), `depends_on` (plan row ids), `wave`,
  `engine` (`tags`|`doc`|`native`|`tags+doc`), `plan_row`, `worker_model`, and optionally
  `port_of` (the incumbent the item retires) and `notes` (the kinds and edges that matter).
  Only an Opus triage edits an entry (re-target, re-pin, re-order), recording why in a
  `retargeted` field; `TestLanguageQueueMatchesPlan` keeps it in step with the plan.
- **Driver.** The harness (a workflow script or `/loop` in a coordinator session) reads the
  queue in order, skips entries whose plan row is `[x]` or `[-]`, and dispatches the first
  entry whose `depends_on` are all done (a queue-item dependency is done at a committed L1
  reading, D6; `indexbench lang-next` lists the eligible items). Fathom serves the facts (`fathom plans next
  --eligible-only` over the plan; the queue file); it never runs the loop (vision 24).
- **Parallelism.** At most two workers at once, on items with disjoint grammar blobs; one
  heavy measurement at a time (the lock in §13.0). Each worker has its own worktree and
  scratch dir.
- **Models.** Foundation rows (LB-F…) and triage: Opus. Queue items: Sonnet, one fresh
  context per item, given only §13, the entry and the manifest template. Re-measurement
  after an engine change, tracker regeneration and the CHANGELOG roll-up: Haiku. A reviewer
  pass on each merged item: Sonnet, reading the diff and the reading only.
- **Tokens.** A worker's context is the recipe (~4k tokens), its entry, the probe output of
  its fixtures and its test output — not the codebase, not the grammar sources. The
  queue entry carries every decision a worker would otherwise have to research. Expected
  spend is recorded per item in the coordinator's report (tokens in/out, wall time), so the
  loop's cost per language is itself measured.
- **Merge.** The coordinator merges items one at a time: rebase, regenerate the tracker,
  run `go test -race ./internal/extractor/... ./internal/connector ./internal/fusion
  ./internal/langbench ./cmd/indexbench`, then `task validate` once per wave, never per item.
- **Failure.** Two failed attempts on an item → the coordinator writes the disposition
  (§14.2) and moves on; the item is re-queued only by an Opus triage with a changed input
  (a new grammar pin, a new strategy, an oracle that became available).
- **Waves.** After each wave: LB-M2 (re-measure everything already registered — Haiku),
  LB-M3/LB-M4 (footprint and binary size), the milestone count (LB-M6), and a CHANGELOG
  roll-up.

## 16. Rollout

1. Foundation wave F (Opus): LB-F1 … LB-F19 in dependency order; LB-M1 baseline before
   LB-F1 lands so parity is measured against today.
2. Waves 1–7 of the queue (Sonnet), each closed by the measurement rows.
3. Retirement: the ports in waves 1–3 delete their incumbents as they land; LB-F19 closes
   the CGO chapter when the last one merges.

## 17. Records this design creates or changes

- ADR-048 (this program). ADR-024's dialect row refined; ADR-013's status gains "the CGO
  bindings leave with LB-F19"; futures F1 promoted; question Q6 answered.
- Invariants I54 (no language is claimed above its reading) and I55 (a grammar or pack
  loads only when the lock or the user-scope trust list describes its bytes and its licence
  is allowed), both `stated` until LB-F17 and LB-F12/LB-F14 land.
- `internal/AGENTS.md` "Adding things → Extractor" points here.

## 18. Excellence program (2026-10-01)

**Owner directive, 2026-10-01**, verbatim:

> keep going until all of the most common languages and tools are at excellent level or as close as we can get.

> especially well in c and cpp and Conan and all cloud infra and iac and policy as code and that whole ecosystem. As well as common proxies like nginx, envoy, mongobetween, etc if applicable. Should fathom be one of the tools that gets done too

And the catalog question: "are you keeping the supported languages and tools and quality catalog updated as we go so we can track what we have and what we need durably?" The answer is the queue (`tier` field), the generated excellence board in `docs/languages.md`, and this section.

### 18.1 Done means

An item is done when every level up to its target level reads **excellent** by the §1.2 bars, or it stops at a **documented ceiling** named in its `-rest` row with the trigger that would lift it (§14.2): an oracle cap (a tier-C oracle caps a level at good), no oracle or no SCIP producer, a grammar error rate over the line, or an upstream defect. "Close" means the written ceiling, never a silent stop.

### 18.2 Decisions (owner-architect, 2026-10-01; the owner may override)

- **D1, dispatch at parity.** A tags or doc port replaces the incumbent extractor in ingest once lang-parity proves it at least as good as the incumbent on every measured metric at its demonstrated level, whatever its target level. Reason: improvements must reach the dispatched path. The PHP dialect is measured, but ingest runs the CGO `phpextract.go`; a better port that ingest never calls helps no agent. Recorded as an ADR-048 amendment. The ingest cost guard LB-M5 (1.85x against the 1.5x bar, O208) must come back under the bar with the switch, or the `-rest` rows say why.
- **D2, concurrent epoch rule.** When two waves both change extraction, the one that ships renumbers to `ExtractorEpoch` = origin/main's + 1. Reason: two concurrent bumps collide on the same number and one silently re-extracts nothing. Recorded in the same amendment.
- **D3, order.** Commonness times the levels still short of excellent: the most common language or tool with the largest gap goes first. One language wave and one tools wave run in parallel, on disjoint files. Reason: parallelism is bounded by shared files (the queue, the plan, the tracker), not by workers.

### 18.3 Tiers and target levels

These tiers are different from the oracle tiers A-D in readings. The 2026-10-01 snapshot:

- **T1, languages (most common first).** TypeScript, JavaScript, Python, Java, C#, Go, C, C++, PHP, Rust, Kotlin at L4; Ruby L3; Swift L3 (refused today on grammar error rate); Dart L4; Scala L4; Shell L2; SQL L2; HTML, CSS, SCSS L3; React L3 (added 2026-10-10, LB-Q-react: components, props, hooks and React Router routes, a sub-row of the TypeScript and JavaScript dialects).
- **T2, build, package and CI.** Dockerfile, Compose, GitHub Actions, GitLab CI, Makefile, package.json, Maven, Gradle, Python packaging, protobuf, CMake (the file API as tier-A oracle), OpenAPI, GraphQL, Taskfile, Cargo, go.mod, MSBuild, Jenkinsfile, Azure Pipelines, CircleCI, composer, Bazel (`cc_*` rules), JSON Schema. The C/C++ emphasis adds Conan L3, vcpkg L3, Meson L3 and `compile_commands.json` L2.
- **T3, IaC.** Terraform/OpenTofu, Terragrunt, Pulumi YAML, CloudFormation/SAM, AWS CDK (`cdk.json`; the programs ride their language), Bicep, ARM templates, Serverless, Ansible, HCL tool configs (Packer, Nomad, docker-bake), Crossplane, Argo, Flux, Helm, Kustomize, Kubernetes.
- **T4, policy as code.** Rego (refused on grammar), Gatekeeper, Kyverno, Conftest (rides Rego), Cedar, AWS IAM policy, Azure Policy, Cloud Custodian, Checkov custom policies, Sentinel (likely ceiling: no open grammar).
- **T5, proxies, gateways and mesh.** nginx, Envoy, HAProxy, Traefik, Caddy, Istio, Kong, Apache httpd (all wave px), mongobetween (applicable only through the K8s or compose launch args that start it, wave px), PgBouncer, ProxySQL.
- **T6, fathom's own formats.** CUE L4, `.fathom/config.toml`, `concerns.cue`, layer packs, plan grammar v2 rows. The oracle must be independent of the extractor: `cue export`, an independent TOML decode, the plan grammar's CUE schema; never fathom's own loader, or the reading proves nothing.
- **T7, observability as code (added 2026-10-09, LB-F318; ADR-062).** Prometheus rules, configuration and promtool tests L3 (rules and PrometheusRule specs share one grammar; Thanos ruler reads it too), the OpenTelemetry Collector configuration; their label matchers and telemetry identities join the runtime topology through `deploy:` keys (§4 "PromQL references"). Wave c3 (2026-10-10, ADR-065): the Alertmanager configuration L3 (its routes join the alerting rules they route, amtool tier A) and Grafana dashboards and provisioning L3 (a panel joins the recording rule it reads; a tier-C oracle caps it at good), §4 "Observability documents"; neither joins a label to a `deploy:` key (K19).

### 18.4 Authority, and the loop

The queue's `tier` field is the source of tier membership (`schema/langqueue`, `#Tier`); the generated **Excellence board** in `docs/languages.md` is the live view; the lists in §18.3 are the 2026-10-01 snapshot and are not edited as the queue moves. An entry without a `tier` is outside the excellence target set.

The loop: after each wave, re-read the board, re-plan from the gaps (D3), and stop an item at excellent or at a written ceiling (§18.1).

The runtime-topology frame (proxy route to Service to Deployment to image to Dockerfile, IaC resource to the policy that governs it) is named only as a pointer to futures F61. It is not a goal of this program.
