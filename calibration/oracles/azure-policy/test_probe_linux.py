import copy
import io
import json
import tempfile
import unittest
from pathlib import Path
from types import SimpleNamespace
from probe_linux import INPUT, LIMIT, LSP, evaluation_inputs, payload_identity, receipt_base, verify


class LinuxProbeTest(unittest.TestCase):
    def test_changed_payload_refused_before_execution(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            (root / 'PolicyLanguageServer').write_bytes(b'authored apphost')
            pin = payload_identity(root)
            self.assertEqual(pin, verify(root, pin))
            (root / 'PolicyLanguageServer').write_bytes(b'changed apphost')
            with self.assertRaisesRegex(ValueError, 'identity'):
                verify(root, pin)

    def test_symlink_refused(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            (root / 'PolicyLanguageServer').write_bytes(b'authored apphost')
            (root / 'link').symlink_to('PolicyLanguageServer')
            with self.assertRaisesRegex(ValueError, 'symlink'):
                payload_identity(root)

    def test_variants_keep_unused_declaration_and_change_fact(self):
        source = json.loads(INPUT.read_text())
        original = copy.deepcopy(source)
        variants = dict(evaluation_inputs(source))
        self.assertEqual(original, source)
        for body in variants.values():
            policy = json.loads(body['policyDefinitions'][0])
            self.assertIn('unused', policy['properties']['parameters'])
        bad = json.loads(variants['unknown_parameter']['policyDefinitions'][0])
        self.assertEqual("[parameters('absent')]", bad['properties']['policyRule']['if']['equals'])
        self.assertNotEqual(variants['valid'], variants['changed_input_fact'])

    def test_lsp_cap_plus_one_refuses_message(self):
        process = SimpleNamespace(stdout=io.BytesIO(('Content-Length: %d\r\n\r\n' % (LIMIT + 1)).encode()))
        lsp = LSP(process, io.BytesIO())
        with self.assertRaisesRegex(ValueError, 'cap'):
            lsp.response(1, 1)

    def test_pending_receipt_claims_no_measurement(self):
        receipt = receipt_base()
        self.assertEqual('pending', receipt['status'])
        self.assertFalse(receipt['native_executed'])
        self.assertFalse(receipt['calibration_measured'])
        self.assertFalse(receipt['tier_granted'])
