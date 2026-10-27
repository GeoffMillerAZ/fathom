#!/usr/bin/env python3
import os, pathlib, subprocess, tempfile, unittest
from trace_driver import decode, events
HERE = pathlib.Path(__file__).resolve().parent

class NativeTrace(unittest.TestCase):

    def setUp(self):
        self.workspace = HERE.parents[2] / 'workspace/tmp'
        self.workspace.mkdir(parents=True, exist_ok=True)

    def test_real_sources_not_cross_file_function_or_failed_or_empty(self):
        with tempfile.TemporaryDirectory(dir=self.workspace) as d:
            trace = pathlib.Path(d).resolve() / 'events'
            trace.touch()
            env = os.environ.copy()
            env.update(BASH_ENV=str(HERE / 'trace_hook.sh'), ORACLE_TRACE_FILE=str(trace))
            p = subprocess.run(['/bin/bash', str(HERE / 'fixtures/probe.sh')], env=env, capture_output=True)
            self.assertEqual(p.returncode, 0, p.stderr)
            (rows, unknown) = decode(trace, HERE / 'fixtures')
            self.assertEqual(unknown, 0)
            self.assertEqual(rows, [('one/child.sh', 1, 'leaf.sh'), ('probe.sh', 3, 'leaf.sh'), ('probe.sh', 7, 'leaf.sh'), ('probe.sh', 10, 'one/child.sh'), ('probe.sh', 11, 'defs.sh'), ('probe.sh', 17, 'leaf.sh')])

    def test_shadowed_source_is_not_a_native_import(self):
        with tempfile.TemporaryDirectory(dir=self.workspace) as d:
            trace = pathlib.Path(d).resolve() / 'events'
            trace.touch()
            env = os.environ.copy()
            env.update(BASH_ENV=str(HERE / 'trace_hook.sh'), ORACLE_TRACE_FILE=str(trace))
            result = subprocess.run(['/bin/bash', str(HERE / 'fixtures/shadow-source.sh')],
                                    cwd=HERE / 'fixtures', env=env, capture_output=True)
            self.assertEqual(result.returncode, 0, result.stderr)
            self.assertEqual(decode(trace, HERE / 'fixtures'), ([], 0))

    def test_debug_observer_preserves_errexit_conditionals_and_status(self):
        with tempfile.TemporaryDirectory(dir=self.workspace) as d:
            trace=pathlib.Path(d)/'events';trace.touch()
            env=dict(os.environ,LC_ALL='C',BASH_ENV=str(HERE/'trace_hook.sh'),ORACLE_TRACE_FILE=str(trace))
            result=subprocess.run(['/bin/bash',str(HERE/'fixtures/strict.sh')],env=env,capture_output=True)
            self.assertEqual(result.returncode,0,result.stderr)
            self.assertEqual(result.stdout,b'survived\n')
            self.assertEqual(result.stderr,b'')

    def test_return_observer_does_not_trigger_subject_err_trap(self):
        with tempfile.TemporaryDirectory(dir=self.workspace) as d:
            trace=pathlib.Path(d)/'events';trace.touch()
            calls=pathlib.Path(d)/'calls';calls.touch()
            env=dict(os.environ,LC_ALL='C',BASH_ENV=str(HERE/'entry_hook.sh'),ORACLE_TRACE_FILE=str(trace),ORACLE_CALL_TRACE_FILE=str(calls))
            result=subprocess.run(['/bin/bash',str(HERE/'fixtures/strict.sh')],env=env,capture_output=True)
            self.assertEqual(result.returncode,0,result.stderr)
            self.assertEqual(result.stdout,b'survived\n')
            self.assertEqual(result.stderr,b'')
            self.assertIn(b'CALL\0',calls.read_bytes())

    def test_missing_native_cwd_is_unknown_not_process_cwd(self):
        with tempfile.TemporaryDirectory(dir=self.workspace) as d:
            trace = pathlib.Path(d) / 'events'
            caller = str(HERE / 'fixtures/probe.sh')
            trace.write_bytes(('ROOT\0' + caller + '\0\0ENTRY\0leaf.sh\0' + caller + '\0' + '1' + '\0\0').encode())
            self.assertEqual(decode(trace, HERE / 'fixtures'), ([], 1))

    def test_malformed_native_trace_refuses(self):
        with tempfile.TemporaryDirectory(dir=self.workspace) as d:
            trace = pathlib.Path(d) / 'events'
            trace.write_bytes(b'garbage\x00')
            with self.assertRaises(ValueError):
                decode(trace, HERE / 'fixtures')

    def test_incomplete_tail_cannot_silently_disappear(self):
        with tempfile.TemporaryDirectory(dir=self.workspace) as d:
            trace = pathlib.Path(d) / 'events'
            trace.write_bytes(b'ROOT\x00file.sh\x00.\x00partial')
            with self.assertRaisesRegex(ValueError, 'incomplete trailing field'):
                decode(trace, HERE / 'fixtures')

    def test_each_native_cap_refuses_limit_plus_one(self):
        with tempfile.TemporaryDirectory(dir=self.workspace) as d:
            trace = pathlib.Path(d) / 'events'
            trace.write_bytes(b'ROOT\x00file.sh\x00.\x00')
            for kwargs, reason in [({'max_bytes': trace.stat().st_size-1}, 'byte cap'),
                                   ({'max_fields': 2}, 'field count cap'),
                                   ({'max_field_bytes': 6}, 'field byte cap')]:
                with self.subTest(kwargs=kwargs), self.assertRaisesRegex(ValueError, reason):
                    list(events(trace, **kwargs))
if __name__ == '__main__':
    unittest.main()
