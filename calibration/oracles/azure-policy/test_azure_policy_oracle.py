"""Tests for azure_policy_oracle.py: `python -m unittest test_azure_policy_oracle` from this directory."""

import json
import os
import tempfile
import unittest

import azure_policy_oracle as ao

DEFINITION = """{
  "properties": {
    "parameters": {"effect": {"type": "String"}, "unused": {"type": "String"}},
    "policyRule": {
      "if": {"field": "type", "equals": "Microsoft.Web/sites"},
      "then": {"effect": "[parameters('effect')] [parameters('undeclared')]"}
    }
  }
}
"""
INITIATIVE = '{"properties": {"policyDefinitions": [{"policyDefinitionReferenceId": "a", "policyDefinitionId": "/x"}, {"policyDefinitionId": "/y"}]}}'
BARE = '{"if": {"field": "type", "equals": "Microsoft.Web/sites"},\n "then": {"effect": "deny"}}'


class OracleTest(unittest.TestCase):
    def test_walk(self):
        with tempfile.TemporaryDirectory() as d:
            for rel, body in {"d.json": DEFINITION, "i.json": INITIATIVE, "b.json": BARE,
                              "params.json": '{"effect": {"type": "String"}}', "broken.json": "{"}.items():
                with open(os.path.join(d, rel), "w") as f:
                    f.write(body)
            out = os.path.join(d, "out.json")
            ao.main(["-subject", d, "-out", out])
            got = json.load(open(out))
        defs = {(x["path"], x["name"], x["kind"]): (x["line"], x["end"]) for x in got["definitions"]}
        self.assertEqual(defs[("d.json", "d.json", "policy")], (2, 8))
        self.assertEqual(defs[("d.json", "effect", "variable_decl")], (3, 3))
        self.assertIn(("d.json", "[parameters('effect')] [parameters('undeclared')]", "rule"), defs)
        self.assertEqual(defs[("i.json", "a", "key")][0], 1)
        self.assertIn(("i.json", "/y", "key"), defs)
        self.assertIn(("b.json", "deny", "rule"), defs)
        self.assertFalse([k for k in defs if k[0] in ("params.json", "broken.json")])
        self.assertEqual([(r["path"], r["to"]) for r in got["references"]], [("d.json", "effect")])


if __name__ == "__main__":
    unittest.main()
