"""Regression controls for public #135 Jenkins m1-m5 disclosures."""
import unittest
from pathlib import Path

ROOT=Path(__file__).resolve().parents[3]
class NativeDisclosuresTest(unittest.TestCase):
    def test_m2_node_limit_is_disclosed_as_per_file_refusal(self):
        self.assertIn('node cap refuses that file', ' '.join((ROOT/'calibration/oracles/jenkinsfile/NATIVE.md').read_text().split()))
    def test_m3_raw_limit_is_disclosed_as_parse_bound(self):
        self.assertIn('bounds parsing, not output size', ' '.join((ROOT/'calibration/oracles/jenkinsfile/NATIVE.md').read_text().split()))
    def test_m4_unreceipted_environment_trials_cannot_attest_identity(self):
        self.assertIn('not individually bound to instrument and bundle identities', ' '.join((ROOT/'calibration/oracles/jenkinsfile/NATIVE.md').read_text().split()))
    def test_m5_d19_is_not_still_the_native_comparison_blocker(self):
        row=next(l for l in (ROOT/'docs/plans/language-breadth.md').read_text().splitlines() if 'id: LB-Q-jenkinsfile-rest,' in l)
        self.assertNotIn('ceiling audit',row)
        self.assertNotIn('L3 remains held for D19',row)

    def test_fix1_does_not_relabel_import_error_as_assertion_red(self):
        for path in ('calibration/oracles/jenkinsfile/NATIVE.md','calibration/oracles/jenkinsfile/OWNER-COMPARISON.md','docs/assessments/2026-10-26-tools-jenkins-native.md'):
            text=' '.join((ROOT/path).read_text().split())
            self.assertIn('ModuleNotFoundError',text)
            self.assertIn('historical hook evidence gap',text)
            self.assertIn('correction assertion RED',text)

    def test_search_receipt_times_and_purposes_are_explicit(self):
        import json
        receipt=json.loads((ROOT/'calibration/oracles/jenkinsfile/comparison-211.json').read_text())
        for row in receipt['search_refresh']:
            self.assertEqual('UNKNOWN',row['retrieved_at'])
            self.assertIn('not retained',row['retrieval_time_gap'])
            self.assertTrue(row['purpose'])
            self.assertEqual(64,len(row['sha256']))
