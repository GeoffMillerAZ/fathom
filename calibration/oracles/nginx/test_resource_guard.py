"""Resource controls must refuse work before fetching or creating subjects."""
import tempfile
import unittest
from pathlib import Path
from types import SimpleNamespace
from unittest.mock import patch

from resource_guard import require_free_space
from grade_native_l2 import prepare


class ResourceGuardTest(unittest.TestCase):
    def test_low_disk_stops_before_subject_creation_or_network(self):
        with tempfile.TemporaryDirectory() as directory:
            subject = Path(directory) / 'not-created'
            with patch('resource_guard.shutil.disk_usage', return_value=SimpleNamespace(free=19 * 1024**3)):
                with patch('grade_native_l2.run') as command:
                    with self.assertRaisesRegex(ValueError, '20 GiB'):
                        prepare({'url': 'unused', 'sha': 'unused'}, subject)
                    command.assert_not_called()
            self.assertFalse(subject.exists())

    def test_threshold_is_inclusive_and_reports_free_space(self):
        with patch('resource_guard.shutil.disk_usage', return_value=SimpleNamespace(free=20 * 1024**3)):
            require_free_space('source build')
        with patch('resource_guard.shutil.disk_usage', return_value=SimpleNamespace(free=11 * 1024**3)):
            with self.assertRaisesRegex(ValueError, '11.00 GiB'):
                require_free_space('source build')
