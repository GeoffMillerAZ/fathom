import json
import unittest

from publish import digest, summarize


class PublicationTests(unittest.TestCase):
    def test_report_retains_counts_and_failure_location_without_subject_names(self):
        path = 'fixtures/private-name.sql'
        pair = {'path': path, 'from': 'child', 'to': 'parent'}
        error = {'path': path, 'line': 3, 'kind': 'insert', 'code': '23503', 'message': 'subject text'}
        raw = {'version': 'test', 'calls': [pair], 'files': [
            {'path': path, 'source_bytes': 42, 'source_sha256': 'abc', 'engine': 'postgresql',
             'receipts': [{'accepted': True}, {'error': '23503'}], 'errors': [error],
             'create_table_failed': 0, 'excluded_catalog_keys': 2}]}
        out = summarize(raw)
        self.assertEqual(out['pair_count'], 1)
        self.assertEqual(out['pairs_sha256'], digest([pair]))
        self.assertEqual(out['statement_outcomes'], {'accepted': 1, 'rejected': 1})
        self.assertEqual(out['errors'][0]['line'], 3)
        self.assertEqual(out['excluded_catalog_keys'], 2)
        self.assertNotIn(path, json.dumps(out))
        self.assertNotIn('subject text', json.dumps(out))
        self.assertEqual(raw['files'][0]['errors'][0], error)

    def test_pair_hash_is_semantic_json_not_layout(self):
        self.assertEqual(digest({'a': 1, 'b': 2}), digest({'b': 2, 'a': 1}))


if __name__ == '__main__':
    unittest.main()
