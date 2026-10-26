"""Full native population must stay distinct from product-selected comparison."""
import copy
import json
import unittest
from pathlib import Path
import grade_native_l2 as grade


class NativePopulationTest(unittest.TestCase):
    def test_site_and_pair_counts_keep_unowned_fields_and_empty_host_family(self):
        route=dict(from_kind='route',field='service',to_kind='service',outcome='owner_registry_resolved',**{'from':'route','to':'app'})
        plugin=dict(from_kind='plugin',field='route',to_kind='route',outcome='owner_registry_resolved',**{'from':'cors','to':'route'})
        native=dict(native_written_fields=[route,plugin,copy.deepcopy(plugin)])
        rows=grade.reference_family_counts('state.yaml',native,False)
        self.assertEqual(2,rows['plugin.route']['resolved_sites'])
        self.assertEqual(1,rows['plugin.route']['semantic_pairs'])
        self.assertEqual(1,rows['route.service']['semantic_pairs'])
        self.assertEqual(0,rows['plugin.route']['name_owned_pairs'])
        self.assertEqual(0,rows['service.host']['semantic_pairs'])

    def test_product_selected_subset_never_grants_a_tier_or_support_grade(self):
        measured=dict(counts={'ref_truth':4,'ref_truth_found':4},metrics={'ref_recall':1},
                      metric_tiers={'ref_recall':'A'},grade={'L2':'excellent'},subjects=[])
        public=grade.public_subset_evidence(measured)
        self.assertFalse(public['l2_tier_a_recall_granted'])
        self.assertFalse(public['support_grade_granted'])
        self.assertNotIn('metric_tiers',public)
        self.assertNotIn('grade',public)
        self.assertEqual(4,public['name_owned_subset_counts']['ref_truth'])

    def test_controls_bind_the_race_test_source_not_only_the_binary(self):
        here=Path(grade.HERE)
        controls=json.loads((here/'native-authored-controls-r1.json').read_text())
        controls['native_test_source_sha256']=grade.sha((here/'native/main_test.go').read_bytes())
        controls['control_script_sha256']=grade.sha((here/'probe_native_controls.py').read_bytes())
        identity=json.loads((here/'native-build-r1.json').read_text())
        grade.validate_control_identity(identity,controls)
        controls['native_test_source_sha256']='0'*64
        with self.assertRaisesRegex(ValueError,'test source'):
            grade.validate_control_identity(identity,controls)
