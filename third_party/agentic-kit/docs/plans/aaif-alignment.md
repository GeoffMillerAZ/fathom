# AAIF alignment work order

Request (owner, 2026-10-23): "look at all those projects and plan and spec durably the decisions you make inspired by that."

Decision [0034](../intent/decisions/0034-open-ecosystem-interoperability.md), goal G15,
[ten-project assessment](../assessments/2026-10-23-aaif-alignment.md), and
[acceptance specification](../designs/aaif-alignment.md) govern this work.
Status: accepted design, **implementation queued, no rows claimed complete**.

## Admission and source anchors

The two current implementation slots and W1-W4 prerequisites remain unchanged.
AF-1/AF-3 refine AE-13's host boundary, AF-2 refines AE-5's provider boundary, and
AF-4 composes AE-1..4/AE-19's evidence path. Metadata maintenance is bounded separately
from a new adapter/host pilot. An open row is planned work, not admission to dispatch
providers, install software, mutate consumers or start external services.

Before an implementation row starts, record its receiver, exact profile/expected scope,
baseline, maintainer, free slot or maintenance classification, resource/spend/disclosure
bounds, review date and stop condition under 0004. The design's five-day/zero-provider
default is a stop-and-review bound, not permission for runtime infrastructure.

Fathom resolved the following existing source seams at `2fc2fe7`: `planHostInstall`
(`cmd/kit-new/host_install.go`), `New` (`foundations/llmclient/openai/config.go`),
`NewHTTPHandler` (`kits/mcp-server/http.go`) and `PrepareDecision`
(`kits/catalog-kit/decision.go`), with tests in their package context. They locate
future changes, not proof of the new behavior. Narrow contracts to actual new helpers
at implementation time. Static reachability alone cannot close the acceptance cases.

## First qualification sequence

- [ ] Qualify Agent Skills artifacts and shared AGENTS.md host bindings without incidental installation [AF-C05] [AF-A01] [AF-A02] {id: AF-1, symbol: planHostInstall, package: cmd/kit-new, ctx: docs/designs/agentic-engineering/security-hosts.md}
- [ ] Qualify existing opt-in Responses mapping against a pinned Open Responses profile; record native/unsupported losses [AF-C06] [AF-A03] [AF-A04] [AF-A05] {id: AF-2, symbol: New, package: foundations/llmclient/openai, ctx: foundations/llmclient/openai/responses.go}
- [ ] Prove the pinned MCP tool path and minimal external-host recipe, starting with goose only when receiver-admitted [AF-C09] [AF-A09] {id: AF-3, symbol: NewHTTPHandler, package: kits/mcp-server, ctx: docs/intent/decisions/0003-mcp-revisions-track-never-chase.md}
- [-] Bind expected scope and compatibility receipts into the same controlled task/profile across two real peers/hosts [AF-C01] [AF-C02] [AF-C03] [AF-C04] [AF-A12] [AF-A13] [AF-A14] {id: AF-4, disposition: deferred, reason: "receiver and common profile not yet selected", trigger: "receiver selects a common interface and its applicable AF-1/2/3 qualifications; record only those dependencies on admission"}

AF-1..3 may be narrowed to the actual receiver's applicable profile; they do not require
every native feature. AF-4 is deliberately not gated on all three: an API-only pilot
need not install a host or MCP service. On admission replace its disposition with the
actual code contract and selected `needs:`. Its independent expected scope must justify
N/A rather than omit a required negative. No common grade or all-AAIF compliance claim.

## Conditional follow-through

- [-] External OpenSandbox execution profile and real isolation/lifecycle proof [AF-C08] [AF-A07] [AF-A08] {id: AF-5, disposition: deferred, reason: "no admitted isolated consumer tool path or runtime budget", trigger: "named receiver requires isolation and admits actual backend proof under a free slot"}
- [-] OpenEnv episode mapping into existing eval-runner with independent domain verdicts [AF-C07] [AF-A06] {id: AF-6, disposition: deferred, reason: "no named environment corpus and independent acceptance oracle", trigger: "receiver supplies pinned environment task and baseline plus bounded episode study"}
- [-] A2A task boundary preserving local authority and uncertain-effect reconciliation [AF-C10] [AF-A10] {id: AF-7, disposition: deferred, reason: "no named independently operated remote-agent workflow; I20 remains", trigger: "consumer names a peer and admits a bounded adapter without a worker executor"}
- [-] Consumer-owned agentgateway policy profile [AF-C11] [AF-A11] {id: AF-8, disposition: deferred, reason: "no selected proxy topology or runtime acceptance", trigger: "consumer needs proxy enforcement and owns identity/TLS/deployment plus live denial probes"}
- [-] Consumer-owned Agent Router endpoint profile [AF-C11] [AF-A11] {id: AF-9, disposition: deferred, reason: "existing endpoint and failover suffice; F7 value trigger unproven", trigger: "named consumer establishes routing need and selects exact API/usage/error acceptance"}
- [-] Submit a minimized reusable upstream case or record a reasoned no-upstream outcome [AF-C12] [AF-A15] {id: AF-10, disposition: deferred, reason: "hypotheses are not demonstrated upstream defects or authorized submissions", trigger: "profile review produces a sanitized nonduplicate case and receiver submission authority"}

If a conditional row is admitted, replace its disposition with a code contract grounded
in the selected receiver. Do not fake a symbol for an unbuilt adapter. A new dependency,
protocol revision or broad consumer/global change retains its own authority boundary.

## Evidence before completion

Retain spec/implementation/profile/configuration/test pins, independent expected cases,
protocol translation loss and actual positive/negative results. Unit mocks are useful
but cannot prove host activation, proxy policy, external isolation or independent peers.
G15 additionally requires real controlled-path evidence and AF-A15's receiver disposition.
Model quality and savings require separate application-specific studies.

Run `task validate`; for exported/library-root changes run the configured
`task downstream-check` and name every actual consumer revision, pass and skip.
Refresh the index through the daemon and inspect strict plan verification before any
row becomes checked:

```sh
fathom verify --service agentic-kit --plan docs/plans/aaif-alignment.md --strict
```

Do not use `--require-all` while deferred work is unadmitted. Operational unknowns and
zero checked rows establish no implementation proof. TRACKING owns implementation
status; this work order owns tasks, scope, dependencies and revival conditions.
Current library anchors can report wiring unknown; bind the actual qualification
entrypoint when implementing rather than waiving wiring to claim interoperability.
