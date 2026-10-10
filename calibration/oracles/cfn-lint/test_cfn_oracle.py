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
            self.assertEqual(x["cross_references"], [{"path": "stacks/main.yaml", "from": "Child", "to_path": "stacks/child.yaml",
                                                      "to": "child.yaml", "edge": "nested_stack"}])
            s = load(summ)["stats"]
            self.assertEqual(s["graph_edges_unexplained"], 0)
            self.assertEqual(s["graph_edges_from_sub"], 0)  # Bucket->Queue is also a GetAtt and a DependsOn
            self.assertEqual(s["rejected"], 1)
            self.assertEqual(s["not_templates"], 1)


    def test_cloudwatch_and_handlers(self):
        """LB-F368..LB-F370: the CloudWatch Sub references (tier A), and the
        identity joins and handler bindings (tier C) on the corpus
        internal/fusion/lambdahandler_test.go ingests."""
        files = {
            "infra/app.yaml": "Transform: AWS::Serverless-2016-10-31\nGlobals:\n  Function:\n    Runtime: python3.12\n    CodeUri: functions/\nResources:\n"
            "  Orders:\n    Type: AWS::Serverless::Function\n    Properties:\n      FunctionName: orders-api\n      Handler: orders.app.handler\n"
            "  Web:\n    Type: AWS::Serverless::Function\n    Properties:\n      Runtime: nodejs20.x\n      CodeUri: ts/\n      Handler: app.lambdaHandler\n"
            "    Metadata:\n      BuildMethod: esbuild\n      BuildProperties:\n        EntryPoints:\n          - src/app.ts\n"
            "  Java:\n    Type: AWS::Serverless::Function\n    Properties:\n      Runtime: java21\n      CodeUri: java/\n      Handler: com.ex.App\n"
            "  Img:\n    Type: AWS::Serverless::Function\n    Properties:\n      PackageType: Image\n"
            "  Gone:\n    Type: AWS::Serverless::Function\n    Properties:\n      Handler: orders.app.gone\n"
            "  Topic:\n    Type: AWS::SNS::Topic\n"
            "  Logs:\n    Type: AWS::Logs::LogGroup\n    Properties:\n      LogGroupName: !Sub /aws/lambda/${Orders}\n"
            "  Filter:\n    Type: AWS::Logs::MetricFilter\n    Properties:\n      LogGroupName: /aws/lambda/orders-api\n      FilterPattern: ERROR\n"
            "      MetricTransformations:\n        - MetricNamespace: Shop\n          MetricName: Errors\n          MetricValue: \"1\"\n"
            "  Alarm:\n    Type: AWS::CloudWatch::Alarm\n    Properties:\n      Namespace: Shop\n      MetricName: Errors\n"
            "      Dimensions:\n        - Name: FunctionName\n          Value: !Sub \"${Orders}\"\n"
            "      OKActions:\n        - !Sub [\"${T}-${Topic.TopicName}\", {T: x}]\n",
            "infra/ops.yaml": "AWSTemplateFormatVersion: '2010-09-09'\nResources:\n"
            "  Remote:\n    Type: AWS::CloudWatch::Alarm\n    Properties:\n      Metrics:\n        - Id: m1\n          MetricStat:\n"
            "            Metric:\n              Namespace: Shop\n              MetricName: Errors\n",
            "infra/functions/orders/app.py": "def handler(event, context):\n    return 1\n",
            "infra/ts/src/app.ts": "export const lambdaHandler = async (e: unknown) => 1;\n",
            "infra/java/src/main/java/com/ex/App.java": "package com.ex;\npublic class App {\n  public String handleRequest(Object in, Object c) { return \"\"; }\n}\n",
        }
        with tempfile.TemporaryDirectory() as d:
            for rel, body in files.items():
                os.makedirs(os.path.dirname(os.path.join(d, rel)), exist_ok=True)
                with open(os.path.join(d, rel), "w") as f:
                    f.write(body)
            out, xref, summ = (os.path.join(d, n) for n in ("o.json", "x.json", "s.json"))
            cfn_oracle.main(["-subject", d, "-out", out, "-xref-out", xref, "-summary-out", summ])
            refs = sorted((r["path"], r["from"], r["to"]) for r in load(out)["references"])
            self.assertEqual(refs, [("infra/app.yaml", "Alarm", "Orders"), ("infra/app.yaml", "Alarm", "Topic"),
                                    ("infra/app.yaml", "Logs", "Orders")])
            got = sorted((x["edge"], x["path"], x["from"], x["to_path"], x["to"]) for x in load(xref)["cross_references"])
            self.assertEqual(got, [
                ("handler", "infra/app.yaml", "Java", "infra/java/src/main/java/com/ex/App.java", "App.handleRequest"),
                ("handler", "infra/app.yaml", "Orders", "infra/functions/orders/app.py", "handler"),
                ("handler", "infra/app.yaml", "Web", "infra/ts/src/app.ts", "lambdaHandler"),
                ("log_group", "infra/app.yaml", "Filter", "infra/app.yaml", "orders-api"),
                ("metric", "infra/app.yaml", "Alarm", "infra/app.yaml", "Shop/Errors"),
                ("metric", "infra/ops.yaml", "Remote", "infra/app.yaml", "Shop/Errors")])
            s = load(summ)["stats"]
            self.assertEqual((s["handler_functions"], s["handler_bound"], s["handler_unbound_image"], s["handler_unbound_no-symbol"]), (5, 3, 1, 1))
            self.assertEqual(s["graph_edges_unexplained"], 0)


if __name__ == "__main__":
    unittest.main()
