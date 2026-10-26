"""T-R13: attributed bytes and EFBIG, with no vendor path publication."""
import hashlib
from pathlib import Path
import tempfile
import unittest
import write_diagnostics as wd

class WriteDiagnosticsTest(unittest.TestCase):
    def test_private_path_bytes_and_exact_limit_file_are_hashed(self):
        with tempfile.TemporaryDirectory() as directory:
            root=Path(directory).resolve();home=root/'home';(home/'tmp').mkdir(parents=True)
            path=home/'tmp'/'private-name';path.write_bytes(b'hello')
            trace=f'12 write(3<{path}>, ""..., 5) = 5\n12 ftruncate(3<{path}>, 9000000) = -1 EFBIG (File too large)\n12 --- SIGXFSZ {{si_signo=SIGXFSZ}} ---\n'
            result=wd.diagnostics(trace,root)
            row=result['files'][0]
            self.assertEqual('tmp',row['area']);self.assertEqual(5,row['observed_write_bytes']);self.assertEqual(5,row['final_size_bytes'])
            self.assertEqual(hashlib.sha256(b'private-name').hexdigest(),row['path_sha256'])
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
        self.assertEqual('memfd',result['limit_file']['area'])
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
    def test_file_count_cap_plus_one_refuses(self):
        with tempfile.TemporaryDirectory() as directory:
            root=Path(directory).resolve();home=root/'home';home.mkdir()
            for index in range(257):(home/str(index)).write_bytes(b'')
            with self.assertRaises(ValueError):wd.diagnostics('',root)
    def test_snapshot_does_not_follow_private_symlink(self):
        with tempfile.TemporaryDirectory() as directory:
            root=Path(directory).resolve();home=root/'home';home.mkdir();outside=root/'outside';outside.mkdir()
            (outside/'private-name').write_bytes(b'secret');(home/'link').symlink_to(outside,target_is_directory=True)
            result=wd.diagnostics('',root)
        self.assertEqual(1,len(result['files']));self.assertEqual('symlink',result['files'][0]['final_type'])
        self.assertNotIn('private-name',str(result))
