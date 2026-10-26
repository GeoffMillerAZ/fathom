"""Run every canonical config and attest every local auth_dbname binding natively."""
import argparse
import hashlib
import json
import subprocess
from pathlib import Path
from native_authdb import Instance, mutate_authdb, enumerate_bindings
from pgbouncer_oracle import claimed, scan, entities, SKIP_DIRS

HERE = Path(__file__).resolve().parent

if __name__ == '__main__':
    parser = argparse.ArgumentParser()
    parser.add_argument('-subject', type=Path, required=True)
    parser.add_argument('-pgbouncer', type=Path, required=True)
    parser.add_argument('-build-receipt', type=Path, required=True)
    parser.add_argument('-work', type=Path, required=True)
    parser.add_argument('-out-dir', type=Path, required=True)
    args = parser.parse_args()
    protocol = json.loads((HERE / 'native-protocol-r2.json').read_text())
    receipt = json.loads(args.build_receipt.read_text())
    sha = subprocess.check_output(['git', '-C', str(args.subject), 'rev-parse', 'HEAD'], text=True).strip()
    if sha != protocol['subject']['sha'] or subprocess.check_output(['git', '-C', str(args.subject), 'status', '--porcelain'], text=True).strip():
        raise SystemExit('subject identity/cleanliness mismatch')
    version = subprocess.check_output([str(args.pgbouncer.resolve()), '-V'], text=True).strip().splitlines()[0]
    if version != protocol['version'] or receipt['version'] != version or hashlib.sha256(args.pgbouncer.read_bytes()).hexdigest() != receipt['binary_sha256']:
        raise SystemExit('native binary identity mismatch')
    definitions, references, audit = [], [], []
    for file in sorted(args.subject.rglob('*.ini')):
        rel = file.relative_to(args.subject).as_posix()
        if any(p in SKIP_DIRS for p in file.relative_to(args.subject).parts):
            continue
        source = file.read_text()
        if not claimed(rel, source):
            continue
        sections = scan(source)
        with Instance(args.pgbouncer, source, args.work / str(len(audit))) as instance:
            membership = instance.membership()
            defs, refs, disagreements = entities(rel, sections, membership)
            if disagreements:
                raise RuntimeError('native membership mismatch: ' + json.dumps(disagreements))
            definitions.extend(defs)
            if '*' in membership[0]:
                raise RuntimeError('wildcard database population needs an additional native protocol')
            native_pairs, enumeration = enumerate_bindings(instance)
            native_pairs = [dict(ref, path=rel) for ref in native_pairs]
            pairs = lambda values: {(r['from'], r['to']) for r in values}
            if pairs(native_pairs) != pairs(refs):
                raise RuntimeError('native/scanner population disagreement; preserve raw native enumeration')
            evidence = []
            for ref in native_pairs:
                error = instance.disabled_binding(ref['from'], ref['to'])
                missing = 'zz_oracle_missing_authdb'
                if '*' in membership[0] or missing in membership[0] or ref['from'] == ref['to']:
                    raise RuntimeError('negative control needs a separate protocol')
                mutated = mutate_authdb(source, ref['from'], ref['to'], missing)
                with Instance(args.pgbouncer, mutated, args.work / ('control-' + str(len(audit)) + '-' + str(len(evidence)))) as control:
                    missing_error = control.probe(ref['from'])
                    native_message = 'authentication database "' + missing + '" is not configured.'
                    log = (control.work / 'native.log').read_text()
                    if missing_error.get('M') != 'bouncer config error' or native_message not in log:
                        raise RuntimeError('native negative control did not distinguish the target')
                evidence.append(dict(ref, native_error=error, control_error=missing_error, control_log_message=native_message))
                references.append(ref)
            audit.append({'path': rel, 'source_sha256': hashlib.sha256(source.encode()).hexdigest(),
                          'definitions': len(defs), 'references': evidence, 'membership_disagreements': disagreements,
                          'native_sources': enumeration, 'native_scanner_population_agrees': True})
    if not audit or not references:
        raise SystemExit('empty native population')
    args.out_dir.mkdir(parents=True, exist_ok=True)
    for name, tier, defs, refs, tool in (
            ('defs', 'C', definitions, [], 'PgBouncer console membership and independent source positions'),
            ('refs', 'A', [], references, 'PgBouncer native SHOW DATABASES population and exhaustive auth_dbname startup enumeration')):
        (args.out_dir / (name + '.json')).write_text(json.dumps(dict(tool=tool, version=version, tier=tier, definitions=defs, references=refs), indent=2) + '\n')
    report = {'subject': protocol['subject'], 'version': version, 'binary_sha256': receipt['binary_sha256'], 'protocol': 'r2',
              'files': audit, 'definitions': len(definitions), 'references': len(references),
              'backend_sql_executed': False, 'backend_endpoint': 'nonexistent private local socket directory'}
    (args.out_dir / 'native-evidence.json').write_text(json.dumps(report, indent=2) + '\n')
    print('native population', len(audit), 'configs', len(definitions), 'definitions', len(references), 'references')
