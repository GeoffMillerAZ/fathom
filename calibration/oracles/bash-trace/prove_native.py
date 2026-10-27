#!/usr/bin/env python3
"""Actual Bash/SCIP independence, known-input changes and native filename proofs."""
import argparse,hashlib,importlib.util,json,os
from pathlib import Path
import shutil,subprocess,unittest
from build_bash import ready
from trace_driver import decode as sources
from function_decode import decode as calls
from call_sites import collect


def digest(value):return hashlib.sha256(json.dumps(value,sort_keys=True).encode()).hexdigest()


class OwnerProof(unittest.TestCase):
    def test_actual_native_truth(self):
        here,base,out=self.here,self.base,self.out
        subject=out/'subject';shutil.copytree(here/'fixtures',subject)
        native=out/'native';native.mkdir()
        spec=importlib.util.spec_from_file_location('bash_syntax',here.parent/'bash-source/bash_source_dump.py')
        syntax=importlib.util.module_from_spec(spec);spec.loader.exec_module(syntax)
        population=collect(subject,['functions.sh'],str(base/'shfmt'),syntax)
        bash=base/'bash-5.3/bash'
        def run(filename,extra=None):
            source=native/'source.trace';function=native/'function.trace'
            source.write_bytes(b'');function.write_bytes(b'')
            env=dict(os.environ,LC_ALL='C',ORACLE_FULLPATH='1',BASH_ENV=str(here/'entry_hook.sh'),
                     ORACLE_TRACE_FILE=str(source),ORACLE_CALL_TRACE_FILE=str(function))
            env.update(extra or {})
            result=subprocess.run([str(bash),str(subject/filename)],cwd=subject,env=env,capture_output=True,timeout=20)
            self.assertEqual(result.returncode,0,result.stderr)
            rows,unknown=sources(source,subject)
            pairs,hits,unpaired=calls(function,subject,population)
            return rows,pairs,source.read_bytes(),function.read_bytes()
        rows,_,_,_=run('probe.sh')
        self.assertEqual(len(rows),6)
        self.assertIn(('probe.sh',11,'defs.sh'),rows)
        self.assertEqual(run('shadow-source.sh')[0],[])
        run('strict.sh')
        self.assertEqual(run('shadow-builtins.sh')[0],[('shadow-builtins.sh',8,'leaf.sh'),('shadow-builtins.sh',9,'defs.sh')])
        _,pairs,_,_=run('functions.sh')
        self.assertIn(('functions.sh','driver','target_one'),pairs)
        self.assertIn(('functions.sh','<module>','driver'),pairs)
        with self.assertRaisesRegex(ValueError,'whole-unit refusal'):
            run('functions.sh',{'ORACLE_CALL_ROW_CAP':'1'})
        before_source,_,source_raw,_=run('probe.sh')
        _,before_calls,_,call_raw=run('functions.sh')
        extracted=subprocess.check_output([str(self.indexbench),'lang-probe','-dialect','bash',str(subject/'functions.sh')])
        for path in [subject/'fathom-reading.json',native/'fathom-reading.json']:path.write_bytes(extracted)
        for path in [subject/'fathom-reading.json',native/'fathom-reading.json']:path.write_text('{"calls":[{"from":"fake","to":"fabricated"}]}\n')
        after_source,_,changed_source_raw,_=run('probe.sh')
        _,after_calls,_,changed_call_raw=run('functions.sh')
        self.assertEqual(before_source,after_source);self.assertEqual(before_calls,after_calls)
        self.assertEqual(source_raw,changed_source_raw);self.assertEqual(call_raw,changed_call_raw)
        source=subject/'probe.sh';source.write_text(source.read_text().replace('. ./leaf.sh','. ./other.sh',1))
        shutil.copyfile(subject/'leaf.sh',subject/'other.sh')
        changed_source=run('probe.sh')[0];self.assertNotEqual(before_source,changed_source)
        function=subject/'functions.sh';function.write_text(function.read_text().replace('alias shorthand=target_one','alias shorthand=target_two'))
        changed_calls=run('functions.sh')[1]
        self.assertEqual(before_calls-changed_calls,{('functions.sh','driver','target_one')})
        self.assertEqual(changed_calls-before_calls,{('functions.sh','driver','target_two')})
        # SCIP receives explicit original inputs, not the extractor artifacts.
        index=native/'index.scip'
        def scip():
            subprocess.run([str(base/'scip-target/release/scip-shell'),'--project-root',str(subject),'-o',str(index),str(function)],capture_output=True,check=True)
            return hashlib.sha256(index.read_bytes()).hexdigest()
        original_scip=scip()
        for path in [subject/'fathom-reading.json',native/'fathom-reading.json']:path.write_bytes(extracted)
        self.assertEqual(original_scip,scip())
        for path in [subject/'fathom-reading.json',native/'fathom-reading.json']:path.write_text('fabricated')
        self.assertEqual(original_scip,scip())
        function.write_text(function.read_text().replace('target_one','changed_target'))
        changed_scip=scip();self.assertNotEqual(original_scip,changed_scip)
        self.receipt.update(source_before=digest(before_source),source_after_extractor_change=digest(after_source),source_after_native_change=digest(changed_source),
            calls_before=digest(sorted(before_calls)),calls_after_extractor_change=digest(sorted(after_calls)),calls_after_native_change=digest(sorted(changed_calls)),
            scip_before=original_scip,scip_after_extractor_change=original_scip,scip_after_native_change=changed_scip,
            source_raw_before_sha256=hashlib.sha256(source_raw).hexdigest(),source_raw_after_extractor_change_sha256=hashlib.sha256(changed_source_raw).hexdigest(),
            calls_raw_before_sha256=hashlib.sha256(call_raw).hexdigest(),calls_raw_after_extractor_change_sha256=hashlib.sha256(changed_call_raw).hexdigest(),
            source_fixture_edges=6,shadowed_source_edges=0,shadowed_metadata_builtin_edges=2,
            native_alias_change='exactly one actual driver/target pair changes from target_one to target_two',canonical_reading_changed=False)


