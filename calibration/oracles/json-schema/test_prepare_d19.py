import tempfile
from pathlib import Path
import unittest
from prepare_d19 import check_inputs,inside_workspace

class PrepareBoundsTest(unittest.TestCase):
 def test_population_caps_refuse_whole_scope_and_symlinks_are_not_followed(self):
  with tempfile.TemporaryDirectory() as directory:
   root=Path(directory);scope=root/'schemas';scope.mkdir();path=scope/'authored.json';path.write_bytes(b'{}')
   limits=dict(source_file_bytes=2,files_per_subject=1,source_bytes_per_subject=2)
   self.assertEqual({'files':1,'bytes':2},check_inputs(root,['schemas'],limits))
   (scope/'link.json').symlink_to(path)
   self.assertEqual(1,check_inputs(root,['schemas'],limits)['files'])
   for key in limits:
    changed=dict(limits);changed[key]-=1
    with self.assertRaises(ValueError):check_inputs(root,['schemas'],changed)
 def test_output_is_private_workspace_only(self):
  with self.assertRaises(ValueError):inside_workspace(Path('/tmp/not-the-repo-workspace'))
