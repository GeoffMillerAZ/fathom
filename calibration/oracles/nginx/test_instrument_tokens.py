import unittest
from instrument_tokens import ANCHOR, SIGNATURE, HOOK_ANCHOR, HOOK, DECLARATIONS, OBSERVED, transform


class TokenInstrumentTest(unittest.TestCase):
    def test_native_body_is_preserved_except_observer_read(self):
        source = (ANCHOR + '\nstatic ngx_int_t\n' + SIGNATURE + '\n' + HOOK_ANCHOR + '\n return native_result;\n}\n').encode()
        changed, shim = transform(source)
        self.assertEqual(64, len(shim))
        self.assertIn(HOOK.encode(), changed)
        self.assertIn(b'return native_result;', changed)
        self.assertIn(b'ORACLE_NGINX_TOKEN_V1', changed)
        with self.assertRaisesRegex(ValueError, 'drift'):
            transform(changed)

    def test_unknown_native_shape_refuses_instead_of_guessing(self):
        with self.assertRaisesRegex(ValueError, 'drift'):
            transform(b'authored unsupported source')
