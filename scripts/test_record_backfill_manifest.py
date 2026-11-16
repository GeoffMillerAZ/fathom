"""Regression tests for the scratch-only record-title manifest generator."""
import importlib.util
from pathlib import Path
import unittest
import hashlib
import json
import subprocess
import sys
import tempfile

spec = importlib.util.spec_from_file_location(
    "record_backfill", Path(__file__).with_name("record-backfill-manifest.py"))
generator = importlib.util.module_from_spec(spec)
spec.loader.exec_module(generator)

# Exact public source rows, retained independently of future plan edits.
ROWS = [('platform-engineering',
  48,
  'PLT-16',
  '- [ ] The lab card `calibration/labs.d/lab-platform-skills.yaml`: surfaces '
  '`skill:fathom-investigate` and `skill:fathom-platform-review`; should-fire prompts drawn by the '
  "prereg's seed from PLT-M0's `code-answerable` item ids on PLT-L0's subjects (review, inventory, "
  'investigation); near-miss should-not prompts (attestation-only items, non-platform questions '
  'sharing keywords); arms without the skills, with them, with them plus a public framework skill, '
  "and grep only; helped metric answer correctness against WAE-M0's labels and `receipt verify`, "
  "false cleans counted, attestation honesty by a lexical check of the answer's attestation "
  'statement; kill PLT-M5 (§4.7) (lane: controller) {id: PLT-16, oracle: lexical, path: '
  'calibration/labs.d/lab-platform-skills.yaml, pattern: "^id: lab-platform-skills$", needs: '
  '"PLT-10,PLT-11,PLT-M0,feature-labs.md#LAB-1"}'),
 ('project-understanding',
  103,
  'PU-W8',
  '- [ ] The lab card `calibration/labs.d/lab-why-card.yaml`: the plugin with and without '
  '`include: why`, a firing criterion and the helped metric (ADR-082 decision 5) (lane: '
  'controller) {id: PU-W8, oracle: lexical, path: calibration/labs.d/lab-why-card.yaml, pattern: '
  '"^id: lab-why-card$", needs: PU-W7}'),
 ('project-understanding',
  113,
  'PU-V2',
  '- [ ] The lab card `calibration/labs.d/lab-reviewer.yaml`: one public reviewer model and '
  "harness, named, with and without fathom's MCP server on a local channel, on ReviewBench at the "
  'pinned revision, same prompts, paired by PR (lane: controller) {id: PU-V2, oracle: lexical, '
  'path: calibration/labs.d/lab-reviewer.yaml, pattern: "^id: lab-reviewer$", needs: PU-V1}')]


class AnnotationRegressionTests(unittest.TestCase):
    def test_real_rows(self):
        for name, number, expected, line in ROWS:
            with self.subTest(plan=name, line=number):
                self.assertEqual(generator.extract_row_id(line), expected)

    def test_quoted_oracle_variants_and_field_order(self):
        patterns = [
            '"^id: oracle$"',
            r'"^id: oracle, id: other {id: nested}$"',
            r'"escaped \"id: oracle\", braces {2}, comma, tail"',
            r'"backslash \\"',
        ]
        for pattern in patterns:
            for fields in (f'id: REAL-1, pattern: {pattern}',
                           f'pattern: {pattern}, id: "REAL-1"'):
                with self.subTest(fields=fields):
                    self.assertEqual(generator.extract_row_id(
                        '- [ ] earlier {id: prose} (lane: worker) {' + fields + '}'),
                        'REAL-1')

    def test_refuses_missing_ambiguous_or_malformed_metadata(self):
        for annotation in (
            '', '{pattern: "id: oracle"}', '{id: }',
            '{id: A, id: B}', '{id: A, id: A}',
            '{id: "A}', '{id: A', '{id: A}}',
            '{id: A, broken}', '{id: A extra}',
            '{id: A} {pattern: "id: oracle"}',
        ):
            with self.subTest(annotation=annotation):
                with self.assertRaises(ValueError):
                    generator.extract_row_id('- [ ] (op) row ' + annotation)


