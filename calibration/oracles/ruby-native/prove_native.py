#!/usr/bin/env python3
"""Released TypeProf/TracePoint truth, perturbations, cap+1 refusal and ISeq."""
import argparse
import hashlib
import json
import os
from pathlib import Path
import subprocess
import unittest

from bootstrap import environment, ready
from trace_decode import decode


def digest(value):
    return hashlib.sha256(json.dumps(value,sort_keys=True,separators=(',',':')).encode()).hexdigest()


class OwnerToolTests(unittest.TestCase):
    def test_owner_answers_and_perturbations(self):
        here,base,out=self.here,self.base,self.out
        state=json.loads((base/'bootstrap.json').read_text())
        ruby=base/state['ruby']['binary']
        env=environment(base,ruby)
        root=out/'subject'
        root.mkdir()
        fixture=root/'native.rb'
        original=(here/'fixtures/native.rb').read_text()
        fixture.write_text(original)
        raw=out/'native'
        raw.mkdir()
        command=[str(ruby),str(here/'typeprof_export.rb'),'--fixture',str(root)]
        def typeprof(extra=None):
            result=subprocess.run(command,env=dict(env,**(extra or {})),capture_output=True,text=True)
            self.assertEqual(result.returncode,0,result.stderr)
            return json.loads(result.stdout)
        native=typeprof()
        pairs={(x['from'],x['to']) for x in native['calls']}
        self.assertEqual(native['tier'],'A')
        self.assertIn(('driver','inherited'),pairs)
        self.assertNotIn(('driver','alternate'),pairs)
        self.assertEqual(native['receipt']['unresolved_native_sites'],2)
        self.assertEqual(native['receipt']['unresolved_policy'],'misses, never dropped')
        syntax=json.loads(subprocess.check_output([str(base/'pyts/bin/python'),'-B',str(here/'syntax_sites.py'),
                                                 '--fixture',str(root)]))
        self.assertEqual(len(syntax['sites']),8)
        trace_env=dict(env,RUBY_ORACLE_SUBJECT=str(root),RUBY_ORACLE_TRACE_DIR=str(raw),
                       RUBYOPT='-r'+str(here/'trace_hook.rb'))
        run=subprocess.run([str(ruby),'-r'+str(fixture),'-e','NativeChild.new.inherited'],
                           env=trace_env,capture_output=True,text=True)
        self.assertEqual(run.returncode,0,run.stderr)
        trace,coverage=decode(list(raw.glob('native-*.json')),syntax)
        self.assertEqual(coverage['attested_sites'],1)
        self.assertEqual(coverage['literal_sites'],8)
        self.assertEqual(trace['calls'],[{'path':'native.rb','from':'inherited','to':'length'}])
        # Put actual Fathom output inside BOTH directories used by collection,
        # then alter it in place. A future broad glob would see these artifacts.
        extracted=subprocess.check_output([str(self.indexbench),'lang-probe','-dialect','ruby',str(fixture)])
        artifacts=[root/'fathom-reading.json',raw/'fathom-reading.json']
        for p in artifacts:p.write_bytes(extracted)
        before_native=typeprof()
        before_trace,_=decode(list(raw.glob('native-*.json')),syntax)
        for p in artifacts:p.write_text('{"calls":[{"from":"fake","to":"fabricated"}]}\n')
        self.assertNotEqual(hashlib.sha256(extracted).hexdigest(),hashlib.sha256(artifacts[0].read_bytes()).hexdigest())
        after_native=typeprof()
        after_trace,_=decode(list(raw.glob('native-*.json')),syntax)
        self.assertEqual(digest(native),digest(before_native))
        self.assertEqual(digest(native),digest(after_native))
        self.assertEqual(digest(trace),digest(before_trace))
        self.assertEqual(digest(trace),digest(after_trace))
        self.assertEqual(original.count("'x'.length"),1)
        fixture.write_text(original.replace("'x'.length","'x'.bytesize"))
        changed=typeprof()
        changed_pairs={(x['from'],x['to']) for x in changed['calls']}
        self.assertEqual(pairs-changed_pairs,{('inherited','length')})
        self.assertEqual(changed_pairs-pairs,{('inherited','bytesize')})
        changed_raw=out/'changed-native'
        changed_raw.mkdir()
        changed_env=dict(trace_env,RUBY_ORACLE_TRACE_DIR=str(changed_raw))
        result=subprocess.run([str(ruby),'-r'+str(fixture),'-e','NativeChild.new.inherited'],
                              env=changed_env,capture_output=True,text=True)
        self.assertEqual(result.returncode,0,result.stderr)
        changed_syntax=json.loads(subprocess.check_output([str(base/'pyts/bin/python'),'-B',str(here/'syntax_sites.py'),
                                                          '--fixture',str(root)]))
        changed_trace,_=decode(list(changed_raw.glob('native-*.json')),changed_syntax)
        self.assertEqual(changed_trace['calls'],[{'path':'native.rb','from':'inherited','to':'bytesize'}])
        fixture.write_text(original)
        self.receipt.update(typeprof_truth_before_sha256=digest(native),typeprof_after_extractor_perturbation_sha256=digest(after_native),
            typeprof_after_native_input_change_sha256=digest(changed),tracepoint_truth_before_sha256=digest(trace),
            tracepoint_after_extractor_perturbation_sha256=digest(after_trace),
            tracepoint_after_native_input_change_sha256=digest(changed_trace),
            perturbation_locations='inside native input directory and original subject root; no extractor file is a native input',
            native_input_change='length to bytesize; exactly one native pair changes in each route',fixture_literal_sites=8,
            fixture_native_unresolved_sites=native['receipt']['unresolved_native_sites'])
        caps=[]
        second=root/'second.rb'
        second.write_text('def other; 1; end\n')
        subprocess.run(['git','init','-q',str(root)],check=True)
        subprocess.run(['git','-C',str(root),'add','native.rb','second.rb'],check=True)
        # The non-fixture route sees two original tracked Ruby files.
        full_command=[str(ruby),str(here/'typeprof_export.rb'),str(root)]
        for variable,value,message,input_count in [
            ('TYPEPROF_FILE_CAP','1','file count cap',2),
            ('TYPEPROF_BYTE_CAP',str(len(original.encode())+second.stat().st_size-1),'source byte cap',len(original.encode())+second.stat().st_size),
            ('TYPEPROF_PAIR_CAP',str(len(native['calls'])-1),'native answer count cap',len(native['calls']))]:
            result=subprocess.run(full_command,env=dict(env,**{variable:value}),capture_output=True,text=True)
            self.assertNotEqual(result.returncode,0)
            self.assertEqual(result.stdout,'')
            self.assertIn('whole-unit refusal: '+message,result.stderr)
            caps.append({'cap':variable,'limit':int(value),'input_count_at_least':input_count,'whole_unit_refused':True})
        for variable,limit,message,count in [
            ('RUBY_SITE_FILE_CAP',1,'syntax file cap',2),
            ('RUBY_SITE_BYTE_CAP',len(original.encode())+second.stat().st_size-1,'syntax byte cap',len(original.encode())+second.stat().st_size),
            ('RUBY_SITE_SITE_CAP',len(syntax['sites'])-1,'syntax site cap',len(syntax['sites']))]:
            result=subprocess.run([str(base/'pyts/bin/python'),'-B',str(here/'syntax_sites.py'),str(root)],
                                  env=dict(env,**{variable:str(limit)}),capture_output=True,text=True)
            self.assertNotEqual(result.returncode,0)
            self.assertEqual(result.stdout,'')
            self.assertIn('whole-unit refusal: '+message,result.stderr)
            caps.append({'cap':variable,'limit':limit,'input_count':count,'whole_unit_refused':True})
        for variable in ['RUBY_ORACLE_ROW_CAP','RUBY_ORACLE_FUNCTION_CAP']:
            capdir=out/variable.lower()
            capdir.mkdir()
            result=subprocess.run([str(ruby),'-r'+str(fixture),'-e','NativeChild.new.driver rescue NameError'],
                env=dict(trace_env,RUBY_ORACLE_TRACE_DIR=str(capdir),**{variable:'1'}),capture_output=True,text=True)
            self.assertEqual(result.returncode,0,result.stderr)
            files=list(capdir.glob('native-*.json'))
            self.assertTrue(files)
            self.assertTrue(json.loads(files[0].read_text())['refused'])
            with self.assertRaisesRegex(ValueError,'whole-unit refusal'):decode(files,syntax)
            caps.append({'cap':variable,'limit':1,'input_count_at_least':2,'whole_unit_refused':True})
        self.receipt['owner_caps']=caps
        iseq=subprocess.check_output([str(ruby),'-e',"puts RubyVM::InstructionSequence.compile('def driver(obj); obj.target; end').disasm"],env=env,text=True)
        self.assertIn('mid:target',iseq)
        self.receipt['iseq_contract']={'ruby':'3.3.8','written_send_target_present':True,
                                     'receiver_target_binding':'absent: compile/disassembly contains a written send, no evaluated receiver',
                                     'dump_sha256':hashlib.sha256(iseq.encode()).hexdigest()}


def main():
    ap=argparse.ArgumentParser(description=__doc__)
    ap.add_argument('--tools',type=Path,required=True)
    ap.add_argument('--out',type=Path,required=True)
    ap.add_argument('--indexbench',type=Path,required=True)
    args=ap.parse_args()
    ready()
    OwnerToolTests.here=Path(__file__).resolve().parent
    OwnerToolTests.base=args.tools.resolve()
    OwnerToolTests.out=args.out.resolve()
    OwnerToolTests.indexbench=args.indexbench.resolve()
    OwnerToolTests.receipt={}
    OwnerToolTests.out.mkdir(parents=True,exist_ok=False)
    result=unittest.TextTestRunner(verbosity=2).run(unittest.defaultTestLoader.loadTestsFromTestCase(OwnerToolTests))
    if not result.wasSuccessful():raise SystemExit(1)
    receipt=OwnerToolTests.receipt
    receipt['oracle_code_sha256']={p.name:hashlib.sha256(p.read_bytes()).hexdigest() for p in OwnerToolTests.here.glob('*') if p.is_file()}
    (OwnerToolTests.out/'proof.json').write_text(json.dumps(receipt,indent=2,sort_keys=True)+'\n')


if __name__=='__main__':main()
