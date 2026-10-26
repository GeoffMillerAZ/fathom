import unittest
from analyze_misses import classify


class MissAttributionTests(unittest.TestCase):
    def test_case_is_counted_without_changing_score(self):
        self.assertEqual(classify(('a.ps1','Run','Target'),{('a.ps1','run','target')},{'Target'}),'case_only')

    def test_generated_mock_requires_actual_written_capture(self):
        target='PesterMock_script_Target_01234567-89ab-cdef-0123-456789abcdef'
        pair=('a.ps1','Run',target)
        self.assertEqual(classify(pair,{('a.ps1','Run','Target')},{'Target'}),'generated_mock_rebinding')
        self.assertEqual(classify(pair,set(),{'Target'}),'written_pair_absent')

    def test_native_alias_is_not_a_syntax_capture_miss(self):
        self.assertEqual(classify(('a.ps1','Run','Native'),{('a.ps1','Run','short')},{'short'}),'engine_target_differs_from_written')
