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


if __name__ == '__main__':
    unittest.main()
