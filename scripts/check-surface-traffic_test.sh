#!/usr/bin/env bash
#
# check-surface-traffic_test.sh — shell-level test for
# scripts/check-surface-traffic.sh (B2-1, docs/plans/falsifiability.md),
# following the scripts/vendor-check_test.sh precedent: build a throwaway
# fixture consumer under a temp dir (its own scripts/, workspace/logs/) so
# the script under test's relative-path resolution ($ROOT derived from
# BASH_SOURCE, workspace/logs/<surface>.stamp) never touches this repo's
# real workspace/ or the live fathom-http daemon.
#
# Proves the script's three verdict classes against a LOCAL FIXTURE HTTP
# server (never the live daemon):
#   1. ok             — surface_traffic has calls > 0, last_ts >= t0 stamp.
#   2. zero-traffic    — surface_traffic block present, but calls == 0.
#   3. missing-dependency — server_stats response has no "surface_traffic"
#      key at all (simulates a daemon predating A1-1).
# Plus one bonus case exercising the t0-stamp comparison itself (traffic
# exists but predates the recorded stamp -> still a FAIL, distinct message).
#
# Usage:
#   bash scripts/check-surface-traffic_test.sh

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

fail() {
  echo "FAIL: $1" >&2
  exit 1
}

pass=0
check_rc() {
  local desc="$1" got="$2" want="$3"
  if [ "${got}" != "${want}" ]; then
    fail "${desc}: expected exit ${want}, got ${got} (see ${TMP}/out.log)"
  fi
  pass=$((pass + 1))
  echo "ok — ${desc} (exit ${got})"
}

check_contains() {
  local desc="$1" file="$2" needle="$3"
  grep -qF "${needle}" "${file}" || fail "${desc}: expected output to contain '${needle}', got: $(cat "${file}")"
  pass=$((pass + 1))
  echo "ok — ${desc}"
}

TMP=$(mktemp -d)
cleanup() {
  [ -n "${FIXTURE_PID:-}" ] && kill "${FIXTURE_PID}" 2>/dev/null
  rm -rf "${TMP}"
}
trap cleanup EXIT

# ─── throwaway consumer layout (own scripts/ + workspace/logs/) ───────────
CONSUMER="${TMP}/consumer"
mkdir -p "${CONSUMER}/scripts" "${CONSUMER}/workspace/logs"
cp "${SCRIPT_DIR}/check-surface-traffic.sh" "${CONSUMER}/scripts/check-surface-traffic.sh"

# ─── fixture HTTP server: serves whatever JSON is currently in
# response.json for every POST, on a free localhost port ─────────────────
RESPONSE_FILE="${TMP}/response.json"

cat > "${TMP}/fixture_server.py" <<'PY'
import http.server
import socketserver
import sys

port = int(sys.argv[1])
response_file = sys.argv[2]


class Handler(http.server.BaseHTTPRequestHandler):
    def do_POST(self):
        length = int(self.headers.get("Content-Length", 0))
        self.rfile.read(length)
        with open(response_file, "rb") as f:
            body = f.read()
        self.send_response(200)
        self.send_header("Content-Type", "application/json")
        self.send_header("Content-Length", str(len(body)))
        self.end_headers()
        self.wfile.write(body)

    def log_message(self, format, *args):
        pass


class ReusableServer(socketserver.TCPServer):
    allow_reuse_address = True


with ReusableServer(("127.0.0.1", port), Handler) as httpd:
    httpd.serve_forever()
PY

PORT=$(python3 -c 'import socket; s = socket.socket(); s.bind(("127.0.0.1", 0)); print(s.getsockname()[1]); s.close()')
echo '{}' > "${RESPONSE_FILE}"
python3 "${TMP}/fixture_server.py" "${PORT}" "${RESPONSE_FILE}" &
FIXTURE_PID=$!

# Wait for the fixture to accept connections (bounded — no long sleeps).
for _ in $(seq 1 50); do
  curl -sS -m 1 -o /dev/null "http://127.0.0.1:${PORT}/rpc" 2>/dev/null && break
  sleep 0.1
done

export FATHOM_HTTP_URL="http://127.0.0.1:${PORT}"
run_check() {
  ( cd "${CONSUMER}" && bash scripts/check-surface-traffic.sh "$@" > "${TMP}/out.log" 2>&1 )
}

# ─── case 1: missing-dependency — no "surface_traffic" key at all ─────────
cat > "${RESPONSE_FILE}" <<'EOF'
{"jsonrpc":"2.0","id":1,"result":{"content":[{"type":"text","text":"{\"uptime_seconds\":5,\"request_count\":1}"}]}}
EOF
set +e
run_check agent
rc=$?
set -e
check_rc "missing-dependency verdict" "${rc}" "1"
check_contains "missing-dependency message" "${TMP}/out.log" "MISSING-DEPENDENCY"

# ─── case 2: zero-traffic — surface_traffic present, calls == 0 ───────────
cat > "${RESPONSE_FILE}" <<'EOF'
{"jsonrpc":"2.0","id":1,"result":{"content":[{"type":"text","text":"{\"surface_traffic\":{\"agent\":{\"calls\":0,\"last_ts\":\"\"}}}"}]}}
EOF
set +e
run_check agent
rc=$?
set -e
check_rc "zero-traffic verdict" "${rc}" "1"
check_contains "zero-traffic message" "${TMP}/out.log" "observed zero calls"

# ─── case 3: ok — calls > 0, no stamp file (falls back to ever-fired) ─────
cat > "${RESPONSE_FILE}" <<'EOF'
{"jsonrpc":"2.0","id":1,"result":{"content":[{"type":"text","text":"{\"surface_traffic\":{\"agent\":{\"calls\":7,\"last_ts\":\"2026-07-22T12:00:00Z\"}}}"}]}}
EOF
set +e
run_check agent
rc=$?
set -e
check_rc "ok verdict (no stamp file)" "${rc}" "0"
check_contains "ok message" "${TMP}/out.log" "OK — 'agent'"
check_contains "no-stamp warning fires" "${TMP}/out.log" "no stamp for 'agent'"

# ─── bonus: ok with a recorded per-surface t0 stamp the traffic postdates ─
echo "2026-07-01T00:00:00Z" > "${CONSUMER}/workspace/logs/agent.stamp"
set +e
run_check agent
rc=$?
set -e
check_rc "ok verdict (traffic after recorded stamp)" "${rc}" "0"

# ─── bonus: traffic exists but predates the recorded per-surface stamp ────
echo "2026-07-23T00:00:00Z" > "${CONSUMER}/workspace/logs/agent.stamp"
set +e
run_check agent
rc=$?
set -e
check_rc "fail verdict (traffic predates recorded stamp)" "${rc}" "1"
check_contains "predates-stamp message" "${TMP}/out.log" "none since its"
rm -f "${CONSUMER}/workspace/logs/agent.stamp"

echo ""
echo "check-surface-traffic_test.sh: ${pass}/${pass} checks passed"
echo ""
echo "NOTE (B2-1, docs/plans/falsifiability.md): this test proves the script's"
echo "verdict classes against ONLY a local fixture, never the live daemon. It"
echo "does not and cannot start the law-8 30-day adoption clock — that clock"
echo "starts only when workspace/logs/adoption-clock-t0.txt is written by an"
echo "operator or a qualifying non-fleet-repo session (docs/plans/activation.md,"
echo "docs/vision/11-dial-to-eleven.md law 8). No such file exists in this repo"
echo "as of this test run; the clock is explicitly NOT started."
