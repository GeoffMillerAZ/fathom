"""T-R13: attributed bytes and EFBIG, with no vendor path publication."""
import hashlib
import json
from pathlib import Path
import tempfile
import unittest
import write_diagnostics as wd

class WriteDiagnosticsTest(unittest.TestCase):
    def test_private_path_bytes_and_exact_limit_file_are_relative(self):
        with tempfile.TemporaryDirectory() as directory:
            root=Path(directory).resolve();home=root/'home';(home/'tmp').mkdir(parents=True)
            path=home/'tmp'/'private-name';path.write_bytes(b'hello')
            trace=f'12 write(3<{path}>, ""..., 5) = 5\n12 ftruncate(3<{path}>, 9000000) = -1 EFBIG (File too large)\n12 --- SIGXFSZ {{si_signo=SIGXFSZ}} ---\n'
            result=wd.diagnostics(trace,root)
            row=result['files'][0]
            self.assertEqual('tmp',row['area']);self.assertEqual(5,row['observed_write_bytes']);self.assertEqual(5,row['final_size_bytes'])
            self.assertNotIn('path_sha256',row)
            self.assertIsNotNone(result['limit_file'])
            self.assertEqual(9000000,result['limit_file']['requested_extent_bytes'])
            self.assertEqual('home/tmp/private-name',row['relative_path']);self.assertNotIn(directory,str(result))
            self.assertEqual(row,result['largest_private_file'])
            self.assertTrue(row['exceeds_previous_limit'])
    def test_unlinked_memfd_limit_is_not_guessed_from_directory_sizes(self):
        trace='22 ftruncate(3</memfd:doublemapper>(deleted), 1073741824) = -1 EFBIG (File too large)\n22 --- SIGXFSZ {si_signo=SIGXFSZ} ---\n'
        with tempfile.TemporaryDirectory() as directory:
            root=Path(directory).resolve();(root/'home'/'tmp').mkdir(parents=True)
            result=wd.diagnostics(trace,root)
        self.assertEqual('other',result['limit_file']['area'])
        self.assertEqual('dotnet_doublemapper',result['limit_file']['kind'])
        self.assertEqual(1073741824,result['limit_file']['requested_extent_bytes'])
        self.assertTrue(result['files'][0]['unlinked'])
    def test_unfinished_syscall_resumes_per_thread(self):
        with tempfile.TemporaryDirectory() as directory:
            root=Path(directory).resolve();(root/'home'/'tmp').mkdir(parents=True)
            path=root/'home'/'value'
            trace=f'4 write(3<{path}>, ""..., 7 <unfinished ...>\n5 write(1<pipe:[17]>, ""..., 1) = 1\n4 <... write resumed>) = 7\n'
            self.assertEqual(7,wd.diagnostics(trace,root)['files'][0]['observed_write_bytes'])
    def test_missing_resumed_write_refuses_whole_diagnostic(self):
        with tempfile.TemporaryDirectory() as directory:
            root=Path(directory).resolve();(root/'home'/'tmp').mkdir(parents=True)
            with self.assertRaises(ValueError):wd.diagnostics('4 write(3</x>, "", 7 <unfinished ...>\n',root)
    def test_signal_without_failed_file_does_not_invent_a_cause(self):
        with tempfile.TemporaryDirectory() as directory:
            root=Path(directory).resolve();(root/'home'/'tmp').mkdir(parents=True)
            result=wd.diagnostics('4 --- SIGXFSZ {si_signo=SIGXFSZ} ---\n',root)
        self.assertTrue(result['sigxfsz_observed']);self.assertIsNone(result['limit_file'])
    def test_file_count_cap_plus_one_keeps_partial_evidence_and_largest(self):
        with tempfile.TemporaryDirectory() as directory:
            root=Path(directory).resolve();home=root/'home';home.mkdir()
            for index in range(wd.MAX_FILES+1):(home/str(index)).write_bytes(b'')
            (home/'last-largest').write_bytes(b'largest')
            result=wd.diagnostics('',root)
            self.assertEqual(wd.MAX_FILES,len(result['files']));self.assertTrue(result['files_truncated'])
            self.assertGreater(result['omitted_file_events'],0)
            self.assertEqual(7,result['largest_private_file']['final_size_bytes'])
    def test_snapshot_does_not_follow_private_symlink(self):
        with tempfile.TemporaryDirectory() as directory:
            root=Path(directory).resolve();home=root/'home';home.mkdir();outside=root/'outside';outside.mkdir()
            (outside/'private-name').write_bytes(b'secret');(home/'link').symlink_to(outside,target_is_directory=True)
            result=wd.diagnostics('',root)
        self.assertEqual(1,len(result['files']));self.assertEqual('symlink',result['files'][0]['final_type'])
        self.assertNotIn('private-name',str(result))

    def test_public_receipt_is_identical_for_different_scratch_and_home_roots(self):
        from probe_linux import receipt_base, attach_diagnostics, CONTRACT
        from receipt_contract import write_receipt
        receipts=[]
        with tempfile.TemporaryDirectory() as directory:
            for leaf,external in [('short','/home/one'),('much-longer-scratch-root','/home/another-long-home')]:
                root=Path(directory)/leaf;(root/'home'/'tmp').mkdir(parents=True)
                private=root/'home'/'tmp'/'value';private.write_bytes(b'hello')
                (root/'home'/'link').symlink_to(external)
                trace=f'12 execve("{root}/payload/PolicyLanguageServer", [""...], 0x1) = 0\n12 write(3<{private}>, ""..., 5) = 5\n12 truncate("{external}/.dotnet/state", 0) = -1 EROFS (Read-only file system)\n12 +++ exited with 0 +++\n'
                value=receipt_base();value.update(status='native_probe_failed',failed_stage='initialize',native_executed=True,native_returncode=0,filesystem_readonly_outside_out=True,vendor_capabilities_dropped=True,filesystem_dotdot_writes_refused=True,filesystem_write_probes=15)
                attach_diagnostics(value,trace,root,dict(sha256='0'*64,version=[5,16]))
                output=root/'public.json';write_receipt(output,value,CONTRACT,public=True);receipts.append(output.read_bytes())
                encoded=output.read_text()
                for path in [str(root),external,'/dev/null']:
                    self.assertNotIn(hashlib.sha256(path.encode()).hexdigest(),encoded)
                self.assertNotIn('path_sha256',encoded)
            self.assertEqual(receipts[0],receipts[1])

    def test_benign_names_relative_targets_and_noncanonical_paths_are_redacted_per_row(self):
        with tempfile.TemporaryDirectory() as directory:
            root=Path(directory).resolve();home=root/'home';home.mkdir()
            for name in ['a+b','a@b','a b','a~b']:(home/name).write_bytes(b'x')
            trace=f'1 truncate("relative", 12) = 0\n1 write(3<{home}/./ordinary>, ""..., 1) = 1\n'
            result=wd.diagnostics(trace,root)
            self.assertTrue(any(r['relative_path'] is None for r in result['files']))
            self.assertFalse(any('path_sha256' in r for r in result['files']))
            self.assertIn('home/ordinary',[r['relative_path'] for r in result['files']])

    def test_outside_scratch_rows_have_no_path_identity(self):
        with tempfile.TemporaryDirectory() as directory:
            result=wd.diagnostics('1 write(2</dev/null>, ""..., 5) = 5\n1 truncate("/home/runner/.dotnet/state", 0) = -1 EROFS (Read-only file system)\n',Path(directory))
        self.assertEqual(1,len(result['files']))
        self.assertIsNone(result['files'][0]['relative_path'])
        self.assertNotIn('path_sha256',str(result))
