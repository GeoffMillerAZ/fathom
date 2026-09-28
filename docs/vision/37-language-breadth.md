# 37 — Language breadth and depth: measured support for the languages and tools people use

Status: accepted 2026-09-27 (ADR-048). Owner directive of 2026-09-27: "our language
support is mostly Go. I want to be very competitive: AST symbols and dialect packs for
the tooling that exists out there. Use other projects as implementation inspiration to
adopt much deeper and wider support. Keep pressing until we have excellent language and
tooling support — efficient tokenomics, background tasks with appropriately sized models,
most common first, until we have a very respectable number." Design:
`docs/design/language-breadth.md` (the engine, the harness and **the worker recipe**).
Plan: `docs/plans/language-breadth.md` (LB-…). Queue: `docs/plans/language-breadth.queue.json`.
Tracker: `docs/languages.md`. Fires futures F1 (promoted into scope), answers Q6, and
un-defers XT-6, XT-6b and XT-7 (`docs/plans/extraction-treesitter.md`).

## 1. Where we stand (read from the tree at `f75dd9c9`, not argued)

| Fact | Evidence |
|---|---|
| 17 languages and ~16 tool formats are classified and extracted | `internal/connector/connector.go` `extClassMap`; `internal/fusion/pipeline.go` `New` |
| Five run on the generic tags-query engine (JS, TS, Python, Ruby, Kotlin): a grammar blob plus a `.scm` file, no per-language Go | `internal/extractor/tsextract.go`, `tsqueries/*.scm`, `tsengine/grammars/` (7.2 MB of wasm, 1.5 MB gzip -9) |
| Nine run on hand-written Go walkers over CGO tree-sitter (Java, C, C++, C#, Rust, Scala, PHP, Bash, Lua) — the tracker calls them "heuristic", which undersells the AST and oversells the coverage | `*extract.go` imports of `github.com/tree-sitter/go-tree-sitter`; `docs/languages.md` |
| Three are regex (SQL, Helm templates, templ); the config family is pure-Go structured decoding | `sqlextract.go`, `helmextract.go`, `templextract.go`; `ghaextract.go`, `composeextract.go`, `k8sextract.go` … |
| The tags engine never sets `EndLine` or `Signature`, though `core.SymbolNode` carries both | `TSExtractor.Extract` |
| No symbol extractor has ever been measured against an independent oracle; `calibration/baseline.sh` compares to a grep proxy | `calibration/README.md` "The oracle" |
| Typed edges beyond Go and CUE are measured only for TypeScript and Python (0.87/0.03 and 0.95/0.12 precision/recall for the file tier); every other "scip" row names a mechanism that has never run on that language | `docs/languages.md`; `docs/bench/readings/l0-ts-py-2026-09-24.md` |
| 14 languages are disclosed as unsupported, per service | `internal/connector/unsupported_language.go` |
| The reference operator's fleet census (2026-09-27, `git ls-files`, vendored trees excluded) holds unclassified HTML (~210 files), CSS/LESS/SCSS (~300), Rego (40), Astro (24), Nix, HCL outside `.tf`, Nushell and Swift (12, in the app-kit family) | census, counts only (ADR-011: no repository names) |

The field counts differently. aider ships tags queries for about 40 languages (Apache-2.0);
the language-pack projects bundle 160–370 grammars; several MCP code-graph servers claim
158–257 languages. None of those numbers is a measured recall against an oracle, and the
largest claims come from template listings whose origin could not be verified. Our number
will be smaller and **every unit of it will be measured**: a language counts at a level
only when a committed reading proves the level on real code (law 3, P22).

## 2. What "supported" means: four levels, each with a threshold and an oracle

| Level | Emits | Store | Excellent means | Good means | Oracle (tier) |
|---|---|---|---|---|---|
| **L1** symbols | definitions with kind, range (`Line`..`EndLine`), qualified name, package, signature for callables | `SymbolNode` | recall ≥ 0.95 and precision ≥ 0.95 on the kind intersection, kind agreement ≥ 0.95, range agreement ≥ 0.90, signature coverage ≥ 0.90 of callables, zero unexplained engine faults | recall ≥ 0.85, precision ≥ 0.90 | universal-ctags (B) or SCIP definitions (A) |
| **L2** syntax edges | call sites inside each definition, by callee name; inheritance where the grammar exposes it | `SymbolNode.Calls` | call-site recall ≥ 0.90 and precision ≥ 0.90 against SCIP references at name granularity | 0.75 / 0.85, or any tier-C-only measurement | SCIP (A); upstream `tags.scm` references (C) |
| **L3** imports and file-resolved edges | parsed imports; cross-file call edges resolved through them, labelled `file-resolved`, tier pre-scan | `file_imports`, typed_edges (file tier) | in-repo import resolution ≥ 0.95; edge precision ≥ 0.95, recall ≥ 0.50 of SCIP's cross-file edges | precision ≥ 0.90, recall ≥ 0.25 | SCIP (A) |
| **L4** typed edges | edges resolved by a type-aware producer: native (Go, CUE) or an imported SCIP index | typed_edges | definition mapping ≥ 0.95 of mappable SCIP definitions; typed-edge recall ≥ 0.95 at fathom granularity | ≥ 0.85 | SCIP (A) or a native type checker |

Tool formats use the same levels: L1 entities (jobs, targets, resources, rules, keys),
L2 references inside a file (`needs`, `depends_on`, `extends`, `Ref`), L3 references across
files or repositories (includes, module sources, image to Dockerfile, `service_ref`).
Structured formats must be exact: L1 ≥ 0.98 both ways, L2 ≥ 0.95, L3 precision ≥ 0.95 and
recall ≥ 0.80, against the tool's own evaluation (`make -pn`, `bazel query`,
`terraform graph`, `helm template`, the format's reference parser) as tier A. L4 does not
apply to tool formats.

**Oracle tiers.** A: semantic (SCIP, a native type checker, the tool's own evaluation).
B: an independent syntactic parser (universal-ctags, run as a separate process). C: an
independent query over the same grammar (upstream `tags.scm` through the tree-sitter CLI),
or an independent parser library. D: a grep proxy or a sample labelled by the worker who
wrote the extractor. **Excellent needs tier A or B; tier C caps a level at good; tier D
proves nothing beyond "tolerated"** — the author labelling their own truth is self-grading
(O56's rule, applied to measurement).

A level is met only when every lower level is met. Performance and tenancy are part of
every level: throughput ≥ 1 MB/s per core on the calibration subjects, no file slower
than 2 s, module memory inside the 64 MiB cap, engine faults ≤ 0.1% of files and every one
named.

## 3. The decisions (ADR-048 carries the record)

1. **Levels as above**, measured per language on at least two pinned calibration subjects,
   with the reading committed under `docs/bench/readings/lang/<id>.json`. The tracker may
   not claim a level its reading does not prove, and a test holds it to that.
2. **Two engines, dialects as data.** Syntax languages run on the one tags-query engine
   (`TSExtractor`, grown: predicates, signatures, ranges, name-captured references,
   imports, injections). Structured tool formats run on one document-path engine
   (YAML/JSON/TOML/XML decode plus declarative selectors). A dialect is a directory of
   data — a manifest, query files, fixtures and goldens — and adding one needs no Go.
   Per-language Go survives only as a closed set of module-resolution strategies (L3) and
   the native typed passes (Go, CUE). The bespoke CGO walkers and the regex extractors
   retire when their tags-query replacement measures at least as well on the corpus;
   then the CGO tree-sitter dependency leaves `go.mod` (F1).
3. **Grammars are embedded, compressed, and loaded lazily; the long tail and private
   dialects are checksum-pinned packs.** Every grammar blob is built from pinned upstream
   source by one recipe, stored deflate-compressed in the binary (about 4.9× smaller:
   the five current blobs go from 7.2 MB to 1.5 MB), decompressed and compiled only when a
   file of that language is first seen. Embedded compressed grammars are capped at 40 MiB;
   past the cap, the least-demanded dialects ship as packs. A dialect pack is the same
   directory plus a `.wasm` blob, installed at user scope from a file or directory and
   loaded only when its sha256 matches the binary's lock or the operator's user-scope
   trust list. The daemon never fetches anything from the network.
4. **Licensing and provenance.** Grammars, queries and fixtures come only from sources
   whose licence is on the allowlist (MIT, Apache-2.0, BSD-2/3-Clause, ISC, Unlicense,
   CC0, Zlib, MPL-2.0 for unmodified files); GPL, LGPL, AGPL, SSPL, PolyForm, BUSL, FSL and
   "no licence" are refused. Every borrowed file carries a header naming its source, commit
   and licence; the grammar lock records repo, tag, commit, tarball sha256, blob sha256,
   size and SPDX id, and a test refuses a blob or a query that the lock does not describe.
   GPL tools (universal-ctags) are used only as separate oracle processes on the
   measurement path, never linked, vendored or shipped. GitNexus (PolyForm Noncommercial)
   is not read for code or queries.
5. **No language raises the daemon's footprint budget.** Grammars compile lazily; at most
   six parser modules live at once process-wide (LRU, idle-closed after two minutes);
   compiled code is byte-accounted in the memo registry (I51) and dropped at shed; each
   module keeps RT-107's 64 MiB cap; the 2 MiB file cap, the binary sniff and a new
   minified/generated sniff gate every file; a language whose corpus run breaks the
   budget does not ship. A polyglot fixture test and an operator reading (LB-M3) hold it.
6. **One queue, most common first, tooling interleaved.** Items are ordered by a demand
   score — survey usage (Stack Overflow 2025, Octoverse 2025, TIOBE and RedMonk) times
   fleet demand (the unsupported-language census) times cross-repo value (formats that
   yield deployment and service edges weigh more) — divided by expected cost (grammar
   quality, oracle availability). Waves mix upgrades of languages we already extract,
   new languages and tool formats.
7. **The loop belongs to the harness.** Fathom serves the facts — the plan rows, the
   queue, the readings, the tracker — and never drives the work (vision 24's dispatch
   boundary). The loop runs one queue item per worker: Sonnet executes the recipe, Haiku
   re-measures and regenerates the tracker after engine changes, Opus writes foundation
   rows and triages escalations. Each worker reads the recipe and one queue entry, not the
   codebase.
8. **The decision port has no role in extraction.** Extraction is deterministic (P2: the
   same bytes give the same symbols); a typed fast decision cannot be. Ambiguous extensions
   (`.m`, `.v`, `.pl`, `.fs`, `.pp`, `.inc`) resolve by deterministic content sniff, and the
   ambiguous count is disclosed. The only place a decision port could help — choosing
   calibration subjects — is a one-off human-reviewed choice that does not justify one.

## 4. What we take from others, and what we do not

| Source (licence) | We take | We do not take |
|---|---|---|
| tree-sitter's own `queries/tags.scm` in upstream grammar repos (per-grammar, mostly MIT) | the capture convention (`@name`, `@definition.*`, `@reference.*`) verbatim, and each grammar's queries as the first draft of ours | predicates we cannot evaluate; doc-association directives |
| aider `aider/queries/**` (Apache-2.0, ~40 languages across two provenance directories) | query drafts, kept in per-source provenance; its split-by-origin layout | its repo-map ranking |
| nvim-treesitter (Apache-2.0) | `injections.scm` (embedded languages) and `locals.scm` shapes where tags are missing | highlight queries |
| github/stack-graphs (Apache-2.0, archived 2025) | the lesson: flat tags are near-universal, cross-file resolution is per-language and hard to sustain — our L3 strategies stay few and measured | the stack-graph DSL |
| Graphify (MIT/Apache-2.0) | the EXTRACTED vs INFERRED edge label — ours already exists as tiers (syntax / file-resolved / typed); every new edge carries one | its graph export |
| SCIP indexers (Apache-2.0/MIT) | typed edges by import (ADR-041 decision 5): scip-java (Java, Kotlin, Scala), scip-typescript, scip-python, rust-analyzer, scip-clang, scip-dotnet, scip-ruby, scip-php, scip-dart | running them inside the daemon; the archived scip-kotlin plugin |
| universal-ctags (GPL-2.0) | the tier-B symbol oracle, as a subprocess on the measurement path (the zoekt and sourcebot pattern) | linking, vendoring or shipping it |
| codebase-memory-mcp, gortex, CodeGraph (MIT/Apache-2.0 as listed) | a reminder that breadth claims are cheap; gortex's CGO build is the distribution we are leaving | their numbers, which are unmeasured |
| GitNexus (PolyForm Noncommercial) | nothing — not opened for code or queries | everything |

## 5. The respectable number

The program's milestones are counted from readings, never from registrations:

| Milestone | Languages | Tool formats |
|---|---|---|
| M1 (after waves 1–2) | 20 at L1 excellent, 12 at L2, 8 at L3, 6 at L4 | 15 at target |
| M2 (after waves 3–5) | 45 at L1 (35 excellent), 25 at L2, 14 at L3, 10 at L4 | 30 at target |
| M3 (the backlog drained) | 60+ at L1, 35 at L2, 20 at L3, 12 at L4 | 40+ at target |

Excellent L1 on sixty languages, measured against ctags or SCIP on real repositories, is
a claim no surveyed tool publishes. Depth where it pays — L3 and L4 for the fifteen
languages that carry most code — is what makes `find_callers`, `blast_radius` and
`localize` answer outside Go.

## 6. What we refuse

- Counting a grammar as support. A registered dialect without a reading is "tolerated",
  printed as such.
- Language servers in the daemon (scope's "no IDE language services" non-goal) and
  per-language type checkers of our own (ADR-041 rejected them; SCIP import stands).
- Emscripten side-module grammars (ADR-013/ADR-021 §4: the monolithic wasm32-wasi build
  stays the only shape).
- Network access from the daemon, auto-installing indexers, or fetching packs implicitly.
- A self-labelled truth set as proof of excellence.

## 7. Risks and open questions

- **Grammar quality varies.** Some languages (VB.NET, ABAP, Delphi, MATLAB, COBOL) have
  weak or unmaintained grammars; their queue items target L1 and may end in a disposition,
  which is an honest outcome, not a failure of the loop.
- **Oracle gaps.** ctags covers most mainstream languages but not all; a language with
  neither ctags nor SCIP support caps at good. The reading says so.
- **Kind vocabulary drift.** New per-language `SourceKind` strings are data; a canonical
  kind (a closed list of 45: 21 for languages, 24 for tool formats) keeps cross-language tools coherent. Changing an
  existing language's kind strings is refused without an `ExtractorEpoch` bump and a
  CHANGELOG line.
- **Binary size.** The 40 MiB compressed cap is the trigger for the pack path; the fetch
  verb waits for a public release host (F2).
- **Q (new):** should L3 strategies ever be data (a resolution DSL) rather than a closed
  Go enum? Not until a fourth strategy would be a copy of an existing one with parameters.

## 8. How this fits what is already decided

- **F1** (CGO-free binary) is promoted: its "Promote when" (per-language parity on the
  corpus) becomes the retirement protocol in the design (§11), and the owner's directive
  answers Q6 ("is the nine-language migration wanted") with yes.
- **ADR-041 decision 5** keeps its order for typed edges; this program adds the levels,
  the oracle rule and the per-language queue after that order.
- **ADR-024**'s dialect row is refined, not reversed: a dialect is still authored by fathom
  and trusted by goldens and calibration, still not a pack in the taxonomy sense; its
  representation becomes data, and a pinned dialect pack may be installed at user scope.
- **ADR-047** is a constraint, not a neighbour: every mechanism above is shaped by the
  footprint budget, the memo registry and RT-107.
- **Vision 24**'s dispatch boundary holds: the queue is a fact fathom serves; the loop is
  the harness's.
