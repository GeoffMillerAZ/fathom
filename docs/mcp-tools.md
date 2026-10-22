# MCP Tools Reference

Fathom exposes a JSON-RPC 2.0 MCP server (stdio or HTTP). This page documents every
registered tool, prompt, and resource.

## The surfaces at a glance

| Surface | Tools | Serves |
|---|---|---|
| `/rpc/agent` (the default `fathom init` prints) | 8 | `blast_radius`, `find_symbol`, `obligations`, `read_source`, `review_diff`, `search`, `symbol_context`, `verify_plan` |
| `/rpc/extended` (alias `/rpc/companion`) | 21 | the eight, plus `find_callers`, `findings_near`, `ingest`, `list_services`, `localize`, `outline`, `pack_context`, `query_symbols`, `references`, `repo_map`, `staleness_report`, `status`, `test_impact` |
| `/rpc` (the full registry) | 25 | the 21, plus `compile_layer`, `cross_repo_callers`, `remove_service`, `rename_service` |

A surface is an allowlist for ergonomics, not a security boundary: a call to a tool off
the registered surface still runs and is flagged. The lists come from
`internal/mcp/surface.go`; `TestMcpToolsDocMatchesRegistry` keeps the tables below in step
with the registry.

`review_diff` caps its complete serialized MCP ToolResult at 16,000 bytes, including
both payload copies and metadata. Oversized reviews retain all eight dimension
statuses and counts, the primary receipt band and deterministic detail prefixes, with
named omissions and logged elision counts; small results are unchanged. See
[change review](use-cases/change-review.md) for the disclosure contract.

See [usage.md](usage.md) for how to connect a client and select a surface.
See [architecture.md](architecture.md) for how the tool groups map to the three-layer design.

---

## Where each part lives

This page is the index; the reference is split per group so parallel work adds
files instead of editing one. `TestMcpToolsDocMatchesRegistry` reads this page and every
file below, in this order.

| File | Holds |
|---|---|
| [surfaces](mcp-tools/surfaces.md) | Tool groups, profiles and surfaces: the profile table, the path-scoped HTTP surfaces and the plain HTTP endpoints |
| [protocol](mcp-tools/protocol.md) | The served protocol contract |
| [cost-classes](mcp-tools/cost-classes.md) | The cost-class table (mirrors the registry) |
| [shared](mcp-tools/shared.md) | Shared tools: the group heading and intro |
| [shared-lookup](mcp-tools/shared-lookup.md) | Shared tools: lookup (find_symbol, status, staleness_report) |
| [shared-obligations-a](mcp-tools/shared-obligations-a.md) | Shared tools: obligations, part a (and the `Finding` shape) |
| [shared-obligations-b](mcp-tools/shared-obligations-b.md) | Shared tools: obligations, part b |
| [shared-obligations-c](mcp-tools/shared-obligations-c.md) | Shared tools: obligations, part c |
| [shared-obligations-d](mcp-tools/shared-obligations-d.md) | Shared tools: obligations, part d |
| [shared-obligations-e](mcp-tools/shared-obligations-e.md) | Shared tools: obligations, part e |
| [shared-obligations-f](mcp-tools/shared-obligations-f.md) | Shared tools: obligations, part f |
| [shared-registry](mcp-tools/shared-registry.md) | Shared tools: registry (ingest, remove_service, rename_service) |
| [shared-context](mcp-tools/shared-context.md) | Shared tools: context (repo_map, pack_context) |
| [coding](mcp-tools/coding.md) | Coding tools |
| [investigation](mcp-tools/investigation.md) | Investigation tools |
| [platform](mcp-tools/platform.md) | Platform tools |
| [module-host](mcp-tools/module-host.md) | Module host (no MCP tools) |
| [symbolnode](mcp-tools/symbolnode.md) | The SymbolNode shape |
| [prompts](mcp-tools/prompts.md) | Prompts |
| [resources](mcp-tools/resources.md) | Resources |

## The result envelope

