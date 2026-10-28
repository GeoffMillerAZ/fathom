"""Archive pins must belong to the repository and commit they describe."""

import hashlib
import io
from pathlib import Path
import tarfile
import tempfile
import unittest

from calibration.oracles.crossplane import verify_archives as checker


class ArchivePinTest(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        cls.tmp = Path(__file__).resolve().parents[3] / "workspace/tmp"
        cls.tmp.mkdir(parents=True, exist_ok=True)

    def test_copied_pin_is_not_accepted(self):
        with tempfile.TemporaryDirectory(dir=self.tmp) as directory:
            path = Path(directory) / "fixture.tar.gz"
            row = {"repo": "fixture/subject", "sha": "a" * 40,
                   "archive_sha256": "0" * 64, "archive_bytes": 1}
            with tarfile.open(path, "w:gz") as archive:
                item = tarfile.TarInfo("subject-" + row["sha"] + "/fixture.yaml")
                item.size = 1
                archive.addfile(item, io.BytesIO(b"x"))
            self.assertFalse(checker.verify(row, path)["matches_pin"])
            row.update(archive_sha256=hashlib.sha256(path.read_bytes()).hexdigest(),
                       archive_bytes=path.stat().st_size)
            self.assertTrue(checker.verify(row, path)["matches_pin"])
            row["sha"] = "b" * 40
            with self.assertRaisesRegex(ValueError, "subject commit"):
                checker.verify(row, path)

    def test_subject_identity_separates_versions(self):
        self.assertNotEqual(checker.archive_identity({"repo": "fixture/subject", "sha": "a" * 40}),
                            checker.archive_identity({"repo": "fixture/subject", "sha": "b" * 40}))


if __name__ == "__main__":
    unittest.main()
