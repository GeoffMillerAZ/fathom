#!/usr/bin/env python3
"""Journal the single ESCALATE-19 replacement without changing the old receipt."""
import argparse
import hashlib
import json
from pathlib import Path
import subprocess

READER = '884c77754d17a8f09129d3fd5b0955bef0db9a81'


def sha(path):
    return hashlib.sha256(path.read_bytes()).hexdigest()


def begin(root, receipt):
    # Only harness and record changes may differ from the original reader.
    changed = subprocess.check_output(['git', 'diff', '--name-only', READER], text=True).splitlines()
    for name in changed:
        allowed = (name.startswith('calibration/oracles/dep-coverage/')
                   or name.startswith('pkg/lang/deps/cov_') and name.endswith('_test.go')
                   or name.startswith('pkg/lang/deps/internal/depbench/cov_') and name.endswith('.go')
                   or name in ('docs/bench/readings/dependency-edges/cov-census.md', 'docs/plans/dependency-edges.md'))
        if not allowed:
            raise ValueError('reader revision changed')
    if (root / 'reading').exists():
        raise ValueError('replacement root already has a campaign')
    metadata = {
        'authority': 'ESCALATE-19', 'campaign': 'replacement-baseline',
        'reader_revision': READER,
        'harness_revision': subprocess.check_output(['git', 'rev-parse', 'HEAD'], text=True).strip(),
        'subjects_sha256': sha(root / 'subjects.json'),
        'aborted_receipt_sha256': sha(receipt),
        'aborted_campaign': 'cov-census',
        'status': 'authorized',
    }
    with (root / 'campaign.json').open('x') as out:
        json.dump(metadata, out, sort_keys=True, separators=(',', ':'))
        out.write('\n')
    print('ESCALATE-19 replacement journal created; prior receipt retained')


def finish(root, receipt):
    path = root / 'campaign.json'
    metadata = json.loads(path.read_text())
    if metadata['status'] != 'authorized':
        raise ValueError('replacement already finished')
    success = (root / 'reading/aggregate.json').exists() and '\nPASS\n' in receipt.read_text() and not any(line.startswith('FAIL') for line in receipt.read_text().splitlines())
    metadata.update(status='completed' if success else 'failed',
                    replacement_receipt_sha256=sha(receipt))
    if metadata['status'] == 'completed':
        metadata['aggregate_sha256'] = sha(root / 'reading/aggregate.json')
    path.write_text(json.dumps(metadata, sort_keys=True, separators=(',', ':')) + '\n')
    print('ESCALATE-19 replacement journal finalized: ' + metadata['status'])


if __name__ == '__main__':
    ap = argparse.ArgumentParser(description=__doc__)
    ap.add_argument('action', choices=['begin', 'finish'])
    ap.add_argument('root', type=Path)
    ap.add_argument('receipt', type=Path)
    args = ap.parse_args()
    try:
        (begin if args.action == 'begin' else finish)(args.root, args.receipt)
    except Exception:
        raise SystemExit('COV campaign journal failed; no private diagnostics emitted')
