#!/usr/bin/env bash
# cloud-setup_test.sh — tests for scripts/cloud-setup.sh and
# scripts/cloud-session.sh (CS-1, CS-3, docs/plans/cloud-sessions.md), run by
# `task scripts-check`. Fakes stand in for go and fathom, so it runs in about
# a second on macOS or Linux and never touches the real repository, HOME,
# daemon or network: cloud-setup.sh runs from a scratch copy of the files it
# reads, with HOME and FATHOM_CLOUD_HOME in the scratch dir.

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd -P)"
scratch="$(mktemp -d "${TMPDIR:-/tmp}/fathom-cloud-setup-test.XXXXXX")"
daemon_pids=""
cleanup() {
  for p in $daemon_pids; do kill "$p" 2>/dev/null || true; done
  for f in "$scratch"/state*/daemon.pid; do [ -f "$f" ] && kill "$(cat "$f")" 2>/dev/null || true; done
  rm -rf "$scratch"
}
trap cleanup EXIT
fail() { echo "cloud-setup_test: FAIL $*" >&2; exit 1; }

fake_bin="${scratch}/fakebin"
mkdir -p "$fake_bin"
base_path="${fake_bin}:/usr/bin:/bin:/usr/sbin:/sbin"

# A fake go: answers the env queries, "installs" a tool by writing an
# executable into $GOBIN (failing the one named in FAKE_GO_FAIL), and
# "builds" bin/fathom as the fake fathom below.
cat >"${fake_bin}/go" <<'EOF'
#!/usr/bin/env bash
echo "go $*" >>"$FAKE_LOG"
case "$1" in
  env)
    case "${2:-}" in
      GOVERSION) echo go1.26.2 ;;
      GOROOT) echo "" ;;
      GOTOOLCHAIN) echo auto ;;
    esac
    ;;
  install)
    mod="${2%@*}"
    name="$(basename "$mod")"
    [ "$name" = "${FAKE_GO_FAIL:-}" ] && { echo "fake go: refusing $name" >&2; exit 1; }
    printf '#!/bin/sh\necho %s\n' "$name" >"${GOBIN}/${name}"
    chmod +x "${GOBIN}/${name}"
    ;;
  build)
    mkdir -p bin
    cp "$FAKE_FATHOM" bin/fathom
    ;;
esac
exit 0
EOF

# A fake fathom: ingest prints the symbol count line; serve-http serves
# /healthz on --addr (python3) and records its flags; remap,
# post-commit-hook and session-hook record what they were given.
cat >"${scratch}/fake-fathom" <<'EOF'
#!/usr/bin/env bash
echo "fathom $*" >>"$FAKE_LOG"
case "$1" in
  ingest) echo "symbols:    42" ;;
  serve-http)
    addr=""
    while [ "$#" -gt 0 ]; do [ "$1" = "--addr" ] && addr="$2"; shift; done
    echo $$ >"${FAKE_STATE}/daemon.pid"
    exec python3 -c '
import http.server, sys
host, port = sys.argv[1].rsplit(":", 1)
class H(http.server.BaseHTTPRequestHandler):
    def do_GET(self):
        self.send_response(200); self.end_headers(); self.wfile.write(b"ok")
    def log_message(self, *a): pass
http.server.HTTPServer((host, int(port)), H).serve_forever()
' "$addr"
    ;;
  session-hook)
    cat >"${FAKE_STATE}/session-hook.stdin"
    echo '{"hookSpecificOutput":{"hookEventName":"SessionStart","additionalContext":"primer"}}'
    ;;
esac
exit 0
EOF
chmod +x "${fake_bin}/go" "${scratch}/fake-fathom"

free_port() { python3 -c 'import socket; s=socket.socket(); s.bind(("127.0.0.1",0)); print(s.getsockname()[1]); s.close()'; }

# ─── cloud-setup.sh ─────────────────────────────────────────────────────────
repo="${scratch}/repo"
mkdir -p "${repo}/scripts"
cp "${SCRIPT_DIR}/cloud-setup.sh" "${SCRIPT_DIR}/cloud-session.sh" "${SCRIPT_DIR}/cloud-tools.env" "${repo}/scripts/"
printf 'module example.com/x\n\ngo 1.26.2\n' >"${repo}/go.mod"
repo="$(cd "$repo" && pwd -P)"

run_setup() { # run_setup STATE [ENV...] — prints the summary line
  local state="$1"
  shift
  mkdir -p "$state"
  env -i PATH="$base_path" HOME="${scratch}/home" FATHOM_CLOUD_ANY_OS=1 FATHOM_CLOUD_SKIP_APT=1 \
    FATHOM_CLOUD_HOME="${scratch}/cloud" FATHOM_CLOUD_BIN_DIR="${scratch}/bin" \
    FAKE_LOG="${state}/log" FAKE_STATE="$state" FAKE_FATHOM="${scratch}/fake-fathom" "$@" \
    bash "${repo}/scripts/cloud-setup.sh"
}

