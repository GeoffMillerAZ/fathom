import importlib.util
import hashlib
import tempfile
import unittest
from pathlib import Path
from unittest import mock

spec = importlib.util.spec_from_file_location("recipes", Path(__file__).with_name("generate.py"))
recipes = importlib.util.module_from_spec(spec)
spec.loader.exec_module(recipes)
fetch_spec = importlib.util.spec_from_file_location("fetch", Path(__file__).with_name("fetch_pinned_cmake.py"))
fetcher = importlib.util.module_from_spec(fetch_spec)
fetch_spec.loader.exec_module(fetcher)


class RecipeTests(unittest.TestCase):
    def test_default_recipe_output_is_ignored_and_summary_has_no_file_list(self):
        self.assertTrue(recipes.OUTPUT.as_posix().startswith("workspace/"))
        import json
        summary = json.loads(Path(__file__).with_name("recipes-summary.json").read_text())
        self.assertEqual(set(summary), {"pins", "populations", "full_recipe_sha256"})
        self.assertNotIn("files", Path(__file__).with_name("recipes-summary.json").read_text())

    def test_modified_cached_cmake_rejected_against_pinned_blob(self):
        original = b'SHADER_FILES_GLSL\n  "demo.frag"\n'
        blob = hashlib.sha1(b"blob " + str(len(original)).encode() + b"\0" + original).hexdigest()
        with tempfile.TemporaryDirectory() as dirname:
            cache_root = Path(dirname)
            cmake = cache_root / "samples" / "demo" / "CMakeLists.txt"
            cmake.parent.mkdir(parents=True)
            cmake.write_bytes(original.replace(b"demo.frag", b"other.frag"))
            tree = f"100644 blob {blob}\tsamples/demo/CMakeLists.txt\n"
            with mock.patch.object(recipes, "CACHE_ROOT", cache_root), \
                 mock.patch.object(recipes, "CACHE", cache_root / "samples"), \
                 mock.patch.object(recipes.subprocess, "check_output", return_value=tree):
                with self.assertRaisesRegex(ValueError, "blob mismatch"):
                    recipes.sample_declarations()

    def test_failed_pinned_source_fetch_exits_nonzero(self):
        with mock.patch.object(fetcher.subprocess, "check_output", side_effect=[fetcher.PIN + "\n", "samples/demo/CMakeLists.txt\n"]), \
             mock.patch.object(fetcher, "fetch", side_effect=OSError("network failed")):
            with self.assertRaises(SystemExit) as failure:
                fetcher.main()
        self.assertEqual(failure.exception.code, 1)

    def test_committed_manifest_populations_and_flags(self):
        import json
        summary = json.loads(Path(__file__).with_name("recipes-summary.json").read_text())
        self.assertEqual([summary["populations"][n] for n in ("vulkan", "spirv-cross", "vulkan-samples")],
                         [353, 306, 309])
        self.assertEqual(len(summary["full_recipe_sha256"]), 64)

    @unittest.skipUnless(Path("workspace/calibration/glsl/vulkan/.git").exists(), "pinned local corpora absent")
    def test_population_and_native_flags(self):
        full = recipes.build()
        data = full["subjects"]
        import json
        encoded = (json.dumps(full, indent=2) + "\n").encode()
        summary = json.loads(recipes.SUMMARY.read_text())
        self.assertEqual(hashlib.sha256(encoded).hexdigest(), summary["full_recipe_sha256"])
        self.assertEqual([data[n]["population"] for n in ("vulkan", "spirv-cross", "vulkan-samples")], [353, 306, 309])
        cross = {r["path"]: r for r in data["spirv-cross"]["files"]}
        self.assertTrue(all("-V" in r["flags"] for r in cross.values()))
        self.assertEqual(sum("-gV" in r["flags"] for r in cross.values()), 1)
        callable_flags = [r["flags"] for r in data["vulkan"]["files"] if r["path"].endswith(".rcall")]
        self.assertEqual(len(callable_flags), 3)
        self.assertTrue(all(flags == ["-V"] for flags in callable_flags))
        sample = data["vulkan-samples"]["files"]
        self.assertEqual(sum(r["declaration_status"] == "unresolved" for r in sample), 14)
        self.assertTrue(any("--target-spv=spv1.5" in d["glslc_additional_arguments"]
                            for r in sample for d in r["cmake_declarations"]))


if __name__ == "__main__":
    unittest.main()
