"""Mocked boundary regressions and mutants; not a Linux isolation receipt."""
import argparse
import hashlib
import io
import json
import sys
import unittest
from pathlib import Path
from unittest.mock import patch
import filesystem_isolation as filesystem
import write_diagnostics as writes
import vendor_exec as vendor

HERE=Path(__file__).parent


def run(out):
    out.mkdir(parents=True,exist_ok=False)
    proof=io.StringIO()
    runner=unittest.TextTestRunner(stream=proof,verbosity=2)
    result=runner.run(unittest.defaultTestLoader.discover(str(HERE),pattern='test_*.py'))
    linux=sys.platform.startswith('linux')
    allowed_skips={'test_filesystem_kernel.LinuxFilesystemKernelTest.test_real_namespace_refuses_dot_dot_writes_and_hidden_submount',
                   'test_write_kernel.LinuxWriteKernelTest.test_real_private_scratch_child_attributes_sigxfsz_to_exact_file'}
    if (not result.wasSuccessful() or (linux and result.skipped) or
            (not linux and {test.id() for test,_ in result.skipped}!=allowed_skips)):
        raise ValueError('boundary regression failed or skipped')
    def hidden_mounts(text):
        return [(Path(row.split()[4]),row.split()[5]) for row in text.splitlines()]
    mutations=[]
    for function,mutant,name in [
        ('require_readonly',lambda path:None,'test_writable_protected_directory_refuses'),
        ('restricted_command',lambda command:list(command),'test_vendor_command_cannot_retain_mount_privileges'),
        ('parse_mountinfo',hidden_mounts,'test_real_runner_proc_overmount_skips_hidden_binfmt_remount'),
        ('path_variants',lambda path:[path],'test_write_controls_preserve_dot_dot_spellings'),
        ('parse_mountinfo',hidden_mounts,'test_stacked_namespace_root_refuses_instead_of_hiding_real_submounts')]:
        test='test_filesystem_isolation.FilesystemIsolationTest.'+name
        with patch.object(filesystem,function,mutant):
            red=runner.run(unittest.defaultTestLoader.loadTestsFromName(test))
        green=runner.run(unittest.defaultTestLoader.loadTestsFromName(test))
        if red.wasSuccessful() or not red.failures or red.errors or not green.wasSuccessful():
            raise ValueError('boundary mutant failed to prove FAIL then GREEN')
        mutations.append(dict(mutant=function,restored_green=True))
    original=writes.diagnostics
    def no_limit(*args,**kwargs):
        value=original(*args,**kwargs);value['limit_file']=None;return value
    def no_bytes(*args,**kwargs):
        value=original(*args,**kwargs)
        for row in value['files']:row['observed_write_bytes']=0
        return value
    namespace=vendor.namespace_command
    for module,function,mutant,test in [
        (writes,'diagnostics',no_limit,'test_write_diagnostics.WriteDiagnosticsTest.test_private_path_bytes_and_exact_limit_file_are_hashed'),
        (writes,'diagnostics',no_bytes,'test_write_diagnostics.WriteDiagnosticsTest.test_private_path_bytes_and_exact_limit_file_are_hashed'),
        (vendor,'namespace_command',lambda *a,**k:[x for x in namespace(*a,**k) if x!='--pid'],'test_write_tracing.WriteTracingTest.test_trace_writer_is_outside_private_pid_and_proc_view'),
        (vendor,'VENDOR_LIMIT',8*1024**2,'test_write_tracing.WriteTracingTest.test_vendor_has_ordinary_signal_semantics_and_only_raises_child_limit')]:
        with patch.object(module,function,mutant):
            red=runner.run(unittest.defaultTestLoader.loadTestsFromName(test))
        green=runner.run(unittest.defaultTestLoader.loadTestsFromName(test))
        if red.wasSuccessful() or not red.failures or red.errors or not green.wasSuccessful():
            raise ValueError('write diagnostic mutant failed to prove FAIL then GREEN')
        mutations.append(dict(mutant=module.__name__+'.'+function,restored_green=True))
    files=['filesystem_isolation.py','probe_linux.py','receipt_contract.py',
           'test_filesystem_isolation.py','test_review_probe.py','test_probe_linux.py','probe_filesystem_controls.py',
           'probe-linux-receipt.schema.json','probe-linux-x64.workflow.yml','run_probe_workflow.sh',
           'native-protocol-r2.json','LINUX-X64-PROBE.md','kernel_filesystem_control.py','test_filesystem_kernel.py',
           'runner-mountinfo-ubuntu-22.04.txt','runner-userns-ubuntu-24.04.stderr.txt',
           'write_diagnostics.py','write_tracing.py','vendor_exec.py','kernel_write_control.py',
           'test_write_diagnostics.py','test_write_tracing.py','test_write_kernel.py','test_probe_run_receipt.py',
           'probe-receipt-run40407284519.json','probe-receipt-run40407284519.schema.json']
    summary=dict(tests=result.testsRun,skips=len(result.skipped),mutations=mutations,linux_namespace_executed=linux,
                 linux_control_required_on_runner=True,
                 native_executed=False,calibration_measured=False,tier_granted=False,
                 sources=[dict(path=name,sha256=hashlib.sha256((HERE/name).read_bytes()).hexdigest()) for name in files])
    (out/'proof.log').write_text(proof.getvalue())
    (out/'summary.json').write_text(json.dumps(summary,indent=2)+'\n')
    return summary


if __name__=='__main__':
    parser=argparse.ArgumentParser();parser.add_argument('-out',type=Path,required=True)
    print(json.dumps(run(parser.parse_args().out),sort_keys=True))
