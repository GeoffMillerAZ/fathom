#!/usr/bin/env python3
"""Native regressions for module scope, inert preambles and helper provenance."""
import argparse
import hashlib
import json
from pathlib import Path
import shutil
import subprocess
import unittest
from fetch import environment, ready


class FollowupTests(unittest.TestCase):
    def collect(self,label,extra):
        subject=self.out/label
        shutil.copytree(self.here/'fixtures',subject)
        for name,text in extra.items():
            path=subject/name;path.parent.mkdir(parents=True,exist_ok=True);path.write_text(text)
        paths=sorted(p.relative_to(subject).as_posix() for p in subject.rglob('*') if p.suffix in {'.ps1','.psm1','.psd1'})
        files=self.out/(label+'-files.json');files.write_text(json.dumps(paths))
        output=self.out/(label+'-native.json')
        result=subprocess.run([str(self.base/'runtime/pwsh'),'-NoLogo','-NoProfile','-NonInteractive',
            '-File',str(self.here/'ResolveCalls.ps1'),'-Subject',str(subject),'-FilesJson',str(files),
            '-Repository',str(subject),'-TrackedJson',str(files),'-Harness','pester',
            '-Manifest',str(subject/'module/Probe.psd1'),'-Out',str(output)],env=environment(self.base),capture_output=True,text=True)
        self.assertEqual(result.returncode,0,result.stderr)
        return subject,json.loads(output.read_text())

    def test_parenthesized_module_scope(self):
        _,raw=self.collect('scope',{'Tests/Parenthesized.Tests.ps1':
            '& (Get-Module Probe) { PrivateTarget }\n. (Get-Module Probe) { PrivateTarget }\n& $handle { PrivateTarget }\n'})
        sites=[s for s in raw['sites'] if s['path']=='Tests/Parenthesized.Tests.ps1' and s['written']=='PrivateTarget']
        self.assertEqual(len(sites),3)
        self.assertTrue(all(s['resolved'] is None and s['unresolved_class']=='private_module_scope' for s in sites))

    def test_definition_preamble_never_executes_transitive_support(self):
        subject,raw=self.collect('guard',{
            'helpers/Preamble.ps1': 'if (-not (Get-Variable ProbeBuild -ErrorAction Ignore)) { . "$PSScriptRoot/Support.ps1" }\nfunction DeclaredPreamble { 1 }\n',
            'helpers/Support.ps1': '. "$PSScriptRoot/DeepSupport.ps1"\n',
            'helpers/DeepSupport.ps1': 'Set-Content "$PSScriptRoot/../executed.txt" "unexpected"\nfunction NestedOnly { 1 }\n',
            'Tests/Guard.Tests.ps1': '. "$PSScriptRoot/../helpers/Preamble.ps1"\nDeclaredPreamble\nNestedOnly\n'})
        self.assertFalse((subject/'executed.txt').exists(),'transitive executable support ran in collector scope')
        sites={s['written']:s for s in raw['sites'] if s['path']=='Tests/Guard.Tests.ps1'}
        self.assertEqual(sites['DeclaredPreamble']['resolved'],'DeclaredPreamble')
        self.assertIsNone(sites['NestedOnly']['resolved'])
        expected=hashlib.sha256((subject/'helpers/Preamble.ps1').read_bytes()).hexdigest()
        self.assertEqual(sites['DeclaredPreamble']['resolved_definition'][1],expected)

    def test_all_helper_crossings_and_shadowed_origins(self):
        _,raw=self.collect('origins',{
            'helpers/A/Same.psm1': 'function SharedTarget { "first" }\n',
            'helpers/Z/Same.psm1': 'function SharedTarget { "last" }\n',
            'Tests/A.Tests.ps1': 'Import-Module "$PSScriptRoot/../helpers/A/Same.psm1"\nSharedTarget\n',
            'Tests/Z.Tests.ps1': 'Import-Module "$PSScriptRoot/../helpers/Z/Same.psm1"\nSharedTarget\n',
            'Tests/Bridge.Tests.ps1': 'Import-Module "$PSScriptRoot/../helpers/Declared.psm1"\n',
            'Tests/ZBridge.Tests.ps1': '. "$PSScriptRoot/Bridge.Tests.ps1"\n',
            'Tests/Through.Tests.ps1': '. "$PSScriptRoot/ZBridge.Tests.ps1"\nDeclaredHelper\n',
            'Tests/Undeclared.Tests.ps1': 'DeclaredHelper\n'})
        bykey={(s['path'],s['written']):s for s in raw['sites']}
        outside=bykey['Tests/Undeclared.Tests.ps1','DeclaredHelper']
        self.assertEqual(outside['resolved'],'DeclaredHelper')
        self.assertTrue(outside.get('session_union_nonlocal_helper',False),'test-helper crossing was not counted')
        self.assertFalse(bykey['Tests/Through.Tests.ps1','DeclaredHelper']['session_union_nonlocal_helper'])
        shadowed=bykey['Tests/A.Tests.ps1','SharedTarget']
        self.assertTrue(shadowed['session_union_nonlocal_helper'])
        self.assertTrue(shadowed['shadowed_declared_helper_ids'])
        self.assertFalse(bykey['Tests/Z.Tests.ps1','SharedTarget']['session_union_nonlocal_helper'])
        receipt=raw['receipt']
        self.assertEqual(receipt['session_union_nonlocal_helper_sites'],sum(s['session_union_nonlocal_helper'] for s in raw['sites']))
        self.assertEqual(receipt['session_union_shadowed_declared_helper_sites'],1)

    def test_declared_suite_runner_globals_are_explicit(self):
        _,raw=self.collect('runner',{
            'test.ps1': 'Import-Module "$PSScriptRoot/helpers/Declared.psm1"\n',
            'tst/Pester.BeforeContainer.ps1': '. "$PSScriptRoot/../helpers/Preamble.ps1"\n',
            'Tests/Runner.Tests.ps1': 'DeclaredHelper\nDeclaredPreamble\n'})
        sites=[s for s in raw['sites'] if s['path']=='Tests/Runner.Tests.ps1']
        self.assertEqual(len(sites),2)
        self.assertTrue(all(s['resolved'] and not s['session_union_nonlocal_helper'] for s in sites))
        self.assertEqual(raw['receipt']['runner_global_declaring_files']['count'],2)


def main():
    p=argparse.ArgumentParser(description=__doc__)
    p.add_argument('--tools',type=Path,required=True);p.add_argument('--out',type=Path,required=True)
    a=p.parse_args();ready()
    FollowupTests.here=Path(__file__).resolve().parent;FollowupTests.base=a.tools.resolve()
    FollowupTests.out=a.out.resolve();FollowupTests.out.mkdir(parents=True,exist_ok=False)
    result=unittest.TextTestRunner(verbosity=2).run(unittest.defaultTestLoader.loadTestsFromTestCase(FollowupTests))
    if not result.wasSuccessful():raise SystemExit(1)
    receipt=dict(native_tests=result.testsRun,passed=True,oracle_code_sha256={name:hashlib.sha256((FollowupTests.here/name).read_bytes()).hexdigest() for name in ['DiscoverHarness.ps1','ResolveCalls.ps1','prove_followups.py']})
    (FollowupTests.out/'followup-proof.json').write_text(json.dumps(receipt,indent=2,sort_keys=True)+'\n')


if __name__=='__main__':main()
