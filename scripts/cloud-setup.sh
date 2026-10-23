#!/usr/bin/env bash
# cloud-setup.sh — provision a cloud session VM for fathom: Claude Code on
# the web (claude.ai/code, `claude --cloud`) and Codex cloud share it
# (docs/adoption/cloud-sessions.md has the owner steps and the text to paste).
#
#   bash scripts/cloud-setup.sh
#
# Runs as root on Ubuntu 24.04 x86_64 before the agent starts; the
# environment cache snapshots the filesystem afterwards (when the script
# finishes in about five minutes), so later sessions start with everything
# below already on disk. Re-running it is cheap: every step checks first.
#
#   1. Go: go.mod's toolchain through GOTOOLCHAIN=auto (proxy.golang.org);
#      with no `go` at all, the same toolchain zip from proxy.golang.org.
#   2. Tools, in parallel: go-task, cue, lefthook by `go install` at the
#      versions pinned in scripts/cloud-tools.env; rsync (stranger-check) by
#      apt when absent; bin/fathom built pure Go, CGO_ENABLED=0 (ADR-066), so
#      the build needs no C compiler. `task validate` still does: Go's race
#      detector needs cgo on Linux, so `task test` runs -race with
#      CGO_ENABLED=1 and a gcc on PATH, which this script does not install.
#   3. fathom installed under $FATHOM_CLOUD_HOME (default ~/.fathom-cloud)
#      and linked onto PATH; the stdio MCP server registered in the VM's own
#      user scope (~/.claude.json, and ~/.codex/config.toml) — the repo's
#      files and anyone's local setup are untouched.
#   4. The repository pre-ingested into $FATHOM_CLOUD_HOME/store.sqlite while
#      the time budget allows, so the index is in the snapshot; otherwise the
#      session hook onboards it through the daemon in the background.
#
# Never fails the session: every step logs, a failed optional step is named
# in the summary, and the script exits 0 (a non-zero exit stops the session
# from starting). No macOS step runs here (codesign, launchctl and the
# launchd daemon are `task deploy`'s, on the owner's Mac).
#
# Network (the Trusted level covers all of it): proxy.golang.org and
# sum.golang.org (toolchain, modules, tools); archive.ubuntu.com and
# security.ubuntu.com (rsync, only when absent); github.com through the
# session's GitHub proxy (the clone itself). No GitHub release asset and no
# dl.google.com download.
#
# Environment: FATHOM_CLOUD_HOME, FATHOM_CLOUD_BIN_DIR (default
# /usr/local/bin when writable, else ~/.local/bin), FATHOM_CLOUD_BUDGET
# (seconds, default 270: the pre-ingest is skipped or stopped so the whole
# run stays inside the cache window), FATHOM_CLOUD_START=1 (start the daemon
# when done: the session hook's background path).

set -u

t0=$(date +%s)
repo="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd -P)"
cloud_home="${FATHOM_CLOUD_HOME:-${HOME}/.fathom-cloud}"
budget="${FATHOM_CLOUD_BUDGET:-270}"
if [ "$(uname -s)" != "Linux" ] && [ "${FATHOM_CLOUD_ANY_OS:-}" != "1" ]; then
  echo "cloud-setup: this provisions a Linux cloud VM; on macOS build with \`task build\` and deploy with \`task deploy\` (nothing done)"
  exit 0
fi

mkdir -p "${cloud_home}/bin" "${cloud_home}/logs"
log="${cloud_home}/setup.log"
steps="${cloud_home}/logs"
failed=""

say() { echo "cloud-setup: $*" | tee -a "$log"; }
elapsed() { echo $(( $(date +%s) - t0 )); }

if [ -n "${FATHOM_CLOUD_BIN_DIR:-}" ]; then
  bin_dir="$FATHOM_CLOUD_BIN_DIR"
elif [ -w /usr/local/bin ]; then
  bin_dir=/usr/local/bin
else
  bin_dir="${HOME}/.local/bin"
fi
mkdir -p "$bin_dir"
export PATH="${bin_dir}:${PATH}"

