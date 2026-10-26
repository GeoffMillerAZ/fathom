#!/usr/bin/env python3
"""Publish only aggregate PHP candidate counts and hashes, never source or file lists."""
import argparse
import hashlib
import json
from pathlib import Path
import re

from fetch import SOURCES


def digest(path):
    return hashlib.sha256(path.read_bytes()).hexdigest()


def test_counts(text):
    lines = re.findall(r'^Tests: .+$', text, re.MULTILINE)
    if not lines:
        return {'completed_summary': False}
    values = dict(re.findall(r'([A-Za-z ]+): ([\d,]+)', lines[-1]))
    return {'completed_summary': True, **{k.strip().lower().replace(' ', '_'): int(v.replace(',', ''))
                                         for k, v in values.items()}}


def main():
    ap = argparse.ArgumentParser(description=__doc__)
    ap.add_argument('--measurement', required=True, type=Path)
    ap.add_argument('--traces', required=True, action='append', type=Path)
    ap.add_argument('--out', required=True, type=Path)
    args = ap.parse_args()
    raw = json.loads((args.measurement/'summary.json').read_text())
    public = {'ruling': 'OL-R9', 'canonical_reading_changed': False,
              'subjects': {name: {'repository': SOURCES[name][0], 'revision': SOURCES[name][1], 'original_php_files': n}
                           for name, n in [('guzzle', 74), ('monolog', 217)]},
              'scip_candidates': {}, 'runtime_environments': {}}
    for name, candidate in raw['candidates'].items():
        subjects = {}
        for subject, r in candidate['subjects'].items():
            subjects[subject] = {'full_scope_audit': r['full_scope_audit'],
                                 'within_index': {k: r['metrics'][k] for k in ['call_precision', 'call_recall']},
                                 'within_index_counts': {k: r['counts'].get(k, 0) for k in [
                                     'call_truth', 'call_truth_found', 'call_fathom_in_truth', 'calls_oracle_blind']},
                                 'scip_sha256': r['scip_sha256']}
        public['scip_candidates'][name] = {'revision': candidate['revision'],
                                           'tier': candidate['tier_under_OL_R9'], 'subjects': subjects}
    for traces in args.traces:
        environment = {}
        for name in ['guzzle', 'monolog']:
            out = traces/name
            r = json.loads((out/'summary.json').read_text())
            environment[name] = {k: r[k] for k in ['files', 'literal_sites', 'attested_sites', 'unattested_sites',
                'coverage', 'native_pairs', 'native_counts', 'pair_source', 'site_population_source',
                'same_line_duplicate_policy', 'nonliteral_policy', 'population_sha256', 'driver', 'driver_exit', 'php', 'xdebug',
                'trace_format', 'trace_bytes', 'trace_complete', 'trace_cap_mib',
                'executed_call_truth', 'executed_call_truth_found', 'executed_call_recall', 'precision']}
            environment[name]['driver_test_counts'] = test_counts((out/'phpunit.log').read_text())
            environment[name]['native_oracle_sha256'] = digest(out/'oracle.json')
            environment[name]['tier'] = 'A on executed population only'
        public['runtime_environments']['PHP '+environment['guzzle']['php']] = environment
    args.out.parent.mkdir(parents=True, exist_ok=True)
    args.out.write_text(json.dumps(public, indent=2, sort_keys=True, allow_nan=False)+'\n')


if __name__ == '__main__':
    main()
