"""Authored end-to-end boundary controls for the diagnostic comparator."""
import hashlib
import json
from pathlib import Path
import subprocess
import sys
import tempfile
import unittest

HERE=Path(__file__).resolve().parent
class CandidateComparisonTest(unittest.TestCase):
    def run_case(self, change=None):
        with tempfile.TemporaryDirectory(dir=HERE.parents[2]/'workspace/tmp') as directory:
            root=Path(directory);inputs=root/'inputs';(inputs/'authored').mkdir(parents=True)
            data=b'pipeline {\n agent any\n}\n';(inputs/'authored/Jenkinsfile').write_bytes(data)
            raw=dict(core_version='authored',tool_declarations=0,files=[dict(subject='authored',sha256=hashlib.sha256(data).hexdigest(),status='parsed',nodes=[dict(type='ModelASTPipelineDef',name=None,line=1,end=3,parent=-1)])])
            candidate=dict(files=[dict(path='authored/Jenkinsfile',error='',result=dict(dialect='jenkinsfile',skipped='',incomplete=False,symbols=[dict(key='pipeline',name='pipeline',kind='JenkinsPipeline',line=1,end_line=3)],references=[]))])
            if change:change(raw,candidate)
            for name,obj in [('raw',raw),('candidate',candidate)]:
                (root/(name+'.json')).write_text(json.dumps(obj))
            run=subprocess.run([sys.executable,'-B',str(HERE/'compare_candidate.py'),'--raw',str(root/'raw.json'),'--candidate',str(root/'candidate.json'),'--inputs',str(inputs),'--out',str(root/'out.json')],capture_output=True,text=True)
            return run,json.loads((root/'out.json').read_text()) if (root/'out.json').exists() else None
    def test_exact_range_intersection_and_unmapped_candidate_accounting(self):
        run,out=self.run_case();self.assertEqual(0,run.returncode,run.stderr)
        self.assertEqual(dict(truth=1,candidate=1,matched=1,precision_lower_bound=1.0),out['subjects']['authored']['definitions'])
        self.assertFalse(out['full_population_native_metrics']);self.assertFalse(out['tier_granted'])
        def change(raw,c):c['files'][0]['result']['symbols'][0]['end_line']=2
        run,out=self.run_case(change);self.assertEqual(0,run.returncode,run.stderr)
        self.assertEqual(0,out['subjects']['authored']['definitions']['matched'])
    def test_candidate_wrong_dialect_or_incomplete_refuses_whole_output(self):
        for key,value in [('dialect','generic-yaml'),('incomplete',True),('skipped','unsupported')]:
            def change(raw,c):c['files'][0]['result'][key]=value
            run,out=self.run_case(change);self.assertNotEqual(0,run.returncode);self.assertIsNone(out)
    def test_missing_duplicate_or_traversing_candidate_refuses(self):
        for mode in ('missing','duplicate','traversal'):
            def change(raw,c):
                if mode=='missing':c['files']=[]
                elif mode=='duplicate':c['files']*=2
                else:c['files'][0]['path']='../Jenkinsfile'
            run,out=self.run_case(change);self.assertNotEqual(0,run.returncode);self.assertIsNone(out)
    def test_native_refusal_does_not_drop_candidate_units(self):
        def change(raw,c):raw['files'][0].update(status='refused',nodes=[])
        run,out=self.run_case(change);self.assertEqual(0,run.returncode,run.stderr)
        self.assertEqual(1,out['subjects']['authored']['definitions']['candidate'])
        self.assertEqual(0,out['subjects']['authored']['definitions']['matched'])
        self.assertFalse(out['tier_granted'])