# ─── pins (scripts/cloud-tools.env: KEY=value, no expansion) ────────────────
# shellcheck disable=SC2034 # value is read through the eval below
while IFS='=' read -r key value; do
  case "$key" in
    '' | \#*) continue ;;
    *[!A-Z_]*) continue ;;
  esac
  eval "pin_${key}=\$value"
done <"${repo}/scripts/cloud-tools.env"

# ─── 1. Go ──────────────────────────────────────────────────────────────────
go_want="$(sed -n 's/^toolchain go//p' "${repo}/go.mod" | head -1)"
[ -n "$go_want" ] || go_want="$(sed -n 's/^go //p' "${repo}/go.mod" | head -1)"
export GOTOOLCHAIN=auto # go.mod's toolchain wins over an older preinstalled go
export CGO_ENABLED=0 GOWORK=off # the build and the tools are pure Go (ADR-066)

if ! command -v go >/dev/null 2>&1; then
  # No go at all: fetch go.mod's toolchain as the module proxy serves it
  # (golang.org/toolchain), never from dl.google.com.
  arch="$(uname -m)"
  case "$arch" in x86_64) arch=amd64 ;; aarch64 | arm64) arch=arm64 ;; esac
  zip_url="https://proxy.golang.org/golang.org/toolchain/@v/v0.0.1-go${go_want}.linux-${arch}.zip"
  goroot="${cloud_home}/go${go_want}"
  if [ ! -x "${goroot}/bin/go" ] &&
    curl -fsSL "$zip_url" -o "${cloud_home}/toolchain.zip" &&
    python3 - "${cloud_home}/toolchain.zip" "$goroot" <<'PY'; then
import os, stat, sys, zipfile
src, dst = sys.argv[1], sys.argv[2]
with zipfile.ZipFile(src) as z:
    for info in z.infolist():
        rel = info.filename.split("/", 1)[1] if "/" in info.filename else ""
        if not rel or info.is_dir():
            continue
        out = os.path.join(dst, rel)
        os.makedirs(os.path.dirname(out), exist_ok=True)
        with z.open(info) as r, open(out, "wb") as w:
            w.write(r.read())
        mode = (info.external_attr >> 16) & 0o777
        os.chmod(out, mode or 0o644)
PY
    rm -f "${cloud_home}/toolchain.zip"
  fi
  [ -x "${goroot}/bin/go" ] && export PATH="${goroot}/bin:${PATH}"
fi
if ! command -v go >/dev/null 2>&1; then
  say "FAILED: no go and the toolchain zip could not be fetched from proxy.golang.org — nothing else can be built (log ${log})"
  exit 0
fi
ts=$(date +%s)
go_have="$(cd "$repo" && go env GOVERSION 2>>"$log")"
if [ -z "$go_have" ]; then
  say "FAILED: go could not switch to go${go_want} (GOTOOLCHAIN=auto through proxy.golang.org); see ${log}"
  exit 0
fi
# A preinstalled go whose own default is GOTOOLCHAIN=local would refuse
# go.mod's newer toolchain in the session; the user go env file fixes that
# for every later go command (an environment variable still wins).
if [ "$(cd / && env -u GOTOOLCHAIN go env GOTOOLCHAIN 2>/dev/null)" = "local" ]; then
  (cd / && env -u GOTOOLCHAIN go env -w GOTOOLCHAIN=auto) 2>>"$log"
fi
# Run everything below on go.mod's toolchain itself, so a tool's own go line
# never triggers a second toolchain download.
goroot="$(cd "$repo" && go env GOROOT 2>/dev/null)"
[ -n "$goroot" ] && [ -x "${goroot}/bin/go" ] && export PATH="${goroot}/bin:${PATH}"
t_go=$(( $(date +%s) - ts ))

# ─── 2. tools in the background, the build in front ─────────────────────────
# The build and the pre-ingest are the critical path to an index in the
# snapshot, so the tool installs (and apt) run niced beside them and are
# waited for only at the end.
tool_job() { # tool_job NAME MODULE VERSION — go install unless that version is on PATH
  local name="$1" module="$2" version="$3"
  if [ -x "${bin_dir}/${name}" ] && [ -f "${cloud_home}/${name}.version" ] && [ "$(cat "${cloud_home}/${name}.version")" = "$version" ]; then
    return 0
  fi
  # Built with the tools' own Go (TOOLS_GOTOOLCHAIN in cloud-tools.env), not go.mod's toolchain.
  if GOTOOLCHAIN="${pin_TOOLS_GOTOOLCHAIN:-auto}" GOBIN="$bin_dir" nice -n 10 go install "${module}@${version}" >"${steps}/${name}.log" 2>&1; then
    echo "$version" >"${cloud_home}/${name}.version"
  else
    return 1
  fi
}

