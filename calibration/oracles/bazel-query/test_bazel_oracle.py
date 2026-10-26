import contextlib,importlib.util,io,subprocess,sys,tempfile,unittest
from pathlib import Path
from unittest.mock import patch
spec=importlib.util.spec_from_file_location('bazel_oracle',Path(__file__).with_name('bazel_oracle.py'));o=importlib.util.module_from_spec(spec);spec.loader.exec_module(o)
class NativeQueryFailureTest(unittest.TestCase):
 def test_failed_query_never_writes_tier_a_truth(self):
  with tempfile.TemporaryDirectory() as d,patch.object(sys,'argv',['oracle','-subject',d,'-output-user-root',d]),patch.object(o.subprocess,'run',return_value=subprocess.CompletedProcess([],1,stdout='',stderr='ERROR: partial query')):
   output=io.StringIO()
   with contextlib.redirect_stdout(output),contextlib.redirect_stderr(io.StringIO()),self.assertRaises(SystemExit):o.main()
   self.assertEqual(output.getvalue(),'')
 def test_failed_version_never_writes_unidentified_truth(self):
  results=[subprocess.CompletedProcess([],0,stdout='',stderr=''),subprocess.CompletedProcess([],1,stdout='',stderr='version unavailable')]
  with tempfile.TemporaryDirectory() as d,patch.object(sys,'argv',['oracle','-subject',d,'-output-user-root',d]),patch.object(o.subprocess,'run',side_effect=results):
   output=io.StringIO()
   with contextlib.redirect_stdout(output),contextlib.redirect_stderr(io.StringIO()),self.assertRaises(SystemExit):o.main()
   self.assertEqual(output.getvalue(),'')
if __name__=='__main__':unittest.main()
