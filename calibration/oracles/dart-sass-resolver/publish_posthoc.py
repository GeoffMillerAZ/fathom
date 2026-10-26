#!/usr/bin/env python3
"""Publish the spent Bourbon runner-path audit as counts/hashes, never fresh validation."""
import argparse
import hashlib
import json
from pathlib import Path
import re
import statistics
import math
import collect as oracle

PIN = 'c3551e46f41738681216a7f957bc88bc79155b16'
NOTE = ('POST HOC on spent Bourbon: subject runner load paths resolve all 85 imports; '
        'the native resolver adds 40 pairs beyond 45 inherited primary attestations. '
        'Fathom resolves 45/85, import_resolution .5294 and L3 unmet. '
        'This is diagnostic evidence, never fresh validation or canonical corpus input.')

def sha(p): return hashlib.sha256(p.read_bytes()).hexdigest()
def load(p): return json.loads(p.read_text())
def require(ok, why):
    if not ok: raise ValueError(why)

def native_counts(data):
    m=data['metadata']
    require(m['source_fingerprints_before']==m['source_fingerprints_after'], 'source drift')
    result={k:m[k] for k in ('source_files','load_paths','declared_dependencies','resolved_dependencies','unresolved_dependencies','parse_failures')}
    if isinstance(result['load_paths'],list): result['load_paths']=len(result['load_paths'])
    require(all(type(x) is int and x>=0 for x in result.values()), 'invalid native counts')
    result['inherited_primary_attestations']=data['inherited_primary_attestations']
    result['native_only_resolved_pairs']=result['resolved_dependencies']-result['inherited_primary_attestations']
    return result

def verify_sources(a,b,actual):
    require(a['metadata']['source_fingerprints_before']==b['metadata']['source_fingerprints_before']==actual,'cross-arm or pinned source drift')

def public_phases(phases):
    for row in phases:
        require(type(row['exit']) is int and all(type(row[k]) in (int,float) and math.isfinite(row[k]) and row[k]>=0 for k in ('launch_load','seconds')),'unsafe phase scalar')
    return [{k:row[k] for k in ('phase','exit','launch_load','seconds')} for row in phases]

def project(base,subject):
    r=load(base/'reading.private.json'); provenance=load(base/'composite.private.json.provenance.json')
    am=load(base/'native-metadata.private.json'); bm=load(base/'runner-native-metadata.private.json')
    oracle.require_clean_subject(subject,PIN)
    actual=oracle.tracked_manifest(subject,None)
    verify_sources(am,bm,actual)
    require(provenance['metadata_sha256']==sha(base/'native-metadata.private.json') and provenance['result_sha256']==sha(base/'composite.private.json'),'provenance artifact drift')
    a=native_counts(am); b=native_counts(bm)
    require(len(r['subjects'])==1 and r['subjects'][0]['sha']==PIN and provenance['subject_sha']==PIN, 'source pin')
    require(a['declared_dependencies']==a['resolved_dependencies']==85 and a['unresolved_dependencies']==0, 'full import population')
    require(a['native_only_resolved_pairs']==40 and b['resolved_dependencies']==85, 'native-only contribution')
    require(r['counts']['imports']==r['counts']['imports_in_repo']==85 and r['counts']['imports_resolved_agree']==45, 'reading population')
    require(r['grade']['L3']=='unmet', 'unexpected L3 result')
    phases=load(base/'phases.json')
    require([x['phase'] for x in phases]==['native-full-population','primary-subject-runner','native-after-runner-primary','build-indexbench','grade-full-population','retime-three'], 'phase population')
    require(all(x['exit']==0 and x['launch_load']<=9 for x in phases), 'failed/load-invalid phase')
    rows=re.findall(r'^run ([0-9]+): ([0-9]+\.[0-9]+) MB/CPU-s$',(base/'retime-three.log').read_text(),re.M)
    require([x[0] for x in rows]==['1','2','3'], 'timing labels')
    rates=[float(x[1]) for x in rows]
    require(statistics.median(rates)==r['tenancy']['throughput_mb_s'], 'median mismatch')
    raw_sha=sha(base/'reading.private.json'); r.setdefault('notes',[]).append(NOTE)
    encoded=(json.dumps(r,indent=2,ensure_ascii=False)+'\n').encode()
    receipt={'format':'scss-spent-runner-posthoc-v1','post_hoc':True,'fresh_validation':False,
             'source_commit':PIN,'fathom_commit':r['fathom_commit'],
             'frozen_primary_native_counts':a,'runner_primary_native_counts':b,
             'counts':r['counts'],'metrics':r['metrics'],'grades':r['grade'],
             'three_runs_mb_per_cpu_second':rates,'median_mb_per_cpu_second':statistics.median(rates),
             'phases':public_phases(phases),'reading_raw_sha256':raw_sha,
             'reading_annotated_sha256':hashlib.sha256(encoded).hexdigest(),
             'collector_sha256':sha(Path(__file__)),
             'inputs_sha256':{n:sha(base/n) for n in ['frozen-primary.private.json','runner-primary.private.json','composite.private.json','runner-composite.private.json','native-metadata.private.json','runner-native-metadata.private.json','composite.private.json.provenance.json','indexbench']}}
    return receipt,encoded

def selftest():
    row={'metadata':{'source_fingerprints_before':{'private':'hash'},'source_fingerprints_after':{'private':'hash'},'source_files':86,'load_paths':2,'declared_dependencies':85,'resolved_dependencies':85,'unresolved_dependencies':0,'parse_failures':0},'inherited_primary_attestations':45,'native_dependencies':[{'private':'NEVER_PUBLISH'}]}
    safe = public_phases([{'phase':'test','exit':0,'launch_load':1,'seconds':1,'private':'NEVER_PUBLISH'}])
    require('private' not in json.dumps(safe), 'phase leak')
    result=native_counts(row);require(result['native_only_resolved_pairs']==40,'gain');require('private' not in json.dumps(result),'private array leak')
    try: verify_sources(row,row,{'other':'hash'})
    except ValueError: pass
    else: raise AssertionError('cross-source mismatch accepted')
    row['metadata']['source_fingerprints_after']={}
    try: native_counts(row)
    except ValueError: pass
    else: raise AssertionError('source drift accepted')
    print('posthoc receipt privacy/source controls: PASS')

def main():
    p=argparse.ArgumentParser(description=__doc__);p.add_argument('--self-test',action='store_true');p.add_argument('--scratch',type=Path);p.add_argument('--subject',type=Path);p.add_argument('--receipt',type=Path);p.add_argument('--reading',type=Path);a=p.parse_args()
    if a.self_test: selftest();return
    require(a.scratch and a.subject and a.receipt and a.reading,'--scratch --subject --receipt --reading required')
    require(not a.receipt.exists() and not a.reading.exists(),'refuse overwrite')
    receipt,reading=project(a.scratch,a.subject.resolve(strict=True))
    a.receipt.write_text(json.dumps(receipt,indent=2,sort_keys=True)+'\n');a.reading.write_bytes(reading)
if __name__=='__main__': main()
