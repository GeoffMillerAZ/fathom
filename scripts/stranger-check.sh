#!/usr/bin/env bash
#
# stranger-check.sh — the v0.7.0 lesson as a task (RP-23,
# docs/plans/reach-parity.md): prove the adoption path works for someone
# who is NOT the author. Copy the tracked tree into scratch, build it
# with GOWORK=off (no sibling checkouts), start a throwaway daemon on a
# free port with a temp store, run `fathom init --harness=none --skills`
# against the copy with a temp HOME, then `fathom doctor`, and fail on any
# FAIL row. Nothing here touches the shared daemon, the shared store, or
# the real home directory.
#
# DIVERGENCE from the plan row's "workspace/tmp": the copy is a Go module
# tree, and `task validate` runs `go vet ./...` in PARALLEL with this
# script — a copy under the repo's own tree is walked by vet mid-rsync
# ("open workspace/tmp/stranger-check/tree/…: no such file or directory",
# 2026-09-02). Go toolchain scratch belongs OUTSIDE the module
# (Taskfile.yml's GOTMPDIR note), so the copy lives under TMPDIR.
#
# Run: task stranger-check   (prints the elapsed seconds; joins `task
# validate` only if that number stays under two minutes)
set -euo pipefail

# A git hook (lefthook's pre-push runs `task validate`, which runs this)
# exports GIT_DIR and friends to every child. Left in place they point
# every `git` call below at the PUSHING repository, so the scratch copy's
# `git init && git add && git commit` re-initialises the real checkout
# instead (O48: it reported "nothing to commit" against the worktree and
# once committed another editor's files as "initial commit"). Scrub them
# before the first git call; the copy must be its own repository.
unset GIT_DIR GIT_WORK_TREE GIT_INDEX_FILE GIT_PREFIX GIT_COMMON_DIR GIT_OBJECT_DIRECTORY GIT_ALTERNATE_OBJECT_DIRECTORIES LEFTHOOK

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"
# shellcheck source=scripts/lib/build-cache.sh
source "${SCRIPT_DIR}/lib/build-cache.sh"
fathom_cache_configure
cd "${REPO_ROOT}"

start=$SECONDS
# RT-25 (docs/design/responsible-tenant.md §10.3): a PRIVATE work dir per
# run, removed by the trap. The fixed ${TMPDIR}/fathom-stranger-check was
# shared by concurrent runs (one run's rm -rf pulled the tree from under
# another) and left 227 MB behind. FATHOM_STRANGER_KEEP=1 keeps it for a
# post-mortem and prints where.
work="$(mktemp -d "${TMPDIR:-/tmp}/fathom-stranger-check.XXXXXX")"
tree="${work}/tree"
home="${work}/home"
mkdir -p "${tree}" "${home}"

cleanup() {
  if [ -n "${daemon_pid:-}" ]; then
    kill "${daemon_pid}" 2>/dev/null || true
    wait "${daemon_pid}" 2>/dev/null || true
  fi
  if [ "${FATHOM_STRANGER_KEEP:-0}" = "1" ]; then
    echo "stranger-check: work dir kept at ${work}" >&2
  else
    rm -rf "${work}"
  fi
}
trap cleanup EXIT

# 1. The tracked tree, as a stranger receives it (working-tree content of
#    tracked paths, so the gate checks what would be committed), made into
#    its OWN repository with its own HEAD — a stranger's clone has one, and
#    the build below stamps that revision, which is what `binary_fresh`
#    compares against. Without it the copy would inherit THIS checkout's
#    HEAD and the row would report a false mismatch (O5's class).
git ls-files | while IFS= read -r f; do [ -e "$f" ] && printf '%s\n' "$f"; done > "${work}/.tracked"
rsync -a --files-from="${work}/.tracked" . "${tree}/"
rm -f "${work}/.tracked"
(
  cd "${tree}"
  git init -q
  git add .
  git -c user.name=stranger -c user.email=stranger@example.invalid commit -q -m "stranger-check: the tracked tree" --no-verify
) >"${work}/git.log" 2>&1 || {
  echo "stranger-check: FAIL could not make the copy its own repository:" >&2
  cat "${work}/git.log" >&2
  exit 1
}

# 2. Build standalone.
if ! (cd "${tree}" && GOWORK=off CGO_ENABLED=1 go build -o bin/fathom ./cmd/fathom >"${work}/build.log" 2>&1); then
  echo "stranger-check: FAIL the tracked tree does not build standalone:" >&2
  head -20 "${work}/build.log" >&2
  exit 1
