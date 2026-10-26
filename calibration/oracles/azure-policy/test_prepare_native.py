import io
import tempfile
import unittest
import zipfile
from pathlib import Path
from prepare_native import unpack_span, PREFIX, MAX_FILES, MAX_SINGLE_BYTES, MAX_TOTAL_BYTES


class NativePreparationTest(unittest.TestCase):
    def archive(self, names):
        out = io.BytesIO()
        with zipfile.ZipFile(out, 'w', zipfile.ZIP_DEFLATED) as archive:
            for name in names:
                archive.writestr(name, b'authored artifact control')
        with zipfile.ZipFile(io.BytesIO(out.getvalue())) as archive:
            rows = [dict(name=e.filename, offset=e.header_offset, method=e.compress_type, flags=e.flag_bits,
                         crc=e.CRC, compressed=e.compress_size, uncompressed=e.file_size,
                         external_attributes=e.external_attr) for e in archive.infolist()]
        return out.getvalue(), rows

    def test_only_verified_native_payload_is_written_and_hashed(self):
        data, rows = self.archive([PREFIX + 'PolicyLanguageServer', PREFIX + 'nested/runtime.dll'])
        with tempfile.TemporaryDirectory() as directory:
            out = Path(directory) / 'payload'
            receipt = unpack_span(data, 0, rows, out)
            self.assertEqual(2, receipt['files'])
            self.assertEqual(50, receipt['bytes'])
            self.assertTrue((out / 'PolicyLanguageServer').is_file())
            self.assertEqual(64, len(receipt['manifest_sha256']))

    def test_bad_crc_or_escaping_path_refuses_before_writing_any_file(self):
        for names in [[PREFIX + 'PolicyLanguageServer', PREFIX + '../escape'],
                      [PREFIX + 'PolicyLanguageServer', PREFIX + 'runtime.dll']]:
            data, rows = self.archive(names)
            if not names[-1].endswith('escape'):
                rows[-1]['crc'] ^= 1
            with tempfile.TemporaryDirectory() as directory:
                out = Path(directory) / 'payload'
                with self.assertRaises(ValueError):
                    unpack_span(data, 0, rows, out)
                self.assertFalse(out.exists())

    def test_count_and_expansion_cap_plus_one_refuse_whole_payload(self):
        data, rows = self.archive([PREFIX + 'PolicyLanguageServer'])
        with tempfile.TemporaryDirectory() as directory:
            out = Path(directory) / 'payload'
            with self.assertRaisesRegex(ValueError, 'count'):
                unpack_span(data, 0, rows * (MAX_FILES + 1), out)
            rows[0]['uncompressed'] = MAX_SINGLE_BYTES + 1
            with self.assertRaisesRegex(ValueError, 'expansion'):
                unpack_span(data, 0, rows, out)
            self.assertFalse(out.exists())

    def test_total_expansion_cap_plus_one_and_missing_apphost_refuse_whole_payload(self):
        data, rows = self.archive([PREFIX + 'runtime.dll'])
        with tempfile.TemporaryDirectory() as directory:
            out = Path(directory) / 'payload'
            with self.assertRaisesRegex(ValueError, 'apphost'):
                unpack_span(data, 0, rows, out)
            expanded = [dict(rows[0], uncompressed=MAX_SINGLE_BYTES),
                        dict(rows[0], uncompressed=MAX_SINGLE_BYTES),
                        dict(rows[0], uncompressed=MAX_TOTAL_BYTES - 2 * MAX_SINGLE_BYTES + 1)]
            with self.assertRaisesRegex(ValueError, 'expansion'):
                unpack_span(data, 0, expanded, out)
            self.assertFalse(out.exists())
