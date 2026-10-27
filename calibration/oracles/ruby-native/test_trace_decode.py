from dataclasses import replace
import json
from pathlib import Path
import tempfile
import unittest

from trace_decode import Limits, decode


class NativeTraceTests(unittest.TestCase):
    def setUp(self):
        scratch=Path(__file__).resolve().parents[3]/'workspace/tmp'
        scratch.mkdir(parents=True,exist_ok=True)
        temp=tempfile.TemporaryDirectory(dir=scratch)
        self.addCleanup(temp.cleanup)
        self.path=Path(temp.name)/'native-1.json'
        self.raw={'ruby':'3.3.8','refused':False,'native_entries':2,
                  'calls':[['native.rb',3,'actual_caller','target','target']],
                  'functions':[['native.rb','actual_caller',1,8]]}
        self.syntax={'sites':[{'path':'native.rb','line':3,'from':'syntax_guess','written_name':'target'}],
                     'nonliteral_lines':[]}
        self.save()

    def save(self):
        self.path.write_text(json.dumps(self.raw))

    def test_native_caller_never_copies_syntax_pair(self):
        oracle,summary=decode([self.path],self.syntax)
        self.assertEqual(oracle['calls'],[{'path':'native.rb','from':'actual_caller','to':'target'}])
        self.assertEqual(summary['attested_sites'],1)

    def test_implicit_callback_is_unattested(self):
        self.raw['calls'][0][2]='each'
        self.save()
        self.assertEqual(decode([self.path],self.syntax)[1]['attested_sites'],0)

    def test_alias_is_unattested(self):
        self.raw['calls'][0][4]='alias_name'
        self.save()
        self.assertEqual(decode([self.path],self.syntax)[1]['unattested_sites'],1)

    def test_same_line_duplicate_is_unattested(self):
        self.syntax['sites']*=2
        self.assertEqual(decode([self.path],self.syntax)[1]['unattested_sites'],2)

    def test_reflective_dispatch_cannot_impersonate_literal(self):
        self.syntax['nonliteral_lines']=[{'path':'native.rb','line':3,'end':3}]
        self.assertEqual(decode([self.path],self.syntax)[1]['attested_sites'],0)

    def test_missing_native_range_is_unattested(self):
        self.raw['functions']=[]
        self.save()
        self.assertEqual(decode([self.path],self.syntax)[1]['unattested_sites'],1)

    def test_missing_output_refuses(self):
        with self.assertRaisesRegex(ValueError,'whole-unit refusal'):
            decode([],self.syntax)

    def test_truncated_json_refuses(self):
        self.path.write_text('{')
        with self.assertRaises(ValueError):decode([self.path],self.syntax)

    def test_raw_refusal_never_emits_partial_truth(self):
        self.raw['refused']=True
        self.save()
        with self.assertRaisesRegex(ValueError,'whole-unit refusal'):decode([self.path],self.syntax)

    def test_wrong_runtime_refuses(self):
        self.raw['ruby']='3.3.7'
        self.save()
        with self.assertRaisesRegex(ValueError,'whole-unit refusal'):decode([self.path],self.syntax)

    def test_file_count_cap_plus_one(self):
        with self.assertRaisesRegex(ValueError,'file count cap'):
            decode([self.path,self.path],self.syntax,replace(Limits(),files=1))

    def test_total_bytes_cap_plus_one(self):
        with self.assertRaisesRegex(ValueError,'total TracePoint byte cap'):
            decode([self.path],self.syntax,replace(Limits(),total_bytes=self.path.stat().st_size-1))

    def test_file_bytes_cap_plus_one(self):
        with self.assertRaisesRegex(ValueError,'raw TracePoint byte cap'):
            decode([self.path],self.syntax,replace(Limits(),file_bytes=self.path.stat().st_size-1))

    def test_rows_cap_plus_one(self):
        self.raw['calls']*=2
        self.save()
        with self.assertRaisesRegex(ValueError,'native row or function count cap'):
            decode([self.path],self.syntax,replace(Limits(),rows=1))

    def test_functions_cap_plus_one(self):
        self.raw['functions']*=2
        self.save()
        with self.assertRaisesRegex(ValueError,'native row or function count cap'):
            decode([self.path],self.syntax,replace(Limits(),functions=1))


if __name__=='__main__':unittest.main()