t_tools0=$(date +%s)
pids="" names=""
start_job() { # start_job NAME CMD... — background, remembered for the wait
  "${@:2}" &
  pids="$pids $!"
  names="$names $1"
}
start_job task tool_job task "${pin_TASK_MODULE:-}" "${pin_TASK_VERSION:-}"
start_job cue tool_job cue "${pin_CUE_MODULE:-}" "${pin_CUE_VERSION:-}"
start_job lefthook tool_job lefthook "${pin_LEFTHOOK_MODULE:-}" "${pin_LEFTHOOK_VERSION:-}"
if [ "${FATHOM_CLOUD_SKIP_APT:-}" != "1" ] && ! command -v rsync >/dev/null 2>&1 && [ "$(id -u)" = 0 ] && command -v apt-get >/dev/null 2>&1; then
  start_job rsync sh -c 'apt-get update -qq && DEBIAN_FRONTEND=noninteractive apt-get install -y -qq rsync' >"${steps}/rsync.log" 2>&1
fi

ts=$(date +%s)
build_ok=""
if (cd "$repo" && go build -o bin/fathom ./cmd/fathom) >"${steps}/build.log" 2>&1; then
  build_ok=1
else
  failed="${failed} build"
  echo "--- build failed:" >>"$log"
  tail -20 "${steps}/build.log" >>"$log" 2>/dev/null
fi
t_build=$(( $(date +%s) - ts ))

# ─── 3. install fathom, register MCP ────────────────────────────────────────
fathom=""
if [ -n "$build_ok" ]; then
  cp "${repo}/bin/fathom" "${cloud_home}/bin/fathom.new" && mv "${cloud_home}/bin/fathom.new" "${cloud_home}/bin/fathom"
  ln -sf "${cloud_home}/bin/fathom" "${bin_dir}/fathom"
  fathom="${cloud_home}/bin/fathom"
fi
cp "${repo}/scripts/cloud-session.sh" "${cloud_home}/cloud-session.sh"

mcp="skipped (no fathom binary)"
if [ -n "$fathom" ]; then
  # The VM's own user scope (~/.claude.json mcpServers): never the repo's
  # .mcp.json, so a local session's MCP setup is exactly what it was.
  if python3 - "${HOME}/.claude.json" "${cloud_home}/cloud-session.sh" <<'PY' 2>>"$log"; then
import json, os, sys
path, script = sys.argv[1], sys.argv[2]
try:
    with open(path) as f:
        cfg = json.load(f)
except (FileNotFoundError, ValueError):
    cfg = {}
cfg.setdefault("mcpServers", {})["fathom"] = {"type": "stdio", "command": "bash", "args": [script, "mcp"], "env": {}}
os.makedirs(os.path.dirname(path) or ".", exist_ok=True)
tmp = path + ".fathom-new"
with open(tmp, "w") as f:
    json.dump(cfg, f, indent=2)
os.replace(tmp, path)
PY
    mcp="registered (VM user scope)"
  else
    mcp="FAILED to register"
    failed="${failed} mcp"
  fi
  codex_cfg="${HOME}/.codex/config.toml"
  if ! grep -qs '^\[mcp_servers\.fathom\]' "$codex_cfg"; then
    mkdir -p "$(dirname "$codex_cfg")"
    printf '\n[mcp_servers.fathom]\ncommand = "bash"\nargs = ["%s", "mcp"]\n' "${cloud_home}/cloud-session.sh" >>"$codex_cfg"
  fi
fi

