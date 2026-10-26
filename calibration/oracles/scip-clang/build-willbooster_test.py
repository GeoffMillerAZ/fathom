#!/usr/bin/env python3
"""Focused archive boundary checks for the private C++ builder."""

import importlib.util
import io
from pathlib import Path
import tarfile
import tempfile
import unittest
from unittest import mock

HERE = Path(__file__).resolve().parent
spec = importlib.util.spec_from_file_location("builder", HERE / "build-willbooster.py")
builder = importlib.util.module_from_spec(spec)
spec.loader.exec_module(builder)


class ArchiveTest(unittest.TestCase):
    def test_wrong_tool_hash_refused(self):
        with tempfile.TemporaryDirectory(dir=builder.ROOT / "workspace") as temp:
            binary = Path(temp) / "zig"
            binary.write_text("#!/bin/sh\necho 0.16.0\n")
            binary.chmod(0o700)
            with mock.patch.object(builder.shutil, "which", return_value=str(binary)):
                with self.assertRaisesRegex(builder.audit.Refusal, "hash mismatch"):
                    builder.tool("zig", "0.16.0", "0" * 64)

    def test_canonical_file_and_link_refusal(self):
        with tempfile.TemporaryDirectory(dir=builder.ROOT / "workspace") as temp:
            root = Path(temp)
            archive = root / "source.crate"
            with tarfile.open(archive, "w:gz") as tar:
                payload = b"source"
                member = tarfile.TarInfo("crate-1/src/parser.c")
                member.size = len(payload)
                tar.addfile(member, io.BytesIO(payload))
            self.assertEqual(builder.extract(archive, root / "extracted", "crate-1"), (1, 1, 6))
            self.assertEqual((root / "extracted/src/parser.c").read_bytes(), b"source")
            with tarfile.open(archive, "w:gz") as tar:
                member = tarfile.TarInfo("crate-1/src/link")
                member.type = tarfile.SYMTYPE
                member.linkname = "/outside"
                tar.addfile(member)
            with self.assertRaises(builder.audit.Refusal):
                builder.extract(archive, root / "linked", "crate-1")

    def test_entry_cap_plus_one_refuses_whole_archive(self):
        with tempfile.TemporaryDirectory(dir=builder.ROOT / "workspace") as temp:
            root = Path(temp)
            archive = root / "source.crate"
            with tarfile.open(archive, "w:gz") as tar:
                for index in range(1001):
                    member = tarfile.TarInfo(f"crate-1/dir{index}")
                    member.type = tarfile.DIRTYPE
                    tar.addfile(member)
            with self.assertRaises(builder.audit.Refusal):
                builder.extract(archive, root / "extracted", "crate-1")


if __name__ == "__main__":
    unittest.main()
