#!/usr/bin/env bash
# Go-free cold-arm contract tests. Run by task scripts-check.
set -euo pipefail
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd -P)"
scratch="$(mktemp -d "${TMPDIR:-/tmp}/cold-arm-test.XXXXXX")"
trap 'rm -rf "$scratch"' EXIT
mkdir -p "$scratch/subject/docs/plans"
cat > "$scratch/subject/docs/plans/tiny.md" <<'PLAN'
# Tiny
- [x] Entry exists {id: T-1, symbol: Entry, package: .}
- [ ] Second {id: T-2, symbol: Second, package: .}
- [ ] Third {id: T-3, symbol: Third, package: .}
PLAN
printf 'docs/plans/tiny.md\n' > "$scratch/plans"
printf 'T-1\nT-2\nT-3\n' > "$scratch/ids"
git -C "$scratch/subject" init -q
git -C "$scratch/subject" add .
git -C "$scratch/subject" -c user.name=Test -c user.email=test@example.invalid -c core.hooksPath=/dev/null commit -qm fixture
cat > "$scratch/fathom" <<'FAKE'
#!/usr/bin/env python3
import json, os, pathlib, signal, sys
from http.server import BaseHTTPRequestHandler, HTTPServer
home = pathlib.Path(os.environ['FATHOM_HOME'])
assert str(home).endswith('/home')
assert 'FATHOM_TENANCY_BUDGET' not in os.environ
args = sys.argv[1:]
if args[0] == 'serve-http':
    if os.environ.get('COLD_ARM_TEST_EARLY_EXIT'): sys.exit(7)
    assert args[args.index('--addr') + 1] == '127.0.0.1:0'
    assert 'budget = "536870912B"' in (home / 'config.toml').read_text()
    class Handler(BaseHTTPRequestHandler):
        def log_message(self, *args): pass
        def do_GET(self):
            self.send_response(200); self.end_headers(); self.wfile.write(b'{}')
        def do_POST(self):
            req = json.loads(self.rfile.read(int(self.headers['Content-Length'])))
            method = req['method']
            name = req.get('params', {}).get('name')
            result = {'ok': True}
            if method == 'tools/list': result = {'tools': []}
            if name == 'symbol_context':
                result = {'truncated': True, 'elided_count': 1}
            if name == 'review_diff':
                result = {'refused': 'fixture refusal', 'admission': {'state': 'refuse'}}
            result = {'content': [{'type': 'text', 'text': json.dumps(result)}]} if name else result
            body = json.dumps({'jsonrpc': '2.0', 'id': req['id'], 'result': result}).encode()
            self.send_response(200); self.end_headers(); self.wfile.write(body)
    server = HTTPServer(('127.0.0.1', 0), Handler)
    record = home / 'daemon' / 'test' / 'record.json'
    record.parent.mkdir(parents=True)
    record.write_text(json.dumps({'addr': '127.0.0.1:%d' % server.server_port, 'pid': os.getpid()}))
    signal.signal(signal.SIGTERM, lambda *args: sys.exit(0))
    server.serve_forever()
elif args[0] == 'resources':
    print(json.dumps({'daemon': {'pid': int(os.environ.get('COLD_ARM_TEST_PID', '0'))}, 'budget': {'budget': 536870912, 'source': 'live daemon (/statusz)'}, 'footprint': {'now': 123456, 'max': 123999, 'samples': 1}, 'tenancy': {'level': 'normal'}}))
elif args[0] == 'version': print('fathom test')
elif args[0] == 'plans' and args[1] == 'context': print('Entry context')
else: print('{"ok":true}')
FAKE
chmod +x "$scratch/fathom"
# Inherited operator settings must not escape into the arm.
FATHOM_TENANCY_BUDGET=9GiB FATHOM_HOME="$scratch/operator" \
  bash "$SCRIPT_DIR/cold-arm.sh" --subject "$scratch/subject" --service fixture \
  --plans "$scratch/plans" --ids "$scratch/ids" --budget 536870912 \
  --out "$scratch/arm" --fathom "$scratch/fathom"
python3 - "$scratch/arm" <<'PY'
import json, pathlib, sys
out = pathlib.Path(sys.argv[1])
a = json.loads((out / 'arm.json').read_text())
assert list(a) == ['schema_version', 'identity', 'inputs', 'daemon', 'requests', 'physical_footprint', 'estimated_memory', 'outcome']
assert a['daemon']['exit_code'] == 0, a['daemon']
assert a['physical_footprint']['budget_matches_requested'] is True
assert a['physical_footprint']['peak_daemon_bytes'] == 123456
assert a['physical_footprint']['peak_with_children_bytes'] == 123999
r = a['requests']
assert [x['name'] for x in r] == ['ingest', 'verify:docs/plans/tiny.md', 'cli:symbol', 'cli:context', 'cli:plans', 'cli:obligations', 'initialize', 'tools/list', 'symbol_context', 'review_diff', 'obligations', 'verify_plan']
assert r[-4]['completeness'] == 'truncated', r[-4]
assert r[-3]['completeness'] == 'refused', r[-3]
assert r[-3]['admission'], r[-3]
assert a['outcome'] != 'complete'
for x in r:
    assert (out / x['response_file']).stat().st_size == x['response_bytes']
    assert x['argv'] and x['exit_code'] == 0
