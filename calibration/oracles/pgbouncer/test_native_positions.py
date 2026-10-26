import os
import tempfile
import unittest
from pathlib import Path
from unittest.mock import patch
import native_positions as positions
from native_positions import MAX_EVENTS, MAX_LOG, MAX_SOURCE, STOP, candidates, decode, measure, reconcile, verify


class PositionReaderTest(unittest.TestCase):
    def test_native_population_cannot_be_selected_by_locator(self):
        events = [dict(kind='section', name='databases'), dict(kind='key', name='unused', value='host=127.0.0.1')]
        with self.assertRaisesRegex(ValueError, 'population'):
            positions.reconcile(events, [dict(kind='section', name='databases', line=1)])

    def test_all_caps_refuse_whole_input(self):
        with self.assertRaisesRegex(ValueError, 'cap'):
            candidates('x' * (MAX_SOURCE + 1))
        with self.assertRaisesRegex(ValueError, 'cap'):
            decode(b'x' * (MAX_LOG + 1))
        event = b' DEBUG parse_ini_file: [databases]\n'
        with self.assertRaisesRegex(ValueError, 'cap'):
            decode(event * (MAX_EVENTS + 1))

    def test_include_or_inline_unknown_syntax_is_never_omitted(self):
        for source in ['%include other.ini\n', '[pgbouncer] max_client_conn=1\n']:
            with self.assertRaisesRegex(ValueError, 'unsupported'):
                candidates(source)

    def test_unknown_native_callback_or_missing_boundary_refuses(self):
        for value in [b' DEBUG parse_ini_file: unknown event\n', b' DEBUG parse_ini_file: [pgbouncer]\n']:
            with self.assertRaises(ValueError):
                decode(value)

    def test_identity_drift_refuses_before_running(self):
        with tempfile.TemporaryDirectory() as directory:
            binary = Path(directory) / 'binary'
            binary.write_bytes(b'authored binary')
            with patch('native_positions.subprocess.check_output') as run:
                with self.assertRaisesRegex(ValueError, 'identity'):
                    verify(binary, dict(binary_sha256='0' * 64))
                run.assert_not_called()


@unittest.skipUnless(os.environ.get('PGB_POSITION_BIN'), 'set PGB_POSITION_BIN to the pinned stock binary')
class NativePositionTest(unittest.TestCase):
    def run_native(self, source):
        with tempfile.TemporaryDirectory() as directory:
            return measure(source, 'authored.ini', Path(os.environ['PGB_POSITION_BIN']), Path(directory))

    def test_native_unused_duplicate_repeated_and_quoted_tokens_have_exact_lines(self):
        source = "# café\n\n[databases]\n'unused' = host=127.0.0.1\n[pgbouncer]\nmax_client_conn=101\n[pgbouncer]\nmax_client_conn=102\n"
        definitions, receipt = self.run_native(source)
        self.assertEqual(6, receipt['native_callbacks'])
        self.assertEqual(4, len(definitions))
        self.assertIn(dict(path='authored.ini', name='pgbouncer', kind='section', line=5, end=8), definitions)
        self.assertIn(dict(path='authored.ini', name='max_client_conn', kind='key', line=8, end=8), definitions)
        self.assertIn(dict(path='authored.ini', name='unused', kind='target', line=4, end=4), definitions)

    def test_verbose_zero_never_hides_later_keys(self):
        definitions, receipt = self.run_native('[pgbouncer]\nverbose=0\nmax_client_conn=101\n')
        self.assertEqual(3, receipt['native_callbacks'])
        self.assertEqual(1, receipt['verbosity_values_observed'])
        self.assertEqual(3, len(definitions))

    def test_invalid_original_verbose_value_is_not_rescued(self):
        with self.assertRaisesRegex(ValueError, 'boundary'):
            self.run_native('[pgbouncer]\nverbose=invalid\nmax_client_conn=101\n')

    def test_invalid_numeric_value_followed_by_blank_lines_has_no_fallback(self):
        with self.assertRaisesRegex(ValueError, 'boundary'):
            self.run_native('[pgbouncer]\nmax_client_conn=invalid\n\n\n')

    def test_existing_logfile_key_is_preserved_with_private_destination(self):
        definitions, receipt = self.run_native('[pgbouncer]\nlogfile=/zz_oracle_nonexistent/never.log\nmax_client_conn=101\n')
        self.assertEqual(1, receipt['logfile_destinations_redirected'])
        self.assertIn(dict(path='authored.ini', name='logfile', kind='key', line=2, end=2), definitions)

    def test_syslog_profile_refuses_before_global_side_effects(self):
        with self.assertRaisesRegex(ValueError, 'syslog'):
            self.run_native('[pgbouncer]\nsyslog=1\n')

    def test_shifted_or_reversed_locator_lines_are_refused(self):
        source = '# prefix\n[pgbouncer]\n\n# pad\nmax_client_conn=101\n\n# tail\n\n'
        original = positions.candidates
        for shift in (1, 2, 'reverse'):
            def misplaced(value):
                sites = original(value)
                line_numbers = [row['line'] for row in sites]
                for index, row in enumerate(sites):
                    row['line'] = line_numbers[-1-index] if shift == 'reverse' else row['line'] + shift
                return sites
            with self.subTest(shift=shift), patch.object(positions, 'candidates', misplaced):
                with self.assertRaisesRegex(ValueError, 'prefix'):
                    self.run_native(source)


class PerBuildIdentityTest(unittest.TestCase):
    def test_new_build_receipt_pins_its_observed_binary_without_editing_history(self):
        import hashlib
        import json
        pin = json.loads((positions.HERE / 'native-position-build-r1.json').read_text())
        with tempfile.TemporaryDirectory() as directory:
            binary = Path(directory) / 'authored-binary'
            binary.write_bytes(b'authored per-build identity fixture')
            receipt = dict(pin, binary_sha256=hashlib.sha256(binary.read_bytes()).hexdigest())
            with patch('native_positions.subprocess.check_output', return_value=pin['version_output']):
                positions.verify(binary, receipt)
            broken = dict(receipt, native_source_sha256={})
            with self.assertRaisesRegex(ValueError,'source/archive'):
                positions.verify(binary, broken)
