import struct
import unittest
from native_authdb import error_fields, data_row, mutate_authdb, attested


class NativeAuthDBTest(unittest.TestCase):
    def test_native_error_and_row_protocol(self):
        payload = b'SERROR\0C08P01\0Mauthentication database "authdb" is disabled\0\0'
        self.assertEqual('08P01', error_fields(payload)['C'])
        self.assertEqual('authentication database "authdb" is disabled', error_fields(payload)['M'])
        row = struct.pack('!H', 3) + struct.pack('!i', 3) + b'one' + struct.pack('!i', -1) + struct.pack('!i', 0)
        self.assertEqual(['one', None, ''], data_row(row))

    def test_ablation_changes_only_the_selected_source_value(self):
        source = '[databases]\napp = host=127.0.0.1 auth_dbname=authdb\nauthdb = dbname=postgres\n'
        result = mutate_authdb(source, 'app', 'authdb', 'missing')
        self.assertEqual(source.replace('auth_dbname=authdb', 'auth_dbname=missing'), result)
        with self.assertRaises(ValueError):
            mutate_authdb(source, 'other', 'authdb', 'missing')

    def test_no_fallback_when_native_binding_differs(self):
        self.assertTrue(attested('authdb', {'M': 'authentication database "authdb" is disabled'}))
        self.assertFalse(attested('authdb', {'M': 'database "app" is disabled'}))
        self.assertFalse(attested('authdb', {'M': 'authentication database "other" is disabled'}))
