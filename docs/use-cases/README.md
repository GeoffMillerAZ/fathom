# Use-case guides for adopting fathom to enhance Claude Code

These guides show how to wire fathom into your AI coding workflows and get concrete value from each use case. Each doc covers the relevant MCP tools and CLI commands, a worked example, and setup notes.

Start with **getting-started.md** if this is your first time setting up fathom.

---

## Guides

| Doc | Description |
|---|---|
| [getting-started.md](getting-started.md) | Build, index, register, and make your first query — zero to live code intelligence in one session |
| [coding-context.md](coding-context.md) | Assemble a precise, token-efficient symbol context slice for active feature work |
| [cross-repo-blast-radius.md](cross-repo-blast-radius.md) | Find all callers of a symbol across every service in the fleet before a refactor or rename |
| [spec-verification.md](spec-verification.md) | Verify implementation against a spec or task list with per-task evidence verdicts |
| [incident-investigation.md](incident-investigation.md) | Triage a production symptom using pre-computed call graphs, complexity hotspots, and commit history |
| [change-review.md](change-review.md) | Gate a PR on blast radius, API breakage, architectural conformance, and dependency drift |
| [fleet-health.md](fleet-health.md) | Surface security posture, complexity hotspots, dead code, and dependency inventory across all services |

---

## Maintaining these docs

Each use case has exactly one doc here. When an MCP tool or CLI command is added, renamed, or removed:

1. Find the doc(s) whose **## Maintained** line lists that tool or command.
2. Update the Tools table, any worked-example calls, and the Maintained line to reflect the change.
3. If a tool moves between profiles (coding / investigation / platform / shared), update any profile guidance in the doc's **Setup & notes** section.

The MCP tool registry lives in `internal/mcp/*.go` (search for `Name:` fields). CLI subcommands are the `case` arms in `cmd/fathom/main.go`.
