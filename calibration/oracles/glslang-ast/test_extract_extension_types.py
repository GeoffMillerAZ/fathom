"""Synthetic checks for pinned GLSL extension type evidence."""
import importlib.util
import unittest
from pathlib import Path

spec = importlib.util.spec_from_file_location("extract_extension_types", Path(__file__).with_name("extract_extension_types.py"))
extractor = importlib.util.module_from_spec(spec)
spec.loader.exec_module(extractor)


class ExtensionTypesTests(unittest.TestCase):
    def test_arithmetic_keywords_and_table_must_agree(self):
        source = """Changes to Chapter 3 of the OpenGL Shading Language Specification
    If the extension GL_EXT_shader_explicit_arithmetic_types_float16 is supported,
    add the following keywords to section 3.6 Keywords:
    float16_t f16vec2 f16vec3 f16vec4
    f16mat2 f16mat3 f16mat4
    f16mat2x2 f16mat2x3 f16mat2x4
    f16mat3x2 f16mat3x3 f16mat3x4
    f16mat4x2 f16mat4x3 f16mat4x4
Changes to Chapter 4 of the OpenGL Shading Language Specification
    Transparent types with explicit bit size
    (if GL_EXT_shader_explicit_arithmetic_types_float16 is supported)
    | float16_t | scalar |
    | f16vec2 | vector |
    | f16vec3 | vector |
    | f16vec4 | vector |
    | f16mat2 | matrix |
    | f16mat3 | matrix |
    | f16mat4 | matrix |
    | f16mat2x2 | matrix |
    | f16mat2x3 | matrix |
    | f16mat2x4 | matrix |
    | f16mat3x2 | matrix |
    | f16mat3x3 | matrix |
    | f16mat3x4 | matrix |
    | f16mat4x2 | matrix |
    | f16mat4x3 | matrix |
    | f16mat4x4 | matrix |
"""
        facts = extractor.arithmetic_types(source, "ext/test.txt", "a" * 64, required_families=1)
        self.assertEqual(len(facts), 16)
        self.assertIn("f16mat4x4", facts)
        self.assertEqual(facts["float16_t"]["extension"], "GL_EXT_shader_explicit_arithmetic_types_float16")
        with self.assertRaisesRegex(ValueError, "keyword/table mismatch"):
            extractor.arithmetic_types(source.replace("    | f16mat4x4 | matrix |\n", ""), "ext/test.txt", "a" * 64,
                                       required_families=1)

    def test_opaque_type_is_not_constructor_claim(self):
        source = """GL_EXT_ray_query
Modify Section 3.6, Keywords
    (add to list of keywords)
    rayQueryEXT
Add following to Section 4.1 (Basic Types)
    Ray Query Opaque Types
    Types Meaning
    rayQueryEXT A handle representing a ray query object
"""
        facts = extractor.named_type(source, "rayQueryEXT", "GL_EXT_ray_query", "ext/ray.txt", "b" * 64,
                                     constructor=False)
        self.assertEqual(facts["rayQueryEXT"]["constructor"], "not_asserted")

    def test_placeholder_is_not_reserved_identifier(self):
        self.assertFalse(extractor.concrete_type("genF16Type"))
        self.assertFalse(extractor.concrete_type("gvecN"))
        self.assertTrue(extractor.concrete_type("f16vec4"))

    def test_arithmetic_ranges_cover_all_declared_shapes(self):
        self.assertEqual(len(extractor.expected_arithmetic("GL_EXT_shader_explicit_arithmetic_types_float16")), 16)
        self.assertEqual(len(extractor.expected_arithmetic("GL_EXT_shader_explicit_arithmetic_types_int64")), 8)
        self.assertIn("f16mat4x3", extractor.expected_arithmetic("GL_EXT_shader_explicit_arithmetic_types_float16"))
        self.assertIn("u64vec4", extractor.expected_arithmetic("GL_EXT_shader_explicit_arithmetic_types_int64"))

    def test_tensor_keyword_and_parameterized_basename_are_distinct(self):
        source = """GL_ARM_tensors
Modify Section 3.6, Keywords
tensor
Add following to Section 4.1 (Basic Types)
tensorARM<float, 4> names a parameterized tensor type.
"""
        keyword = extractor.named_type(source, "tensor", "GL_ARM_tensors", "arm/tensor.txt", "a" * 64)
        basename = extractor.named_type(source, "tensorARM", "GL_ARM_tensors", "arm/tensor.txt", "a" * 64)
        self.assertEqual(keyword["tensor"]["category"], "reserved_type_keyword")
        self.assertEqual(basename["tensorARM"]["category"], "parameterized_type_basename")
        self.assertEqual(basename["tensorARM"]["constructor"], "not_asserted")

    def test_independent_challenger_rejects_unreviewed_type_source(self):
        with self.assertRaisesRegex(ValueError, "unresolved extension type source candidates"):
            extractor.challenge_sources({"new/GLSL_NEW_types.txt": b"Add the following opaque types to the language"})

    def test_registry_half_float_keyword_typo_is_recorded_from_type_table(self):
        names = extractor.expected_arithmetic("GL_EXT_shader_explicit_arithmetic_types_float16")
        keywords = " ".join(sorted(names - {"f16mat2x3"} | {"fl6mat2x3"}))
        table = "\n".join("| " + name + " | type |" for name in sorted(names))
        source = ("(add the following to the list of reserved keywords at p. 18)\n" + keywords +
                  "\nAdditions to Chapter 4\nModify Section 4.1, Basic Types\n" + table +
                  "\nModify Section 4.1.4, Floating-Point Variables\n")
        facts = extractor.registry_transparent_types("AMD/AMD_gpu_shader_half_float.txt", source, "a" * 64)
        self.assertIn("3.6 typo fl6mat2x3", facts["f16mat2x3"]["evidence"])
        self.assertEqual(facts["f16mat2x3"]["repository"], "KhronosGroup/OpenGL-Registry")
        with self.assertRaisesRegex(ValueError, "keyword/table disagreement"):
            extractor.registry_transparent_types("AMD/AMD_gpu_shader_half_float.txt",
                                                 source.replace("fl6mat2x3", "bogusMatrix"), "a" * 64)

    def test_cooperative_matrix2_tensor_names_need_declarations(self):
        source = """GL_NV_cooperative_matrix2
Add a new Section 4.1.X, Tensor Layout Types
struct tensorLayoutNV<uint N> {};
struct tensorViewNV<uint N> {};
See Section 8.X, Cooperative Matrix Functions
"""
        facts = extractor.tensor2_types(source, "nv/GLSL_NV_cooperative_matrix2.txt", "b" * 64)
        self.assertEqual(set(facts), {"tensorLayoutNV", "tensorViewNV"})
        with self.assertRaisesRegex(ValueError, "tensor type mismatch"):
            extractor.tensor2_types(source.replace("struct tensorViewNV", "struct View"),
                                    "nv/GLSL_NV_cooperative_matrix2.txt", "b" * 64)

    def test_registry_basic_family_requires_keyword_table_agreement(self):
        source = """Add to section 3.6 "Keywords"
The following new sampler types are added:
samplerRenderbuffer, isamplerRenderbuffer, usamplerRenderbuffer
Add to section 4.1 "Basic Types"
samplerRenderbuffer handle for renderbuffer
isamplerRenderbuffer handle for integer renderbuffer
usamplerRenderbuffer handle for unsigned renderbuffer
Add to section 8.7 "Texture Lookup Functions"
"""
        facts = extractor.registry_basic_family("NV/NV_explicit_multisample.txt", source, "c" * 64)
        self.assertEqual(len(facts), 3)
        with self.assertRaisesRegex(ValueError, "table mismatch"):
            extractor.registry_basic_family("NV/NV_explicit_multisample.txt",
                                            source.replace("usamplerRenderbuffer handle", "wrongType handle"), "c" * 64)

    def test_registry_challenger_requires_hash_and_line_bound_dispositions(self):
        raw = b"GLSL\nThis extension adds a new object type, not a shader data type.\n"
        path = "EXT/example.txt"
        digest = extractor.hashlib.sha256(raw).hexdigest()
        audit = extractor.challenge_registry_sources(
            {path: raw}, {path: (digest, 2, 0)}, independent_review=True)
        self.assertTrue(audit["complete"])
        self.assertEqual(audit["unresolved_candidates"], 0)
        self.assertEqual(audit["non_type_dispositions"][0]["line"], 2)
        with self.assertRaisesRegex(ValueError, "digest mismatch"):
            extractor.challenge_registry_sources(
                {path: raw}, {path: ("0" * 64, 2, 0)}, independent_review=True)
        with self.assertRaisesRegex(ValueError, "citation mismatch"):
            extractor.challenge_registry_sources(
                {path: raw}, {path: (digest, 99, 0)}, independent_review=True)

    def test_registry_challenger_fails_closed_for_new_source(self):
        source = {"EXT/new.txt": b"GLSL adds a new opaque type to the shading language.\n"}
        audit = extractor.challenge_registry_sources(source, {})
        self.assertFalse(audit["complete"])
        self.assertEqual(audit["unresolved_paths"], ["EXT/new.txt"])

    def test_independent_heading_challenger_fails_closed(self):
        source = {"EXT/new.txt": b"Modify Section 4.1, Basic Types\nnewType a new type\n"}
        audit = extractor.challenge_type_headings(source, set(), {})
        self.assertFalse(audit["complete"])
        self.assertEqual(audit["unresolved_paths"], ["EXT/new.txt"])


if __name__ == "__main__":
    unittest.main()
