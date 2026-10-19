#!/usr/bin/env python3
"""Future native controls; no stubs or Fathom answers construct expected truth.

materialize NEW_ROOT writes only this fixture's static files into a new directory.
MODE RAW_JSON ROOT checks output from the actual Roslyn DLL, run separately
under an owned compiler window. This verifier never launches a compiler/restore.
Positive/negative native exits must also be recorded by the external runner.
"""
import hashlib
import json
from pathlib import Path
import sys

FIXTURE = Path(__file__).with_name("fixture.json")


def materialize(destination):
    destination = Path(destination)
    if destination.exists() or destination.is_symlink():
        raise ValueError("Fixture root must be new; no overwrite")
    data = json.loads(FIXTURE.read_text())
    destination.mkdir(parents=True)
    for name, text in data["files"].items():
        parts = name.split("/")
        if name.startswith("/") or any(p in ("", ".", "..") for p in parts):
            raise ValueError("Noncanonical static fixture path")
        target = destination.joinpath(*parts)
        target.parent.mkdir(parents=True, exist_ok=True)
        target.write_bytes(text.encode())
    (destination / "NuGet.Config").write_text(
        '<configuration><packageSources><clear /></packageSources>'
        '<auditSources><clear /></auditSources></configuration>\n'
    )
    print(json.dumps({"materialized_files": len(data["files"]), "root": str(destination.resolve())}))


