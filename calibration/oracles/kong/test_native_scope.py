"""Historical selector aliases must preserve every native query, not pad truth."""
import unittest
from grade_native_l2 import original_scope


class OriginalScopeTest(unittest.TestCase):
    def test_repeated_legacy_roots_are_aliases_without_dropping_queries(self):
        root = 'workspace/calibration/kong-td/deck/'
        prior = dict(notes=['fixture: restricted to cfg.yaml, ' + root + 'cfg.yaml, ' + root + root + 'cfg.yaml (-only; recorded)'])
        self.assertEqual(['cfg.yaml'] * 3, original_scope(prior, 'fixture'))

    def test_traversal_is_refused(self):
        with self.assertRaisesRegex(ValueError, 'unsafe'):
            original_scope(dict(notes=['fixture: restricted to ../cfg.yaml (-only; recorded)']), 'fixture')
