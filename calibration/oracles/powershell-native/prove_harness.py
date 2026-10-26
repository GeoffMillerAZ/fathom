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
        for name,required in [('pester',{'Pester','Axiom','p','TestHelpers'}),('psa',{'Pester','PSScriptAnalyzer'})]:
            row=summary['subjects'][name]
            native=row['native']
            self.assertTrue(required.issubset({m['name'] for m in native['load_after']}))
            self.assertEqual({m['name'] for m in native['load_before']},set(native['explicit_runtime_modules']))
            self.assertEqual(sum(native['unresolved_by_class'].values()),row['unresolved_sites'])
            self.assertTrue(all(m['code_sha256'] for m in native['declared_harness_inputs']))
            self.assertGreater(native['resolved_harness_sites'],0)
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
    (args.out/'harness-proof.json').write_text(json.dumps({'declared_harness_loaded':True,
        'explicit_runtime_baseline_asserted':True,'unresolved_breakdown_complete':True})+'\n')


if __name__=='__main__':main()
