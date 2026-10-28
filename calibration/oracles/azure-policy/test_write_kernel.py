"""Mandatory runner regression, explicitly skipped only off Linux."""
import sys
import os
import subprocess
import tempfile
import unittest
from pathlib import Path
from kernel_write_control import run

class LinuxWriteKernelTest(unittest.TestCase):
    @unittest.skipUnless(sys.platform.startswith('linux'),'requires Linux namespaces and strace')
    def test_real_private_scratch_child_attributes_sigxfsz_to_exact_file(self):
        with tempfile.TemporaryDirectory() as directory:
            value=run(Path(directory)/'write-control')
        self.assertTrue(value['private_scratch_boundary'])
        self.assertTrue(value['exact_limit_file_identified'])
        self.assertFalse(value['native_executed'])


class WriteKernelSetupTest(unittest.TestCase):
    def test_child_relative_tmp_write_survives_mount_setup_cwd_change(self):
        from unittest.mock import patch
        import vendor_exec as vendor
        original_popen=subprocess.Popen
        previous=Path.cwd()
        for authored in [False,True]:
            with self.subTest(authored=authored), tempfile.TemporaryDirectory() as directory:
                root=Path(directory).resolve();home=root/'home';(home/'tmp').mkdir(parents=True)
                payload=root/'payload';payload.mkdir()
                def child_process(command,**kwargs):
                    # Execute only this authored relative open. Keep the actual
                    # launch cwd supplied by vendor.child; do not run a vendor,
                    # namespace operation, privilege change or file-size limit.
                    return original_popen([sys.executable,'-B','-c',
                        "import os,sys\ntry:fd=os.open('tmp/limit-control',os.O_CREAT|os.O_WRONLY,0o600)\nexcept OSError as error:sys.exit(64+error.errno)\nos.write(fd,b'hello');os.close(fd)"],
                        stdout=subprocess.DEVNULL,stderr=subprocess.DEVNULL,**kwargs)
                try:
                    with patch.object(vendor.os,'getpid',return_value=1), \
                            patch.object(vendor,'mount_readonly',side_effect=os.chdir), \
                            patch.object(vendor,'close_extra_fds'), \
                            patch.object(vendor,'restricted_command',side_effect=lambda command:command), \
                            patch.object(vendor.subprocess,'Popen',side_effect=child_process):
                        with self.assertRaises(SystemExit) as exited:vendor.child(home,payload,authored)
                    self.assertEqual(0,exited.exception.code)
                    self.assertEqual(b'hello',(home/'tmp'/'limit-control').read_bytes())
                    self.assertFalse((root/'tmp').exists())
                finally:os.chdir(previous)

    def control_result(self,root):
        from write_diagnostics import diagnostics
        home=root/'home';(home/'tmp').mkdir(parents=True)
        path=home/'tmp'/'limit-control';path.write_bytes(b'hello')
        binary=home/'authored-write-control'
        # strace -yy decodes the descriptor to its absolute target even though
        # the child originally opened the relative spelling tmp/limit-control.
        trace=(f'3 execve("{binary}", [], 0x1) = 0\n'
               f'3 write(3<{path}>, ""..., 5) = 5\n'
               f'3 ftruncate(3<{path}>, 32768) = -1 EFBIG (File too large)\n'
               '3 --- SIGXFSZ {} ---\n3 +++ killed by SIGXFSZ +++\n')
        return diagnostics(trace,root,binary)

    def test_absolute_fd_target_retains_tmp_area_and_control_relative_path(self):
        from kernel_write_control import validate_result
        with tempfile.TemporaryDirectory() as directory:
            result=self.control_result(Path(directory).resolve())
            self.assertEqual('tmp',result['limit_file']['area'])
            self.assertEqual('home/tmp/limit-control',result['limit_file']['relative_path'])
            self.assertEqual(-25,result['native_returncode'])
            validate_result(result,153)

    def test_control_failure_describes_compared_fields_only_in_test_error(self):
        from kernel_write_control import validate_result
        with tempfile.TemporaryDirectory() as directory:
            result=self.control_result(Path(directory).resolve())
            result.update(native_returncode=1,sigxfsz_observed=False,limit_file=None)
            with self.assertRaises(ValueError) as failed:validate_result(result,1)
            text=str(failed.exception)
            for expected in ['"native_returncode": 1','"sigxfsz_observed": false',
                             '"limit_file": null','"outer_returncode": 1',
                             '"vendor_exec_observed": true','"area": "tmp"',
                             '"relative_path": "home/tmp/limit-control"',
                             '"files_truncated": false','"snapshot_truncated": false']:
                self.assertIn(expected,text)
            self.assertNotIn(directory,text)

            result=self.control_result(Path(directory)/'byte-control')
            next(row for row in result['files'] if row['relative_path']=='home/tmp/limit-control')['observed_write_bytes']=0
            with self.assertRaises(ValueError) as failed:validate_result(result,153)
            self.assertIn('per-path written-byte control failed:',str(failed.exception))
            self.assertIn('"observed_write_bytes": 0',str(failed.exception))
            self.assertNotIn(directory,str(failed.exception))
