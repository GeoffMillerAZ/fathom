import copy
import unittest
from native_refs import native_output


class NativeEvidenceTest(unittest.TestCase):
    def setUp(self):
        self.protocol = {'version': '0.54.0', 'binary_sha256': 'binary'}
        self.report = {'subject_sha': 'pin', 'binary_sha256': 'binary', 'files': [
            {'path': '.circleci/config.yml', 'version': '0.54.0', 'yaml_v3_pairs': 1, 'sites': [
                {'scope': 'local', 'owner': 'build', 'target': 'setup', 'references_agree': True,
                 'definition_agrees': False}]}]}
        self.raw = {'tool': 'compiler and walk', 'version': 'cli', 'tier': 'C', 'definitions': [],
                    'references': [{'path': '.circleci/config.yml', 'from': 'workflow', 'to': 'build'},
                                   {'path': '.circleci/config.yml', 'from': 'build', 'to': 'setup'}]}

    def test_references_are_authority_even_when_definition_disagrees(self):
        result = native_output(self.raw, self.report, self.protocol, 'pin')
        self.assertEqual('A', result['tier'])
        self.assertEqual(self.raw['references'], result['references'])
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
