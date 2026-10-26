from dataclasses import replace
import json
from pathlib import Path
import tempfile
import unittest

from trace_decode import Limits,decode


class NativeFunctionTests(unittest.TestCase):
    def setUp(self):
        workspace=Path(__file__).resolve().parents[3]/'workspace/tmp'
        workspace.mkdir(parents=True,exist_ok=True)
        self.temp=tempfile.TemporaryDirectory(dir=workspace)
        self.addCleanup(self.temp.cleanup)
        self.path=Path(self.temp.name)/'native.json'
        self.raw={'pwsh':'7.6.9','refused':False,'calls':[['native.ps1',3,4,'ActualCaller','ActualTarget','writtenAlias']],
                  'native_events':1,'original_breakpoints':1,'retained_breakpoints':1,'driver_status':'completed','exception_sha256':None}
        self.population={'sites':[{'path':'native.ps1','line':3,'column':4,'from':'actualcaller',
                                   'written':'writtenAlias','kind':'command'}]}
        self.save()

    def save(self):self.path.write_text(json.dumps(self.raw))

    def test_native_names_do_not_come_from_parser_pair(self):
        oracle,_=decode(self.path,self.population)
        self.assertEqual(oracle['calls'],[{'path':'native.ps1','from':'ActualCaller','to':'ActualTarget'}])

    def test_native_alias_target_is_not_written_alias(self):
        oracle,_=decode(self.path,self.population)
        self.assertEqual(oracle['calls'][0]['to'],'ActualTarget')
        self.assertNotEqual(oracle['calls'][0]['to'],self.population['sites'][0]['written'])

    def test_implicit_callback_is_unattested(self):
        self.raw['calls'][0][3]='ImplicitCallback'
        self.save()
        self.assertEqual(decode(self.path,self.population)[1]['attested_sites'],0)

    def test_same_line_different_column_is_unattested(self):
        self.raw['calls'][0][2]=5
        self.save()
        self.assertEqual(decode(self.path,self.population)[1]['attested_sites'],0)

    def test_same_position_duplicate_is_unattested(self):
        self.population['sites']*=2
        self.assertEqual(decode(self.path,self.population)[1]['unattested_sites'],2)

    def test_unexecuted_member_stays_in_denominator(self):
        self.population['sites'].append(dict(self.population['sites'][0],kind='member'))
        self.assertEqual(decode(self.path,self.population)[1]['literal_sites'],2)
        self.assertEqual(decode(self.path,self.population)[1]['unattested_sites'],1)

    def test_wrong_runtime_refuses(self):
        self.raw['pwsh']='7.6.8'
        self.save()
        with self.assertRaisesRegex(ValueError,'whole-unit refusal'):decode(self.path,self.population)

    def test_native_cap_refuses(self):
        self.raw['refused']=True
        self.save()
        with self.assertRaisesRegex(ValueError,'whole-unit refusal'):decode(self.path,self.population)

    def test_bytes_cap_plus_one(self):
        with self.assertRaisesRegex(ValueError,'native byte cap'):
            decode(self.path,self.population,replace(Limits(),raw_bytes=self.path.stat().st_size-1))

    def test_row_cap_plus_one(self):
        self.raw['calls']*=2
        self.save()
        with self.assertRaisesRegex(ValueError,'native row or site cap'):
            decode(self.path,self.population,replace(Limits(),rows=1))

    def test_site_cap_plus_one(self):
        self.population['sites']*=2
        with self.assertRaisesRegex(ValueError,'native row or site cap'):
            decode(self.path,self.population,replace(Limits(),sites=1))

    def test_truncated_json_refuses(self):
        self.path.write_text('{')
        with self.assertRaises(ValueError):decode(self.path,self.population)


if __name__=='__main__':unittest.main()
