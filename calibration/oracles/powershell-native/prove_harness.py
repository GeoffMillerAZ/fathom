#!/usr/bin/env python3
"""Verify the actual pinned subjects' declared harness is in binder load state."""
import argparse
import json
from pathlib import Path
import subprocess
import sys
import unittest


class DeclaredHarnessTests(unittest.TestCase):
    def test_declared_dependencies_and_complete_breakdown(self):
        summary=json.loads((self.out/'summary.json').read_text())
        for name,required in [('pester',{'Pester','Axiom','p','PTestHelpers','TestHelpers','Pester.Runtime.Wrapper'}),('psa',{'Pester','PSScriptAnalyzer','PSScriptAnalyzerTestHelper'})]:
            row=summary['subjects'][name]
            raw=json.loads((self.out/name/'native-answers.json').read_text())
            self.assertTrue('harness_inventory' in raw,'native parser declaration inventory missing')
            declared=raw['harness_inventory']
            ids=[d['id'] for d in declared]
            self.assertEqual(len(ids),len(set(ids)))
            self.assertTrue(declared)
            loaded={d['id'] for d in declared if d['status']=='loaded'}
            refused={d['id'] for d in declared if d['status']=='declared_not_loaded'}
            self.assertEqual(set(ids),loaded|refused)
            self.assertFalse(loaded&refused)
            self.assertTrue(all(d['reason'] for d in declared))
            self.assertTrue(all(d['code_sha256'] for d in declared if d['status']=='loaded'))
            for d in declared:
                if not Path(d['target'].split('#')[0]).is_file():
                    self.assertIsNone(d['code_sha256'],'absent target must not carry a fabricated code hash')
            if name=='psa':
                self.assertTrue(any(d['target'].endswith('/build.psm1') and d['status']=='loaded' for d in declared))
            else:
                for suffix in ['/src/Format.ps1','/src/TypeClass.ps1']:
                    self.assertTrue(any(d['target'].endswith(suffix) and d['status']=='loaded' for d in declared))
            native=row['native']
            self.assertEqual(loaded,{d['declaration_id'] for d in native['declared_harness_inputs'] if 'declaration_id' in d})
            self.assertEqual(refused,{d['declaration_id'] for d in native['declared_inventory']['declared_not_loaded']})
            self.assertEqual(native['declared_inventory']['targets'],len(ids))
            self.assertEqual(native['declared_inventory']['declarations'],sum(d['declaration_count'] for d in declared))
            self.assertEqual(native['session_union_nonlocal_private_helper_sites'],sum(s['session_union_nonlocal_private_helper'] for s in raw['sites']))
            self.assertTrue(required.issubset({m['name'] for m in native['load_after']}))
            self.assertEqual({m['name'] for m in native['load_before']},set(native['explicit_runtime_modules']))
            self.assertEqual(sum(native['unresolved_by_class'].values()),row['unresolved_sites'])
            self.assertTrue(all(m['code_sha256'] for m in native['declared_harness_inputs']))
            self.assertGreater(native['resolved_harness_sites'],0)
            self.assertIn('found_not_admitted',native['unresolved_by_class'])
            if name=='psa':
                self.assertIn('5.7.1',{m['version'] for m in native['load_after'] if m['name']=='Pester'})


def main():
    ap=argparse.ArgumentParser(description=__doc__)
    ap.add_argument('--tools',type=Path,required=True)
    ap.add_argument('--indexbench',type=Path,required=True)
    ap.add_argument('--out',type=Path,required=True)
    args=ap.parse_args()
    here=Path(__file__).resolve().parent
    subprocess.run([sys.executable,'-B',str(here/'measure_binder.py'),'--tools',str(args.tools),
                    '--indexbench',str(args.indexbench),'--out',str(args.out)],check=True)
    DeclaredHarnessTests.out=args.out
    result=unittest.TextTestRunner(verbosity=2).run(unittest.defaultTestLoader.loadTestsFromTestCase(DeclaredHarnessTests))
    if not result.wasSuccessful():raise SystemExit(1)
    (args.out/'harness-proof.json').write_text(json.dumps({'parser_inventory_partition_verified':True,'every_loaded_declaration_has_hashed_input':True,'every_refused_declaration_named_with_reason':True,
        'explicit_runtime_baseline_asserted':True,'unresolved_breakdown_complete':True})+'\n')


if __name__=='__main__':main()
