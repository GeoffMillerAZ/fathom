import unittest
from native_measure import truth, compare


class NativeMeasureTests(unittest.TestCase):
    def test_native_population_not_extractor_and_context_not_double_counted(self):
        d = dict(path='tasks/main.yml', line=2, name='native', kind='task', external=False)
        rows = [dict(input='site.yml', mode='playbook', status='loaded', declarations=[d], bindings=[], unresolved=[]),
                dict(input='tasks/main.yml', mode='tasks', status='loaded', declarations=[d], bindings=[], unresolved=[]),
                dict(input='tasks/main.yml', mode='handlers', status='loaded', declarations=[dict(d, kind='handler')], bindings=[], unresolved=[])]
        t = truth(dict(results=rows))
        self.assertEqual(t['declarations'], [d])
        self.assertEqual(compare(t, {'files': []})['declarations'], {'matched': 0, 'native': 1, 'ratio': 0.0})
        files = {'files': [dict(path='tasks/main.yml', result=dict(dialect='ansible', symbols=[dict(name='native', line=2, kind='AnsibleTask')], references=[]))]}
        self.assertEqual(compare(t, files)['declarations']['matched'], 1)
        files['files'][0]['result']['symbols'][0]['line'] = 3
        self.assertEqual(compare(t, files)['declarations']['matched'], 0)

    def test_unresolved_dynamic_cases_remain_misses_and_external_is_disclosed(self):
        d = dict(path='x.yml', line=1, name='task', kind='task', external=False)
        ext = dict(d, path='../dep/tasks/main.yml', external=True)
        t = truth(dict(results=[dict(input='x.yml', mode='tasks', status='loaded', declarations=[d,ext], bindings=[], unresolved=[dict(source=d, reason='unresolved dynamic include_tasks')])]))
        r = compare(t, {'files': []})
        self.assertEqual(t['external_declarations'], 1)
        self.assertEqual(r['unresolved_misses'], 1)
        self.assertEqual(r['bindings_including_unresolved']['native'], 1)
        self.assertEqual(r['bindings_including_unresolved']['matched'], 0)

    def test_binding_requires_native_target_and_correct_source(self):
        s = dict(path='x.yml', line=2, name='task', kind='task', external=False)
        target = dict(path='x.yml', line=5, external=False)
        t = dict(declarations=[s,dict(target,name='handler',kind='handler')], bindings=[dict(source=s,target=target,notification='handler')], unresolved=[])
        result = dict(dialect='ansible', symbols=[dict(name='task',key='task',line=2,kind='AnsibleTask'),dict(name='handler',key='handler',line=5,kind='AnsibleHandler')], references=[dict(edge='notify', **{'from':'task'},to_key='handler')])
        self.assertEqual(compare(t, {'files':[dict(path='x.yml',result=result)]})['bindings']['matched'], 1)
        result['symbols'][1]['line'] = 6
        self.assertEqual(compare(t, {'files':[dict(path='x.yml',result=result)]})['bindings']['matched'], 0)


if __name__ == '__main__': unittest.main()
