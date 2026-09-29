"""Self-test of cfn_oracle.py: run with cfn-lint's interpreter,
`<cfn-lint libexec>/bin/python -m unittest test_cfn_oracle` from this directory."""

import json
import os
import tempfile
import unittest

import cfn_oracle

TEMPLATE = """AWSTemplateFormatVersion: "2010-09-09"
Parameters:
  Env:
    Type: String
Conditions:
  IsProd: !Equals [!Ref Env, prod]
Resources:
  Queue:
    Type: AWS::SQS::Queue
    Condition: IsProd
  Bucket:
    Type: AWS::S3::Bucket
    DependsOn: Queue
    Properties:
      Name: !Sub "${Env}-${Queue.QueueName}"
      Arn: !GetAtt Queue.Arn
      Region: !Ref AWS::Region
      Mode: !If [IsProd, a, b]
  Child:
    Type: AWS::CloudFormation::Stack
    Properties:
      TemplateURL: ./child.yaml
Outputs:
  Out:
    Value: !Ref Bucket
"""

CHILD = "Resources:\n  T:\n    Type: AWS::SNS::Topic\n"


def load(path):
    with open(path) as f:
        return json.load(f)


class OracleTest(unittest.TestCase):
    def test_subject(self):
        with tempfile.TemporaryDirectory() as d:
            os.makedirs(os.path.join(d, "stacks"))
            os.makedirs(os.path.join(d, "testdata"))
            for rel, body in {"stacks/main.yaml": TEMPLATE, "stacks/child.yaml": CHILD,
                              "stacks/config.yaml": "Resources: 1\n", "testdata/t.yaml": TEMPLATE,
                              "stacks/bad.yaml": "Resources: [unterminated\n"}.items():
                with open(os.path.join(d, rel), "w") as f:
                    f.write(body)
            out, xref, summ = (os.path.join(d, n) for n in ("o.json", "x.json", "s.json"))
            cfn_oracle.main(["-subject", d, "-out", out, "-xref-out", xref, "-summary-out", summ])
            o = load(out)
            self.assertEqual(o["tier"], "A")
            got = sorted((x["path"], x["name"], x["line"], x["kind"]) for x in o["definitions"])
            self.assertEqual(got, sorted([
                ("stacks/child.yaml", "child.yaml", 1, "package"), ("stacks/child.yaml", "T", 2, "resource"),
                ("stacks/main.yaml", "main.yaml", 1, "package"), ("stacks/main.yaml", "Env", 3, "variable_decl"),
                ("stacks/main.yaml", "IsProd", 6, "rule"), ("stacks/main.yaml", "Queue", 8, "resource"),
                ("stacks/main.yaml", "Bucket", 11, "resource"), ("stacks/main.yaml", "Child", 19, "resource"),
                ("stacks/main.yaml", "Out", 24, "output")]))
            refs = sorted((r["from"], r["to"]) for r in o["references"])
            self.assertEqual(refs, [("Bucket", "IsProd"), ("Bucket", "Queue"), ("Out", "Bucket"), ("Queue", "IsProd")])
            x = load(xref)
            self.assertEqual(x["tier"], "C")
            self.assertEqual(x["cross_references"], [{"path": "stacks/main.yaml", "from": "Child", "to_path": "stacks/child.yaml"}])
            s = load(summ)["stats"]
            self.assertEqual(s["graph_edges_unexplained"], 0)
            self.assertEqual(s["graph_edges_from_sub"], 0)  # Bucket->Queue is also a GetAtt and a DependsOn
            self.assertEqual(s["rejected"], 1)
            self.assertEqual(s["not_templates"], 1)


if __name__ == "__main__":
    unittest.main()
