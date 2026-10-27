"""The native multiset fixes recall population before independent C positions."""
import copy
import unittest
from grade_native_l2 import project


class NativeProjectionTest(unittest.TestCase):
    def sample(self):
        native = dict(native_entities=[dict(kind='service', name='app'), dict(kind='service', name='unused')],
                      native_written_fields=[dict(from_kind='service', **{'from':'app'}, field='host',
                                                  to_kind='upstream', to='pool', outcome='owner_registry_resolved')])
        located = dict(tier='C', entities=[dict(kind='service', name='app', line=3, end=5),
                                          dict(kind='service', name='unused', line=6, end=7)])
        return native, located

    def test_native_unused_definition_and_binding_survive_projection(self):
        native, located = self.sample()
        definitions, refs, counts = project('kong.yml', native, located)
        self.assertEqual(['app','unused'], [row['name'] for row in definitions])
        self.assertEqual([dict(path='kong.yml', **{'from':'app','to':'pool'})], refs)
        self.assertEqual({'owner_registry_resolved':1}, counts)

    def test_locator_omission_extra_and_tier_change_refuse_whole_input(self):
        native, located = self.sample()
        for mutation in ('omitted', 'extra', 'tier'):
            changed = copy.deepcopy(located)
            if mutation == 'omitted': changed['entities'].pop()
            elif mutation == 'extra': changed['entities'].append(dict(kind='route',name='extra',line=8,end=8))
            else: changed['tier'] = 'A'
            with self.assertRaises(ValueError):
                project('kong.yml', native, changed)

    def test_unknown_native_outcome_refuses_instead_of_disappearing(self):
        native, located = self.sample()
        native['native_written_fields'][0]['outcome']='future_unclassified_binding'
        with self.assertRaisesRegex(ValueError,'outcome'):
            project('kong.yml', native, located)

    def test_unnamed_fields_and_external_hosts_remain_counted_unscored(self):
        native, located = self.sample()
        native['native_entities'].append(dict(kind='route'))
        native['native_written_fields'].append(dict(outcome='literal_host_without_named_upstream'))
        native['native_written_fields'].append(dict(outcome='owner_registry_resolved', **{'from':'','to':'pool'}))
        definitions, refs, counts = project('kong.yml', native, located)
        self.assertEqual(2,len(definitions));self.assertEqual(1,len(refs))
        self.assertEqual(1,counts['unnamed_endpoint_unscored'])
        self.assertEqual(1,counts['literal_host_without_named_upstream'])
