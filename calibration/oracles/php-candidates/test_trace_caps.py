from dataclasses import replace
import json
from pathlib import Path
import tempfile
import unittest

from trace import Limits, write_collection


class TraceCapsTests(unittest.TestCase):
    def setUp(self):
        workspace = Path(__file__).resolve().parents[3]/'workspace/tmp'
        workspace.mkdir(parents=True, exist_ok=True)
        self.temp = tempfile.TemporaryDirectory(dir=workspace)
        self.addCleanup(self.temp.cleanup)
        self.output = Path(self.temp.name)
        self.root = self.output/'subject'
        self.root.mkdir()
        self.trace = self.output/'trace.1.xt'
        self.trace.write_text('TRACE END   [2026-10-26 00:00:00]\n')
        self.reflection = self.output/'reflection-1.json'
        self.reflection.write_text(json.dumps({'functions': []}))
        self.syntax = {'files': 1, 'sites': []}

    def refuse(self, message, limits=Limits(), status=0):
        # A failed re-decode must remove a stale successful oracle too.
        (self.output/'oracle.json').write_text('{"tier":"A"}\n')
        with self.assertRaisesRegex(ValueError, message):
            write_collection(self.root, self.output, self.syntax, '8.5.8', status, limits)
        self.assertFalse((self.output/'oracle.json').exists())
        self.assertTrue(json.loads((self.output/'refusal.json').read_text())['refused'])

    def test_trace_file_count_cap_plus_one(self):
        (self.output/'trace.2.xt').write_bytes(self.trace.read_bytes())
        self.refuse('trace file count cap', replace(Limits(), trace_files=1))

    def test_reflection_file_count_cap_plus_one(self):
        (self.output/'reflection-2.json').write_bytes(self.reflection.read_bytes())
        self.refuse('reflection file count cap', replace(Limits(), reflection_files=1))

    def test_total_trace_byte_cap_plus_one(self):
        self.refuse('total trace byte cap', replace(Limits(), trace_bytes=self.trace.stat().st_size-1))

    def test_per_trace_byte_cap_plus_one(self):
        self.refuse('per-trace byte cap', replace(Limits(), per_trace_bytes=self.trace.stat().st_size-1))

    def test_reflection_byte_cap_plus_one(self):
        self.refuse('reflection byte cap', replace(Limits(), reflection_bytes=self.reflection.stat().st_size-1))

    def test_trace_line_byte_cap_plus_one(self):
        self.refuse('trace line byte cap', replace(Limits(), line_bytes=self.trace.stat().st_size-1))

    def test_stack_depth_cap_plus_one(self):
        self.trace.write_text('3\t1\t0\t0.0\t1\ttarget\t1\t\tfile.php\t3\nTRACE END   [2026-10-26 00:00:00]\n')
        self.refuse('stack depth cap', replace(Limits(), depth=2))

    def test_truncated_trace_refuses_whole_collection(self):
        self.trace.write_text('TRACE START\n')
        self.refuse('incomplete trace')

    def test_timeout_refuses_whole_collection(self):
        self.refuse('timed out', status='timeout')

    def test_footer_text_inside_payload_is_not_completion(self):
        self.trace.write_text('1\t1\t0\tTRACE END   [2026-10-26 00:00:00]\n')
        self.refuse('incomplete trace')

    def test_signal_refuses_whole_collection(self):
        self.refuse('signalled', status=-25)

    def test_complete_suite_failure_is_disclosed_not_incomplete(self):
        oracle, summary = write_collection(self.root, self.output, self.syntax, '8.5.8', 2)
        self.assertEqual(oracle['tier'], 'A')
        self.assertTrue(summary['trace_complete'])


if __name__ == '__main__':
    unittest.main()
