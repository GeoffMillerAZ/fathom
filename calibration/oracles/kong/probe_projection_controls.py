"""Prove that C positions cannot choose or trim the native definition multiset."""
import argparse
import copy
import io
import json
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
    result,green=focused('test_native_projection.NativeProjectionTest')
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
    summary=dict(tests=result.testsRun,skips=0,mutant='filter native definition multiset by independent locator candidates',
                 failure_line=next(line for line in red_log.splitlines() if line.startswith('FAIL:')),restored_green=True)
    (out/'summary.json').write_text(json.dumps(summary,indent=2)+'\n')
    return summary


if __name__=='__main__':
    parser=argparse.ArgumentParser();parser.add_argument('-out',type=Path,required=True);args=parser.parse_args()
    print(json.dumps(run(args.out),sort_keys=True))
