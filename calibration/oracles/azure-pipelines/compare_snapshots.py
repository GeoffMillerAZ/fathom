"""Compare every existing YAML fixture's live baseline/candidate dispatch.

Equivalence only: no support-level or oracle-tier claim is derived from fathom.
"""
import argparse
import hashlib
import json
import subprocess
from pathlib import Path

if __name__ == '__main__':
    parser = argparse.ArgumentParser()
    parser.add_argument('-baseline', type=Path, required=True)
    parser.add_argument('-candidate', type=Path, required=True)
    parser.add_argument('-root', type=Path, default=Path('.'))
    parser.add_argument('-out', type=Path, required=True)
    args = parser.parse_args()
    args.out.parent.mkdir(parents=True, exist_ok=True)
    records = []
    for label, binary in (('baseline', args.baseline), ('candidate', args.candidate)):
        raw = subprocess.check_output([str(binary.resolve()), '-root', str(args.root)])
        (args.out.parent / (label + '-snapshot.json')).write_bytes(raw)
        records.append(json.loads(raw))
    before, after = ({x['path']: x for x in result} for result in records)
    if before.keys() != after.keys():
        raise SystemExit('fixture population changed')
    changes, new_azure, counts = [], [], {}
    for path, original in before.items():
        current = after[path]
        owner = path.split('/')[3]
        count = counts.setdefault(owner, {'files': 0, 'identical': 0, 'new_azure': 0})
        count['files'] += 1
        if original == current:
            count['identical'] += 1
        elif original.get('skipped', '') in ('unclassified', 'no-extractor') and current['tool_kind'] == 'azure-pipelines':
            count['new_azure'] += 1
            new_azure.append({'before': original, 'after': current})
        else:
            changes.append({'before': original, 'after': current})
    output = {'kind': 'regression equivalence only; not independent language truth',
              'fixture_files': len(before), 'by_fixture_dialect': counts,
              'changed_prior_claims': changes, 'newly_classified_azure': new_azure,
              'baseline_snapshot_sha256': hashlib.sha256(json.dumps(records[0], sort_keys=True).encode()).hexdigest(),
              'candidate_snapshot_sha256': hashlib.sha256(json.dumps(records[1], sort_keys=True).encode()).hexdigest()}
    args.out.write_text(json.dumps(output, indent=2) + '\n')
    if changes:
        raise SystemExit('earlier YAML extraction changed')
    print('YAML fixture population:', len(before), 'prior-owner changes:', len(changes), 'new Azure claims:', len(new_azure))
