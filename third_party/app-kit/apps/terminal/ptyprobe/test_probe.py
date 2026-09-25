"""Dependency-free oracle/preflight checks; not OS/PTY execution evidence."""
import contextlib
import io
from pathlib import Path
import signal
import tempfile
import unittest
from unittest.mock import Mock, call, patch

from run import Probe, attrs_json, configuration


class ConfigurationTests(unittest.TestCase):
    def test_explicit_executable_paths_required(self):
        with contextlib.redirect_stderr(io.StringIO()):
            with self.assertRaises(SystemExit) as error:
                configuration([])
            self.assertEqual(error.exception.code, 2)
            with tempfile.TemporaryDirectory(prefix='appkit-pty-config-') as directory:
                binary = Path(directory) / 'fixture'
                binary.touch()
                with self.assertRaises(SystemExit):
                    configuration(['--binary', str(binary), '--deadline-binary', str(binary)])
                binary.chmod(0o700)
                result = configuration(['--binary', str(binary), '--deadline-binary', str(binary), '--case', 'sigint'])
                self.assertEqual(result.case, 'sigint')
                with self.assertRaises(SystemExit):
                    configuration(['--binary', 'relative', '--deadline-binary', str(binary)])
                with self.assertRaises(SystemExit):
                    configuration(['--binary', str(binary), '--deadline-binary', str(binary), '--inject-echo-drift'])

    def test_snapshot_keeps_every_field_and_pending_input_flag(self):
        value = [1, 2, 3, 0x200005CB, 9600, 9600, [b'\x00', b'\xff', 4]]
        snapshot = attrs_json(value)
        self.assertEqual(snapshot, {'iflag': 1, 'oflag': 2, 'cflag': 3,
                                   'lflag': 0x200005CB, 'ispeed': 9600, 'ospeed': 9600, 'cc': [0, 255, 4]})
        self.assertEqual(value[6], [b'\x00', b'\xff', 4])

    def test_repeat_accepts_declared_bounds_and_rejects_outside_them(self):
        with contextlib.redirect_stderr(io.StringIO()):
            with tempfile.TemporaryDirectory(prefix='appkit-pty-config-') as directory:
                binary = Path(directory) / 'fixture'
                binary.touch(mode=0o700)
                base = ['--binary', str(binary), '--deadline-binary', str(binary)]
                self.assertEqual(configuration([*base, '--repeat', '1']).repeat, 1)
                self.assertEqual(configuration([*base, '--repeat', '50']).repeat, 50)
                for repeat in ('0', '51', 'not-an-integer'):
                    with self.assertRaises(SystemExit):
                        configuration([*base, '--repeat', repeat])


class OracleTests(unittest.TestCase):
    def probe(self, data, cursor=0):
        probe = object.__new__(Probe)
        probe.name = 'synthetic-oracle'
        probe.raw = bytearray(data)
        probe.cursor = cursor
        probe.result = {'assertions': []}
        probe.pump = lambda *_: None
        return probe

    def test_old_frame_cannot_satisfy_later_action(self):
        data = b'Detail lines 4-9'
        probe = self.probe(data, len(data))
        with patch('run.time.monotonic', side_effect=[0, 7]):
            with self.assertRaises(AssertionError):
                probe.expect(data)
        self.assertFalse(probe.result['assertions'][-1]['pass'])

    def test_exact_full_row_or_pinned_delta_but_not_other_cursor(self):
        alternatives = ('Detail lines 4-9', b'\x1b[5;14H4-9 ')
        for data in (b'Detail lines 4-9', b'\x1b[5;14H4-9 '):
            probe = self.probe(data)
            probe.expect(alternatives)
            self.assertTrue(probe.result['assertions'][-1]['pass'])
        probe = self.probe(b'\x1b[6;14H4-9 ')
        with patch('run.time.monotonic', side_effect=[0, 7]):
            with self.assertRaises(AssertionError):
                probe.expect(alternatives)

    def test_missing_session_metadata_fails(self):
        probe = self.probe(b'APPKIT_PTY_EXIT')
        with patch('run.time.monotonic', side_effect=[0, 7]):
            with self.assertRaises(AssertionError):
                probe.metadata('EXIT')

    def test_cleanup_must_follow_shutdown_not_old_mouse_toggle(self):
        cleanup = b'\x1b[?1049l\x1b[?25h\x1b[?2004l\x1b[?1002l'
        for current in (b'', cleanup):
            probe = self.probe(cleanup + b'quitting' + current + b'APPKIT_PTY_EXIT 0\n')
            probe.shutdown_cursor = len(cleanup)
            if current:
                probe.check_cleanup()
            else:
                with self.assertRaises(AssertionError):
                    probe.check_cleanup()

    def test_cleanup_after_app_exit_cannot_count(self):
        probe = self.probe(b'APPKIT_PTY_EXIT 0\n\x1b[?1049l\x1b[?25h\x1b[?2004l\x1b[?1002l')
        probe.shutdown_cursor = 0
        with self.assertRaises(AssertionError):
            probe.check_cleanup()


