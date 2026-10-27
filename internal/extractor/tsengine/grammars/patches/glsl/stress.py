"""Screen malformed, deep and large GLSL inputs in separate CLI processes.

Usage: python3 internal/extractor/tsengine/grammars/patches/glsl/stress.py <indexbench>
Build that CLI from the checkout being tested. Writes only under workspace/tmp/.
This records observed completion or contained refusal within 15 seconds per case;
it does not attribute a WASM trap to a particular cap or measure peak RSS.
"""
import hashlib
import json
from pathlib import Path
import subprocess
import sys
import time

root = Path('workspace/tmp/glsl-stress')
root.mkdir(parents=True, exist_ok=True)
cases = {
    'unterminated-layout': b'layout(' + b'x,' * 4096,
    'deep-expression': b'void main() { float x = ' + b'(' * 4096 + b'1.0' + b')' * 4096 + b'; }',
    'array-prefix-flood': b'void main() { float a[2]; float x = 0;\n' + b'x += a[1];\n' * 32768 + b'}',
    'two-mib-token-flood': (b'float x;\n' * ((2 * 1024 * 1024) // 9 + 1))[:2 * 1024 * 1024],
}
results = []
for name, source in cases.items():
    path = root / (name + '.frag')
    path.write_bytes(source)
    start = time.monotonic()
    try:
        run = subprocess.run([sys.argv[1], 'lang-probe', '-dialect', 'glsl', str(path)],
                             stdout=subprocess.PIPE, stderr=subprocess.PIPE, timeout=15)
        result = {'case': name, 'bytes': len(source), 'exit': run.returncode,
                  'wall_ms': round((time.monotonic() - start) * 1000),
                  'stdout_sha256': hashlib.sha256(run.stdout).hexdigest(),
                  'stderr_sha256': hashlib.sha256(run.stderr).hexdigest()}
        # Record contained parser refusal/trap separately; reject host crashes and timeouts.
        refused = b'parse exceeded' in run.stderr or b'budget' in run.stderr or b'malloc' in run.stderr
        trapped = b'wasm error:' in run.stderr
        result['bounded'] = (run.returncode == 0 or (run.returncode == 1 and (refused or trapped))) and b'panic:' not in run.stderr
        result['parse_refusal'] = run.returncode == 1 and refused
        result['wasm_trap'] = run.returncode == 1 and trapped

        if name == 'unterminated-layout':
            result['bounded'] = result['bounded'] and b'(ERROR' in run.stdout
    except subprocess.TimeoutExpired:
        result = {'case': name, 'bytes': len(source), 'wall_ms': 15000, 'bounded': False, 'timeout': True}
    results.append(result)
print(json.dumps(results, indent=2))
raise SystemExit(0 if all(row['bounded'] for row in results) else 1)
