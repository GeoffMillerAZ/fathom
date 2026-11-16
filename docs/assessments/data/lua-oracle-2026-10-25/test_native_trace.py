"""Integration assertion over native traces; never consumes Fathom output."""
import json
import os
from pathlib import Path
import unittest


class TelescopeNativeCoverage(unittest.TestCase):
    def test_frozen_population_and_executed_coverage(self):
        root = Path(os.environ['LUA_TRACE_RESULT'])
        frozen = json.loads(Path(__file__).with_name('telescope-native-population.json').read_text())
        population = json.loads((root / 'population.json').read_text())
        key = lambda row: (row['path'], row['line'], row['module'], row['to_path'])
        self.assertEqual(sorted(map(key, frozen['imports'])), sorted(map(key, population['imports'])))
        self.assertEqual(len(population['imports']), 237)
        observed = set()
        for path in root.glob('*.jsonl'):
            for line in path.read_text().splitlines():
                row = json.loads(line)
                if row.get('accepted') and row.get('to_path'):
                    observed.add(key(row))
        covered = set(map(key, population['imports'])) & observed
        self.assertGreaterEqual(len(covered), 213, f'{len(covered)}/237 native sites executed')
        self.assertGreaterEqual(len(covered) * 10, 237 * 9)
        self.assertEqual(json.loads((root / 'summary.json').read_text())['test_exit'], 0)


if __name__ == '__main__':
    unittest.main()
