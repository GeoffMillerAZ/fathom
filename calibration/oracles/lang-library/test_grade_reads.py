import unittest

import grade_reads as g

HCL = '''resource "aws_security_group" "a" {
  name = "x"
  dynamic "ingress" {
    for_each = var.rules
    content {
      from_port = "${ingress.value}}"
      description = <<EOT
dynamic "no" {
EOT
    }
  }
  egress {
    to_port = 0
  }
  # dynamic "commented" {
}
'''


class GradeTest(unittest.TestCase):
    def test_dynamic_ranges(self):
        self.assertEqual(g.dynamic_ranges(HCL), [(3, 11)])

    def test_in_schema(self):
        blk = {"attrs": ["name", "tags"], "blocks": {"ingress": {"attrs": ["from_port"], "blocks": {}}}}
        self.assertTrue(g.in_schema("name", blk))
        self.assertTrue(g.in_schema("ingress[*].from_port", blk))
        self.assertFalse(g.in_schema("depends_on", blk))
        self.assertFalse(g.in_schema("ingress[*]", blk))
        self.assertFalse(g.in_schema("tags.Name", blk))

    def test_folded_form(self):
        for s in ('true ? "a" : "b"', "!true", "1 + 2", "[for x in [1] : x]", "-var.x"):
            self.assertTrue(g.folded_form(s), s)
        for s in ('upper("x")', 'var.x', '"a?b:c"'):
            self.assertFalse(g.folded_form(s), s)

    def test_agree(self):
        ctx = {"variables": {"x": {"has_default": True}, "y": {"has_default": False}}}
        o = lambda c, refs=(), d=False: {"class": c, "refs": list(refs), "defaultable": d}
        self.assertTrue(g.agree("terraform", "literal", o("constant"), ctx))
        self.assertFalse(g.agree("terraform", "literal", o("references"), ctx))
        self.assertTrue(g.agree("terraform", "reference", o("references"), ctx))
        self.assertTrue(g.agree("terraform", "defaulted", o("references", ["var.x"]), ctx))
        self.assertFalse(g.agree("terraform", "defaulted", o("references", ["var.y"]), ctx))
        self.assertFalse(g.agree("terraform", "defaulted", o("references", ["aws_s3_bucket.b.id"]), ctx))
        self.assertIsNone(g.agree("terraform", "defaulted", o("references", ["local.l", "var.x"]), ctx))
        self.assertTrue(g.agree("terraform", "expression", o("empty"), ctx))
        self.assertTrue(g.agree("terraform", "expression", o("constant"), {"variables": {}, "rhs": ' true ? "a" : "b"'}))
        self.assertFalse(g.agree("terraform", "expression", o("constant"), {"variables": {}, "rhs": ' upper("x")'}))
        self.assertFalse(g.agree("terraform", "unknown", o("references"), ctx))
        self.assertTrue(g.agree("cloudformation", "defaulted", o("references", d=True), ctx))
        self.assertFalse(g.agree("cloudformation", "defaulted", o("references", d=False), ctx))
        self.assertFalse(g.agree("cloudformation", "expression", o("constant"), ctx))
        self.assertTrue(g.agree("pulumi-yaml", "unknown", o("unresolved"), ctx))


if __name__ == "__main__":
    unittest.main()
