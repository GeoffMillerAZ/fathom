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
        self.assertEqual(decode(self.trace,self.root,self.population)[0],set())

    def test_multiple_commands_on_one_line_stay_unattested(self):
        self.population['sites'].append(dict(self.population['sites'][0],written_name='other'))
        self.assertEqual(decode(self.trace,self.root,self.population)[1],set())

    def test_dynamic_call_cannot_impersonate_a_literal(self):
        self.population['sites'][0]['literal']=False
        self.assertEqual(decode(self.trace,self.root,self.population)[1],set())

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


if __name__=='__main__':unittest.main()
