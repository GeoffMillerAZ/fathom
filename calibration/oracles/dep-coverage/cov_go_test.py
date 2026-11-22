"""Fixture controls for COV-M3. No corpus input is used here."""
import importlib.util
from pathlib import Path
import tempfile
import unittest

spec = importlib.util.spec_from_file_location('cov_go', Path(__file__).with_name('cov_go.py'))
cov_go = importlib.util.module_from_spec(spec)
spec.loader.exec_module(cov_go)


class NativeControl(unittest.TestCase):
    def fixture(self, root):
        files = {
            'go.mod': 'module example.com/covcontrol\n\ngo 1.26\n',
            'app/a.go': 'package app\nimport _ "example.com/covcontrol/lib"\n',
            'app/b.go': 'package app\nimport _ "fmt"\n',
            'app/a_test.go': 'package app\nimport "testing"\nfunc TestA(t *testing.T) {}\n',
            'app/x_test.go': 'package app_test\nimport _ "example.com/covcontrol/lib"\n',
            'app/no_windows.go': 'package app\n',
            'app/native.go': 'package app\nimport "C"\n',
            'app/embed.go': 'package app\nimport _ "embed"\n//go:embed payload.txt\nvar payload string\n',
            'app/payload.txt': 'payload',
            'lib/lib.go': 'package lib\n',
        }
        for name, src in files.items():
            p = root / name
            p.parent.mkdir(parents=True, exist_ok=True)
            p.write_text(src)
        return sorted(n for n in files if n.endswith('.go'))

    def test_native_membership_and_file_attribution(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            selected = self.fixture(root)
            first = cov_go.capture(root, selected, '0')
            rows = {r['Path']: r['Edges'] for r in first['truth']}
            self.assertEqual(first['gaps']['app/no_windows.go'], 'native-build-excluded')
            self.assertEqual(first['gaps']['app/native.go'], 'native-build-excluded')
            self.assertIn('app/a_test.go', rows)
            self.assertIn('app/x_test.go', rows)
            imports = lambda n: [e for e in rows[n] if e['Kind'] == 'import']
            self.assertEqual([e['To'] for e in imports('app/a.go')], ['lib'])
            self.assertEqual([e['To'] for e in imports('app/b.go')], ['go:fmt'])
            self.assertEqual([e['To'] for e in imports('app/x_test.go')], ['lib'])
            self.assertTrue(any(e['Kind'] == 'embed' and e['To'] == 'app/payload.txt' for e in rows['app/embed.go']))
            before = cov_go.digest(first['truth'])
            # Observation mutation has no route into capture; source mutation does.
            fake_observation = [{'Path': 'app/a.go', 'Edges': []}]
            self.assertEqual(before, cov_go.digest(cov_go.capture(root, selected, '0')['truth']))
            self.assertTrue(fake_observation)
            (root / 'app/a.go').write_text('package app\n')
            self.assertNotEqual(before, cov_go.digest(cov_go.capture(root, selected, '0')['truth']))

    def test_nested_module_and_unlisted_denominators(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            selected = self.fixture(root)
            extra = {
                'nested/go.mod': 'module example.com/nested\n\ngo 1.26\n',
                'nested/a.go': 'package nested\nimport _ "example.com/nested/sub"\n',
                'nested/sub/a.go': 'package sub\n',
                'testdata/unlisted.go': 'package unlisted\n',
            }
            for name, text in extra.items():
                p = root / name
                p.parent.mkdir(parents=True, exist_ok=True)
                p.write_text(text)
            selected += [n for n in extra if n.endswith('.go')]
            result = cov_go.capture(root, sorted(selected), '0')
            rows = {r['Path']: r['Edges'] for r in result['truth']}
            self.assertEqual(result['gaps']['testdata/unlisted.go'], 'native-unlisted')
            self.assertTrue(any(e['To'] == 'nested/sub' for e in rows['nested/a.go']))
            self.assertEqual(len(rows) + len(result['gaps']), len(selected))

    def test_unresolved_and_cgo_are_gaps(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            selected = self.fixture(root)
            (root / 'lib/lib.go').write_text('package lib\nimport _ "example.invalid/missing"\n')
            result = cov_go.capture(root, selected, '1')
            self.assertIn('lib/lib.go', result['gaps'])
            self.assertEqual(result['gaps']['app/native.go'], 'native-cgo-attribution')
            self.assertEqual(result['gaps']['app/a.go'], 'native-cgo-attribution')
            self.assertNotIn('lib/lib.go', [r['Path'] for r in result['truth']])

    def test_ambiguous_embed_is_not_package_truth(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            selected = self.fixture(root)
            (root / 'app/embed.go').write_text('package app\nimport _ "embed"\n//go:embed payload.txt other.txt\nvar payload string\n')
            (root / 'app/other.txt').write_text('other')
            self.assertEqual(cov_go.capture(root, selected, '0')['gaps']['app/embed.go'], 'native-embed-attribution')


if __name__ == '__main__':
    unittest.main()
