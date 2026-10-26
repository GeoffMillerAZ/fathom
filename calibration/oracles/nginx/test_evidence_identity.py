"""Stable native identity receipts preserve binding equality across ASLR."""
import copy
import unittest
from native_population import normalized_evidence
from grade_native_l2 import oracle_sources


class EvidenceIdentityTest(unittest.TestCase):
    def test_pointer_renumbering_keeps_truth_but_changed_binding_does_not(self):
        rows = [dict(native_events=[dict(domain='http', pointer=41, flags=0),
                                   dict(domain='http', pointer=41, flags=1),
                                   dict(domain='http', pointer=42, flags=1)])]
        changed = copy.deepcopy(rows)
        for event in changed[0]['native_events']: event['pointer'] += 1000
        expected = normalized_evidence(rows)
        self.assertEqual(expected, normalized_evidence(changed))
        self.assertEqual('declaration:1', expected[0]['native_events'][0]['pointer'])
        changed[0]['native_events'][0]['pointer'] = 1042
        self.assertNotEqual(expected, normalized_evidence(changed))
        self.assertEqual(41, rows[0]['native_events'][0]['pointer'])

    def test_grader_emits_source_identity_from_current_committed_drivers(self):
        rows = oracle_sources()
        self.assertEqual(len(rows), len({row['path'] for row in rows}))
        self.assertIn('nginx_oracle.py', {row['path'] for row in rows})
        self.assertTrue(all(len(row['sha256']) == 64 for row in rows))
