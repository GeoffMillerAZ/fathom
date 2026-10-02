"""Tests for custodian_oracle.py (the pure helpers; c7n itself runs in the oracle):
`python -m unittest test_custodian_oracle` from this directory, in the venv that holds c7n."""

import unittest

import custodian_oracle as co

YAML = """policies:
  - name: a
    resource: aws.s3
  - name: b
    resource: ec2
    filters:
      - or:
          - type: value
"""


class HelperTest(unittest.TestCase):
    def test_policy_lines(self):
        self.assertEqual(co.policy_lines(YAML), [("a", 2, 3), ("b", 4, 8)])
        self.assertIsNone(co.policy_lines("- a\n- b\n"))

    def test_types_of(self):
        out = []
        co.types_of([{"type": "value"}, "stop", {"or": [{"type": "x"}, {"and": ["y"]}]}, {"tag:a": "absent"}], None, out)
        self.assertEqual(out, ["value", "stop", "x", "y"])
        out = []
        co.types_of([{"type": "known"}, {"type": "unknown"}], {"known": 1}, out)
        self.assertEqual(out, ["known"])

    def test_type_map_is_sorted_and_has_both_spellings(self):
        m = co.type_map()
        self.assertEqual(m["aws.s3"], "AWS::S3::Bucket")
        self.assertEqual(m["s3"], "AWS::S3::Bucket")
        self.assertEqual(list(m), sorted(m))


if __name__ == "__main__":
    unittest.main()
