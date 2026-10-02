"""Tests for parliament_oracle.py: run with the venv that holds parliament,
`python -m unittest test_parliament_oracle` from this directory."""

import json
import os
import tempfile
import unittest

import parliament_oracle as po


POLICY = """{
  "Version": "2012-10-17",
  "Statement": [
    {"Sid": "Read", "Effect": "Allow", "Action": ["s3:GetObject"], "Resource": "arn:aws:s3:::b/*"},
    {"Effect": "Deny", "NotAction": "s3:Put*", "Principal": {"AWS": ["arn:aws:iam::111122223333:root"], "Service": "x.amazonaws.com"}, "NotResource": "*"}
  ]
}
"""


class OracleTest(unittest.TestCase):
    def run_oracle(self, files):
        with tempfile.TemporaryDirectory() as d:
            for rel, body in files.items():
                os.makedirs(os.path.dirname(os.path.join(d, rel)) or d, exist_ok=True)
                with open(os.path.join(d, rel), "w") as f:
                    f.write(body)
            out = os.path.join(d, "out.json")
            po.main(["-subject", d, "-out", out, "-no-analyze"])
            return json.load(open(out))

    def test_definitions_and_references(self):
        got = self.run_oracle({"p/policy.json": POLICY, "p/other.json": '{"Version": "1.0", "Statement": []}',
                               "p/bad.json": "{not json"})
        defs = {(x["name"], x["kind"]): x["line"] for x in got["definitions"]}
        self.assertEqual(defs, {("policy.json", "policy"): 1, ("Read", "rule"): 4, ("1", "rule"): 5})
        refs = {(r["from"], r["to"]) for r in got["references"]}
        self.assertEqual(refs, {("Read", "s3:GetObject"), ("Read", "arn:aws:s3:::b/*"), ("1", "s3:Put*"),
                                ("1", "arn:aws:iam::111122223333:root"), ("1", "x.amazonaws.com"), ("1", "*")})
        self.assertEqual(got["tier"], "C")


if __name__ == "__main__":
    unittest.main()