def check(mode, output, root):
    data = json.loads(Path(output).read_text())
    root = Path(root).resolve()
    assert data["schema"] == "fathom.roslyn.project-imports/v1"
    assert Path(data["repository_root"]).resolve() == root
    assert "file_edges" not in data and "imports" not in data
    assert not data["workspace_failures"], data["workspace_failures"]
    expected = json.loads(FIXTURE.read_text())["files"]
    native_hashes = {row["path"]: row["sha256"] for row in data["native_input_hashes"]}
    for file in data["files"]:
        path = file["path"]
        assert path in expected, path
        digest = hashlib.sha256(expected[path].encode()).hexdigest()
        assert file["source_sha256"] == digest
        assert native_hashes[str(root / path)] == digest
    rows = data["usings"]
    assert len({(r["project"], r["path"], r["span"]["start_offset"]) for r in rows}) == len(rows)

    def find(path, name, alias=None, static=False, global_using=False):
        found = [r for r in rows if r["path"] == path and r["written_name"] == name
                 and r["alias"] == alias and r["is_static"] == static and r["is_global"] == global_using]
        assert len(found) == 1, (path, name, alias, len(found))
        return found[0]

    if mode.startswith("metadata"):
        assert not data["compile_errors"], data["compile_errors"]
        path = "src/MetadataConsumer/Consumer.cs.input"
        selected = (find(path, "BoundNamespace"), find(path, "BoundNamespace.Known", alias="Alias"),
                    find(path, "BoundNamespace.Known", static=True))
        project = next(p for p in data["projects"] if p["name"] == "MetadataConsumer")
        binary = root / "binary/NativeDependency.dll"
        references = [r for r in project["native_references"] if r["path"] and Path(r["path"]).resolve() == binary]
        assert len(references) == 1 and references[0]["reference_kind"] == "portable-executable"
        assert project["project_references"] == [], "Do not fake source ProjectReferences for this native control"
        assert native_hashes[str(binary)] == hashlib.sha256(binary.read_bytes()).hexdigest()
        for row in selected:
            if mode == "metadata":
                assert row["status"] == "source" and row["source_contribution_mapping_complete"]
                assert set(row["all_source_target_paths"]) == {"src/ExactSource/First.cs.input", "src/ExactSource/Second.cs.input"}
                assert all("DifferentIdentity" not in t for t in row["all_source_target_paths"])
            elif mode == "metadata-wrong-identity":
                assert row["status"] == "external_metadata" and not row["all_source_target_paths"]
                assert row["source_contribution_mapping_complete"]
            else:
                assert mode == "metadata-duplicate-identity"
                assert row["status"] == "native_mapping_unknown" and not row["all_source_target_paths"]
                assert row["source_contribution_mapping_complete"] is False
                assert any(len(c["source_identity_candidates"]) == 2 for c in row["contributions"])
    elif mode == "negative":
        assert data["compile_errors"], "Native compile failures must remain failures"
        path = "src/Error/Broken.cs.input"
        assert find(path, "Missing.Active")["status"] == "unresolved"
        ambiguous = find(path, "Clash.Duplicate", alias="Conflict")
        assert ambiguous["status"] == "ambiguous"
        assert ambiguous["errors"] and len(ambiguous["candidates"]) >= 2
        invalid = find(path, "Shared", static=True)
        assert invalid["status"] == "invalid_statement" and invalid["errors"]
        inaccessible = find(path, "Shared.Hidden", alias="Hidden")
        assert inaccessible["status"] == "inaccessible" and inaccessible["errors"]
        for row in (ambiguous, invalid, inaccessible, find(path, "Missing.Active")):
            assert not row["all_source_target_paths"]
            assert row["source_contribution_mapping_complete"] is False
    else:
        assert not data["compile_errors"], data["compile_errors"]
        assert all(r["source_contribution_mapping_complete"] for r in rows), rows
        if mode == "global":
            path = "src/Global/Global.cs.input"
            for row in (find(path, "Shared", global_using=True),
                        find(path, "Shared.Tools", alias="G", global_using=True)):
                assert row["scope"] == "compilation" and row["status"] == "source"
            projects = [p for p in data["projects"] if p["name"] == "Global"]
            assert len(projects) == 1 and projects[0]["language_version"] == "CSharp10"
            assert any(f["path"] == "src/Global/Other.cs.input" for f in data["files"])
        else:
            path = "src/Consumer/Importer.cs.input"
            expected_shared = {
                "src/A/OddPlace.cs.input", "src/A/SecondPartial.cs.input", "src/A/Child.cs.input",
                "src/B/NothingLikeNamespace.cs.input", "src/Consumer/SameNamespace.cs.input",
            }
            wildcard = find(path, "Shared")
            namespace_alias = find(path, "global::Shared", alias="NS")
            for row in (wildcard, namespace_alias):
                assert row["status"] == "source"
                assert row["bound_symbol"]["documentation_id"] == "N:Shared"
                assert set(row["all_source_target_paths"]) == expected_shared
                assert not row["is_global"], "global:: qualification is not global using"
            partial = {"src/A/OddPlace.cs.input", "src/A/SecondPartial.cs.input"}
            for row in (find(path, "Shared.Tools", alias="T"), find(path, "Shared.Tools", static=True),
                        find(path, "Shared.Tools", alias="Local")):
                assert set(row["all_source_target_paths"]) == partial
                assert row["bound_symbol"]["kind"] == "NamedType"
                assert row["bound_symbol"]["documentation_id"] == "T:Shared.Tools"
            assert find(path, "Shared.Tools", alias="Local")["scope"] == "namespace"
            assert find(path, "System")["status"] == "mixed_source_metadata"
            assert set(find(path, "System")["all_source_target_paths"]) == {"src/B/MixedSystem.cs.input"}
            for external in (find(path, "System.Collections.Generic"),
                             find(path, "System.Text.StringBuilder", alias="Ext")):
                assert external["status"] == "external_metadata" and not external["all_source_target_paths"]
            assert set(find(path, "Parent")["all_source_target_paths"]) == {"src/A/ParentChild.cs.input"}
            assert set(find(path, "NeverUsed")["all_source_target_paths"]) == {"src/A/Unused.cs.input"}
            self_only = find("src/Consumer/SelfOnly.cs.input", "SelfOnly")
            assert self_only["target_scope"] == "selected_self_only"
            assert not self_only["measured_cross_file_target_paths"]
            assert not any("Missing.Inactive" == r["written_name"] for r in rows)
            source = [f for f in data["files"] if f["path"] == path]
            assert len(source) == 1 and source[0]["inactive_regions"]
            assert not any("Unrelated" in p for r in rows if r["path"].startswith("src/Consumer/")
                           for p in r["all_source_target_paths"])
            consumer = [p for p in data["projects"] if p["name"] == "Consumer"]
            assert len(consumer) == 1 and consumer[0]["language_version"] == "CSharp9"
            assert {Path(p["path"]).stem for p in consumer[0]["project_references"]} == {"A", "B"}
            assert any(p["name"] == "Unrelated" for p in data["projects"])
            if mode == "narrow":
                assert data["source_directory"] == "src/Consumer"
                assert find(path, "Shared.Tools", alias="T")["target_scope"] == "outside_emission_selection"
                assert find(path, "Shared.Tools", alias="T")["selected_target_paths"] == []
                assert all(f["path"].startswith("src/Consumer/") for f in data["files"])
            else:
                assert mode == "positive" and data["source_directory"] == "src"
    print(json.dumps({"mode": mode, "native_files": len(data["files"]), "native_usings": len(rows),
                      "native_compile_errors": len(data["compile_errors"]), "checks": "PASS",
                      "scope": "Raw native fixture assertions only; no Fathom reading, dependency or language grade"}))


if __name__ == "__main__":
    if len(sys.argv) == 3 and sys.argv[1] == "materialize":
        materialize(sys.argv[2])
    elif len(sys.argv) == 4 and sys.argv[1] in ("positive", "negative", "narrow", "global", "metadata", "metadata-wrong-identity", "metadata-duplicate-identity"):
        check(sys.argv[1], sys.argv[2], sys.argv[3])
    else:
        raise SystemExit("usage: verify.py materialize NEW_ROOT | MODE RAW_JSON ROOT; modes: positive, negative, narrow, global, metadata, metadata-wrong-identity, metadata-duplicate-identity")
