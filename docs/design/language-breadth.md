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
convention). L4 is `not-applicable` for tool formats and is never claimed.

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
| `file_edge_precision` / `file_edge_recall` | file-resolved edges vs SCIP cross-file edges at fathom granularity; with no SCIP index, vs an import dump's (file, imported file) pairs at the dump's tier (LB-F57: statements, not references, §10); Go's are its typed pass's edges (LB-F58) | | | | | ≥ .95 / ≥ .50 | ≥ .90 / ≥ .25 | | |
| `def_mapping` | SCIP definitions of mappable kinds mapped to a fathom symbol ÷ those definitions | | | | | | | ≥ .95 | ≥ .85 |
| `typed_recall` | SCIP reference edges present as typed edges at fathom granularity ÷ SCIP edges | | | | | | | ≥ .95 | ≥ .85 |

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
   `"package": {"strategy": "capture"}`; otherwise the directory rule (`pkgFromPath`) stands.
6. **Injections** (LB-F6). `injections.scm` captures `@injection.content` with a static
   `#set! injection.language "<dialect id>"` (the one `#set!` the engine reads). The engine
   re-runs the injected dialect over that byte range, offsetting lines, one level deep only,
   and qualifies nothing across the boundary. This covers `<script>`/`<style>` in HTML, Vue,
   Svelte and Astro, and fenced code in Markdown (off by default for Markdown: documentation
   code is not the repository's code).
7. **Imports** (existing SU-11 vocabulary, generalized in LB-F7): `@import.stmt`,
   `@import.module`, `@import.name`, `@import.alias`, `@import.wildcard`, plus the
   `@import.from*`/`@import.rel*` captures the Python and JS queries already use.

Queries stay predicate-light and structural. The engine's match-scan budget
(`fathom_query_cursor_exec_bounded`, 10 s, the host backstop at 2×) applies unchanged.

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
`image-ref`, `module-source`, `service-name`, `label-selector`).

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

## 6. L3: import resolution strategies

`ResolveFileEdges` today resolves TypeScript/JavaScript and Python. L3 generalizes it by a
closed set of strategies (LB-F11), each a Go function over (importer path, module spec,
repo file set) → target files:

| Strategy | Languages | Rule |
|---|---|---|
| `relative-path` | JS/TS, CSS/SCSS/LESS `@import`, HTML `src`/`href`, Markdown links | existing `resolveTSModule` rule plus configured extensions and index files |
| `dotted-module` | Python, Elixir (`Mix` apps), Erlang, Haskell, OCaml, Julia | dotted name → path under discovered source roots (`src/`, `lib/`, the repo root) |
| `namespace-dir` | Java, Kotlin, Scala, C#, PHP (PSR-4 via `composer.json`), Dart (`package:` + `pubspec.yaml`), Groovy | package/namespace → directory; PHP and Dart read their manifest's autoload/name |
| `include-path` | C, C++, Objective-C, CUDA, GLSL | quoted includes relative to the file, then `-I` roots from `compile_commands.json` when present, then sibling `include/` dirs |
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
   builds are then possible is measured, not assumed (F1).

## 12. The tracker

`docs/languages.md` becomes generated (LB-F17): `go run ./cmd/indexbench lang-tracker >
docs/languages.md` renders one row per dialect from the manifests, the readings and the
queue — dialect, kind, engine, grammar (repo@tag, licence), target, level met with grade,
oracle tier, typed-edge producer, reading date, next. The hand-written preamble and the
typed-edge prose above the table are kept as a template. `TestLanguagesTrackerIsGenerated`
fails when the committed file differs from the rendering. Workers never hand-edit the table.

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
   `GOWORK=off`, your own scratch dir `/Volumes/m2ext/tmp/lang-<id>/` (never shared), and
   `GOFLAGS=-p=2`. Never start a daemon, never touch `127.0.0.1:7575` or `~/.fathom`.
   If a read under `/Volumes` fails with "Operation not permitted", run
   `tccutil reset SystemPolicyRemovableVolumes com.mitchellh.ghostty` and retry.
4. Take the measurement lock before any heavy step (clone, indexer, harness):
   `mkdir /Volumes/m2ext/tmp/lang-measure.lock` (retry every 60 s; remove it when the step
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
