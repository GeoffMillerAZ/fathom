### `plugin` — one tree, two manifests

```sh
fathom plugin render [--out DIR] [--mcp-url U] [--marketplace] [--client cursor]
```

Renders the plugin tree into `--out` (the output dir belongs to the
generator, so re-rendering overwrites) from the same embedded sources
`init` installs — never hand-maintained:

| File | Loader |
|---|---|
| `plugin.json`, `mcp.json` | Agent Plugins 1.0 (VS Code, Cursor, Copilot, Codex, Kiro, …): the closed `$schema`-selected manifest and the `streamable-http` loopback registration, plus an `X-Fathom-Default-Surface` header naming the `/rpc/agent` default eight (RH-5) |
| `.claude-plugin/plugin.json`, `.mcp.json`, `hooks/hooks.json` | Claude Code: its manifest (whose open `metadata` field also carries `defaultSurface`/`surfaceGroups`, RH-5), the `type: http` registration, the full hook template spelled via the shim-stable `fathom` |
| `.cursor-plugin/plugin.json` (only with `--client cursor`) | Cursor's own manifest (RH-8): `mcpServers` is a path string (`"./mcp.json"`) pointing at the portable file above, not an inline server map — the one documented structural difference from the Agent Plugins pair |
| `skills/<name>/SKILL.md` | both — ten skills, spec-only frontmatter (the six Agent Skills fields are the shared floor) |

Both manifests vet against `schema/plugin/schema.cue` under `task cue-vet`
(`TestPluginManifestsVet`), the Agent Plugins pair is golden-pinned
(`cmd/fathom/testdata/plugin/`), and `scripts/ci.sh` runs `claude plugin
validate --strict` over the rendered tree when a `claude` binary is on PATH
(a named skip otherwise). `--marketplace` also writes
`.claude-plugin/marketplace.json` beside the plugin dir. A rendered
template's content is pinned to a digest (`pluginTemplateDigestPin`,
`TestPluginTemplateDigestPinned`): regenerate the pin and bump
`pluginVersion` together whenever a skill, hook, or the default surface
moves.

---
