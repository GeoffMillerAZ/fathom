# Eval fixture — a gated plan checklist

This document is a labeled eval corpus for the spec-bindings compiler
(docs/design/semantic-layers.md §4.6). Task lines below have known correct
scopes, recorded in labels.json.

## Tasks

- [ ] Query results must be capped and sorted before serving {id: SB-1, symbol: Search, package: internal/query}
- [ ] MCP tool handlers return typed rpcError, never bare fmt.Errorf {id: SB-2, symbol: dispatch, package: internal/mcp}
- [ ] Store anchors stay repo-relative across remap {id: SB-3, symbol: Remap, package: internal/store}
- [x] Historical note: the old sqlite backend is fully retired {id: SB-4, package: docs}
- [-] Deferred: vector search index over claim text {id: SB-5, disposition: deferred, reason: "no consumer yet"}
- [ ] Untracked idea, no annotation at all: maybe add a cache someday
