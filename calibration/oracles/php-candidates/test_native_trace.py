import json
from pathlib import Path
import tempfile
import unittest

from trace import decode


class NativeAnswerTests(unittest.TestCase):
    def setUp(self):
        self.workspace = Path(__file__).resolve().parents[3]/'workspace/tmp'
        self.workspace.mkdir(parents=True, exist_ok=True)

    def test_native_caller_is_not_the_syntax_walk_pair(self):
        with tempfile.TemporaryDirectory(dir=self.workspace) as temp:
            output = Path(temp)
            root = output/'subject'
            root.mkdir()
            filename = str(root/'file.php')
            (output/'reflection-1.json').write_text(json.dumps({'functions': [
                {'file': filename, 'name': 'Native\\actual_caller', 'start': 1, 'end': 10}]}))
            (output/'trace.1.xt').write_text(
                '1\t0\t0\t0.0\t1\tNative\\actual_caller\t1\t\t'+filename+'\t0\n'
                '2\t1\t0\t0.0\t1\tNative\\target\t1\t\t'+filename+'\t3\n'
                '2\t2\t0\t0.0\t1\tNative\\resolved_alias\t1\t\t'+filename+'\t4\n')
            syntax = {'files': 1, 'sites': [
                {'path': 'file.php', 'line': 3, 'from': 'parser_guess', 'written_name': 'target'},
                {'path': 'file.php', 'line': 4, 'from': 'parser_guess', 'written_name': 'written_alias'}]}
            oracle, summary = decode(root, output, syntax)
            self.assertEqual(oracle['calls'], [{'path': 'file.php', 'from': 'actual_caller', 'to': 'target'}])
            self.assertEqual(summary['unattested_sites'], 1)
            self.assertEqual(summary['native_counts']['unpaired_native_entries'], 1)

    def test_implicit_callback_does_not_attest_a_coincident_literal_name(self):
        with tempfile.TemporaryDirectory(dir=self.workspace) as temp:
            output = Path(temp)
            root = output/'subject'
            root.mkdir()
            filename = str(root/'file.php')
            (output/'reflection-1.json').write_text(json.dumps({'functions': [
                {'file': filename, 'name': 'driver', 'start': 1, 'end': 10}]}))
            (output/'trace.1.xt').write_text(
                '1\t1\t0\t0.0\t1\tarray_map\t0\t\t'+filename+'\t3\n'
                '2\t2\t0\t0.0\t1\ttarget\t1\t\t'+filename+'\t3\n')
            syntax = {'files': 1, 'sites': [
                {'path': 'file.php', 'line': 3, 'from': 'driver', 'written_name': 'target'}]}
            oracle, summary = decode(root, output, syntax)
            self.assertEqual(oracle['calls'], [])
            self.assertEqual(summary['attested_sites'], 0)

    def test_dynamic_call_on_same_line_cannot_attest_unexecuted_literal(self):
        with tempfile.TemporaryDirectory(dir=self.workspace) as temp:
            output = Path(temp)
            root = output/'subject'
            root.mkdir()
            filename = str(root/'file.php')
            (output/'reflection-1.json').write_text(json.dumps({'functions': [
                {'file': filename, 'name': 'driver', 'start': 1, 'end': 10}]}))
            (output/'trace.1.xt').write_text(
                '1\t1\t0\t0.0\t1\tdriver\t1\t\t'+filename+'\t1\n'
                '2\t2\t0\t0.0\t1\ttarget\t1\t\t'+filename+'\t3\n')
            syntax = {'files': 1, 'sites': [
                {'path': 'file.php', 'line': 3, 'from': 'driver', 'written_name': 'target'}],
                'nonliteral_call_lines': [{'path': 'file.php', 'line': 3, 'end': 3}]}
            oracle, summary = decode(root, output, syntax)
            self.assertEqual(oracle['calls'], [])
            self.assertEqual(summary['attested_sites'], 0)


if __name__ == '__main__':
    unittest.main()
