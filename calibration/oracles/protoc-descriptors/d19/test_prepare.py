from pathlib import Path
import io
import tempfile
import unittest
from unittest.mock import patch
import zipfile
from prepare import check_sources,download,unpack,verify_sdk


class PrepareTest(unittest.TestCase):
 def test_source_caps_plus_one_refuse_whole_population(self):
  with tempfile.TemporaryDirectory() as directory:
   root=Path(directory);(root/'authored.proto').write_bytes(b'xy')
   limits=dict(source_file_bytes=2,files_per_subject=1,source_bytes_per_subject=2)
   self.assertEqual(dict(proto_files=1,proto_bytes=2),check_sources(root,limits))
   for key in limits:
    changed=dict(limits);changed[key]-=1
    with self.assertRaises(ValueError):check_sources(root,changed)

 def test_archive_caps_plus_one_and_dotdot_refuse_before_extraction(self):
  with tempfile.TemporaryDirectory() as directory:
   root=Path(directory);archive=root/'authored.zip'
   with zipfile.ZipFile(archive,'w') as target:target.writestr('value',b'xy')
   limits=dict(archive_members=1,archive_expanded_bytes=2,archive_member_bytes=2)
   for key in limits:
    changed=dict(limits);changed[key]-=1
    with self.assertRaises(ValueError):unpack(archive,root/'out',changed)
    self.assertFalse((root/'out').exists())
   with zipfile.ZipFile(archive,'w') as target:target.writestr('../outside',b'xy')
   with self.assertRaises(ValueError):unpack(archive,root/'out',limits)
   self.assertFalse((root/'out').exists())

 def test_download_cap_plus_one_leaves_no_partial_archive(self):
  with tempfile.TemporaryDirectory() as directory:
   path=Path(directory)/'archive'
   with patch('prepare.urllib.request.urlopen',return_value=io.BytesIO(b'xy')):download('authored',path,2)
   self.assertEqual(b'xy',path.read_bytes());path.unlink()
   with patch('prepare.urllib.request.urlopen',return_value=io.BytesIO(b'xyz')):
    with self.assertRaises(ValueError):download('authored',path,2)
   self.assertFalse(path.exists())

 def test_cached_payload_bytes_and_symlink_are_reverified(self):
  with tempfile.TemporaryDirectory() as directory:
   root=Path(directory);archive=root/'authored.zip';target=root/'out'
   with zipfile.ZipFile(archive,'w') as source:source.writestr('value',b'xy')
   unpack(archive,target,dict(archive_members=1,archive_expanded_bytes=2,archive_member_bytes=2))
   self.assertEqual(dict(members=1,expanded_bytes=2),verify_sdk(archive,target))
   (target/'value').write_bytes(b'zz')
   with self.assertRaises(ValueError):verify_sdk(archive,target)
   (target/'value').unlink();(target/'value').symlink_to(archive)
   with self.assertRaises(ValueError):verify_sdk(archive,target)
