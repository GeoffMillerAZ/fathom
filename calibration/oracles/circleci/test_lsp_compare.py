import unittest
import lsp_compare as audit

SOURCE = '''version: 2.1
executors:
  pyexec:
    docker: [{image: cimg/python:3.12}]
commands:
  greet:
    steps: [{run: echo hello}]
  wrap:
    steps:
      - when:
          condition: true
          steps: [greet]
jobs:
  build:
    executor: {name: pyexec}
    steps: [greet, {wrap: {}}]
workflows:
  main:
    jobs: [build]
'''

class SiteContract(unittest.TestCase):
    def test_all_nested_command_and_named_executor_uses_have_source_and_target_positions(self):
        sites = audit.candidate_sites(SOURCE)
        self.assertEqual([(x['owner'], x['target'], x['family']) for x in sites],
                         [('build', 'pyexec', 'executor'), ('build', 'greet', 'command'),
                          ('build', 'wrap', 'command'), ('wrap', 'greet', 'command')])
        for site in sites:
            self.assertGreaterEqual(site['position']['line'], 0)
            self.assertLess(site['declaration']['line'], site['position']['line'])

    def test_unknown_orb_and_builtin_steps_are_not_local_candidates(self):
        self.assertEqual(audit.candidate_sites('jobs:\n  a:\n    steps: [checkout, orb/run, {run: hi}]\n'), [])

    def test_utf16_position_after_a_non_bmp_character(self):
        sites = audit.candidate_sites('commands:\n  greet: {steps: []}\njobs:\n  a:\n    steps: ["🦀", greet]\n')
        self.assertEqual(sites[0]['position'], {'line': 4, 'character': 18})

    def test_null_and_location_link_responses(self):
        self.assertEqual(audit.locations(None), [])
        rng = {'start': {'line': 1, 'character': 0}, 'end': {'line': 2, 'character': 0}}
        self.assertEqual(audit.locations([{'targetUri': 'file:///config.yml', 'targetSelectionRange': rng}]), [('file:///config.yml', rng)])

if __name__ == '__main__':
    unittest.main()
