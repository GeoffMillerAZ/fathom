"""Binary identity guard regression tests."""
import pathlib
import sys
import unittest
from unittest import mock

sys.path.insert(0, str(pathlib.Path(__file__).resolve().parent))
import grade


class BinaryIdentityTests(unittest.TestCase):
    def test_native_go_build_settings_accept_clean_exact_revision(self):
        output = "binary: go1.27.3\n\tbuild\tvcs.revision=" + grade.BASE_COMMIT + "\n\tbuild\tvcs.modified=false\n"
        with mock.patch.object(grade.subprocess, "check_output", return_value=output), mock.patch.object(grade, "digest", return_value="digest"):
            self.assertEqual(grade.source_binary(pathlib.Path("binary")), "digest")

    def test_dirty_wrong_or_missing_identity_refuses(self):
        for revision, modified in ((grade.BASE_COMMIT, "true"), ("0" * 40, "false"), ("", "")):
            output = "\tbuild\tvcs.revision=" + revision + "\n\tbuild\tvcs.modified=" + modified + "\n"
            with self.subTest(revision=revision, modified=modified), mock.patch.object(grade.subprocess, "check_output", return_value=output):
                with self.assertRaisesRegex(RuntimeError, "not a clean build"):
                    grade.source_binary(pathlib.Path("binary"))


if __name__ == "__main__":
    unittest.main()
