import tempfile
import unittest
from pathlib import Path
from types import SimpleNamespace
from unittest.mock import patch
from nginx_oracle import nginx_check


class ObservationCaptureTest(unittest.TestCase):
    def test_refused_native_profile_is_still_removed(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory) / 'source'; root.mkdir()
            (root / 'nginx.conf').write_text('events {}\nhttp {}\n')
            observed = []
            def refuse(record):
                observed.append(Path(record['directory']))
                raise ValueError('unknown native event')
            with patch('nginx_oracle.subprocess.run', return_value=SimpleNamespace(returncode=0, stdout='', stderr='bad-event')):
                with self.assertRaises(ValueError):
                    nginx_check(str(root), '', 'unused', str(Path(directory) / 'work'), 'main', 'nginx.conf', observer=refuse)
            self.assertFalse(observed[0].exists())

    def test_positive_stderr_is_observed_before_scratch_cleanup(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory) / 'source'; root.mkdir()
            (root / 'nginx.conf').write_text('events {}\nhttp {}\n')
            captured = []
            def observe(record):
                self.assertTrue(Path(record['directory']).is_dir())
                captured.append(record)
            with patch('nginx_oracle.subprocess.run', return_value=SimpleNamespace(returncode=0, stdout='', stderr='native-event')):
                result = nginx_check(str(root), '', 'unused', str(Path(directory) / 'work'), 'main', 'nginx.conf', observer=observe)
            self.assertTrue(result[0])
            self.assertEqual(4, len(result))
            self.assertEqual('native-event', captured[0]['stderr'])
            self.assertEqual('main', captured[0]['mode'])
            self.assertFalse(Path(captured[0]['directory']).exists())
