"""Required runner proof; explicitly skipped off Linux, never on refusal."""
import sys
import tempfile
import unittest
from pathlib import Path
from kernel_filesystem_control import run


class LinuxFilesystemKernelTest(unittest.TestCase):
    @unittest.skipUnless(sys.platform.startswith('linux'),'requires a real Linux namespace')
    def test_real_namespace_refuses_dot_dot_writes_and_hidden_submount(self):
        with tempfile.TemporaryDirectory() as directory:
            result=run(Path(directory).resolve()/'kernel-control')
            self.assertTrue(result['linux_namespace_executed'])
            self.assertEqual(2,result['scenarios'])
            self.assertEqual(12,result['write_denials'])
            self.assertFalse(result['native_executed'])
