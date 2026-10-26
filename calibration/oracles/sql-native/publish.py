#!/usr/bin/env python3
"""Publish aggregate SQL receipts; keep raw subject paths and outputs in workspace."""
import argparse
from collections import Counter
import hashlib
import json
from pathlib import Path


def digest(value):
    return hashlib.sha256(json.dumps(value, sort_keys=True, separators=(',', ':')).encode()).hexdigest()


def summarize(data):
    result = {'publication': 'sql-aggregate-v1', 'raw_json_sha256': digest(data)}
    if isinstance(data.get('files'), list):
        files = data['files']
        receipts = [r for f in files for r in f['receipts']]
        result.update(version=data['version'], file_count=len(files),
                      source_bytes=sum(f['source_bytes'] for f in files),
                      file_manifest_sha256=digest([{k: f[k] for k in ('path', 'source_bytes', 'source_sha256')} for f in files]),
                      statement_count=len(receipts), statement_receipts_sha256=digest(receipts),
                      statement_outcomes=dict(sorted(Counter(('rejected' if 'error' in r else 'accepted') for r in receipts).items())),
                      create_table_failed=sum(f['create_table_failed'] for f in files),
                      excluded_catalog_keys=sum(f.get('excluded_catalog_keys', 0) for f in files),
                      pair_count=len(data['calls']), pairs_sha256=digest(data['calls']),
                      engines=dict(sorted(Counter(f['engine'] for f in files).items())),
                      errors=[{'source_path_sha256': hashlib.sha256(e['path'].encode()).hexdigest(),
                               'line': e['line'], 'kind': e['kind'], 'code': e['code']}
                              for f in files for e in f['errors']])
    elif 'calls' in data:
        result.update(tool=data['tool'], version=data['version'], tier=data['tier'],
                      pair_count=len(data['calls']), pairs_sha256=digest(data['calls']))
    elif 'records' in data:
        result.update(sqlite=data['sqlite'], file_count=data['files'], success=data['success'],
                      rejected=data['files']-data['success'], records_sha256=digest(data['records']))
    elif 'results' in data:
        result.update(file_count=len(data['results']),
                      success=sum(r['error'] is None for r in data['results']),
                      error_codes=dict(sorted(Counter(r['error']['code'] for r in data['results'] if r['error']).items())),
                      versions=sorted({r['version'] for r in data['results']}))
    elif 'sqlfluff' in data:
        result['sqlfluff'] = data['sqlfluff']
        for name in ('sqitch', 'pagila'):
            r = data[name]
            result[name] = {k: v for k, v in r.items() if k != 'pairs'}
            result[name].update(pair_count=len(r['pairs']), pairs_sha256=digest(r['pairs']))
    else:
        raise ValueError('unsupported raw receipt shape')
    return result


def main():
    ap = argparse.ArgumentParser(description=__doc__)
    ap.add_argument('--input', required=True, type=Path)
    ap.add_argument('--out', required=True, type=Path)
    args = ap.parse_args()
    args.out.mkdir(parents=True, exist_ok=True)
    for path in sorted(args.input.glob('*.json')):
        data = json.loads(path.read_text())
        if any(k in data for k in ('calls', 'files', 'results', 'records', 'sqlfluff')):
            (args.out / path.name).write_text(json.dumps(summarize(data), indent=2, sort_keys=True)+'\n')


if __name__ == '__main__':
    main()
