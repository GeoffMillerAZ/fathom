"""Receipt honesty controls for an authored offline feasibility probe."""
import contextlib
import io
import os
import runpy
import traceback
from unittest import mock
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

class CorrectionTest(OfflineProbeTest):
    def invoke(self, root, content, runtime=False, out=None):
        here=Path(__file__).resolve().parent
        out=out or root/('result' if runtime else 'summary.json')
        if runtime:
            dotnet=root/'dotnet';assembly=root/'probe.dll'
            dotnet.write_bytes(b'mocked SDK');assembly.write_bytes(b'mocked assembly')
            argv=[str(here/'run_offline_211.py'),'--dotnet',str(dotnet),'--assembly',str(assembly),'--out',str(out)]
        else:
            raw=root/'input.json';raw.write_bytes(content)
            argv=[str(here/'offline_probe_211.py'),'--raw',str(raw),'--out',str(out)]
        capture=io.StringIO();code=0
        launched=mock.Mock(return_value=subprocess.CompletedProcess([],0,content,b''))
        with mock.patch.object(sys,'argv',argv), mock.patch('subprocess.run',launched), contextlib.redirect_stdout(capture), contextlib.redirect_stderr(capture):
            try:runpy.run_path(argv[0],run_name='__main__')
            except SystemExit as exc:code=exc.code
            except Exception:
                code=1;capture.write(traceback.format_exc())
        return code,capture.getvalue(),out,launched

    def scratch(self):
        return tempfile.TemporaryDirectory(dir=Path(__file__).resolve().parents[3]/'workspace/tmp')

    def assert_refused(self, result, runtime):
        code,output,out,_=result
        self.assertEqual(1,code,output)
        self.assertIn('receipt_refused:',output)
        self.assertNotIn('Traceback',output)
        self.assertFalse((out/'receipt.json').exists() if runtime else out.exists())
        self.assertNotIn('"support_claim": true',output)

    def test_recursive_duplicate_keys_both_entrypoints(self):
        good=json.dumps(self.receipt())
        variants=[good.replace('"support_claim": false','"support_claim": true, "support_claim": false'),
                  good.replace('"case": "template"','"case": "wrong", "case": "template"'),
                  good.replace('"cases": [','"extra": {"x": 1, "x": 2}, "cases": [')]
        for runtime in (False,True):
            for content in variants:
                with self.subTest(runtime=runtime,content=content),self.scratch() as folder:
                    self.assert_refused(self.invoke(Path(folder),content.encode(),runtime),runtime)

    def test_wrong_shape_families_both_entrypoints(self):
        variants=[None,False,7,1.2,'receipt',[],[{}]]
        for field in self.receipt():
            for value in (None,False,7,'bad',[],{}):
                if value is False and field in ('support_claim','native_owner_attested'):continue
                raw=self.receipt();raw[field]=value;variants.append(raw)
        for value in (None,False,7,'row',[],['row']):
            raw=self.receipt();raw['cases'][0]=value;variants.append(raw)
        for field in self.receipt()['cases'][0]:
            for value in (None,False,7,{},[],['bad']):
                raw=self.receipt();raw['cases'][0][field]=value;variants.append(raw)
        for runtime in (False,True):
            for raw in variants:
                with self.subTest(runtime=runtime,raw=raw),self.scratch() as folder:
                    self.assert_refused(self.invoke(Path(folder),json.dumps(raw).encode(),runtime),runtime)

    def test_valid_then_invalid_removes_owned_success(self):
        for runtime in (False,True):
            for content in (b'{',b'null',json.dumps(dict(self.receipt(),support_claim=True)).encode(),b' '*(1024**2+1)):
                with self.subTest(runtime=runtime,content=content[:30]),self.scratch() as folder:
                    root=Path(folder)
                    code,output,out,_=self.invoke(root,json.dumps(self.receipt()).encode(),runtime)
                    self.assertEqual(0,code,output)
                    self.assertTrue((out/'receipt.json').exists() if runtime else out.exists())
                    self.assert_refused(self.invoke(root,content,runtime,out),runtime)

    def test_alias_refused_before_input_mutation(self):
        here=Path(__file__).resolve().parent
        for mode in ('same','symlink','hardlink'):
            with self.subTest(mode=mode),self.scratch() as folder:
                root=Path(folder);raw=root/'input.json';content=json.dumps(self.receipt()).encode();raw.write_bytes(content)
                out=raw if mode=='same' else root/'summary.json'
                if mode=='symlink':out.symlink_to(raw)
                if mode=='hardlink':os.link(raw,out)
                code,output,_,_=self.invoke(root,content,out=out)
                self.assertEqual(1,code,output);self.assertIn('receipt_refused: aliased input/output',output)
                self.assertEqual(content,raw.read_bytes());self.assertEqual(content,out.read_bytes())
        for name in ('receipt.json','raw.json','stderr.log'):
            with self.subTest(runtime_alias=name),self.scratch() as folder:
                root=Path(folder);out=root/'result';out.mkdir();assembly=root/'probe.dll';assembly.write_bytes(b'mocked assembly')
                (out/name).symlink_to(assembly)
                code,output,_,launched=self.invoke(root,json.dumps(self.receipt()).encode(),True,out)
                self.assertEqual(1,code,output);self.assertIn('receipt_refused: aliased input/output',output)
                self.assertEqual(b'mocked assembly',assembly.read_bytes());launched.assert_not_called()

    def test_wrong_shapes_are_typed_refusals(self):
        import offline_probe_211 as probe
        for raw in (None,[],dict(self.receipt(),cases=[None]*4)):
            with self.subTest(raw=raw):
                try:validate(raw)
                except Exception as exc:
                    self.assertEqual('ReceiptRefusal',type(exc).__name__)
                    self.assertIsInstance(exc,ValueError)
                else:self.fail('invalid receipt was accepted')

    def test_runtime_guards_override_inherited_values(self):
        required={'DOTNET_GENERATE_ASPNET_CERTIFICATE':'false','DOTNET_CLI_TELEMETRY_OPTOUT':'1',
                  'DOTNET_NOLOGO':'1','DOTNET_ADD_GLOBAL_TOOLS_TO_PATH':'false',
                  'DOTNET_SKIP_FIRST_TIME_EXPERIENCE':'1','DOTNET_CLI_USE_MSBUILD_SERVER':'0',
                  'MSBUILDDISABLENODEREUSE':'1','DOTNET_CLI_WORKLOAD_UPDATE_NOTIFY_DISABLE':'1'}
        with self.scratch() as folder,mock.patch.dict(os.environ,{key:'0' for key in required}):
            root=Path(folder)
            code,output,out,launched=self.invoke(root,json.dumps(self.receipt()).encode(),True)
            self.assertEqual(0,code,output);launched.assert_called_once()
            env=launched.call_args.kwargs['env']
            for key,value in required.items():self.assertEqual(value,env.get(key),key)
            for key in ('DOTNET_CLI_HOME','NUGET_PACKAGES','NUGET_HTTP_CACHE_PATH'):
                self.assertTrue(Path(env[key]).is_relative_to(out),key)
            self.assertEqual(os.environ.get('HOME'),env.get('HOME'))
            self.assertEqual('/usr/bin/sandbox-exec',launched.call_args.args[0][0])
            self.assertIn('(deny network*)',launched.call_args.args[0][2])

if __name__=='__main__':unittest.main()
