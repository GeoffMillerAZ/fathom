#!/usr/bin/env python3
"""Focused tests for the isolated OpenCL native grammar builder."""

import gzip
import hashlib
import importlib.util
import io
import json
from pathlib import Path
import subprocess
import tarfile
import tempfile
import unittest
from unittest import mock


MODULE_PATH = Path(__file__).with_name("build_native.py")
SPEC = importlib.util.spec_from_file_location("opencl_c_build_native", MODULE_PATH)
build_native = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(build_native)


COMMIT = "1" * 40
LICENSE = b"Permission is hereby granted, free of charge\n"


def sha256(data):
    return hashlib.sha256(data).hexdigest()


def candidate(**updates):
    row = {
        "id": "opencl-generated",
        "repository": "tree-sitter/tree-sitter-c",
        "version": "0.24.1",
        "commit": COMMIT,
        "archive_sha256": "0" * 64,
        "license_sha256": sha256(LICENSE),
        "licence": "MIT",
        "entry_point": "tree_sitter_opencl",
        "generate": True,
        "scanner": False,
        "patches": [{
            "path": "patches/opencl/0001-opencl-c-syntax.patch",
            "sha256": "2" * 64,
        }],
    }
    row.update(updates)
    return row


def descriptor(row=None, **updates):
    document = {
        "schema": "opencl-native-grammar-candidates-v1",
        "tree_sitter_cli": "0.26.11",
        "candidates": [row or candidate()],
    }
    document.update(updates)
    return document


def archive(files, *, root="tree-sitter-c-pin"):
    stream = io.BytesIO()
    with tarfile.open(fileobj=stream, mode="w", format=tarfile.PAX_FORMAT,
                      pax_headers={"comment": COMMIT}) as handle:
        root_info = tarfile.TarInfo(root)
        root_info.type = tarfile.DIRTYPE
        handle.addfile(root_info)
        for name, content in files.items():
            info = tarfile.TarInfo(f"{root}/{name}")
            info.size = len(content)
            handle.addfile(info, io.BytesIO(content))
    return gzip.compress(stream.getvalue()), stream.getvalue()


class DescriptorTests(unittest.TestCase):
    def test_selects_one_strictly_pinned_candidate(self):
        with tempfile.TemporaryDirectory() as td:
            path = Path(td) / "grammar-candidates.json"
            path.write_text(json.dumps(descriptor()))
            selected, digest = build_native.load_candidate(path, "opencl-generated")
        self.assertEqual("tree_sitter_opencl", selected["entry_point"])
        self.assertEqual(64, len(digest))

    def test_rejects_invalid_schema_duplicate_id_and_pin(self):
        cases = [
            descriptor(schema="wrong"),
            descriptor(candidates=[candidate(), candidate()]),
            descriptor(candidate(commit="ABC")),
            descriptor(candidate(generate=1)),
            descriptor(candidate(licence="Apache-2.0")),
            descriptor(candidate(patches=[{"path": "../escape.patch", "sha256": "2" * 64}])),
        ]
        for document in cases:
            with self.subTest(document=document):
                with tempfile.TemporaryDirectory() as td:
                    path = Path(td) / "grammar-candidates.json"
                    path.write_text(json.dumps(document))
                    with self.assertRaises(ValueError):
                        build_native.load_candidate(path, "opencl-generated")

    def test_tree_sitter_argument_is_required_only_for_generate(self):
        build_native.validate_tree_sitter_argument(candidate(), None)
        with self.assertRaises(ValueError):
            build_native.require_tree_sitter(candidate(), None)
        self.assertIsNone(build_native.require_tree_sitter(
            candidate(generate=False), None))

    def test_accepts_the_committed_descriptor_shape(self):
        selected, _ = build_native.load_candidate(
            MODULE_PATH.with_name("grammar-candidates.json"), "opencl-c-patched")
        self.assertEqual("tree_sitter_opencl", selected["entry_point"])

    def test_rejects_unknown_descriptor_and_candidate_fields(self):
        cases = [descriptor(extra="value"), descriptor(candidate(extra="value"))]
        for document in cases:
            with self.subTest(document=document), tempfile.TemporaryDirectory() as td:
                path = Path(td) / "grammar-candidates.json"
                path.write_text(json.dumps(document))
                with self.assertRaises(ValueError):
                    build_native.load_candidate(path, "opencl-generated")