fi
bin="${tree}/bin/fathom"

# 3. A throwaway daemon on a free port with a temp store.
port=$(python3 -c 'import socket; s=socket.socket(); s.bind(("127.0.0.1",0)); print(s.getsockname()[1]); s.close()')
url="http://127.0.0.1:${port}"
HOME="${home}" "${bin}" serve-http --store "${work}/store.sqlite" --addr "127.0.0.1:${port}" >"${work}/daemon.log" 2>&1 &
daemon_pid=$!
for _ in $(seq 1 50); do
  if curl -s --max-time 1 "${url}/healthz" 2>/dev/null | grep -q ok; then break; fi
  sleep 0.2
done
if ! curl -s --max-time 1 "${url}/healthz" 2>/dev/null | grep -q ok; then
  echo "stranger-check: FAIL the throwaway daemon did not come up:" >&2
  tail -20 "${work}/daemon.log" >&2
  exit 1
fi

# 4. init as a stranger: temp HOME, --harness=none (stanza + .fathom/ +
#    skills, no hook file), the copy's own binary.
init_rc=0
HOME="${home}" "${bin}" init --path "${tree}" --url "${url}" --bin "${bin}" --harness=none --skills --service stranger >"${work}/init.log" 2>&1 || init_rc=$?
if [ "${init_rc}" -ne 0 ]; then
  # init prints dozens of routine "ok …" and "MANUAL: …" lines last, so a
  # plain tail once showed only those and hid the line that failed (a
  # 2026-09-25 push refusal whose cause the log never named). Print the
  # non-routine lines first, then the tail.
  echo "stranger-check: FAIL fathom init exited ${init_rc}; its non-routine output:" >&2
  grep -vE '^(ok |MANUAL:|shim installed:|[[:space:]])' "${work}/init.log" | tail -30 >&2 || true
  echo "stranger-check: last 10 lines of init's output:" >&2
  tail -10 "${work}/init.log" >&2
  exit 1
fi

# 5. One real tool call against /rpc/agent, the way a stranger's first
#    session makes one — this is what proves the surface answers, and it
#    is what doctor's surface_traffic row counts.
if ! curl -s --max-time 5 -H 'Content-Type: application/json' \
      -d '{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"capabilities","arguments":{}}}' \
      "${url}/rpc/agent" | grep -q '"result"'; then
  echo "stranger-check: FAIL a tools/call against ${url}/rpc/agent returned no result" >&2
  exit 1
fi

# 6. doctor: any FAIL row fails the check, with ONE named exception:
#    obligations_served fails honestly on a repo whose obligations layer
#    has never been compiled, and its remedy (`fathom compile`) spends LLM
#    budget — outside this gate's scope (docs/intent/autonomy.md). It is
#    printed as expected, never hidden. Rows doctor cannot prove are SKIP
#    with a reason, which is the contract — never a guessed PASS.
HOME="${home}" "${bin}" doctor --path "${tree}" --url "${url}" --harness agents --service stranger --json >"${work}/doctor.json" 2>"${work}/doctor.err" || true
fails=$(python3 - "${work}/doctor.json" "${work}/doctor.fails" <<'PY'
import json, sys
d = json.load(open(sys.argv[1]))
rows = d.get("checks", d if isinstance(d, list) else [])
expected = {"obligations_served"}
bad = [r for r in rows if str(r.get("status", "")).lower() == "fail" and r.get("name") not in expected]
for r in rows:
    if str(r.get("status", "")).lower() == "fail" and r.get("name") in expected:
        print(f"  expected for a never-compiled repo: {r.get('name')} — {r.get('reason')}")
for r in bad:
    print(f"  FAIL {r.get('name')}: {r.get('reason')} (remedy: {r.get('remedy')})")
open(sys.argv[2], "w").write(str(len(bad)))
PY
)
nfail=$(cat "${work}/doctor.fails")
elapsed=$((SECONDS - start))
if [ "${nfail}" != "0" ]; then
  echo "stranger-check: FAIL — ${nfail} doctor row(s) FAIL for a stranger (${elapsed}s):" >&2
  echo "${fails}" >&2
  exit 1
fi
[ -n "${fails}" ] && echo "${fails}"
echo "stranger-check: the tracked tree builds standalone, inits with --harness=none --skills, answers a tools/call on /rpc/agent, and doctor reports no unexpected FAIL row (${elapsed}s)"
