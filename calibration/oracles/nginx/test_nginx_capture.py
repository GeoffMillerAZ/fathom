"""Exercise the real measurement boundary, not only the decoder's cap."""
import tempfile
import os
from contextlib import contextmanager
import unittest
from pathlib import Path
from unittest.mock import patch
import nginx_oracle


class CaptureLimitTest(unittest.TestCase):
    def test_nginx_check_bounds_child_output_before_capture(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            subject = root / 'subject'; subject.mkdir()
            (subject / 'nginx.conf').write_text('events {}\n')
            binary = root / 'authored-native'
            binary.write_text('#!/usr/bin/env python3\nimport os\n'
                              'for i in range(20000): os.write(2,b"x"*1024)\n')
            binary.chmod(0o700)
            observed, sizes = [], []
            temporary_file = tempfile.TemporaryFile
            @contextmanager
            def checked_file(*args, **kwargs):
                with temporary_file(*args, **kwargs) as stream:
                    try:
                        yield stream
                    finally:
                        sizes.append(os.fstat(stream.fileno()).st_size)
            # Exercise the production 8 MiB bound at the real boundary.
            with patch.object(nginx_oracle.tempfile, 'TemporaryFile', checked_file):
                with self.assertRaisesRegex(ValueError, 'output.*budget'):
                    nginx_oracle.nginx_check(str(subject), '', str(binary), str(root / 'work'),
                                             'main', 'nginx.conf', observer=observed.append)
            self.assertEqual([], observed)
            self.assertEqual(nginx_oracle.MAX_OBSERVER_OUTPUT + 1, max(sizes))
            self.assertEqual(2, len(sizes))

    def test_bounded_capture_refuses_exact_cap_plus_one(self):
        with tempfile.TemporaryDirectory() as directory:
            binary = Path(directory) / 'authored-native'
            binary.write_text('#!/usr/bin/env python3\nimport os\nos.write(2,b"x"*4097)\n')
            binary.chmod(0o700)
            with patch.object(nginx_oracle, 'MAX_OBSERVER_OUTPUT', 4096, create=True):
                with self.assertRaisesRegex(ValueError, 'output.*budget'):
                    nginx_oracle.observer_run([str(binary)], directory)
