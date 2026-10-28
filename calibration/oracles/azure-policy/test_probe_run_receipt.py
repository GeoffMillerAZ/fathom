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


    def test_admitted_rerun_receipt_matches_frozen_hash_and_contract(self):
        from probe_linux import CONTRACT
        path=Path(__file__).with_name('probe-receipt-run40504219895.json')
        self.assertEqual('f013b8c01bb5c8aef55e9eb9b07140d9077c202afb23c6451d052179274f2b1c',hashlib.sha256(path.read_bytes()).hexdigest())
        value=read_receipt(path,CONTRACT)
        self.assertEqual('probe_complete_requires_analysis',value['status'])
        self.assertIs(value['native_executed'],True);self.assertEqual(0,value['native_returncode'])
        self.assertTrue(value['netns_loopback_only']);self.assertTrue(value['filesystem_readonly_outside_out'])
        self.assertTrue(value['filesystem_dotdot_writes_refused']);self.assertTrue(value['vendor_capabilities_dropped'])
        self.assertTrue(value['shutdown_clean'])
        self.assertEqual(['experimental','workspace'],value['capability_keys'])
        self.assertIs(value['definition_provider'],False);self.assertIs(value['reference_provider'],False)
        for key in ('definition','references','cache_command'):
            self.assertEqual(-32601,value[key]['error_code'])
        rows=value['evaluations']
        self.assertEqual(['valid','unknown_parameter','changed_input_fact'],[row['variant'] for row in rows])
        self.assertEqual({500},{row['http_status'] for row in rows})
        self.assertEqual(1,len({(row['sha256'],row['bytes']) for row in rows}))
        diag=value['write_diagnostics']
        self.assertTrue(diag['sigxfsz_observed']);self.assertEqual('dotnet_doublemapper',diag['limit_file']['kind'])
        self.assertEqual(2199023255552,diag['limit_file']['requested_extent_bytes'])
        self.assertFalse(value['calibration_measured']);self.assertFalse(value['tier_granted'])
