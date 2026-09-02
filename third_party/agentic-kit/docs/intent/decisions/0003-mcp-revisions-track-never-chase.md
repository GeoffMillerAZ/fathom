# 0003: MCP protocol revisions are tracked with a clock, never chased

**Status:** accepted
**Type:** dependency
**Date:** 2026-09-02

## Context

The 2026-07-28 MCP revision is the largest since launch: a stateless core
without the initialize handshake, a mandatory discovery call, a new
result envelope, tasks as an extension, deprecation of several primitives
with a 12-month removal floor, and a different HTTP posture. The kit's
`mcp-server` and `mcp-client` implement 2025-11-25. The official SDKs are
first-tier implementations the kit cannot depend on (I1: stdlib only).
Chasing every revision inside a stdlib-only kit is a treadmill; ignoring
revisions is how a consumer discovers a removed feature in production.

## Decision

- `mcp-server` and `mcp-client` export `ProtocolRevision` and a
  `Deprecations` table (feature, deprecated-in, removable-after,
  still-implemented).
- A test with an injected clock fails the moment a still-implemented
  feature passes its removal floor. The failure is the reminder; it is not
  suppressed by a build tag.
- The stateless move (no initialize, `_meta` per request, the new result
  envelope) is a deferred decision with its own floor, not a build. It is
  taken when a named consumer's client stops accepting 2025-11-25, or the
  floor test fires, whichever is first.
- Tool annotations (decision 0002) are adopted now because they are additive
  in both revisions.

## Rationale

The clock turns an invisible drift into a failing test on a known date. The
deferral keeps the kit honest about what it implements (belief 6) without
spending a wave on a revision no consumer has asked for.

## Alternatives rejected

- **Adopt 2026-07-28 now** — no consumer pulls; the kit's own server serves
  local clients that still speak 2025-11-25.
- **Vendor the official SDK** — I1.
- **Ignore the revision** — the removal floor makes ignoring it a dated
  liability.

## Consequences

- `practices.md` gains the protocol-revision-drift entry.
- `futures.md` F10 (sessions, prompts, resources) is annotated: the target
  moved; do not build against 2025-11-25's session model.

## Revisit when

The floor test fires; a consumer's client rejects 2025-11-25; or MCP
publishes another revision that deprecates something the kit implements.