def main():
    p=argparse.ArgumentParser(description=__doc__)
    for n in ['tools','indexbench','out']:p.add_argument('--'+n,type=Path,required=True)
    p.add_argument('--allow-loaded-host',action='store_true',help='owner Q7 permits remaining checks; never a performance measurement')
    a=p.parse_args()
    if a.allow_loaded_host:
        assert shutil.disk_usage(Path.home()).free>=20*(1<<30),'requires 20 GiB free'
        assert os.getpriority(os.PRIO_PROCESS,0)>=10,'requires nice10'
    else:ready()
    OwnerProof.here=Path(__file__).resolve().parent;OwnerProof.base=a.tools.resolve();OwnerProof.out=a.out.resolve();OwnerProof.indexbench=a.indexbench.resolve()
    OwnerProof.out.mkdir(parents=True,exist_ok=False);OwnerProof.receipt=dict(allow_loaded_host_owner_Q7=a.allow_loaded_host,observed_start_load=os.getloadavg()[0],nice=os.getpriority(os.PRIO_PROCESS,0),performance_claim=False)
    result=unittest.TextTestRunner(verbosity=2).run(unittest.defaultTestLoader.loadTestsFromTestCase(OwnerProof))
    if not result.wasSuccessful():raise SystemExit(1)
    OwnerProof.receipt['code_sha256']={p.name:hashlib.sha256(p.read_bytes()).hexdigest() for p in OwnerProof.here.iterdir() if p.is_file()}
    (OwnerProof.out/'proof.json').write_text(json.dumps(OwnerProof.receipt,indent=2,sort_keys=True)+'\n')


if __name__=='__main__':main()
