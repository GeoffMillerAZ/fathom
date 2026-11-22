"""Offline controls for journal exclusivity and aggregate-only path classes."""
import importlib.util
import json
from pathlib import Path
import tempfile
import unittest
from unittest.mock import patch

HERE = Path(__file__).resolve().parent


def load(name):
    spec = importlib.util.spec_from_file_location(name, HERE / (name + '.py'))
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    return module


campaign = load('cov_campaign')
metadata = load('cov_metadata')


class CampaignControls(unittest.TestCase):
    def test_path_classes_without_paths(self):
        self.assertEqual(metadata.classify('src/a.ts'), [])
        self.assertEqual(metadata.classify('src/a (copy)[test].ts'), ['square brackets', 'parentheses'])
        self.assertEqual(metadata.classify('../a.ts'), ['invalid path segment'])
        self.assertEqual(metadata.classify('é.ts'), ['non-ASCII'])
        self.assertEqual(metadata.classify('a' * 513), ['over 512 bytes'])

    def test_journal_is_exclusive_and_failed_result_stays_failed(self):
        with tempfile.TemporaryDirectory() as d:
            root = Path(d)
            (root / 'subjects.json').write_text('{}')
            receipt = root / 'receipt.log'
            receipt.write_text('FAIL\n')
            with patch.object(campaign.subprocess, 'check_output', side_effect=['', 'harness\n']):
                campaign.begin(root, receipt)
            with patch.object(campaign.subprocess, 'check_output', side_effect=['', 'harness\n']):
                with self.assertRaises(FileExistsError):
                    campaign.begin(root, receipt)
            (root / 'reading').mkdir()
            (root / 'reading/aggregate.json').write_text('{}')
            campaign.finish(root, receipt)
            self.assertEqual(json.loads((root / 'campaign.json').read_text())['status'], 'failed')
            with self.assertRaises(ValueError):
                campaign.finish(root, receipt)

    def test_reader_edit_refuses_authorization(self):
        with tempfile.TemporaryDirectory() as d:
            with patch.object(campaign.subprocess, 'check_output', return_value='pkg/lang/deps/reader.go\n'):
                with self.assertRaises(ValueError):
                    campaign.begin(Path(d), Path(d) / 'receipt.log')


if __name__ == '__main__':
    unittest.main()
