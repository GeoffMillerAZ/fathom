"""Checkbox controls and real checked-row receipt reproduction."""

import argparse
import copy
import json
from pathlib import Path
import sys
import tempfile
import unittest
from unittest.mock import patch

sys.path.insert(0, str(Path(__file__).resolve().parent))
import qualification_contract as driver


class RowStateTest(unittest.TestCase):
    def setUp(self):
        self.row = driver.row_from((driver.ROOT / driver.PLAN).read_bytes())
        self.open_row = self.row.replace('- [x] ', '- [ ] ', 1)

    def test_open_and_checked_have_the_same_contract_hash(self):
        checked = self.open_row.replace('- [ ] ', '- [x] ', 1)
        self.assertEqual(driver.row_from(checked.encode()), checked)
        self.assertEqual(driver.normalized_row(checked), self.open_row)
        self.assertEqual(driver.sha256(driver.normalized_row(checked).encode()),
                         driver.sha256(driver.normalized_row(self.open_row).encode()))

    def test_refused_row_is_rejected(self):
        refused = self.open_row.replace('- [ ] ', '- [-] ', 1)
        with self.assertRaises(RuntimeError):
            driver.row_from(refused.encode())

    def test_duplicate_row_is_rejected(self):
        with self.assertRaises(RuntimeError):
            driver.row_from((self.open_row * 2).encode())


def test_checked_receipt(args):
    """Run every real driver case on a checked copy, never the production row."""
    original_plan = (driver.ROOT / driver.PLAN).read_bytes()
    original_reading = (driver.ROOT / driver.READING).read_bytes()
    row = driver.row_from(original_plan)
    checked = row.replace('- [ ] ', '- [x] ', 1)
    expected = json.loads((driver.ROOT / 'docs/assessments/data/2026-10-27-crossplane-qualification-contract.json').read_bytes())
    expected['authored_state'] = '[x]'
    scratch_parent = driver.ROOT / 'workspace/tmp'
    scratch_parent.mkdir(parents=True, exist_ok=True)
    with tempfile.TemporaryDirectory(prefix='crossplane-checked-test-', dir=scratch_parent) as temporary:
        root = Path(temporary)
        (root / driver.PLAN).parent.mkdir(parents=True)
        (root / driver.READING).parent.mkdir(parents=True)
        (root / driver.PLAN).write_text(checked, encoding='utf-8')
        (root / driver.READING).write_bytes(original_reading)
        run_args = copy.copy(args)
        run_args.out = str(root / 'workspace/artifacts/checked-receipt.json')
        # Only the input root changes. Native tools, identity checks, verifier,
        # grader, all fourteen cases and receipt serialization run unmocked.
        with patch.object(driver, 'ROOT', root):
            driver.run(run_args)
        actual = json.loads(Path(run_args.out).read_bytes())
        unittest.TestCase().assertEqual(actual, expected)
        unittest.TestCase().assertEqual((root / driver.PLAN).read_text(), checked)
    unittest.TestCase().assertEqual((driver.ROOT / driver.PLAN).read_bytes(), original_plan)
    unittest.TestCase().assertEqual((driver.ROOT / driver.READING).read_bytes(), original_reading)
    print('checked-row driver: committed receipt reproduced; only authored_state differs')


if __name__ == '__main__':
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--fathom', required=True)
    parser.add_argument('--indexbench', required=True)
    parser.add_argument('--store', required=True)
    parser.add_argument('--main-commit', default=driver.MAIN_COMMIT)
    args = parser.parse_args()
    result = unittest.TextTestRunner().run(unittest.defaultTestLoader.loadTestsFromTestCase(RowStateTest))
    if not result.wasSuccessful():
        raise SystemExit(1)
    test_checked_receipt(args)
