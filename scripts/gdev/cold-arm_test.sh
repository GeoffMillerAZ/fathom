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
            if name == 'ingest' and os.environ.get('COLD_ARM_TEST_DEGRADED'):
                result = {'ok': True, 'degraded': True}
            if name == 'symbol_context' and not os.environ.get('COLD_ARM_TEST_CLEAN'):
                result = {'truncated': True, 'elided_count': 1}
            if name == 'review_diff' and not os.environ.get('COLD_ARM_TEST_CLEAN'):
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
elif args[0] == 'plans' and args[1] == 'context':
    if os.environ.get('COLD_ARM_TEST_MUTATE'): pathlib.Path(os.environ['COLD_ARM_TEST_MUTATE']).write_text('changed')
    print('Entry context')
else: print('{"ok":true}')
FAKE
chmod +x "$scratch/fathom"
arm_code=0
# Inherited operator settings must not escape into the arm.
FATHOM_TENANCY_BUDGET=9GiB FATHOM_HOME="$scratch/operator" \
  bash "$SCRIPT_DIR/cold-arm.sh" --subject "$scratch/subject" --service fixture \
  --plans "$scratch/plans" --ids "$scratch/ids" --budget 536870912 \
  --out "$scratch/arm" --fathom "$scratch/fathom" || arm_code=$?
[ "$arm_code" -eq 3 ] || { echo "incomplete arm exited $arm_code, want 3"; exit 1; }
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
    assert x['request'] is not None
    if x['name'].startswith(('cli:', 'verify:')):
        assert x['request'] == {'argv': x['argv']}
assert a['identity']['subject']['post_arm_status_unchanged'] is True
assert a['inputs']['ids'] == ['T-1', 'T-2', 'T-3']
assert a['identity']['subject']['dirty'] is False
assert r[-3]['request']['params']['arguments']['changed_files']
assert 'ctx: Entry' in (out / 'context.md').read_text()
print('cold-arm: order, isolation, envelopes, truncation, refusal, peaks and identity ok')
PY
# Clean controls isolate degraded ingest and subject mutation outcomes.
for scenario in clean degraded changed; do
  code=0
  if [ "$scenario" = changed ]; then
    # Mutate from a CLI control after the pre-arm identity was captured.
    export COLD_ARM_TEST_MUTATE="$scratch/subject/new.txt"
  fi
  if [ "$scenario" = degraded ]; then export COLD_ARM_TEST_DEGRADED=1; fi
  COLD_ARM_TEST_CLEAN=1 bash "$SCRIPT_DIR/cold-arm.sh" \
    --subject "$scratch/subject" --service fixture --plans "$scratch/plans" \
    --ids "$scratch/ids" --budget 536870912 --out "$scratch/$scenario" \
    --fathom "$scratch/fathom" || code=$?
  if [ "$scenario" = clean ]; then [ "$code" -eq 0 ]; else [ "$code" -eq 3 ]; fi
  python3 - "$scratch/$scenario/arm.json" "$scenario" <<'PYSCENARIO'
import json, sys
a = json.load(open(sys.argv[1]))
scenario = sys.argv[2]
assert a['outcome'] == ('complete' if scenario == 'clean' else 'incomplete')
assert a['identity']['subject']['post_arm_status_unchanged'] == (scenario != 'changed')
if scenario == 'degraded': assert a['requests'][0]['completeness'] != 'complete'
PYSCENARIO
  unset COLD_ARM_TEST_MUTATE COLD_ARM_TEST_DEGRADED
done
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
# Exercise real sync and async ingest envelopes, plus unknown-key diagnostics.
import contextlib, io
probe = out / 'ingest-probe.json'
ledger = out / 'requests.jsonl'
saved = ledger.read_text()
for value, expected in [({'ok': True}, 'done'), ({'ok': False}, 'failed'),
    ({'completed': False, 'state': 'queued', 'run': 'r1'}, 'r1'),
    ({'completed': False, 'state': 'running', 'run': 'r1'}, 'r1'),
    ({'completed': True, 'state': 'done', 'ok': True, 'run': 'r1'}, 'done'),
    ({'completed': True, 'state': 'done', 'ok': False, 'run': 'r1'}, 'failed'),
    ({'completed': False, 'state': 'unknown', 'run': 'r1'}, 'unknown'),
    ({'novel': True}, 'unknown'), ({'completed': False, 'state': 'queued'}, 'unknown')]:
    probe.write_text(json.dumps({'result': {'structuredContent': value}}))
    ledger.write_text(json.dumps({'response_file': probe.name, 'completeness': 'complete'}) + '\n')
    diagnostics = io.StringIO()
    with contextlib.redirect_stderr(diagnostics): assert m.ingest_state(out) == expected, value
    if expected == 'unknown': assert 'keys=' in diagnostics.getvalue()
ledger.write_text(saved)
assert m.facts({'degraded': True})[1]
assert m.facts({'degradation': {'reason': 'partial'}})[1]
assert m.facts({'outcome': 'degraded'})[0]
assert m.facts({'status': 'degradation'})[0]
assert not m.facts({'degraded': False})[1]
for text, expected in [('unresolved references in context', 'complete'),
    ('elided', 'truncated'), ('truncated', 'truncated'),
    ('refresh refused (HTTP 503): pressure', 'refused'), ('refusal: pressure', 'refused'),
    ('fathom busy: pressure', 'refused'), ('over budget: footprint', 'refused'),
    ('halted in-flight ingest-class work', 'refused'), ('admission closed', 'refused')]:
    result = m.record(out, 'cli:context', 'text', '-', [sys.executable, '-c', 'print(' + repr(text) + ')'], clock=lambda: 0)
    assert result['completeness'] == expected, result
result = m.record(out, 'verify:fixture', 'json', '-', [sys.executable, '-c', 'print("{}"); raise SystemExit(1)'], clock=lambda: 0)
assert result['completeness'] == 'error'
print('cold-arm: ingest shapes, text refusals, argv requests and failed verify ok')
print('cold-arm: unknown physical samples and injected reporting clock ok')
PYTEST
echo 'cold-arm tests: ok'
