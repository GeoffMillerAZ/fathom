"""Controls for native-model receipt scope and privacy."""
import unittest
import tempfile
import hashlib
from pathlib import Path
import native_model_probe as probe

class NativeModelTest(unittest.TestCase):
    def raw(self):
        return dict(core_version='authored',tool_declarations=0,files=[dict(subject='authored',status='parsed',sha256='0'*64,
                    nodes=[dict(type='ModelASTStage',name='private example',line=2,end=4,parent=-1)])])
    def test_summary_has_counts_not_subject_names(self):
        result=probe.summarize(self.raw())
        self.assertEqual(1,result['inputs'])
        self.assertEqual(1,result['positioned_nodes'])
        self.assertEqual({'ModelASTStage':1},result['model_classes'])
        self.assertNotIn('private example',str(result))
        self.assertFalse(result['calibration_measured'])
    def test_refusals_remain_in_population(self):
        value=self.raw();value['files'].append(dict(subject='authored',status='refused',sha256='1'*64,nodes=[]))
        result=probe.summarize(value)
        self.assertEqual(2,result['inputs']);self.assertEqual(1,result['refused'])
    def test_bad_position_refuses_whole_receipt(self):
        value=self.raw();value['files'][0]['nodes'][0]['end']=1
        with self.assertRaises(ValueError):probe.summarize(value)
    def test_input_cap_plus_one_refuses(self):
        value=self.raw();value['files']*=129
        with self.assertRaises(ValueError):probe.summarize(value)
    def test_unknown_native_field_is_not_published(self):
        value=self.raw();value['files'][0]['identity']='private example'
        with self.assertRaises(ValueError):probe.summarize(value)
    def test_nested_owner_class_is_admitted(self):
        value=self.raw();value['files'][0]['nodes'][0]['type']='ModelASTValue$ConstantValue'
        self.assertEqual(1,probe.summarize(value)['model_classes']['ModelASTValue$ConstantValue'])
    def test_node_cap_plus_one_refuses(self):
        value=self.raw();value['files'][0]['nodes']*=100001
        with self.assertRaises(ValueError):probe.summarize(value)
    def test_uncontrolled_subject_alias_refuses(self):
        value=self.raw();value['files'][0]['subject']='private example'
        with self.assertRaises(ValueError):probe.summarize(value)
    def test_unknown_header_refuses(self):
        value=self.raw();value['personal_metadata']='private example'
        with self.assertRaises(ValueError):probe.summarize(value)
    def test_native_bundle_rejects_changed_or_extra_plugins(self):
        from native_model_probe import verify_bundle
        with tempfile.TemporaryDirectory() as directory:
            root=Path(directory);java=root/'java';war=root/'war';plugins=root/'plugins';plugins.mkdir()
            java.write_bytes(b'authored jdk');war.write_bytes(b'authored core');plugin=plugins/'sample.jpi';plugin.write_bytes(b'authored plugin')
            def digest(path):return hashlib.sha256(path.read_bytes()).hexdigest()
            bundle=dict(java_binary_sha256=digest(java),sdk_downloads=[dict(name='jenkins.war',sha256=digest(war))],plugins=[dict(id='sample',sha256=digest(plugin))])
            self.assertEqual(1,verify_bundle(java,war,plugins,bundle))
            plugin.write_bytes(b'drift')
            with self.assertRaises(ValueError):verify_bundle(java,war,plugins,bundle)
            plugin.write_bytes(b'authored plugin');(plugins/'extra.jpi').write_bytes(b'extra')
            with self.assertRaises(ValueError):verify_bundle(java,war,plugins,bundle)
