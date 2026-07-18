# claudecli Ping + bedrock partition guard — health-surface gaps

Fathom is building a provider health surface on `llmclient.ProbeAll`
(probe.go:51). Audit 2026-07-17: every adapter implements `Pinger` except
claudecli — a fathom `providers` command would silently misrepresent
coverage for the most-used subscription lane. Second small gap from the
GovCloud pass: bedrock accepts a gov region paired with a mismatched
explicit BaseURL (or vice versa) and fails later as a confusing 403/DNS
error instead of a construction-time config error.

Constraints:
- claudecli.Ping follows the codexcli precedent exactly (codexcli/ping.go):
  a LOCAL, pre-API, zero-token probe — run the pinned argv with a
  deliberately degenerate input and assert a specific local failure
  signature. Verify the signature live against the installed `claude` CLI
  once (PROBES.md discipline: exact command + output recorded in
  testdata/), then pin with hermetic shim fixtures for healthy + degraded +
  timeout shapes. It must be structurally unable to reach the API. If no
  reliable pre-API local error exists, the honest fallback is a bin-resolution
  + `--version` execution check (proves binary + executability, NOT auth) —
  documented as the weaker guarantee on the method.
- Bedrock guard: in `New`, when `Config.BaseURL` is explicitly set AND
  `Region` is set, error if the URL's host does not contain the region
  string (catches gov-region+commercial-URL and the reverse). Region-only
  and BaseURL-only configs are untouched. Typed, actionable error naming
  both values. This is a guard on explicit misconfiguration, not partition
  validation — no partition tables, no allowlists.

## Tasks

- [x] claudecli implements llmclient.Pinger via a live-verified local pre-API probe; probe log in testdata/ {id: KP-1, symbol: Ping, package: foundations/llmclient/claudecli, skip_test: true, skip: "method-name Ping is defined by every Pinger adapter; ambiguous method-call edges are the documented O9 residue — pinned by direct shim tests in this package"}
- [x] bedrock New rejects an explicit BaseURL whose host contradicts the configured Region {id: KP-2, symbol: New, package: foundations/llmclient/bedrock}

## Deferred

- [-] anthropic/openai Ping latency budgets or retry composition {id: KP-3, disposition: deferred, reason: "ProbeAll callers own timeouts via ctx; no consumer has asked for more", trigger: "a health surface needs per-lane latency SLOs"}