# 1. A first run with one tool failing: exit 0, the failure named, the rest done.
s1="${scratch}/state1"
out="$(run_setup "$s1" FAKE_GO_FAIL=lefthook)" || fail "cloud-setup exited non-zero on a failed optional tool"
case "$out" in *"failed: lefthook"*) ;; *) fail "the failed tool is not named in the summary: $out" ;; esac
case "$out" in *"index ingested (42 symbols)"*) ;; *) fail "the pre-ingest is not in the summary: $out" ;; esac
case "$out" in *"mcp registered (VM user scope)"*) ;; *) fail "the MCP registration is not in the summary: $out" ;; esac
case "$out" in *"go1.26.2"*) ;; *) fail "the resolved toolchain is not in the summary: $out" ;; esac

# Every pin is installed exactly as cloud-tools.env names it.
while IFS='=' read -r key value; do
  case "$key" in *_MODULE) ;; *) continue ;; esac
  version_key="${key%_MODULE}_VERSION"
  version="$(sed -n "s/^${version_key}=//p" "${SCRIPT_DIR}/cloud-tools.env")"
  grep -q "^go install ${value}@${version}\$" "${s1}/log" || fail "no 'go install ${value}@${version}' in: $(cat "${s1}/log")"
done <"${SCRIPT_DIR}/cloud-tools.env"
grep -q '^go build -o bin/fathom ./cmd/fathom$' "${s1}/log" || fail "bin/fathom was not built: $(cat "${s1}/log")"
grep -q "^fathom ingest --service fathom --path ${repo} --store ${scratch}/cloud/store.sqlite\$" "${s1}/log" || fail "the pre-ingest did not index the repo into the cloud store: $(cat "${s1}/log")"
[ -L "${scratch}/bin/fathom" ] || fail "fathom is not linked onto PATH (${scratch}/bin/fathom)"
grep -q "^INDEXED_ROOT=${repo}\$" "${scratch}/cloud/env" || fail "the env file does not record the indexed root: $(cat "${scratch}/cloud/env")"
python3 - "${scratch}/home/.claude.json" "${scratch}/cloud/cloud-session.sh" <<'PY' || fail "the MCP server is not registered in the VM user scope"
import json, sys
cfg = json.load(open(sys.argv[1]))
s = cfg["mcpServers"]["fathom"]
assert s["command"] == "bash" and s["args"] == [sys.argv[2], "mcp"], s
PY

# 2. No time left in the budget: the pre-ingest is deferred, never started.
s3="${scratch}/state3"
out="$(run_setup "$s3" FATHOM_CLOUD_BUDGET=0 FAKE_GO_FAIL=lefthook)" || fail "a zero budget exited non-zero"
case "$out" in *"deferred to the session"*) ;; *) fail "a spent budget did not defer the pre-ingest: $out" ;; esac
grep -q '^fathom ingest' "${s3}/log" && fail "the pre-ingest ran with no budget left"

# 3. A re-run (a cached snapshot): installed pins are not rebuilt, the
#    failed one is retried, and the Codex block is not duplicated.
s2="${scratch}/state2"
out="$(run_setup "$s2")" || fail "the re-run exited non-zero"
case "$out" in *"failed: none"*) ;; *) fail "the re-run should succeed once the tool installs: $out" ;; esac
grep -q "^go install .*task@" "${s2}/log" && fail "the re-run rebuilt task, already at its pin"
grep -q "^go install .*lefthook@" "${s2}/log" || fail "the re-run did not retry the tool that failed"
[ "$(grep -c '^\[mcp_servers.fathom\]' "${scratch}/home/.codex/config.toml")" = 1 ] || fail "the Codex MCP block was written twice"

# 4. Off Linux it does nothing and says so (the owner's Mac deploys with task deploy).
if [ "$(uname -s)" != "Linux" ]; then
  out="$(env -i PATH="$base_path" HOME="${scratch}/home2" bash "${repo}/scripts/cloud-setup.sh")"
  case "$out" in *"nothing done"*) ;; *) fail "off Linux it should do nothing: $out" ;; esac
  [ -e "${scratch}/home2/.fathom-cloud" ] && fail "off Linux it wrote ${scratch}/home2/.fathom-cloud"
fi

# ─── cloud-session.sh ───────────────────────────────────────────────────────
session="${repo}/scripts/cloud-session.sh"

