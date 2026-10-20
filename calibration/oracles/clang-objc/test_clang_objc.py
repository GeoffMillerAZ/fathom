#!/usr/bin/env python3
"""test_clang_objc.py — the oracle against the dialect's own goldens.

Runs clang_objc.py over a copy of internal/extractor/dialects/objective-c/testdata and checks that, for the
fixtures the compiler parses clean, its message sends and calls are the calls the dialect's hand-read goldens list
for each method, that a file with a syntax error is left out as untrusted, and that an id-typed receiver is an
exclusion. Skipped where libclang or the SDK is absent.

    python3 test_clang_objc.py
"""
import json
import os
import shutil
import subprocess
import sys
import tempfile
import unittest

HERE = os.path.dirname(os.path.abspath(__file__))
REPO = os.path.abspath(os.path.join(HERE, "..", "..", ".."))
TESTDATA = os.path.join(REPO, "internal", "extractor", "dialects", "objective-c", "testdata")
LIBCLANG = os.environ.get("LIBCLANG", "/Library/Developer/CommandLineTools/usr/lib/libclang.dylib")
BIND = os.environ.get("CLANG_PYBIND", "")


@unittest.skipUnless(os.path.exists(LIBCLANG) and os.path.isdir(BIND), "libclang or its Python bindings are absent")
class OracleAgainstGoldens(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        cls.tmp = tempfile.mkdtemp(prefix="clang-objc-test-")
        subject = os.path.join(cls.tmp, "subject")
        os.makedirs(subject)
        for name in os.listdir(TESTDATA):
            if name.endswith((".m", ".h")):
                shutil.copy(os.path.join(TESTDATA, name), subject)
        out = os.path.join(cls.tmp, "oracle.json")
        subprocess.run([sys.executable, os.path.join(HERE, "clang_objc.py"), "--subject", subject, "--out", out],
                       check=True, capture_output=True)
        with open(out) as fh:
            cls.doc = json.load(fh)
        with open(os.path.join(cls.tmp, "oracle.errored.json")) as fh:
            cls.errored = json.load(fh)

    @classmethod
    def tearDownClass(cls):
        shutil.rmtree(cls.tmp, ignore_errors=True)

    def golden_calls(self, fixture):
        with open(os.path.join(TESTDATA, fixture + ".golden.json")) as fh:
            g = json.load(fh)
        out = set()
        for s in g["symbols"]:
            owner, _, name = s["name"].rpartition(".")
            for c in s.get("calls") or []:
                out.add((fixture, name, c))
        return out

    def oracle_calls(self, fixture):
        return {(c["path"], c["from"], c["to"]) for c in self.doc["calls"] if c["path"] == fixture}

    def test_sends_are_the_goldens(self):
        # sends.m reads clean; its goldens were read by hand from the fixture, the oracle from the compiler.
        # NSLog and the C helper are calls too, so the sets are compared whole.
        for fixture in ("sends.m", "Shape.m"):
            self.assertEqual(self.golden_calls(fixture), self.oracle_calls(fixture), fixture)

    def test_syntax_error_is_untrusted(self):
        self.assertIn("broken.m", self.errored)
        self.assertFalse([c for c in self.doc["calls"] if c["path"] == "broken.m"])

    def test_id_receiver_is_an_exclusion(self):
        self.assertIn({"path": "sends.m", "to": "performSelector:",
                       "reason": "every send of this selector in the file has a receiver of type id"},
                      self.doc["call_exclusions"])

    def test_imports_resolve_to_subject_files(self):
        pairs = {(i["path"], i["to_path"]) for i in self.doc["imports"] if not i.get("same_package")}
        self.assertIn(("Shape.m", "Shape.h"), pairs)
        self.assertTrue(all(not to.startswith("/") for _, to in pairs))


if __name__ == "__main__":
    unittest.main()
