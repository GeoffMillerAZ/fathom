# sextant → fathom migration

The project is renamed **fathom** (2026-07-07, ADR-015). In-repo, everything is
done: module path `github.com/geoffmilleraz/fathom`, binary `bin/fathom`, MCP
server name, docs, scripts. Historical records (CHANGELOG entries,
USAGE_JOURNAL.md, dogfood-observations.md) intentionally keep the old name —
they describe past events.

**Back-compat shipped with the rename** (no fleet action needed):

- `.sextantignore` files keep working — the connector reads `.fathomignore`
  first and falls back to the legacy name (`connector.LegacyIgnoreFileName`).
- `scripts/fathom-adoption.sh` counts both `mcp__fathom__` and legacy
  `mcp__sextant__` tool-call prefixes, so pre-rename transcripts still measure.
- `fathom session-hook` treats the `sextant`/`fathom` dir↔service names as one
  identity, so it works before and after the fleet re-ingest and any directory
  rename.
- `scripts/reingest-fleet.sh` resolves the `fathom` service to this repo even
  while the directory is still named `sextant`.

## One-time operator migration (run from a real terminal)

**Status (verified 2026-07-08):** steps 1–3 below are **already done** — the
launchd job is `com.geoffmilleraz.fathom-http`, the MCP server is registered as
`fathom` (HTTP → `localhost:7575`), and the fleet index carries clean service
IDs. The **only** remaining `sextant` artifact in the live system is the on-disk
**directory name** and the absolute paths that point at it (the plist and two
Claude Code hook commands). To finish, run the turnkey script (step 5) — steps
1–4 are retained below as the historical record of what was done.

**1. Swap the launchd daemon** (paths assume the directory has NOT been renamed
yet — adjust if it has):

```sh
launchctl unload ~/Library/LaunchAgents/com.geoffmilleraz.sextant-http.plist
cat > ~/Library/LaunchAgents/com.geoffmilleraz.fathom-http.plist <<'EOF'
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0"><dict>
  <key>Label</key><string>com.geoffmilleraz.fathom-http</string>
  <key>ProgramArguments</key><array>
    <string>/Volumes/m2ext/code/geoffmilleraz/sextant/bin/fathom</string>
    <string>serve-http</string>
    <string>--store</string><string>/Volumes/m2ext/code/geoffmilleraz/sextant/workspace/db/fleet.duckdb</string>
    <string>--addr</string><string>127.0.0.1:7575</string>
  </array>
  <key>RunAtLoad</key><true/>
  <key>KeepAlive</key><true/>
  <key>WorkingDirectory</key><string>/Volumes/m2ext/code/geoffmilleraz/sextant</string>
  <key>StandardOutPath</key><string>/tmp/fathom-http.log</string>
  <key>StandardErrorPath</key><string>/tmp/fathom-http.err</string>
</dict></plist>
EOF
launchctl load ~/Library/LaunchAgents/com.geoffmilleraz.fathom-http.plist
rm ~/Library/LaunchAgents/com.geoffmilleraz.sextant-http.plist
tail -2 /tmp/fathom-http.log   # expect: serve-http: listening on 127.0.0.1:7575
```

**2. Re-register the MCP server** (tool prefix becomes `mcp__fathom__…` in new
sessions; running sessions keep the old registration until restarted):

```sh
claude mcp remove sextant -s user
claude mcp add --transport http --scope user fathom http://localhost:7575/rpc
claude mcp get fathom   # expect ✔ Connected
```

**3. Rebuild the fleet index with clean service IDs** (registers this repo as
service `fathom`; stops/starts the daemon itself):

```sh
/Volumes/m2ext/code/geoffmilleraz/sextant/scripts/reingest-fleet.sh
```

**4. Update the SessionStart hook and add the obligations edit-hook** in
`~/.claude/settings.json` (agent-config changes are yours to approve — an
agent session cannot install these for you):

- In the existing `SessionStart` entry, change the command
  `…/sextant/bin/sextant session-hook` → `…/sextant/bin/fathom session-hook`.
- Add the PostToolUse injection hook (the forced delivery channel for
  semantic-layer obligations — `docs/design/coding-agent-excellence.md` §5.2;
  silent on clean files, silent on any error, never blocks):

```json
"PostToolUse": [
  {
    "matcher": "Edit|Write|MultiEdit",
    "hooks": [
      {
        "type": "command",
        "command": "/Volumes/m2ext/code/geoffmilleraz/sextant/bin/fathom edit-hook",
        "timeout": 5
      }
    ]
  }
]
```

**4b. Compile the obligations layer** (once, after the daemon swap; pennies —
estimate first): the daemon picks up `ANTHROPIC_API_KEY` from its environment
to enable `compile_layer`; or run it directly against a stopped daemon's store:

```sh
bin/fathom compile --service fathom --url http://localhost:7575            # dry-run estimate
bin/fathom compile --service fathom --url http://localhost:7575 --dry-run=false
```

**5. Rename the directory (the last remaining step) — one turnkey script:**

```sh
scripts/complete-dir-rename.sh
```

It stops the daemon, `mv`s the directory, repoints the plist (binary, `--store`,
`--call-log`, `WorkingDirectory`) and the two Claude Code hook command paths in
`~/.claude/settings.json`, reloads the daemon, and verifies it's listening on
`127.0.0.1:7575`. Idempotent. `go.work` uses relative paths and needs no change;
Claude Code project history is keyed by path, so a fresh project context starts
at the new path.

**⚠️ Run it from a real terminal, not via an agent** — two human-only gates:

1. It reloads the launchd daemon; `launchctl` daemon ops wedge when driven from
   the Claude Code Bash tool.
2. **The daemon binary path holds a path-keyed TCC `SystemPolicyRemovableVolumes`
   grant** (granted 2026-07-08; see the comment in the plist). Renaming the
   directory changes the binary path and **invalidates that grant** — the
   daemon's next start touches `/Volumes/m2ext` and hangs silently in `tccd`
   until you answer a macOS **Allow** dialog (a GUI click only a human at the
   machine can make). If the daemon doesn't come up, look for the dialog and
   click Allow, then re-run the script.

Also close any other Claude Code / editor session working in this directory
first (their cwd breaks on the move).

**GitHub:** the repository is renamed `GeoffMillerAZ/fathom`; the old URL
redirects. Local remotes were updated in the same change.

**Fleet docs:** other repos' CLAUDE.md files that name `mcp__sextant__` tools
will drift-but-work (the session hook injects current guidance). Regenerate
with `fathom gen-claude-md` opportunistically.
