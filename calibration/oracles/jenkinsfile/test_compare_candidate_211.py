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
    def run_case(self, change=None, stale=False):
        with tempfile.TemporaryDirectory(dir=HERE.parents[2]/'workspace/tmp') as directory:
            root=Path(directory);inputs=root/'inputs';(inputs/'authored').mkdir(parents=True)
            data=b'pipeline {\n agent any\n}\n';(inputs/'authored/Jenkinsfile').write_bytes(data)
            (inputs/'authored/other.jenkinsfile').write_bytes(data)
            raw=dict(core_version='authored',tool_declarations=0,files=[dict(subject='authored',sha256=hashlib.sha256(data).hexdigest(),status='parsed',nodes=[dict(type='ModelASTPipelineDef',name=None,line=1,end=3,parent=-1)])])
            candidate=dict(files=[dict(path='authored/Jenkinsfile',error='',result=dict(dialect='jenkinsfile',skipped='',incomplete=False,symbols=[dict(key='pipeline',name='pipeline',kind='JenkinsPipeline',line=1,end_line=3)],references=[]))])
            for name,obj in [('raw',raw),('candidate',candidate)]:
                (root/(name+'.json')).write_text(json.dumps(obj))
            command=[sys.executable,'-B',str(HERE/'compare_candidate.py'),'--raw',str(root/'raw.json'),'--candidate',str(root/'candidate.json'),'--inputs',str(inputs),'--out',str(root/'out.json')]
            if stale:
                good=subprocess.run(command,capture_output=True,text=True)
                self.assertEqual(0,good.returncode,good.stderr)
                self.assertTrue((root/'out.json').exists())
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

    def test_refusal_invalidates_existing_success(self):
        for mode in ('incomplete', 'malformed', 'unreadable'):
            def change(raw,c):
                if mode=='incomplete':c['files'][0]['result']['incomplete']=True
                elif mode=='malformed':c['files']=None
                else:c['files'][0]['path']='authored/missing'
            run,out=self.run_case(change,stale=True)
            self.assertNotEqual(0,run.returncode)
            self.assertIsNone(out,run.stderr)
            self.assertIn('"error": "comparison_refused"',run.stderr)

    def test_malformed_candidate_records_refuse_whole_unit(self):
        cases=[('line',True),('line',0),('line','1'),('end_line',0),('end_line',4),('key',''),('name',None)]
        for key,value in cases:
            with self.subTest(key=key,value=value):
                def change(raw,c):c['files'][0]['result']['symbols'][0][key]=value
                run,out=self.run_case(change,stale=True)
                self.assertNotEqual(0,run.returncode,run.stdout)
                self.assertIsNone(out)
        for mode in ('undefined','bool_reference','range_reference','symbol_cap','reference_cap'):
            with self.subTest(mode=mode):
                def change(raw,c):
                    r=c['files'][0]['result']
                    if mode=='symbol_cap':r['symbols']*=100001;return
                    ref=dict(**{'from':'pipeline','to':'echo'},line=1)
                    if mode=='undefined':r['symbols']=[]
                    if mode=='bool_reference':ref['line']=True
                    if mode=='range_reference':ref['line']=4
                    r['references']=[ref]*(100001 if mode=='reference_cap' else 1)
                    raw['files'][0]['nodes'].append(dict(type='ModelASTStep',name='echo',line=1,end=1,parent=0))
                run,out=self.run_case(change,stale=True)
                self.assertNotEqual(0,run.returncode,run.stdout)
                self.assertIsNone(out)

    def test_malformed_envelopes_have_structured_refusal(self):
        for mode in ('row','result','symbol','reference','flag'):
            with self.subTest(mode=mode):
                def change(raw,c):
                    if mode=='row':c['files']=[1]
                    elif mode=='result':c['files'][0]['result']=None
                    elif mode=='symbol':c['files'][0]['result']['symbols']=[None]
                    elif mode=='reference':c['files'][0]['result']['references']=[None]
                    else:c['files'][0]['result']['incomplete']=0
                run,out=self.run_case(change,stale=True)
                self.assertNotEqual(0,run.returncode)
                self.assertIsNone(out)
                self.assertIn('"error": "comparison_refused"',run.stderr)

    def test_later_invalid_file_cannot_publish_partial_population(self):
        import copy
        def change(raw,c):
            raw['files'].append(copy.deepcopy(raw['files'][0]))
            second=copy.deepcopy(c['files'][0]);second['path']='authored/other.jenkinsfile'
            second['result']['symbols'][0]['line']=True
            c['files'].append(second)
        run,out=self.run_case(change,stale=True)
        self.assertNotEqual(0,run.returncode)
        self.assertIsNone(out)
        self.assertIn('"error": "comparison_refused"',run.stderr)

    def test_public_reader_unavailable_reference_location(self):
        """Consume actual public-reader JSON, including its documented zero sentinel."""
        repo=HERE.parents[2]
        with tempfile.TemporaryDirectory(dir=repo/'workspace/tmp') as directory:
            root=Path(directory);inputs=root/'inputs';(inputs/'authored').mkdir(parents=True)
            data=b"pipeline {\n agent any\n stages {\n stage('Build') {\n steps { echo 'ok' }\n }\n }\n}\n"
            (inputs/'authored/Jenkinsfile').write_bytes(data)
            read=subprocess.run(['go','run','./cmd/fathom-lang','extract','-in-process','-root',str(inputs),'authored/Jenkinsfile'],cwd=repo,capture_output=True,text=True)
            self.assertEqual(0,read.returncode,read.stderr)
            candidate=json.loads(read.stdout)
            refs=candidate['files'][0]['result']['references']
            self.assertTrue(refs)
            self.assertEqual({0},{r['line'] for r in refs})
            # Authored owner model is independent of candidate owners/coordinates.
            raw=dict(core_version='authored',tool_declarations=0,files=[dict(subject='authored',sha256=hashlib.sha256(data).hexdigest(),status='parsed',nodes=[dict(type='ModelASTPipelineDef',name=None,line=1,end=8,parent=-1)])])
            (root/'raw.json').write_text(json.dumps(raw))
            (root/'candidate.json').write_text(read.stdout)
            command=[sys.executable,'-B',str(HERE/'compare_candidate.py'),'--raw',str(root/'raw.json'),'--candidate',str(root/'candidate.json'),'--inputs',str(inputs),'--out',str(root/'out.json')]
            run=subprocess.run(command,capture_output=True,text=True)
            self.assertEqual(0,run.returncode,run.stderr)
            out=json.loads((root/'out.json').read_text())
            self.assertEqual(len(refs),out['subjects']['authored']['pairs']['candidate'])
            self.assertFalse(out['tier_granted']);self.assertFalse(out['full_population_native_metrics'])
            self.assertEqual(read.stdout,(root/'candidate.json').read_text())

    def test_reference_coordinates_and_unknown_owner_remain_strict(self):
        for line in (-1, True, False, 1.0, '0', None, 4):
            with self.subTest(line=line):
                def change(raw,c):
                    c['files'][0]['result']['references']=[{'from':'pipeline','to':'echo','line':line}]
                run,out=self.run_case(change,stale=True)
                self.assertNotEqual(0,run.returncode);self.assertIsNone(out)
                self.assertIn('"error": "comparison_refused"',run.stderr)
        for line in (0, 1, 3):
            with self.subTest(valid=line):
                def change(raw,c):
                    c['files'][0]['result']['references']=[{'from':'pipeline','to':'echo','line':line}]
                run,out=self.run_case(change)
                self.assertEqual(0,run.returncode,run.stderr)
                self.assertEqual(1,out['subjects']['authored']['pairs']['candidate'])
        def undefined(raw,c):
            c['files'][0]['result']['references']=[{'from':'missing','to':'echo','line':0}]
        run,out=self.run_case(undefined,stale=True)
        self.assertNotEqual(0,run.returncode);self.assertIsNone(out)
        def outside(raw,c):
            c['files'][0]['result']['symbols'][0].update(line=2,end_line=2)
            c['files'][0]['result']['references']=[{'from':'pipeline','to':'echo','line':1}]
        run,out=self.run_case(outside,stale=True)
        self.assertNotEqual(0,run.returncode);self.assertIsNone(out)
