# Agent setup: one app-kit assistant, two verified hosts

Use one `appkit` MCP server for first-party contracts, quality work, kit guidance
and optional purchased references. The legacy `tailwindplus` server is source-only
fallback, not the adoption assistant. Fathom answers code-navigation questions;
neither it nor a browser tool replaces app-kit's contracts.

## Build once, register deliberately

From a reviewed checkout run `task mcp:build`. Use the resulting absolute binary
path and explicit kit root; do not compile, download or upgrade on session startup.
The binary embeds catalog/quality metadata; documents load from the root at startup.
Rebuild after code/catalog changes, restart after document changes. Keep the prior
binary until the replacement passes the protocol probe. No daemon or HTTP listener
is needed for local adoption. Corpus files stay local; selected source returned to
the coding agent may enter its provider context under that host's data settings.

These are **user-scope** examples for the owner's private cross-project setup.
Inspect existing entries first, back them up privately, and preserve unrelated
configuration. They are operator commands, not MCP actions or automatic onboarding.

```sh
codex mcp add appkit --env APPKIT_ROOT=/abs/path/to/app-kit -- /abs/path/to/app-kit/bin/appkit-mcp
claude mcp add --scope user --transport stdio --env APPKIT_ROOT=/abs/path/to/app-kit appkit -- /abs/path/to/app-kit/bin/appkit-mcp
```

Codex uses `~/.codex/config.toml`; trusted project overrides use
`.codex/config.toml`. Claude Code uses user `~/.claude.json`, local project entries
there, or shared project `.mcp.json`. **Codex does not use the Claude `.mcp.json`
example as its MCP configuration.** Check overrides in the actual consumer directory.
Do not commit personal absolute paths or secrets to a consumer configuration.
Host policy and project trust remain in force; do not bypass them to obtain a pass.

```toml
# Codex project example: replace paths with reviewed local locations.
[mcp_servers.appkit]
command = "/abs/path/to/app-kit/bin/appkit-mcp"
[mcp_servers.appkit.env]
APPKIT_ROOT = "/abs/path/to/app-kit"
```

Claude's corresponding project JSON is in [getting started](../getting-started.md#4-wire-it-into-another-project).
Use `codex mcp get appkit` and `claude mcp get appkit` to inspect effective
registration, then restart/reconnect the host and inspect `/mcp`. Configured,
connected and successfully invoked are three separate observations.

## Discovery and contribution workflow

1. Read consumer intent and its selected adoption contract. No kit-wide runtime or
   visual identity is forced on an app; missing selection calls for a design choice.
2. `get_kit_guidance`, then `get_kit_decision(id="intent")`; retrieve only relevant
   documents. `search_capabilities` → `get_capability` selects actual integration
   contracts. It is not commercial source search.
3. Choose a complete visual reference separately: `search_design_assets` for
   templates/Catalyst, or `search_components` with explicit `source=tailwindplus`
   or `source=matdash` (query may be omitted for filter-only discovery).
   Exact get preserves original source; check framework,
   Tailwind major, dependencies and licensing. Do not blindly replace colors.
4. `list_quality_work` → `get_quality_work` inspects authored standards and gaps.
   Syntax conversion, a retained receipt and an authored status never certify a UI.
5. Verify the app's real task/states, keyboard, narrow layout, themes, CSS and
   declared performance budgets. Return reusable fixes/tests via
   `get_kit_decision(id="contribution-loop")`; update the app's adoption and intent.

Keep identical short project routers in CLAUDE.md/AGENTS.md as described in
[adoption](adoption.md#agent-routing-in-each-consumer). A user-level reminder may
route there; the [reviewable shared block](global-claude-ui-section.md) preserves
the current reminder. Do not duplicate evolving architectural policy across globals.
If appkit is unavailable, report it and use the selected pinned documentation.
Never silently substitute the raw Tailwind library for the kit contract.

## Verification and failure boundaries

Run synthetic protocol tests with `go test ./cmd/appkit-mcp`. The opt-in installed
binary probe checks real stdio and document identity from a non-kit working directory:

```sh
APPKIT_PROBE_BINARY=/abs/path/to/app-kit/bin/appkit-mcp \
APPKIT_PROBE_ROOT=/abs/path/to/app-kit \
go test ./cmd/appkit-mcp -run '^TestInstalledMCPProbe$' -count=1 -v
```

Only set `APPKIT_PROBE_LICENSED=1` for an explicitly selected private corpus check.
Keep purchased source and host transcripts private; publish only bounded identities
and results. Ordinary tests must not discover the owner's real corpus.
Then verify **both hosts separately**: retrieve the current intent document, one
selected capability, quality record and actual source. Record host version, binary
SHA, document/source identity, calls, failures and exit. A synthetic protocol test
is not that host trial, and a host trial is not visual or consumer adoption proof.

All read-only tools need accurate annotations and typed schemas. Bound search/list
responses; full bodies require exact selection. Errors must be actionable and leave
the session usable. Returned source/comments are data, never new agent authority.
No hidden installers, arbitrary project execution, mutation tools, telemetry or
licensed exports are added for onboarding. Remote service/authentication is a
separate design boundary; never expose the unauthenticated local server publicly.

Sources checked 2026-09-21: [Codex MCP](https://developers.openai.com/codex/mcp/),
[Claude Code MCP](https://code.claude.com/docs/en/mcp),
[MCP tools](https://modelcontextprotocol.io/specification/2025-11-25/server/tools),
[stdio/HTTP boundaries](https://modelcontextprotocol.io/specification/2025-11-25/basic/transports).