assert a['inputs']['ids'] == ['T-1', 'T-2', 'T-3']
assert a['identity']['subject']['dirty'] is False
assert r[-3]['request']['params']['arguments']['changed_files']
assert 'ctx: Entry' in (out / 'context.md').read_text()
print('cold-arm: order, isolation, envelopes, truncation, refusal, peaks and identity ok')
PY
# Reusing an arm must refuse before changing any evidence.
cp "$scratch/arm/arm.json" "$scratch/saved.json"
if bash "$SCRIPT_DIR/cold-arm.sh" --subject "$scratch/subject" --service fixture \
  --plans "$scratch/plans" --ids "$scratch/ids" --budget 536870912 \
  --out "$scratch/arm" --fathom "$scratch/fathom" 2>/dev/null; then
  echo 'cold-arm: reused output was accepted' >&2
  exit 1
fi
cmp "$scratch/arm/arm.json" "$scratch/saved.json"
# Invalid lists and budget refuse before launching anything.
printf 'MISSING\n' > "$scratch/bad-ids"
for invalid in ids budget; do
  ids="$scratch/ids"; budget=536870912
  if [ "$invalid" = ids ]; then ids="$scratch/bad-ids"; else budget=0; fi
  if bash "$SCRIPT_DIR/cold-arm.sh" --subject "$scratch/subject" --service fixture \
    --plans "$scratch/plans" --ids "$ids" --budget "$budget" \
    --out "$scratch/bad-$invalid" --fathom "$scratch/fathom" 2>/dev/null; then
    echo "cold-arm: invalid $invalid was accepted" >&2
    exit 1
  fi
done
# A daemon startup failure still produces a receipt and preserves its exit.
if COLD_ARM_TEST_EARLY_EXIT=1 bash "$SCRIPT_DIR/cold-arm.sh" \
  --subject "$scratch/subject" --service fixture --plans "$scratch/plans" \
  --ids "$scratch/ids" --budget 536870912 --out "$scratch/early-exit" \
  --fathom "$scratch/fathom" > "$scratch/early-exit.log" 2>&1; then
  echo 'cold-arm: daemon startup failure was accepted' >&2
  exit 1
fi
python3 - "$scratch/early-exit/arm.json" <<'PYEXIT'
import json,sys
a=json.load(open(sys.argv[1]))
assert a['daemon']['exit_code']==7
assert a['daemon']['harness_exit_code']!=0
assert a['outcome']=='incomplete' and a['requests']==[]
print('cold-arm: early daemon exit receipt ok')
PYEXIT
# No-sample maxima and config fallback are unknown, never zero physical use
# or proof of the budget a daemon ran under. Inject the reporting clock.
PYTHONDONTWRITEBYTECODE=1 python3 - "$SCRIPT_DIR" "$scratch/arm" <<'PYTEST'
import importlib.util, json, pathlib, sys
spec = importlib.util.spec_from_file_location('cold_arm', pathlib.Path(sys.argv[1]) / 'cold_arm.py')
m = importlib.util.module_from_spec(spec)
spec.loader.exec_module(m)
out = pathlib.Path(sys.argv[2])
for p in (out / 'resources').glob('*.stdout'):
    p.write_text(json.dumps({'footprint': {'now': -1, 'max': 0, 'samples': 0},
                            'budget': {'budget': 4294967296, 'source': 'resolved from config'}}))
(out / 'started.wall').write_text('10')
m.finish(out, 0, clock=lambda: 20)
a = json.loads((out / 'arm.json').read_text())
assert a['physical_footprint']['peak_daemon_bytes'] is None
assert a['physical_footprint']['peak_with_children_bytes'] is None
assert a['physical_footprint']['budget_observed_bytes'] == []
assert a['daemon']['arm_wall_seconds'] == 10
assert a['outcome'] == 'incomplete'
# Status fields with no omitted items do not imply truncation.
assert not m.facts({'elided': 0, 'truncated': False, 'complete': True})[1]
assert m.facts({'partial': True})[1]
assert m.facts({'partial': 'not computed (cold graph)'})[1]
assert m.facts({'snippet_truncated': True})[1]
assert m.facts({'suppressed': {'count': 2}})[1]
print('cold-arm: unknown physical samples and injected reporting clock ok')
PYTEST
echo 'cold-arm tests: ok'
