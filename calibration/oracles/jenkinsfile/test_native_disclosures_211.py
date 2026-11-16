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
