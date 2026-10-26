"""Adapter unit checks. AST excerpt is historical glslang issue #1146, not 16.6.0 proof."""
import importlib.util
import hashlib
import json
import tempfile
import unittest
from pathlib import Path
from unittest import mock

spec = importlib.util.spec_from_file_location("glslang_ast", Path(__file__).with_name("glslang_ast.py"))
adapter = importlib.util.module_from_spec(spec)
spec.loader.exec_module(adapter)
extract_spec = importlib.util.spec_from_file_location("extract_builtins", Path(__file__).with_name("extract_builtins.py"))
extractor = importlib.util.module_from_spec(extract_spec)
extract_spec.loader.exec_module(extractor)


class AdapterTests(unittest.TestCase):
    def test_historical_ast_excerpt_prelinked_only(self):
        # KhronosGroup/glslang#1146 shows these Function Definition/Call forms.
        output = """0:4 Function Definition: main( ( global void)
0:6   Function Call: getColor( ( global 4-component vector of float)
Linked vertex stage:
0:9 Function Definition: shouldNotCount( ( global void)
"""
        defs, sites = adapter.parse_ast(output, "shaders/demo.vert")
        self.assertEqual([d["name"] for d in defs], ["main"])
        self.assertEqual(sites, [("main", "getColor")])

    def test_actual_glslang_16_6_0_original_fixtures(self):
        expected = {"calls.frag.ast": [("shade", "luminance"), ("main", "shade")],
                    "shapes.vert.ast": [("main", "diffuse")],
                    "interface.comp.ast": [("main", "clampValue")]}
        for filename, pairs in expected.items():
            with self.subTest(filename=filename):
                output = (Path(__file__).parent / "testdata" / filename).read_text()
                definitions, sites = adapter.parse_ast(output, filename)
                self.assertGreaterEqual(len(definitions), 2)
                self.assertEqual(sites, pairs)

    def test_actual_compiled_ast_prelinked_only(self):
        output = (Path(__file__).parent / "testdata" / "include_main.frag.ast").read_text()
        definitions, sites = adapter.parse_ast(output, "include_main.frag")
        self.assertEqual([d["name"] for d in definitions], ["includedScale", "main"])
        self.assertEqual(sites, [("main", "includedScale")])

    def test_location_prefix_tree_indentation(self):
        output = """0:1 Function Definition: helper(\n0:2 Function Definition: main(\n0:3   Function Call: helper(\n"""
        _, sites = adapter.parse_ast(output, "x.vert")
        self.assertEqual(sites, [("main", "helper")])

    def test_ast_requires_definition(self):
        with self.assertRaisesRegex(ValueError, "no Function Definition"):
            adapter.parse_ast("Linked vertex stage:\n", "a.vert")

    def test_malformed_call_line_fails_closed(self):
        with self.assertRaisesRegex(ValueError, "unrecognized"):
            adapter.parse_ast("0:4 Function Definition: main(\n Function Call: ???\n", "a.vert")

    def test_subject_invocation_flags(self):
        root = Path("/tmp/subject")
        artifact = Path("/tmp/oracle/artifacts/a.spv")
        cases = [
            ("vulkan", "shaders/glsl/rayquery/a.frag", ["-V", "--target-env", "vulkan1.2"],
             {"flags": ["-V", "--target-env", "vulkan1.2"]}),
            ("vulkan", "shaders/glsl/foo/a.mesh", ["-V", "--target-env", "spirv1.4"],
             {"flags": ["-V", "--target-env", "spirv1.4"]}),
            ("spirv-cross", "shaders/a.spv16.frag", ["--amb", "--target-env", "vulkan1.3", "-V"],
             {"flags": ["--amb", "--target-env", "vulkan1.3", "-V"]}),
            ("vulkan-samples", "shaders/foo/glsl/a.vert", ["-V", "-I/tmp/subject/shaders/includes/glsl"],
             {"declaration_status": "declared", "cmake_declarations": [{"glslc_additional_arguments": []}]}),
        ]
        for kind, rel, flags, recipe in cases:
            with self.subTest(kind=kind, rel=rel):
                command, reason = adapter.invocation(kind, rel, Path("/tmp/glslangValidator"), root, artifact, recipe)
                self.assertIsNone(reason)
                self.assertEqual(command[1:2], ["-i"])
                self.assertEqual(command[2:2 + len(flags)], flags)
                self.assertEqual(command[-3:], ["-o", str(artifact), str(root / rel)])

    def test_unknown_stage_receipt_reason(self):
        command, reason = adapter.invocation("vulkan", "include/shared.glsl", Path("/tmp/glslangValidator"), Path("/tmp/subject"))
        self.assertIsNone(command)
        self.assertEqual(reason, "fragment_or_unknown_stage")

    def test_glslc_spirv_target_flag_translation(self):
        recipe = {"declaration_status": "declared", "cmake_declarations": [
            {"glslc_additional_arguments": ["--target-spv=spv1.4"]}]}
        command, reason = adapter.invocation("vulkan-samples", "shaders/foo/glsl/a.mesh",
                                             Path("/tmp/glslang"), Path("/tmp/subject"), None, recipe)
        self.assertIsNone(reason)
        self.assertEqual(command[command.index("--target-env") + 1], "spirv1.4")

    def test_pinned_glslc_implicit_include_extension_translation(self):
        recipe = {"native_compiler": "glslc", "declaration_status": "declared",
                  "cmake_declarations": [{"glslc_additional_arguments": []}]}
        command, reason = adapter.invocation("vulkan-samples", "shaders/foo/a.frag", Path("/tmp/glslang"),
                                             Path("/tmp/subject"), None, recipe, has_include=True)
        self.assertIsNone(reason)
        self.assertIn("-P" + adapter.SHADERC_INCLUDE_PREAMBLE, command)
        tensor = dict(recipe, native_compiler="glslang")
        command, _ = adapter.invocation("vulkan-samples", "shaders/foo/a.frag", Path("/tmp/glslang"),
                                        Path("/tmp/subject"), None, tensor, has_include=True)
        self.assertFalse(any(arg.startswith("-P") for arg in command))

    def test_conflicting_native_targets_are_both_enumerated(self):
        recipe = {"native_compiler": "glslc", "declaration_status": "declared", "cmake_declarations": [
            {"cmake": "samples/a/CMakeLists.txt", "glslc_additional_arguments": []},
            {"cmake": "samples/b/CMakeLists.txt", "glslc_additional_arguments": ["-I .."]}]}
        variants = adapter.invocation_variants("vulkan-samples", "shaders/base.frag", Path("/tmp/glslang"),
                                               Path("/tmp/subject"), Path("/tmp/out.spv"), recipe, False)
        self.assertEqual(len(variants), 2)
        self.assertNotEqual(variants[0]["command"], variants[1]["command"])
        self.assertEqual({s for v in variants for s in v["cmake_sources"]},
                         {"samples/a/CMakeLists.txt", "samples/b/CMakeLists.txt"})

    def test_variant_calls_require_all_inputs_then_union(self):
        details = [{"status": "parsed"}, {"status": "parsed"}]
        results = [([], [("main", "one")], {"main": True}),
                   ([], [("main", "two"), ("header", "skip")], {"main": True, "header": False})]
        self.assertEqual(adapter.validated_variant_calls("a.frag", details, results, set(), set()),
                         {("a.frag", "main", "one"), ("a.frag", "main", "two")})
        with self.assertRaisesRegex(ValueError, "not every"):
            adapter.validated_variant_calls("a.frag", [{"status": "parsed"}, {"status": "compile_failed"}],
                                            results[:1], set(), set())

    def test_below_threshold_removes_stale_oracle_output(self):
        with tempfile.TemporaryDirectory() as dirname:
            out = Path(dirname) / "oracle.json"
            out.write_text('{"stale":true}')
            with self.assertRaisesRegex(ValueError, "below 95%"):
                adapter.publish_oracle(out, {"calls": []}, 94, 100)
            self.assertFalse(out.exists())

    @unittest.skipUnless(Path("workspace/calibration/glsl/vulkan/.git").exists(), "pinned local corpus absent")
    def test_subject_identity_requires_pinned_head(self):
        root = Path("workspace/calibration/glsl/vulkan").resolve()
        identity = adapter.subject_identity(root, "1cbc8d87b731b438f47dea00a2e083ba7d59051d")
        self.assertEqual(identity["subject_sha"], "1cbc8d87b731b438f47dea00a2e083ba7d59051d")
        self.assertGreater(identity["shader_tree_file_count"], 353)
        with self.assertRaisesRegex(ValueError, "pin mismatch"):
            adapter.subject_identity(root, "0" * 40)

    @unittest.skipUnless(Path("workspace/calibration/glsl/vulkan/.git").exists(), "pinned local corpus absent")
    def test_subject_identity_rejects_untracked_shader_input(self):
        root = Path("workspace/calibration/glsl/vulkan").resolve()
        original_run = adapter.subprocess.run

        def run_with_untracked(args, **kwargs):
            if args[0] == "git" and "--others" in args:
                return mock.Mock(stdout="shaders/injected.glsl\n")
            return original_run(args, **kwargs)

        with mock.patch.object(adapter.subprocess, "run", side_effect=run_with_untracked):
            with self.assertRaisesRegex(ValueError, "untracked shader input"):
                adapter.subject_identity(root, "1cbc8d87b731b438f47dea00a2e083ba7d59051d")

    def test_include_definition_origin_excludes_header_caller(self):
        with tempfile.TemporaryDirectory() as dirname:
            root = Path(dirname)
            header = root / "helper.glsl"
            main = root / "main.frag"
            text = f'#line 1 "{header}"\nfloat helper(float x) {{ return x; }}\n#line 1 "{main}"\nvoid main() {{ helper(1.0); }}\n'
            defs = [{"name": "helper", "line": 1}, {"name": "main", "line": 1}]
            self.assertEqual(adapter.definition_origins(text, main, defs), {"helper": False, "main": True})

    def test_all_discovered_files_accounted_including_fragments(self):
        with tempfile.TemporaryDirectory() as dirname:
            root = Path(dirname)
            for name in ("a.vert", "b.frag", "shared.glsl", "c.asm.frag", "d.invalid.vert", "e.spv"):
                (root / name).touch()
            self.assertEqual([p.name for p in adapter.files(root)], ["a.vert", "b.frag", "shared.glsl"])

    def test_builtin_index_requires_complete_spec_provenance(self):
        with tempfile.TemporaryDirectory() as dirname:
            path = Path(dirname) / "index.json"
            path.write_text('{"spec":"GLSL 4.60","complete":false,"source":"https://registry.khronos.org/OpenGL/specs/gl/GLSLangSpec.4.60.html","functions":["sin"]}')
            with self.assertRaisesRegex(ValueError, "complete"):
                adapter.builtin_index(path)

    def test_compiler_identity_rejects_unpinned_binary(self):
        with tempfile.TemporaryDirectory() as dirname:
            compiler = Path(dirname) / "glslang"
            compiler.write_bytes(b"not the official release binary")
            with self.assertRaisesRegex(ValueError, "binary digest mismatch"):
                adapter.compiler_identity(compiler)

    def test_full_recipe_requires_committed_summary_digest(self):
        self.assertTrue(adapter.RECIPES_PATH.as_posix().startswith("workspace/"))
        with tempfile.TemporaryDirectory() as dirname:
            path = Path(dirname) / "recipes.json"
            path.write_text('{"pins":{},"subjects":{}}')
            with self.assertRaisesRegex(ValueError, "committed summary"):
                adapter.load_recipe_manifest(path)

    def test_core_type_constructors(self):
        names = adapter.core_constructor_names()
        self.assertTrue({"bool", "vec4", "dmat3x2", "mat4", "sampler2D", "image2D", "atomic_uint"} <= names)
        self.assertFalse({"texture", "customStruct"} & names)

    def test_catalog_modes_are_explicit_and_full_registry_is_pinned(self):
        core = Path(__file__).with_name("glsl460-functions.json")
        baseline_functions = adapter.builtin_index(core)[0]
        functions, constructors, metadata = adapter.catalog_for_mode(
            "baseline-core", core)
        self.assertEqual(functions, baseline_functions)
        self.assertEqual(len(constructors), 44)
        self.assertEqual(metadata["catalog_mode"], "baseline-core")
        self.assertEqual((metadata["catalog_function_count"], metadata["catalog_constructor_count"]),
                         (197, 44))
        functions, constructors, metadata = adapter.catalog_for_mode(
            "core-plus-types", core)
        self.assertEqual(len(constructors), 126)
        self.assertEqual(metadata["catalog_mode"], "core-plus-types")
        with tempfile.TemporaryDirectory() as dirname:
            root = Path(dirname)
            source = {"repository": "KhronosGroup/GLSL", "revision": "a" * 40,
                      "archive_sha256": "b" * 64, "source_count": 1}
            catalog = {"complete": True, "functions": ["extFn"], "types": ["extType"],
                       "functions_by_extension": {"GL_EXT_x": ["extFn"]},
                       "types_by_extension": {"GL_EXT_x": ["extType"]},
                       "sources": [source], "audit": {"unresolved_candidates": 0,
                                                        "challenger_gaps": 0}}
            index = root / "registered-extensions.json"
            index.write_text(json.dumps(catalog))
            review = root / "registered-extensions-review.json"
            review.write_text(json.dumps({
                "verdict": "accepted", "extension_catalog_sha256": hashlib.sha256(index.read_bytes()).hexdigest(),
                "accepted_extension_sources": [source], "review_method": "independent source review"}))
            functions, constructors, metadata = adapter.catalog_for_mode(
                "full-registry", core, index, review)
            self.assertTrue({"extFn", "sin"} <= functions)
            self.assertTrue({"extType", "vec4"} <= constructors)
            self.assertEqual(metadata["catalog_mode"], "full-registry")
            review.write_text(json.dumps({"verdict": "pending"}))
            with self.assertRaisesRegex(ValueError, "accepted"):
                adapter.catalog_for_mode("full-registry", core, index, review)

    def test_full_exclusions_do_not_depend_on_ast_calls(self):
        exclusions = adapter.exclusions_for_file("a.vert", {"sin", "texture"}, {"vec4"})
        self.assertEqual({x[1] for x in exclusions}, {"sin", "texture", "vec4"})

    def test_official_spec_catalog_coverage(self):
        index = Path(__file__).with_name("glsl460-functions.json")
        data = json.loads(index.read_text())
        self.assertEqual(len(data["coverage"]), 19)
        self.assertEqual(len(data["functions"]), 197)
        self.assertTrue({"sin", "texture", "imageLoad", "barrier"} <= set(data["functions"]))
        self.assertEqual(data["type_coverage"]["image_type_keyword_count"], 33)
        self.assertEqual(len(data["types"]), 126)
        self.assertEqual(set(data["functions"]), adapter.builtin_index(index)[0])

    def test_pre_type_baseline_catalog_is_immutable_and_same_core_functions(self):
        baseline = Path(__file__).with_name("glsl460-baseline-functions.json")
        current = Path(__file__).with_name("glsl460-functions.json")
        self.assertEqual(hashlib.sha256(baseline.read_bytes()).hexdigest(),
                         "b0da4128d106468636a49f08889ef0412cae3fc7bbb450efda92dde0792a0bb5")
        before = json.loads(baseline.read_text())
        after = json.loads(current.read_text())
        self.assertNotIn("types", before)
        self.assertEqual(before["functions"], after["functions"])

    def test_extractor_requires_each_section_from_synthetic_html(self):
        sections = ''.join(f'<h3 id="x">8.{i}. Test</h3><tr><td><strong>fn{i}</strong>(x)</td></tr>'
                           for i in range(1, 20))
        html = '<h2 id="built-in-functions"></h2>' + sections + '<h2 id="shading-language-grammar"></h2>'
        names, coverage = extractor.extract(html)
        self.assertEqual(len(names), 19)
        self.assertEqual(len(coverage), 19)

    def test_modified_catalog_cannot_add_arbitrary_exclusions(self):
        with tempfile.TemporaryDirectory() as dirname:
            root = Path(dirname)
            data = json.loads(Path(__file__).with_name("glsl460-functions.json").read_text())
            data["functions"].append("inventedUserFunction")
            index = root / "glsl460-functions.json"
            index.write_text(json.dumps(data))
            with self.assertRaisesRegex(ValueError, "catalog.*mismatch"):
                adapter.builtin_index(index)

    def test_whitespace_include_detected(self):
        self.assertTrue(adapter.has_include_directive(' # include "private.glsl"\n'))

    def test_preprocessed_input_boundary_rejects_external_and_untracked(self):
        with tempfile.TemporaryDirectory() as dirname:
            root = Path(dirname)
            shaders = root / "shaders"
            shaders.mkdir()
            main = shaders / "main.frag"
            included = shaders / "tracked.glsl"
            untracked = shaders / "untracked.glsl"
            for path in (main, included, untracked):
                path.write_text("\n")
            tracked = {"shaders/main.frag", "shaders/tracked.glsl"}
            adapter.validate_preprocessed_inputs(f'#line 1 "{main}"\n#line 1 "{included}"', root, tracked)
            self.assertEqual(adapter.validate_preprocessed_inputs('# line 1 "shaders/tracked.glsl"', root, tracked),
                             {"shaders/tracked.glsl"})
            with self.assertRaisesRegex(ValueError, "untracked"):
                adapter.validate_preprocessed_inputs(f'#line 1 "{untracked}"', root, tracked)
            with self.assertRaisesRegex(ValueError, "outside"):
                adapter.validate_preprocessed_inputs(f'#line 1 "{root.parent / "external.glsl"}"', root, tracked)
            with self.assertRaisesRegex(ValueError, "unrecognized"):
                adapter.validate_preprocessed_inputs('#line 1 <unknown>', root, tracked)

    def test_l2_payload_never_exports_ast_definitions(self):
        payload = adapter.oracle_payload("16.6.0", [{"path": "a.vert", "name": "main", "line": 1, "kind": "function"}],
                                         {("a.vert", "main", "helper")}, set(), ["a.vert"], "abc", 1,
                                         "vulkan")
        self.assertEqual(payload.get("definitions"), [])
        self.assertEqual(payload["calls"], [{"path": "a.vert", "from": "main", "to": "helper"}])

    def test_glslc_translation_disclosure_is_vulkan_samples_only(self):
        arguments = ("16.6.0", [], set(), set(), ["a.vert"], "abc", 1)
        for subject in ("vulkan", "spirv-cross", "vulkan-samples"):
            with self.subTest(subject=subject):
                payload = adapter.oracle_payload(*arguments, subject)
                translated = [note for note in payload["disclose"] if "translated to glslang" in note]
                if subject == "vulkan-samples":
                    self.assertEqual(translated, [
                        "Vulkan-Samples glslc/per-target inputs are translated to glslang; tier B"
                    ])
                else:
                    self.assertEqual(translated, [])


if __name__ == "__main__":
    unittest.main()
