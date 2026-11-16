import copy
import tempfile
from pathlib import Path
import unittest
from export_native_trace import export, read, MAX_BYTES, MAX_EVENTS


class ExportNativeTrace(unittest.TestCase):
    def setUp(self):
        self.rows = [dict(path=f'f{i}.lua', line=1, module=f'f{i}', to_path=f'm{i}.lua') for i in range(237)]
        self.population = {'imports': self.rows}

    def events(self, count):
        return [dict(row, accepted=True) for row in self.rows[:count]]

    def test_tier_and_statement_denominator(self):
        self.assertEqual(export(self.population, self.events(214), self.population)['tier'], 'A')
        self.assertEqual(export(self.population, self.events(213), self.population)['tier'], 'B')

    def test_failed_wrong_target_and_duplicate_events_have_no_truth(self):
        events = self.events(212)
        events += [dict(self.rows[212], accepted=False), dict(self.rows[212], accepted=True, to_path='wrong.lua')]
        events += self.events(212)
        result = export(self.population, events, self.population)
        self.assertEqual(result['tier'], 'B')
        self.assertEqual(len(result['imports']), 212)

    def test_scope_mutation_refused(self):
        changed = copy.deepcopy(self.population)
        changed['imports'][0]['line'] = 2
        with self.assertRaisesRegex(ValueError, 'population changed'):
            export(changed, self.events(237), self.population)

    def test_whole_unit_refusal_at_byte_cap_plus_one(self):
        with tempfile.TemporaryDirectory() as tmp:
            path = Path(tmp) / 'input'
            path.write_bytes(b' ' * (MAX_BYTES + 1))
            with self.assertRaisesRegex(ValueError, 'byte cap'):
                read(path)

    def test_whole_unit_refusal_at_event_cap_plus_one(self):
        with self.assertRaisesRegex(ValueError, 'event cap'):
            export(self.population, [{}] * (MAX_EVENTS + 1), self.population)


if __name__ == '__main__':
    unittest.main()