A tool result is `content` (one text block holding the JSON result) and, for an object-shaped result, `structuredContent` (the same object). Beside them the daemon may add `suppressed`, `refresh` and `budget`, which a client does not show the model. Fields are only ever added.

When a read was answered from an index that may be behind the working tree (a refresh deferred under load, or queued and not yet run), the result object itself carries `freshness`: service id to a one-line disclosure, for example `deferred (shed) -- served from the index as it stands; freshness not checked`. It is in the text block and in `structuredContent`, so the agent reads it with the answer. The envelope's `refresh` key still carries the same map. A result with no refresh note has no `freshness` member. A tool that already returns its own `freshness` member keeps it, and the envelope adds nothing.

`find_symbol`, `symbol_context`, `obligations` and `review_diff` check cited files against the index's existing content fingerprints. A citation whose bytes differ carries `citation_status: {state: "stale", reason: ...}`; an index fingerprint that changes during the read produces `refreshing`, and unavailable source identity produces `unknown` (or `refreshing` while refresh is pending). These typed states qualify the indexed location, even when Git HEAD has not changed; a byte-identical file stays clean during a service refresh. Obligation summaries and edit/pre-edit/prompt hooks qualify non-clean citations with a short tag. A `symbol_context` snippet shares its definition's status; stale or refreshing definition snippets are withheld. Clean citations retain their existing JSON bytes. Each request streams and hashes each distinct cited file once, keeps only cited line windows, and reads refresh state at call start and end. Checks use the same containment, suppression and size limits as source reads; live-tree spec-reference locations and raw fallback rules are current by construction and are never marked or hashed. Legacy fingerprints without a recorded extractor epoch are unknown on mismatch, rather than asserting that bytes changed.

`read_source` reads the working tree by default and returns `source_identity: {state: "working_tree", content_sha256: ...}` for the exact bytes it sliced. With `ref`, it returns `source_identity: {state: "commit", commit: ..., content_sha256: ...}` alongside the existing `ref` and resolved `commit`. A working-tree read adds `matches_index` only when its already-computed identity check proves a match or a content mismatch; it omits that field when the indexed identity is uncertain. Both digests share one pass over the content.

Verified clones and worktrees resolve to their base service by shared Git common directory, or matching normalized origin and root commits, with ambiguous matches unresolved. `search` accepts a clone `root`; a clone subdirectory searches only that subdirectory, attaching index receipts only to unchanged files. `read_source` accepts an absolute clone file and reads that checkout. `find_symbol`, `symbol_context`, and `blast_radius` accept an optional absolute `path` for clone identity. Clone answers carry `freshness_note: "clone at <head>, index at <indexed commit>"`; new or modified clone files and their indexed symbols carry `status: "unknown"` with a reason such as `changed in clone`. Clone-mode unknown answers replace `summary` and omit `not_found`, `not_indexed`, and `hint`, so base-index absence is never presented as clone absence.

Linked worktrees require Git's own registration and backlink. This is an integrity check, not an access control. Against a local user who can write files, the directory boundary is the directories they can place a `.git` in other than home and its ancestors, subject to the indexed-root refusals and tracked-file checks. Against MCP clients that cannot write files, it is the checkouts the user created. Clone roots cannot be the filesystem root, home or an ancestor of home, or contain an indexed service root; these refusals and direct indexed matches compare file identity, including case aliases. Clone mode is refused with a reason if the home directory cannot be determined. Clone reads and searches serve only tracked paths listed by `git ls-files --cached`, with a literal pathspec restricting enumeration to the service subtree. An ordinary untracked clone file returns `status: "unknown"` without source bytes; ignored files retain their refusals.

Resolver infrastructure errors preserve the original handler. Raw-text search requires exactly one of `service` or `root` (`search: give exactly one of service or root`). A verified clone `root` combined with a different indexed `service` returns `path resolves to a different indexed service`. Absolute `changed_files` paths are canonicalized before checkout-relative validation.

---

