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
