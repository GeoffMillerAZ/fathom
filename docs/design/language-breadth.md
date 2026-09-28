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
| `file_edge_precision` / `file_edge_recall` | file-resolved edges vs SCIP cross-file edges at fathom granularity | | | | | ≥ .95 / ≥ .50 | ≥ .90 / ≥ .25 | | |
| `def_mapping` | SCIP definitions of mappable kinds mapped to a fathom symbol ÷ those definitions | | | | | | | ≥ .95 | ≥ .85 |
| `typed_recall` | SCIP reference edges present as typed edges at fathom granularity ÷ SCIP edges | | | | | | | ≥ .95 | ≥ .85 |

Always required, every level: `faults ≤ 0.1%` of files with every faulting file named;
`throughput ≥ 1 MB/s/core`; `max_file_ms ≤ 2000`; `peak_module_pages ≤ 1024`.

Tool formats: L1 `sym_recall` and `sym_precision` ≥ .98 (good ≥ .95); L2 reference
recall and precision ≥ .95 (good ≥ .90); L3 precision ≥ .95, recall ≥ .80 (good .90/.60).

### 1.3 Oracle tiers and what they can prove

| Tier | Oracles | May prove |
|---|---|---|
| A | SCIP index from the language's maintained indexer; a native type checker; the tool's own evaluation (`make -pn`, `bazel query`, `terraform graph -type=plan`, `helm template`, `kustomize build`, `docker compose config`, `gitlab-ci-local --list`, the format's reference parser) | any level, excellent |
| B | universal-ctags (`--output-format=json --fields=+nKzSe --extras=-{anonymous}`) run as a subprocess | L1 excellent |
| C | the grammar's upstream `queries/tags.scm` run by the tree-sitter CLI; an independent parser library | at most good, any level it covers |
| D | grep proxy (`baseline.sh`); a sample labelled by the extractor's own author | "tolerated" only |

A reading records the tier per metric. The harness computes `level_met` from the metrics
and tiers alone; nobody types it.

### 1.4 Matching rules

- **Definitions**: key (path, name, line). A fathom definition matches an oracle definition
  in the same file with the same bare name and |Δline| ≤ 2 (decorators and attributes move
  a definition's first line); nearest wins, one-to-one. Qualified names are compared on the
  last segment; containment is checked separately (`scope_agreement`, reported, not gated).
- **Kind intersection**: the manifest's `oracle_kinds` maps each oracle kind (ctags kind
  letters/names, SCIP descriptor suffixes) to a canonical kind. Definitions whose kind is
  outside the intersection on either side are excluded from both counts and reported as
  `excluded_by_kind`. A dialect may not exclude a kind the vision's L1 list names for the
  language (the manifest's `required_kinds`) — the harness refuses the reading.
- **Calls**: truth pairs are (enclosing definition, callee bare name) derived from SCIP
  reference occurrences whose symbol is callable (SCIP method descriptor `().`), inside a
  definition that maps to a fathom symbol.
- **Scope**: generated files (`// Code generated`, `@generated`, minified), vendored trees
  and test fixtures inside the subject are excluded by the same rules fathom's connector
  applies, so the oracle is run over exactly the file list fathom would read (the harness
  passes it `-L <filelist>`).

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

Selectors are a deliberately small JSONPath subset (`$`, `.key`, `.*`, `[*]`, `$key`,
alternation `|`), implemented in Go over the decoded tree — no expression language, no
templating. Formats whose meaning needs evaluation (Helm templates, Jsonnet, Starlark,
Nix, CUE) are not doc-engine formats: they are syntax dialects on the tags engine, and
where a tool can evaluate them it is the tier-A oracle. A doc-engine tool format reaches
L3 through `cross_file` selectors resolved by strategy (`repo-path`, `image-ref`,
`module-source`, `service-name`).

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
relative to the importer first), `case: "snake"` and `labels: "bazel"`; an omitted one takes
the strategy's default, which for `relative-path` and `dotted-module` is the pre-LB-F11
TS/JS and Python rule (`internal/fusion/resolvestrategy.go` documents each strategy's reads).
Under `include-path`, `file-symbol` and `swift-module` a whole-module import brings every
top-level name of its target into scope, as does any `@import.wildcard`; a name two targets
define is left unresolved.
A new strategy is a foundation-level change (an Opus row), never something a worker adds.
Edges keep the existing label: `file-resolved`, tier pre-scan, never presented as typed.

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
      "entry_points": ["tree_sitter_swift"], "subdir": "", "scanner": true,
      "generate": false, "abi": 14,
      "blob_sha256": "<64 hex, of the uncompressed .wasm>", "blob_bytes": 0, "gz_bytes": 0 }
  ]
}
```

`build.sh <blob>` builds one grammar from the lock (fetch tarball, check `tarball_sha256`,
compile with the existing zig recipe plus the predicate export, `wasm-opt -Oz`, gzip -9),
and prints the three numbers to paste back. A grammar whose release ships only
`grammar.js` sets `"generate": true` and the recipe runs a pinned `tree-sitter-cli` via
`npx --yes tree-sitter-cli@<pinned> generate` in scratch (never a global install).
`PROVENANCE.md` becomes a generated rendering of the lock plus the prose that is there today.

### 8.2 The licence allowlist

Allowed for grammars, queries and fixtures: MIT, Apache-2.0, BSD-2-Clause, BSD-3-Clause,
ISC, Unlicense, CC0-1.0, Zlib, 0BSD; MPL-2.0 only for files taken unmodified. Refused: GPL
(any), LGPL, AGPL, SSPL, EUPL, PolyForm (any), BUSL, FSL, Commons Clause, "no licence" and
"licence unclear". The check reads the grammar repository's top-level `LICENSE` at the
pinned commit **and** any licence file under `queries/` and `src/`; a mismatch refuses.
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
| Process-wide parser pool | ≤ 6 live parser modules (LRU across all dialects); a module idle 2 minutes is closed; `tsengine.LiveModules()` stays the instrument | LB-F15 |
| Compiled-code accounting | each compiled grammar's size (measured once at compile, recorded in the reading) is charged to the memo registry's byte budget (I51); shed evicts least-recently-used compiled grammars, refuse drops all | LB-F15 |
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
  [-tool-oracle workspace/tmp/oracle/<id>-<subject>.json] \
  -out docs/bench/readings/lang/swift.json   # merges subjects: one reading per dialect
```

