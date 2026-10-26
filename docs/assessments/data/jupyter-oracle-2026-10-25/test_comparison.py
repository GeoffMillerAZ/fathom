#!/usr/bin/env python3
"""Prevent an analyzer's nonempty answer from laundering syntax pairs into tier A."""
import unittest
from compare_native import answered_syntax_oracle


class SyntaxTier(unittest.TestCase):
    def test_shadowed_written_name_is_not_resolved_target_truth(self):
        # A local join followed by an import of os.path.join has the same written
        # name. An external answer cannot confirm a reference to the local join.
        receipt = {
            'server': 'basedpyright 1.40.2', 'resolved_calls': 1, 'calls': 2,
            'rejected_cells': 0, 'opaque_magic_lines': 0,
            'records': [{'path': 'shadow.ipynb', 'calls': [
                {'from': '<module>', 'to': 'join', 'definitions': [{'category': 'external'}]},
                {'from': '<module>', 'to': 'unknown', 'definitions': []},
            ]}],
        }
        result = answered_syntax_oracle(receipt)
        self.assertEqual(result['tier'], 'C')
        self.assertFalse(result['resolved_target_truth'])
        self.assertEqual(result['references'], [{'path': 'shadow.ipynb', 'from': '<module>', 'to': 'join'}])
        self.assertIn('not basedpyright target references', result['pair_source'])
        self.assertIn('lower bound', result['disclose'][0])
        self.assertIn('1 unknown', result['disclose'][0])


if __name__ == '__main__':
    unittest.main()
