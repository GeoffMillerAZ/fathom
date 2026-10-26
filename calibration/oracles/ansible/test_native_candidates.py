"""Authored native controls; no subject data is copied into fixtures."""
import json
import os
from pathlib import Path
import subprocess
import tempfile
import unittest
from measure_native_candidates import inventory_counts, utf8_locale


class NativeInventoryTest(unittest.TestCase):
    @unittest.skipUnless(os.environ.get('FATHOM_ORACLE_ANSIBLE_INVENTORY'), 'set FATHOM_ORACLE_ANSIBLE_INVENTORY to the pinned native CLI')
    def test_ini_ranges_and_children_are_native_inventory(self):
        with tempfile.TemporaryDirectory() as directory:
            work = Path(directory)
            file = work / 'inventory'
            file.write_text('[workers]\nnode[1:2]\n[prod:children]\nworkers\n')
            env = dict(os.environ, LC_ALL=utf8_locale(), LC_CTYPE=utf8_locale(), ANSIBLE_HOME=str(work / 'home'), ANSIBLE_CONFIG=str(work / 'absent.cfg'),
                       ANSIBLE_LOCAL_TEMP=str(work / 'tmp'), ANSIBLE_INVENTORY_ENABLED='ini', ANSIBLE_INVENTORY_UNPARSED_IS_FAILED='true')
            result = subprocess.run([str(Path(env['FATHOM_ORACLE_ANSIBLE_INVENTORY']).resolve()), '-i', str(file), '--list'],
                                    cwd=work, env=env, capture_output=True, text=True, timeout=30)
            self.assertEqual(0, result.returncode, result.stderr)
            document = json.loads(result.stdout)
            self.assertEqual(['workers'], document['prod']['children'])
            self.assertEqual(['node1', 'node2'], document['workers']['hosts'])
            self.assertEqual(2, inventory_counts(document)['hosts'])

    @unittest.skipUnless(os.environ.get('FATHOM_ORACLE_ANSIBLE_INVENTORY'), 'requires the pinned native environment')
    def test_position_comes_from_core_origin_not_a_lint_default(self):
        from ansible._internal._datatag._tags import Origin
        from ansiblelint.file_utils import Lintable
        from ansiblelint.utils import parse_yaml_linenumbers, task_in_list
        with tempfile.TemporaryDirectory() as directory:
            file = Path(directory) / 'tasks.yml'
            file.write_text('# authored control\n- name: hello\n  ansible.builtin.debug:\n    msg: hi\n')
            lintable = Lintable(file, kind='tasks')
            tasks = list(task_in_list(parse_yaml_linenumbers(lintable), lintable, 'tasks'))
            self.assertEqual(1, len(tasks))
            self.assertEqual(2, Origin.get_tag(tasks[0].raw_task).line_num)
            self.assertEqual(2, tasks[0].line)


if __name__ == '__main__':
    unittest.main()
