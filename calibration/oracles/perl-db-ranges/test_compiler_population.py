"""Target test for the compiler-only control; failure is a disclosed route limit."""
import os
from pathlib import Path
import unittest


class CompilerPopulation(unittest.TestCase):
    def test_native_population_retains_anonymous_and_constant_calls(self):
        raw = Path(os.environ['PERL_POPULATION_TRACE']).read_text()
        self.assertIn('ANONYMOUS_CVS=1', raw)
        self.assertIn('CONTROL=anonymous', raw)
        self.assertIn('gv[*CompilerPopulationControl::target]', raw)
        constant = raw.split('CONTROL=constant_caller\n', 1)[1].split('CONTROL=literal_caller\n', 1)[0]
        self.assertIn('gv[*CompilerPopulationControl::native_constant]', constant,
                      'post-optimization native op tree lost the constant source call')


if __name__ == '__main__':
    unittest.main()
