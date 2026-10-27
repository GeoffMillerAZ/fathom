"""Bounded source inspection must never silently fetch the whole VSIX."""
import io
import struct
import unittest
from unittest.mock import patch
from audit_extension import fetch_range, directory_rows, inspect_claims


class Response(io.BytesIO):
    def __init__(self, data, status=206, content_range='bytes 10-12/100'):
        super().__init__(data)
        self.status = status
        self.headers = {'Content-Range': content_range}


class ExtensionAuditTest(unittest.TestCase):
    def test_range_is_exact_and_response_size_is_bounded(self):
        with patch('audit_extension.urllib.request.urlopen', return_value=Response(b'abc')):
            self.assertEqual(b'abc', fetch_range(10, 3, total=100))
        for response in [Response(b'abc', 200), Response(b'abc', content_range='bytes 0-2/100'),
                         Response(b'ab'), Response(b'abcd')]:
            with patch('audit_extension.urllib.request.urlopen', return_value=response):
                with self.assertRaises(ValueError):
                    fetch_range(10, 3, total=100)

    def test_directory_rejects_truncated_or_extra_records(self):
        name = b'extension/dist/extension.bundle.js'
        fixed = struct.pack('<4s6H3I5H2I', b'PK\x01\x02', 20, 20, 0, 8, 0, 0,
                            1, 2, 3, len(name), 0, 0, 0, 0, 0, 17)
        rows = directory_rows(fixed + name, expected=1)
        self.assertEqual(17, rows[0]['offset'])
        for data, expected in [(fixed + name[:-1], 1), (fixed + name, 2), (b'bad', 1)]:
            with self.assertRaises(ValueError):
                directory_rows(data, expected=expected)

    def test_native_route_does_not_imply_measured_reference_or_position_support(self):
        claims = inspect_claims('out/artifacts/${n}-${t}/PolicyLanguageServer ${t} arm64 '
                                'http://localhost:6490/evaluatePolicy SET_CACHE_FOLDER_PATH',
                                ['extension/out/artifacts/osx-arm64/PolicyLanguageServer'])
        self.assertTrue(claims['arm64_payload_present'])
        self.assertTrue(claims['loopback_evaluation_route_present'])
        self.assertFalse(claims['native_executed'])
        self.assertFalse(claims['native_population_measured'])
