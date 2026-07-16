# Surfaces week-1 — the evidence-gated cleanup (docs/vision/05, ADR-014 amendment)

Call-log evidence (71/87 tools zero-call, batch never used, companion dead
after day one, two gated bounces teaching abandonment) executes H1 item 5's
first tranche. Gate semantics change to execute-and-flag: surfaces are
ergonomics and measurement, never security — bare /rpc is already the full
registry.

## Kills (zero-call, structural redundancy)

- [x] Alias registrations deleted: platform_callers, test_debt, api_routes, deps — canonicals stay {id: SW-1, symbol: registerPlatformTools, package: internal/mcp}
- [x] set_profile removed entirely (HTTP already rejected it; surfaces supersede profiles — amends the ADR-010/014 stdio clause) {id: SW-2, symbol: registerMetaTools, package: internal/mcp}

## The agent surface

- [x] Companion membership becomes the 15-tool agent set: minus batch and brief, plus verify_plan and capabilities; served at /rpc/agent with /rpc/companion as a compatible alias {id: SW-3, symbol: companionTools, package: internal/mcp}
- [x] Off-surface tools/call executes and is logged with an off-surface flag instead of erroring; hard gating stays available behind a server option {id: SW-4, symbol: toolOnSurface, package: internal/mcp}
- [x] serve-http default addr becomes loopback 127.0.0.1:7575 — surfaces were never a security boundary and the default must not listen on all interfaces {id: SW-5, symbol: cmdServeHTTP, package: cmd/fathom}
- [x] ADR-014 amendment recorded in ADR.md; docs/mcp-tools.md + surface_test.go updated in the same change (the three-place membership contract) {id: SW-6, symbol: companionTools, skip_test: true, skip: "docs deliverable anchored to the changed membership symbol per plans-README rule"}

## Operator step (recorded when done)

- [ ] User-scope MCP registration re-pointed at /rpc/agent — the one config line companion failed for want of {id: SW-7, symbol: cmdServeHTTP, skip_test: true, skip: "settings.json change outside the repo; anchored to the serving symbol"}

## Deferred (30-day kill clock runs from SW-7 with origin tagging live)

- [-] Family merges (callgraph/complexity/security rollups) {id: SW-8, disposition: deferred, reason: "zero-demand tools deserve no consolidation effort", trigger: "30-day post-flip log shows selection confusion persisting"}
- [-] Zero-call tool deletions beyond aliases {id: SW-9, disposition: deferred, reason: "current log measures abandonment, not demand", trigger: "30 days of origin-tagged log after the registration flip"}
