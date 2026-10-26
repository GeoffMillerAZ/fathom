"""Replay the first actual cloud receipt without upgrading its evidence."""
import hashlib
import json
import unittest
from pathlib import Path
from receipt_contract import read_receipt

class ProbeRunReceiptTest(unittest.TestCase):
    def test_original_receipt_matches_frozen_hash_and_schema(self):
        here=Path(__file__).parent
        path=here/'probe-receipt-run40407284519.json'
        self.assertEqual('ac6dee711f8f4e7dab06b242ea55f2361d95a3e38219838590e3c201c75d626c',hashlib.sha256(path.read_bytes()).hexdigest())
        schema=json.loads((here/'probe-receipt-run40407284519.schema.json').read_text())
        value=read_receipt(path,schema)
        self.assertEqual(-25,value['native_returncode']);self.assertEqual('initialize',value['failed_stage'])
        self.assertTrue(value['filesystem_readonly_outside_out'])
        self.assertEqual(15,value['filesystem_write_probes'])
        self.assertEqual(0,value['stderr_bytes']);self.assertEqual(0,value['lsp_bytes'])
        self.assertFalse(value['calibration_measured']);self.assertFalse(value['tier_granted'])
