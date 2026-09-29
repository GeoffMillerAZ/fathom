# Governed provider upgrade

The stable provider schema/API expands in this pre-1.0 breaking release. Existing
Chat Completions and synchronous client entry points retain their defaults.

- `openai.Config.UseResponses` explicitly selects `/v1/responses`. It does not
  support Azure deployment routing, stop sequences, explicit thinking budgets or
  caller cache hints. Unsupported combinations fail before HTTP.
- Responses `ProviderParams` cannot override lane-owned request, model, storage,
  tool, stream, reasoning or schema fields. Use the typed request/config fields.
  Completed tools are emitted only after terminal response reconciliation; partial
  calls are never executable. Server storage defaults off and cannot be overridden
  through this lane's extension map.
- Anthropic `Request.Effort` now maps to `output_config.effort`, preserving typed
  `OutputSchema`. Remove raw `ProviderParams.output_config` when either typed field
  is set: ambiguous precedence now fails, rather than silently dropping constraints.
  Low/medium/high/xhigh/max are wire values; model support remains caller capability
  policy. Mock clients accept and record first-class effort.
- `DescribeProvider` is optional and descriptive. Unknown families cannot establish
  independence; wrappers with changing/multiple routes report unknown. Explicit
  compatible-provider family configuration is an operator assertion, not proof.

Receipts and schema-constrained transport are not independent semantic validation.
Keep consumer redaction, paid-attempt accounting and publication authorization at
their existing boundaries. The race/conformance suites use synthetic HTTP fixtures;
they are not claims of live provider quality.
