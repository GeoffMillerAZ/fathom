"""A failed preparation must not become a green setup or reach any trials."""
import sys
sys.dont_write_bytecode = True
import json
from pathlib import Path
import subprocess
import tempfile
import unittest
from unittest.mock import patch
from remeasure import measure, shell_command


class FailedPreparation(unittest.TestCase):
    def check_failure(self, setup, cwd_exists):
        with tempfile.TemporaryDirectory() as temporary:
            job = Path(temporary) / 'workspace'
            source = job / 'subject'
            source.mkdir(parents=True)
            (source / 'go.sum').write_text('locked\n')
            (job / 'logs').mkdir()
            (job / 'receipts').mkdir()
            # Without staged Git metadata, the non-repository source fails staging.
            if not cwd_exists:
                # Staging succeeds according to its existing-metadata guard,
                # then changing to the source directory must fail.
                (job / 'builds' / 'fixture' / 'source').mkdir(parents=True)
                (job / 'builds' / 'fixture' / 'source' / '.git').mkdir()
            calls = []
            real_run = subprocess.run

            def run(argv, **kwargs):
                calls.append(argv)
                script = argv[-1].replace('/work', str(job / 'builds' / 'fixture')).replace('/subject', str(source))
                if not cwd_exists:
                    script = script.replace('cd ' + str(job / 'builds' / 'fixture' / 'source'), 'cd /bench-deliberately-absent')
                # Unexpected later phases are counted, never executed on the host.
                if 'INSTALLER_REACHED' not in script:
                    return subprocess.CompletedProcess(argv, 7)
                return real_run(['sh', '-c', script], **kwargs)

            def git(argv, **kwargs):
                return 'a' * 40 + '\n' if 'rev-parse' in argv else ''

            config = {'id': 'fixture', 'language': 'Go', 'source': str(source),
                      'lock': 'go.sum', 'setup': setup, 'test': 'true'}
            with patch('remeasure.subprocess.run', side_effect=run), patch('remeasure.subprocess.check_output', side_effect=git):
                result = measure(config, job)
            self.assertNotEqual(result['setup']['exit_code'], 0)
            self.assertEqual(len(calls), 1)
            self.assertEqual(result['runs'], [])
            self.assertNotIn('INSTALLER_REACHED', (job / 'logs' / 'fixture-setup.log').read_text())
            self.assertEqual(json.loads((job / 'receipts' / 'fixture-suite.json').read_text())['runs'], [])

    def test_failed_staging_stops_before_installer(self):
        self.check_failure('export UNUSED=1; printf INSTALLER_REACHED', True)

    def test_failed_working_directory_stops_before_installer(self):
        self.check_failure('export UNUSED=1; printf INSTALLER_REACHED', False)

    def test_successful_semicolon_body_runs_in_staged_source(self):
        with tempfile.TemporaryDirectory() as temporary:
            work = Path(temporary) / 'workspace'
            (work / 'source' / '.git').mkdir(parents=True)
            command = shell_command('TypeScript', 'setup',
                                    'export MARKER=success; test -d .git; printf %s "$MARKER"')
            command = 'corepack() { return 0; }; ' + command.replace('/work', str(work))
            result = subprocess.run(['sh', '-c', command], capture_output=True, text=True)
            self.assertEqual(result.returncode, 0)
            self.assertEqual(result.stdout, 'success')


if __name__ == '__main__':
    unittest.main()
