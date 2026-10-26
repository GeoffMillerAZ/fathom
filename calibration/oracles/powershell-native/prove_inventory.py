#!/usr/bin/env python3
"""Native declaration discovery, complete load partition and fixture refusal."""
import argparse
import hashlib
import json
from pathlib import Path
import shutil
import subprocess
import unittest
from fetch import environment, ready


class InventoryTests(unittest.TestCase):
    def test_every_declared_helper_is_loaded_or_named(self):
        here,base,out=self.here,self.base,self.out
        subject=out/'subject'
        shutil.copytree(here/'fixtures',subject)
        paths=sorted(p.relative_to(subject).as_posix() for p in subject.rglob('*') if p.suffix in {'.ps1','.psm1','.psd1'})
        files=out/'files.json'
        files.write_text(json.dumps(paths)+'\n')
        target=out/'native.json'
        command=[str(base/'runtime/pwsh'),'-NoLogo','-NoProfile','-NonInteractive','-File',str(here/'ResolveCalls.ps1'),
            '-Subject',str(subject),'-FilesJson',str(files),'-Repository',str(subject),'-TrackedJson',str(files),
            '-Harness','pester','-Manifest',str(subject/'module/Probe.psd1'),'-Out',str(target)]
        result=subprocess.run(command,env=environment(base),capture_output=True,text=True)
        self.assertEqual(result.returncode,0,result.stderr)
        raw=json.loads(target.read_text())
        inventory=raw['harness_inventory']
        entries={Path(d['target']).name:d for d in inventory}
        self.assertEqual(set(entries),{'Declared.psm1','Preamble.ps1','Executable.ps1','Absent.psm1'})
        self.assertEqual(entries['Declared.psm1']['status'],'loaded')
        self.assertEqual(entries['Preamble.ps1']['status'],'loaded')
        self.assertEqual(entries['Executable.ps1']['status'],'declared_not_loaded')
        self.assertTrue(all(d['reason'] for d in inventory))
        self.assertTrue(all(len(d['code_sha256'])==64 for d in inventory if d['status']=='loaded'))
        self.assertEqual(entries['Absent.psm1']['status'],'declared_not_loaded')
        self.assertIsNone(entries['Absent.psm1']['code_sha256'])
        sites={s['written']:s for s in raw['sites'] if s['path']=='Tests/Declared.Tests.ps1'}
        self.assertEqual(sites['DeclaredHelper']['resolved'],'DeclaredHelper')
        self.assertEqual(sites['DeclaredPreamble']['resolved'],'DeclaredPreamble')
        self.assertIsNone(sites['UnexecutedHelper']['resolved'])
        self.assertEqual(sites['UnexecutedHelper']['unresolved_class'],'declared_not_loaded')
        self.assertEqual(entries['Executable.ps1']['site_count'],1)
        self.assertIsNone(sites['PrivateTarget']['resolved'])
        self.assertEqual(sites['PrivateTarget']['unresolved_class'],'private_module_scope')
        independent=[s for s in raw['sites'] if s['path']=='Tests/Independent.Tests.ps1' and s['written']=='UnexecutedHelper']
        self.assertEqual(len(independent),1)
        self.assertEqual(independent[0]['unresolved_class'],'same_file_or_BeforeAll')
        caps=[]
        total=sum((subject/p).stat().st_size for p in paths)
        for variable,limit,count in [('PWSH_ORACLE_HELPER_FILE_CAP',len(paths)-1,len(paths)),('PWSH_ORACLE_HELPER_BYTE_CAP',total-1,total)]:
            refused=out/(variable+'.json')
            capped=command.copy();capped[-1]=str(refused)
            env=environment(base);env[variable]=str(limit)
            result=subprocess.run(capped,env=env,capture_output=True,text=True)
            self.assertNotEqual(result.returncode,0)
            self.assertIn('whole-unit refusal: helper inventory',result.stderr)
            self.assertFalse(refused.exists())
            caps.append(dict(cap=variable,limit=limit,input_count=count,whole_unit_refused=True))
        self.receipt.update(caps=caps,native_declaration_discovery=True,all_declarations_partitioned=True,
            missing_helper_regression=True,executable_fixture_not_run=True,module_variable_private_ancestor=True,
            inventory_targets=len(inventory),loaded_targets=2,declared_not_loaded_targets=2)


def main():
    ap=argparse.ArgumentParser(description=__doc__)
    ap.add_argument('--tools',type=Path,required=True)
    ap.add_argument('--out',type=Path,required=True)
    args=ap.parse_args()
    ready()
    InventoryTests.here=Path(__file__).resolve().parent
    InventoryTests.base=args.tools.resolve()
    InventoryTests.out=args.out.resolve()
    InventoryTests.out.mkdir(parents=True,exist_ok=False)
    InventoryTests.receipt={}
    r=unittest.TextTestRunner(verbosity=2).run(unittest.defaultTestLoader.loadTestsFromTestCase(InventoryTests))
    if not r.wasSuccessful():raise SystemExit(1)
    InventoryTests.receipt['oracle_code_sha256']={p:hashlib.sha256((InventoryTests.here/p).read_bytes()).hexdigest() for p in ['DiscoverHarness.ps1','ResolveCalls.ps1','prove_inventory.py']}
    (InventoryTests.out/'inventory-proof.json').write_text(json.dumps(InventoryTests.receipt,indent=2,sort_keys=True)+'\n')


if __name__=='__main__':main()
