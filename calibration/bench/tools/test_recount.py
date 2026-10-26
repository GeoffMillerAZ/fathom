"""D26 policy traps; run without producing bytecode outside workspace/."""
import sys
sys.dont_write_bytecode = True
import unittest
from pathlib import Path
from unittest.mock import patch
from recount import product_path, recount


class ProductCallers(unittest.TestCase):
    def test_excludes_support_in_every_language(self):
        for path in ['foo_test.go', 'middleware/client_ip_example_test.go', '_examples/main.go',
                     'bench/foo.py', 'typing_tests/foo.py', 'testing/foo.py', 'src/test_foo.py',
                     'src/lib.spec.ts', 'src/constant.test-d.ts', 'src/__tests__/foo.ts', 'packages/tests/src/foo.ts',
                     'docs/x.py', 'sample/foo.ts', 'tools/main.go', 'scripts/main.go',
                     'website/foo.ts', 'perf-testing/foo.ts', '__performance_tests__/foo.ts']:
            with self.subTest(path=path):
                self.assertFalse(product_path(path))

    def test_units_exclude_self_and_all_test_callers(self):
        row = {'symbol': 'pkg.target', 'sites': [
            'src/pkg/self.py:1:1', 'src/pkg/test_self.py:1:1',
            'tests/a.py:1:1', 'examples/a.py:1:1', 'tools/a.py:1:1',
            'src/a/a.py:1:1', 'src/a/a.py:2:1', 'src/b/b.py:1:1',
            'src/b/b.py:2:1', 'src/c/c.py:1:1']}
        with patch('recount.subprocess.check_output', return_value='src/pkg/__init__.py\0'):
            result = recount('Python', Path('.'), [row])[0]
            self.assertEqual(result['calls'], 5)
            self.assertEqual(result['units'], ['src/a', 'src/b', 'src/c'])
            self.assertTrue(result['qualifies'])
            row['sites'].pop()
            self.assertFalse(recount('Python', Path('.'), [row])[0]['qualifies'])

    def test_keeps_product(self):
        for path in ['assert/assertions.go', 'mock/mock.go', 'src/_pytest/fixtures.py',
                     'src/_pytest/assertion/rewrite.py', 'packages/toolkit/src/query/core/apiState.ts']:
            with self.subTest(path=path):
                self.assertTrue(product_path(path))


if __name__ == '__main__':
    unittest.main()
