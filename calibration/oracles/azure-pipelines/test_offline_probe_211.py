"""Receipt honesty controls for an authored offline feasibility probe."""
import copy
import unittest
import json
import subprocess
import sys
import tempfile
from pathlib import Path
from offline_probe_211 import validate

class OfflineProbeTest(unittest.TestCase):
    def receipt(self):
        return dict(classification='feasibility-only',support_claim=False,native_owner_attested=False,
                    cases=[dict(case='template',status='evaluated',stages=['Build','Publish'],jobs=['Compile','Release'],dependencies=['Publish:Build']),
                           dict(case='changed',status='evaluated',stages=['Changed','Publish'],jobs=['Compile','Release'],dependencies=['Publish:Changed']),
                           dict(case='malformed',status='refused',stages=[],jobs=[],dependencies=[]),
                           dict(case='missing',status='refused',stages=[],jobs=[],dependencies=[])])
    def test_complete_authored_controls(self):
        self.assertEqual(4,validate(self.receipt())['inputs'])
    def test_no_support_tier_or_owner_attestation_can_be_inferred(self):
        for key,val in [('support_claim',True),('native_owner_attested',True),('classification','support')]:
            r=self.receipt();r[key]=val
            with self.assertRaises(ValueError):validate(r)
    def test_missing_duplicate_and_failed_case_refuses_whole_receipt(self):
        for mode in ('missing','duplicate','failure','malformed-success'):
            r=self.receipt()
            if mode=='missing':r['cases'].pop()
            elif mode=='duplicate':r['cases'][1]=copy.deepcopy(r['cases'][0])
            elif mode=='failure':r['cases'][0]['status']='refused'
            else:r['cases'][2]['status']='evaluated'
            with self.assertRaises(ValueError):validate(r)
    def test_changed_fact_and_entity_output_required(self):
        for key in ('stages','jobs'):
            r=self.receipt();r['cases'][0][key]=[]
            with self.assertRaises(ValueError):validate(r)
        r=self.receipt();r['cases'][1]['stages']=['Build','Publish']
        with self.assertRaises(ValueError):validate(r)

    def test_cli_cap_plus_one_and_malformed_json_refuse_without_output(self):
        here=Path(__file__).resolve().parent
        for content in (b' '* (1024**2+1), b'{'):
            with tempfile.TemporaryDirectory(dir=here.parents[2]/'workspace/tmp') as folder:
                root=Path(folder);raw=root/'raw.json';out=root/'out.json';raw.write_bytes(content)
                r=subprocess.run([sys.executable,'-B',str(here/'offline_probe_211.py'),'--raw',str(raw),'--out',str(out)],capture_output=True)
                self.assertNotEqual(0,r.returncode);self.assertFalse(out.exists())
    def test_unknown_fields_and_missing_dependency_output_refuse(self):
        r=self.receipt();r['tier']='A'
        with self.assertRaises(ValueError):validate(r)
        r=self.receipt();r['cases'][0]['dependencies']=[]
        with self.assertRaises(ValueError):validate(r)

if __name__=='__main__':unittest.main()