## Secrets: what a read tool serves (SR-76)

No secret value is stored (LB-F367, ADR-065): a dialect selects no secret-typed field, and the
sentinel test finds the planted value in no row of any store table, full-text tables included. Every
tool that answers from the store therefore answers without one. Two answers also read repository
bytes, and withhold them from a file whose values may be secrets: one holding an entity of a kind a
dialect declares sensitive (every Alertmanager and Grafana provisioning entity, embedded in a
Kubernetes object or not), or a core Kubernetes Secret (`Secret/<name>`, apiVersion `v1`).

- `symbol_context` serves no `snippet` for a definition in such a file, its own kind sensitive or not
  (the Secret or ConfigMap that embeds the configuration too); `snippet_withheld` says why. While the
  service's symbol index loads it serves none either: the file cannot be checked yet.
- `references` serves an occurrence in such a file with an empty `text`; `text_withheld` says why.
  Its location stays.

Two readers serve repository bytes by design, and are the declared exemptions: `read_source` (the
lines of the file the caller names, as on disk or, with `ref`, as committed; `.env`-pattern files refused) and `search`'s literal
grep mode (no `kind`: the hit lines and their `context` lines, as on disk). A secret committed to the
repository is served by both as committed; no file is filtered. `search` with `kind: "docs"` answers
from the doc index (stored text) and is not exempt. `TestSecretSurface_NoReadToolServesASecret`
asks every tool on `/rpc/extended` about the secret-bearing files and entities and finds the sentinel
(in the clear or base64-encoded) in no answer; a tool added to that surface without an entry fails
`TestSecretSurface_EveryToolHasAnEntry`.

## Cold and unresolved answers (O1446, O1447, O1451)

Every ingest and daemon start drops a service's symbol index and call graph, and the next call that
needs them rebuilds them: seconds on a small service, a minute on a large one.

- `symbol_context` on a cold call graph that its deadline cannot hold answers within the deadline:
  `def` (with its signature) and `index_commit` from the symbol index, `partial` starting
  `call graph not built (cold)`, and `callers`, `callees` and `related_tests` as the string
  `not computed (cold graph)`, never an empty list. The build keeps running, so a later call answers
  the full neighbourhood. A service with no build finished since the server started has an unknown
  build time, taken as longer than the deadline, and its `partial` says so.
- `symbol_context` resolves the name before it waits on a cold call graph, against the resident symbol
  index or the store lookup `find_symbol` uses: a name with more than one definition answers its
  candidates and a name with none answers `not_found` at once. Only a unique name waits for the graph.
  While the symbol index is still loading, `not_found` carries no nearest names and its `hint` says so.
- `search` with a `service` never waits for the symbol index: while it loads, the hits carry no
  `sym`/`kind` and `receipts_omitted` says why. A resident index is read in place.
- `symbol_context` on a name nothing resolves answers `not_found: true` with the nearest names in
  `candidates` (`symbol_key`, `file`, `line`; case-insensitive, then substring, then edit distance)
  and a `hint`, and no `def`, `callers`, `callees` or `related_tests`. A repository-relative key
  (`dir/file.Name`, a candidate key without its `service::` prefix) resolves within the call's service.

`search`'s `path` is a directory, or a glob under one. The leading literal directory is the search root, so `.gitignore` and `.fathomignore` still apply under it; the rest of a glob filters the returned paths.

## Local landing evidence (CLI)

`fathom gate receipt write --log FILE [--command validate]` writes clean-tree gate
receipts and their logs; `fathom gate receipt check [--tree HEAD] [--command validate]`
checks exact identity and log integrity (exit 0 accepted, 1 refused).
`fathom gate record-ids [--cross-plan] [--config FILE]` checks authored record ids.
These are local CLI operations; no MCP tool, transport or store table is added.
See [configuration and the default-off pre-push snippet](configuration.md#gate-receipts-gate)
and [the use-case guide](use-cases/spec-verification.md#local-gate-evidence-and-record-ids).