# 5. A local session (CLAUDE_CODE_REMOTE unset): exit 0, nothing printed,
#    the wrapped command never runs.
out="$(echo '{}' | env -i PATH="$base_path" HOME="${scratch}/home" bash "$session" hook touch "${scratch}/ran")"
[ -z "$out" ] || fail "a local session printed: $out"
[ -e "${scratch}/ran" ] && fail "a local session ran the wrapped command"

run_hook() { # run_hook STATE PORT [ENV...] -- stdin from the caller
  local state="$1" port="$2"
  shift 2
  mkdir -p "$state"
  env -i PATH="$base_path" HOME="${scratch}/home" CLAUDE_CODE_REMOTE=true \
    CLAUDE_PROJECT_DIR="$repo" CLAUDE_ENV_FILE="${state}/claude.env" TMPDIR=/tmp \
    FATHOM_CLOUD_HOME="${scratch}/cloud" FATHOM_CLOUD_ADDR="127.0.0.1:${port}" \
    FAKE_LOG="${state}/log" FAKE_STATE="$state" "$@" \
    bash "$session" hook "${scratch}/cloud/bin/fathom" session-hook
}
stop_daemon() { [ -f "$1/daemon.pid" ] && kill "$(cat "$1/daemon.pid")" 2>/dev/null; rm -f "$1/daemon.pid"; }

# 6. A cloud session on the provisioned VM: the daemon starts with the
#    unmanaged-but-unbounded flags, a refresh is queued (the indexed root is
#    this clone), the primer gets the hook's stdin, PATH reaches the session.
s6="${scratch}/state6"
port="$(free_port)"
out="$(echo '{"hook_event_name":"SessionStart"}' | run_hook "$s6" "$port")" || fail "the cloud hook exited non-zero"
stop_daemon "$s6"
[ "$out" = '{"hookSpecificOutput":{"hookEventName":"SessionStart","additionalContext":"primer"}}' ] || fail "the hook's stdout must be the primer alone, got: $out"
grep -q -- "serve-http --store ${scratch}/cloud/store.sqlite --addr 127.0.0.1:${port} --auto-refresh=true --exit-with-parent=false --idle-exit=0 --ttl=0 --call-log ${scratch}/cloud/mcp-calls.jsonl" "${s6}/log" || fail "the daemon did not start with the cloud flags: $(cat "${s6}/log")"
grep -q "^fathom post-commit-hook --dir ${repo} --service fathom --quiet" "${s6}/log" || fail "no refresh was queued for the clone: $(cat "${s6}/log")"
grep -q '"hook_event_name":"SessionStart"' "${s6}/session-hook.stdin" || fail "the primer did not get the hook's stdin"
grep -q "^export PATH=\"${scratch}/bin:" "${s6}/claude.env" || fail "PATH was not exported to the session: $(cat "${s6}/claude.env" 2>/dev/null)"
grep -q "^export TMPDIR=\"${scratch}/cloud/tmp\"" "${s6}/claude.env" || fail "a /tmp TMPDIR was not moved off /tmp for the session: $(cat "${s6}/claude.env" 2>/dev/null)"

# 7. The clone lives elsewhere than the checkout setup indexed: the service
#    is remapped before the daemon owns the store.
printf 'INDEXED_ROOT=/somewhere/else\n' >>"${scratch}/cloud/env"
: >"${scratch}/cloud/store.sqlite"
s7="${scratch}/state7"
port="$(free_port)"
echo '{}' | run_hook "$s7" "$port" >/dev/null || fail "the remap hook exited non-zero"
stop_daemon "$s7"
grep -q "^fathom remap --service fathom --path ${repo} --store ${scratch}/cloud/store.sqlite\$" "${s7}/log" || fail "a moved clone was not remapped: $(cat "${s7}/log")"
[ "$(sed -n 's/^INDEXED_ROOT=//p' "${scratch}/cloud/env" | tail -1)" = "$repo" ] || fail "the remap was not recorded"

# 8. Never provisioned: say so on stdout; with no setup script in the clone,
#    nothing is launched.
bare="${scratch}/bare"
mkdir -p "$bare"
out="$(echo '{}' | env -i PATH="$base_path" HOME="${scratch}/home3" FATHOM_CLOUD_HOME="${scratch}/cloud-bare" CLAUDE_CODE_REMOTE=true CLAUDE_PROJECT_DIR="$bare" bash "$session" hook "${scratch}/cloud/bin/fathom" session-hook)"
case "$out" in *"not provisioned in this cloud VM"*"is missing"*) ;; *) fail "an unprovisioned VM was not said: $out" ;; esac

echo "cloud-setup_test: ok"
