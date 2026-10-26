"""Prove that C positions cannot choose or trim the native definition multiset."""
import argparse
import copy
import io
import json
import hashlib
import unittest
from pathlib import Path
from unittest.mock import patch
import grade_native_l2


def focused(name):
    log=io.StringIO()
    result=unittest.TextTestRunner(stream=log,verbosity=2).run(unittest.defaultTestLoader.loadTestsFromName(name))
    return result,log.getvalue()


def run(out):
    out.mkdir(parents=True,exist_ok=False)
    log=io.StringIO()
    suite=unittest.defaultTestLoader.discover(str(Path(__file__).parent),pattern='test_native_*.py')
    result=unittest.TextTestRunner(stream=log,verbosity=2).run(suite)
    green=log.getvalue()
    if not result.wasSuccessful() or result.skipped:
        raise ValueError('native projection controls failed or skipped')
    original=grade_native_l2.project
    def scanner_selected(rel,native,located):
        native=copy.deepcopy(native)
        allowed={(row['kind'],row['name']) for row in located['entities']}
        native['native_entities']=[row for row in native['native_entities'] if (row['kind'],row.get('name')) in allowed]
        return original(rel,native,located)
    name='test_native_projection.NativeProjectionTest.test_locator_omission_extra_and_tier_change_refuse_whole_input'
    with patch('test_native_projection.project',scanner_selected):
        red,red_log=focused(name)
    restored,restored_log=focused(name)
    if red.wasSuccessful() or not red.failures or red.errors or not restored.wasSuccessful():
        raise ValueError('native population selection mutant did not FAIL then GREEN')
    (out/'proof.log').write_text(green+'\nMUTANT\n'+red_log+'\nRESTORED\n'+restored_log)
    mutations=[dict(mutant='filter native definition multiset by independent locator candidates',
                    failure_line=next(line for line in red_log.splitlines() if line.startswith('FAIL:')),restored_green=True)]
    evidence=grade_native_l2.public_subset_evidence
    def grant_subset_grade(measured):
        row=evidence(measured);row['l2_tier_a_recall_granted']=True;return row
    family_counts=grade_native_l2.reference_family_counts
    def count_sites_as_pairs(*args):
        rows=family_counts(*args)
        for row in rows.values():row['semantic_pairs']=row['resolved_sites']
        return rows
    for function,mutant,name,label in [
        ('public_subset_evidence',grant_subset_grade,'test_product_selected_subset_never_grants_a_tier_or_support_grade','grant tier-A recall from product-selected subset'),
        ('reference_family_counts',count_sites_as_pairs,'test_site_and_pair_counts_keep_unowned_fields_and_empty_host_family','count repeated reference sites as distinct pairs')]:
        test='test_native_population.NativePopulationTest.'+name
        with patch('grade_native_l2.'+function,mutant):
            failed,red_text=focused(test)
        passed,green_text=focused(test)
        if failed.wasSuccessful() or not failed.failures or failed.errors or not passed.wasSuccessful():
            raise ValueError('coverage/family mutant did not FAIL then GREEN')
        with (out/'proof.log').open('a') as proof:proof.write('\nMUTANT\n'+red_text+'\nRESTORED\n'+green_text)
        mutations.append(dict(mutant=label,failure_line=next(line for line in red_text.splitlines() if line.startswith('FAIL:')),restored_green=True))
    here=Path(__file__).parent
    summary=dict(control_script_sha256=hashlib.sha256(Path(__file__).read_bytes()).hexdigest(),
                 grade_source_sha256=hashlib.sha256((here/'grade_native_l2.py').read_bytes()).hexdigest(),
                 python_test_sources=[dict(path=name,sha256=hashlib.sha256((here/name).read_bytes()).hexdigest()) for name in
                                      ('test_native_projection.py','test_native_population.py','test_native_scope.py')],
                 mutations=mutations,tests=result.testsRun,skips=0,mutant='filter native definition multiset by independent locator candidates',
                 failure_line=next(line for line in red_log.splitlines() if line.startswith('FAIL:')),restored_green=True)
    (out/'summary.json').write_text(json.dumps(summary,indent=2)+'\n')
    return summary


if __name__=='__main__':
    parser=argparse.ArgumentParser();parser.add_argument('-out',type=Path,required=True);args=parser.parse_args()
    print(json.dumps(run(args.out),sort_keys=True))
