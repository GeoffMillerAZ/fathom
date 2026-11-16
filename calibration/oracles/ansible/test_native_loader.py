"""Native loader controls; fixtures are authored, never calibration subjects."""
import json
import os
from pathlib import Path
import tempfile
import unittest

import native_loader as n


class NativeLoaderTests(unittest.TestCase):
    def setUp(self):
        self.tmp = tempfile.TemporaryDirectory(dir=os.environ['FATHOM_ANSIBLE_TEST_TMP'])
        self.addCleanup(self.tmp.cleanup)
        self.root = Path(self.tmp.name)

    def write(self, path, data):
        p = self.root / path
        p.parent.mkdir(parents=True, exist_ok=True)
        p.write_text(data)
        return p

    def run_native(self, path, mode='playbook'):
        return n.run_unit(self.root, path, mode, self.root / 'work')

    def test_native_positions_binding_shadowing_listen_and_no_execution(self):
        self.write('site.yml', '''- hosts: all
  tasks:
    - name: change
      ansible.builtin.command: touch SHOULD_NOT_EXIST
      notify: [restart, topic]
  handlers:
    - name: restart
      ansible.builtin.debug: {msg: older}
    - name: restart
      ansible.builtin.debug: {msg: newer}
    - name: listener
      listen: topic
      ansible.builtin.debug: {msg: listener}
''')
        r = self.run_native('site.yml')
        self.assertEqual(r['status'], 'loaded', r)
        self.assertIn(('site.yml', 3, 'change'), [(d['path'], d['line'], d['name']) for d in r['declarations']])
        self.assertEqual([(b['notification'], b['target']['line']) for b in r['bindings']], [('restart', 7), ('topic', 11)])
        self.assertFalse((self.root / 'SHOULD_NOT_EXIST').exists())
        self.assertTrue(all('end_line' not in d for d in r['declarations']))

    def test_static_import_position_and_role_handler_context(self):
        self.write('roles/demo/tasks/main.yml', '- name: role task\n  ansible.builtin.debug: {msg: x}\n  notify: restart\n')
        self.write('roles/demo/handlers/main.yml', '- name: restart\n  ansible.builtin.debug: {msg: x}\n')
        self.write('tasks/child.yml', '- name: imported\n  ansible.builtin.debug: {msg: x}\n  notify: restart\n')
        self.write('site.yml', '- hosts: all\n  roles: [demo]\n  tasks:\n    - ansible.builtin.import_tasks: tasks/child.yml\n')
        r = self.run_native('site.yml')
        self.assertEqual(r['status'], 'loaded', r)
        self.assertIn(('tasks/child.yml', 1), [(d['path'],d['line']) for d in r['declarations']])
        self.assertEqual(len(r['bindings']), 2, r)
        self.assertTrue(all(b['target']['path']=='roles/demo/handlers/main.yml' and b['target']['line']==1 for b in r['bindings']))

    def test_role_loader_and_nested_sections(self):
        self.write('roles/demo/tasks/main.yml', '''- block:
    - name: body
      ansible.builtin.debug: {msg: body}
  rescue:
    - name: recover
      ansible.builtin.debug: {msg: recover}
  always:
    - name: cleanup
      ansible.builtin.debug: {msg: cleanup}
''')
        r = self.run_native('roles/demo', 'role')
        self.assertEqual(r['status'], 'loaded', r)
        self.assertTrue({'body', 'recover', 'cleanup'} <= {d['name'] for d in r['declarations']})

    def test_dynamic_and_unknown_notifications_are_misses(self):
        self.write('site.yml', '''- hosts: all
  tasks:
    - ansible.builtin.include_tasks: "{{ selection }}.yml"
    - name: change
      ansible.builtin.debug: {msg: test}
      notify: absent
''')
        r = self.run_native('site.yml')
        self.assertEqual(r['status'], 'loaded', r)
        self.assertEqual(len(r['unresolved']), 2, r)
        self.assertEqual(r['bindings'], [])
        self.assertTrue(any('include_tasks' in u['reason'] for u in r['unresolved']))

    def test_malformed_and_whole_unit_cap_plus_one(self):
        self.write('bad.yml', '- hosts: [')
        r = self.run_native('bad.yml')
        self.assertEqual(r['status'], 'rejected')
        self.assertEqual(r['declarations'], [])
        self.write('large.yml', '- hosts: all\n  tasks:\n' + '    - ansible.builtin.debug: {msg: x}\n' * 3)
        r = n.run_unit(self.root, 'large.yml', 'playbook', self.root / 'work', max_objects=3)
        self.assertEqual(r['status'], 'refused', r)
        self.assertEqual(r['declarations'], [])
        self.assertEqual(r['bindings'], [])

    def test_input_edge_depth_and_output_caps(self):
        self.write('huge.yml', 'x' * (n.MAX_INPUT_BYTES + 1))
        r = self.run_native('huge.yml')
        self.assertEqual((r['status'], r['reason']), ('refused', 'input cap'))
        self.write('edges.yml', '- hosts: all\n  tasks:\n    - ansible.builtin.debug: {msg: x}\n      notify: [a,b,c,d]\n  handlers:\n    - name: h\n      listen: [a,b,c,d]\n      ansible.builtin.debug: {msg: x}\n')
        r = n.run_unit(self.root, 'edges.yml', 'playbook', self.root / 'work', max_objects=3)
        self.assertEqual((r['status'], r['reason']), ('refused', 'edge cap'))
        self.assertEqual(r['bindings'], [])
        self.assertEqual(r['declarations'], [])
        # Core's blocks add depth for every authored block nesting.
        text = '- ansible.builtin.debug: {msg: x}\n'
        for _ in range(n.MAX_DEPTH + 1):
            text = '- block:\n' + ''.join('    '+line+'\n' for line in text.splitlines())
        self.write('deep.yml', text)
        r = self.run_native('deep.yml', 'tasks')
        self.assertEqual((r['status'],r['reason']), ('refused','nesting cap'), r)
        self.assertEqual(r['declarations'], [])
        work = self.root / 'output-work'
        with self.assertRaises(n.Refused):
            n.bounded([os.sys.executable, '-c', 'print("x" * '+str(n.MAX_OUTPUT_BYTES + 1)+')'],
                      work, n.private_env(work))

    def test_population_includes_non_task_yaml_and_rejects_symlink(self):
        self.write('vars.yml', 'ordinary: value\n')
        self.write('nested/task.yaml', '- ansible.builtin.debug: {msg: x}\n')
        self.assertEqual(n.population(self.root), ['nested/task.yaml', 'vars.yml'])
        (self.root / 'escape.yml').symlink_to('/etc/passwd')
        with self.assertRaises(n.Refused):
            n.population(self.root)

    def test_truth_independent_of_extractor_and_changes_with_input(self):
        self.write('site.yml', '- hosts: all\n  tasks:\n    - name: first\n      ansible.builtin.debug: {msg: x}\n')
        a = self.run_native('site.yml')
        self.write('extractor-output.json', '[{"name":"fabricated"}]')
        self.assertEqual(a, self.run_native('site.yml'))
        self.write('site.yml', '- hosts: all\n  tasks:\n    - name: second\n      ansible.builtin.debug: {msg: x}\n')
        self.assertNotEqual(a, self.run_native('site.yml'))

    def test_sandbox_blocks_network_and_outside_write(self):
        r = n.containment_probe(self.root / 'work')
        self.assertEqual(r, {'network_blocked': True, 'write_blocked': True, 'read_blocked': True, 'fork_blocked': True})




