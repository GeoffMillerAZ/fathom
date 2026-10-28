#!/usr/bin/env python3
"""Export/check the deferred-minor census without rewriting historical evidence."""
import argparse
import hashlib
import json
from pathlib import Path

HERE=Path(__file__).resolve().parent
REPO=HERE.parents[2]
DATA=REPO/'docs/assessments/data/powershell-oracle-2026-10-26'
FILES=['DiscoverHarness.ps1','ResolveCalls.ps1','measure_binder.py','tracked_set.py',
       'prove_followups.py','prove_inventory.py','prove_harness.py','prove_mutations.py','followup_evidence.py']


def sha(path):return hashlib.sha256(path.read_bytes()).hexdigest()


def check(data):
    assert data['canonical_reading_sha256']==sha(REPO/'docs/bench/readings/lang/powershell.json')
    assert data['historical_measurements_sha256']==sha(DATA/'measurements.json')
    old=json.loads((DATA/'measurements.json').read_text())['binder']['subjects']
    assert data['fixtures']['passed'] and data['fixtures']['native_tests']==4
    assert len(data['mutations'])==6
    assert all(m['FAIL'].startswith('FAIL:') and m['restored']=='GREEN' for m in data['mutations'])
    assert data['harness']['psa_fresh_declared_probes']==dict(union_only_answers_unresolved=2,shadowed_same_name_answers_with_original_origin=2)
    for name,row in data['binder']['subjects'].items():
        for key in ['native_oracle_sha256','literal_population_sha256','native_pairs','resolved_sites','unresolved_sites']:
            assert row[key]==old[name][key],name+' changed native truth or source population'
        assert row['tracked_before']==row['tracked_after']==old[name]['tracked_before']
        native=row['native']
        assert sum(native['unresolved_by_class'].values())==row['unresolved_sites']
        assert native['declaration_closure_unresolved_sensitivity']==row['unresolved_sites']+native['declaration_closure_missing_name_sites']
        assert native['session_union_shadowed_declared_helper_sites']<=native['session_union_nonlocal_helper_sites']
    psa=data['binder']['subjects']['psa']['native']
    assert (psa['session_union_nonlocal_helper_sites'],psa['session_union_shadowed_declared_helper_sites'],psa['declaration_closure_missing_name_sites'])==(4,2,2)
    assert psa['declaration_closure_unresolved_sensitivity']==582
    for name,digest in data['current_code_sha256'].items():
        assert name in FILES and sha(HERE/name)==digest,name+' code drift'
    assert set(data['current_code_sha256'])==set(FILES)
    for unit in [data['binder'],data['fixtures'],data['inventory']]:
        for name,digest in unit['oracle_code_sha256'].items():assert sha(HERE/name)==digest,name+' stale run'


def main():
    p=argparse.ArgumentParser(description=__doc__)
    p.add_argument('--capture',type=Path);p.add_argument('--check',type=Path)
    p.add_argument('--out',type=Path,default=DATA/'followups.json')
    a=p.parse_args()
    if a.check:
        check(json.loads(a.check.read_text()));print('PASS: unchanged native truth, complete crossing census, fresh probes, mutations and hashes');return
    assert a.capture,'--capture or --check required'
    def read(folder,name):return json.loads((a.capture/folder/name).read_text())
    data=dict(binder=read('binder-publish','summary.json'),fixtures=read('fixtures-publish','followup-proof.json'),
        mutations=read('mutations','mutations.json'),inventory=read('inventory-proof','inventory-proof.json'),
        harness=read('binder-publish','harness-proof.json'),
        current_code_sha256={name:sha(HERE/name) for name in FILES},
        canonical_reading_sha256=sha(REPO/'docs/bench/readings/lang/powershell.json'),
        historical_measurements_sha256=sha(DATA/'measurements.json'),
        historical_evidence_revision='4e4dd054323143b7b7bba6826dde543fe7270ae5',
        policy='Fresh full binder/fixture runs; unchanged original populations and native pair digests. Historical runtime, resolved-pair scores and 21-mutant proof remain at the recorded revision, not relabelled as current. No canonical regrade. Declaration-closure sensitivity is not a simulated isolated whole-suite run; four fresh PSA probes verify its two missing answers and two same-name shadowed origins.')
    check(data)
    a.out.parent.mkdir(parents=True,exist_ok=True);a.out.write_text(json.dumps(data,indent=2,sort_keys=True)+'\n')
    print('PASS: exported counts/hashes only')


if __name__=='__main__':main()
