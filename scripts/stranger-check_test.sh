#!/usr/bin/env bash
#
# stranger-check_test.sh — shell-level test for scripts/stranger-check.sh
# (RP-23, O1339).
#
# Builds a throwaway repository laid out like fathom (go.mod naming the
# fathom module, cmd/fathom, the fauxtown fixture fleet, and a bulk of other
# tracked files standing in for the rest of the tree) and runs the real
# stranger-check.sh against it with `go` and the built `fathom` replaced by
# stubs. The stub fathom's init confirms an index only when the tree it
# wires fits a fixed amount of work: a first index's time grows with the
# tree, and on a loaded host the whole repository outlasted init's wait
# (O1339), so a check that wires the whole tree is not deterministic.
#
# Asserts: the check wires a stranger's repository (the fixture fleet, its
# own git repository) rather than the whole tracked tree; doctor reads that
# repository; binary_fresh is still proven against the copied tree; and the
# check fails, never skips, when init does not confirm the index, when the
# first tool call does not find the stranger's code, or when the copy's
# build does not stamp its own HEAD.
#
# Usage:
#   bash scripts/stranger-check_test.sh

set -euo pipefail
unset GIT_DIR GIT_WORK_TREE GIT_INDEX_FILE GIT_PREFIX GIT_COMMON_DIR GIT_OBJECT_DIRECTORY GIT_ALTERNATE_OBJECT_DIRECTORIES LEFTHOOK
# The build-cache helper must not trim or redirect a real cache from here.
unset FATHOM_CACHE_ROOT FATHOM_CACHE_CONFIGURED FATHOM_STRANGER_READING FATHOM_STRANGER_KEEP FATHOM_STRANGER_WIRE

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

fail() {
  echo "FAIL: $1" >&2
  exit 1
}

pass=0
ok() {
  pass=$((pass + 1))
  echo "ok — $1"
}

# A template under TMPDIR: macOS mktemp with none ignores TMPDIR.
TMP=$(mktemp -d "${TMPDIR:-/tmp}/stranger-check-test.XXXXXX")
trap 'chmod -R u+w "${TMP}" 2>/dev/null || true; rm -rf "${TMP}"' EXIT

# --- the repository a stranger clones ---------------------------------------
R="${TMP}/repo"
FX="internal/connector/testdata/fauxtown"
mkdir -p "${R}/scripts/lib" "${R}/cmd/fathom" "${R}/bulk" \
  "${R}/${FX}/checkout-service" "${R}/${FX}/polyglot-service" "${R}/${FX}/orders-api"
cp "${SCRIPT_DIR}/stranger-check.sh" "${R}/scripts/stranger-check.sh"
cp "${SCRIPT_DIR}/lib/build-cache.sh" "${R}/scripts/lib/build-cache.sh"
printf 'module github.com/geoffmilleraz/fathom\n\ngo 1.26\n' >"${R}/go.mod"
printf 'package main\n\nfunc main() {}\n' >"${R}/cmd/fathom/main.go"
printf 'package checkout\n\nfunc FetchItem() {}\n' >"${R}/${FX}/checkout-service/checkout.go"
printf 'module github.com/example/polyglot\n\ngo 1.21\n' >"${R}/${FX}/polyglot-service/go.mod"
printf 'def handler():\n    pass\n' >"${R}/${FX}/polyglot-service/app.py"
printf 'package main\n\nfunc main() {}\n' >"${R}/${FX}/orders-api/main.go"
fleet_files=4
# The rest of the tree: far more work than one first index may take here.
for i in $(seq 1 300); do printf 'tracked file %d\n' "$i" >"${R}/bulk/f${i}.txt"; done
(cd "${R}" && git init -q && git add . && git -c user.name=t -c user.email=t@example.invalid commit -q -m fixture --no-verify)

# --- stubs: go and the fathom binary it "builds" -----------------------------
S="${TMP}/stubs"
STATE="${TMP}/state"
mkdir -p "${S}" "${STATE}"

