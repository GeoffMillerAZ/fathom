"""Authored input-sensitivity/tier controls; independence is structural review."""
import argparse
import hashlib
import io
import json
from pathlib import Path
import subprocess
import tempfile
import unittest
from unittest.mock import patch
from test_prepare_d19 import PrepareBoundsTest

HERE=Path(__file__).resolve().parent
ORACLE=HERE/'json_schema_oracle.mjs'
TOOLS=None


class OracleControlTest(unittest.TestCase):
 def test_native_truth_is_input_sensitive_at_tier_c(self):
  with tempfile.TemporaryDirectory() as directory:
   root=Path(directory);subject=root/'subject';subject.mkdir()
   schema={'$schema':'https://json-schema.org/draft/2020-12/schema'}
   for name in ['first','second','zero']:
    (subject/(name+'.schema.json')).write_text(json.dumps(schema))
   entry=subject/'entry.schema.json'
   def input(target):
    entry.write_text(json.dumps(dict(schema,properties={'remote':{'allOf':[{'$ref':target+'.schema.json'},{'$ref':target+'.schema.json'}]}})))
   def native(number):
    out=root/str(number);out.mkdir()
    subprocess.run(['node','--max-old-space-size=1024',str(ORACLE),'-tools',str(TOOLS),'-subject',str(subject),'-out',str(out/'defs.json'),'-xref-out',str(out/'xrefs.json'),'-files-out',str(out/'files')],check=True,stdout=subprocess.DEVNULL,stderr=subprocess.DEVNULL,timeout=30)
    defs=json.loads((out/'defs.json').read_text());xref=json.loads((out/'xrefs.json').read_text())
    self.assertEqual('C',defs['tier']);self.assertEqual('C',xref['tier'])
    self.assertEqual(4,len((out/'files').read_text().splitlines()))
    self.assertEqual(1,len(xref['cross_references']))
    return (out/'defs.json').read_bytes(),(out/'xrefs.json').read_bytes(),xref
   input('first');before=native(0)
   input('second');changed=native(2)
   self.assertNotEqual(before[1],changed[1])
   self.assertEqual('first.schema.json',before[2]['cross_references'][0]['to_path'])
   self.assertEqual('second.schema.json',changed[2]['cross_references'][0]['to_path'])


def run(tools,out):
 global TOOLS
 TOOLS=tools.resolve();out.mkdir(parents=True,exist_ok=False)
 log=io.StringIO();runner=unittest.TextTestRunner(stream=log,verbosity=2)
 suite=unittest.TestSuite([unittest.defaultTestLoader.loadTestsFromTestCase(OracleControlTest),unittest.defaultTestLoader.loadTestsFromTestCase(PrepareBoundsTest)])
 green=runner.run(suite)
 if not green.wasSuccessful() or green.skipped:raise ValueError('oracle controls refused')
 mutations=[]
 source=ORACLE.read_text()
 for number in [0,1]:
  # Alter only the requested output family's authority, preserving all truth.
  pieces=source.split('tier: "C"');assert len(pieces)==3
  mutant=pieces[0]+('tier: "A"' if number==0 else 'tier: "C"')+pieces[1]+('tier: "A"' if number==1 else 'tier: "C"')+pieces[2]
  path=out/f'tier-mutant-{number}.mjs';path.write_text(mutant)
  with patch(__name__+'.ORACLE',path):
   red=runner.run(unittest.defaultTestLoader.loadTestsFromTestCase(OracleControlTest))
  restored=runner.run(unittest.defaultTestLoader.loadTestsFromTestCase(OracleControlTest))
  if red.wasSuccessful() or not red.failures or red.errors or not restored.wasSuccessful():raise ValueError('tier mutant not sensitive')
  mutations.append(dict(family='definitions' if number==0 else 'cross_references',mutant='C to A',restored_green=True))
 summary=dict(tests=green.testsRun,skips=len(green.skipped),mutations=mutations,independence_basis="structural_argv_and_source_review",output_perturbation_control=False,input_sensitive=True,zero_reference_inputs_retained=True,repeated_sites_use_existing_distinct_file_pair_unit=True,oracle_sha256=hashlib.sha256(ORACLE.read_bytes()).hexdigest())
 (out/'proof.log').write_text(log.getvalue());(out/'summary.json').write_text(json.dumps(summary,indent=2)+'\n')
 return summary


if __name__=='__main__':
 parser=argparse.ArgumentParser();parser.add_argument('-tools',type=Path,required=True);parser.add_argument('-out',type=Path,required=True);args=parser.parse_args()
 print(json.dumps(run(args.tools,args.out),sort_keys=True))