class CleanupTests(unittest.TestCase):
    def probe(self, root, data=b'bounded transcript'):
        probe = object.__new__(Probe)
        probe.name = 'synthetic-cleanup'
        probe.root = Path(root)
        probe.master, probe.slave = 101, 102
        probe.raw = bytearray(data)
        probe.result = {'assertions': []}
        probe.child = Mock(pid=4242)
        probe.child.poll.return_value = 1
        probe.drain = Mock()
        return probe

    def test_overflow_during_final_drain_still_persists_and_closes(self):
        with tempfile.TemporaryDirectory(prefix='appkit-pty-cleanup-') as directory:
            probe = self.probe(directory)
            probe.drain.side_effect = RuntimeError('PTY transcript exceeded the declared 1 MiB bound')
            with patch('run.os.killpg', side_effect=ProcessLookupError), patch('run.os.close') as close:
                probe.close()
            transcript = Path(directory) / 'synthetic-cleanup.pty'
            self.assertEqual(transcript.read_bytes(), b'bounded transcript')
            self.assertEqual(probe.result['transcript_bytes'], len(b'bounded transcript'))
            self.assertIn('1 MiB bound', probe.result['cleanup_error'])
            self.assertEqual(close.call_args_list, [call(101), call(102)])

    def test_write_failure_retains_receipt_fields_and_closes(self):
        with tempfile.TemporaryDirectory(prefix='appkit-pty-cleanup-') as directory:
            probe = self.probe(directory)
            with (patch('run.os.killpg', side_effect=ProcessLookupError),
                  patch('run.Path.write_bytes', side_effect=OSError('disk full')),
                  patch('run.os.close') as close):
                probe.close()
            self.assertEqual(probe.result['transcript_bytes'], len(b'bounded transcript'))
            self.assertRegex(probe.result['transcript_sha256'], r'^[0-9a-f]{64}$')
            self.assertEqual(probe.result['cleanup_error'], 'disk full')
            self.assertEqual(close.call_count, 2)

    def test_surviving_owned_app_is_signaled_after_keeper_death(self):
        with tempfile.TemporaryDirectory(prefix='appkit-pty-cleanup-') as directory:
            probe = self.probe(directory)
            probe.app_pid = 4343
            signals = []

            def killpg(pgid, sent):
                signals.append((pgid, sent))
                if sent == 0:
                    raise ProcessLookupError

            with (patch('run.os.getpgid', side_effect=[probe.child.pid, ProcessLookupError(), ProcessLookupError()]),
                  patch('run.os.getsid', return_value=probe.child.pid),
                  patch('run.os.killpg', side_effect=killpg),
                  patch('run.os.close')):
                probe.close()
            self.assertTrue(probe.result['forced_cleanup'])
            self.assertEqual(signals, [(4242, signal.SIGCONT), (4242, signal.SIGTERM), (4242, 0)])
            self.assertNotIn('cleanup_error', probe.result)

    def test_unanchored_numeric_group_is_never_destructively_signaled(self):
        with tempfile.TemporaryDirectory(prefix='appkit-pty-cleanup-') as directory:
            probe = self.probe(directory)
            probe.app_pid = 4343
            signals = []

            def killpg(pgid, sent):
                signals.append((pgid, sent))

            with (patch('run.os.getpgid', return_value=9999),
                  patch('run.os.getsid') as getsid,
                  patch('run.os.killpg', side_effect=killpg),
                  patch('run.os.close')):
                probe.close()
            self.assertEqual(signals, [(4242, 0)])
            getsid.assert_not_called()
            self.assertNotIn('forced_cleanup', probe.result)
            self.assertIn('process group still present', probe.result['cleanup_error'])


if __name__ == '__main__':
    unittest.main()
