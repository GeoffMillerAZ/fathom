from dataclasses import replace
from pathlib import Path
import tempfile,unittest
from scope_limits import Limits,validate,merge_observations


class ScopeRefusalTests(unittest.TestCase):
    def test_each_source_cap_accepts_n_then_refuses_n_plus_one(self):
        root=Path(__file__).resolve().parents[3]/'workspace/tmp';root.mkdir(parents=True,exist_ok=True)
        with tempfile.TemporaryDirectory(dir=root) as d:
            root=Path(d)
            (root/'a.sh').write_bytes(b'12');(root/'b.sh').write_bytes(b'3')
            files=['a.sh','b.sh']
            validate(root,files,[1,2],[1,2],Limits(files=2,source_bytes=3,file_bytes=2,sites=2))
            for field,value in [('files',1),('source_bytes',2),('file_bytes',1),('sites',1)]:
                with self.subTest(field=field),self.assertRaisesRegex(ValueError,'whole-unit refusal'):
                    validate(root,files,[1,2],[1,2],replace(Limits(),**{field:value}))

    def test_cannot_raise_production_caps(self):
        with self.assertRaisesRegex(ValueError,'invalid source limits'):
            validate(Path('.'),[],limits=replace(Limits(),files=2049))


class NativeUnionTests(unittest.TestCase):
    def test_repeated_observations_do_not_expand_but_cap_plus_one_refuses(self):
        result=set()
        merge_observations(result,['a','b','a'],max_items=2)
        self.assertEqual(result,{'a','b'})
        with self.assertRaisesRegex(ValueError,'whole-unit refusal: combined native'):
            merge_observations(result,['c'],max_items=2)
        with self.assertRaisesRegex(ValueError,'invalid native union'):
            merge_observations(set(),[],max_items=1_000_001)
