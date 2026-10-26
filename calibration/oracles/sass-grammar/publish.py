#!/usr/bin/env python3
"""Publish counts and hashes from native Sass census output, excluding file lists/content."""
import argparse
import collections
import hashlib
import json
from pathlib import Path


def digest(value):
    return hashlib.sha256(json.dumps(value, sort_keys=True, separators=(',', ':')).encode()).hexdigest()


def summarize(gate, reference):
    keys = ('candidate', 'commit', 'licence', 'tree_sitter', 'abi', 'files', 'files_in_error', 'error_rate', 'gate', 'scope_note', 'archive_sha256', 'registered_at')
    result = {k: gate[k] for k in keys if k in gate}
    subjects = []
    for subject in gate['subjects']:
        errors = subject['errors']
        if len(errors) != subject['files_in_error']:
            raise ValueError('native error count differs from diagnostic list')
        hist = collections.Counter()
        for error in errors:
            for item in error['diagnostics']:
                hist['missing' if item.get('missing') else item['type']] += 1
        subjects.append({**{k:subject[k] for k in ('name','sha','scope','files','bytes','files_in_error','error_rate')},
                         'diagnostic_kinds': dict(sorted(hist.items())),
                         'diagnostic_list_sha256': digest(errors),
                         'timeouts': sum(bool(e.get('timeout')) for e in errors)})
    if sum(s['files'] for s in subjects) != gate['files'] or sum(s['files_in_error'] for s in subjects) != gate['files_in_error']:
        raise ValueError('native totals differ from subject sums')
    result['subjects'] = subjects
    refs = []
    for row in reference['results']:
        rejected = row['reference_rejected']
        if row['reference_accepted'] + len(rejected) != row['files']:
            raise ValueError('reference acceptance counts do not sum')
        refs.append({**{k:row[k] for k in ('name','files','reference_accepted','grammar_errors_on_reference_accepted')},
                     'reference_rejected': len(rejected), 'rejection_list_sha256': digest(rejected)})
    result['reference_crosscheck'] = {'tool':reference['tool'],'version':reference['version'],'results':refs}
    result['publication'] = {'schema':1,'native_output_sha256':digest(gate),'reference_output_sha256':digest(reference),
                             'policy':'Counts, hashes and summaries only. Raw per-file diagnostics and compiler messages stay in scratch.'}
    return result


def self_test():
    gate={'files':1,'files_in_error':1,'subjects':[{'name':'synthetic','sha':'a'*40,'scope':'.','files':1,'bytes':1,'files_in_error':1,'error_rate':1,'errors':[{'path':'unpublished.sass','diagnostics':[{'type':'ERROR','missing':False,'content':'DO_NOT_PUBLISH'}],'timeout':False}]}]}
    ref={'tool':'synthetic','version':'test','results':[{'name':'synthetic','files':1,'reference_accepted':0,'grammar_errors_on_reference_accepted':0,'reference_rejected':[{'path':'unpublished.sass','message':'DO_NOT_PUBLISH'}]}]}
    result=summarize(gate,ref)
    text=json.dumps(result)
    assert 'unpublished.sass' not in text and 'DO_NOT_PUBLISH' not in text
    assert result['subjects'][0]['diagnostic_kinds']=={'ERROR':1}
    assert result['reference_crosscheck']['results'][0]['reference_rejected']==1
    gate['subjects'][0]['files_in_error']=0
    try: summarize(gate,ref)
    except ValueError: pass
    else: raise AssertionError('inconsistent counts accepted')
    print('PUBLICATION SELF-TEST PASS')


def main():
    parser=argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--self-test',action='store_true')
    parser.add_argument('--gate',type=Path)
    parser.add_argument('--reference',type=Path)
    parser.add_argument('--out',type=Path)
    args=parser.parse_args()
    if args.self_test: self_test();return
    if not args.gate or not args.out:parser.error('--gate and --out required')
    gate=json.loads(args.gate.read_text())
    reference=json.loads(args.reference.read_text()) if args.reference else gate['reference_crosscheck']
    args.out.write_text(json.dumps(summarize(gate,reference),indent=2)+'\n')


if __name__=='__main__':main()
