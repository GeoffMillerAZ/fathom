# Foundations — verified capabilities (dogfood plan)

Contract-bearing checklist gated by `fathom verify` at pre-push (convention:
fathom's `docs/plans/README.md`). `[x]` is a claim; claims are checked against
the code index.

- [x] Deterministic, scriptable LLM client for tests {symbol: NewMockClient}
- [x] Failover across LLM lanes {symbol: NewFailover}
- [x] Prompt-injection detector {symbol: NewDetector, package: foundations/security/injection-detector}
