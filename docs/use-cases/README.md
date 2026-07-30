# Use-case guides for putting fathom to work

These guides show how to apply an already-wired-in fathom to concrete
workflows. Each doc covers the relevant MCP tools and CLI commands, a worked
example, and setup notes.

**First time setting up fathom?** Go to
**[docs/adoption/](../adoption/)** instead — the definitive zero-to-working
guide per AI coding assistant
([claude-code.md](../adoption/claude-code.md),
[codex.md](../adoption/codex.md),
[spec-driven-development.md](../adoption/spec-driven-development.md)). Come
back here once fathom is indexed, registered, and returning results.
[getting-started.md](getting-started.md) remains the from-source manual
walkthrough behind `fathom init`, and [coding-agents.md](coding-agents.md)
is the canonical short wiring summary the adoption guides expand on.

---

## Guides

| Doc | Description |
|---|---|
| [coding-agents.md](coding-agents.md) | **The canonical wiring guide**: `fathom init`, hooks, gates, CLAUDE.md/AGENTS.md, the agent surface, and the honest cost story |
| [getting-started.md](getting-started.md) | Build, index, register, and make your first query — zero to live code intelligence in one session |
| [coding-context.md](coding-context.md) | Assemble a precise, token-efficient symbol context slice for active feature work |
| [cross-repo-blast-radius.md](cross-repo-blast-radius.md) | Find all callers of a symbol across every service in the fleet before a refactor or rename |
| [spec-verification.md](spec-verification.md) | Verify implementation against a spec or task list with per-task evidence verdicts |
| [incident-investigation.md](incident-investigation.md) | Triage a production symptom using pre-computed call graphs, complexity hotspots, and commit history |
| [change-review.md](change-review.md) | Gate a PR on blast radius, API breakage, architectural conformance, and dependency drift |
| [fleet-health.md](fleet-health.md) | Surface security posture, complexity hotspots, dead code, and dependency inventory across all services |
| [platform-modules.md](platform-modules.md) | Bind a compiled-once platform module into a repo's edit-time obligations, and guard critical scopes for pre-edit guidance |
| [research-flywheel.md](research-flywheel.md) | Aggregate fathom's own local instrumentation into an honest report, and produce a sanitized, review-first bundle you may choose to share |

---

## Maintaining these docs

Each use case has exactly one doc here. When an MCP tool or CLI command is added, renamed, or removed:

1. Find the doc(s) whose **## Maintained** line lists that tool or command.
2. Update the Tools table, any worked-example calls, and the Maintained line to reflect the change.
3. If a tool moves between profiles (coding / investigation / platform / shared), update any profile guidance in the doc's **Setup & notes** section.

The MCP tool registry lives in `internal/mcp/*.go` (search for `Name:` fields). CLI subcommands are the `case` arms in `cmd/fathom/main.go`.
