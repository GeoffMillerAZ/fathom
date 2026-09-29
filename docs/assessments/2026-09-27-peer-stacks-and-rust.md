# Peer stacks, and whether fathom should be Rust

Created: 2026-09-27 (summarized into the repository 2026-09-28).
Status: a reading of public repositories, package manifests and issue trackers on 2026-09-27; not
re-run. The full survey is `data/2026-09-27-peer-stacks.json` (30 entries, each with its sources).
Decisions it drove: ADR-049 decision 11 (no Rust rewrite), `docs/vision/38-proof-layer-roadmap.md`
section 8, future F29, plan row RH-M5, and the extractor list of `docs/design/system-graph.md`.

## Verdict

Fathom's stack is among the leanest and most predictable in the field, and Go is not the bottleneck.
Most peers are TypeScript or Python processes with a separate database (Chroma, Kuzu, Neo4j, Redis,
Postgres, Qdrant) or one language server per language beside them; fathom is one static Go binary,
pure-Go SQLite, tree-sitter grammars under wazero, no Node, Python, Docker or JVM runtime, and one
loopback daemon under an OS-measured memory budget (ADR-047). **Do not rewrite in Rust.** Rust is a
real and growing choice among peers, but no source published a Rust-versus-Go benchmark; the
advantages cited are architectural and qualitative. The cheap experiment, if a trigger ever fires, is
a Rust-compiled wasm grammar or query module under wazero (F29), never a rewrite.

This is a reading of peers' documentation and code (*estimate*), not a head-to-head measurement:
RH-M5 measures peak memory, cold index time and index size for fathom and at least two peers on one
corpus, and until it reads, "leanest" is *unmeasured*.

## How to read the grades

Facts read from a repository, manifest or issue tracker on 2026-09-27 are *measured on that date*.
Anything a README asserts about itself (speed, savings, language counts) is *claimed by a vendor*.
Fields the survey could not confirm say "unverified"; the survey hit GitHub API rate limits on a few
targets, and qmd, OpenCode and Cline were only partly verified. The JSON carries every field and every
source URL.

## The survey, compressed

Thirty entries: 28 peers, one entry (qmd) that could not be identified as a standalone repository
(it surfaced only as a companion CLI mentioned by another project) and one duplicate name.

