from pathlib import Path
import tempfile,unittest
from scip_audit import audit,diagnostic_tier


def varint(n):
    out=bytearray()
    while n>127:out.append((n&127)|128);n>>=7
    return bytes(out)+bytes([n])


def field(n,value):return varint(n*8+2)+varint(len(value))+value


def document(name,symbol=b'scip-shell shell fixture 1 actual().'):
    occurrence=field(1,b'\x00\x00\x06')+field(2,symbol)
    return field(2,field(1,name.encode())+field(2,occurrence))


class ScipAuditTests(unittest.TestCase):
    def setUp(self):
        root=Path(__file__).resolve().parents[3]/'workspace/tmp';root.mkdir(parents=True,exist_ok=True)
        temp=tempfile.TemporaryDirectory(dir=root);self.addCleanup(temp.cleanup)
        self.path=Path(temp.name)/'index.scip'
        self.path.write_bytes(document('a.sh'))
        self.population={'sites':[{'path':'a.sh','line':1,'column':1,'literal':True},
                                  {'path':'b.sh','line':1,'column':1,'literal':True}]}

    def test_complete_population_keeps_unindexed_file_sites(self):
        result=audit(self.path,['a.sh','b.sh'],self.population)
        self.assertEqual(result['native_function_sites'],1)
        self.assertEqual(result['unindexed_file_command_sites'],1)
        self.assertEqual(result['unattested_command_sites'],1)

    def test_missing_native_caller_ranges_are_disclosed(self):
        definition=field(1,b'\x00\x00\x06')+field(2,b'scip-shell shell fixture 1 actual().')+varint(3*8)+varint(1)
        self.path.write_bytes(field(2,field(1,b'a.sh')+field(2,definition)))
        result=audit(self.path,['a.sh','b.sh'],self.population)
        self.assertEqual(result['function_definitions'],1)
        self.assertEqual(result['function_definitions_without_native_enclosing_range'],1)
        self.assertTrue(result['pair_diagnostic_tier'].startswith('C:'))
        self.assertEqual(diagnostic_tier(result),'C')
        self.assertEqual(result['native_command_sites'],0)

    def test_variable_is_not_a_bound_command(self):
        self.path.write_bytes(document('a.sh',b'local 0'))
        self.assertEqual(audit(self.path,['a.sh','b.sh'],self.population)['native_command_sites'],0)

    def test_caps_accept_n_then_refuse_n_plus_one(self):
        self.path.write_bytes(document('a.sh')+document('b.sh'))
        size=self.path.stat().st_size
        audit(self.path,['a.sh','b.sh'],self.population,max_bytes=size,max_documents=2,max_occurrences=2)
        for kwargs in [dict(max_bytes=size-1),dict(max_documents=1),dict(max_occurrences=1)]:
            with self.subTest(kwargs=kwargs),self.assertRaisesRegex(ValueError,'whole-unit refusal'):
                audit(self.path,['a.sh','b.sh'],self.population,**kwargs)

    def test_duplicate_or_outside_scope_document_refuses(self):
        for content in [document('a.sh')*2,document('outside.sh')]:
            self.path.write_bytes(content)
            with self.assertRaisesRegex(ValueError,'whole-unit refusal'):audit(self.path,['a.sh','b.sh'],self.population)
