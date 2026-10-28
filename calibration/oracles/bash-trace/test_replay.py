import unittest

from replay import oracle_payload


class DiagnosticTierTests(unittest.TestCase):
    def test_perfect_observed_coverage_cannot_promote_independent_source_population(self):
        native = dict(bash_version='5.3.20', coverage=1.0,
                      function_pairs=[dict(path='input.sh', **{'from':'driver','to':'actual_target'})])
        imports = [dict(path='input.sh', line=1, to_path='actual_opened.sh', module='written.sh')]
        source = oracle_payload('source', native, imports)
        executed = oracle_payload('calls', native, imports)
        self.assertEqual(source['tier'], 'C')
        self.assertNotIn('calls', source)
        self.assertEqual(executed['tier'], 'A')
        self.assertEqual(executed['calls'][0]['to'], 'actual_target')
        self.assertNotIn('imports', executed)
