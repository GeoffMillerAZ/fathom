import tempfile,unittest
from pathlib import Path
import azure_oracle as oracle
class OracleTest(unittest.TestCase):
 def test_stages_jobs_and_literal_dependencies(self):
  with tempfile.TemporaryDirectory() as d:
   Path(d,'azure-pipelines.yml').write_text('stages:\n- stage: one\n  jobs:\n  - job: build\n- stage: two\n  dependsOn: one\n  jobs:\n  - deployment: ship\n    dependsOn: [build, absent]\n')
   r,bad=oracle.measure(Path(d))
   self.assertEqual(bad,0)
   self.assertEqual([(x['from'],x['to']) for x in r['references']],[('two','one'),('ship','build')])
   self.assertEqual(len(r['definitions']),5)
 def test_cyclic_yaml_alias_and_malformed_input(self):
  with tempfile.TemporaryDirectory() as d:
   Path(d,'azure-pipelines.yml').write_text('jobs: &loop\n- job: first\n  extra: *loop\n')
   r,bad=oracle.measure(Path(d));self.assertEqual(len(r['definitions']),2)
   Path(d,'azure-pipelines.yml').write_text('jobs: [ }')
   r,bad=oracle.measure(Path(d));self.assertEqual(bad,1);self.assertEqual(r['definitions'],[])
if __name__=='__main__':unittest.main()
