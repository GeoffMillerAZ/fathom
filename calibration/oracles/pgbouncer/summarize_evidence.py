"""Publish aggregate native evidence, never subject contents or file inventories."""
import argparse
import hashlib
import json
from collections import Counter
from pathlib import Path

if __name__ == '__main__':
    parser = argparse.ArgumentParser()
    parser.add_argument('-input', type=Path, required=True)
    parser.add_argument('-out', type=Path, required=True)
    args = parser.parse_args()
    raw = args.input.read_bytes()
    report = json.loads(raw)
    files = report['files']
    bindings = [ref for file in files for ref in file['references']]
    if any(file['membership_disagreements'] for file in files):
        raise SystemExit('native membership disagreement')
    if report.get('protocol') != 'r2' or not all(f['native_scanner_population_agrees'] for f in files):
        raise SystemExit('native population was not enumerated independently')
    if len(bindings) != report['references'] or sum(f['definitions'] for f in files) != report['definitions']:
        raise SystemExit('population count mismatch')
    summary = {k: report[k] for k in ('subject', 'version', 'binary_sha256', 'definitions', 'references', 'backend_sql_executed', 'backend_endpoint')}
    summary.update(configs=len(files), membership_disagreements=0, negative_controls=len(bindings),
                   protocol='r2', native_databases_probed=sum(len(f['native_sources']) for f in files),
                   native_outcomes=dict(sorted(Counter(r['outcome'] for f in files for r in f['native_sources']).items())),
                   native_scanner_population_agrees=True,
                   native_population_sha256=hashlib.sha256(json.dumps([f['native_sources'] for f in files], sort_keys=True).encode()).hexdigest(),
                   native_evidence_sha256=hashlib.sha256(raw).hexdigest(),
                   population_sha256=hashlib.sha256(json.dumps(files, sort_keys=True).encode()).hexdigest(),
                   native_error_sqlstates=sorted({r['native_error']['C'] for r in bindings}),
                   control_error_sqlstates=sorted({r['control_error']['C'] for r in bindings}))
    args.out.write_text(json.dumps(summary, indent=2) + '\n')
