import copy
import unittest
from native_refs import native_output


class NativeEvidenceTest(unittest.TestCase):
    def setUp(self):
        self.protocol = {'version': '0.54.0', 'binary_sha256': 'binary'}
        self.report = {'subject_sha': 'pin', 'binary_sha256': 'binary', 'files': [
            {'path': '.circleci/config.yml', 'version': '0.54.0', 'yaml_v3_pairs': 1, 'all_declarations_queried': True, 'declarations': [
                {'target': 'setup', 'family': 'command', 'references': [
                    {'owner': 'build', 'target': 'setup', 'classification': 'local-use'}]}], 'sites': [
                {'scope': 'local', 'owner': 'build', 'target': 'setup', 'references_agree': True,
                 'definition_agrees': False}]}]}
        self.raw = {'tool': 'compiler and walk', 'version': 'cli', 'tier': 'C', 'definitions': [],
                    'references': [{'path': '.circleci/config.yml', 'from': 'workflow', 'to': 'build'},
                                   {'path': '.circleci/config.yml', 'from': 'build', 'to': 'setup'}]}

    def test_references_are_authority_even_when_definition_disagrees(self):
        result = native_output(self.raw, self.report, self.protocol, 'pin')
        self.assertEqual('A', result['tier'])
        self.assertEqual(sorted(self.raw['references'], key=lambda r: (r['path'], r['from'], r['to'])), result['references'])
        self.assertEqual('C', self.raw['tier'])

    def test_native_reference_miss_never_falls_back_to_walk(self):
        self.report['files'][0]['sites'][0]['references_agree'] = False
        with self.assertRaises(ValueError):
            native_output(self.raw, self.report, self.protocol, 'pin')

    def test_rejects_wrong_tool_subject_or_population(self):
        for field, value in [('subject_sha', 'wrong'), ('binary_sha256', 'wrong')]:
            report = copy.deepcopy(self.report)
            report[field] = value
            with self.assertRaises(ValueError):
                native_output(self.raw, report, self.protocol, 'pin')
        self.report['files'][0]['sites'][0]['owner'] = 'other'
        with self.assertRaises(ValueError):
            native_output(self.raw, self.report, self.protocol, 'pin')

    def test_rejects_wrong_server_version(self):
        self.report['files'][0]['version'] = 'other'
        with self.assertRaises(ValueError):
            native_output(self.raw, self.report, self.protocol, 'pin')

    def test_native_extra_pairs_are_kept_in_recall_truth(self):
        self.report['files'][0]['declarations'].append({'target': 'linux', 'family': 'executor', 'references': [
            {'owner': 'build', 'target': 'linux', 'classification': 'workflow-executor-parameter'}]})
        result = native_output(self.raw, self.report, self.protocol, 'pin')
        self.assertIn({'path': '.circleci/config.yml', 'from': 'build', 'to': 'linux'}, result['references'])

    def test_namespace_error_requires_compiler_adjudication(self):
        self.report['files'][0]['declarations'][0]['references'].append(
            {'owner': 'workflow', 'target': 'setup', 'classification': 'same-name-workflow-job-oracle-error'})
        with self.assertRaisesRegex(ValueError, 'compiler adjudication'):
            native_output(self.raw, self.report, self.protocol, 'pin')
        result = native_output(self.raw, self.report, self.protocol, 'pin', [
            {'sha': 'pin', 'family': 'command', 'target': 'setup', 'agrees': True}])
        self.assertNotIn({'path': '.circleci/config.yml', 'from': 'workflow', 'to': 'setup'}, result['references'])
