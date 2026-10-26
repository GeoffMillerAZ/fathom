"""Synthetic checks for fail-closed Khronos extension source auditing."""
import importlib.util
import json
import tempfile
import unittest
from pathlib import Path


spec = importlib.util.spec_from_file_location("extract_extensions", Path(__file__).with_name("extract_extensions.py"))
extractor = importlib.util.module_from_spec(spec)
spec.loader.exec_module(extractor)


class ExtensionAuditTests(unittest.TestCase):
    @staticmethod
    def accepted_review(report):
        candidates = [row for part in report.values() for item in part["sources"]
                      for row in item["candidates"] if row["status"].startswith("candidate_")]
        return {"verdict": "accepted",
                "accepted_candidate_sha256": extractor.candidate_digest(report),
                "accepted_candidate_count": len(candidates),
                "accepted_function_count": len({row["name"] for row in candidates}),
                "source_pins": {name: {"revision": part.get("revision"),
                                        "archive_sha256": part.get("archive_sha256"),
                                        "source_count": part["source_count"]}
                                for name, part in sorted(report.items())},
                "review_method": "independent source-line positive review"}

    def test_extension_ids_come_only_from_name_strings_block(self):
        source = """Name

    EXT_example
Name Strings

    GL_EXT_example
    GL_EXT_example_second
Contact
    GL_EXT_unrelated_contact
"""
        self.assertEqual(extractor.extension_name_strings(source),
                         ["GL_EXT_example", "GL_EXT_example_second"])
        self.assertEqual(extractor.extension_name_strings("Name String\n\n GL_EXT_single\nContact\n"),
                         ["GL_EXT_single"])
        self.assertEqual(extractor.extension_name_strings("Name\n GL_KHR_direct\nContact\n"),
                         ["GL_KHR_direct"])
        self.assertEqual(extractor.extension_name_strings(
            "Name Strings\n none (implied by GL_NV_gpu_program5 or GL_NV_gpu_shader5)\nContact\n"),
            ["GL_NV_gpu_program5", "GL_NV_gpu_shader5"])

    def test_shader_definitions_do_not_become_builtin_declarations(self):
        source = """Additions to Chapter 8 of the OpenGL Shading Language Specification
    vec4 userHelper(vec4 x) {
        return x;
    }
    void main()
    {
        userHelper(vec4(1));
    }
    vec4 realBuiltinEXT(vec4 x);
"""
        rows = extractor.audit_text(source)
        self.assertEqual([(row["name"], row["status"]) for row in rows],
                         [("userHelper", "excluded_shader_definition"),
                          ("main", "excluded_shader_definition"),
                          ("realBuiltinEXT", "candidate_prototype")])

    def test_example_and_pseudocode_headings_end_declaration_context(self):
        source = """Additions to Chapter 8 of the OpenGL Shading Language Specification
    vec4 declaredEXT(vec4 x);
Usage Examples
    vec4 helper(vec4 x);
Additions to Chapter 8 of the OpenGL Shading Language Specification
    vec4 anotherEXT(vec4 x);
Pseudo-code:
    vec4 ComputeAccessedLod(vec4 x);
"""
        rows = extractor.audit_text(source)
        self.assertEqual([(row["name"], row["status"]) for row in rows],
                         [("declaredEXT", "candidate_prototype"),
                          ("helper", "excluded_example"),
                          ("anotherEXT", "candidate_prototype"),
                          ("ComputeAccessedLod", "excluded_example")])

    def test_normative_prototypes_and_example_boundary(self):
        source = """Modifications to the OpenGL Shading Language Specification
Modify Section 8.4, Built-in Functions
Syntax:
    void rayQueryProceedEXT(uint query);
    bool textureFootprintNV(vec2 x)
Examples
    void main() { rayQueryProceedEXT(1); }
Issues
"""
        rows = extractor.audit_text(source)
        self.assertEqual([(r["name"], r["section"]) for r in rows if r["status"] == "candidate_prototype"],
                         [("rayQueryProceedEXT", "normative"), ("textureFootprintNV", "normative")])

    def test_unknown_return_type_remains_unclassified(self):
        source = """Modifications to the OpenGL Shading Language Specification
Modify Section 8.4, Built-in Functions
Syntax:
    opaqueNewTypeEXT novelFunctionEXT(int a);
"""
        rows = extractor.audit_text(source)
        self.assertEqual(rows[0]["status"], "unclassified_return_type")
        with_verified_type = extractor.audit_text(source, extra_types={"opaqueNewTypeEXT"})
        self.assertEqual(with_verified_type[0]["status"], "candidate_prototype")

    def test_prose_sentence_before_call_is_not_a_signature(self):
        source = """Additions to Chapter 8 of the OpenGL Shading Language Specification
The clockARB() function returns a value.
and textureGather() can be used here.
a controlBarrierArrive() call blocks here.
"""
        self.assertEqual([(row["name"], row["status"]) for row in extractor.audit_text(source)],
                         [("clockARB", "excluded_prose_sentence"),
                          ("textureGather", "excluded_prose_sentence"),
                          ("controlBarrierArrive", "excluded_prose_sentence")])

    def test_prose_function_reference_is_audited(self):
        source = """Modifications to the OpenGL Shading Language Specification
Add new built-in function mysteryEXT() to Section 8.5.
"""
        rows = extractor.audit_text(source)
        self.assertEqual(rows[0]["name"], "mysteryEXT")
        self.assertEqual(rows[0]["status"], "unclassified_prose_declaration")

    def test_missing_modification_heading_is_not_silent(self):
        rows = extractor.audit_text("Syntax:\n    vec4 legacyBuiltInARB(vec4 x);\n")
        self.assertEqual(rows[0]["status"], "unclassified_no_modifications_section")

    def test_api_modifications_do_not_count_as_shader_builtins(self):
        source = """Modifications to the OpenGL 4.5 Specification
    void glApiThingEXT(int value);
Modifications to the OpenGL Shading Language Specification
Modify Section 8.4, Built-in Functions
    vec4 shaderThingEXT(vec4 value);
Modifications to the OpenGL 4.5 Specification
    void glAnotherApiThingEXT(int value);
"""
        rows = extractor.audit_text(source)
        self.assertEqual([r["name"] for r in rows if r["status"] == "candidate_prototype"],
                         ["shaderThingEXT"])
        self.assertEqual([r["name"] for r in rows if r["status"] == "unclassified_outside_shader_section"],
                         ["glApiThingEXT", "glAnotherApiThingEXT"])

    def test_interactions_section_ends_shader_function_amendment(self):
        source = """Additions to Chapter 8 of the OpenGL Shading Language Specification
    void shaderBuiltinEXT();
Interactions with NV_depth_buffer_float
    void DepthRangeArraydvNV(uint first);
"""
        rows = extractor.audit_text(source)
        self.assertEqual([(row["name"], row["status"]) for row in rows],
                         [("shaderBuiltinEXT", "candidate_prototype"),
                          ("DepthRangeArraydvNV", "unclassified_outside_shader_section")])

    def test_explicit_none_ends_empty_shader_amendment(self):
        source = """Additions to Chapter 8 of the OpenGL Shading Language Specification
None
Dependencies on Core Profile
    void MatrixLoadfEXT(float x);
"""
        rows = extractor.audit_text(source)
        self.assertEqual(rows[0]["status"], "unclassified_outside_shader_section")

    def test_split_return_and_name_cannot_disappear(self):
        source = """Modifications to the OpenGL Shading Language Specification
Modify Section 8.4, Built-in Functions
Syntax:
    vec4
    multilineBuiltinEXT(vec4 x);
"""
        rows = extractor.audit_text(source)
        self.assertIn("multilineBuiltinEXT", [r["name"] for r in rows])
        self.assertIn("unclassified_split_prototype", [r["status"] for r in rows])

    def test_nested_parameterized_return_signatures_and_body_exclusion(self):
        source = """Modifications to the OpenGL Shading Language Specification
Add Section 8.X, Cooperative Matrix Functions
    tensorLayoutNV<Dim, Mode> createTensorLayoutNV(uint32_t Dim);
    tensorViewNV<Dim, hasDimensions, p0, ..., p<Dim-1>> createTensorViewNV(uint32_t Dim);
    vector<floate2m1_t, 8> unpackFloat8xfe2m1EXT(uint32_t value);
Examples
    vector<float16_t, 2> decode_q8_0_v2(uint32_t value) {
        return vector<float16_t, 2>(0);
    }
"""
        rows = extractor.audit_text(source, extra_types={"tensorLayoutNV", "tensorViewNV", "vector"})
        self.assertEqual([(row["name"], row["status"]) for row in rows],
                         [("createTensorLayoutNV", "candidate_prototype"),
                          ("createTensorViewNV", "candidate_prototype"),
                          ("unpackFloat8xfe2m1EXT", "candidate_prototype"),
                          ("decode_q8_0_v2", "excluded_shader_definition")])
        challenged = {row["name"] for row in extractor.challenger_text(source)}
        self.assertTrue({"createTensorLayoutNV", "createTensorViewNV", "unpackFloat8xfe2m1EXT",
                         "decode_q8_0_v2"} <= challenged)
        local_intro = """Modifications to the OpenGL Shading Language Specification
Modify Section 4.1, Basic Types
Tensor layouts are created by calling:
    tensorLayoutNV<Dim, Mode> createTensorLayoutNV(uint32_t Dim);
"""
        self.assertEqual(extractor.audit_text(local_intro, extra_types={"tensorLayoutNV"})[0]["status"],
                         "candidate_prototype")

    def test_split_host_api_and_hypothetical_program_functions_are_not_glsl(self):
        source = """Modifications to the OpenGL 4.5 Specification
    void
    getCounterByName(int x);
    (add hypothetical built-in functions SharedMemoryLoad() and
    SharedMemoryStore() to the GPU program instruction description)
"""
        rows = extractor.audit_text(source)
        self.assertIn(("getCounterByName", "unclassified_outside_shader_section"),
                      [(row["name"], row["status"]) for row in rows])
        self.assertIn(("SharedMemoryLoad", "excluded_hypothetical_program_function"),
                      [(row["name"], row["status"]) for row in rows])

    def test_shader_example_outside_function_section_is_unclassified(self):
        source = """Modifications to the OpenGL Shading Language Specification
Modify Section 4.3, Variables
    void main() { }
Modify Section 8.5, Built-in Functions
    float actualBuiltinEXT(float x);
"""
        rows = extractor.audit_text(source)
        self.assertEqual(rows[0]["status"], "excluded_shader_definition")
        self.assertEqual(rows[1]["status"], "candidate_prototype")

    def test_post_issues_shader_amendment_is_scanned(self):
        source = """Issues
    vec4 exampleFoo(vec4 x);
Additions to Chapter 8 of the OpenGL Shading Language Specification
    vec4 texture2DRect(vec2 x);
"""
        rows = extractor.audit_text(source)
        self.assertEqual(rows[-1]["status"], "candidate_prototype")
        self.assertEqual(rows[-1]["name"], "texture2DRect")

    def test_standalone_section_8_heading_licenses_syntax_table(self):
        source = """Modifications to the OpenGL Shading Language Specification
Section 8.9.X, Texture Footprint Query Functions
Syntax:
    bool textureFootprintNV(gsampler2D sampler);
"""
        rows = extractor.audit_text(source)
        self.assertEqual(rows[0]["status"], "candidate_prototype")
        plain_title = """Additions to the OpenGL Shading Language
Section 8.13.1 Derivative Functions
    getType fwidth(getType p)
"""
        self.assertEqual(extractor.audit_text(plain_title)[0]["status"], "unclassified_return_type")

    def test_quoted_section_reference_does_not_end_function_section(self):
        source = """Additions to Chapter 8 of the OpenGL Shading Language Specification
    Add Section 8.19, Ray Tracing Functions
    Section 7.3 "Built-In Constants".
    bool reportIntersectionNV(float hitT, uint hitKind);
"""
        rows = extractor.audit_text(source)
        self.assertEqual(rows[0]["status"], "candidate_prototype")
        lower_reference = """Additions to Chapter 8 of the OpenGL Shading Language Specification
    section 2.15.4 of the API specification describes texel fetches.
    vec4 texelFetchBuffer(samplerBuffer sampler, int coord)
"""
        self.assertEqual(extractor.audit_text(lower_reference)[0]["status"], "candidate_prototype")

    def test_modification_of_other_extension_chapter_8_is_function_amendment(self):
        source = """Modifications to the OpenGL Shading Language Specification
Modifications of the additions in the GL_KHR_shader_subgroup extension to
Chapter 8 of the OpenGL Shading Language Specification
    bool subgroupAllEqual(genI8Type value);
"""
        rows = extractor.audit_text(source)
        self.assertEqual(rows[0]["status"], "candidate_prototype")

    def test_numeric_section_8_and_subsection_table_amendments(self):
        source = """Modifications to the OpenGL Shading Language Specification
Modify 8.16. Shader Invocation Control Functions
| void controlBarrierArrive() | Description |
Modify 4.3 Shared Variables
Add to the table at the end of subsection 8.9.1 the following functions:
    int textureSize(gtexture2D texture, int lod)
"""
        rows = extractor.audit_text(source)
        self.assertEqual([(row["name"], row["status"]) for row in rows],
                         [("controlBarrierArrive", "candidate_prototype"),
                          ("textureSize", "candidate_prototype")])
        change_chapter = """Modifications to the OpenGL Shading Language Specification
Change Section 7.1 Shader Variables
Change Chapter 8, built-in functions:
Syntax:
    vec4 texelFetchBuffer(samplerBuffer sampler, int coord)
"""
        self.assertEqual(extractor.audit_text(change_chapter)[0]["status"], "candidate_prototype")

    def test_dependency_amendment_explicitly_adding_texture_functions(self):
        source = """Modifications to the OpenGL Shading Language Specification
Modify Section 4.1, Basic Types
Dependencies on ARB_sparse_texture2
    If enabled, this extension adds additional texture lookup
    functions, whose prototypes differ from the existing ones.
| Syntax | Description |
| int sparseTextureARB(f16sampler2D s, out f16vec4 texel) | Added overload |
"""
        rows = extractor.audit_text(source)
        self.assertEqual(rows[0]["status"], "candidate_prototype")
        gather = """Modifications to the OpenGL Shading Language Specification
Modify Section 4.1, Basic Types
Dependencies on AMD_texture_gather_bias_lod
    This extension adds additional texture
    gather functions, whose prototypes differ from existing ones.
| f16vec4 textureGather(f16sampler2D sampler) | Added overload |
"""
        self.assertEqual(extractor.audit_text(gather)[0]["status"], "candidate_prototype")

    def test_explicit_function_list_intros_without_standard_chapter_heading(self):
        cases = [
            ("The existing isotropic vertex texture functions:\n"
             "    vec4 texture2DLodEXT(sampler2D sampler);\n", "texture2DLodEXT"),
            ("Dependencies on GLSL 4.30\n"
             "If GLSL 4.30 is supported, add the following atomic memory functions to\n"
             "section 8.11 (Atomic Memory Functions):\n"
             "    uint atomicIncWrap(inout uint mem, uint wrap);\n", "atomicIncWrap"),
            ("Modifications to the OpenGL ES Shading Language\n"
             "Add to the list of texture gather functions as introduced with EXT_gpu_shader5\n"
             "and core functionality in ESSL 3.1 the following additional function:\n"
             "    vec4 textureGather(samplerExternal2D sampler);\n", "textureGather"),
        ]
        for source, name in cases:
            with self.subTest(name=name):
                self.assertIn((name, "candidate_prototype"),
                              [(row["name"], row["status"]) for row in extractor.audit_text(source)])

    def test_standalone_new_builtin_section_is_scanned(self):
        source = """New Built-in function
    vec3 rgb_2_yuv(vec3 color);
"""
        rows = extractor.audit_text(source)
        self.assertEqual(rows[0]["status"], "candidate_prototype")

    def test_normative_functions_label_licenses_signature_list(self):
        source = """Modifications to the OpenGL Shading Language Specification
Modify Section 4.1 Basic Types
Atomic memory functions are supported for buffer variables.
Functions:
    uint atomicAdd(inout uint mem, uint data);
"""
        self.assertEqual(extractor.audit_text(source)[0]["status"], "candidate_prototype")

    def test_named_chapter_8_extension_amendment(self):
        source = """Modifications to the OpenGL Shading Language Specification
At the end of 8.X, Cooperative Matrix Functions, add the following conversion functions.
    void bitcastQCOM(SrcTy src, DstTy dst);
"""
        self.assertEqual(extractor.audit_text(source)[0]["status"], "candidate_prototype")

    def test_independent_challenger_sees_split_and_prose_declarations(self):
        source = """Some shader amendment says built-in function proseOnlyEXT is available.
    vec4
    splitCandidateEXT(vec4 x);
"""
        names = {row["name"] for row in extractor.challenger_text(source)}
        self.assertTrue({"proseOnlyEXT", "splitCandidateEXT"} <= names)
        self.assertFalse({"s", "t"} & names)
        self.assertEqual(extractor.challenger_text("    shaderCall(x);\n"), [])

    def test_preamble_builtin_intro_and_non_normative_mapping(self):
        source = """Overview
    These objects support reading through the following functions:
        gvec4 subpassLoad(gsubpassInput subpass);
Mapping to SPIR-V
    For informational purposes (non-normative)
    bool textureFootprintNV(gsampler2D sampler);
Modifications to the OpenGL Shading Language Specification
    Section 8.9.X, Texture Footprint Query Functions
    bool textureFootprintNV(gsampler2D sampler);
"""
        rows = extractor.audit_text(source)
        self.assertEqual([(r["name"], r["status"]) for r in rows],
                         [("subpassLoad", "candidate_prototype"),
                          ("textureFootprintNV", "excluded_non_normative_mapping"),
                          ("textureFootprintNV", "candidate_prototype")])

    def test_explicit_function_variants_after_interactions_heading(self):
        source = """Modifications to the OpenGL Shading Language Specification
Interactions with GL_AMD_shader_image_load_store_lod
    Add the following new function variants:
    u64vec4 imageLoadLodAMD(readonly IMAGE64_LOD_PARAMS);
"""
        rows = extractor.audit_text(source)
        self.assertEqual(rows[0]["status"], "candidate_prototype")
        additional = source.replace("new function variants", "additional function variants")
        self.assertEqual(extractor.audit_text(additional)[0]["status"], "candidate_prototype")

    def test_prose_prefix_and_attributed_example_body_are_not_functions(self):
        source = """Contact
    metadataToken contactToken (contact 'at' example.invalid)
Overview
    void main() [[attribute]] { ... }
Modifications to the OpenGL Shading Language Specification
    set by (or if provided, must be consistent with) a maximum
"""
        rows = extractor.audit_text(source)
        self.assertEqual([r["status"] for r in rows],
                         ["unclassified_no_modifications_section", "excluded_shader_definition",
                          "unclassified_outside_function_section"])

    def test_reviewed_prose_receipt_is_exact_and_fails_on_new_candidates(self):
        import hashlib
        import json
        row = {"line_in_normative_section": 11, "name": "contactToken", "return_type": "metadataToken",
               "status": "unclassified_no_modifications_section"}
        item = {"path": "arm/example.txt", "source_sha256": "pinned-source", "candidates": [row]}
        entry = (item["path"], item["source_sha256"], 11, "contactToken", "metadataToken", row["status"])
        digest = hashlib.sha256(json.dumps([entry], separators=(",", ":")).encode()).hexdigest()
        extractor.adjudicate_reviewed_glsl_prose([item], digest, 1)
        self.assertEqual(row["status"], "excluded_source_reviewed_prose")
        changed = {**row, "name": "newBuiltin", "status": "unclassified_no_modifications_section"}
        with self.assertRaises(ValueError):
            extractor.adjudicate_reviewed_glsl_prose([{**item, "candidates": [changed]}], digest, 1)

    def test_pointer_return_in_normative_syntax(self):
        source = """Modifications to the OpenGL Shading Language Specification
Add to the end of Section 8.3, Common Functions
Syntax:
    void *packPtr(uvec2 a);
    uvec2 unpackPtr(void *a);
"""
        rows = extractor.audit_text(source)
        self.assertEqual([(r["name"], r["status"]) for r in rows],
                         [("packPtr", "candidate_prototype"),
                          ("unpackPtr", "candidate_prototype")])
        self.assertIn("packPtr", {r["name"] for r in extractor.challenger_text(source)})
        self.assertIn("packPtr", {r["name"] for r in extractor.challenger_text(source)
                                  if r["kind"] == "syntax_block_name"})

    def test_catalog_composition_requires_complete_independent_types(self):
        source = {"path": "nv/a.txt", "source_sha256": "a", "name_strings": ["GL_NV_a"],
                  "candidates": [{"name": "aFunction", "return_type": "void",
                                  "line_in_normative_section": 4, "status": "candidate_prototype"}],
                  "challenger_gaps": []}
        empty = {"source_count": 0, "challenger_gap_count": 0, "sources": [],
                 "revision": "registry-pin", "archive_sha256": "registry-hash"}
        report = {"glsl": {"source_count": 1, "challenger_gap_count": 0,
                           "sources": [source], "revision": "glsl-pin",
                           "archive_sha256": "glsl-hash"}, "registry": empty}
        types = {"types": ["aType"], "types_by_extension": {"GL_NV_a": ["aType"]},
                 "constructors": [], "type_audit": {"complete": False, "unresolved_candidates": 1,
                                                    "challenger_gaps": 1},
                 "type_sources": [{"repository": "KhronosGroup/GLSL", "revision": "glsl-pin",
                                   "archive_sha256": "glsl-hash", "source_count": 1},
                                  {"repository": "KhronosGroup/OpenGL-Registry",
                                   "revision": "registry-pin", "archive_sha256": "registry-hash",
                                   "source_count": 0}]}
        review = self.accepted_review(report)
        with self.assertRaises(ValueError):
            extractor.compose_catalog(report, types, review, {"glsl": 1, "registry": 0})
        types["type_audit"] = {"complete": True, "unresolved_candidates": 0,
                               "challenger_gaps": 0}
        catalog = extractor.compose_catalog(report, types, review, {"glsl": 1, "registry": 0})
        self.assertEqual((catalog["functions"], catalog["types"], catalog["audit"]),
                         (["aFunction"], ["aType"], {"unresolved_candidates": 0,
                                                      "challenger_gaps": 0}))

    def test_qualified_generic_prototype_is_accounted(self):
        source = """Modifications to the OpenGL Shading Language Specification
Modify Section 8.2
    highp igenType maskedFunctionEXT(igenType x);
"""
        rows = extractor.audit_text(source)
        self.assertEqual(rows[0]["name"], "maskedFunctionEXT")
        self.assertEqual(rows[0]["status"], "candidate_prototype")

    def test_generic_return_notation_in_function_tables(self):
        source = """Additions to Chapter 8 of the OpenGL Shading Language Specification
| bvec lessThan(genF16Type x, genF16Type y) |
| dmat matrixCompMult(dmat x, dmat y) |
"""
        self.assertEqual([(row["name"], row["status"]) for row in extractor.audit_text(source)],
                         [("lessThan", "candidate_prototype"),
                          ("matrixCompMult", "candidate_prototype")])

    def test_pinned_source_return_typo_requires_exact_line(self):
        row = {"line_in_normative_section": 236, "name": "fwidth", "return_type": "getType",
               "status": "unclassified_return_type"}
        extractor.adjudicate_return_notation("ARB/ARB_derivative_control.txt", [row])
        self.assertEqual(row["status"], "candidate_prototype")
        self.assertEqual(row["reviewed_return_notation"], "source generic-type typo")
        bad = dict(row, line_in_normative_section=237, status="unclassified_return_type")
        with self.assertRaisesRegex(ValueError, "return notation"):
            extractor.adjudicate_return_notation("ARB/ARB_derivative_control.txt", [bad])

    def test_split_syntax_table_rows_are_declared_even_for_core_name(self):
        source = """Additions to the OpenGL ES Shading Language Specification
Modifications to Section 3.7 Keywords
Modify section 8.15 Shader Invocation Control Functions
| Syntax | Description |
| void | descriptive words |
| barrier(void) | tessellation invocations |
"""
        rows = extractor.audit_text(source)
        self.assertIn(("barrier", "candidate_table_prototype"),
                      [(row["name"], row["status"]) for row in rows])
        self.assertIn("barrier", [row["name"] for row in extractor.challenger_text(source)])
        raw = next(row for row in extractor.challenger_text(source) if row["kind"] == "split_table_span")
        self.assertEqual(extractor.classify_challenger(source, raw, set()), "unresolved_split_table_span")

    def test_challenger_disposes_grammar_words_but_keeps_named_reference(self):
        source = "The built-in functions that exist.\nThe built-in function novelBuiltInEXT is available.\nNew built-in functions Add below.\n"
        raw = extractor.challenger_text(source)
        decisions = [extractor.classify_challenger(source, row, set()) for row in raw]
        self.assertEqual(decisions, ["ordinary_prose", "unresolved_named_reference", "ordinary_prose"])

    def test_cross_source_prose_reference_requires_declared_prototype(self):
        report = {"glsl": {"sources": [{"path": "nv/a.txt", "candidates": [],
                  "challenger_dispositions": [{"name": "beginInvocationInterlockARB", "line": 4,
                                              "kind": "prose_span", "disposition": "unresolved_named_call"}],
                  "challenger_gaps": [{"name": "beginInvocationInterlockARB", "line": 4,
                                       "kind": "prose_span", "disposition": "unresolved_named_call"}],
                  "source_sha256": "a"}]},
                  "registry": {"sources": [{"path": "ARB/b.txt", "candidates": [
                      {"name": "beginInvocationInterlockARB", "line_in_normative_section": 8,
                       "status": "candidate_prototype"}], "challenger_dispositions": [],
                      "challenger_gaps": [], "source_sha256": "b"}]}}
        extractor.reconcile_cross_references(report)
        self.assertEqual(report["glsl"]["sources"][0]["challenger_gaps"], [])
        self.assertEqual(report["glsl"]["sources"][0]["challenger_dispositions"][0]["disposition"],
                         "cross_source_prose_reference")

    def test_reviewed_span_must_match_exact_pinned_source_line(self):
        original = extractor.REVIEWED_NON_SHADER_SPANS
        extractor.REVIEWED_NON_SHADER_SPANS = {("A/spec.txt", 7, "hostThing"): "host API section"}
        try:
            row = {"line": 7, "name": "hostThing", "disposition": "unresolved_prototype_span"}
            report = {"registry": {"sources": [{"path": "A/spec.txt", "challenger_dispositions": [row],
                                                "challenger_gaps": [row]}]}}
            extractor.adjudicate_reviewed_non_shader_spans(report)
            self.assertEqual(row["disposition"], "reviewed_non_shader_span")
            self.assertEqual(report["registry"]["challenger_gap_count"], 0)
            with self.assertRaisesRegex(ValueError, "no longer match"):
                extractor.adjudicate_reviewed_non_shader_spans({"registry": {"sources": []}})
        finally:
            extractor.REVIEWED_NON_SHADER_SPANS = original

    def test_function_catalog_emitter_requires_exact_review_digest_and_zero_unknowns(self):
        source = {"path": "EXT/EXT_example.txt", "source_sha256": "a" * 64,
                  "name_strings": ["GL_EXT_example"],
                  "candidates": [{"name": "realBuiltinEXT", "return_type": "vec4",
                                  "line_in_normative_section": 8, "status": "candidate_prototype"}],
                  "challenger_gaps": []}
        empty = {"source_count": 0, "challenger_gap_count": 0, "sources": []}
        report = {"glsl": empty,
                  "registry": {"source_count": 1, "challenger_gap_count": 0, "sources": [source]}}
        review = self.accepted_review(report)
        with self.assertRaisesRegex(ValueError, "review digest"):
            extractor.build_function_catalog(
                report, {**review, "accepted_candidate_sha256": "0" * 64},
                {"glsl": 0, "registry": 1})
        catalog = extractor.build_function_catalog(report, review, {"glsl": 0, "registry": 1})
        self.assertEqual(catalog["functions_by_extension"], {"GL_EXT_example": ["realBuiltinEXT"]})
        self.assertEqual(catalog["function_provenance"], [{
            "extension": "GL_EXT_example", "repository": "registry",
            "path": "EXT/EXT_example.txt", "source_sha256": "a" * 64,
            "line": 8, "name": "realBuiltinEXT", "return_type": "vec4",
            "status": "candidate_prototype",
        }])
        self.assertEqual(catalog["source_review_receipts"], {})
        source["candidates"].append({"name": "uncertain", "return_type": "newType",
                                     "line_in_normative_section": 9, "status": "unclassified_return_type"})
        with self.assertRaisesRegex(ValueError, "unclassified"):
            extractor.build_function_catalog(report, review, {"glsl": 0, "registry": 1})

    def test_function_review_rejects_pending_verdict_and_wrong_counts(self):
        source = {"path": "EXT/EXT_example.txt", "source_sha256": "a" * 64,
                  "name_strings": ["GL_EXT_example"],
                  "candidates": [{"name": "realBuiltinEXT", "return_type": "vec4",
                                  "line_in_normative_section": 8,
                                  "status": "candidate_prototype"}],
                  "challenger_gaps": []}
        empty = {"revision": "g", "archive_sha256": "gh", "source_count": 0,
                 "challenger_gap_count": 0, "sources": []}
        report = {"glsl": empty,
                  "registry": {"revision": "r", "archive_sha256": "rh", "source_count": 1,
                               "challenger_gap_count": 0, "sources": [source]}}
        review = self.accepted_review(report)
        with self.assertRaisesRegex(ValueError, "verdict"):
            extractor.build_function_catalog(report, {**review, "verdict": "pending"},
                                             {"glsl": 0, "registry": 1})
        with self.assertRaisesRegex(ValueError, "counts"):
            extractor.build_function_catalog(
                report, {**review, "accepted_candidate_count": 2},
                {"glsl": 0, "registry": 1})

    def test_registry_review_is_exact_complete_and_promotes_only_builtins(self):
        candidates = [
            {"name": "realBuiltinEXT", "return_type": "vec4",
             "line_in_normative_section": 8,
             "status": "unclassified_no_modifications_section"},
            {"name": "HostCall", "return_type": "void",
             "line_in_normative_section": 12,
             "status": "unclassified_outside_function_section"},
        ]
        source = {"path": "EXT/EXT_example.txt", "source_sha256": "a" * 64,
                  "name_strings": ["GL_EXT_example"], "candidates": candidates,
                  "challenger_gaps": []}
        report = {"registry": {"revision": "registry-pin", "archive_sha256": "registry-hash",
                               "source_count": 1, "challenger_gap_count": 0,
                               "sources": [source]}}
        review = {"source_revision": "registry-pin", "source_archive_sha256": "registry-hash",
                  "candidate_identity_schema": extractor.REGISTRY_REVIEW_CANDIDATE_SCHEMA,
                  "row_count": 2, "rows": [
                      {"source": source["path"], "source_sha256": source["source_sha256"],
                       "line": 8, "name": "realBuiltinEXT", "return_type": "vec4",
                       "original_status": "unclassified_no_modifications_section",
                       "disposition": "builtin_declaration"},
                      {"source": source["path"], "source_sha256": source["source_sha256"],
                       "line": 12,
                       "candidate_sha256": extractor.registry_review_candidate_sha256(
                           source["source_sha256"], 12, "HostCall", "void"),
                       "original_status": "unclassified_outside_function_section",
                       "disposition": "host_api"},
                  ]}
        extractor.adjudicate_registry_review(report, review)
        self.assertEqual([row["status"] for row in candidates],
                         ["candidate_reviewed_prototype", "excluded_source_reviewed_host_api"])
        incomplete = {**review, "row_count": 1, "rows": review["rows"][:1]}
        candidates[0]["status"] = "unclassified_no_modifications_section"
        candidates[1]["status"] = "unclassified_outside_function_section"
        with self.assertRaisesRegex(ValueError, "review rows do not exactly match"):
            extractor.adjudicate_registry_review(report, incomplete)

    def test_registry_rejected_review_digest_is_fail_closed(self):
        candidate = {"name": "metadataToken", "return_type": "contactToken",
                     "line_in_normative_section": 12,
                     "status": "unclassified_no_modifications_section"}
        source = {"path": "EXT/EXT_example.txt", "source_sha256": "a" * 64,
                  "candidates": [candidate]}
        report = {"registry": {"revision": "registry-pin", "archive_sha256": "registry-hash",
                               "sources": [source]}}
        identity = extractor.registry_review_candidate_sha256(
            source["source_sha256"], 12, candidate["name"], candidate["return_type"])
        row = {"source": source["path"], "source_sha256": source["source_sha256"],
               "line": 12, "candidate_sha256": identity,
               "original_status": "unclassified_no_modifications_section",
               "disposition": "prose_or_metadata_non_signature"}
        base = {"source_revision": "registry-pin", "source_archive_sha256": "registry-hash",
                "candidate_identity_schema": extractor.REGISTRY_REVIEW_CANDIDATE_SCHEMA,
                "row_count": 1, "rows": [row]}

        stale = {**base, "rows": [{**row, "candidate_sha256":
                                    extractor.registry_review_candidate_sha256(
                                        source["source_sha256"], 12, "staleToken", "contactToken")}]}
        duplicate = {**base, "row_count": 2, "rows": [row, dict(row)]}
        missing = {**base, "rows": [{key: value for key, value in row.items()
                                      if key != "candidate_sha256"}]}
        changed = ("0" if identity[0] != "0" else "1") + identity[1:]
        tampered = {**base, "rows": [{**row, "candidate_sha256": changed}]}
        raw_name = {**base, "rows": [{**row, "name": "metadataToken"}]}
        raw_return = {**base, "rows": [{**row, "return_type": "contactToken"}]}
        wrong_source = {**base, "rows": [{**row, "source": "EXT/EXT_other.txt"}]}
        wrong_source_hash = {**base, "rows": [{**row, "source_sha256": "b" * 64}]}
        wrong_line = {**base, "rows": [{**row, "line": 13}]}
        wrong_schema = {**base, "candidate_identity_schema": "unsupported"}
        for label, review in (("stale", stale), ("duplicate", duplicate),
                              ("missing", missing), ("tampered", tampered),
                              ("raw-name", raw_name), ("raw-return-type", raw_return),
                              ("wrong-source", wrong_source), ("wrong-source-hash", wrong_source_hash),
                              ("wrong-line", wrong_line), ("wrong-schema", wrong_schema)):
            candidate["status"] = "unclassified_no_modifications_section"
            with self.subTest(label=label), self.assertRaisesRegex(ValueError, "registry review"):
                extractor.adjudicate_registry_review(report, review)

    def test_tracked_registry_review_redacts_every_rejected_candidate(self):
        review = json.loads(Path(__file__).with_name(
            "registry-ambiguous-source-review.json").read_text())
        self.assertEqual(review["candidate_identity_schema"],
                         extractor.REGISTRY_REVIEW_CANDIDATE_SCHEMA)
        rejected = [row for row in review["rows"]
                    if row["disposition"] != "builtin_declaration"]
        admitted = [row for row in review["rows"]
                    if row["disposition"] == "builtin_declaration"]
        self.assertTrue(rejected)
        self.assertTrue(all("candidate_sha256" in row and "name" not in row
                            and "return_type" not in row for row in rejected))
        self.assertTrue(all("candidate_sha256" not in row and "name" in row
                            and "return_type" in row for row in admitted))

    def test_non_shader_rows_require_pinned_host_command_membership(self):
        host = {"name": "glHostFunction", "return_type": "void",
                "line_in_normative_section": 19,
                "status": "unclassified_outside_shader_section"}
        prose = {"name": "Lookup", "return_type": "Table",
                 "line_in_normative_section": 20,
                 "status": "unclassified_outside_shader_section"}
        report = {"registry": {"sources": [{"candidates": [host, prose]}]}}
        extractor.adjudicate_host_api_sections(report, {"glHostFunction"})
        self.assertEqual(host["status"], "excluded_verified_host_api")
        self.assertEqual(host["review_basis"],
                         "non-GLSL section and exact command in pinned Registry XML")
        self.assertEqual(prose["status"], "unclassified_outside_shader_section")

    def test_registry_xml_command_variants_are_case_sensitive(self):
        xml = b'''<registry><commands>
          <command><proto>void <name>glCreateThingEXT</name></proto></command>
          <command><proto>void <name>glXChooseThingEXT</name></proto></command>
          <command><proto>void <name>wglUseThingEXT</name></proto></command>
        </commands></registry>'''
        names = extractor.registry_command_names({"gl.xml": xml, "glx.xml": xml, "wgl.xml": xml})
        self.assertTrue({"glCreateThingEXT", "CreateThingEXT", "glXChooseThingEXT",
                         "ChooseThingEXT", "wglUseThingEXT", "UseThingEXT"} <= names)
        self.assertNotIn("createthingext", names)

    def test_outside_shader_review_is_exhaustive_and_can_promote_a_builtin(self):
        builtin = {"name": "lateBuiltinEXT", "return_type": "vec4",
                   "line_in_normative_section": 30,
                   "status": "unclassified_outside_shader_section"}
        prose = {"name": "Lookup", "return_type": "Table",
                 "line_in_normative_section": 31,
                 "status": "unclassified_outside_shader_section"}
        source = {"path": "EXT/EXT_example.txt", "source_sha256": "a" * 64,
                  "candidates": [builtin, prose]}
        report = {"registry": {"revision": "registry-pin", "archive_sha256": "registry-hash",
                               "sources": [source]}}
        review = {"source_revision": "registry-pin", "source_archive_sha256": "registry-hash",
                  "row_count": 2, "rows": [
                      {"source": source["path"], "source_sha256": source["source_sha256"],
                       "line": 30, "name": "lateBuiltinEXT", "return_type": "vec4",
                       "original_status": "unclassified_outside_shader_section",
                       "disposition": "shader_builtin_declaration"},
                      {"source": source["path"], "source_sha256": source["source_sha256"],
                       "line": 31, "name": "Lookup", "return_type": "Table",
                       "original_status": "unclassified_outside_shader_section",
                       "disposition": "prose_or_metadata_non_signature"},
                  ]}
        extractor.adjudicate_outside_shader_review(report, review)
        self.assertEqual([row["status"] for row in source["candidates"]],
                         ["candidate_reviewed_prototype",
                          "excluded_source_reviewed_prose_or_metadata_non_signature"])
        source["candidates"][0]["status"] = "unclassified_outside_shader_section"
        source["candidates"][1]["status"] = "unclassified_outside_shader_section"
        with self.assertRaisesRegex(ValueError, "exactly match"):
            extractor.adjudicate_outside_shader_review(
                report, {**review, "row_count": 1, "rows": review["rows"][:1]})

    def test_registry_review_survives_concordant_scanner_improvement(self):
        candidate = {"name": "realBuiltinEXT", "return_type": "vec4",
                     "line_in_normative_section": 8, "status": "candidate_prototype"}
        source = {"path": "EXT/EXT_example.txt", "source_sha256": "a" * 64,
                  "candidates": [candidate]}
        report = {"registry": {"revision": "registry-pin", "archive_sha256": "registry-hash",
                               "sources": [source]}}
        review = {"source_revision": "registry-pin", "source_archive_sha256": "registry-hash",
                  "candidate_identity_schema": extractor.REGISTRY_REVIEW_CANDIDATE_SCHEMA,
                  "row_count": 1, "rows": [{
                      "source": source["path"], "source_sha256": source["source_sha256"],
                      "line": 8, "name": "realBuiltinEXT", "return_type": "vec4",
                      "original_status": "unclassified_no_modifications_section",
                      "disposition": "builtin_declaration",
                  }]}
        extractor.adjudicate_registry_review(report, review)
        self.assertEqual(candidate["status"], "candidate_reviewed_prototype")

    def test_archive_digest_is_fail_closed(self):
        with tempfile.TemporaryDirectory() as dirname:
            path = Path(dirname) / "source.tar.gz"
            path.write_bytes(b"wrong archive")
            with self.assertRaisesRegex(ValueError, "archive digest mismatch"):
                extractor.read_archive(path, "0" * 64)



if __name__ == "__main__":
    unittest.main()
