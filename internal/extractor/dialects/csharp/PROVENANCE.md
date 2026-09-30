# csharp — provenance

- Grammar: `tree-sitter/tree-sitter-c-sharp v0.23.5` (MIT), commit
  `cac6d5fb595f5811a076336682d5d595ac1c9e85`, ABI 15, has a scanner; built into
  `internal/extractor/tsengine/grammars/csharp.wasm.gz` (3,056,786 bytes raw,
  603,861 bytes gzip — under the §8.2 3 MiB note threshold). Pin and digests in
  `internal/extractor/tsengine/grammars/grammars.lock.json`
  (`internal/extractor/tsengine/grammars/PROVENANCE.md` regenerates its own
  tables from it).
- `tags.scm`: written in this repository (LB-Q-csharp, docs/design/language-breadth.md
  §13), a port of the incumbent CGO `csharpextract.go` walker. Upstream's own
  `queries/tags.scm` was read for reference (captures class/interface/method/
  namespace definitions plus several `@reference.class`/`@reference.send`
  type references fathom does not consume, design §3.4) but not reused
  verbatim — every pattern here was written against this repository's own
  `indexbench lang-probe` output (design §13 step 12), which surfaced this
  grammar's actual field names (`name`, `body`, `accessors`, `value`,
  `returns`, `parameters`) directly. Kept from the incumbent (§11 step 3, port
  parity): `CSharpNamespace`, `CSharpClass`, `CSharpInterface`, `CSharpStruct`,
  `CSharpEnum`, `CSharpMethod` source-kind strings unchanged;
  `record_declaration` mapped to `CSharpClass` (the incumbent's own choice —
  `record`/`record class`/`record struct` are not distinguished by any named
  field in this grammar, verified directly, so this port does not distinguish
  them either); `constructor_declaration` mapped to `CSharpMethod`; no
  recursion into enum bodies (member values are not symbols). Added beyond
  the incumbent (a port may add, never drop, §11 step 3):
  - `CSharpProperty` (queue note "properties" — the incumbent never extracted
    properties at all). Not a callable kind, so it carries no `Signature`
    (the engine only computes one for `callable` kinds, `tsextract.go`).
  - File-scoped namespace handled identically to block-scoped (queue note
    "namespaces (block and file-scoped)" — the incumbent's own `walkCSharp`
    already special-cased both node kinds), plus `@scope.package`/`@scope` so
    `SymbolNode.Package` carries the namespace instead of `pkgFromPath`'s
    directory guess (design §3.5, "C#'s shape" — see
    `internal/extractor/tspackage_test.go`'s own comment naming this exact
    case). A block namespace's own node range is the package scope for
    everything inside it, nested block namespaces join with "."
    (`tsPackageFromCapture`'s outer-first ordering); a file-scoped namespace
    has no bounded body node in this grammar at all (verified: its siblings
    in `compilation_unit` are not its children), so it reads as the file-wide
    default instead (the same shape Kotlin/Java package headers use).
  - Conversion operators (`public static implicit/explicit operator T(...)`)
    — real symbols the incumbent never extracted. This grammar's
    `conversion_operator_declaration` has no name field at all (verified:
    `lang-probe` on `JToken.cs`'s own conversion operators shows only
    modifier/type/parameters/body), so the target type stands in for a name
    (`Money.bool`, `Money.Money` in `testdata/operators.cs`) — real and
    useful to fathom's own callers, but it does not name-match
    universal-ctags' own synthetic `"operator bool"`/`"operator JToken"`
    naming (a literal `"operator "` prefix no structural capture can
    reproduce — the query engine captures node text, never a synthesized
    string). A plain symbolic operator (`operator +`, `operator ==`, ...) has
    no name-adjacent capturable node whatsoever in this grammar (not even a
    stand-in type) and is not attempted — a disclosed, bounded L1 gap (2
    instances total across both calibration subjects, `docs/bench/readings/lang/csharp.json`'s
    notes).
  - Partial types: "partial" is a plain modifier token in this grammar (not
    a distinct node), so two `partial class` bodies for the same type — even
    across files — simply produce two `CSharpClass` definitions, matching
    every other partial-type precedent in this codebase (no merge step
    exists anywhere in the tags engine, verified against `testdata/shapes.cs`'s
    own two-declaration `Circle`).
- `imports.scm`: written in this repository (no upstream or incumbent
  equivalent — the CGO walker never resolved imports). `using X;`/
  `using X.Y.Z;`/`using Alias = X.Y;`/`using static X.Y;`/`global using X.Y;`
  via two anchored patterns (a single optional `name:` field ahead of an
  unqualified `(_)` double-matched every aliased statement — verified
  directly, tree-sitter's query engine explores both "alias consumed the
  first child" and "alias skipped, `(_)` consumed it instead" as independent
  matches once a later unqualified capture could re-bind it; "." anchors
  pin exact child count and order instead, one match per statement, either
  shape). `imports.strategy: namespace-dir` (design §6) with
  `suffixes: [".cs"]`; no `source_roots` (C# does not enforce a
  namespace-mirrors-directory convention the way Java does, so the
  strategy's own directory-tree fallback is the only lever available).
  `type_wildcard: "members"` (O145, LB-F45): a wildcard naming a type is
  `using static N.T;`, which brings T's members into scope. An aliased
  `using A = N.T;` or a `using static N.T;` resolves to the files declaring
  the type (namespace-dir's declared-type lookup: the symbol whose package
  and name spell N.T, a nested N.Outer.Inner included), wherever they sit, so
  `A.M()` binds T's member M. A `using static` member binds a bare `M()` only
  when no type enclosing the caller declares M (C#'s simple-name lookup searches
  the enclosing types before any using directive) and no other `using static`
  type declares it too.
  `internal/fusion/csharpfileresolve_test.go` proves the wiring end to end
  for the one call shape namespace-dir's `whole` binding actually reaches
  (an aliased, receiver-qualified call) — **not** the ordinary
  `using X; then Y.Z();` idiom, which this strategy's `whole`-as-prefix
  semantics structurally cannot resolve (namespace-dir is not in
  `ResolveFileEdges`' `wholeIsWildcard` closed set); see the reading's own
  notes and `LB-Q-csharp-rest`.
- **Foundation fix alongside this dialect** (`cmd/indexbench/langoracle.go`):
  `lang-oracle` always measured fusion's live dispatch (`ExtractForBench`,
  `with=nil`), which for any not-yet-merged port (§11 step 2) is the
  incumbent, not the dialect just written — contradicting design §10's own
  "runs the dialect's extractor on each". Verified directly: the first
  csharp run under the unpatched tool read `signature_coverage: 0`
  (`csharpextract.go` never sets `Signature`) despite this dialect signing
  every callable in its own goldens; a fresh `lang-oracle -dialect c` run
  on the **c** dialect's own fixtures reproduced the identical symptom on
  this exact checkout, so it was not csharp-specific. Fixed by wiring the
  already-built `ExtractForBenchWith`/`with` mechanism (LB-F18, built for
  `lang-parity`, never connected to `lang-oracle`) — `lang-oracle` now
  constructs the dialect's own `TSExtractor` and measures it directly
  whenever `resolveBenchSpec` found a `dialect.json`. This affects every
  other not-yet-merged port's own future re-measurement, not just this one.
- `preprocess: "csharp-conditionals"` (O148, LB-F53): this grammar parses an
  `#if` group as a node whose branches must each hold whole declarations, so a
  group whose branches are two headers of one body (Newtonsoft.Json's
  `TestFixtureBase.cs` L279-L284: `#if DNXCORE50 / protected TestFixtureBase()
  / #else / [SetUp] protected void TestSetup() / #endif`, then the body), or
  one that closes a type and opens another, can leave the file's
  `namespace_declaration` unparsed, and every symbol then falls back to the
  directory's package. A file whose tree has an ERROR or MISSING node is
  parsed as `csharpConditionalTexts` (`csharpextract.go`) gives it: first
  with each group's directive lines blanked, every branch kept; then with
  each group read as its first branch. The first tree with no error is read;
  otherwise the file as written. Blanking keeps every byte offset and line.
  The CGO incumbent calls the same step (`withCSharpTree`). Measured on both
  subjects (the CGO grammar): of the 65 of Newtonsoft.Json's 944 files and 4
  of Polly's 801 with an error as written, 42 and 3 parse under it, 39 and 2
  of them with every branch kept; the first-branch
  reading drops a member declared only in an `#elif`/`#else` of such a file
  (`testdata/conditional.cs`'s `OnlyElse`). The compiler's own reading with
  no symbol defined was measured and refused: it drops every `#if
  HAVE_ASYNC`-wrapped file (sym_recall .9876 to .9623) and never reads
  `XUnitAssert`, which is declared under `#if DNXCORE50`.
- `testdata/`: fixtures written in this repository. `shapes.cs` (file-scoped
  namespace, every required kind, a nested class, a multi-line method
  signature, two call shapes — plain and qualified, two partial `Circle`
  declarations, a record); `namespaces.cs` (nested block namespaces,
  proving `@scope.package` nesting joins with "."); `imports.cs` (all five
  `using` shapes for L3); `operators.cs` (a conversion operator pair);
  `broken.cs` (a syntax error the parser must survive, `Good`/`Survivor`
  still extracting either side of it); `conditional.cs` (O148's shape,
  reduced from `TestFixtureBase.cs` with its trigger kept, read as first
  branches) and `branches.cs` (a group inside an argument list, read with its
  directive lines blanked and every branch kept). Goldens hand-derived from the
  documented engine algorithms (`tsextract.go`'s `tsSignature`/`tsEndLine`/
  `qualifyTSDefs`/`tsPackageFromCapture`) applied to each fixture's own
  content, cross-checked against `indexbench lang-probe`'s real capture
  output before the test first ran — and it passed on the first run against
  every fixture, i.e. this worker's own derivation matched the engine's
  actual behavior exactly.
- **Measurement (`docs/bench/readings/lang/csharp.json`,
  `calibration/corpus.d/csharp.yaml`):** universal-ctags 6.2.1 (tier B) on
  JamesNK/Newtonsoft.Json 13.0.4 (944 `.cs` files) and App-vNext/Polly 8.8.0
  (801 `.cs` files). `level_met` is `none`; L1 is `unmet`: `sym_recall` is
  good on both subjects (.8968, .9167) but `sym_precision` falls short of
  good (.8539, .8343 vs. .90) with no single dominant, fixable cause
  isolated across three investigation rounds — see the reading's own
  `notes` for exactly what was checked and ruled out (namespace tagging,
  a full-file diff of `JsonSerializer.cs`) and what was added but did not
  move the metric (conversion operators). L2–L4 were not attempted this
  round: no dotnet SDK in this worktree to build a scip-dotnet index, and
  L1 itself was not yet good. Full detail in the reading's own
  `notes`/`disposition`.
- **Merge gate (§11 step 2):** the target (L4) is not met, so the CGO
  incumbent (`csharpextract.go`) stays fusion's live `.cs` dispatch
  (`internal/fusion/pipeline.go`) — this dialect is written, tested and
  measured but not yet switched in. A future item reaching L4 (or a
  retargeted level, an Opus decision) flips `pipeline.go`'s one line and
  deletes `csharpextract.go`/`csharpextract_test.go`.
