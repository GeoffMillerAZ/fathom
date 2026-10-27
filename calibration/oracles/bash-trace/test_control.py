from pathlib import Path
import tempfile
import unittest

from control import outcomes


class DriverOutcomeTests(unittest.TestCase):
    def setUp(self):
        parent = Path(__file__).resolve().parents[3] / 'workspace/tmp'
        parent.mkdir(parents=True, exist_ok=True)
        temporary = tempfile.TemporaryDirectory(dir=parent)
        self.addCleanup(temporary.cleanup)
        self.log = Path(temporary.name) / 'driver.log'

    def test_skip_is_not_a_pass_and_failure_is_retained(self):
        self.log.write_text('1..3\nok 1 ordinary\nok 2 unavailable # skip platform\nnot ok 3 failed assertion\n')
        counts, failures = outcomes(self.log)
        self.assertEqual(counts, dict(tests=3, passed=1, skipped=1, failed=1))
        self.assertEqual(failures, {'failed assertion'})

    def test_truncated_or_repeated_results_cannot_certify_driver_completion(self):
        for text in ['', '1..2\nok 1 partial\n', '1..2\nok 1 first\nok 1 repeated\n']:
            with self.subTest(text=text):
                self.log.write_text(text)
                with self.assertRaisesRegex(ValueError, 'whole-unit refusal'):
                    outcomes(self.log)
