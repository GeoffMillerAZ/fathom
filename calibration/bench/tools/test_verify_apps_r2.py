"""Mutation checks for the second application reading's evidence bindings."""
import sys
sys.dont_write_bytecode = True
import json
from pathlib import Path
import tempfile
import unittest
from verify_apps_r2 import verify, leaf_tests, go_leaf_names, product_path, skipped_go_tests, skipped_vitest_tests, skipped_pytest_tests, markdown_rows

READING = Path('docs/bench/readings/bench-prep/apps-r2.json')


class LeafCounting(unittest.TestCase):
    def test_markdown_preserves_backticks_and_escaped_delimiters(self):
        self.assertEqual(markdown_rows('| subject | `literal` name \\| quote |'),
                         [['subject', '`literal` name | quote']])

    def test_pytest_skip_message_is_evidence(self):
        raw = b'<testsuite><testcase classname="p" name="test"><skipped message="Windows test">guard</skipped></testcase></testsuite>'
        self.assertEqual(skipped_pytest_tests(raw), {('p', 'test'): ['Windows test', 'guard']})

    def test_tempdir_subtest_comparison_preserves_cases(self):
        def events(number):
            return [{'Action': 'run', 'Package': 'github.com/goreleaser/goreleaser/v2/cmd',
                     'Test': 'TestCheckPathLiteralExecutable//work/tmp/TestCheckPathLiteralExecutable'+number+'/'+case}
                    for case in ['001/tool', '002/tool']]
        one, two = events('123'), events('456')
        self.assertEqual(go_leaf_names(one, comparable=True), go_leaf_names(two, comparable=True))
        self.assertEqual(len(go_leaf_names(one, comparable=True)), 2)
        two[0]['Test'] += '-different'
        self.assertNotEqual(go_leaf_names(one, comparable=True), go_leaf_names(two, comparable=True))

    def test_skipped_parent_invocation_is_still_a_leaf(self):
        events = [
            {'Action': 'run', 'Package': 'p', 'Test': 'parent'},
            {'Action': 'run', 'Package': 'p', 'Test': 'parent/child'},
            {'Action': 'run', 'Package': 'p', 'Test': 'parent'},
            {'Action': 'output', 'Package': 'p', 'Test': 'parent', 'Output': 'missing database\n'},
            {'Action': 'skip', 'Package': 'p', 'Test': 'parent'}]
        self.assertEqual(go_leaf_names(events), {('p', 'parent'), ('p', 'parent/child')})
        self.assertEqual(set(skipped_go_tests(events)), {('p', 'parent')})

    def test_test_support_paths_are_not_product(self):
        for path in ['features/test_starter/src/pipeline.py', 'test-packages/foo/index.ts',
                     'internal/testlib/helper.go', 'rfc/drafts/proof.ts']:
            self.assertFalse(product_path(path), path)
        self.assertTrue(product_path('src/features/auth/index.ts'))

    def test_vitest_skip_identity_includes_occurrence(self):
        report = {'testResults': [{'name': '/work/source/test.ts', 'assertionResults': [
            {'fullName': 'same', 'status': 'passed'},
            {'fullName': 'same', 'status': 'pending'}]}]}
        self.assertEqual(set(skipped_vitest_tests(report)), {('/work/source/test.ts', 'same', 2)})

    def test_duplicate_vitest_names_count_as_leaves(self):
        report = {'numTotalTests': 3, 'testResults': [
            {'name': '/work/source/test.ts', 'assertionResults': [
                {'fullName': 'same name'}, {'fullName': 'same name'}, {'fullName': 'other name'}]}]}
        raw = json.dumps(report).encode()
        self.assertEqual(leaf_tests('TypeScript', {'one': raw, 'two': raw}, ['one', 'two']), 3)


class SecondReadingMutations(unittest.TestCase):
    def test_original(self):
        verify(READING)

    def mutation(self, change_json=None, change_reading=None):
        with tempfile.TemporaryDirectory() as temporary:
            target = Path(temporary) / READING.name
            data = json.loads(READING.read_text())
            reading = READING.with_suffix('.md').read_text()
            (target.parent / 'raw').symlink_to(READING.parent.resolve() / 'raw', target_is_directory=True)
            if change_json:
                change_json(data)
            if change_reading:
                reading = change_reading(reading, data)
            target.write_text(json.dumps(data))
            target.with_suffix('.md').write_text(reading)
            with self.assertRaises(AssertionError):
                verify(target)

    def test_reading_count(self):
        def change(reading, data):
            subject = data['subjects'][0]
            old = f"{subject['primary_lines']:,} / {subject['band']}"
            return reading.replace(old, f"{subject['primary_lines']+1:,} / {subject['band']}", 1)
        self.mutation(change_reading=change)

    def test_reading_exclusion_quote(self):
        def change(reading, data):
            subject = next(s for s in data['subjects'] if s['excluded'])
            quote = subject['excluded'][0]['log']
            self.assertIn(quote, reading)
            return reading.replace(quote, 'altered exclusion quote', 1)
        self.mutation(change_reading=change)

    def test_reading_skip_count(self):
        def change(data):
            next(s for s in data['subjects'] if s['status'] == 'kept')['env_skips'] += 1
        self.mutation(change_json=change)

    def test_reading_hash(self):
        self.mutation(change_reading=lambda reading, data: reading.replace(data['archive']['sha256'], '0'*64, 1))

    def test_uid_receipt(self):
        def change(data):
            next(s for s in data['subjects'] if s['status'] == 'kept')['suite']['warmup']['uid'] = 0
        self.mutation(change_json=change)

    def test_env_skip_reclassified_as_platform(self):
        def change(data):
            subject = next(s for s in data['subjects'] if s.get('env_skips'))
            subject['skipped_leaves'][0]['classification'] = 'platform'
            subject['env_skips'] -= 1
            subject['platform_skips'] += 1
        self.mutation(change_json=change)

    def test_archived_warmup(self):
        def change(data):
            suite = next(s['suite'] for s in data['subjects'] if s['suite'] and 'warmup' in s['suite'])
            suite['warmup']['exit_code'] += 1
        def reading_change(reading, data):
            suite = next(s['suite'] for s in data['subjects'] if s['suite'] and 'warmup' in s['suite'])
            warmup = suite['warmup']
            return reading.replace(f"{warmup['seconds']} / {warmup['exit_code']-1}",
                                   f"{warmup['seconds']} / {warmup['exit_code']}", 1)
        self.mutation(change_json=change, change_reading=reading_change)


if __name__ == '__main__':
    unittest.main()
