"""Independent authored units; no extractor output defines owner truth."""
import copy
import unittest
import owner_compare as compare

class OwnerCompareTest(unittest.TestCase):
    def raw(self):
        def n(t,name,line,end,parent):
            return dict(type='ModelAST'+t,name=name,line=line,end=end,parent=parent)
        return dict(core_version='authored',tool_declarations=0,files=[dict(subject='authored',sha256='0'*64,status='parsed',nodes=[
            n('PipelineDef',None,1,12,-1),n('Stages',None,3,11,0),n('Stage','Build',4,10,1),
            n('Branch','default',5,9,2),n('Step','echo',6,6,3),n('ScriptBlock','script',7,8,3),
            n('Value$ConstantValue',None,7,8,5),n('Key','argument',6,6,4)])])
    def test_written_unit_ranges_and_ownership(self):
        p=compare.project(self.raw())
        self.assertEqual([('pipeline','job',1,12),('stages','section',3,11),('Build','stage',4,10)],p['definitions'])
        self.assertEqual([],p['pairs']) # Default branch does not certify a written steps declaration.
        self.assertEqual(1,p['disclosures']['written_steps_unmapped'])
        self.assertEqual(1,p['disclosures']['opaque_bodies'])
        self.assertEqual(8,sum(p['accounting'].values()))
        self.assertFalse(p['complete_l1']);self.assertFalse(p['complete_l2'])
    def test_only_direct_owner_call_is_a_pair(self):
        raw=self.raw();raw['files'][0]['nodes'][4]['parent']=2
        p=compare.project(raw)
        self.assertEqual([('Build','echo')],p['pairs'])
    def test_range_and_owner_changes_are_detected(self):
        raw=self.raw();p=compare.project(raw)
        changed=copy.deepcopy(raw);changed['files'][0]['nodes'][2]['end']=11
        self.assertNotEqual(p['definitions'],compare.project(changed)['definitions'])
        changed=copy.deepcopy(raw);changed['files'][0]['nodes'][1]['parent']=2
        with self.assertRaises(ValueError):compare.project(changed)
    def test_refusal_is_incomplete_not_empty_success(self):
        raw=self.raw();raw['files'][0].update(status='refused',nodes=[])
        p=compare.project(raw)
        self.assertEqual(1,p['disclosures']['refused_files'])
        self.assertFalse(p['complete_l1']);self.assertFalse(p['complete_l2'])
    def test_exact_multiset_comparison_not_names_only(self):
        truth=[('Build','stage',4,10),('Build','stage',4,10)]
        self.assertEqual(dict(truth=2,candidate=1,matched=1),compare.match(truth,[truth[0]]))
        self.assertEqual(0,compare.match(truth,[('Build','stage',4,11)])['matched'])
    def test_caps_refuse_whole_unit(self):
        raw=self.raw();raw['files']*=129
        with self.assertRaises(ValueError):compare.project(raw)
        raw=self.raw();raw['files'][0]['nodes']*=12501
        with self.assertRaises(ValueError):compare.project(raw)
    def test_candidate_mutation_cannot_change_truth(self):
        raw=self.raw();before=compare.project(raw)
        compare.match(before['definitions'],[])
        self.assertEqual(before,compare.project(raw))
        raw['files'][0]['nodes'][2]['name']='Changed'
        self.assertNotEqual(before['definitions'],compare.project(raw)['definitions'])
    def test_opaque_and_unknown_classes_cannot_grant_complete_coverage(self):
        raw=self.raw();raw['files'][0]['nodes']=[raw['files'][0]['nodes'][0],dict(type='ModelASTFuture',name=None,line=2,end=3,parent=0)]
        self.assertFalse(compare.project(raw)['complete_l1'])

    def test_declaration_keywords_are_not_reference_units(self):
        raw=self.raw();raw['files'][0]['nodes'][4].update(parent=2,name='environment')
        p=compare.project(raw)
        self.assertEqual([],p['pairs'])
        self.assertEqual(1,p['disclosures']['declaration_keyword_calls'])