cat >"${S}/fathom-stub" <<'STUB'
#!/usr/bin/env bash
# The stub fathom: records every call; init confirms an index only for a
# tree that fits STUB_CAPACITY tracked files (a first index's work).
set -uo pipefail
echo "$(pwd) $*" >>"${STUB_STATE}/calls.log"
cmd="$1"; shift
arg() { local want="$1"; shift; while [ $# -gt 0 ]; do [ "$1" = "$want" ] && { echo "$2"; return; }; shift; done; }
case "$cmd" in
  serve-http)
    addr=$(arg --addr "$@")
    exec python3 - "${addr##*:}" "${STUB_STATE}" <<'PY'
import http.server, json, os, re, sys
port, state = int(sys.argv[1]), sys.argv[2]
class H(http.server.BaseHTTPRequestHandler):
    def log_message(self, *a): pass
    def _send(self, body):
        b = body.encode()
        self.send_response(200); self.send_header("Content-Length", str(len(b))); self.end_headers(); self.wfile.write(b)
    def do_GET(self):
        self._send("ok" if self.path == "/healthz" else "")
    def do_POST(self):
        req = json.loads(self.rfile.read(int(self.headers["Content-Length"])))
        name = req["params"]["arguments"].get("name", "")
        hits = []
        try:
            root = open(os.path.join(state, "indexed")).read().strip()
        except OSError:
            root = ""
        if root and os.environ.get("STUB_EMPTY_INDEX") != "1":
            for d, _, fs in os.walk(root):
                if "/.git" in d: continue
                for f in fs:
                    p = os.path.join(d, f)
                    try:
                        if re.search(r"^func " + re.escape(name) + r"\b", open(p).read(), re.M):
                            hits.append(os.path.relpath(p, root))
                    except (OSError, UnicodeDecodeError):
                        pass
        text = json.dumps({"matches": sorted(hits)})
        self._send(json.dumps({"jsonrpc": "2.0", "id": req.get("id"), "result": {"content": [{"type": "text", "text": text}]}}))
http.server.HTTPServer(("127.0.0.1", port), H).serve_forever()
PY
    ;;
  init)
    path=$(arg --path "$@")
    echo "${path}" >"${STUB_STATE}/init.path"
    n=$(git -C "${path}" ls-files | wc -l | tr -d ' ')
    echo "${n}" >"${STUB_STATE}/init.files"
    # The check removes its work dir on exit: record now whether the path
    # is the top of its own git repository.
    if [ "$(git -C "${path}" rev-parse --show-toplevel 2>/dev/null)" = "$(cd "${path}" && pwd -P)" ]; then
      echo own >"${STUB_STATE}/init.repo"
    else
      echo not-own >"${STUB_STATE}/init.repo"
    fi
    printf '# AGENTS.md\n<!-- fathom stanza -->\n' >"${path}/AGENTS.md"
    if [ "${STUB_INIT_RC:-0}" != "0" ] || [ "${n}" -gt "${STUB_CAPACITY}" ]; then
      echo "error: exiting 2 — service \"stranger\" did not confirm as indexed; wiring finished, only the index is unconfirmed" >&2
      exit 2
    fi
    echo "${path}" >"${STUB_STATE}/indexed"
    echo "ok indexed ${n} files"
    ;;
  ingest | verify) exit 0 ;;
  doctor)
    path=$(arg --path "$@")
    echo "${path}" >>"${STUB_STATE}/doctor.paths"
    stamp=$(cat "${STUB_STATE}/stamp")
    [ "${STUB_BAD_STAMP:-0}" = "1" ] && stamp=0000000
    if grep -q '^module github.com/geoffmilleraz/fathom$' "${path}/go.mod" 2>/dev/null; then
      head=$(git -C "${path}" rev-parse HEAD)
      if [ "${head}" = "${stamp}" ]; then fresh=pass; else fresh=fail; fi
    else
      fresh=skip
    fi
    stanza=fail; [ -f "${path}/AGENTS.md" ] && stanza=pass
    printf '{"checks":[{"name":"binary_fresh","status":"%s","reason":"stub"},{"name":"stanza_present","status":"%s","reason":"stub"}]}\n' "${fresh}" "${stanza}"
    ;;
  *) exit 0 ;;
esac
STUB
chmod +x "${S}/fathom-stub"

