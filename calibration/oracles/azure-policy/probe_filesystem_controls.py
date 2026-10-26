"""Mocked boundary regressions and mutants; not a Linux isolation receipt."""
import argparse
import hashlib
import io
import json
import unittest
from pathlib import Path
from unittest.mock import patch
import filesystem_isolation as filesystem

HERE=Path(__file__).parent


def run(out):
    out.mkdir(parents=True,exist_ok=False)
    proof=io.StringIO()
    runner=unittest.TextTestRunner(stream=proof,verbosity=2)
    result=runner.run(unittest.defaultTestLoader.discover(str(HERE),pattern='test_*.py'))
    if not result.wasSuccessful() or result.skipped:
        raise ValueError('boundary regression failed or skipped')
    mutations=[]
    for function,mutant,name in [
        ('require_readonly',lambda path:None,'test_writable_protected_directory_refuses'),
        ('restricted_command',lambda command:list(command),'test_vendor_command_cannot_retain_mount_privileges')]:
        test='test_filesystem_isolation.FilesystemIsolationTest.'+name
        with patch.object(filesystem,function,mutant):
            red=runner.run(unittest.defaultTestLoader.loadTestsFromName(test))
        green=runner.run(unittest.defaultTestLoader.loadTestsFromName(test))
        if red.wasSuccessful() or not red.failures or red.errors or not green.wasSuccessful():
            raise ValueError('boundary mutant failed to prove FAIL then GREEN')
        mutations.append(dict(mutant=function,restored_green=True))
    files=['filesystem_isolation.py','probe_linux.py','receipt_contract.py',
           'test_filesystem_isolation.py','test_review_probe.py','test_probe_linux.py','probe_filesystem_controls.py',
           'probe-linux-receipt.schema.json','probe-linux-x64.workflow.yml','run_probe_workflow.sh',
           'native-protocol-r2.json','LINUX-X64-PROBE.md']
    summary=dict(tests=result.testsRun,skips=0,mutations=mutations,linux_namespace_executed=False,
                 native_executed=False,calibration_measured=False,tier_granted=False,
                 sources=[dict(path=name,sha256=hashlib.sha256((HERE/name).read_bytes()).hexdigest()) for name in files])
    (out/'proof.log').write_text(proof.getvalue())
    (out/'summary.json').write_text(json.dumps(summary,indent=2)+'\n')
    return summary


if __name__=='__main__':
    parser=argparse.ArgumentParser();parser.add_argument('-out',type=Path,required=True)
    print(json.dumps(run(parser.parse_args().out),sort_keys=True))
