"""Tests for plan_rows_oracle.py: python3 -m unittest test_plan_rows_oracle (from this directory)."""

import os
import tempfile
import unittest

import plan_rows_oracle as o


class ParserTest(unittest.TestCase):
    def test_row_with_block(self):
        rows = o.parse_plan("- [x] A title {id: A-1, symbol: Foo, package: pkg/x, needs: \"A-2,b.md#B-1\"}\n")
        self.assertEqual(len(rows), 1)
        r = rows[0]
        self.assertEqual((r["id"], r["line"], r["end"], r["state"]), ("A-1", 1, 1, "x"))
        self.assertEqual(r["ann"]["symbol"], "Foo")
        self.assertEqual(o.needs_of(r), ["A-2", "b.md#B-1"])

    def test_wrapped_row_reads_its_continuation(self):
        text = "- [ ] first\n  title {id: W-1,\n  oracle: lexical}\n- [ ] next {id: W-2}\n"
        rows = o.parse_plan(text)
        self.assertEqual([(r["id"], r["line"], r["end"]) for r in rows], [("W-1", 1, 3), ("W-2", 4, 4)])

    def test_row_with_block_ignores_the_note_under_it(self):
        text = "- [ ] a {id: N-1}\n  {id: ignored}\n"
        self.assertEqual([r["id"] for r in o.parse_plan(text)], ["N-1"])

    def test_quoted_value_keeps_commas_and_braces(self):
        rows = o.parse_plan('- [-] skipped {id: Q-1, disposition: deferred, reason: "a, b {c}", trigger: x}\n')
        self.assertEqual(rows[0]["ann"]["reason"], "a, b {c}")

    def test_row_without_id_has_no_name(self):
        self.assertEqual(o.parse_plan("- [ ] plain item\n")[0]["id"], "")

    def test_references_and_cross_references(self):
        with tempfile.TemporaryDirectory() as d:
            os.makedirs(os.path.join(d, "docs", "plans"))
            open(os.path.join(d, "docs", "plans", "a.md"), "w").write(
                '- [ ] x {id: A-1, needs: "A-2,A-2,B-1,b.md#B-1,b.md#B-9,a.md#A-2"}\n- [ ] y {id: A-2}\n')
            open(os.path.join(d, "docs", "plans", "b.md"), "w").write("- [ ] z {id: B-1}\n")
            open(os.path.join(d, "docs", "plans", "README.md"), "w").write("- [ ] r {id: R-1}\n")
            _plans, defs, refs, xrefs, _anchors = o.definitions_and_refs(d, False)
        self.assertEqual(sorted(x["name"] for x in defs), ["A-1", "A-2", "B-1"])
        self.assertEqual([(r["from"], r["to"]) for r in refs], [("A-1", "A-2")])
        self.assertEqual([(x["from"], x["to_path"], x["to"]) for x in xrefs], [("A-1", "docs/plans/b.md", "B-1")])


if __name__ == "__main__":
    unittest.main()
