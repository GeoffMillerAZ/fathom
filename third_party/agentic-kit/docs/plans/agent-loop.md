# Agent loop — verified capabilities (dogfood plan)

Contract-bearing checklist gated by `fathom verify` at pre-push (convention:
fathom's `docs/plans/README.md`). `[x]` is a claim; claims are checked against
the code index.

- [x] Harness construction from config and deps {symbol: New, package: kits/agent-harness}
- [x] Single-turn execution without tool registry {symbol: RunOnce}
- [x] Streaming event channel for multi-turn runs {symbol: RunStream}
- [x] Evidence-preserving compactor construction {symbol: New, package: foundations/agent/evidence-compactor}
