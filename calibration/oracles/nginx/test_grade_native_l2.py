import unittest
import tempfile
import subprocess
from pathlib import Path
from grade_native_l2 import original_scope, tracked_receipt, scope_summary
from native_upstream import reference_output


class ScopeAndAuthorityTest(unittest.TestCase):
    def test_all_previously_disclosed_files_are_retained(self):
        prior = {'notes': [
            'sample: restricted to first.conf, second.conf (-only; historical)',
            'sample: restricted to first.conf (-only; later duplicate)']}
        self.assertEqual(['first.conf', 'second.conf'], original_scope(prior, 'sample'))

    def test_scanner_population_never_claims_native_recall(self):
        payload = reference_output([], 'nginx/1.31.6')
        self.assertEqual('C', payload['tier'])

    def test_missing_scope_is_refused(self):
        with self.assertRaises(ValueError):
            original_scope({'notes': []}, 'sample')

    def test_native_references_cannot_disappear_through_harness_exclusions(self):
        result = dict(counts=dict(oracle_defs=1, ref_truth=0), subjects=[dict(files=1)])
        receipts = [dict(configs=2, definitions=2, references=1)]
        with self.assertRaisesRegex(ValueError, 'excluded'):
            scope_summary(result, receipts)
        result['counts']['ref_truth'] = 1
        summary = scope_summary(result, receipts)
        self.assertEqual(2, summary['native_inputs'])
        self.assertEqual(1, summary['harness_files'])
        self.assertEqual(1, summary['native_references_all_inputs'])

    def test_exact_normative_manifest_detects_input_and_untracked_changes(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory); subject = root / 'subject'; subject.mkdir()
            subprocess.run(['git', '-C', str(subject), 'init', '-q'], check=True)
            (subject / 'sample.conf').write_text('events {}\n')
            subprocess.run(['git', '-C', str(subject), 'add', 'sample.conf'], check=True)
            before = tracked_receipt(subject, '.', root / 'before')
            self.assertEqual(1, before['files'])
            self.assertEqual(0, before['untracked_entries'])
            (subject / 'sample.conf').write_text('events { worker_connections 128; }\n')
            (subject / 'generated.conf').write_text('http {}\n')
            after = tracked_receipt(subject, '.', root / 'after')
            self.assertNotEqual(before['sha256'], after['sha256'])
            self.assertEqual(1, after['untracked_entries'])
