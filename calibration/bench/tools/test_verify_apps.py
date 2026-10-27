"""Reading, receipt and archived run evidence must agree independently."""
import sys
sys.dont_write_bytecode = True
import json
from pathlib import Path
import tempfile
import unittest
from verify_apps import verify, check_privacy

READING = Path('docs/bench/readings/bench-prep/apps-r1.json')


class ReadingMutations(unittest.TestCase):
    def test_original_evidence(self):
        verify(READING)

    def test_fixed_identity_patterns(self):
        for value in [b'/' + b'Users/' + b'account/file', b'/' + b'home/' + b'account/file', b'fixture' + b'@' + b'real-domain.invalid', b'fixture' + b'@' + b'0domain.invalid']:
            with self.assertRaises(AssertionError):
                check_privacy(value, 'fixture')
        check_privacy(b'/work/home/cache fixture@example.invalid package@1.0.0.tgz', 'fixture')

    def check_mutation(self, change_json=None, change_md=None):
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            data = json.loads(READING.read_text())
            md = READING.with_suffix('.md').read_text()
            (root / 'raw').symlink_to(READING.parent.resolve() / 'raw', target_is_directory=True)
            if change_json:
                change_json(data)
            if change_md:
                md = change_md(md)
            path = root / 'apps-r1.json'
            path.write_text(json.dumps(data))
            path.with_suffix('.md').write_text(md)
            with self.assertRaises(AssertionError):
                verify(path)

    def test_mutated_reading_count(self):
        self.check_mutation(change_md=lambda s: s.replace('31,687 / middle', '31,688 / middle', 1))

    def test_mutated_reading_archive_sha(self):
        digest = json.loads(READING.read_text())['archive']['sha256']
        self.check_mutation(change_md=lambda s: s.replace(digest, '0' * 64, 1))

    def test_mutated_json_warmup(self):
        self.check_mutation(
            change_json=lambda d: d['subjects'][2]['suite']['warmup'].update(exit_code=0),
            change_md=lambda s: s.replace('| derailed/k9s | 154.987 / 0 | 213.534 / 2 |',
                                          '| derailed/k9s | 154.987 / 0 | 213.534 / 0 |', 1))


if __name__ == '__main__':
    unittest.main()
