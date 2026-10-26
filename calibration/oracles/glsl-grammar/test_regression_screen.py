from __future__ import annotations

import hashlib
import importlib.util
import json
import os
from pathlib import Path
import subprocess
import tempfile
import unittest
from unittest import mock

MODULE_PATH = Path(__file__).with_name("regression_screen.py")
SPEC = importlib.util.spec_from_file_location("regression_screen", MODULE_PATH)
assert SPEC is not None and SPEC.loader is not None
screen = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(screen)


class Node:
    def __init__(
        self, kind="translation_unit", *, error=False, missing=False, children=(),
        start_byte=0, end_byte=0,
    ):
        self.type = kind
        self.is_error = error
        self.is_missing = missing
        self.children = list(children)
        self.is_named = True
        self.start_byte = start_byte
        self.end_byte = end_byte
        self.start_point = (1, 2)
        self.end_point = (1, 3)


class Parser:
    def __init__(self, outcomes):
        self.outcomes = outcomes

    def parse(self, data):
        nodes = self.outcomes.get(data, ())
        return type("Tree", (), {
            "root_node": Node(children=nodes, start_byte=0, end_byte=len(data))
        })()


class RegressionScreenTest(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory(dir="workspace/tmp")
        self.base = Path(self.temp.name)
        self.repo = self.base / "subject"
        self.repo.mkdir()
        subprocess.run(["git", "init", "-q", str(self.repo)], check=True)
        subprocess.run(["git", "-C", str(self.repo), "config", "user.name", "Test"], check=True)
        subprocess.run(["git", "-C", str(self.repo), "config", "user.email", "test@example.invalid"], check=True)
        subprocess.run(["git", "-C", str(self.repo), "remote", "add", "origin", "https://example.invalid/subject.git"], check=True)
        self.dialect = self.base / "dialect.json"
        self.dialect.write_text(json.dumps({"match": {
            "extensions": [".frag"],
            "sniff": {"extensions": [".fs"], "pattern": r"^\s*#version", "scan_bytes": 32},
        }}))
        self.lib1 = self.base / "one.dylib"
        self.lib2 = self.base / "two.dylib"
        self.lib1.write_bytes(b"one")
        self.lib2.write_bytes(b"two")

    def tearDown(self):
        self.temp.cleanup()

    def commit(self):
        subprocess.run(["git", "-C", str(self.repo), "add", "."], check=True)
        subprocess.run(["git", "-C", str(self.repo), "commit", "--no-verify", "-qm", "fixture"], check=True)
        return subprocess.check_output(["git", "-C", str(self.repo), "rev-parse", "HEAD"], text=True).strip()

    def test_classification_symlink_and_no_result_filtering_with_deltas(self):
        (self.repo / "good.frag").write_bytes(b"void main() {}")
        (self.repo / "bad.frag").write_bytes(b"precision strange float;")
        (self.repo / "shader.fs").write_bytes(b"#version 330\nvoid main() {}")
        (self.repo / "plain.fs").write_bytes(b"plain text")
        (self.repo / "link.frag").symlink_to("good.frag")
        commit = self.commit()
        parsers = iter([
            Parser({b"precision strange float;": [Node("ERROR", error=True)]}),
            Parser({
                b"void main() {}": [Node("missing", missing=True)],
                b"#version 330\nvoid main() {}": [Node("missing", missing=True)],
                b"precision strange float;": [Node("ERROR", error=True), Node("missing", missing=True)],
            }),
        ])
        out = self.base / "out"
        summary = screen.run(
            self.repo, commit, [("base", self.lib1), ("candidate", self.lib2)],
            self.dialect, out, parser_factory=lambda _: next(parsers),
        )
        self.assertEqual(summary["tracked"]["symlinks"], 1)
        self.assertEqual(summary["eligible"]["files"], 3)
        self.assertEqual(summary["macro_precision_candidate_files"], 1)
        self.assertEqual(summary["grammars"][0]["error_only_files"], 1)
        self.assertEqual(summary["grammars"][1]["missing_only_files"], 2)
        self.assertEqual(summary["grammars"][1]["both_files"], 1)
        self.assertEqual(summary["grammars"][1]["union_files"], 3)
        self.assertEqual(summary["grammars"][1]["new_passing_to_failing_vs_first"], 2)
        rows = (out / "private" / "candidate-results.jsonl").read_text().splitlines()
        self.assertEqual(len(rows), 3)
        self.assertFalse(any("link.frag" in row for row in rows))

    def test_macro_candidate_cohort_is_lexical_not_error_attribution(self):
        # Comments may match. Unrelated errors in a candidate file stay cohort
        # counts, never a claim that its precision statement caused the error.
        data = b"// precision COMMENT_MACRO float;\nprecision\nCUSTOM\nfloat;\nBROKEN"
        (self.repo / "candidate.frag").write_bytes(data)
        commit = self.commit()
        summary = screen.run(
            self.repo, commit, [("base", self.lib1)], self.dialect,
            self.base / "out", parser_factory=lambda _: Parser({data: [Node("ERROR", error=True)]}),
        )
        self.assertEqual(summary["macro_precision_candidate_files"], 1)
        self.assertEqual(summary["grammars"][0]["macro_precision_candidate_files_with_errors"], 1)
        self.assertEqual(len(list(screen.MACRO_PRECISION.finditer(data))), 2)

    def test_precision_recognition_ignores_comments_strings_and_preprocessor(self):
        data = (
            b"// precision COMMENT float;\n"
            b"/* precision BLOCK\nfloat; */\n"
            b"#define DECL precision PREPROC float;\n"
            b'const char *s = "precision STRING float;";\n'
            b"precision /* retained offsets */\n CUSTOM_PRECISION\n float ;\n"
            b"precision highp vec4;\n"
        )
        statements = screen.precision_statements(data)
        self.assertEqual(
            [(row["qualifier"], row["type"], row["qualifier_kind"]) for row in statements],
            [("CUSTOM_PRECISION", "float", "macro"), ("highp", "vec4", "literal")],
        )
        for row in statements:
            self.assertEqual(data[row["start_byte"]:row["end_byte"]].lstrip()[:9], b"precision")

    def test_statement_inspection_uses_original_span_and_ignores_neighbor_error(self):
        data = b"BROKEN; precision\nCUSTOM float; ALSO_BROKEN;"
        statement = screen.precision_statements(data)[0]
        start, end = statement["start_byte"], statement["end_byte"]
        declaration = Node("declaration", start_byte=start, end_byte=end)
        root = Node(children=(
            Node("ERROR", error=True, start_byte=0, end_byte=6),
            declaration,
            Node("ERROR", error=True, start_byte=end + 1, end_byte=len(data)),
        ), start_byte=0, end_byte=len(data))
        result = screen.inspect_statement(root, start, end)
        self.assertEqual(result["state"], "passing")
        self.assertEqual(result["container"]["type"], "declaration")

    def test_split_macro_error_overlaps_statement_even_when_tail_recovers(self):
        data = b"precision CUSTOM float;"
        statement = screen.precision_statements(data)[0]
        start, end = statement["start_byte"], statement["end_byte"]
        root = Node(children=(
            Node("ERROR", error=True, start_byte=start, end_byte=start + len(b"precision")),
            Node("declaration", start_byte=start + len(b"precision "), end_byte=end),
        ), start_byte=0, end_byte=end)
        result = screen.inspect_statement(root, start, end)
        self.assertEqual(result["state"], "unrecognized")
        self.assertFalse(result["recognized"])
        self.assertEqual(result["overlapping_diagnostics"][0]["start_byte"], start)

    def test_exact_declaration_under_outer_error_can_pass(self):
        data = b"precision highp float;"
        statement = screen.precision_statements(data)[0]
        start, end = statement["start_byte"], statement["end_byte"]
        enclosing = Node("ERROR", error=True, start_byte=0, end_byte=end + 5, children=(
            Node("declaration", start_byte=start, end_byte=end),
        ))
        root = Node(children=(enclosing,), start_byte=0, end_byte=end + 5)
        result = screen.inspect_statement(root, start, end)
        self.assertEqual(result["state"], "passing")
        self.assertTrue(result["recognized"])
        self.assertEqual(len(result["overlapping_diagnostics"]), 1)

    def test_statement_swallowed_by_larger_error_is_unrecognized(self):
        data = b"precision highp float;"
        statement = screen.precision_statements(data)[0]
        start, end = statement["start_byte"], statement["end_byte"]
        root = Node(children=(
            Node("ERROR", error=True, start_byte=0, end_byte=end + 4),
        ), start_byte=0, end_byte=end + 4)
        result = screen.inspect_statement(root, start, end)
        self.assertEqual(result["state"], "unrecognized")
        self.assertFalse(result["recognized"])

    def test_missing_node_at_statement_boundary_counts(self):
        data = b"precision highp float;"
        statement = screen.precision_statements(data)[0]
        start, end = statement["start_byte"], statement["end_byte"]
        declaration = Node("declaration", start_byte=start, end_byte=end, children=(
            Node(";", missing=True, start_byte=end, end_byte=end),
        ))
        root = Node(children=(declaration,), start_byte=0, end_byte=end)
        result = screen.inspect_statement(root, start, end)
        self.assertEqual(result["state"], "missing")
        self.assertEqual(result["missing_nodes"], 1)

    def test_injected_grammars_separate_macro_regression_from_literal_support(self):
        data = b"precision CUSTOM float;\nprecision highp float;"
        (self.repo / "precision.frag").write_bytes(data)
        commit = self.commit()
        statements = screen.precision_statements(data)
        macro, literal = statements

        def declaration(statement):
            return Node(
                "declaration", start_byte=statement["start_byte"],
                end_byte=statement["end_byte"],
            )

        parsers = iter((
            Parser({data: [declaration(macro), declaration(literal)]}),
            Parser({data: [
                Node(
                    "ERROR", error=True, start_byte=macro["start_byte"],
                    end_byte=macro["start_byte"] + len(b"precision"),
                ),
                Node(
                    "declaration", start_byte=macro["start_byte"] + len(b"precision "),
                    end_byte=macro["end_byte"],
                ),
                declaration(literal),
            ]}),
            Parser({data: [declaration(macro), declaration(literal)]}),
        ))
        out = self.base / "out"
        summary = screen.run(
            self.repo, commit,
            [("baseline", self.lib1), ("withdrawn", self.lib2), ("corrected", self.lib1)],
            self.dialect, out, parser_factory=lambda _: next(parsers),
        )
        grammar = {row["name"]: row["precision_statement_subtrees"] for row in summary["grammars"]}
        self.assertEqual(summary["precision_statements"]["total"], 2)
        self.assertEqual(summary["precision_statements"]["literal"], 1)
        self.assertEqual(summary["precision_statements"]["macro"], 1)
        self.assertEqual(grammar["baseline"]["macro"]["passing"], 1)
        self.assertEqual(grammar["withdrawn"]["macro"]["unrecognized"], 1)
        self.assertEqual(grammar["corrected"]["macro"]["passing"], 1)
        self.assertTrue(all(row["literal"]["passing"] == 1 for row in grammar.values()))
        private = (out / "private" / "withdrawn-precision-statements.jsonl").read_text().splitlines()
        macro_row = json.loads(private[0])
        self.assertEqual(
            data[macro_row["start_byte"]:macro_row["end_byte"]],
            b"precision CUSTOM float;",
        )

    def test_source_drift_fails_closed(self):
        source = self.repo / "shader.frag"
        source.write_bytes(b"void main() {}")
        commit = self.commit()
        original = screen.scan_grammar

        def drifting(*args, **kwargs):
            result = original(*args, **kwargs)
            source.write_bytes(b"changed")
            return result

        with mock.patch.object(screen, "scan_grammar", side_effect=drifting):
            with self.assertRaisesRegex(screen.ScreenError, "not clean|changed"):
                screen.run(
                    self.repo, commit, [("base", self.lib1)], self.dialect,
                    self.base / "out", parser_factory=lambda _: Parser({}),
                )

    def test_manifest_schema_and_symlink_target_hash(self):
        (self.repo / "shader.frag").write_bytes(b"x")
        (self.repo / "alias").symlink_to("shader.frag")
        self.commit()
        summary, regular, _ = screen.capture_manifest(self.repo, self.base / "receipt")
        lines = (self.base / "receipt.manifest").read_text().splitlines()
        entries = [json.loads(line) for line in lines]
        self.assertTrue(all(len(entry) == 3 for entry in entries))
        alias = next(entry for entry in entries if entry[0] == "alias")
        self.assertEqual(alias, ["alias", "symlink", hashlib.sha256(b"shader.frag").hexdigest()])
        self.assertEqual(summary["files"], 1)
        self.assertEqual([os.fsdecode(path) for path in regular], ["shader.frag"])


if __name__ == "__main__":
    unittest.main()