class ManifestTests(unittest.TestCase):
    def setUp(self):
        scratch = Path(__file__).resolve().parents[1] / 'workspace/tmp'
        scratch.mkdir(parents=True, exist_ok=True)
        self.temp = tempfile.TemporaryDirectory(dir=scratch)
        self.addCleanup(self.temp.cleanup)
        self.root = Path(self.temp.name)
        self.plans = self.root / 'docs/plans'
        self.plans.mkdir(parents=True)
        self.raw = ('- [ ] (op) gate {id: G-1}\n'
                    '- [x] work (lane: worker) {pattern: "id: decoy", id: L-1}\n'
                    '- [ ] ordinary {id: OMIT}\n'
                    '  - [ ] (op) indented {id: OMIT-2}\n').encode()
        (self.plans / 'a.md').write_bytes(self.raw)
        (self.plans / 'b.md').write_text('unselected plan\n')
        (self.plans / 'README.md').write_bytes(self.raw)
        (self.plans / 'archive').mkdir()
        (self.plans / 'archive/old.md').write_bytes(self.raw)
        self.git('init', '-q')
        self.git('add', 'docs')
        self.git('-c', 'user.name=Test', '-c',
                 'user.email=test@example.invalid', 'commit', '-q', '-m', 'test source')
        self.base = self.git('rev-parse', 'HEAD').strip()

    def git(self, *args):
        return subprocess.check_output(['git', '-C', str(self.root), *args], text=True)

    def test_actual_stale_base_refuses_before_output(self):
        source = Path(__file__).resolve().parents[1]
        output = self.root / 'stale-candidate.md'
        # The real 468dc705 tree disagrees on library-roadmap and telemetry-links.
        result = subprocess.run([
            sys.executable, '-B', generator.__file__, '--root', str(source),
            '--base', '468dc705813382d584926887b451cbea145c76ac',
            '--output', str(output)], capture_output=True, text=True)
        self.assertEqual(result.returncode, 2, result.stdout + result.stderr)
        self.assertIn('source plan does not match base', result.stderr)
        self.assertIn('468dc705813382d584926887b451cbea145c76ac', result.stderr)
        self.assertFalse(output.exists())

    def test_preserves_format_selection_hashes_and_order(self):
        manifest = generator.generate(self.root)
        sha = lambda value: hashlib.sha256(value).hexdigest()
        self.assertEqual(manifest, dict(
            schema='fathom.record-title-backfill/v1', base=self.base,
            files={'docs/plans/a.md': sha(self.raw)},
            rows=[dict(path='docs/plans/a.md', line=n, row_id=row_id,
                       before_sha256=sha(self.raw.splitlines()[n-1]), gate=gate, lane=lane)
                  for n, row_id, gate, lane in [(1, 'G-1', True, ''),
                                               (2, 'L-1', False, 'worker')]]))
        rendered = generator.render(manifest)
        self.assertTrue(rendered.startswith(generator.HEADER))
        self.assertEqual(rendered.split('~~~json\n')[1],
                         json.dumps(manifest, indent=2) + '\n~~~\n')
        self.assertEqual(generator.generate(self.root), manifest)

    def run_cli(self, *args):
        return subprocess.run([sys.executable, '-B', str(Path(generator.__file__)),
                               '--root', str(self.root), *map(str, args)],
                              capture_output=True, text=True)

    def test_requires_new_explicit_scratch_output(self):
        self.assertEqual(self.run_cli().returncode, 2)
        companion = self.root / 'docs/contracts/records-integration-backfill.md'
        companion.parent.mkdir()
        companion.write_text('protected')
        self.assertEqual(self.run_cli('--output', companion).returncode, 2)
        self.assertEqual(companion.read_text(), 'protected')
        output = self.root / 'workspace/tmp/candidate.md'
        output.parent.mkdir(parents=True)
        self.assertEqual(self.run_cli('--output', output).returncode, 0)
        before = output.read_bytes()
        self.assertEqual(self.run_cli('--output', output).returncode, 2)
        self.assertEqual(output.read_bytes(), before)
        link = output.parent / 'escape'
        link.symlink_to(companion.parent, target_is_directory=True)
        self.assertEqual(self.run_cli('--output', link / 'new.md').returncode, 2)
        self.assertFalse((companion.parent / 'new.md').exists())

    def test_default_and_explicit_bind_to_source_commit(self):
        output = self.root / 'workspace/tmp/default.md'
        output.parent.mkdir(parents=True)
        self.assertEqual(self.run_cli('--output', output).returncode, 0)
        manifest = json.loads(output.read_text().split('~~~json\n')[1].split('\n~~~')[0])
        self.assertEqual(manifest['base'], self.base)
        explicit = output.with_name('explicit.md')
        self.assertEqual(self.run_cli('--base', self.base, '--output', explicit).returncode, 0)
        self.assertEqual(output.read_bytes(), explicit.read_bytes())
        for invalid in ('arbitrary text', '--help', '0' * 40):
            target = output.with_name('invalid.md')
            self.assertEqual(self.run_cli('--base=' + invalid, '--output', target).returncode, 2)
            self.assertFalse(target.exists())

    def test_source_mismatch_refuses_for_default_and_explicit(self):
        output = self.root / 'workspace/tmp/mismatch.md'
        output.parent.mkdir(parents=True)
        # Includes an unselected plan: the binding covers all scanned inputs.
        cases = [('a.md', self.raw + b'changed\n'),
                 ('b.md', b'changed unselected plan\n'),
                 ('new.md', b'no selected rows\n')]
        for name, raw in cases:
            path = self.plans / name
            original = path.read_bytes() if path.exists() else None
            path.write_bytes(raw)
            for flags in ((), ('--base', self.base)):
                self.assertEqual(self.run_cli(*flags, '--output', output).returncode, 2)
                self.assertFalse(output.exists())
            if original is None:
                path.unlink()
            else:
                path.write_bytes(original)
        (self.plans / 'a.md').unlink()
        self.assertEqual(self.run_cli('--base', self.base, '--output', output).returncode, 2)
        self.assertFalse(output.exists())

    def test_bad_candidate_refuses_before_output(self):
        (self.plans / 'a.md').write_text('- [ ] (op) row {pattern: "id: decoy"}')
        self.git('add', 'docs')
        self.git('-c', 'user.name=Test', '-c',
                 'user.email=test@example.invalid', 'commit', '-q', '-m', 'invalid candidate')
        output = self.root / 'workspace/tmp/candidate.md'
        output.parent.mkdir(parents=True)
        self.assertEqual(self.run_cli('--output', output).returncode, 2)
        self.assertFalse(output.exists())


if __name__ == "__main__":
    unittest.main()