| Peer | Kind | Language and runtime | Storage | Parsing or method | Embeddings | Licence | Cross-service | Resource complaint found |
|---|---|---|---|---|---|---|---|---|
| Graphify | knowledge-graph skill | Python, a Claude Code skill (no daemon) | none: in-memory graph, graph.json, a SHA256 file cache, optional Neo4j export | tree-sitter (13 languages) for code; a model for docs, PDFs, images | graph embeddings only, no vector store | Apache-2.0 at origin, pyproject says MIT (inconsistent) | no | none found |
| CodeGraph | code-graph MCP | TypeScript plus a native Rust kernel (napi-rs), bundled runtime | SQLite (better-sqlite3) with FTS5 | tree-sitter: Rust kernel for 20 languages, WASM fallback, 34 in the README grid | none | MIT | yes: route nodes and handler edges across 17 web frameworks | #857 memory leak (macOS) |
| GitNexus | code-graph MCP | TypeScript, Node or browser WASM | LadybugDB embedded graph database | tree-sitter native or WASM; several grammars vendored | opt-in local embeddings, off by default | PolyForm Noncommercial 1.0.0 (not OSI open source) | yes: route_map, api_impact, Spring Actuator ingestion, AsyncAPI modeling | #706 stack overflow on a ~10K-file Go repo; #1143 crash at 60 percent; hosted indexing is memory-bound |
| codebase-memory-mcp | code-graph MCP | pure C, one static binary | SQLite in memory, LZ4 | tree-sitter with 162 vendored grammars; a "hybrid LSP" layer for about 10 to 12 languages | none | MIT | yes: cross-service HTTP linking, IaC nodes (Dockerfile, Kubernetes, Kustomize) | unverified (rate limit) |
| Serena | LSP MCP toolkit | Python | none (memories are markdown files) | no parser: wraps real language servers for 40 or more languages | none (feature request open) | MIT per component | no | #1937 partial results while the graph loads |
| claude-mem | agent memory | TypeScript, Node or Bun | SQLite with FTS5 plus Chroma | none: compresses session transcripts | yes, via Chroma | Apache-2.0 | no | unverified |
| Claude OS | agent memory | Python 3.11 to 3.12 | SQLite vector search | none | Ollama or OpenAI | MIT per README, no licence file | no | self-disclosed Ollama load |
| Context7 | library-docs MCP | TypeScript client, closed cloud backend | undisclosed | n/a (third-party docs) | yes (implied) | MIT (client only) | no | n/a |
| Aider (repo map) | CLI agent | Python | diskcache (SQLite) tag cache | tree-sitter tags, PageRank over the tag graph | no | Apache-2.0 | no | #3899 GitPython mmap on large repos |
| OpenCode | CLI agent | TypeScript on Bun | SQLite (Drizzle) for session state | unverified, likely per-language LSP | unverified | MIT | unverified | unverified |
| Cline | IDE agent | TypeScript | unverified | reportedly tree-sitter for a definitions tool (unverified now) | not found | Apache-2.0 | unverified | unverified |
| Continue | IDE agent | TypeScript | SQLite plus LanceDB | tree-sitter (WASM) chunking | yes, local ONNX or cloud | Apache-2.0 | unverified | #1318 CPU and memory growth until crash |
| Spec Kit | spec-driven | Python | none (files) | none | no | MIT | no | n/a |
| OpenSpec | spec-driven | TypeScript | none (files) | none | no | MIT | no | n/a |
| Task Master | task management | JavaScript or TypeScript | JSON or markdown files; optional Supabase | none | not found | MIT | no | n/a |
| Beads | issue tracker and memory | Go binary | Dolt (versioned SQL) | none | no | MIT | no | n/a |
| Mem0 | agent memory | Python | Qdrant by default, many optional stores | n/a | yes (OpenAI by default) | Apache-2.0 | no | none (feature requests only) |
| Graphiti | temporal memory graph | Python | Neo4j by default (FalkorDB, Neptune) | n/a | yes | Apache-2.0 | no | #992 OOM in community building |
| Zep | managed memory service | managed service | proprietary graph service | n/a | yes | proprietary (the Graphiti library is Apache-2.0) | no | n/a (opaque) |
| Letta | stateful agent platform | TypeScript (the V1 Python server is archived) | unverified | n/a | unverified | Apache-2.0 | no | #1813 100 GB or more spike; #259 leak |
| PR-Agent | PR review | Python | none | sends diffs to a model | none | MIT, now a community fork (not the Qodo free tier) | no | n/a |
| Sourcegraph (Cody, Amp, SCIP) | code search platform | Go core; Cody TypeScript | Postgres, Zoekt trigram index, SCIP indexes | SCIP indexers, compiler-based for typed languages | Cody historically (unverified) | unverified; public repos are "public snapshot" mirrors | cross-repo navigation is the core; no HTTP-route feature confirmed | not searched |
| Augment Context Engine | hosted context engine | proprietary | undisclosed | "semantic indexing" (undisclosed) | yes (implied) | proprietary | claimed, no detail | n/a |
| ast-grep | structural search | Rust | none | tree-sitter | no | MIT | no | #2741 memory spikes |
| universal-ctags | symbol indexer | C | flat tags file | hand-written per-language parsers | no | GPL-2.0 | no | none |
| Semgrep | static analysis | OCaml core, Python CLI | none | per-language parsers into a generic AST | no | LGPL-2.1 | no | #5172, #10044 |
| CodeQL | security analysis | closed CLI (OCaml, C++); queries in a repo | compiled relational database | compiler-integrated extraction | no | MIT for queries, engine proprietary | no | #16780, #19374, #14138 |
| GitHub stack-graphs | name-resolution library | Rust | none (the consumer's) | tree-sitter into stack-graph rules | no | MIT and Apache-2.0 dual | no | none found |

## Findings

**The shape of the field.** Mostly TypeScript or Python; memory tools reach for a vector database or a
graph server; several run one language server per language (Serena, OpenCode by inference). A minority
are native: codebase-memory-mcp (C, one binary), CodeGraph's Rust kernel, ast-grep and stack-graphs
(Rust), Beads (Go). None is pure Go with an embedded parser engine as fathom is.

**Resource complaints are real** (issues read 2026-09-27): CodeGraph #857 (memory leak on macOS),
GitNexus #706 (stack overflow on a roughly 10,000-file Go repository) and #1143 (crash at 60 percent),
Graphiti #992 (out-of-memory in community building), Letta #1813 (a 100 GB or more spike), Continue
#1318 (CPU and memory growth until crash), Aider #3899 (GitPython memory mapping on large
repositories), Semgrep #5172, CodeQL memory issues, ast-grep #2741; Claude OS's own README discloses
high Ollama load. This supports the landscape trend "resource use is a real complaint" and makes the
memory budget (ADR-047) a selling point, provided RH-M5 confirms it.

