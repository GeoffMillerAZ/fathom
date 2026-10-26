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

class FullPopulationContract(unittest.TestCase):
    def test_unused_declarations_are_queried(self):
        declarations = audit.candidate_declarations(SOURCE)
        self.assertEqual({'pyexec', 'greet', 'wrap'}, {x['target'] for x in declarations})

    def test_workflow_executor_parameters_are_native_truth_even_without_a_walk_site(self):
        source = '''executors:
  linux: {machine: true}
  other: {machine: true}
jobs:
  build:
    parameters:
      platform: {type: executor}
    executor: << parameters.platform >>
    steps: []
workflows:
  ci:
    jobs:
      - build:
          matrix:
            parameters:
              platform: [linux, other]
'''
        declaration = next(d for d in audit.candidate_declarations(source) if d['target'] == 'linux')
        point = {'line': 15, 'character': 25}
        rng = {'start': point, 'end': {'line': 15, 'character': 30}}
        result = audit.classify_reference(source, declaration, rng)
        self.assertEqual(('build', 'linux', 'workflow-executor-parameter'),
                         (result['owner'], result['target'], result['classification']))

    def test_command_reference_to_a_workflow_job_is_an_oracle_error(self):
        source = '''commands:
  same: {steps: []}
jobs:
  same: {steps: [same]}
workflows:
  ci:
    jobs: [same]
'''
        declaration = audit.candidate_declarations(source)[0]
        rng = {'start': {'line': 6, 'character': 11}, 'end': {'line': 6, 'character': 15}}
        result = audit.classify_reference(source, declaration, rng)
        self.assertEqual('same-name-workflow-job-oracle-error', result['classification'])

    def test_unclassified_native_location_fails_closed(self):
        declaration = audit.candidate_declarations(SOURCE)[0]
        rng = {'start': {'line': 0, 'character': 0}, 'end': {'line': 0, 'character': 4}}
        with self.assertRaisesRegex(ValueError, 'unclassified'):
            audit.classify_reference(SOURCE, declaration, rng)
