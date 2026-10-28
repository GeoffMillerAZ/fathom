import tempfile
from pathlib import Path
import unittest
from measure_d19 import bounded_json


class ArtifactBoundsTest(unittest.TestCase):
 def test_artifact_cap_plus_one_refuses_whole_json(self):
  with tempfile.TemporaryDirectory() as directory:
   path=Path(directory)/'authored.json';path.write_bytes(b'{}')
   self.assertEqual({},bounded_json(path,2))
   path.write_bytes(b'{}\n')
   with self.assertRaises(ValueError):bounded_json(path,2)
