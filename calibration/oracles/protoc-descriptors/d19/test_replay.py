from pathlib import Path
import tempfile
import unittest
from replay import bounded_json,native_value


class ReplayTest(unittest.TestCase):
 def test_artifact_cap_plus_one_and_mutant_authority_refuse_whole_unit(self):
  with tempfile.TemporaryDirectory() as directory:
   path=Path(directory)/'authored.json';path.write_bytes(b'{}')
   self.assertEqual({},bounded_json(path,2));path.write_bytes(b'{}\n')
   with self.assertRaises(ValueError):bounded_json(path,2)
   path.write_text('{"tool":"protoc","version":"34.1","tier":"A"}')
   self.assertEqual('A',native_value(path,1024)['tier'])
   path.write_text('{"tool":"protoc","version":"34.1","tier":"C"}')
   with self.assertRaises(ValueError):native_value(path,1024)

 def test_timeout_kills_sleeping_process_group_and_discards_partial_artifact(self):
  import os,signal,subprocess,sys,time
  from unittest.mock import patch
  import replay
  with tempfile.TemporaryDirectory() as directory:
   root=Path(directory);target=root/'timeout';marker=root/'child-survived'
   child="import time;from pathlib import Path;time.sleep(2);Path("+repr(str(marker))+").write_bytes(b'alive')"
   parent="import subprocess,sys,time;subprocess.Popen([sys.executable,'-c',"+repr(child)+"]);print(''+"+repr('{"tool":"protoc","version":"34.1","tier":"A"}')+",flush=True);time.sleep(60)"
   original_popen=subprocess.Popen;original_killpg=os.killpg;processes=[]
   def start(*args,**kwargs):
    value=original_popen(*args,**kwargs);processes.append(value);return value
   with patch.object(replay.subprocess,'Popen',side_effect=start),patch.object(replay.os,'killpg',wraps=original_killpg) as killed:
    with self.assertRaises(subprocess.TimeoutExpired):replay.run([sys.executable,'-B','-c',parent],target,1,4096)
   killed.assert_called_once_with(processes[0].pid,signal.SIGKILL)
   self.assertEqual(-signal.SIGKILL,processes[0].returncode)
   time.sleep(1.3)
   self.assertFalse(marker.exists(),'sleeping child survived process-group termination')
   self.assertFalse(target.with_suffix('.json').exists(),'valid-looking partial output must not be admitted')
