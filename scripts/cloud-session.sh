#!/usr/bin/env bash
# cloud-session.sh — fathom's per-session side of a cloud session VM
# (docs/adoption/cloud-sessions.md). scripts/cloud-setup.sh provisions the VM
# once (tools, bin/fathom, a pre-built index) and the environment cache keeps
# that on disk; what a cache cannot keep is a running process, so every
# session starts its own daemon here.
#
#   cloud-session.sh hook <command...>
#       The SessionStart wrapper (.claude/settings.json). Outside a cloud
#       session (CLAUDE_CODE_REMOTE != true) it exits 0 at once and prints
#       nothing: a local session's own hooks are untouched. In a cloud session
#       it ensures the daemon, exports PATH for the session's Bash commands
#       (CLAUDE_ENV_FILE), installs the lefthook gates, then runs <command>
#       (fathom session-hook) with the hook's stdin, so the session primer is
#       read from a live daemon. When the VM was never provisioned it says so
#       and starts scripts/cloud-setup.sh in the background.
#   cloud-session.sh ensure
#       Start the VM-local daemon (127.0.0.1:7575) when /healthz does not
#       answer; idempotent; onboards or remaps the service when the store
#       needs it. Codex cloud has no hooks: run this once in a task.
#   cloud-session.sh mcp
#       The stdio MCP command cloud-setup.sh registers in the VM's user scope:
#       ensure, then exec `fathom serve-mcp --url .../rpc/agent`, a stdio
#       front for the one daemon (the hooks talk to the same one).
#
# Always exits 0 from `hook` and `ensure` (a hook that fails the session gets
# uninstalled — I9); `mcp` exits with the front's own status. Bash 3.2-safe:
# this file is parsed by every local session's SessionStart on macOS.

set -u

sub="${1:-}"
[ "$#" -gt 0 ] && shift

if [ "$sub" = "hook" ] && [ "${CLAUDE_CODE_REMOTE:-}" != "true" ]; then
  exit 0 # a local session: nothing to do, nothing printed
fi

cloud_home="${FATHOM_CLOUD_HOME:-}"
if [ -z "$cloud_home" ] && [ -r /etc/fathom-cloud.env ]; then
  cloud_home="$(sed -n 's/^FATHOM_CLOUD_HOME=//p' /etc/fathom-cloud.env | head -1)"
fi
[ -n "$cloud_home" ] || cloud_home="${HOME}/.fathom-cloud"
env_file="${cloud_home}/env"

read_env() { # read_env KEY — a value cloud-setup.sh recorded, or empty
  [ -r "$env_file" ] || return 0
  sed -n "s/^$1=//p" "$env_file" | tail -1
}

store="$(read_env STORE)"
[ -n "$store" ] || store="${cloud_home}/store.sqlite"
addr="${FATHOM_CLOUD_ADDR:-127.0.0.1:7575}"
base="http://${addr}"
self_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd -P)"

repo_root() {
  if [ -n "${CLAUDE_PROJECT_DIR:-}" ] && [ -d "${CLAUDE_PROJECT_DIR}" ]; then
    (cd "$CLAUDE_PROJECT_DIR" && pwd -P)
    return
  fi
  git rev-parse --show-toplevel 2>/dev/null || pwd -P
}

# The snapshot and the VM-scope MCP registration belong to the cloud
# environment, not to this repository, so a session on another repo in the
# same environment reaches this script too (through the MCP front). Only a
# clone of this repository may re-point, onboard or refresh the service.
is_this_repo() { [ -f "$1/scripts/cloud-session.sh" ] && [ -f "$1/scripts/cloud-tools.env" ]; }

fathom_bin() {
  local b
  b="$(read_env FATHOM_BIN)"
  if [ -n "$b" ] && [ -x "$b" ]; then echo "$b"; return; fi
  command -v fathom 2>/dev/null && return
  [ -x "${cloud_home}/bin/fathom" ] && echo "${cloud_home}/bin/fathom"
}

healthy() { curl -s --max-time 1 "${base}/healthz" 2>/dev/null | grep -q '^ok$'; }

wait_healthy() { # wait_healthy SECONDS
  local i=0 tries=$(( $1 * 5 ))
  while [ "$i" -lt "$tries" ]; do
    healthy && return 0
    sleep 0.2
    i=$((i + 1))
  done
  healthy
}

