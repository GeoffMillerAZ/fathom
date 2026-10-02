"""Tests for fathom_config_oracle.py: python3 -m unittest test_fathom_config_oracle (from this directory)."""

import os
import tomllib
import unittest

import fathom_config_oracle as o

HERE = os.path.dirname(os.path.abspath(__file__))


def load(name):
    p = os.path.join(HERE, "fixtures", name, ".fathom", "config.toml")
    src = open(p, encoding="utf-8").read()
    return o.entities(".fathom/config.toml", tomllib.loads(src), src)


class OracleTest(unittest.TestCase):
    def test_profiles_fixture(self):
        defs, refs = load("profiles")
        by = {d["name"]: d for d in defs}
        self.assertEqual(by["default_profile"]["kind"], "key")
        self.assertEqual(by["example-primary"]["kind"], "key")
        self.assertEqual(by["daemon"]["kind"], "section")
        self.assertEqual(by["daemon.auto_refresh"]["kind"], "key")
        self.assertEqual(by["route"]["kind"], "rule")
        self.assertEqual(by["verb"]["kind"], "rule")
        self.assertNotIn("profiles", by)
        self.assertEqual(by["example-primary"]["line"], 7)
        self.assertEqual(by["example-primary"]["end"], 11)
        self.assertEqual(
            sorted((r["from"], r["to"]) for r in refs),
            [("daemon", "example-alt"), ("decide", "example-local"), ("default_profile", "example-primary")],
        )

    def test_dangling_default_profile_is_no_reference(self):
        _defs, refs = load("layered")
        self.assertEqual([(r["from"], r["to"]) for r in refs], [("daemon", "example-main")])

    def test_multiline_array_extent(self):
        src = 'a = 1\n[t]\nk = [\n  "x",\n  "y",\n]\nz = 2\n'
        defs, _ = o.entities("c", tomllib.loads(src), src)
        by = {d["name"]: d for d in defs}
        self.assertEqual((by["t.k"]["line"], by["t.k"]["end"]), (3, 6))
        self.assertEqual((by["t"]["line"], by["t"]["end"]), (2, 7))


if __name__ == "__main__":
    unittest.main()
