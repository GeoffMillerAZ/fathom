import tempfile
import unittest
from pathlib import Path
from population_capture import NativeCapture
from test_population import native_line


class PopulationCaptureTest(unittest.TestCase):
    def record(self, directory, events, returncode=0):
        return dict(directory=str(directory), prefix='conf', target='conf/site.conf',
                    mode='http', returncode=returncode, stderr='\n'.join(events), stdout='')

    def test_native_events_are_captured_before_cleanup_and_paths_restore_prefix(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            capture = NativeCapture('conf/site.conf')
            events = [native_line('http', 1, 0, 8, str(root / 'site.conf'), 'proxy_pass', 'http://app', 'app'),
                      native_line('http', 1, 1, 3, 'site.conf', 'upstream', 'app', 'app'),
                      native_line('http', 2, 1, 1, str(root / '_wrapper.conf'), 'upstream', 'context', 'context')]
            capture(self.record(root, events))
            self.assertEqual(['conf/site.conf', 'conf/site.conf', '@context-wrapper'],
                             [e['path'] for e in capture.selected()['events']])
            self.assertEqual('conf/site.conf', capture.selected()['target'])

    def test_failed_context_never_contributes_partial_native_truth(self):
        capture = NativeCapture('conf/site.conf')
        capture(self.record(Path('/unused'), ['ORACLE_NGINX_UPSTREAM_ERROR null result'], returncode=1))
        self.assertEqual(1, capture.failed_contexts)
        with self.assertRaisesRegex(ValueError, 'one successful'):
            capture.selected()

    def test_positive_unknown_protocol_and_outside_paths_fail_closed(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            for event in ['ORACLE_NGINX_UPSTREAM_ERROR missing',
                          native_line('http', 1, 0, 8, str(root.parent / 'outside.conf'), 'proxy_pass', 'http://app', 'app')]:
                with self.assertRaises(ValueError):
                    NativeCapture('conf/site.conf')(self.record(root, [event]))

    def test_multiple_positive_profiles_and_wrong_targets_are_refused(self):
        with tempfile.TemporaryDirectory() as directory:
            capture = NativeCapture('conf/site.conf')
            record = self.record(Path(directory), [])
            capture(record)
            with self.assertRaisesRegex(ValueError, 'one successful'):
                capture(record)
            record['target'] = 'conf/other.conf'
            with self.assertRaisesRegex(ValueError, 'target'):
                NativeCapture('conf/site.conf')(record)