ensure_daemon() { # ensure_daemon WAIT_SECONDS — 0 when the daemon answers
  local wait="$1" bin repo indexed lock started=""
  healthy && return 0
  bin="$(fathom_bin)"
  if [ -z "$bin" ]; then
    echo "cloud-session: no fathom binary (the VM was not provisioned by scripts/cloud-setup.sh)" >&2
    return 1
  fi
  mkdir -p "$cloud_home"
  lock="${cloud_home}/daemon.lock"
  # One starter at a time (the SessionStart hook and the MCP front race at
  # launch). A lock older than two minutes is a starter that died.
  if [ -d "$lock" ] && [ -n "$(find "$lock" -maxdepth 0 -mmin +2 2>/dev/null)" ]; then
    rmdir "$lock" 2>/dev/null
  fi
  if mkdir "$lock" 2>/dev/null; then
    started=1
    repo="$(repo_root)"
    indexed="$(read_env INDEXED_ROOT)"
    if [ -f "$store" ] && [ -n "$indexed" ] && [ "$indexed" != "$repo" ] && is_this_repo "$repo"; then
      # The snapshot indexed the checkout setup saw; this session's clone
      # may live elsewhere. Re-point the service before the daemon owns the
      # store (a daemon refuses a differing path without remap).
      if "$bin" remap --service fathom --path "$repo" --store "$store" >>"${cloud_home}/session.log" 2>&1; then
        printf 'INDEXED_ROOT=%s\n' "$repo" >>"$env_file"
      fi
    fi
    FATHOM_TENANCY_BUDGET="${FATHOM_TENANCY_BUDGET:-6GiB}" nohup "$bin" serve-http \
      --store "$store" --addr "$addr" --auto-refresh=true --exit-with-parent=false \
      --idle-exit=0 --ttl=0 --call-log "${cloud_home}/mcp-calls.jsonl" \
      </dev/null >>"${cloud_home}/daemon.log" 2>&1 &
  fi
  wait_healthy "$wait"
  local rc=$?
  if [ -n "$started" ]; then
    rmdir "$lock" 2>/dev/null
    if [ "$rc" -eq 0 ]; then
      repo="$(repo_root)"
      if ! is_this_repo "$repo"; then
        : # another repository's session: serve the snapshot's index as it is
      elif [ -z "$(read_env INDEXED_ROOT)" ]; then
        # No index recorded (setup ran out of budget, or never ran): onboard
        # the service through the daemon's ingest tool (remap: a partial
        # store may name another root). The call runs detached in the
        # daemon, so the short client timeout never cancels it. INDEXED_ROOT
        # stays unrecorded: the next session onboards again, which on a
        # finished index is just a refresh.
        curl -s --max-time 3 -H 'Content-Type: application/json' \
          -d "{\"jsonrpc\":\"2.0\",\"id\":1,\"method\":\"tools/call\",\"params\":{\"name\":\"ingest\",\"arguments\":{\"service\":\"fathom\",\"path\":\"${repo}\",\"remap\":true}}}" \
          "${base}/rpc/extended" >>"${cloud_home}/session.log" 2>&1
      else
        # Queue a refresh: the clone may be ahead of the snapshot's commit.
        "$bin" post-commit-hook --dir "$repo" --service fathom --quiet --url "$base" >>"${cloud_home}/session.log" 2>&1
      fi
    else
      echo "cloud-session: the daemon did not answer ${base}/healthz within ${wait}s; see ${cloud_home}/daemon.log" >&2
    fi
  fi
  return "$rc"
}

