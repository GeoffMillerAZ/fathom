# Architecture Decision Records (eval fixture)

This document is a labeled eval corpus for the adr-rules compiler
(docs/design/semantic-layers.md §4.6). Decisions below have known correct
scopes, recorded in labels.json; two are deliberate scope traps.

## ADR-eval-001 — Stores are born portable

**Decision.** Every store under internal/store/ must anchor every path it
persists repo-relative — never an absolute filesystem path — so a store
file can be copied to a different machine or checked out at a different
root and still resolve.

**Consequences.** internal/query/ and internal/mcp/ only READ paths the
store already resolved; they are not bound by this decision — anchor
discipline is the store layer's responsibility alone.

## ADR-eval-002 — Query answers are derived, never driven

**Decision.** internal/query/ must never persist mutable state across
calls: every query recomputes its answer fresh from the store's current
content on each call, with no cached result carried over from the last one.

## ADR-eval-003 — MCP tool errors are typed

**Decision.** Every MCP tool handler under internal/mcp/ must return a
typed rpcError on failure — a bare fmt.Errorf response is a hidden
protocol violation the client cannot branch on.

## Rejected alternative (not a decision)

An earlier draft considered giving cmd/fathom/ its own error type separate
from internal/mcp/'s rpcError. This was rejected as needless duplication —
nothing here binds cmd/fathom/ to anything; this section only records why
the alternative was NOT chosen.