cat >"${S}/go" <<'STUB'
#!/usr/bin/env bash
# The stub go: `go build -o OUT ./cmd/fathom` installs the stub fathom and
# stamps the HEAD of the tree it was run in, as -buildvcs does.
set -euo pipefail
if [ "$1" = "build" ]; then
  echo "$*" >"${STUB_STATE}/build.args"
  out=""; while [ $# -gt 0 ]; do [ "$1" = "-o" ] && out="$2"; shift; done
  mkdir -p "$(dirname "${out}")"
  cp "${STUB_DIR}/fathom-stub" "${out}"
  git rev-parse HEAD >"${STUB_STATE}/stamp"
  exit 0
fi
[ "$1 ${2:-}" = "env GOMODCACHE" ] && { echo "${STUB_TMP}/modcache"; exit 0; }
exit 0
STUB
chmod +x "${S}/go"

run_check() {
  rm -f "${STATE}"/*
  (
    cd "${R}"
    PATH="${S}:${PATH}" STUB_DIR="${S}" STUB_STATE="${STATE}" STUB_TMP="${TMP}" \
      STUB_CAPACITY="${STUB_CAPACITY:-50}" GOMODCACHE="${TMP}/modcache" TMPDIR="${TMP}" \
      bash scripts/stranger-check.sh
  ) >"${TMP}/out.log" 2>&1
}

# --- the check passes, wiring a stranger's repository ------------------------
run_check || { cat "${TMP}/out.log" >&2; fail "stranger-check failed against a repository whose whole tree outlasts one first index (init wired $(cat "${STATE}/init.files" 2>/dev/null || echo '?') tracked files)"; }
ok "the check passes when the whole tree is more work than a first index may take"

grep -qw -- '-trimpath' "${STATE}/build.args" || fail "the copy was built without -trimpath: $(cat "${STATE}/build.args")"
ok "the copy is built with -trimpath (machine-independent output and recorded paths)"

init_path=$(cat "${STATE}/init.path")
[ "$(cat "${STATE}/init.files")" = "${fleet_files}" ] ||
  fail "init wired ${init_path} with $(cat "${STATE}/init.files") tracked files; want the ${fleet_files}-file fixture fleet, not the whole tree"
[ "$(cat "${STATE}/init.repo")" = own ] ||
  fail "the stranger's repository ${init_path} is not its own git repository"
ok "init wires the fixture fleet as its own git repository (${fleet_files} files)"

grep -qxF "${init_path}" "${STATE}/doctor.paths" || fail "doctor never read the stranger's repository ${init_path}"
grep -q '/tree$' "${STATE}/doctor.paths" || fail "binary_fresh was not read against the copied tree: $(tr '\n' ' ' <"${STATE}/doctor.paths")"
ok "doctor reads the stranger's repository, and binary_fresh the copied tree"

# --- never weakened ----------------------------------------------------------
STUB_INIT_RC=2 run_check && fail "the check passed although init did not confirm the index"
grep -q 'did not confirm as indexed' "${TMP}/out.log" || fail "an unconfirmed index failed without init's own reason"
ok "an unconfirmed first index fails the check"

STUB_EMPTY_INDEX=1 run_check && fail "the check passed although the first tool call found none of the stranger's code"
ok "a first tool call that finds none of the stranger's code fails the check"

STUB_BAD_STAMP=1 run_check && fail "the check passed although the copy's build does not stamp its own HEAD"
grep -q 'binary_fresh' "${TMP}/out.log" || fail "a stale build stamp failed without naming binary_fresh"
ok "a build that does not stamp the copy's HEAD fails the check, naming binary_fresh"

# --- the opt-in whole-tree instrument -----------------------------------------
FATHOM_STRANGER_WIRE=tree STUB_CAPACITY=100000 run_check || { cat "${TMP}/out.log" >&2; fail "FATHOM_STRANGER_WIRE=tree failed"; }
grep -q '/tree$' "${STATE}/init.path" || fail "FATHOM_STRANGER_WIRE=tree wired $(cat "${STATE}/init.path"), not the copied tree"
ok "FATHOM_STRANGER_WIRE=tree wires the whole copied tree (the cold-ingest reading's instrument)"

echo "stranger-check_test: ${pass} passed"