session_env() { # export what the session's Bash commands need (CLAUDE_ENV_FILE)
  [ -n "${CLAUDE_ENV_FILE:-}" ] || return 0
  local bindir gobin
  bindir="$(read_env BIN_DIR)"
  gobin="$(read_env GO_BIN)"
  {
    # go.mod's toolchain first, so a test module with its own go line never
    # sends the preinstalled go off to download another toolchain.
    if [ -n "$gobin" ] && [ -x "${gobin}/go" ]; then
      case ":${PATH}:" in *":${gobin}:"*) ;; *) printf 'export PATH="%s:$PATH"\n' "$gobin" ;; esac
    fi
    if [ -n "$bindir" ]; then
      case ":${PATH}:" in *":${bindir}:"*) ;; *) printf 'export PATH="%s:$PATH"\n' "$bindir" ;; esac
    fi
    printf 'export FATHOM_CLOUD_HOME="%s"\n' "$cloud_home"
    # Temp dirs off /tmp: fathom reads a root under /tmp as scratch and
    # ignores its ledger readings, which is right for a stray clone and
    # wrong for a test's t.TempDir() (macOS temp dirs are not under /tmp).
    case "${TMPDIR:-/tmp}" in
      /tmp | /tmp/*) mkdir -p "${cloud_home}/tmp" && printf 'export TMPDIR="%s"\n' "${cloud_home}/tmp" ;;
    esac
    if command -v go >/dev/null 2>&1 && [ "$(go env GOTOOLCHAIN 2>/dev/null)" = "local" ]; then
      echo 'export GOTOOLCHAIN=auto' # go.mod pins the toolchain; local would refuse it
    fi
  } >>"$CLAUDE_ENV_FILE"
}

case "$sub" in
  hook)
    input=""
    [ -t 0 ] || input="$(cat)"
    if [ -z "$(fathom_bin)" ]; then
      # Never provisioned (no setup script on this environment, or it ran
      # before the clone existed): provision now, in the background, so the
      # rest of this session gains fathom when it finishes.
      repo="$(repo_root)"
      mkdir -p "$cloud_home"
      if [ ! -f "${repo}/scripts/cloud-setup.sh" ]; then
        echo "fathom: not provisioned in this cloud VM, and ${repo}/scripts/cloud-setup.sh is missing, so fathom's hooks and MCP tools stay silent this session."
      elif mkdir "${cloud_home}/setup.lock" 2>/dev/null; then
        FATHOM_CLOUD_START=1 nohup bash "${repo}/scripts/cloud-setup.sh" </dev/null >>"${cloud_home}/setup.log" 2>&1 &
        echo "fathom: this cloud VM was not provisioned (no environment setup script ran scripts/cloud-setup.sh), so fathom is provisioning in the background now (about 3 to 5 minutes; log ${cloud_home}/setup.log). Until it finishes, fathom's hooks and MCP tools stay silent. Paste the setup script from docs/adoption/cloud-sessions.md into this environment so the next session starts provisioned."
      else
        echo "fathom: not provisioned in this cloud VM; provisioning is already running in the background (log ${cloud_home}/setup.log)."
      fi
      exit 0
    fi
    # Refresh the installed copy the MCP registration runs (the snapshot can
    # be up to a week older than this clone).
    if [ -f "${cloud_home}/cloud-session.sh" ] && ! cmp -s "${self_dir}/cloud-session.sh" "${cloud_home}/cloud-session.sh"; then
      cp "${self_dir}/cloud-session.sh" "${cloud_home}/cloud-session.sh.new" 2>/dev/null && mv "${cloud_home}/cloud-session.sh.new" "${cloud_home}/cloud-session.sh"
    fi
    ensure_daemon 15 >/dev/null 2>>"${cloud_home}/session.log"
    session_env
    repo="$(repo_root)"
    if command -v lefthook >/dev/null 2>&1 && [ -f "${repo}/lefthook.yml" ] && ! grep -qs lefthook "${repo}/.git/hooks/pre-push"; then
      (cd "$repo" && lefthook install >/dev/null 2>&1)
    fi
    [ "$#" -gt 0 ] || exit 0
    printf '%s' "$input" | "$@"
    exit 0
    ;;
  ensure)
    ensure_daemon "${FATHOM_CLOUD_WAIT:-30}" && echo "cloud-session: daemon up at ${base} (store ${store})"
    exit 0
    ;;
  mcp)
    ensure_daemon 30 >/dev/null 2>>"${cloud_home}/session.log"
    bin="$(fathom_bin)"
    if [ -z "$bin" ]; then
      echo "cloud-session: no fathom binary to serve MCP from (run scripts/cloud-setup.sh)" >&2
      exit 1
    fi
    exec "$bin" serve-mcp --url "${base}/rpc/agent" --wait 30s
    ;;
  *)
    echo "usage: cloud-session.sh hook <command...> | ensure | mcp" >&2
    exit 2
    ;;
esac
