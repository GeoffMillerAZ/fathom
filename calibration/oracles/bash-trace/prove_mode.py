#!/usr/bin/env python3
"""Native function-only collection retains original source scope as unmeasured."""
import argparse,json,os
from pathlib import Path
import subprocess,sys,unittest
from build_bash import ready
from trace_driver import events


class ModeProof(unittest.TestCase):
    def test_disabled_source_probe_is_unmeasured_with_population_retained(self):
        subject=self.out/'subject';(subject/'test').mkdir(parents=True)
        (subject/'defs.sh').write_text('target() { :; }\ndriver() { target; }\n')
        probe=subject/'test/probe'
        probe.write_text('#!/usr/bin/env bash\n. ../defs.sh\ndriver\n')
        probe.chmod(0o755)
        subprocess.run(['git','init','--quiet',str(subject)],check=True)
        subprocess.run(['git','-C',str(subject),'add','defs.sh','test/probe'],check=True)
        native=self.out/'native'
        result=subprocess.run([sys.executable,'-B',str(self.here/'collect.py'),'--repo',str(self.here.parents[2]),
            '--subject',str(subject),'--shfmt',str(self.tools/'shfmt'),'--bash',str(self.tools/'bash-5.3/bash'),
            '--fullpath','--driver','nvm','--functions-only','--timeout','10','--out',str(native),'--cache',str(self.out/'cache')],capture_output=True,text=True)
        self.assertEqual(result.returncode,0,result.stderr)
        row=json.loads((native/'summary.json').read_text())
        self.assertGreater(row['denominator'],0)
        self.assertFalse(row['source_probe_enabled'])
        self.assertIsNone(row['coverage'])
        self.assertEqual(row['imports'],[])
        self.assertEqual({kind for kind,_ in events(native/'native.trace')},{'VERSION'})
        self.assertTrue(row['function_pairs'])
        self.receipt.update(source_population_retained=True,disabled_source_probe_unmeasured=True,
            no_debug_source_events=True,actual_function_pairs_observed=True)


def main():
    p=argparse.ArgumentParser(description=__doc__)
    for name in ['tools','out']:p.add_argument('--'+name,type=Path,required=True)
    a=p.parse_args();ready()
    ModeProof.here=Path(__file__).resolve().parent;ModeProof.tools=a.tools.resolve();ModeProof.out=a.out.resolve()
    ModeProof.out.mkdir(parents=True,exist_ok=False);ModeProof.receipt={}
    r=unittest.TextTestRunner(verbosity=2).run(unittest.defaultTestLoader.loadTestsFromTestCase(ModeProof))
    if not r.wasSuccessful():raise SystemExit(1)
    (ModeProof.out/'mode-proof.json').write_text(json.dumps(ModeProof.receipt,sort_keys=True)+'\n')


if __name__=='__main__':main()
