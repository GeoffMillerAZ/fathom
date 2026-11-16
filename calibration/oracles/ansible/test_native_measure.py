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




class CollectionBoundaryTests(unittest.TestCase):
    def setUp(self):
        import copy
        import native_measure as m
        self.m, self.copy = m, copy.deepcopy
        self.d = dict(path='a.yml', line=1, name='a', kind='task', external=False)
        self.inventory = dict(subject='fixture', sha='a'*40, yaml=['a.yml', 'b.yml'])
        self.document = dict(subject='fixture', sha='a'*40, yaml_candidates=2, role_candidates=0,
                             tasks_executed=False, results=[dict(input=p, mode=mode, status='loaded',
                             declarations=[self.d] if p == 'a.yml' else [], bindings=[], unresolved=[])
                             for p in self.inventory['yaml'] for mode in ['playbook', 'tasks', 'handlers']])
        self.product = dict(files=[dict(path=p, result=None) for p in self.inventory['yaml']])

    def test_partial_header_cannot_yield_complete_ratio(self):
        partial = self.copy(self.document); partial['results'] = partial['results'][:1]
        with self.assertRaises(ValueError):
            self.m.truth(partial)

    def test_frozen_inventory_modes_and_product_coverage(self):
        # The exact entry CLI, with valid synthetic inputs and malformed variants.
        import tempfile, os, json
        from pathlib import Path
        from unittest.mock import patch
        with tempfile.TemporaryDirectory(dir=os.environ['FATHOM_ANSIBLE_TEST_TMP']) as tmp:
            root = Path(tmp)
            native, product, population, out = [root / p for p in ['native.json','product.json','pop.json','out.json']]
            def invoke(doc, prod, inv):
                native.write_text(json.dumps(doc)); product.write_text(json.dumps(prod)); population.write_text(json.dumps([inv]))
                out.write_text('{"stale":true}')
                argv = ['measure', '--native', str(native), '--product', str(product), '--population', str(population), '--out', str(out)]
                with patch('sys.argv', argv):
                    try: self.m.main()
                    except (ValueError, OSError, SystemExit): pass
            invoke(self.document, self.product, self.inventory)
            self.assertTrue(out.exists())
            self.assertNotIn('stale', json.loads(out.read_text()))
            complete = json.loads(out.read_text())
            self.assertEqual(complete['collection']['attempted'], 6)
            self.assertEqual(complete['collection']['expected'], 6)
            self.assertEqual(complete['collection']['statuses'], {'loaded': 6, 'rejected': 0, 'refused': 0})
            for mutate in ['missing', 'duplicate', 'unexpected', 'badstatus', 'badschema', 'header', 'identity', 'execution', 'mode']:
                doc = self.copy(self.document)
                if mutate == 'missing': doc['results'].pop()
                if mutate == 'duplicate': doc['results'].append(doc['results'][0])
                if mutate == 'unexpected': doc['results'][0]['input'] = 'unknown.yml'
                if mutate == 'badstatus': doc['results'][0]['status'] = 'unknown'
                if mutate == 'badschema': doc['results'][0]['declarations'] = [{}]
                if mutate == 'header': doc['yaml_candidates'] = 1
                if mutate == 'identity': doc['sha'] = 'b'*40
                if mutate == 'execution': doc['tasks_executed'] = True
                if mutate == 'mode': doc['results'][0]['mode'] = 'other'
                with self.subTest(mutate=mutate):
                    invoke(doc, self.product, self.inventory)
                    self.assertFalse(out.exists(), 'invalid collection must not publish a reduced denominator')
            for files in [self.product['files'][:1], self.product['files'] * 2,
                          [dict(path='unknown.yml', result=None)], [dict(path='a.yml', result=[])]]:
                with self.subTest(files=files):
                    invoke(self.document, dict(files=files), self.inventory)
                    self.assertFalse(out.exists())
            doc = self.copy(self.document)
            for row, status in zip(doc['results'], ['refused', 'rejected']):
                row.update(status=status, reason='synthetic', declarations=[])
            invoke(doc, self.product, self.inventory)
            self.assertEqual(json.loads(out.read_text())['collection']['statuses'], {'loaded': 4, 'rejected': 1, 'refused': 1})
            # Even self-consistent truncation must fail against the external freeze.
            doc = self.copy(self.document); doc['yaml_candidates'] = 1; doc['results'] = doc['results'][:3]
            invoke(doc, dict(files=self.product['files'][:1]), self.inventory)
            self.assertFalse(out.exists())

    def test_measure_failure_invalidates_stale_and_preserves_aliases(self):
        import tempfile, os, json
        from pathlib import Path
        from unittest.mock import patch
        with tempfile.TemporaryDirectory(dir=os.environ['FATHOM_ANSIBLE_TEST_TMP']) as tmp:
            root = Path(tmp); native = root/'native.json'; product = root/'product.json'; out = root/'out.json'
            native.write_text('bad json'); product.write_text('{"files":[]}')
            def invoke(output):
                with patch('sys.argv', ['measure','--native',str(native),'--product',str(product),'--out',str(output)]):
                    try: self.m.main()
                    except (ValueError, OSError, SystemExit): pass
            out.write_text('{"ratio":1.0}'); invoke(out)
            self.assertFalse(out.exists())
            for source in [native, product]:
                hard = root / (source.stem + '-hard.json'); os.link(source, hard)
                link = root / (source.stem + '-link.json'); link.symlink_to(source.name)
                for alias in [source, hard, link]:
                    before = source.read_bytes(); invoke(alias)
                    self.assertEqual(source.read_bytes(), before)
                    self.assertEqual(alias.read_bytes(), before)


if __name__ == '__main__':
    unittest.main()
