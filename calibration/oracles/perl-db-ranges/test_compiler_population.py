"""Known-limitation control, not proof of complete native call coverage."""
import os
from pathlib import Path
import subprocess
import unittest


class CompilerPopulation(unittest.TestCase):
    def test_native_population_known_constant_target_loss(self):
        control = Path(__file__).with_name('compiler_population_control.pl')
        for mode, flags in [('normal', []), ('noopt', ['-d'])]:
            with self.subTest(mode=mode):
                env = dict(os.environ, LC_ALL='C', LANG='C', LC_CTYPE='C')
                env.pop('PERL5DB', None)
                if mode == 'noopt':
                    env['PERL5DB'] = 'BEGIN { $^P = 0x14; }'
                probe = subprocess.run(
                    ['/usr/bin/sandbox-exec', '-p',
                     '(version 1) (allow default) (deny network*)',
                     '/usr/bin/perl', *flags, '-c', str(control)],
                    env=env, stdout=subprocess.PIPE, stderr=subprocess.STDOUT,
                    text=True, timeout=30)
                raw = probe.stdout
                self.assertEqual(probe.returncode, 0, raw)
                self.assertIn(f'{control} syntax OK', raw)
                self.assertEqual(raw.splitlines().count('ANONYMOUS_CVS=1'), 1)
                for section in ['constant_caller', 'literal_caller', 'anonymous']:
                    self.assertEqual(raw.splitlines().count(f'CONTROL={section}'), 1)
                constant = raw.split('CONTROL=constant_caller\n', 1)[1].split(
                    'CONTROL=literal_caller\n', 1)[0]
                anonymous = raw.split('CONTROL=anonymous\n', 1)[1]
                self.assertIn('gv[*CompilerPopulationControl::target]', anonymous)
                self.assertIn('const[IV 7]', constant)
                self.assertNotIn(
                    'gv[*CompilerPopulationControl::native_constant]', constant,
                    'native constant target recovered: revisit the HOLD assessment '
                    'and test the full capability bar')


if __name__ == '__main__':
    unittest.main()