class BoundaryTests(unittest.TestCase):
    """Pure/mocked correction controls: no Ansible population or consumer."""
    def setUp(self):
        from unittest.mock import patch
        self.patch = patch
        self.tmp = tempfile.TemporaryDirectory(dir=os.environ['FATHOM_ANSIBLE_TEST_TMP'])
        self.addCleanup(self.tmp.cleanup)
        self.root = Path(self.tmp.name).resolve()
        self.subject = self.root / 'subject'
        self.subject.mkdir()
        (self.subject / 'a.yml').write_text('- debug: {}\n')

    def fake_native(self, notices, cap, name='task', action='debug'):
        import types
        import sys
        task = types.SimpleNamespace(implicit=False, name=name, action=action, notify=notices,
                                     get_path=lambda: str(self.subject / 'a.yml') + ':1')
        class Loader:
            def set_basedir(self, *args): pass
            def load_from_file(self, *args): return []
        class Play:
            @staticmethod
            def load(*args, **kwargs): return types.SimpleNamespace(handlers=[])
        class Strategy:
            def search_handlers_by_notification(self, *args): return []
        def mod(**attrs):
            m = types.ModuleType('fake'); m.__dict__.update(attrs); return m
        modules = {
            'ansible.parsing.dataloader': mod(DataLoader=Loader),
            'ansible.playbook': mod(Playbook=object),
            'ansible.playbook.play': mod(Play=Play),
            'ansible.playbook.block': mod(Block=type('Block', (), {})),
            'ansible.playbook.role': mod(Role=object),
            'ansible.playbook.role.include': mod(RoleInclude=object),
            'ansible.playbook.helpers': mod(load_list_of_blocks=lambda *a, **k: [task]),
            'ansible.plugins.loader': mod(init_plugin_loader=lambda: None),
            'ansible.plugins.strategy': mod(StrategyBase=Strategy),
            'ansible.vars.manager': mod(VariableManager=lambda **k: None),
        }
        with self.patch.dict(sys.modules, modules):
            return n.native(self.subject, 'a.yml', 'tasks', cap)

    def test_template_identity_and_every_edge_append_cap(self):
        notices = ['{{ a }}', '{{ b }}', '{{ c }}', '{{ d }}']
        below = self.fake_native(notices[:3], 3)
        self.assertEqual(below['status'], 'loaded')
        self.assertEqual(len(below['unresolved']), 3)
        self.assertEqual({u.get('notification') for u in below['unresolved']}, set(notices[:3]))
        for case in [(notices, 'task', 'debug'), ([], '{{ task }}', 'include_tasks'),
                     (['a', 'b'], 'task', 'include_tasks')]:
            with self.subTest(case=case):
                cap = 3 if case[0] == notices else 1
                r = self.fake_native(case[0], cap, case[1], case[2])
                self.assertEqual((r['status'], r.get('reason')), ('refused', 'edge cap'))
                self.assertEqual([r[k] for k in ('declarations', 'bindings', 'unresolved')], [[], [], []])
        repeated = self.fake_native(['{{ a }}'] * 4, 3)
        self.assertEqual(repeated['status'], 'refused', 'cap counts appends before dedup')

    def test_consumed_symlinks_and_directory_links_never_launch(self):
        (self.subject / 'alias.yml').symlink_to('a.yml')
        (self.subject / 'real').mkdir()
        (self.subject / 'real' / 'b.yml').write_text('[]')
        (self.subject / 'linked').symlink_to('real', target_is_directory=True)
        for path in ['alias.yml', 'linked/b.yml', 'linked', '../subject/a.yml']:
            with self.subTest(path=path), self.patch.object(n, 'bounded', return_value=n.empty('loaded', 'mock')) as child:
                try:
                    r = n.run_unit(self.subject, path, 'tasks', self.root / 'work')
                except n.Refused:
                    r = n.empty('refused', 'path')
                self.assertEqual(r['status'], 'refused')
                child.assert_not_called()
        with self.patch.object(n, 'bounded', return_value=n.empty('loaded', 'mock')) as child:
            self.assertEqual(n.run_unit(self.subject, 'real/b.yml', 'tasks', self.root / 'work')['status'], 'loaded')
            child.assert_called_once()

    def test_bad_child_schema_is_empty_classified_refusal(self):
        for child in [[], None, {}, {'status': 'loaded'}, n.empty('bogus', 'x'),
                      dict(n.empty('loaded', ''), declarations=[{}]),
                      dict(n.empty('refused', 'x'), bindings=[{}]),
                      dict(n.empty('loaded', ''), unresolved='not a list'),
                      dict(n.empty('loaded', ''), input='spoof.yml'),
                      dict(n.empty('loaded', ''), mode='role')]:
            with self.subTest(child=child), self.patch.object(n, 'bounded', return_value=child):
                r = n.run_unit(self.subject, 'a.yml', 'tasks', self.root / 'work')
                self.assertIsInstance(r, dict)
                self.assertEqual(r.get('status'), 'refused')
                self.assertEqual([r[k] for k in ('declarations', 'bindings', 'unresolved')], [[], [], []])
                self.assertIn('child', r['reason'])

    def test_bad_child_json_is_typed_refusal(self):
        from types import SimpleNamespace
        def run(*args, **kwargs):
            kwargs['stdout'].write(b'not JSON')
            return SimpleNamespace(returncode=0)
        work = self.root / 'work'
        env = n.private_env(work)
        with self.patch.object(n.subprocess, 'run', side_effect=run):
            try:
                n.bounded(['synthetic'], work, env)
            except Exception as exc:
                self.assertIsInstance(exc, n.Refused, 'decoding failure must be classified')
            else:
                self.fail('malformed child accepted')

    def invoke(self, population, out, response=None):
        argv = ['native_loader', '--subject', str(self.subject), '--population', str(population),
                '--work', str(self.root / 'work'), '--out', str(out)]
        with self.patch.object(n.sys, 'argv', argv), self.patch.object(n.subprocess, 'check_output', return_value='a'*40), \
             self.patch.object(n, 'bounded', side_effect=response or [n.empty('loaded', 'mock')] * 6) as child:
            try:
                n.main()
            except (ValueError, OSError, SystemExit):
                pass
            except Exception as exc:
                self.fail('unclassified driver error: ' + type(exc).__name__)
            return child

    def test_driver_accounts_remaining_units_after_malformed_child(self):
        pop = self.root / 'population.json'
        pop.write_text(json.dumps([dict(subject='subject', sha='a'*40, yaml=['a.yml', 'b.yml'])]))
        (self.subject / 'b.yml').write_text('[]')
        out = self.root / 'out.json'
        self.invoke(pop, out, [[], *[n.empty('rejected', 'mock')] * 5])
        rows = json.loads(out.read_text())['results']
        self.assertEqual(len(rows), 6)
        self.assertEqual(rows[0]['status'], 'refused')
        self.assertEqual([r['status'] for r in rows[1:]], ['rejected'] * 5)

    def test_loader_failure_invalidates_stale_and_preserves_input_aliases(self):
        pop = self.root / 'population.json'; pop.write_text('bad json')
        out = self.root / 'out.json'; out.write_text('{"success":true}')
        self.invoke(pop, out)
        self.assertFalse(out.exists(), 'failed invocation must remove owned stale success')
        original = pop.read_bytes()
        hard = self.root / 'hard.json'; os.link(pop, hard)
        link = self.root / 'link.json'; link.symlink_to(pop.name)
        for alias in [pop, hard, link]:
            child = self.invoke(pop, alias)
            self.assertEqual(pop.read_bytes(), original)
            self.assertEqual(alias.read_bytes(), original)
            child.assert_not_called()

    def test_read_probe_uses_real_outside_lane_boundary(self):
        outside = Path(os.environ['FATHOM_ANSIBLE_OUTSIDE_CANARY']).resolve()
        outside.write_text('owned synthetic read canary')
        self.assertFalse(outside.is_relative_to(Path(n.__file__).resolve().parents[3]))
        r = n.containment_probe(self.root / 'probe', outside)
        self.assertEqual(r, dict(network_blocked=True, write_blocked=True, read_blocked=True, fork_blocked=True))

    def test_role_subject_root_is_admitted_and_symlink_subject_is_not(self):
        with self.patch.object(n, 'bounded', return_value=n.empty('loaded', 'mock')) as child:
            r = n.run_unit(self.subject, '.', 'role', self.root / 'work')
            self.assertEqual(r['status'], 'loaded')
            child.assert_called_once()
        link = self.root / 'subject-link'; link.symlink_to(self.subject, target_is_directory=True)
        pop = self.root / 'population.json'
        pop.write_text(json.dumps([dict(subject='subject', sha='a'*40, yaml=['a.yml'])]))
        with self.patch.object(self, 'subject', link):
            child = self.invoke(pop, self.root / 'out.json')
            child.assert_not_called()

    def test_valid_population_output_aliases_are_never_overwritten(self):
        pop = self.root / 'population.json'
        pop.write_text(json.dumps([dict(subject='subject', sha='a'*40, yaml=['a.yml'])]))
        hard = self.root / 'aliased-subject.yml'; os.link(self.subject / 'a.yml', hard)
        parent = self.root / 'parent-link'; parent.symlink_to(self.root, target_is_directory=True)
        for alias in [pop, hard, self.subject / 'a.yml', parent / 'population.json']:
            before = pop.read_bytes(); source = (self.subject / 'a.yml').read_bytes()
            child = self.invoke(pop, alias)
            child.assert_not_called()
            self.assertEqual(pop.read_bytes(), before)
            self.assertEqual((self.subject / 'a.yml').read_bytes(), source)

    def test_general_read_boundary_removal_changes_probe(self):
        work = self.root / 'probe-mutant'
        profile = n.sandbox_profile(work)
        marker = '(deny file-read-data (require-all (subpath "/Users")'
        with self.patch.object(n, 'sandbox_profile', return_value=profile[:profile.index(marker)]):
            result = n.containment_probe(work)
        self.assertFalse(result['read_blocked'])
        self.assertTrue(result['network_blocked'])
        self.assertTrue(result['write_blocked'])
        self.assertTrue(result['fork_blocked'])


if __name__ == '__main__':
    unittest.main()
