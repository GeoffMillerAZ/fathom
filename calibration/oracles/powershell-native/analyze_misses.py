"""Counts-only attribution of executed native pairs absent from product output."""
from collections import Counter,defaultdict
import hashlib
import json
import re
from trace_decode import bare

MOCK=re.compile(r'^PesterMock_.+_[0-9a-fA-F]{8}(?:-[0-9a-fA-F]{4}){3}-[0-9a-fA-F]{12}$')


def folded(pair):return pair[0],pair[1].casefold(),pair[2].casefold()


def classify(pair,predicted,witnesses):
    if folded(pair) in {folded(p) for p in predicted}:return 'case_only'
    written={(pair[0],pair[1],bare(invoked)) for invoked in witnesses}
    captured=bool(written & predicted)
    if MOCK.fullmatch(pair[2]) and captured:return 'generated_mock_rebinding'
    if captured:return 'engine_target_differs_from_written'
    return 'written_pair_absent'


def analyze(raw,population,oracle,unmatched):
    truth={(r['path'],r['from'],r['to']) for r in oracle['calls']}
    missed={(r['path'],r['caller'],r['callee']) for r in unmatched['calls']['oracle_only']}
    predicted={(r['path'],r['caller'],r['callee']) for r in unmatched['calls']['fathom_only']} | (truth-missed)
    assert missed<=truth
    lookup=defaultdict(list)
    for site in population['sites']:
        if site['kind']=='command':lookup[(site['path'],site['line'],site['column'])].append(site)
    witnesses=defaultdict(set)
    for path,line,column,caller,target,invoked in raw['calls']:
        sites=lookup[(path,line,column)]
        if len(sites)!=1:continue
        site=sites[0]
        if invoked.casefold()!=site['written'].casefold() or bare(caller).casefold()!=bare(site['from']).casefold():continue
        witnesses[(path,bare(caller),bare(target))].add(invoked)
    assert all(pair in witnesses for pair in missed),'unattributed native miss'
    counts=Counter(classify(pair,predicted,witnesses[pair]) for pair in missed)
    return dict(native_pairs=len(truth),found_pairs=len(truth)-len(missed),missed_pairs=len(missed),
                misses_by_cause=dict(sorted(counts.items())),
                policy='mutually exclusive pair counts; generated mock attribution requires a native GUID-shaped target and the written invocation pair in actual product output; no missed pair removed from executed recall')