It lists the subject's files through the connector's own discovery (so exclusions match),
runs the dialect's extractor on each, runs the oracle(s) on the same file list, matches per
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

`level_met` is the highest level whose metrics all pass at excellent or good with an
admissible tier; `grade` says which. The file is written with
`json.MarshalIndent(reading, "", "  ")` so `"level_met": "L3"` is always one line — the plan
rows' lexical oracle matches that line. Readings are small (a few KB) and committed; raw
oracle output stays under `workspace/`.

`TestLanguagesTrackerMatchesRegistry` gains a third assertion (LB-F17): a tracker row's
level is never higher than its reading's `level_met`, and a registered dialect without a
reading is listed as `tolerated`.

## 11. Retiring heuristics and bespoke walkers

The nine CGO walkers and three regex extractors each have a queue item whose target is at
least their current output. The protocol (LB-F18, and each port item):

1. Write the dialect (tags or doc engine) alongside the incumbent; both registered, the
   incumbent still dispatched.
2. `indexbench lang-parity -dialect <id> -incumbent <extractor>` runs both on the calibration
   subjects and reports, per metric, new minus old against the same oracle. The port may
   merge when every L1 metric is ≥ the incumbent's and the target level is met.
3. Switch dispatch to the dialect, delete the incumbent and its CGO grammar import, bump
   `ExtractorEpoch`, record the kinds diff (identical `SourceKind` strings are required; a
   kind the incumbent emitted and the dialect does not is a regression unless the row's
   reason names why).
