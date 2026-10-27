### Added

- The egress guard (`egress-guard` and `guard-pre-web` capabilities, ADR-083) adds one advisory line before a Bash command or a Claude Code `WebFetch`/`WebSearch` that sends data to a host off `[guards] egress_allow`. `[guards] egress` is `"warn"` by default, `"enforce"` denies, `"off"` silences it.
- It is a string-match tripwire, not data-loss prevention: a command built at run time, Codex web search and MCP calls, and Claude Code's MCP fetch and browser tools pass it (limits in `docs/security.md`).
- The user sets the mode and the user-only allowlist; a repository may tighten the mode and never loosen it, and only `~/.fathom/capabilities.cue` may switch `egress-guard` off. `fathom doctor`'s `egress_guard` row warns when the guard is off.
- Migration: run `fathom init` (or `fathom fleet reconcile --apply`) to wire the new Claude Code `PreToolUse` `WebFetch|WebSearch` entry, then list the hosts you push and fetch to in `egress_allow`, or set `egress = "off"`.
