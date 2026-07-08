# Engineering standards (eval fixture)

This document is a labeled eval corpus for the obligations compiler
(ADR-016 §4.6). Rules below have known correct scopes, recorded in
labels.json; two are deliberate scope traps.

## Determinism

In the query engine under internal/query/, never call time.Now() inside
query or ranking logic — inject a clock function instead.

All command output must be sorted deterministically before it is returned,
in every package.

## Scope trap: passing mention

The query engine's ranking (internal/query/) must cap result lists at
their documented limits. Note that internal/mcp/ merely consumes these
results over JSON-RPC and is NOT governed by this rule — the cap is the
engine's responsibility alone.

## HTTP serving

HTTP handlers in cmd/fathom/servehttp.go must set request timeouts on
every outbound client they construct.

## Behavioral rule (globs express this poorly)

Every MCP tool registered under internal/mcp/ must return bounded output:
hard caps on list lengths, omitempty on optional fields.
