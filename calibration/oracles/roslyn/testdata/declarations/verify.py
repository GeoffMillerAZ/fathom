#!/usr/bin/env python3
"""Verify actual native exporter output; never manufactures oracle answers.

Usage: verify.py valid|error output.json
These source regression checks are unexecuted until the native exporter runs.
They are not a language-grade scorer.
"""
import hashlib
import json
from pathlib import Path
import sys


def verify(mode, output):
    fixture = Path(__file__).resolve().parent
    expected = json.loads((fixture / "expected.json").read_text())
    data = json.loads(Path(output).read_text())
    assert data["schema"] == expected["schema"]
    assert not data["workspace_failures"], data["workspace_failures"]
    rows = data["declarations"]
    assert rows, "No native declaration rows"
    for file in data["files"]:
        source = Path(data["repository_root"]) / file["path"]
        assert file["source_sha256"] == hashlib.sha256(source.read_bytes()).hexdigest()
    if mode == "error":
        diagnostics = "\n".join(error["message"] for error in data["compile_errors"])
        for code in expected["expected_error_diagnostics"]:
            assert code in diagnostics, (code, diagnostics)
        assert any(row["name_value"] == "Invalid" for row in rows)
        print("PASS native error rows and explicit compilation diagnostics (not a grade)")
        return
    assert mode == "valid"
    assert not data["compile_errors"], data["compile_errors"]
    assert len(data["files"]) == 2, data["files"]
    main = [row for row in rows if row["path"].endswith("/Declarations.cs.input")]

    def named(name):
        hits = [row for row in main if row["name_text"] == name]
        assert len(hits) == 1, (name, len(hits))
        return hits[0]

    # Later declarators retain their own name and definition position while
    # preserving the larger shared statement as independently exposed evidence.
    for name in ("first", "second", "third", "Ready", "Finished"):
        row = named(name)
        assert row["name_span"]["start_line"] == expected["name_lines"][name], row
        assert row["definition_span"]["start_line"] == expected["name_lines"][name], row
    assert named("second")["statement_span"]["start_line"] == 6
    assert named("second")["statement_span"]["end_line"] == 7
    assert named("Finished")["statement_span"]["start_line"] == 8
    assert named("Finished")["semantic_kind"] == "Event"
    assert named("@event")["name_value"] == "event"
    assert named("café")["name_value"] == "café"
    assert named("unicodeAfterEmoji")["name_span"]["start_column_utf16"] == expected["unicode_identifier_column_utf16"]
    assert named("this")["raw_kind"] == "indexer"
    assert named("Compute")["signature_normalized_candidate"] == expected["expression_signature"]
    assert named("Compute")["name_span"]["start_line"] == expected["name_lines"]["Compute"]
    assert named("Compute")["definition_span"]["start_line"] == expected["name_lines"]["Compute"] - 1
    bodyless = named("Bodyless")
    assert bodyless["signature_normalized_candidate"] == expected["bodiless_signature"]
    assert "void Bodyless(int value)" in bodyless["raw_header"]
    assert bodyless["name_span"]["start_line"] == expected["name_lines"]["Bodyless"]
    long_method = named(expected["long_method_name"])
    signature = long_method["signature_normalized_candidate"]
    assert len(signature) == 200 and signature.endswith("…"), signature
    assert expected["expected_active"] in {row["name_text"] for row in main}
    assert expected["expected_absent"] not in {row["name_text"] for row in main}
    assert "FIXTURE_ACTIVE" in next(file for file in data["files"] if file["path"].endswith("/Declarations.cs.input"))["preprocessor_symbols"]
    types = [row for row in rows if row["name_text"] == "Host" and row["raw_kind"] == "class"]
    assert len(types) == 2
    assert len({row["documentation_id"] for row in types}) == 1
    assert all(row["metadata_name"] == "Host`1" and len(row["declaring_syntax_references"]) == 2 for row in types)
    assert all(row["containing_namespace"] == "Fixture.Outer" for row in types)
    assert any(row["raw_kind"] == "operator" and row["method_kind"] == "UserDefinedOperator" for row in rows)
    assert any(row["raw_kind"] == "conversionOperator" and row["method_kind"] == "Conversion" for row in rows)
    assert any(row["raw_kind"] == "destructor" and row["method_kind"] == "Destructor" for row in rows)
    assert {"record", "recordStruct", "delegate", "enum", "enumMember", "struct"} <= {row["raw_kind"] for row in rows}
    assert all(not row["signature_contract_applies"] for row in rows if row["raw_kind"] in expected["unsupported_for_product_signature"])
    assert all(row["binding_status"] == "resolved" for row in rows)
    print("PASS native spans, containment, partial identity, signature contract, active configuration and raw-kind regressions (not a grade)")


if __name__ == "__main__":
    if len(sys.argv) != 3:
        raise SystemExit(__doc__)
    verify(sys.argv[1], sys.argv[2])
