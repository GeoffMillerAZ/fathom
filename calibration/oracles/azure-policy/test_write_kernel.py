"""Mandatory runner regression, explicitly skipped only off Linux."""
import sys
import tempfile
import unittest
from pathlib import Path
from kernel_write_control import run

class LinuxWriteKernelTest(unittest.TestCase):
    @unittest.skipUnless(sys.platform.startswith('linux'),'requires Linux namespaces and strace')
    def test_real_private_scratch_child_attributes_sigxfsz_to_exact_file(self):
        with tempfile.TemporaryDirectory() as directory:
            value=run(Path(directory)/'write-control')
        self.assertTrue(value['private_scratch_boundary'])
        self.assertTrue(value['exact_limit_file_identified'])
        self.assertFalse(value['native_executed'])
