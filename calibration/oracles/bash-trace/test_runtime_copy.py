from pathlib import Path
import tempfile
import unittest
from runtime_copy import copy_subject


class RuntimeCopyTests(unittest.TestCase):
    def setUp(self):
        parent=Path(__file__).resolve().parents[3]/'workspace/tmp'
        parent.mkdir(parents=True,exist_ok=True)
        tmp=tempfile.TemporaryDirectory(dir=parent)
        self.addCleanup(tmp.cleanup)
        self.base=Path(tmp.name)
        self.source=self.base/'original'
        self.source.mkdir()
        (self.source/'target').write_text('original')
        (self.source/'link').symlink_to('target')
        (self.source/'.git').mkdir()
        (self.source/'.git/HEAD').write_text('ref: refs/heads/fixture\n')

    def test_symlink_identity_and_copy_isolation(self):
        child=self.base/'copy'
        copy_subject(self.source,child)
        self.assertTrue((child/'link').is_symlink())
        self.assertEqual((child/'link').readlink(),Path('target'))
        (child/'target').write_text('child')
        self.assertEqual((self.source/'target').read_text(),'original')

    def test_git_metadata_is_copied_without_shared_worktree_reference(self):
        child=self.base/'copy'
        copy_subject(self.source,child)
        self.assertTrue((child/'.git/HEAD').is_file(),'copied Git metadata is required')
        self.assertEqual((child/'.git/HEAD').read_text(),'ref: refs/heads/fixture\n')
        (child/'.git/HEAD').write_text('detached')
        self.assertEqual((self.source/'.git/HEAD').read_text(),'ref: refs/heads/fixture\n')
        (self.source/'.git/HEAD').unlink()
        (self.source/'.git').rmdir()
        (self.source/'.git').write_text('gitdir: outside\n')
        with self.assertRaisesRegex(ValueError,'whole-unit refusal'):
            copy_subject(self.source,self.base/'unsafe')

    def test_driver_scratch_is_inside_allowed_cache(self):
        from runtime_copy import runtime_environment
        inherited={'TMPDIR':'/outside','XDG_CONFIG_HOME':'/outside','BATS_TMPDIR':'/outside','KEEP':'value','HOME':'/outside','PROFILE':'/outside','npm_config_cache':'/outside'}
        cache=self.base/'cache'
        result=runtime_environment(cache,inherited)
        self.assertEqual(result['TMPDIR'],str(cache/'tmp'))
        self.assertEqual(result['TMP'],result['TMPDIR'])
        self.assertNotIn('BATS_TMPDIR',result)
        self.assertNotIn('PROFILE',result)
        for key in ['HOME','npm_config_cache','TMPDIR','TMP','TEMP','XDG_CACHE_HOME','XDG_CONFIG_HOME','XDG_DATA_HOME','XDG_STATE_HOME']:
            self.assertTrue(Path(result[key]).is_dir())
            self.assertTrue(Path(result[key]).is_relative_to(cache))
        self.assertEqual(result['KEEP'],'value')
        self.assertEqual(inherited['TMPDIR'],'/outside')
