"""The version check must precede argument processing and filesystem writes."""
import unittest
from unittest.mock import patch

import fetch


class FetchVersionTest(unittest.TestCase):
    def test_old_python_exits_before_creating_output(self):
        with patch.object(fetch.sys, 'version_info', (3, 9, 6)), \
                patch.object(fetch.Path, 'mkdir') as mkdir:
            with self.assertRaisesRegex(SystemExit, 'Python 3.13'):
                fetch.main()
            mkdir.assert_not_called()


if __name__ == '__main__':
    unittest.main()
