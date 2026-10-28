from pathlib import Path
import tempfile,unittest
from function_decode import decode


class NativeFunctionTests(unittest.TestCase):
    def setUp(self):
        parent=Path(__file__).resolve().parents[3]/'workspace/tmp'
        parent.mkdir(parents=True,exist_ok=True)
        temp=tempfile.TemporaryDirectory(dir=parent);self.addCleanup(temp.cleanup)
        self.root=Path(temp.name);self.trace=self.root/'calls.trace'
        self.caller=self.root/'native.sh';self.caller.touch()
        self.target=self.root/'definitions.sh';self.target.touch()
        self.fields=['CALL',str(self.caller),'3','driver','resolved_target',str(self.target)]
        self.population={'sites':[{'path':'native.sh','line':3,'from':'driver','written_name':'alias_name','literal':True}]}
        self.save()

    def save(self):self.trace.write_bytes(('\0'.join(self.fields)+'\0').encode())

    def test_actual_native_alias_target_is_truth(self):
        pairs,hits,_=decode(self.trace,self.root,self.population)
        self.assertEqual(pairs,{('native.sh','driver','resolved_target')})
        self.assertEqual(hits,{0})

    def test_wrong_native_caller_cannot_use_parser_guess(self):
        self.fields[3]='implicit_callback';self.save()
        pairs,hits,_=decode(self.trace,self.root,self.population)
        self.assertEqual(pairs,{('native.sh','implicit_callback','resolved_target')})
        self.assertEqual(hits,set())

    def test_multiple_commands_on_one_line_stay_unattested(self):
        self.population['sites'].append(dict(self.population['sites'][0],written_name='other'))
        pairs,hits,_=decode(self.trace,self.root,self.population)
        self.assertEqual(pairs,{('native.sh','driver','resolved_target')})
        self.assertEqual(hits,set())

    def test_dynamic_call_cannot_impersonate_a_literal(self):
        self.population['sites'][0]['literal']=False
        pairs,hits,_=decode(self.trace,self.root,self.population)
        self.assertEqual(pairs,{('native.sh','driver','resolved_target')})
        self.assertEqual(hits,set())

    def test_no_parser_site_still_enters_native_population(self):
        self.population['sites']=[]
        pairs,hits,_=decode(self.trace,self.root,self.population)
        self.assertEqual(pairs,{('native.sh','driver','resolved_target')})
        self.assertEqual(hits,set())

    def test_relative_native_filename_is_never_decoder_cwd(self):
        self.fields[1]='native.sh';self.save()
        self.assertEqual(decode(self.trace,self.root,self.population)[1],set())

    def test_incomplete_native_tail_refuses(self):
        self.trace.write_bytes(self.trace.read_bytes()[:-1])
        with self.assertRaisesRegex(ValueError,'whole-unit refusal'):decode(self.trace,self.root,self.population)

    def test_byte_cap_plus_one(self):
        with self.assertRaisesRegex(ValueError,'byte cap'):
            decode(self.trace,self.root,self.population,max_bytes=self.trace.stat().st_size-1)

    def test_field_count_cap_plus_one(self):
        with self.assertRaisesRegex(ValueError,'field count cap'):
            decode(self.trace,self.root,self.population,max_fields=5)

    def test_field_bytes_cap_plus_one(self):
        largest=max(len(x.encode()) for x in self.fields)
        with self.assertRaisesRegex(ValueError,'field byte cap'):
            decode(self.trace,self.root,self.population,max_field_bytes=largest-1)

    def test_relocated_capture_names_original_absolute_path_requirement(self):
        import hashlib,json,shutil
        from unittest.mock import patch
        import redecode
        original=self.root/'capture';phase=original/'phases/0';phase.mkdir(parents=True)
        self.fields[1]=str(original/'child/native.sh')
        self.fields[5]=str(original/'child/definitions.sh');self.save()
        shutil.copyfile(self.trace,phase/'functions.trace')
        summary=dict(instrument_sha256={},source_sha256={'native.sh':hashlib.sha256(self.caller.read_bytes()).hexdigest()},
            function_population_sha256=hashlib.sha256(json.dumps(self.population,sort_keys=True).encode()).hexdigest(),
            function_attested_sites=1,function_pairs=[dict(path='native.sh',**{'from':'driver','to':'resolved_target'})],
            runs=[dict(phase='phases/0',raw_sha256={'functions.trace':hashlib.sha256(self.trace.read_bytes()).hexdigest()},
                       changed_source=False,timeout=False,exit=0)])
        (original/'summary.json').write_text(json.dumps(summary))
        moved=self.root/'restored';shutil.copytree(original,moved)
        command=['redecode.py','--subject',str(self.root),'--shfmt','unused-fixture-parser',
                 '--capture-revision','0'*40]
        with patch.object(redecode,'collect',return_value=self.population):
            with patch('sys.argv',command+['--native',str(moved),'--out',str(self.root/'refused')]):
                with self.assertRaisesRegex(AssertionError,'recorded absolute path'):
                    redecode.main()
            self.assertFalse((self.root/'refused').exists())
            with patch('sys.argv',command+['--native',str(original),'--out',str(self.root/'accepted')]):
                redecode.main()
        accepted=json.loads((self.root/'accepted/summary.json').read_text())
        self.assertEqual(accepted['function_pairs'],summary['function_pairs'])
        self.assertEqual((moved/'phases/0/functions.trace').read_bytes(),self.trace.read_bytes())



class CertifiedPhaseTests(unittest.TestCase):
    def test_rejected_phase_drops_every_function_measure(self):
        from collect import merge_function_phase
        pair=('fixture.sh','caller','target')
        for changed,timed,exit_code in [(False,True,-9),(False,True,0),(True,False,0),(False,False,-9)]:
            pairs,hits=set(),set()
            count=merge_function_phase(pairs,hits,{pair},{3},7,changed=changed,timed=timed,exit_code=exit_code)
            self.assertEqual((pairs,hits,count),(set(),set(),0))
        pairs,hits=set(),set()
        count=merge_function_phase(pairs,hits,{pair},{3},7,changed=False,timed=False,exit_code=2)
        self.assertEqual((pairs,hits,count),({pair},{3},7))

if __name__=='__main__':unittest.main()
