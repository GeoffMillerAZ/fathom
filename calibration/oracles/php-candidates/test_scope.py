import unittest

from audit_scope import summarize_scope


class CompletePopulationTests(unittest.TestCase):
    def test_omitted_documents_are_unattested_even_when_local_blind_share_is_small(self):
        pairs = {('indexed.php', 'run', 'call'+str(n)) for n in range(20)}
        pairs |= {('omitted.php', 'run', 'call'+str(n)) for n in range(30)}
        result = summarize_scope(pairs, {'indexed.php'}, 1, 19)
        self.assertEqual(result['written_pairs_in_unindexed_files'], 30)
        self.assertEqual(result['full_unattested_pairs'], 31)
        self.assertEqual(result['full_unattested_fraction'], .62)
        self.assertEqual(result['matched_fraction_lower_bound_with_unattested_as_misses'], .38)


if __name__ == '__main__':
    unittest.main()
