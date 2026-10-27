import unittest
from driver_outcome import outcome


class DriverOutcomeTests(unittest.TestCase):
    def test_zero_exit_does_not_hide_stopped_default_phase(self):
        result=outcome('passed 5, failed 0, total 5\npassed 2, failed 3, total 5\nP tests failed!\n',0,'completed')
        self.assertEqual(result['status'],'stopped-after-p-test-failures')
        self.assertEqual(result['test_results']['failed'],3)
        self.assertEqual(result['test_results']['passed'],7)

    def test_final_summary_wins_over_inner_test_summaries(self):
        result=outcome('Tests Passed: 1, Failed: 1, Skipped: 0, Inconclusive: 0, NotRun: 0\nTests Passed: 2,928, Failed: 0, Skipped: 3, Inconclusive: 0, NotRun: 0',0,'completed')
        self.assertEqual(result['test_results']['passed'],2928)
        self.assertEqual(result['status'],'tests-passed')

    def test_missing_summary_is_not_success(self):
        with self.assertRaises(ValueError):outcome('',0,'completed')
