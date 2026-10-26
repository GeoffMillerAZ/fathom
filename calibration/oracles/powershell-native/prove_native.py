#!/usr/bin/env python3
"""Real native binder/trace context, truth perturbation and cap+1 refusals."""
import argparse
import hashlib
import json
import os
from pathlib import Path
import shutil
import subprocess
import unittest

from fetch import environment,ready
from trace_decode import decode


def digest(value):return hashlib.sha256(json.dumps(value,sort_keys=True,separators=(',',':')).encode()).hexdigest()


class OwnerToolTests(unittest.TestCase):
    def test_native_scope_alias_truth_and_caps(self):
        here,base,out=self.here,self.base,self.out
        subject=out/'subject'
        shutil.copytree(here/'fixtures',subject)
        env=environment(base)
        pwsh=base/'runtime/pwsh'
        files=out/'files.json'
        paths=['module/Probe.psd1','module/Probe.psm1','native.ps1']
        files.write_text(json.dumps(paths)+'\n')
        manifest=subject/'module/Probe.psd1'
        number=0
        def binder(extra=None):
            nonlocal number
            number+=1
            target=out/('binder-'+str(number)+'.json')
            command=[str(pwsh),'-NoLogo','-NoProfile','-NonInteractive','-File',str(here/'ResolveCalls.ps1'),
                     '-Subject',str(subject),'-FilesJson',str(files),'-Manifest',str(manifest),'-Out',str(target)]
            result=subprocess.run(command,env=dict(env,**(extra or {})),capture_output=True,text=True)
            return result,target
        result,path=binder()
        self.assertEqual(result.returncode,0,result.stderr)
        population=json.loads(path.read_text())
        def truth(data):return {k:data[k] for k in ['tool','version','tier','calls']}
        native=truth(population)
        pairs={(p['path'],p['from'],p['to']) for p in native['calls']}
        self.assertEqual(native['tier'],'A')
        self.assertIn(('module/Probe.psm1','PublicEntry','PrivateTarget'),pairs)
        self.assertNotIn(('native.ps1','DeclaredButUnloaded','PrivateTarget'),pairs)
        self.assertNotIn(('native.ps1','DeclaredButUnloaded','git'),pairs)
        self.assertNotIn(('native.ps1','DeclaredButUnloaded','Install-Module'),pairs)
        self.assertIn(('native.ps1','NativeDriver','NativeOne'),pairs)
        self.assertGreater(population['receipt']['unresolved_sites'],0)
        self.assertEqual(population['receipt']['native_module_source_files'],1)
        self.assertGreater(population['receipt']['global_context_sites'],0)
        self.assertGreater(population['receipt']['module_context_sites'],0)
        self.assertEqual({x['name'] for x in population['receipt']['load_after']},
                         {'Microsoft.PowerShell.Management','Microsoft.PowerShell.Utility','Probe'})
        def trace(label,pop):
            target=out/(label+'.json')
            command=[str(pwsh),'-NoLogo','-NoProfile','-NonInteractive','-File',str(here/'TraceFunctions.ps1'),
                     '-Subject',str(subject),'-FilesJson',str(files),'-Out',str(target),'-Driver','fixture']
            result=subprocess.run(command,env=env,capture_output=True,text=True)
            self.assertEqual(result.returncode,0,result.stderr)
            raw=json.loads(target.read_text())
            self.assertEqual(raw['driver_status'],'completed',result.stderr)
            self.assertEqual(raw['retained_breakpoints'],raw['original_breakpoints'])
            oracle,coverage=decode(target,pop)
            return oracle,coverage,target,command
        runtime,coverage,raw,trace_command=trace('native-baseline',population)
        runtime_pairs={(p['path'],p['from'],p['to']) for p in runtime['calls']}
        self.assertIn(('native.ps1','NativeDriver','NativeOne'),runtime_pairs)
        self.assertIn(('module/Probe.psm1','PublicEntry','PrivateTarget'),runtime_pairs)
        self.assertLess(coverage['attested_sites'],coverage['literal_sites'])
        extracted=subprocess.check_output([str(self.indexbench),'lang-probe','-dialect','powershell',str(subject/'native.ps1')])
        artifacts=[subject/'fathom-reading.json',out/'fathom-reading.json']
        for artifact in artifacts:artifact.write_bytes(extracted)
        result,path=binder()
        self.assertEqual(result.returncode,0,result.stderr)
        self.assertEqual(digest(native),digest(truth(json.loads(path.read_text()))))
        for artifact in artifacts:artifact.write_text('{"calls":[{"from":"fabricated","to":"fake"}]}\n')
        result,path=binder()
        self.assertEqual(result.returncode,0,result.stderr)
        perturbed=truth(json.loads(path.read_text()))
        self.assertEqual(digest(native),digest(perturbed))
        redecode,_=decode(raw,population)
        self.assertEqual(digest(runtime),digest(redecode))
        source=subject/'module/Probe.psm1'
        original=source.read_text()
        self.assertEqual(original.count('Set-Alias nativeAlias NativeOne'),1)
        source.write_text(original.replace('Set-Alias nativeAlias NativeOne','Set-Alias nativeAlias NativeTwo'))
        result,path=binder()
        self.assertEqual(result.returncode,0,result.stderr)
        changed_population=json.loads(path.read_text())
        changed=truth(changed_population)
        changed_pairs={(p['path'],p['from'],p['to']) for p in changed['calls']}
        removed={('native.ps1','NativeDriver','NativeOne'),('module/Probe.psm1','PublicEntry','NativeOne')}
        added={('native.ps1','NativeDriver','NativeTwo'),('module/Probe.psm1','PublicEntry','NativeTwo')}
        self.assertEqual(pairs-changed_pairs,removed)
        self.assertEqual(changed_pairs-pairs,added)
        changed_runtime,_,_,_=trace('changed-native',changed_population)
        changed_runtime_pairs={(p['path'],p['from'],p['to']) for p in changed_runtime['calls']}
        self.assertEqual(runtime_pairs-changed_runtime_pairs,removed)
        self.assertEqual(changed_runtime_pairs-runtime_pairs,added)
        source.write_text(original)
        caps=[]
        total=sum((subject/p).stat().st_size for p in paths)
        site_count=len(population['sites'])
        for variable,limit,count in [('PWSH_ORACLE_FILE_CAP',2,3),('PWSH_ORACLE_BYTE_CAP',total-1,total),
                                     ('PWSH_ORACLE_SITE_CAP',site_count-1,site_count)]:
            result,path=binder({variable:str(limit)})
            self.assertNotEqual(result.returncode,0)
            self.assertIn('whole-unit refusal',result.stderr)
            self.assertFalse(path.exists())
            caps.append({'cap':variable,'limit':limit,'input_count':count,'whole_unit_refused':True})
        capped=out/'trace-cap.json'
        command=trace_command.copy()
        command[command.index('-Out')+1]=str(capped)
        result=subprocess.run(command,env=dict(env,PWSH_TRACE_ROW_CAP='1'),capture_output=True,text=True)
        self.assertEqual(result.returncode,0,result.stderr)
        self.assertTrue(json.loads(capped.read_text())['refused'])
        with self.assertRaisesRegex(ValueError,'whole-unit refusal'):decode(capped,population)
        caps.append({'cap':'PWSH_TRACE_ROW_CAP','limit':1,'input_count_at_refusal':2,'whole_unit_refused':True})
        self.receipt.update(binder_before_sha256=digest(native),binder_after_extractor_perturbation_sha256=digest(perturbed),
            binder_after_native_input_change_sha256=digest(changed),trace_before_sha256=digest(runtime),
            trace_after_extractor_perturbation_sha256=digest(redecode),trace_after_native_input_change_sha256=digest(changed_runtime),
            known_input_change='native alias NativeOne to NativeTwo changes exactly two caller/target pairs in each route',
            fixture_literal_sites=site_count,caps=caps,
            declared_scope='private target resolves only inside native module-owned source; ordinary source does not inherit module privacy; external/unknown/member sites remain misses',
            module_load_state='recorded runtime Management/Utility plus explicit Probe only; autoload disabled',
            script_qualified_breakpoint_regression='global native breakpoint survives New-Module; script-qualified breakpoint mutant must fail')


def main():
    ap=argparse.ArgumentParser(description=__doc__)
    ap.add_argument('--tools',type=Path,required=True)
    ap.add_argument('--indexbench',type=Path,required=True)
    ap.add_argument('--out',type=Path,required=True)
    args=ap.parse_args()
    ready()
    OwnerToolTests.here=Path(__file__).resolve().parent
    OwnerToolTests.base=args.tools.resolve()
    OwnerToolTests.out=args.out.resolve()
    OwnerToolTests.indexbench=args.indexbench.resolve()
    OwnerToolTests.out.mkdir(parents=True,exist_ok=False)
    OwnerToolTests.receipt={}
    result=unittest.TextTestRunner(verbosity=2).run(unittest.defaultTestLoader.loadTestsFromTestCase(OwnerToolTests))
    if not result.wasSuccessful():raise SystemExit(1)
    OwnerToolTests.receipt['oracle_code_sha256']={p.name:hashlib.sha256(p.read_bytes()).hexdigest() for p in OwnerToolTests.here.iterdir() if p.is_file()}
    (OwnerToolTests.out/'proof.json').write_text(json.dumps(OwnerToolTests.receipt,indent=2,sort_keys=True)+'\n')


if __name__=='__main__':main()
