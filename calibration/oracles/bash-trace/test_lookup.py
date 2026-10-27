from pathlib import Path
import tempfile,unittest
from measure_lookup import decode


class LookupTests(unittest.TestCase):
    def setUp(self):
        root=Path(__file__).resolve().parents[3]/'workspace/tmp';root.mkdir(parents=True,exist_ok=True)
        tmp=tempfile.TemporaryDirectory(dir=root);self.addCleanup(tmp.cleanup)
        self.path=Path(tmp.name)/'raw'

    def test_available_requires_native_inventory_and_matching_kind(self):
        self.path.write_bytes(b'F\0real\0real 1 fixture.sh\0B\0echo\0\0Q\0real\0function\0Q\0ghost\0function\0Q\0echo\0file\0')
        answers,_,_,_=decode(self.path)
        self.assertEqual(answers,{'real':'real'})

    def test_truncation_duplicate_and_unknown_records_refuse(self):
        for raw in [b'Q\0x\0missing',b'Q\0x\0missing\0Q\0x\0missing\0',b'X\0x\0x\0',b'F\0x\0x\0F\0x\0x\0']:
            with self.subTest(raw=raw):
                self.path.write_bytes(raw)
                with self.assertRaisesRegex(ValueError,'whole-unit refusal'):decode(self.path)

    def test_lookup_caps_accept_n_refuse_n_plus_one(self):
        self.path.write_bytes(b'B\0echo\0\0Q\0echo\0builtin\0')
        size=self.path.stat().st_size
        decode(self.path,max_bytes=size,max_records=2,max_field_bytes=7)
        for kwargs in [dict(max_bytes=size-1),dict(max_records=1),dict(max_field_bytes=6)]:
            with self.subTest(kwargs=kwargs),self.assertRaisesRegex(ValueError,'whole-unit refusal'):decode(self.path,**kwargs)
