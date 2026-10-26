import unittest
from run_traces import validate_driver


class DriverCompletionTests(unittest.TestCase):
    def test_timeout_refuses(self):
        with self.assertRaisesRegex(ValueError,'whole-unit refusal'):
            validate_driver('timeout','')

    def test_signal_refuses(self):
        with self.assertRaisesRegex(ValueError,'whole-unit refusal'):
            validate_driver(-9,'')

    def test_instrumentation_traceback_refuses(self):
        with self.assertRaisesRegex(ValueError,'instrumentation traceback'):
            validate_driver(1,'trace_hook.rb:23:in block: NoMethodError')

    def test_normal_suite_failure_remains_disclosed(self):
        validate_driver(1,'957 tests, 4 failures, 0 errors')


if __name__=='__main__':unittest.main()
