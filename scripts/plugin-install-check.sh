#!/usr/bin/env bash
# plugin-install-check.sh — the core plugin's install check (docs/design/claude-code-mods.md §7.1).
#
# Renders the core plugin into a scratch HOME under workspace/tmp/, runs `claude plugin validate --strict` on it,
# loads it with --plugin-dir and asserts that Claude Code lists the plugin and its MCP server. Every claude command
# here reads files or lists what loaded: none starts a model turn, so the check spends nothing. No gate runs it.
#
# Exit 0: validated and listed. Exit 1: a check failed (the output names it). Exit 2: the check could not run
# (no `claude` on PATH, or no fathom binary and no `go` to build one), the reason named.
#
# The scratch directory is kept: the printed steps open one ordinary session in it (the Claude Code install test
# in docs/plans/reach.md). Remove it with the printed `rm -rf` line when done.
#
# FATHOM_BIN names a fathom binary to render with; otherwise this builds one from the checkout, so the render is the
# tree's own.

set -euo pipefail

root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd -P)"

if ! command -v claude >/dev/null 2>&1; then
  echo "plugin-install-check: not run: no \`claude\` binary on PATH" >&2
  exit 2
fi

mkdir -p "${root}/workspace/tmp"
scratch="$(mktemp -d "${root}/workspace/tmp/plugin-install-check.XXXXXX")"
home="${scratch}/home"
plugin="${scratch}/fathom"
mkdir -p "${home}"

fathom="${FATHOM_BIN:-}"
if [ -z "${fathom}" ]; then
  if ! command -v go >/dev/null 2>&1; then
    echo "plugin-install-check: not run: FATHOM_BIN is unset and no \`go\` on PATH to build fathom" >&2
    exit 2
  fi
  fathom="${scratch}/bin/fathom"
  (cd "${root}" && go build -o "${fathom}" ./cmd/fathom)
fi

# cc runs claude against the scratch HOME from inside the scratch directory, so neither the person's own settings,
# plugins and MCP servers nor this repository's project files load beside the plugin under test.
cc() {
  (cd "${scratch}" && env -u CLAUDE_CONFIG_DIR -u CLAUDE_CODE_PLUGIN_DIRS HOME="${home}" claude "$@")
}

fail=0
echo "plugin-install-check: claude $(claude --version 2>/dev/null | head -n 1)"
echo "scratch: ${scratch}"

"${fathom}" plugin render --out "${plugin}" >/dev/null
echo "rendered: ${plugin}"

echo "step: claude plugin validate --strict ${plugin}"
if cc plugin validate --strict "${plugin}"; then
  echo "  validate: ok"
else
  echo "  validate: FAILED"
  fail=1
fi

# The listing commands and flags this check relies on, recorded because the design names them unverified.
echo "listing: claude --plugin-dir <plugin> plugin list --json"
listing="$(cc --plugin-dir "${plugin}" plugin list --json 2>&1 || true)"
if grep -q '"id": *"fathom@' <<<"${listing}"; then
  echo "  plugin listed: $(grep -o '"id": *"fathom@[^"]*"' <<<"${listing}" | head -n 1)"
else
  echo "  plugin listed: NO"
  printf '%s\n' "${listing}" | sed 's/^/    /'
  fail=1
fi

echo "listing: claude --plugin-dir <plugin> mcp list"
servers="$(cc --plugin-dir "${plugin}" mcp list 2>&1 || true)"
if grep -q '^plugin:fathom:fathom:' <<<"${servers}"; then
  echo "  MCP server listed: $(grep '^plugin:fathom:fathom:' <<<"${servers}" | head -n 1)"
else
  echo "  MCP server listed: NO"
  printf '%s\n' "${servers}" | sed 's/^/    /'
  fail=1
fi

echo "listing: claude --plugin-dir <plugin> plugin details fathom"
details="$(cc --plugin-dir "${plugin}" plugin details fathom 2>&1 || true)"
grep -E '^  (Skills|Agents|Hooks|MCP servers) \(' <<<"${details}" | sed 's/^  /  inventory: /' || true

if [ "${fail}" -ne 0 ]; then
  echo "plugin-install-check: FAILED (scratch kept: ${scratch})"
  exit 1
fi

cat <<EOF
plugin-install-check: ok

One ordinary session in this HOME (the Claude Code install test, docs/plans/reach.md):
  1. Start the daemon this plugin registers (fathom serve-http) with this repository indexed.
  2. cd ${scratch} && HOME=${home} claude --plugin-dir ${plugin}
     (log in when asked: the scratch HOME holds no credentials)
  3. Run /mcp and confirm plugin:fathom:fathom is connected and lists its tools.
  4. Ask one question a fathom tool answers, such as where cmdPluginRender is defined in service fathom.
  5. Record docs/bench/readings/reach/install-claude-code.md, opening with
     "Installed and answered on <date>", the Claude Code version and the tool that answered.
  6. rm -rf ${scratch}
EOF
