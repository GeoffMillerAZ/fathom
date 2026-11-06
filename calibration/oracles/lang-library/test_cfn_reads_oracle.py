"""Run with cfn-lint's interpreter: <libexec>/bin/python -m unittest test_cfn_reads_oracle"""
import unittest

import cfn_reads_oracle as c

TPL = {
    "Parameters": {
        "Size": {"Type": "String", "Default": "small"},
        "NoDefault": {"Type": "String"},
        "Ami": {"Type": "AWS::SSM::Parameter::Value<AWS::EC2::Image::Id>", "Default": "/ami/path"},
    },
    "Resources": {
        "Bucket": {
            "Type": "AWS::S3::Bucket",
            "Properties": {
                "BucketName": "fixed",
                "Size": {"Ref": "Size"},
                "Other": {"Ref": "NoDefault"},
                "Image": {"Ref": "Ami"},
                "Region": {"Ref": "AWS::Region"},
                "Arn": {"Fn::GetAtt": ["Other", "Arn"]},
                "Joined": {"Fn::Join": ["-", ["a", "b"]]},
                "SubConst": {"Fn::Sub": "no variables ${!Literal}"},
                "SubMapConst": {"Fn::Sub": ["${a}", {"a": "x"}]},
                "SubRef": {"Fn::Sub": "x-${Size}"},
                "Tags": [{"Key": "k", "Value": "v"}, {"Key": "k2", "Value": {"Ref": "Size"}}],
                "Empty": {},
                "NoList": [],
            },
        },
        "Bare": {"Type": "AWS::SNS::Topic"},
        "Whole": {"Type": "AWS::SNS::Topic", "Properties": {"Fn::If": ["C", {"A": 1}, {"B": 2}]}},
        "Typeless": {"Properties": {}},
    },
}


class CfnTest(unittest.TestCase):
    def test_leaves(self):
        got = c.read_template(TPL)
        self.assertEqual(set(got), {"Bucket", "Bare", "Whole"})
        l = got["Bucket"]["leaves"]
        cl = lambda p: [(x["class"], x["defaultable"]) for x in l[p]]
        self.assertEqual(cl("BucketName"), [("constant", False)])
        self.assertEqual(cl("Size"), [("references", True)])
        self.assertEqual(cl("Other"), [("references", False)])
        self.assertEqual(cl("Image"), [("references", False)])
        self.assertEqual(cl("Region"), [("references", False)])
        self.assertEqual(cl("Arn"), [("references", False)])
        self.assertEqual(cl("Joined"), [("empty", False)])
        self.assertEqual(cl("SubConst"), [("constant", False)])
        self.assertEqual(cl("SubMapConst"), [("constant", False)])
        self.assertEqual(cl("SubRef"), [("references", True)])
        self.assertEqual(cl("Tags[*].Key"), [("constant", False), ("constant", False)])
        self.assertEqual(cl("Tags[*].Value"), [("constant", False), ("references", True)])
        self.assertEqual(cl("Empty"), [("constant", False)])
        self.assertEqual(cl("NoList"), [("constant", False)])
        self.assertEqual(got["Bare"]["leaves"], {})
        self.assertEqual(list(got["Whole"]["leaves"]), [""])


if __name__ == "__main__":
    unittest.main()