**Cross-service capability among peers** (input to the system graph's extractor list, ADR-050
decisions 3 and 4): CodeGraph routes across 17 web frameworks (Django, Flask, FastAPI, Express,
NestJS, Laravel, Drupal, Rails, Spring, Play, the Go routers, Axum, actix, Rocket, ASP.NET, Vapor,
Astro) plus navigation edges and native-to-JS bridge resolution; GitNexus has `route_map` and
`api_impact`, Spring Actuator ingestion and AsyncAPI channel modeling; codebase-memory-mcp links HTTP
across services and models IaC (Dockerfile, Kubernetes, Kustomize); Sourcegraph does cross-repository
search and navigation. Serena, Graphify, claude-mem and Mem0 have none.

**Licence screen** (adoption and partnership risk): GitNexus is PolyForm Noncommercial, source-available
and not OSI open source (already refused as a grammar or query input by ADR-048 decision 6); Graphify
shows Apache-2.0 at its origin and MIT in its manifest; Claude OS has no licence file; Sourcegraph moved
to public-snapshot mirrors; PR-Agent is now a community fork; the Zep server is proprietary; the CodeQL
engine is closed. Fathom's own licence is ADR-052.

## Rust in peers, and the verdict

Confirmed in the survey: ast-grep (a core crate and an LSP crate), CodeGraph's parsing kernel
(`codegraph-kernel`, shipped to Node through napi-rs with an automatic WASM and JavaScript fallback),
and GitHub's stack-graphs (which powers parts of github.com's own precise navigation). The advantages
those projects state are consistent and qualitative: native speed with no garbage-collection pauses in
parse-heavy loops (CodeGraph's stated aim is one JavaScript-to-Rust crossing per file), memory safety
without a runtime so a Rust core can embed in a Node host or any host process, and link-time
optimization as a common performance lever. None of the sources published a Rust-versus-Go
microbenchmark (*unmeasured*). Fathom's pure Go plus tree-sitter-on-wazero choice sidesteps the native
toolchain and distribution cost that CodeGraph (a Rust addon with a WASM fallback) and GitNexus (native
addons and a C++ toolchain for some grammars) have to solve.

**Verdict (ADR-049 decision 11, vision 38 section 8): the stack stays; no Rust rewrite, and no Rust
extraction worker.** Parsing is wasm and the store is SQLite, so Go is not the bottleneck, and a Rust
worker would add a toolchain, a process or FFI seam and a second language on a bus factor of one.
**Trigger** to run the cheap experiment (F29): the typed-pass or extraction children cannot meet the
300 MiB ingest target after the RT waves, or garbage-collection pauses show in hook p99 at fleet
scale (RT-M7's GC CPU reading above 25 percent below the refuse rung). Even then the experiment is a
wasm module under wazero: no FFI, the same sandbox, the same module cap.

## What it drove

| Decision | Where |
|---|---|
| The stack stays; Rust is a trigger-gated wasm experiment | ADR-049 decision 11; vision 38 section 8; F29 |
| A shared-corpus reading of peers' memory, cold index time and index size | RH-M5 (`docs/plans/reach.md`) |
| The extractor list for cross-service links (frameworks above) | `docs/design/system-graph.md` section 5; ADR-050 |
| GitNexus's licence excludes it as an input | ADR-048 decision 6 |

## What this does not claim

That fathom uses less memory than any named peer (RH-M5 has not read); that any peer's complaint
describes its current release; that Rust would or would not be faster (no benchmark exists in the
sources); that the licence notes are legal advice; that the survey is exhaustive or current after
2026-09-27.

## Sources (public, read 2026-09-27)

Each peer's repository, manifest (`package.json`, `pyproject.toml`, `Cargo.toml`, `go.mod`) and issue
tracker, listed per entry in `data/2026-09-27-peer-stacks.json`. Rust note sources: the ast-grep
`Cargo.toml`, the CodeGraph `codegraph-kernel/Cargo.toml` and README section on its Rust kernel, and
the github/stack-graphs repository.