class ArchiveTests(unittest.TestCase):
    def test_verifies_sha_commit_extracts_common_root_and_license(self):
        payload, tar_bytes = archive({
            "LICENSE": LICENSE,
            "src/parser.c": b"const TSLanguage *tree_sitter_opencl(void) {}\n",
        })
        row = candidate(archive_sha256=sha256(payload))
        with tempfile.TemporaryDirectory() as td:
            archive_path = Path(td) / "source.tar.gz"
            source = Path(td) / "source"
            archive_path.write_bytes(payload)
            completed = subprocess.CompletedProcess(
                ["git", "get-tar-commit-id"], 0, (COMMIT + "\n").encode(), b"")
            with mock.patch.object(build_native, "run_command", return_value=completed) as run:
                evidence = build_native.verify_and_extract_archive(archive_path, row, source)
        self.assertEqual(sha256(LICENSE), evidence["license_sha256"])
        self.assertEqual(sha256(b"const TSLanguage *tree_sitter_opencl(void) {}\n"),
                         evidence["parser_sha256"])
        self.assertEqual(["git", "get-tar-commit-id"], run.call_args.args[0])
        self.assertEqual(tar_bytes, run.call_args.kwargs["input_bytes"])

    def test_rejects_archive_hash_before_git(self):
        payload, _ = archive({"LICENSE": LICENSE})
        with tempfile.TemporaryDirectory() as td:
            path = Path(td) / "source.tar.gz"
            path.write_bytes(payload)
            with mock.patch.object(build_native, "run_command") as run:
                with self.assertRaises(ValueError):
                    build_native.verify_and_extract_archive(path, candidate(), Path(td) / "out")
            run.assert_not_called()

    def test_rejects_multi_root_and_traversal_tar_members(self):
        for names in ({"one/LICENSE": LICENSE, "two/src/parser.c": b"x"},
                      {"root/../escape": b"x", "root/LICENSE": LICENSE}):
            stream = io.BytesIO()
            with tarfile.open(fileobj=stream, mode="w") as handle:
                for name, content in names.items():
                    info = tarfile.TarInfo(name)
                    info.size = len(content)
                    handle.addfile(info, io.BytesIO(content))
            with self.subTest(names=names), tempfile.TemporaryDirectory() as td:
                with self.assertRaises(ValueError):
                    build_native.extract_common_root(stream.getvalue(), Path(td) / "out")


