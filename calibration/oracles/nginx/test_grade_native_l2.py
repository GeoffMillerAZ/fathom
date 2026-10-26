import unittest
from grade_native_l2 import original_scope
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
