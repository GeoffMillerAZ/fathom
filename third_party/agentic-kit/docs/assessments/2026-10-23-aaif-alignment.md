# AAIF alignment assessment

Research date: 2026-10-23. Kit baseline: `2fc2fe715456a432b0858fa9e24bb3565ce143c2`.
Decision: [0034](../intent/decisions/0034-open-ecosystem-interoperability.md).
Accepted design and acceptance cases: [alignment specification](../designs/aaif-alignment.md).
This is a source assessment, not installation, interoperability testing or endorsement.

## What changed

The [Linux Foundation announcement](https://www.linuxfoundation.org/press/agentic-ai-foundation-announces-four-new-projects-agent-skills-open-responses-openenv-and-opensandbox)
is dated **2026-10-22**, rather than October 23. It adds **Agent Skills, Open Responses,
OpenEnv and OpenSandbox**. The [AAIF directory](https://aaif.io/projects) lists ten projects.
Hosting is evidence of ecosystem direction, not proof of protocol maturity, safety,
cross-provider parity or compatibility with this kit.

**Recommendation:** make governed interoperability a project outcome. Reuse the open
formats and external systems at their actual boundaries; contribute portable failure
cases and domain-independent fixes. Retain the kit's evidence, authority and outcome
contracts. Do not turn ten upstream projects into ten new local implementations.

## All ten projects: decisions and gaps

The IDs below are stable assessment references. "Qualify" means design accepted and
tests still needed. "Conditional" means no implementation admission yet.

| ID / project | Current kit evidence | Disposition | Missing proof / first useful contribution hypothesis |
|---|---|---|---|
| AAIF-01 Agent Skills, **new** | Authored `.claude/skills`; `.agents/skills` host link | Qualify portable artifacts alongside AGENTS.md | Pinned format validation, dependency disclosure and real host activation; upstream missing validator/host-behavior counterexample |
| AAIF-02 Open Responses, **new** | `foundations/llmclient/openai/config.go` already has `UseResponses`; `responses.go` disables implicit storage and bounds native parameters | Qualify this existing lane against a dated Open Responses profile; retain `LLMClient` | Shared schema/SSE/tool lifecycle and loss matrix across independent implementations; upstream missing stream/extension acceptance case |
| AAIF-03 OpenEnv, **new** | `kits/eval-runner` owns benchmark evidence and caller declarations | Conditional episode-source adapter, not a trainer | Environment/task/image/seed/action identity, independent verdict and real repeatability; upstream episode/termination or reproducibility contract case |
| AAIF-04 OpenSandbox, **new** | `foundations/agent/bounded-exec` explicitly lacks isolation; harness owns dispatch/effect approval | Conditional external execution adapter; never relabel bounded-exec | Runtime isolation, finite lifetime, egress, credentials, cancellation and cleanup; upstream lifecycle/egress/partial-execution regression |
| AAIF-05 MCP | Client/server explicitly implement 2025-11-25; revision/deprecation tests already ship | Maintain and qualify the pinned profile under 0003 | Actual client/server tool/auth/error compatibility; review a revision bump only on 0003's trigger; upstream narrow conformance reproducer |
| AAIF-06 AGENTS.md | `cmd/kit-new/host_install.go` owns shared instructions and host-specific installation | Retain one semantic source, qualify each host/version | Instruction resolution and observed hook behavior are separate; upstream reproducible precedence/packaging documentation |
| AAIF-07 goose | No supported native host adapter; MCP tools are the smaller existing boundary | Conditional recipe/host pilot after the pinned MCP proof | Real discovery/call/error/denied-effect behavior, bounded retries and usable setup; upstream governed-tool example with host responsibilities |
| AAIF-08 A2A | No kit A2A wire adapter; local harness is not an A2A implementation | Defer until a named remote-agent workflow needs it | Authenticated remote task, idempotency and uncertain-effect reconciliation; upstream card/task authorization case if a real gap appears |
| AAIF-09 agentgateway | Tool policy and harness approval exist locally; MCP HTTP has no built-in auth/TLS | Conditional consumer-owned external proxy | Both tool discovery and invocation denial, identity/audience propagation, stale metadata and outage behavior; upstream policy consistency case |
| AAIF-10 Agent Router | Existing configurable OpenAI `BaseURL` and ordered failover | Conditional endpoint profile; do not build another router | Exact API shape, retries, usage and auth semantics; OpenAI-compatible does not imply Responses or Decisions support; upstream compatibility row |

No current source evidence establishes Agent Skills activation across hosts, Open
Responses multi-provider conformance, goose adoption, A2A support, OpenEnv execution,
OpenSandbox isolation, or either gateway's installed behavior. These remain gaps.

## Primary-source basis and reproducible snapshots

Default-branch HEADs were read with `git ls-remote HEAD`; they are discovery pins,
not installed versions or a promise that a moving documentation site matches a tag.
Read the linked pinned tree before any implementation or redistribution.

| Project | Inspected upstream HEAD | License recorded at research time | Contract / documentation |
|---|---|---|---|
| Agent Skills | [69ef37e](https://github.com/agentskills/agentskills/tree/69ef37e9424c0a7ea9dd2293b559e43ec8176379) | Apache-2.0 code; CC-BY-4.0 docs | [Format](https://agentskills.io/specification), [README](https://github.com/agentskills/agentskills/blob/69ef37e9424c0a7ea9dd2293b559e43ec8176379/README.md) |
| Open Responses | [8907bc3](https://github.com/openresponses/openresponses/tree/8907bc3b7c4300ff5c21fd647cfde9929243afa1) | Apache-2.0 | [Specification](https://www.openresponses.org/specification), [changelog](https://www.openresponses.org/changelog), [compliance](https://www.openresponses.org/compliance) |
| OpenEnv | [6e5c73c](https://github.com/openenv/OpenEnv/tree/6e5c73c0231dae82878c1a94c43e155527e26105) | BSD-3-Clause | [Concepts](https://huggingface.co/docs/openenv/guides/concepts), [source](https://github.com/openenv/OpenEnv) |
| OpenSandbox | [f3f4a7f](https://github.com/opensandbox-group/OpenSandbox/tree/f3f4a7f419e526de4c3deb250d72fc034986fc28) | Apache-2.0 | [API](https://github.com/opensandbox-group/OpenSandbox/blob/f3f4a7f419e526de4c3deb250d72fc034986fc28/docs/api/index.md), [egress](https://github.com/opensandbox-group/OpenSandbox/blob/f3f4a7f419e526de4c3deb250d72fc034986fc28/specs/egress-api.yaml) |
| MCP | [dae694f](https://github.com/modelcontextprotocol/modelcontextprotocol/tree/dae694f61a31f7a3ec2af3aff709e11f70f61e48) | MIT | [2026-07-28 specification](https://modelcontextprotocol.io/specification/2026-07-28) |
| AGENTS.md | [d001185](https://github.com/agentsmd/agents.md/tree/d001185d792eb6402a58e4cbef1c228b309ec25d) | MIT | [Format](https://agents.md/) |
| goose | [e970eea](https://github.com/aaif-goose/goose/tree/e970eea8ac23067eb5f3bbad767e51cfaf917a91) | Apache-2.0 | [Recipe reference](https://goose-docs.ai/docs/guides/recipes/recipe-reference/) |
| A2A | [98279d6](https://github.com/a2aproject/A2A/tree/98279d62316361ba8451eaef300f72b72e169185) | Apache-2.0 | [Specification](https://a2a-protocol.org/latest/specification/) |
| agentgateway | [b75bb31](https://github.com/agentgateway/agentgateway/tree/b75bb313e5fb930ea3d98fa5f53fb9b68746308b) | Apache-2.0 | [MCP authorization](https://agentgateway.dev/docs/standalone/latest/documentation/configuration/security/mcp-authz/) |
| Agent Router | [6d63a4f](https://github.com/theagentrouter/agent-router/tree/6d63a4f71f56e9cc370ced2da862787d8d04f06c) | Apache-2.0 | [Documentation](https://theagentrouter.ai/docs/), [AAIF entry](https://aaif.io/projects/agent-router) |

Research details that change the design:

- Agent Skills requires named frontmatter and folder agreement; `allowed-tools` is
  experimental. A validator does not prove safe scripts or real host activation.
- Open Responses' current dated spec is 2026-04-24. Its SSE terminal events carry
  completion/failure/incompleteness; `[DONE]` is optional. Native extensions and optional
  WebSocket/compaction support need their own profile, not an inference from `/responses`.
- OpenEnv exposes reset/step/state and environment-owned reward/episode termination.
  Seed support is not a cross-runtime reproducibility guarantee. Its docs warn of API
  changes; the referenced documentation lists v0.10.0, not a kit-qualified installation.
- OpenSandbox permits disabled expiration and has policy-dependent egress; its named
  isolated diff/commit endpoints are specified but currently unavailable (503). Debug
  diagnostics are best-effort, not an authoritative audit channel.
- MCP's 2026-07-28 specification is known already by 0003. Its availability is not a
  newly fired upgrade trigger; the kit's removal floor is 2027-07-28 for the retained
  handshake/ping features. Agent Router and agentgateway are distinct projects.

## Implications for our projects

| Receiver | Useful next design input | Authority retained by receiver |
|---|---|---|
| Platform engineering / Review Kit | Review standards profiles, unsupported capabilities, data lineage and planted boundary failures | Criteria, evidence tiers, deployment and acceptance policy |
| Fathom | Index pinned contract/source anchors and expose stale/unknown evidence | Static source proof does not certify a running host, sandbox or protocol peer |
| app-kit | Reuse the workbench's capability, denied/unknown, evidence and reconciliation presentations | Presentation contracts, accessibility and actual runtime controls remain distinct |
| consumer-a | Exercise actual MCP/tool/provider seams against an exact optional profile | Domain evidence and source/authorization floors |
| consumer-c | Bounded provider/retrieval/prose comparisons and a small human review surface | Private data, publication policy and independently adjudicated labels |
| Consumer-d | Later display a receiver's exact capability/health/control receipts | Existing G3/UI-C and future admission; do not infer controls from observation |

These are receiver targets, not changes or commitments made in their repositories.
No services, models, third-party code, installers or public contribution tools were run.
Fathom supplied code locations at the kit baseline; raw/incomplete obligations remain
O11, so current source reads supplied the cited behavior. Firecrawl rejected its token;
official project pages, repositories and Linux Foundation sources supplied research.
