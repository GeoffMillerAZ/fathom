# LLM surface — verified capabilities (dogfood plan)

Contract-bearing checklist gated by `fathom verify` at pre-push (convention:
fathom's `docs/plans/README.md`). `[x]` is a claim; claims are checked against
the code index.

- [x] Failover across LLM lanes {symbol: NewFailover}
- [x] Shared conformance suite validates any LLM adapter {symbol: RunConformance, package: foundations/llmclient/llm-test}
- [x] Canonical request hash for replay matching {symbol: RequestHash}
- [x] OpenAI adapter construction from config {symbol: New, package: foundations/llmclient/openai}