4. When the last CGO walker is gone: `go.mod` loses `github.com/tree-sitter/go-tree-sitter`
   and the per-language CGO modules (LB-F19, a `gone: module` row), `internal/CLAUDE.md`'s
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
   (`fathom plans status`). If not, stop and report `blocked: <ids>`.
3. Work in a fresh worktree on branch `lang/<id>` from the integration branch, with
   `GOWORK=off`, your own scratch dir `/Volumes/m2ext/tmp/lang-<id>/` (never shared), and
   `GOFLAGS=-p=2`. Never start a daemon, never touch `127.0.0.1:7575` or `~/.fathom`.
   If a read under `/Volumes` fails with "Operation not permitted", run
   `tccutil reset SystemPolicyRemovableVolumes com.mitchellh.ghostty` and retry.
4. Take the measurement lock before any heavy step (clone, indexer, harness):
   `mkdir /Volumes/m2ext/tmp/lang-measure.lock` (retry every 60 s; remove it when the step
   ends). One heavy measurement runs machine-wide at a time.

### 13.1 Pin the grammar (tags-engine items only; skip for doc-engine items)

5. Confirm the licence at the pinned commit: fetch the repository's `LICENSE` (and any
   licence under `queries/` and `src/`) at the tag you will pin. If it is not on the
   allowlist (§8.2), stop: disposition `refused`, reason `licence <SPDX>`.
6. Resolve the tag to a commit (`git ls-remote`), download the tarball, compute its sha256,
   and add a lock row (§8.1). Prefer a release tag that ships `src/parser.c`; if the grammar
   only ships `grammar.js`, set `"generate": true` and pin the CLI version.
7. Build: `internal/extractor/tsengine/grammars/build.sh <blob>`. Paste `blob_sha256`,
   `blob_bytes`, `gz_bytes` into the lock. If the gzip size exceeds 3 MiB, note it in your
   report (it spends the embedded budget).
8. If the grammar is already in the lock (a variant or a port), skip 5–7.

### 13.2 Write the dialect (red first)

9. Create `internal/extractor/dialects/<id>/dialect.json` from the manifest template (§2.2):
   kinds from the language's real vocabulary, `required_kinds` from your entry's
   `required_kinds` (or, if absent, every definition kind the language has), `oracle_kinds`
   for the oracle(s) in your entry, strategies from §6 (`imports.strategy` only when the
   target is L3 or L4).
10. Write 3–6 fixtures in `testdata/`, each ≤ 200 lines, covering every required kind,
    nesting (a method in a class in a namespace), a callable with a multi-line signature,
    calls (plain, method, qualified), imports (when L3), and one syntax error the parser must
    survive. Write each `.golden.json` **by hand, from reading the fixture**, before any
    query exists. Run `go test ./internal/extractor -run TestDialectGoldens/<id>` and see it
    fail (red). Commit nothing yet.
11. Draft `tags.scm`: start from, in order, the grammar's own `queries/tags.scm`, aider's
    `aider/queries/**/<lang>-tags.scm`, then nvim-treesitter's `locals.scm` for shapes —
    each only if its licence is on the allowlist; never GitNexus. Put the provenance header
    (§8.2) on the file. Add `@body` captures for signatures, `@scope.package` if the
    language declares packages, and `@name` inside every `@reference.call`.
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
    know it, your L1 oracle is tier C (the upstream `tags.scm` through
    `npx --yes tree-sitter-cli@<pinned> tags`), and L1 can only be `good`.
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
| L3 | L2 done; `imports.scm` + a strategy; `import_resolution` and file-edge metrics measured against SCIP |
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
  entry whose `depends_on` are all done. Fathom serves the facts (`fathom plans next
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
- `internal/CLAUDE.md` "Adding things → Extractor" points here.