class BuildHelpersTests(unittest.TestCase):
    def test_resolves_relative_and_path_tools_before_cwd_changes(self):
        with tempfile.TemporaryDirectory() as td:
            caller = Path(td)
            tools = caller / "workspace" / "tools"
            tools.mkdir(parents=True)
            for name in ("tree-sitter", "clang", "clang++"):
                path = tools / name
                path.write_text("#!/bin/sh\n")
                path.chmod(0o755)
            resolved = build_native.preflight_executables(
                candidate(), "workspace/tools/tree-sitter",
                "workspace/tools/clang", "workspace/tools/clang++", caller)
        expected = str((tools / "tree-sitter").resolve())
        self.assertEqual(expected, resolved["tree_sitter"])
        self.assertEqual(str((tools / "clang").resolve()), resolved["cc"])
        self.assertEqual(str((tools / "clang++").resolve()), resolved["cxx"])
        with mock.patch.object(build_native, "run_command", return_value=subprocess.CompletedProcess(
                [], 0, b"tree-sitter 0.26.11\n", b"")) as version:
            build_native.exact_tree_sitter_version(resolved["tree_sitter"])
        self.assertEqual(expected, version.call_args.args[0][0])
        generate = [resolved["tree_sitter"], "generate"]
        corpus = build_native.corpus_test_command(
            resolved["tree_sitter"], "native", "proof", "config.json")
        self.assertEqual(expected, generate[0])
        self.assertEqual(expected, corpus[0])
        self.assertEqual(
            [expected, "test", "--config-path", "config.json", "--lib-path", "native",
             "--lang-name", "proof"], corpus)

    def test_resolves_bare_path_command_and_refuses_nonexecutable(self):
        with tempfile.TemporaryDirectory() as td:
            caller = Path(td)
            executable = caller / "clang"
            executable.write_text("#!/bin/sh\n")
            executable.chmod(0o755)
            with mock.patch.object(build_native.shutil, "which", return_value=str(executable)):
                self.assertEqual(str(executable.resolve()),
                                 build_native.resolve_executable("clang", caller))
            executable.chmod(0o644)
            with self.assertRaises(ValueError):
                build_native.resolve_executable(str(executable), caller)

    def test_refuses_existing_or_non_workspace_output(self):
        with tempfile.TemporaryDirectory() as td:
            root = Path(td)
            (root / "workspace").mkdir()
            existing = root / "workspace" / "existing"
            existing.mkdir()
            with self.assertRaises(ValueError):
                build_native.validate_output(root, existing)
            with self.assertRaises(ValueError):
                build_native.validate_output(root, root / "elsewhere")

    def test_exact_tool_pin_and_test_count_parsing(self):
        completed = subprocess.CompletedProcess([], 0, b"tree-sitter 0.26.11\n", b"")
        with mock.patch.object(build_native, "run_command", return_value=completed):
            self.assertEqual("tree-sitter 0.26.11",
                             build_native.exact_tree_sitter_version("/tools/tree-sitter"))
        wrong = subprocess.CompletedProcess([], 0, b"tree-sitter 0.26.10\n", b"")
        with mock.patch.object(build_native, "run_command", return_value=wrong):
            with self.assertRaises(ValueError):
                build_native.exact_tree_sitter_version("tree-sitter")
        self.assertEqual({"passed": 12, "failed": 0, "total": 12},
                         build_native.parse_test_counts("12 tests passed; 0 tests failed"))
        self.assertEqual({}, build_native.parse_test_counts("all corpus files looked fine"))

    def test_parses_real_tree_sitter_summary_with_ansi_and_checks_total(self):
        output = ("\x1b[32mTotal parses: 135; successful parses: 135; "
                  "failed parses: 0; success percentage: 100.00%; "
                  "average speed: 4621 bytes/ms\x1b[0m")
        self.assertEqual(
            {"total": 135, "passed": 135, "failed": 0, "skipped": 0},
            build_native.parse_test_counts(output))
        self.assertEqual({}, build_native.parse_test_counts(
            "Total parses: 135; successful parses: 134; failed parses: 0"))

    def test_private_environment_preserves_home_and_redirects_xdg(self):
        with tempfile.TemporaryDirectory() as td, \
                mock.patch.dict(build_native.os.environ, {"HOME": "/preserved"}, clear=True):
            environment = build_native._private_environment(Path(td))
        self.assertEqual("/preserved", environment["HOME"])
        self.assertNotIn("/home", environment["HOME"])
        for name in ("XDG_CACHE_HOME", "XDG_CONFIG_HOME", "XDG_DATA_HOME"):
            self.assertTrue(environment[name].startswith(td))

    def test_proof_copy_changes_only_factory_and_names(self):
        parser = b"static int tables[] = {1, 2};\nconst TSLanguage *tree_sitter_opencl(void) { return 0; }\n"
        with tempfile.TemporaryDirectory() as td:
            source = Path(td) / "source"
            proof = Path(td) / "proof"
            (source / "src/tree_sitter").mkdir(parents=True)
            (source / "test/corpus").mkdir(parents=True)
            (source / "src/parser.c").write_bytes(parser)
            (source / "src/grammar.json").write_text(
                '{"extras":[{"type":"SYMBOL","name":"comment"}],'
                '"rules":{},"name":"opencl"}')
            (source / "src/node-types.json").write_text("[]")
            (source / "src/tree_sitter/parser.h").write_text("/* header */")
            (source / "test/corpus/basic.txt").write_text("===\nBasic\n===\n")
            unique = build_native.prepare_proof_tree(source, proof, sha256(parser))
            changed = (proof / "src/parser.c").read_text()
            grammar_text = (proof / "src/grammar.json").read_text()
            config = json.loads((proof / "config.json").read_text())
        self.assertEqual("openclproof" + sha256(parser)[:12], unique)
        self.assertIn(f"tree_sitter_{unique}(void)", changed)
        self.assertIn("static int tables[] = {1, 2};", changed)
        self.assertNotIn("tree_sitter_opencl(void)", changed)
        self.assertTrue(grammar_text.startswith('{"name":"' + unique + '"'))
        self.assertEqual(unique, json.loads(grammar_text)["name"])
        self.assertLess(grammar_text.index('"name":"' + unique + '"'),
                        grammar_text.index('"name":"comment"'))
        self.assertEqual({"parser-directories": []}, config)

    def test_compile_command_uses_pic_optimization_and_scanner_language(self):
        with tempfile.TemporaryDirectory() as td:
            root = Path(td)
            (root / "src").mkdir()
            (root / "src/parser.c").write_text("parser")
            (root / "src/scanner.cc").write_text("scanner")
            out = root / "native.so"
            completed = subprocess.CompletedProcess([], 0, b"", b"")
            with mock.patch.object(build_native, "run_logged", return_value=completed) as run:
                build_native.compile_library(root, out, True, "clang", "clang++", "compile")
            command = run.call_args.args[0]
        self.assertEqual("clang++", command[0])
        self.assertIn("-O2", command)
        self.assertIn("-fPIC", command)
        self.assertIn("-dynamiclib" if build_native.sys.platform == "darwin" else "-shared",
                      command)
        self.assertIn("scanner.cc", " ".join(map(str, command)))


if __name__ == "__main__":
    unittest.main()