# ─── 4. pre-ingest inside the budget ────────────────────────────────────────
store="${cloud_home}/store.sqlite"
indexed=""
index="skipped (no fathom binary)"
t_index=0
if [ -n "$fathom" ]; then
  left=$(( budget - $(elapsed) ))
  if [ "${FATHOM_CLOUD_SKIP_INGEST:-}" = "1" ]; then
    index="skipped (FATHOM_CLOUD_SKIP_INGEST)"
  elif [ "$left" -lt 45 ]; then
    index="deferred to the session (${left}s of the ${budget}s budget left)"
  else
    ts=$(date +%s)
    verb=ingested
    [ -f "$store" ] && verb="refreshed (incremental)"
    "$fathom" ingest --service fathom --path "$repo" --store "$store" >"${steps}/ingest.log" 2>&1 &
    ipid=$!
    while kill -0 "$ipid" 2>/dev/null && [ $(( $(date +%s) - ts )) -lt "$left" ]; do sleep 1; done
    if kill -0 "$ipid" 2>/dev/null; then
      kill "$ipid" 2>/dev/null
      wait "$ipid" 2>/dev/null
      rm -f "$store" "${store}-wal" "${store}-shm"
      index="deferred to the session (stopped at the ${budget}s budget; the session hook onboards it through the daemon)"
    elif wait "$ipid"; then
      indexed="$repo"
      index="$(sed -n 's/^symbols: *\([0-9][0-9]*\)$/\1 symbols/p' "${steps}/ingest.log" | tail -1)"
      [ "$verb" = ingested ] || index=""
      index="${verb}${index:+ (${index})}"
    else
      index="FAILED (see ${steps}/ingest.log)"
      failed="${failed} ingest"
      rm -f "$store" "${store}-wal" "${store}-shm"
    fi
    t_index=$(( $(date +%s) - ts ))
  fi
fi

# ─── the tools started in step 2 ────────────────────────────────────────────
set -- $names
for pid in $pids; do
  name="$1"
  shift
  if ! wait "$pid"; then
    failed="${failed} ${name}"
    echo "--- ${name} failed:" >>"$log"
    tail -20 "${steps}/${name}.log" >>"$log" 2>/dev/null
  fi
done
t_tools=$(( $(date +%s) - t_tools0 ))

# Generated files merge by regeneration: set the repo-local merge driver
# .gitattributes names. Best effort, a missing task never fails setup.
if PATH="${bin_dir}:${PATH}" command -v task >/dev/null 2>&1; then
  (cd "$repo" && PATH="${bin_dir}:${PATH}" task merge-drivers) >>"$log" 2>&1 || true
fi

# ─── record what the session side reads ─────────────────────────────────────
{
  echo "FATHOM_CLOUD_HOME=${cloud_home}"
  echo "STORE=${store}"
  echo "BIN_DIR=${bin_dir}"
  [ -n "$goroot" ] && [ -x "${goroot}/bin/go" ] && echo "GO_BIN=${goroot}/bin"
  [ -n "$fathom" ] && echo "FATHOM_BIN=${fathom}"
  [ -n "$indexed" ] && echo "INDEXED_ROOT=${indexed}"
  echo "SETUP_AT=$(date -u +%Y-%m-%dT%H:%M:%SZ)"
  echo "SETUP_COMMIT=$(git -C "$repo" rev-parse HEAD 2>/dev/null)"
} >"${cloud_home}/env"
if [ -z "${FATHOM_CLOUD_HOME:-}" ] && [ "$(id -u)" = 0 ] && [ -w /etc ]; then
  echo "FATHOM_CLOUD_HOME=${cloud_home}" >/etc/fathom-cloud.env # a session under another user finds the store
fi

if [ "${FATHOM_CLOUD_START:-}" = "1" ] && [ -n "$fathom" ]; then
  FATHOM_CLOUD_HOME="$cloud_home" bash "${cloud_home}/cloud-session.sh" ensure >>"$log" 2>&1
  rmdir "${cloud_home}/setup.lock" 2>/dev/null
fi

tools="task ${pin_TASK_VERSION:-?}, cue ${pin_CUE_VERSION:-?}, lefthook ${pin_LEFTHOOK_VERSION:-?}"
say "done in $(elapsed)s — ${go_have} (${t_go}s) · bin/fathom (${t_build}s) · index ${index} (${t_index}s) · ${tools} (${t_tools}s, beside the build) · mcp ${mcp} · failed:${failed:- none} · log ${log}"
exit 0
