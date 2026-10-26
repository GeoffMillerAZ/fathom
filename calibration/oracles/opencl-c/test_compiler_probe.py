import importlib.util
from pathlib import Path
import unittest
import subprocess
import sys
import tempfile
from types import SimpleNamespace

spec = importlib.util.spec_from_file_location('compiler_probe', Path(__file__).with_name('compiler_probe.py'))
probe = importlib.util.module_from_spec(spec)
spec.loader.exec_module(probe)


class CompilerProbeTests(unittest.TestCase):
    def test_compiler_snapshot_uses_captured_bytes_after_original_changes(self):
        with tempfile.TemporaryDirectory() as td:
            root = Path(td)
            source = root / 'original' / 'nested' / 'kernel.cl'
            source.parent.mkdir(parents=True)
            source.write_bytes(b'verified source')
            checkout = SimpleNamespace(root=root / 'original', eligible_sources={source: source.read_bytes()})
            source.write_bytes(b'transient source change')
            private = root / 'private'
            private.mkdir()
            snapshot = probe.snapshot_sources(checkout, private)
            self.assertEqual((snapshot / 'nested/kernel.cl').read_bytes(), b'verified source')
            self.assertEqual(source.read_bytes(), b'transient source change')
            self.assertEqual((snapshot / 'nested/kernel.cl').stat().st_mode & 0o222, 0)

    def test_pins_remain_enforced_under_optimized_python(self):
        with tempfile.TemporaryDirectory() as td:
            root = Path(td)
            compiler = root / 'bad-clang'
            compiler.write_bytes(b'not the pinned compiler')
            result = subprocess.run([sys.executable, '-I', '-B', '-O', str(Path(__file__).with_name('compiler_probe.py')),
                                     '--clang', str(compiler), '--resource-dir', str(root),
                                     '--sdk', str(root), '--out', str(root / 'output')],
                                    capture_output=True, text=True)
            self.assertNotEqual(result.returncode, 0)
            self.assertIn('compiler pin mismatch', result.stderr)
            self.assertFalse((root / 'output').exists())

    def test_exact_kernel_attribute_kinds(self):
        for kind, expected in [('DeviceKernelAttr', True), ('OpenCLKernelAttr', True), ('FormatAttr', False)]:
            with self.subTest(kind=kind):
                node = {'kind': 'FunctionDecl', 'inner': [{'kind': kind}]}
                self.assertEqual(probe.is_kernel(node), expected)

    def test_calls_are_resolved_function_calls_not_vector_construction(self):
        call = lambda name: {'kind': 'CallExpr', 'inner': [{'kind': 'ImplicitCastExpr', 'inner': [{'kind': 'DeclRefExpr', 'referencedDecl': {'kind': 'FunctionDecl', 'name': name}}]}]}
        ast = {'inner': [
            {'kind': 'FunctionDecl', 'name': 'helper', 'range': {'end': {'offset': 10, 'col': 1, 'tokLen': 1}}, 'inner': [{'kind': 'CompoundStmt'}]},
            {'kind': 'FunctionDecl', 'name': 'kernel', 'range': {'end': {'offset': 20, 'col': 1, 'tokLen': 1}}, 'inner': [{'kind': 'DeviceKernelAttr'}, {'kind': 'CompoundStmt', 'inner': [call('helper'), call('get_global_id'), {'kind': 'CompoundLiteralExpr'}]}]},
        ]}
        got = probe.summarize_ast(ast)
        self.assertEqual(got['kernel_definitions'], 1)
        self.assertEqual(got['call_sites'], 2)
        self.assertEqual(got['source_function_call_sites'], 1)
        self.assertEqual(got['external_call_sites'], 1)
        self.assertEqual(got['vector_literal_nodes'], 1)
        self.assertEqual(got['definition_end_ranges'], 2)


if __name__ == '__main__':
    unittest.main()
