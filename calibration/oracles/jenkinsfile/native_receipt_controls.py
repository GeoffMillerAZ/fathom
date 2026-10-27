"""Meaningful receipt-boundary mutants, without executing Jenkins."""
import argparse
import copy
import io
import json
from pathlib import Path
import unittest
from unittest.mock import patch
import native_model_probe as probe

HERE=Path(__file__).resolve().parent

def run(out):
    out.mkdir(parents=True,exist_ok=False)
    stream=io.StringIO();runner=unittest.TextTestRunner(stream=stream,verbosity=2)
    suite=unittest.defaultTestLoader.discover(str(HERE),pattern='test_native_model_probe.py')
    baseline=runner.run(suite)
    if not baseline.wasSuccessful() or baseline.skipped:raise ValueError('unit control failed or skipped')
    original=probe.summarize
    def lose_unknown(raw):
        value=copy.deepcopy(raw)
        for row in value['files']:row.pop('identity',None)
        return original(value)
    def truncate(raw):
        value=copy.deepcopy(raw);value['files']=value['files'][:128]
        return original(value)
    mutations=[]
    for field,mutant,test in [
        ('verify_bundle',lambda *args:1,'test_native_bundle_rejects_changed_or_extra_plugins'),
        ('summarize',lose_unknown,'test_unknown_native_field_is_not_published'),
        ('summarize',truncate,'test_input_cap_plus_one_refuses')]:
        name='test_native_model_probe.NativeModelTest.'+test
        with patch.object(probe,field,mutant):red=runner.run(unittest.defaultTestLoader.loadTestsFromName(name))
        green=runner.run(unittest.defaultTestLoader.loadTestsFromName(name))
        if red.wasSuccessful() or not red.failures or red.errors or not green.wasSuccessful():
            (out/'proof.log').write_text(stream.getvalue())
            raise ValueError('mutant did not FAIL then GREEN')
        mutations.append(dict(test=test,restored_green=True))
    result=dict(tests=baseline.testsRun,skips=0,mutations=mutations,native_executed=False)
    (out/'proof.log').write_text(stream.getvalue())
    (out/'summary.json').write_text(json.dumps(result,indent=2)+'\n')
    return result

if __name__=='__main__':
    parser=argparse.ArgumentParser();parser.add_argument('--out',type=Path,required=True)
    print(json.dumps(run(parser.parse_args().out),sort_keys=True))
