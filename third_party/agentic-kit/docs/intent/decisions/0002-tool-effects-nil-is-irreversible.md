# 0002: Tool effects are declared; an undeclared effect is irreversible under Strict

**Status:** accepted
**Type:** architecture
**Date:** 2026-09-02

Extension 2026-09-13: [0008](0008-bound-approval-transactions.md) adds stored,
version-bound approvals inside `kits/hitl`; the synchronous effect gate and its
defaults are unchanged. The rejection below concerns a separate bearer-token system.

## Context

The year's worst agent incidents were failures of authority, not detection:
an agent with an over-privileged token deleted a production database and its
backups in seconds through a legacy endpoint with no delayed delete; a
prompt injection made an editor write its own MCP configuration; permissive
coding agents take out-of-scope actions on benign tasks at measurable rates.
Behavioral guardrails were bypassed by adaptive attacks in every published
evaluation. The field's answer is structural: know which actions are
irreversible and gate those, structurally, before the call.

The kit's `tool-registry` describes a tool's schema and nothing about its
effect. The harness screens tool *results* (practices §5) but dispatches any
tool the policy allows. MCP now carries tool annotations
(`readOnlyHint`, `destructiveHint`, `idempotentHint`, `openWorldHint`) and
says a client must treat them as untrusted unless the server is trusted.

## Decision

- `tool-registry` gains `Effect{ReadOnlyHint, DestructiveHint, IdempotentHint,
  OpenWorldHint}` — the MCP tool-annotation vocabulary verbatim, so the kit
  invents no second taxonomy. A tool MAY declare one.
- Under `EffectPolicy` Strict in `agent-harness`, a tool with no declared
  effect is treated as destructive. The zero value is the strictest value,
  mirroring the trifecta doctrine (an unknown is a hazard, not a pass).
- A destructive dispatch is routed through the existing `PreDispatch` seam to
  a `kits/hitl` approval; a denied or absent approval returns `is_error` to
  the model and records a `gate_denied` event. No new token type: the kit
  already has a typed human-in-the-loop channel and composes it. `Defs()` is
  unchanged, so a model sees the same tool list; only dispatch changes.
- `mcp-server` emits annotations from a declared effect. `mcp-client` maps
  annotations in as untrusted unless the caller marks the server trusted.
- This decision is orthogonal to the accepted seam that the trifecta
  mechanism is not re-checked per call (`threat-model.md`). The effect gate
  is per call by design and says so.

## Rationale

A declared effect is data with a contract (belief 1), a fail-closed default
(belief 4), and an honest boundary (belief 6): the gate stops the harness
from *dispatching* an undeclared irreversible tool without a human token; it
does not make a tool reversible, it does not sandbox anything, and it cannot
see effects a tool lies about.

## Alternatives rejected

- **Trust the model's judgement about reversibility** — the incidents are
  exactly that judgement failing.
- **A per-tool allowlist in the policy layer** — already exists
  (`tool-execution-policy`) and says nothing about *why* a tool is dangerous;
  the effect is what an approval UI, a log, and a reviewer need.
- **A kit-specific effect taxonomy or a single-use approval token** — MCP's
  annotation names are the interchange, and `kits/hitl` is the approval
  channel; inventing either is the wheel.
- **Make the default permissive** — the PocketOS shape: every tool is safe
  until the one that is not.

## Consequences

- `practices.md` gains the irreversible-actions entry with STATUS shipped.
- The governed-investigation scenario dispatches one irreversible tool
  through the gate so the kit has a consumer.
- Consumers on `experimental` harness options see no behavior change until
  they opt into Strict.

## Revisit when

A consumer reports a legitimate tool that cannot declare an effect; MCP's
annotation vocabulary changes; or the trifecta decision is reopened.
