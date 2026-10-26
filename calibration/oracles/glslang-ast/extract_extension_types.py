#!/usr/bin/env python3
"""Extract GLSL extension type facts from independently pinned Khronos archives.

Types are reserved/built-in identifiers. ``constructors`` is narrower: only a
normative constructor spelling is credited. Generic meta-types never enter either.
"""
from collections import defaultdict
import hashlib
import re
import tarfile
from pathlib import Path

GLSL_SHA256 = "4ffb1bb817c970670644ea2d19168dc1127a1d5502eb6bbfeb6600859090a23d"
GLSL_REVISION = "0099bf83b0288469917fc2e2972cb0a2b74d6711"
REGISTRY_SHA256 = "fa80b86e5c2d91c3088b67a128d4070f2520dac6bb823f0538f4870ec9e50942"
REGISTRY_REVISION = "6af574a14089ccfee87efe230ebcdd8742859813"

ARITHMETIC = "ext/GL_EXT_shader_explicit_arithmetic_types.txt"
SIMPLE = {
    "ext/GL_EXT_bfloat16.txt": {"GL_EXT_bfloat16": ("bfloat16_t", "bf16vec")},
    "ext/GL_EXT_float8_e5m2_e4m3.txt": {
        "GL_EXT_float_e5m2": ("floate5m2_t", "fe5m2vec"),
        "GL_EXT_float_e4m3": ("floate4m3_t", "fe4m3vec")},
    "ext/GL_EXT_ocp_microscaling_types.txt": {
        "GL_EXT_float_e2m1": ("floate2m1_t", "fe2m1vec"),
        "GL_EXT_float_e3m2": ("floate3m2_t", "fe3m2vec"),
        "GL_EXT_float_e2m3": ("floate2m3_t", "fe2m3vec"),
        "GL_EXT_float_ue8m0": ("floatue8m0_t", "fue8m0vec"),
        "GL_EXT_float_mxint8": ("floatmxint8_t", "fmxint8vec")},
}
NAMED = {
    "ext/GLSL_EXT_ray_query.txt": {"GL_EXT_ray_query": ["rayQueryEXT", "accelerationStructureEXT"]},
    "ext/GLSL_EXT_ray_tracing.txt": {"GL_EXT_ray_tracing": ["accelerationStructureEXT"]},
    "nv/GLSL_NV_ray_tracing.txt": {"GL_NV_ray_tracing": ["accelerationStructureNV"]},
    "arm/GL_ARM_tensors.txt": {"GL_ARM_tensors": ["tensor", "tensorARM"]},
    "khr/GLSL_KHR_cooperative_matrix.txt": {"GL_KHR_cooperative_matrix": ["coopmat"]},
    "nv/GLSL_NV_cooperative_matrix.txt": {"GL_NV_cooperative_matrix": ["fcoopmatNV", "icoopmatNV", "ucoopmatNV"]},
    "nv/GLSL_NV_cooperative_vector.txt": {"GL_NV_cooperative_vector": ["coopvecNV"]},
    "nv/GLSL_NV_shader_invocation_reorder.txt": {"GL_NV_shader_invocation_reorder": ["hitObjectNV"]},
    "ext/GLSL_EXT_shader_invocation_reorder.txt": {"GL_EXT_shader_invocation_reorder": ["hitObjectEXT"]},
    "ext/GLSL_EXT_shader_tile_image.txt": {"GL_EXT_shader_tile_image":
                                           ["attachmentEXT", "iattachmentEXT", "uattachmentEXT"]},
    "ext/GLSL_EXT_long_vector.txt": {"GL_EXT_long_vector": ["vector"]},
    "ext/GLSL_EXT_spirv_intrinsics.txt": {"GL_EXT_spirv_intrinsics": ["spirv_type"]},
}
IMAGE64 = "ext/GLSL_EXT_shader_image_int64.txt"
STORAGE = "ext/GL_EXT_shader_16bit_storage.txt"
VULKAN = "khr/GL_KHR_vulkan_glsl.txt"
NAMED["nv/GLSL_NV_displacement_micromap.txt"] = {
    "GL_NV_displacement_micromap": ["accelerationStructureEXT"]}
TENSOR2 = "nv/GLSL_NV_cooperative_matrix2.txt"
REGISTRY_TYPE_SOURCES = {
    "OES/OES_EGL_image_external.txt",
    "OES/OES_EGL_image_external_essl3.txt",
    "EXT/EXT_YUV_target.txt",
    "AMD/AMD_gpu_shader_half_float_fetch.txt",
    "AMD/AMD_gpu_shader_half_float.txt",
    "AMD/AMD_gpu_shader_int16.txt",
    "AMD/AMD_gpu_shader_int64.txt",
    "ARB/ARB_gpu_shader_int64.txt",
    "ARB/ARB_gpu_shader_fp64.txt",
    "ARB/ARB_shader_image_load_store.txt",
    "EXT/EXT_shader_image_load_store.txt",
    "EXT/EXT_texture_cube_map_array.txt",
    "OES/OES_texture_cube_map_array.txt",
    "EXT/EXT_texture_buffer.txt",
    "OES/OES_texture_buffer.txt",
    "NV/NV_gpu_shader5.txt",
    "ARB/ARB_shader_atomic_counters.txt",
    "ARB/ARB_texture_cube_map_array.txt",
    "ARB/ARB_texture_multisample.txt",
    "EXT/EXT_texture_array.txt",
    "OES/OES_texture_storage_multisample_2d_array.txt",
    "NV/NV_explicit_multisample.txt",
    "NV/NV_shadow_samplers_array.txt",
    "NV/NV_shadow_samplers_cube.txt",
    "NV/NV_shader_buffer_load.txt",
    "NV/NV_EGL_stream_consumer_external.txt",
    "AMD/AMD_vertex_shader_tessellator.txt",
    "EXT/EXT_gpu_shader4.txt",
    "EXT/EXT_shadow_samplers.txt",
    "NV/NV_non_square_matrices.txt",
    "NV/NV_texture_array.txt",
    "OES/OES_texture_3D.txt",
    "ARB/ARB_texture_rectangle.txt",
}
# Broad challenger hits which do not declare a concrete GLSL type. Each row
# binds the disposition to the pinned source bytes and a 1-based source line.
REGISTRY_NON_TYPE_REASONS = (
    'OpenGL host API typedef/object/token/type terminology, not a GLSL data type',
    'qualifier or reserved-keyword amendment, not a concrete GLSL data type',
    'normative New Types section says None',
    'rule over existing or separately introduced GLSL types; no concrete type declared here',
    'shader/program/primitive stage terminology, not a GLSL data-type declaration',
    'built-in function/variable declaration or rule over existing types',
    'revision-history or explanatory prose triggered the broad scan; no GLSL type declaration',
)
REGISTRY_NON_TYPE_SOURCES = {
    'AMD/AMD_debug_output.txt': ('dcca3609afd24e53d4064c7aadae9022c52e5e7f3b1320d74fb15c5773bb983a', 123, 0),
    'AMD/AMD_shader_explicit_vertex_parameter.txt': ('c4e830469a721b73d471ddea45c28147fbfd9b9329ed0cc398a65d8a87b97097', 81, 1),
    'ANGLE/ANGLE_translated_shader_source.txt': ('555595547c1cdb6891981c75a060af18ee0d3c16ab0249de1f95541b93cbb751', 54, 2),
    'ARB/ARB_ES2_compatibility.txt': ('638cdb0a7647027ffdb44c6c80bf1dc74b0de69e03b8af265c6a98a2d7aafb39', 123, 0),
    'ARB/ARB_ES3_compatibility.txt': ('7cc0b3067399c107bf17dee5c59101b815449bdfc58b923a3a90784e542d8c19', 377, 0),
    'ARB/ARB_bindless_texture.txt': ('aebe0ea1dc63dfe21eacb17cb6f92f6e3442bc31c0823e7652dc5cb81f4cae51', 101, 3),
    'ARB/ARB_compute_shader.txt': ('13ccba75666fef01700d471268e668c9e8e421911a8b646f71c61948fc69a2a2', 67, 4),
    'ARB/ARB_compute_variable_group_size.txt': ('47e25420ad6034018cc8a4f5a529b5b3ca4cede96d79199b3557c2173f9a6629', 199, 1),
    'ARB/ARB_cull_distance.txt': ('1de1643657da3e98846865c20c739967b6dc9c20e543d9a4756274ec079264ae', 76, 2),
    'ARB/ARB_debug_output.txt': ('cfcd02eab72ee39f0f3deefa7403760e86c3cc0e713c2e0a6dc230ed07f0968e', 176, 0),
    'ARB/ARB_explicit_attrib_location.txt': ('7ee71f570249811b5d75202b2f5f78e255ab4ffb1690dce72bea25211862eaef', 232, 1),
    'ARB/ARB_fragment_coord_conventions.txt': ('d429a46a6cfaafa517bfb1b8ecd1d4f31731839db035aa45125f3bba6864e1ff', 178, 1),
    'ARB/ARB_geometry_shader4.txt': ('35fc0e1c38e50968b27f6d41d8dfe02d694686abe93012e4555297b44fb2e1f0', 71, 4),
    'ARB/ARB_gl_spirv.txt': ('45ec29db073ff9a6a6c7065f63d47b6d3c7a822ee8c43efcd30a30b0f8051beb', 419, 3),
    'ARB/ARB_gpu_shader5.txt': ('debfa409fb6ba536294657997912af173398e026dd033956441464eb843a510e', 94, 3),
    'ARB/ARB_program_interface_query.txt': ('67a3eea83d11a2e521555cfa0eb44d807f67937ea16ee713a428829b8d775be5', 1457, 0),
    'ARB/ARB_provoking_vertex.txt': ('e2b57a9393d7d72951cdc509f09244d11237ee27a249b9a9af4e1910cd75ca25', 526, 4),
    'ARB/ARB_sampler_objects.txt': ('34c62f1470f5e51e36cbfb15b7cfebea811a84ee4026f4c9f0003cc8dd79c2ab', 75, 0),
    'ARB/ARB_shader_objects.txt': ('5f008cd7e5d494810a5bd0a496358ca8cd24547f4c208397d47a66776f24383b', 118, 0),
    'ARB/ARB_shader_precision.txt': ('ecf890406953be37e870f2cb68243b511abae4613624058cbba8744e8cd17d25', 80, 2),
    'ARB/ARB_shader_storage_buffer_object.txt': ('9e61e06c475184a5d5c63b0769be9fd118376570c71d3dc48b040f0271887fb1', 464, 1),
    'ARB/ARB_shader_subroutine.txt': ('41c06fbefdb298bf0f55b24fb7fae8ca424a32b14005299cc63b8cffe0f503ab', 328, 1),
    'ARB/ARB_shading_language_100.txt': ('f9c384385a9a65226fdd011f2da5e974900eea2f2f25de82a64e6e6a29c4612e', 116, 0),
    'ARB/ARB_shading_language_420pack.txt': ('55382a5b6ffa98621ba2f5a3e478e2326864e990da5024f791cbecc625260d93', 85, 3),
    'ARB/ARB_shading_language_include.txt': ('35e585da722b3b403625b81f952f19b4822c78360650fbf713b4032dcbfc3800', 298, 0),
    'ARB/ARB_shading_language_packing.txt': ('67e51fff7f8652717b62ae0e072e778f1f3e9c28ec3ed2e5df2ff41bec1bf015', 235, 3),
    'ARB/ARB_sparse_texture2.txt': ('e38fe533526f1e77fe4f1ff4fea7fa05565ce6550a1d07d25c966cbf46d33813', 243, 5),
    'ARB/ARB_sparse_texture_clamp.txt': ('adf39d239588060c7edbf5a2c77e284c6c9edf6072dc3926eb3a8cb96ed28e4e', 560, 5),
    'ARB/ARB_tessellation_shader.txt': ('9dfef9d67b041f4d7a38e70bd6646a267827a81f72b6723eaf551233953a3d12', 79, 4),
    'ARB/ARB_texture_buffer_object.txt': ('1f0d5715058c993311d45ed14e59858e7debb6f492ce6fd0a2299caf547b4148', 63, 0),
    'ARB/ARB_uniform_buffer_object.txt': ('08e705825444ba78b455deab368af61045abb567f5e91ccc61f1688347d8f790', 233, 2),
    'ARB/ARB_vertex_attrib_64bit.txt': ('54d1fd94d3d2fbc56945df02d0c222039bdb84e0c7a7ed3f386dd7872b173d62', 494, 3),
    'ARB/ARB_vertex_shader.txt': ('d803f93fa003bfdd0ffb61d94e34e3ca76efb32a475399af363a99adb961c64b', 121, 5),
    'ARM/ARM_shader_core_properties.txt': ('abc6042718f87d48b3689cd47b1c185ffd39e42589f6436c8ba83c701b98f37b', 181, 0),
    'ARM/ARM_shader_framebuffer_fetch.txt': ('d4f140eca3909b396de31732d6fe5d8223011661ad1887a94215da316e369d0b', 75, 5),
    'ARM/ARM_shader_framebuffer_fetch_depth_stencil.txt': ('63b28766c62e15936da88752747bd87be5afa008e9810faad7ab8c82e2b9b0fe', 77, 5),
    'EXT/EXT_bindable_uniform.txt': ('fcd5ca09b5d6561f64e38d9a1801e99f6d0236a987fbb5e33af7a55fd90404ea', 421, 1),
    'EXT/EXT_blend_func_extended.txt': ('7223ff5c3f5484d86c5dca9eafbef79477d30594ba9cdc62b32d5d95ad58c647', 351, 1),
    'EXT/EXT_frag_depth.txt': ('2bece5dfa6f357c176fb96d02bd221ec4fe2b37d9f73d88a1239f6d5d2e60cfa', 110, 5),
    'EXT/EXT_framebuffer_object.txt': ('cc30255f8d0c1e2b00799b69a525be5335bbbf0779e880e611f977c6586335be', 128, 0),
    'EXT/EXT_geometry_shader.txt': ('dadffc9dc14ba880598af910ea2e34435819a52d13f82c55648602204af29e9d', 70, 4),
    'EXT/EXT_geometry_shader4.txt': ('461e69ce328027039be61194120c5de070dec995ea93de51fa548323fe50a4e5', 54, 4),
    'EXT/EXT_gpu_shader5.txt': ('66a0221ea28829de24cadb9c077efa34e911bdd4bff80e367a2661112b803376', 348, 3),
    'EXT/EXT_mesh_shader.txt': ('b3813daafa27ff5ae769cd835bb976a15708ef61cfb78b766c4c434958f1ad2b', 55, 4),
    'EXT/EXT_packed_float.txt': ('7e6d3d7e61fa7f8f3294d23fb929cda6e6ce627376530a7e1510d2c3fef39023', 427, 0),
    'EXT/EXT_provoking_vertex.txt': ('2fe3926ff6d1fafd21b5c6877be559cda6ca5abc2d80a5e76da40547bf028e88', 434, 4),
    'EXT/EXT_shader_framebuffer_fetch.txt': ('42e43ce1fbd5533c760244bd0b117e19cf4aa2735800da7746480ca94069f729', 71, 1),
    'EXT/EXT_shader_implicit_conversions.txt': ('902599679d0f08ab9996d7f86c6cf3b6ae9abeeb2638cb78c81167cdc10ac5c8', 49, 3),
    'EXT/EXT_shader_io_blocks.txt': ('3cdbcc8a08715b2b2c3d7807136c887e142ac0ac75ad6da95439ac5edadde6af', 261, 1),
    'EXT/EXT_shader_pixel_local_storage.txt': ('0b67ef0e42b35bddf2b39bd3f03b0ece3ad90febb60b21e7e4e83b7a6a5e340e', 116, 1),
    'EXT/EXT_shader_pixel_local_storage2.txt': ('6a1bdfa27c9e71962adb204984b63970532f283469362a8b672f61ccb0e983f7', 85, 5),
    'EXT/EXT_sparse_texture2.txt': ('a8c54287d1505c50c3567d53af42abe5f89ad867486d183cfe0e87494422790d', 262, 5),
    'EXT/EXT_tessellation_shader.txt': ('b48ff0838050c3f823e4ad6a6deb81bada74f3eaf3166bec6ed91c6520ada06e', 71, 4),
    'EXT/EXT_texture_buffer_object.txt': ('f2131251abff603eb555d75bb290bfe2a759b4847a43458ba87e65da55e4c16a', 49, 0),
    'EXT/EXT_texture_shared_exponent.txt': ('7f4bf01aea3c7feb543e435afc3c9424c9ce9cf1ecdf563d3025beaa64ce4b94', 324, 0),
    'EXT/EXT_texture_view.txt': ('e9b127ff51c7d41acf47e3aa7db88a9404b9ca92aa7d86fa7fcb3066ca8df19e', 76, 0),
    'EXT/EXT_vertex_attrib_64bit.txt': ('37ddfde0418b4502d384251862c2c0bf3bb5ae709f6e06518d2020bc48b089ad', 450, 3),
    'IMG/IMG_bindless_texture.txt': ('a008ff7b97a2297bef3d3bdc04d881b8411b4a8a3518e4633842983704f046e7', 59, 3),
    'IMG/IMG_pvric_end_to_end_signature.txt': ('be3f43adc39f716f6cd454f033f70a2c5b90a31e2cd63e3dab90094f4e86a496', 74, 2),
    'IMG/IMG_tile_region_protection.txt': ('f2eb450bb97fe160d65aa9411c2824c33790dd5bf1548f4e1e5a97d8adc737a7', 78, 2),
    'KHR/KHR_debug.txt': ('cbf267a4d88fd18b71b9adcd94ba4a92101a73d28579a85ee78d2940ead70383', 239, 0),
    'MESA/MESA_shader_integer_functions.txt': ('31226db770f7e87e49ad75e6b25e046410ee6cc49448fc897eef2b906fd645c9', 66, 3),
    'NV/NV_bindless_texture.txt': ('1f8f2a7ef455df699a96cb68e83f0a9465928bd884c452edc00c607ac8f29bb3', 96, 3),
    'NV/NV_command_list.txt': ('0de7699818d54e151f767557ce671573ad9f8d4d1f36c83be3fe6cc7396a846a', 813, 1),
    'NV/NV_conservative_raster.txt': ('2474ea436c1e1942a78bddb089577e7729ac255b434667b42fe2f8b9a55ff19f', 237, 6),
    'NV/NV_geometry_program4.txt': ('fd6bef871fdcb44a6f364a211e5c023a5078f6b12fba3d10a2426d441890d172', 73, 4),
    'NV/NV_geometry_shader_passthrough.txt': ('010abfe8d0c5d04901cd7b3c030e5fbd68c4ea9d2c41926e85301cad5586a6a4', 116, 1),
    'NV/NV_gpu_program4.txt': ('ceff2fe39752015eb53f44f1c28eb7eb7be3149b1fe88419ec5b6aa0ac1777d5', 68, 4),
    'NV/NV_gpu_program5.txt': ('685b96ad3bf0c4a3faa3f8b8d47d1f767dc32f306ea8f1282f582f3663a02f25', 1356, 4),
    'NV/NV_mesh_shader.txt': ('030fcdee403e1a3d384cff7e3d8b6e5386390ab110906498082d374ebe64f6ff', 60, 4),
    'NV/NV_shader_noperspective_interpolation.txt': ('d019220160ad2815b0f0f3e894acf3d6b1299c84eb3c2acc11b1e13cb57f4f08', 47, 1),
    'NV/NV_tessellation_program5.txt': ('ebd323f2043fc352e8f41e7aed10880936804e4ab1a81b57f62e89bc34a003b1', 50, 4),
    'NV/NV_vdpau_interop.txt': ('5b057f67f11f866b29a3ad92485c9b6e898f311d90d754c9ddfb40be25e36081', 62, 0),
    'NV/NV_vdpau_interop2.txt': ('8917428e102b17951bc4c7c434c4d1a413d4621057df3bf73ab0a1c58ea90a5a', 52, 2),
    'NV/NV_vertex_program3.txt': ('72eb4246a4c08d358dad690d53e1aab1b67dfbe4656d99d05203a664bfbc5936', 189, 1),
    'NV/NV_vertex_program4.txt': ('ae0bc3a3c65f374b5101b61c4c5673369f866bddde26496bd088669fb834ae33', 813, 0),
    'OES/OES_geometry_shader.txt': ('09734f05424426490d730d5f646f191c52385dfae90f5a0faac7ae9b68a87d52', 82, 4),
    'OES/OES_gpu_shader5.txt': ('17f06d8d76c5a3eab7ee9649348df7b86c288d1008c88249af1181b564e40827', 359, 3),
    'OES/OES_sample_variables.txt': ('efe81fe72023b5308157d34f83b5608bd608fbe17774e98342ebd2a3605a799e', 129, 5),
    'OES/OES_shader_io_blocks.txt': ('11ae8a5b10ca2e67a039e6fd9ee08fddc17faf5819e1efc6a6ad3bac87d03e78', 270, 1),
    'OES/OES_shader_multisample_interpolation.txt': ('5f6de4dd9e2e5482e7302314904b7bf4f18e9ba7d48a4a5ae000a58a88af61d7', 70, 1),
    'OES/OES_standard_derivatives.txt': ('326ff7c7d4eb0d3e6fd0121126b2b87b133de0aad522b609a996a4e811e88b51', 51, 5),
    'OES/OES_tessellation_shader.txt': ('b6c12aee821f8cf26cb641c5da0f3c0681c84af9da5ac5e6981d98cb78cd3f22', 82, 4),
    'OES/OES_texture_view.txt': ('ef4e06499dcecf92c8ff37c07829049f64b13a3b7d23bb2c7bf97ce083bc36e6', 91, 0),
    'QCOM/QCOM_texture_foveated_subsampled_layout.txt': ('6eeaceec8c6c09f2964ab47638f86b65f682b88f466ea95c7865ce2928f6d972', 58, 1),
}
REGISTRY_DISPOSITION_REVIEWED = True
TYPE_HEADING = re.compile(
    r"Basic Types|Opaque Types|Transparent Types|(?:Add|Adds|added)[^\n]{0,80}\bkeyword", re.I)
GLSL_HEADING_NON_TYPES = {
    "ext/GLSL_EXT_demote_to_helper_invocation.txt": ("656e0f7cf2231bba2d0dba7d9dbd3dc905b59d13037be04616aad0611bdb5cb4", 43, "jump keyword"),
    "ext/GLSL_EXT_descriptor_heap.txt": ("1055a75abe85dfbc19b518fb8de806310eb8670d1a07078deabe152cc824cf18", 119, "layout rule over opaque-type arrays"),
    "ext/GLSL_EXT_nontemporal_keyword.txt": ("835f65d5ee4034a45be0b0e3060236515e8050c7681de684a0f7947ec4de3f7e", 44, "memory operand keyword"),
    "ext/GL_EXT_subgroupuniform_qualifier.txt": ("a9aa1ef39969d75ba7ee3aace6fd29ac95d3ae292c10b8127ea9fb14366c9bef", 72, "type qualifier"),
    "ext/GL_EXT_vulkan_glsl_relaxed.txt": ("08e960f8a0d4cab63e37beb00eb1d54cac113f50f08203ef5dcef71b97b9ead0", 129, "rule over existing opaque types"),
}
REGISTRY_HEADING_NON_TYPES = {
    "ARB/ARB_ES3_1_compatibility.txt": ("461fdccbd57799f9490954d20fe69b80c740becec53ea8f3474e2f2e293206d1", 309, "rule over existing opaque types"),
    "ARB/ARB_arrays_of_arrays.txt": ("3e48e28b88c9822312c4ead37249a189c22845f7578f6933e1ac814f9f19788b", 179, "array rule over existing types"),
    "ARB/ARB_vertex_program.txt": ("a5901ee424291bcfbde88a57fd807dc6b0ce96584bb99086cd9779560d443a66", 5247, "assembly-language keyword"),
    "ARB/WGL_ARB_pixel_format.txt": ("3ed8b23bc58c66e60c62d3e047c6dbd473f7ff9d6df4d64f0e237053af48579a", 51, "host API pixel-format object"),
    "EXT/WGL_EXT_pixel_format.txt": ("e9a5220f00956190a6962ba1718803e09ff54be3fc4fa3a95bd77bc13e0e12c1", 26, "host API pixel-format object"),
    "NV/NV_path_rendering.txt": ("1d0ccff2d0fd002c59b0bd3cb0488c5f6542e223da30900c7bf7342f5f4e4ce8", 9426, "issue text about existing opaque types"),
}
REQUIRED = {ARITHMETIC, IMAGE64, STORAGE, VULKAN, TENSOR2, *SIMPLE, *NAMED}
NON_TYPE_SOURCES = {
    "ext/GLSL_EXT_buffer_reference.txt": ("buffer_reference\" layout qualifier", "layout qualifier; block names are user-defined"),
    "ext/GLSL_EXT_buffer_reference2.txt": ("GL_EXT_buffer_reference to enable array indexing", "operations on user-defined reference types"),
    "ext/GLSL_EXT_fragment_shader_barycentric.txt": ("pervertexEXT auxiliary storage qualifier", "storage qualifier"),
    "ext/GLSL_EXT_mesh_shader.txt": ("taskPayloadSharedEXT storage qualifier", "storage/layout qualifiers"),
    "ext/GLSL_EXT_shader_subgroup_extended_types.txt": ("genI8Type", "generic signature types, not concrete identifiers"),
    "ext/GLSL_EXT_structured_descriptor_heap.txt": ("new `resourceheap` and `samplerheap` blocks", "block/storage/layout qualifiers; block types are user-defined"),
    "ext/GL_EXT_nonuniform_qualifier.txt": ("nonuniformEXT type qualifier", "type qualifier, not a type"),
    "ext/GL_EXT_shader_64bit_indexing.txt": ("Add int64_t and uint64_t to the set of allowed types", "uses separately declared arithmetic types"),
    "ext/GL_EXT_shared_memory_block.txt": ("Add \"shared\" to the keywords", "existing storage qualifier on interface blocks"),
    "ext/GL_EXT_terminate_invocation.txt": ("new type of jump to the language", "jump statement, not a data type"),
    "huawei/GLSL_HUAWEI_cluster_culling_shader.txt": ("The shared qualifier is used", "shader stage and qualifier amendments"),
    "huawei/GLSL_HUAWEI_subpass_shading.txt": ("Subpass-input types are only available", "stage availability for Vulkan types"),
    "nv/GLSL_NV_explicit_typecast.txt": ("convert expressions from one type", "cast syntax for existing types"),
    "nv/GLSL_NV_fragment_shader_barycentric.txt": ("pervertexNV", "storage qualifier"),
    "nv/GLSL_NV_mesh_shader.txt": ("perprimitiveNV auxiliary storage qualifier", "storage/layout qualifiers"),
}
IDENTIFIER = re.compile(r"[A-Za-z_]\w*\Z")
META = re.compile(r"(?:gen[A-Za-z0-9_]*Type|[a-z]*vecN|[a-z]*matN|[A-Za-z_]*Type)$")


def sha256(path):
    digest = hashlib.sha256()
    with Path(path).open("rb") as source:
        for chunk in iter(lambda: source.read(1024 * 1024), b""):
            digest.update(chunk)
    return digest.hexdigest()


def concrete_type(name):
    return bool(IDENTIFIER.fullmatch(name) and not META.fullmatch(name))


def _fact(extension, name, path, source_hash, line, evidence, constructor="not_asserted", category="type",
          repository="KhronosGroup/GLSL"):
    if not concrete_type(name):
        raise ValueError("meta-type or invalid identifier: " + name)
    return {"extension": extension, "name": name, "repository": repository, "source_path": path,
            "source_sha256": source_hash, "line": line, "evidence": evidence,
            "constructor": constructor, "category": category}


def _line(source, needle, start=0):
    index = source.find(needle, start)
    if index < 0:
        raise ValueError("normative type evidence missing: " + needle)
    return source.count("\n", 0, index) + 1


def expected_arithmetic(extension):
    family = extension.rsplit("_", 1)[1]
    if family.startswith("float"):
        bits = family.removeprefix("float")
        prefix = "f" + bits
        return {"float" + bits + "_t"} | {f"{prefix}vec{n}" for n in range(2, 5)} | \
            {f"{prefix}mat{n}" for n in range(2, 5)} | \
            {f"{prefix}mat{c}x{r}" for c in range(2, 5) for r in range(2, 5)}
    if family.startswith("int"):
        bits = family.removeprefix("int")
        return {f"{sign}int{bits}_t" for sign in ("", "u")} | \
            {f"{sign}{bits}vec{n}" for sign in ("i", "u") for n in range(2, 5)}
    raise ValueError("unknown explicit arithmetic family: " + extension)


def arithmetic_types(source, path, source_hash, required_families=7):
    """Chapter 3 keyword rows must equal Chapter 4 transparent-type rows."""
    heading = "Changes to Chapter 4 of the OpenGL Shading Language Specification"
    if heading not in source:
        raise ValueError("missing Chapter 4 type-table boundary")
    keywords, table = source.split(heading, 1)
    pattern = re.compile(r"If the extension (GL_EXT_shader_explicit_arithmetic_types_\w+) is supported,\s*"
                         r"add the following keywords to section 3\.6 Keywords:\s*([\s\S]*?)"
                         r"(?=If the extension GL_EXT_shader_explicit_arithmetic_types_|\Z)", re.I)
    result = {}
    seen = set()
    for match in pattern.finditer(keywords):
        extension = match.group(1)
        seen.add(extension)
        expected = expected_arithmetic(extension)
        lines = match.group(2).strip().splitlines()
        actual = set(" ".join(lines).split())
        if actual != expected:
            raise ValueError("keyword/range mismatch for " + extension)
        block_match = re.search(r"\(if " + re.escape(extension) + r" is supported\)([\s\S]*?)"
                                r"(?=\(if GL_EXT_shader_explicit_arithmetic_types_|Modify subsection|\Z)", table)
        if block_match is None:
            raise ValueError("missing Chapter 4 table for " + extension)
        table_names = set(re.findall(r"^\s*\|\s*([A-Za-z_]\w*)\s*\|", block_match.group(1), re.M))
        if table_names != expected:
            raise ValueError("keyword/table mismatch for " + extension)
        for name in sorted(expected):
            line = _line(source, name, match.start(2))
            result[name] = _fact(extension, name, path, source_hash, line,
                                 "3.6 keyword and 4.1 transparent-type table")
    if len(seen) != required_families:
        raise ValueError("explicit arithmetic family count is incomplete")
    return result


def named_type(source, name, extension, path, source_hash, constructor=False):
    if extension not in source:
        raise ValueError("extension directive missing: " + extension)
    keyword_name = "tensor" if name == "tensorARM" else name
    if name == "spirv_type":
        keyword_region = source.split("Modify Section 3.6, Keywords", 1)[1].split("Modify Section 3.7", 1)[0]
        keyword = re.search(r"\bspirv_type\b", keyword_region)
        keyword_offset = source.index("Modify Section 3.6, Keywords") + len("Modify Section 3.6, Keywords")
        basic = re.search(r"Add a new section 4\.14\. SPIR-V Types", source)
    else:
        keyword = re.search(r"^\s*" + re.escape(keyword_name) + r"\s*$", source, re.M)
        keyword_offset = 0
        basic = re.search(r"Section 4\.(?:1|14).*?(?:Basic|Opaque|Tensor|Cooperative|SPIR-V Types)",
                          source, re.I | re.S)
    if not keyword or not basic or name not in source[basic.end():]:
        raise ValueError("named type lacks keyword and basic-type evidence: " + name)
    category = ("type_specifier" if name == "spirv_type" else
                "reserved_type_keyword" if name in {"tensor", "vector"} else
                "parameterized_type_basename" if name in {"tensorARM", "coopmat", "coopvecNV"} else
                "opaque_type" if name in {"rayQueryEXT", "accelerationStructureEXT", "accelerationStructureNV",
                                          "hitObjectEXT", "hitObjectNV", "attachmentEXT", "iattachmentEXT",
                                          "uattachmentEXT"} else "type")
    evidence_offset = source.index(name, basic.end()) if name == "tensorARM" else keyword_offset + keyword.start()
    return {name: _fact(extension, name, path, source_hash,
                        source.count("\n", 0, evidence_offset) + 1,
                        "3.6 keyword and normative type section" if name != "tensorARM" else
                        "4.1 parameterized type basename; 3.6 reserves tensor keyword",
                        "source_proven" if constructor else "not_asserted", category)}


def simple_types(source, groups, path, source_hash):
    keyword_start = re.search(r"Modify Section 3\.6, Keywords", source)
    table_start = re.search(r"Modify Section 4\.1, Basic Types", source)
    if not keyword_start or not table_start or keyword_start.end() >= table_start.start():
        raise ValueError("simple type keyword/table sections missing")
    keywords = source[keyword_start.end():table_start.start()]
    table = source[table_start.end():]
    result = {}
    for extension, (scalar, vector_prefix) in groups.items():
        if extension not in source:
            raise ValueError("type extension absent from source: " + extension)
        for name in [scalar, *(vector_prefix + str(n) for n in range(2, 5))]:
            if not re.search(r"\b" + re.escape(name) + r"\b", keywords) or not re.search(
                    r"^\s*" + re.escape(name) + r"\s+", table, re.M):
                raise ValueError("keyword/table mismatch for " + name)
            constructor_section = re.search(r"Add constructors? for[^.]*\.", source, re.I | re.S)
            constructor = ("source_proven" if name == scalar and constructor_section and
                           scalar in constructor_section.group(0) else "not_asserted")
            result[name] = _fact(extension, name, path, source_hash, _line(source, name, keyword_start.end()),
                                 "3.6 keyword and 4.1 transparent-type table", constructor)
    return result


def image64_types(source, path, source_hash):
    keyword_head = "Add the following keywords:"
    table_head = "Additions to Chapter 4 of the OpenGL Shading Language Specification"
    if keyword_head not in source or table_head not in source:
        raise ValueError("image64 keyword/type sections missing")
    keywords = source.split(keyword_head, 1)[1].split(table_head, 1)[0]
    names = set(re.findall(r"\b(?:i64image|u64image)[A-Za-z0-9]*\b", keywords))
    suffixes = {name.removeprefix("i64image").removeprefix("u64image") for name in names}
    if len(suffixes) < 10 or names != {prefix + suffix for prefix in ("i64image", "u64image") for suffix in suffixes}:
        raise ValueError("image64 signed/unsigned family is incomplete")
    table = source.split(table_head, 1)[1]
    table_names = set(re.findall(r"^\s*\|\s*((?:i64image|u64image)[A-Za-z0-9]*)\s*\|", table, re.M))
    if table_names != names:
        raise ValueError("image64 keyword/table mismatch")
    return {name: _fact("GL_EXT_shader_image_int64", name, path, source_hash,
                        _line(source, name, source.index(keyword_head)), "3.6 keyword and 4.1 opaque type table",
                        category="opaque_type") for name in sorted(names)}


def storage_types(source, path, source_hash):
    """The 8/16-bit storage spec assigns one Chapter 4 table to two directives."""
    chapter4 = "Changes to Chapter 4 of the OpenGL Shading Language Specification"
    if chapter4 not in source:
        raise ValueError("storage type table missing")
    before, after = source.split(chapter4, 1)
    expected = {
        "GL_EXT_shader_8bit_storage": {"int8_t", "uint8_t"} |
            {f"{sign}8vec{n}" for sign in ("i", "u") for n in range(2, 5)},
        "GL_EXT_shader_16bit_storage": {"float16_t", "int16_t", "uint16_t"} |
            {f"{sign}16vec{n}" for sign in ("f", "i", "u") for n in range(2, 5)},
    }
    table = after.split("Add a new subsection 4.1.X", 1)[0]
    table_names = set(re.findall(r"^\s*\|\s*([A-Za-z_]\w*)\s*\|", table, re.M)) - {"Type"}
    if table_names != set().union(*expected.values()):
        raise ValueError("storage keyword/table mismatch")
    result = {}
    for extension, names in expected.items():
        heading = f"If the extension {extension} is supported,"
        if heading not in before:
            raise ValueError("missing storage extension block: " + extension)
        block = before.split(heading, 1)[1].split("If the extension", 1)[0]
        keywords = set(re.findall(r"\b(?:float16_t|(?:int|uint)(?:8|16)_t|[fiu](?:8|16)vec[234])\b", block))
        if keywords != names:
            raise ValueError("storage keyword/range mismatch: " + extension)
        for name in sorted(names):
            result[(extension, name)] = _fact(extension, name, path, source_hash,
                                               _line(source, name, before.index(heading)),
                                               "3.6 keyword and 4.1 transparent-type table")
    return result


def vulkan_types(source, path, source_hash):
    """Vulkan GLSL adds opaque texture, sampler, and subpass type keywords."""
    head = "Add the following keywords to section 3.6 Keywords:"
    tail = "Move the following keywords in section 3.6 Keywords"
    if head not in source or tail not in source:
        raise ValueError("Vulkan type keyword block missing")
    block = source.split(head, 1)[1].split(tail, 1)[0]
    names = set(block.split())
    suffixes = ("1D", "2D", "3D", "Cube", "1DArray", "2DArray", "Buffer", "2DMS", "2DMSArray", "CubeArray")
    expected = {prefix + suffix for prefix in ("texture", "itexture", "utexture") for suffix in suffixes}
    expected |= {"sampler", "samplerShadow"}
    expected |= {prefix + suffix for prefix in ("", "i", "u") for suffix in ("subpassInput", "subpassInputMS")}
    if names != expected:
        raise ValueError("Vulkan keyword/range mismatch")
    table = source.split("Add into the tables in section 4.1 Basic Types", 1)[1].split(
        "Remove the entry from the table in section 4.1 Basic Types", 1)[0]
    sampler_table = source.split('"Sampler Opaque Types', 1)[1].split("Remove \"structure member selection\"", 1)[0]
    for name in names:
        if not re.search(r"^\s*" + re.escape(name) + r"(?:\s|$)", table + sampler_table, re.M):
            raise ValueError("Vulkan opaque type table mismatch: " + name)
    return {name: _fact("GL_KHR_vulkan_glsl", name, path, source_hash,
                        _line(source, name, source.index(head)), "3.6 keyword and 4.1 opaque-type table",
                        category="opaque_type") for name in sorted(names)}


def tensor2_types(source, path, source_hash):
    heading = "Add a new Section 4.1.X, Tensor Layout Types"
    if heading not in source or "GL_NV_cooperative_matrix2" not in source:
        raise ValueError("cooperative matrix2 tensor section missing")
    section = source.split(heading, 1)[1].split("See Section 8.X", 1)[0]
    names = set(re.findall(r"\bstruct\s+(tensor[A-Za-z]+NV)\s*<", section))
    if names != {"tensorLayoutNV", "tensorViewNV"}:
        raise ValueError("cooperative matrix2 tensor type mismatch")
    return {name: _fact("GL_NV_cooperative_matrix2", name, path, source_hash,
                        _line(source, "struct " + name + "<", source.index(heading)),
                        "4.1.X parameterized tensor type declaration", category="parameterized_type_basename")
            for name in sorted(names)}


def registry_transparent_types(path, source, source_hash):
    """Cross-check legacy AMD/ARB keyword rows against normative type tables."""
    extension = "GL_" + path.split("/", 1)[1].removesuffix(".txt")
    if path.endswith("gpu_shader_half_float.txt"):
        expected = expected_arithmetic("GL_EXT_shader_explicit_arithmetic_types_float16")
    elif path.endswith("gpu_shader_int16.txt"):
        expected = expected_arithmetic("GL_EXT_shader_explicit_arithmetic_types_int16")
    elif path.endswith("gpu_shader_int64.txt"):
        expected = expected_arithmetic("GL_EXT_shader_explicit_arithmetic_types_int64")
    elif path.endswith("gpu_shader_fp64.txt"):
        expected = {"double"} | {f"dvec{n}" for n in range(2, 5)} | \
            {f"dmat{n}" for n in range(2, 5)} | \
            {f"dmat{c}x{r}" for c in range(2, 5) for r in range(2, 5)}
    else:
        raise ValueError("unrouted Registry transparent source: " + path)
    head = re.search(r"\(add the following to the list of (?:reserved )?keywords[^\n]*\)", source, re.I)
    basic = re.search(r"Modify Section 4\.1, Basic Types", source)
    if not head or not basic or head.end() >= basic.start():
        raise ValueError("Registry transparent keyword/basic type sections missing: " + path)
    keywords = source[head.end():basic.start()].split("Additions to Chapter 4", 1)[0]
    kw_names = {name for line in keywords.splitlines()
                if re.fullmatch(r"\s*[A-Za-z_]\w*(?:\s+[A-Za-z_]\w*)*\s*", line)
                for name in line.split()}
    table = source[basic.end():].split("Modify Section 4.1.", 1)[0]
    table_names = set(re.findall(r"^\s*\|\s*([A-Za-z_]\w*)\s*\|", table, re.M))
    if path.endswith("gpu_shader_fp64.txt"):
        table_names |= set(re.findall(r"^\s*(d(?:ouble|vec[234]|mat[234](?:x[234])?))\s+", table, re.M))
    table_names &= expected
    keyword_delta = kw_names ^ expected
    documented_typo = path == "AMD/AMD_gpu_shader_half_float.txt" and keyword_delta == {
        "fl6mat2x3", "f16mat2x3"}
    if table_names != expected or (keyword_delta and not documented_typo):
        raise ValueError("Registry transparent keyword/table disagreement: " + path + " " +
                         repr((sorted(expected - table_names), sorted(keyword_delta))))
    repo = "KhronosGroup/OpenGL-Registry"
    facts = {}
    for name in sorted(expected):
        typo = documented_typo and name == "f16mat2x3"
        line = _line(source, name, basic.end()) if typo else _line(source, name, head.end())
        evidence = "4.1 type table; 3.6 typo fl6mat2x3" if typo else "3.6 keyword and 4.1 transparent type table"
        facts[name] = _fact(extension, name, path, source_hash, line, evidence,
                            repository=repo)
    return facts


def registry_image_types(path, source, source_hash):
    head = "(add the following to the list of keywords, p. 14)"
    tail = "(remove from the list of reserved keywords"
    if head not in source or tail not in source:
        raise ValueError("image keyword section missing: " + path)
    keywords = source.split(head, 1)[1].split(tail, 1)[0]
    names = set(re.findall(r"\b(?:image|iimage|uimage)[A-Za-z0-9]+\b", keywords))
    suffixes = {name.removeprefix("image").removeprefix("iimage").removeprefix("uimage") for name in names}
    if len(suffixes) != 11 or names != {prefix + suffix for prefix in ("image", "iimage", "uimage") for suffix in suffixes}:
        raise ValueError("image keyword family incomplete: " + path)
    if "Section 4.1.X, Images" not in source:
        raise ValueError("normative image type section missing: " + path)
    extension = "GL_" + path.split("/", 1)[1].removesuffix(".txt")
    return {name: _fact(extension, name, path, source_hash, _line(source, name, source.index(head)),
                        "3.6 keyword and 4.1.X image type section", category="opaque_type",
                        repository="KhronosGroup/OpenGL-Registry") for name in sorted(names)}


def registry_essl_opaque_types(path, source, source_hash):
    if "texture_cube_map_array" in path:
        head = "keywords on p. 14:"
        tail = 'Add to section 4.1 "Basic Types"'
        expected_count = 7
    else:
        head = "remove them from the list of reserved keywords on p. 15:"
        tail = 'Add to section 4.1 "Basic Types"'
        expected_count = 6
    if head not in source or tail not in source:
        raise ValueError("ESSL opaque keyword/type boundary missing: " + path)
    keywords = source.split(head, 1)[1].split(tail, 1)[0]
    names = set(re.findall(r"\b(?:[iu]?(?:sampler|image))[A-Za-z0-9]+\b", keywords))
    if len(names) != expected_count:
        raise ValueError("ESSL opaque type keyword count mismatch: " + path)
    table = source.split(tail, 1)[1].split("Modify the second paragraph", 1)[0]
    for name in names:
        if not re.search(r"^\s*" + re.escape(name) + r"\s*(?:$|\s)", table, re.M):
            raise ValueError("ESSL opaque type table mismatch: " + name)
    extension = "GL_" + path.split("/", 1)[1].removesuffix(".txt")
    return {name: _fact(extension, name, path, source_hash, _line(source, name, source.index(head)),
                        "ESSL keyword and 4.1 opaque type table", category="opaque_type",
                        repository="KhronosGroup/OpenGL-Registry") for name in sorted(names)}


def nv_gpu_shader5_types(source, path, source_hash):
    head = "(add the following to the list of reserved keywords)"
    tail = "(note:  the \"float64_t\""
    table_head = '(add to the basic "Transparent Types" table, p. 18)'
    if head not in source or tail not in source or table_head not in source:
        raise ValueError("NV gpu_shader5 keyword/type sections missing")
    keywords = source.split(head, 1)[1].split(tail, 1)[0]
    names = set(re.findall(r"\b(?:(?:int|uint)(?:8|16|32|64)_t|float(?:16|32|64)_t|[iuf](?:8|16|32|64)vec[234])\b", keywords))
    if len(names) != 44:
        raise ValueError("NV gpu_shader5 scalar/vector family incomplete")
    table = source.split(table_head, 1)[1].split("Modify Section 4.1.", 1)[0]
    table_names = set(re.findall(r"^\s+([A-Za-z_]\w*)\s+", table, re.M)) & names
    if table_names != names:
        raise ValueError("NV gpu_shader5 keyword/table mismatch: " + repr(sorted(names - table_names)))
    return {name: _fact("GL_NV_gpu_shader5", name, path, source_hash,
                        _line(source, name, source.index(head)), "3.6 keyword and 4.1 transparent type table",
                        repository="KhronosGroup/OpenGL-Registry") for name in sorted(names)}


REGISTRY_BASIC_FAMILIES = {
    "ARB/ARB_texture_cube_map_array.txt": 4,
    "ARB/ARB_texture_multisample.txt": 6,
    "EXT/EXT_texture_array.txt": 4,
    "OES/OES_texture_storage_multisample_2d_array.txt": 3,
    "NV/NV_explicit_multisample.txt": 3,
}


def registry_basic_family(path, source, source_hash):
    """Check comma-separated shader keywords against the adjacent 4.1 rows."""
    start = re.search(r"Add to section 3\.[67] [\"“]Keywords[\"”]", source, re.I)
    end = re.search(r"Add to section 4\.1 [\"“]Basic Types[\"”]", source, re.I)
    if not start or not end or start.end() >= end.start():
        raise ValueError("Registry basic type amendment sections missing: " + path)
    keywords = source[start.end():end.start()]
    names = set(re.findall(r"\b(?:[iu]?sampler|[iu]?image)[A-Za-z0-9]+\b", keywords))
    if len(names) != REGISTRY_BASIC_FAMILIES[path]:
        raise ValueError("Registry basic type keyword count mismatch: " + path)
    table = source[end.end():].split("Add to section 8.", 1)[0]
    for name in names:
        if not re.search(r"^\s*" + re.escape(name) + r"\s+(?:a |handle)", table, re.M):
            raise ValueError("Registry basic type table mismatch: " + path + ":" + name)
    extension = "GL_" + path.split("/", 1)[1].removesuffix(".txt")
    return {name: _fact(extension, name, path, source_hash, _line(source, name, start.end()),
                        "3.6/3.7 keyword and 4.1 basic type declaration", category="opaque_type",
                        repository="KhronosGroup/OpenGL-Registry") for name in sorted(names)}


def registry_single_basic_type(path, source, source_hash):
    if path == "ARB/ARB_shader_atomic_counters.txt":
        name = "atomic_uint"
        first = source.index("Add to 3.6 Keywords")
        second = source.index("Add to Section 4.1 Basic Types")
        if not (first < source.index(name, first) < second < source.index(name, second)):
            raise ValueError("atomic counter keyword/type disagreement")
        evidence = "3.6 keyword and 4.1 atomic-counter type"
    elif path in {"NV/NV_shadow_samplers_array.txt", "NV/NV_shadow_samplers_cube.txt"}:
        name = "sampler2DArrayShadowNV" if "array" in path else "samplerCubeShadowNV"
        first = source.index("Append the following row to the unnamed table")
        if name not in source[first:]:
            raise ValueError("NV shadow sampler type row missing")
        evidence = "ESSL 4.1 basic type row"
    else:
        raise ValueError("unrouted single basic type source: " + path)
    extension = "GL_" + path.split("/", 1)[1].removesuffix(".txt")
    return {name: _fact(extension, name, path, source_hash, _line(source, name, first), evidence,
                        category="opaque_type", repository="KhronosGroup/OpenGL-Registry")}


def registry_pointer_types(source, path, source_hash):
    head = "(add the following to the list of reserved keywords)"
    table_head = '(add to the basic "Transparent Types" table, p. 18)'
    if head not in source or table_head not in source:
        raise ValueError("NV pointer keyword/type sections missing")
    keywords = source.split(head, 1)[1].split("Modify Section 4.1", 1)[0]
    names = set(re.findall(r"\b(?:intptr_t|uintptr_t)\b", keywords))
    table = source.split(table_head, 1)[1].split("(replace the last paragraph", 1)[0]
    if names != {"intptr_t", "uintptr_t"} or not all(
            re.search(r"^\s*" + name + r"\s+", table, re.M) for name in names):
        raise ValueError("NV pointer keyword/type table mismatch")
    return {name: _fact("GL_NV_shader_buffer_load", name, path, source_hash,
                        _line(source, name, source.index(head)), "3.6 reserved keyword and 4.1 transparent type table",
                        repository="KhronosGroup/OpenGL-Registry") for name in sorted(names)}


def amd_vertex_sampler_types(source, path, source_hash):
    head = "Add the keywords"
    tail = "Additions to Chapter 4"
    if head not in source or tail not in source:
        raise ValueError("AMD vertex sampler keyword block missing")
    keywords = source.split(head, 1)[1].split(tail, 1)[0]
    names = set(re.findall(r"\b__(?:i|u)?samplerVertexAMD\b", keywords))
    if names != {"__samplerVertexAMD", "__isamplerVertexAMD", "__usamplerVertexAMD"}:
        raise ValueError("AMD vertex sampler keyword family incomplete")
    for name in names:
        if not re.search(r"\bvertexFetchAMD\(\s*" + re.escape(name) + r"\s+sampler", source):
            raise ValueError("AMD vertex sampler function type use missing: " + name)
    return {name: _fact("GL_AMD_vertex_shader_tessellator", name, path, source_hash,
                        _line(source, name, source.index(head)), "3.6 keyword and typed vertexFetchAMD parameter",
                        category="opaque_type", repository="KhronosGroup/OpenGL-Registry") for name in sorted(names)}


def ext_gpu_shader4_types(source, path, source_hash):
    head = 'Add to section 3.6 "Keywords"'
    tail = 'Add to section 4.1 "Basic Types"'
    if head not in source or tail not in source:
        raise ValueError("EXT gpu_shader4 keyword/type amendments missing")
    keywords = source.split(head, 1)[1].split(tail, 1)[0]
    names = set(re.findall(r"\b(?:uvec[234]|[iu]?sampler[A-Za-z0-9]+)\b", keywords))
    if len(names) != 25:
        raise ValueError("EXT gpu_shader4 type keyword family incomplete")
    table = source.split(tail, 1)[1].split('Change section 4.1.3 "Integers"', 1)[0]
    table_names = {name for name in names if re.search(r"^\s*" + re.escape(name) + r"\s+", table, re.M)}
    if names - table_names != {"samplerCubeShadow"}:
        raise ValueError("EXT gpu_shader4 keyword/type table discrepancy: " + repr(sorted(names - table_names)))
    return {name: _fact("GL_EXT_gpu_shader4", name, path, source_hash,
                        _line(source, name, source.index(head)),
                        "3.6 reserved type keyword; absent from added 4.1 table" if name == "samplerCubeShadow"
                        else "3.6 keyword and 4.1 basic type table",
                        category="reserved_type_keyword" if name == "samplerCubeShadow" else "type",
                        repository="KhronosGroup/OpenGL-Registry") for name in sorted(names)}


REGISTRY_EXACT_ROWS = {
    "EXT/EXT_shadow_samplers.txt": ("Append the following row to the table in section 4.1, Basic Types:",
                                    {"sampler2DShadow"}, "opaque_type"),
    "NV/NV_non_square_matrices.txt": ("Add the following types to the Basic Types table in section 4.1:",
                                      {f"mat{c}x{r}" for c, r in ((2, 3), (3, 2), (2, 4), (4, 2), (3, 4), (4, 3))},
                                      "type"),
    "NV/NV_texture_array.txt": ("Add the following to the table of basic types in section 4.1:",
                                {"sampler2DArrayNV"}, "opaque_type"),
    "OES/OES_texture_3D.txt": ("Add the following to the table of basic types in section 4.1:",
                               {"sampler3D"}, "opaque_type"),
    "ARB/ARB_texture_rectangle.txt": ("Add the following (previously reserved) keywords",
                                      {"sampler2DRect", "sampler2DRectShadow"}, "reserved_type_keyword"),
}


def registry_exact_rows(path, source, source_hash):
    heading, expected, category = REGISTRY_EXACT_ROWS[path]
    if heading not in source:
        raise ValueError("Registry exact type section missing: " + path)
    section = source.split(heading, 1)[1]
    if path == "ARB/ARB_texture_rectangle.txt":
        section = section.split('Add to section 8.7 "Texture Lookup Functions"', 1)[0]
    else:
        section = section.split("Additions to Chapter 8", 1)[0]
    names = {name for name in expected if re.search(r"^\s*" + re.escape(name) + r"\b", section, re.M)}
    if names != expected:
        raise ValueError("Registry exact type rows incomplete: " + path)
    extension = "GL_" + path.split("/", 1)[1].removesuffix(".txt")
    return {name: _fact(extension, name, path, source_hash,
                        _line(source, name, source.index(heading)),
                        "4.1 basic type row" if category != "reserved_type_keyword" else
                        "3.6 formerly reserved type keyword and typed lookup signature",
                        category=category, repository="KhronosGroup/OpenGL-Registry") for name in sorted(names)}


def registry_types(path, source, source_hash):
    repo = "KhronosGroup/OpenGL-Registry"
    if path in REGISTRY_EXACT_ROWS:
        return registry_exact_rows(path, source, source_hash)
    if path == "AMD/AMD_vertex_shader_tessellator.txt":
        return amd_vertex_sampler_types(source, path, source_hash)
    if path == "EXT/EXT_gpu_shader4.txt":
        return ext_gpu_shader4_types(source, path, source_hash)
    if path == "NV/NV_shader_buffer_load.txt":
        return registry_pointer_types(source, path, source_hash)
    if path in REGISTRY_BASIC_FAMILIES:
        return registry_basic_family(path, source, source_hash)
    if path in {"ARB/ARB_shader_atomic_counters.txt", "NV/NV_shadow_samplers_array.txt",
                "NV/NV_shadow_samplers_cube.txt"}:
        return registry_single_basic_type(path, source, source_hash)
    if path.endswith("shader_image_load_store.txt"):
        return registry_image_types(path, source, source_hash)
    if path.endswith(("texture_cube_map_array.txt", "texture_buffer.txt")):
        return registry_essl_opaque_types(path, source, source_hash)
    if path == "NV/NV_gpu_shader5.txt":
        return nv_gpu_shader5_types(source, path, source_hash)
    if path.startswith(("AMD/AMD_gpu_shader_", "ARB/ARB_gpu_shader_")) and path.endswith(
            ("half_float.txt", "int16.txt", "int64.txt", "fp64.txt")):
        return registry_transparent_types(path, source, source_hash)
    if path in {"OES/OES_EGL_image_external.txt", "NV/NV_EGL_stream_consumer_external.txt"}:
        name = "samplerExternalOES"
        keyword = source.index('Add to the list of keywords:')
        basic = source.index('Add to the list of basic types:')
        if not (keyword < source.index('"' + name + '"') < basic <
                source.index('"' + name + '   a handle')):
            raise ValueError("OES external sampler keyword/type disagreement")
        extension = "GL_" + path.split("/", 1)[1].removesuffix(".txt")
        return {name: _fact(extension, name, path, source_hash,
                            _line(source, '"' + name + '"', keyword),
                            "ESSL 3.7 keyword and 4.1 basic type", category="opaque_type", repository=repo)}
    if path == "OES/OES_EGL_image_external_essl3.txt":
        name = "samplerExternalOES"
        keyword = source.index("Add to the list of keywords:")
        basic = source.index("Add to the list of Floating Point Sampler Types")
        if not (keyword < source.index('"' + name + '"') < basic <
                source.index('"' + name + '   a handle')):
            raise ValueError("OES ESSL3 external sampler keyword/type disagreement")
        return {name: _fact("GL_OES_EGL_image_external_essl3", name, path, source_hash,
                            _line(source, '"' + name + '"', keyword),
                            "ESSL 3.7 keyword and 4.1 opaque sampler type",
                            category="opaque_type", repository=repo)}
    if path == "EXT/EXT_YUV_target.txt":
        name = "__samplerExternal2DY2YEXT"
        heading = "New Types & Keywords"
        if not (heading in source and "one new sampler type" in source and
                name in source.split(heading, 1)[1].split("Here is one example:", 1)[0]):
            raise ValueError("EXT YUV sampler type evidence missing")
        return {name: _fact("GL_EXT_YUV_target", name, path, source_hash,
                            _line(source, name, source.index(heading)), "new sampler type declaration",
                            category="opaque_type", repository=repo)}
    if path == "AMD/AMD_gpu_shader_half_float_fetch.txt":
        head = "(add the following to the list of reserved keywords at p. 18)"
        table_head = "(insert new types, interleaved with the existing types"
        if head not in source or table_head not in source:
            raise ValueError("AMD half-float opaque type sections missing")
        keywords = source.split(head, 1)[1].split("Additions to Chapter 4", 1)[0]
        names = set(re.findall(r"\bf16(?:sampler|image)[A-Za-z0-9]+\b", keywords))
        table = source.split(table_head, 1)[1].split("Additions to Chapter 5", 1)[0]
        table_names = set(re.findall(r"^\s*\|\s*(f16(?:sampler|image)[A-Za-z0-9]+)\s*\|", table, re.M))
        if len(names) != 29 or names != table_names:
            raise ValueError("AMD half-float keyword/type table discrepancy: " +
                             repr((sorted(names - table_names), sorted(table_names - names))))
        return {name: _fact("GL_AMD_gpu_shader_half_float_fetch", name, path, source_hash,
                            _line(source, name, source.index(head)), "3.6 reserved keyword and 4.1 opaque type table",
                            category="opaque_type", repository=repo) for name in sorted(names)}
    raise ValueError("unrouted Registry type source: " + path)


def _archive_sources(path, expected, required):
    if sha256(path) != expected:
        raise ValueError("pinned extension archive digest mismatch")
    contents = {}
    with tarfile.open(path, "r:gz") as archive:
        for member in archive:
            if not member.isfile() or "/extensions/" not in member.name:
                continue
            relative = member.name.split("/extensions/", 1)[1]
            if relative.endswith(".txt"):
                if relative in contents:
                    raise ValueError("duplicate extension source: " + relative)
                contents[relative] = archive.extractfile(member).read()
    if not set(required) <= set(contents):
        raise ValueError("missing pinned extension type sources: " + repr(sorted(set(required) - set(contents))))
    return contents


def challenge_sources(sources):
    """Independently route amendment language across every pinned GLSL spec."""
    candidate = re.compile(r"\b(?:add|introduce|define|new)\b[^.\n]{0,80}\b"
                           r"(?:type|types|keyword|keywords)\b", re.I)
    routed = set(REQUIRED) | set(NON_TYPE_SOURCES)
    found = set()
    for path, raw in sources.items():
        source = raw.decode("utf-8", errors="strict")
        lower = source.lower()
        if candidate.search(source) or ("keyword" in lower and any(
                term in lower for term in ("basic types", "opaque types", "transparent types", "type-specifier"))):
            found.add(path)
    unresolved = found - routed
    if unresolved:
        raise ValueError("unresolved extension type source candidates: " + repr(sorted(unresolved)))
    dispositions = []
    for path, (needle, reason) in sorted(NON_TYPE_SOURCES.items()):
        raw = sources[path]
        source = raw.decode("utf-8", errors="strict")
        dispositions.append({"source_path": path, "source_sha256": hashlib.sha256(raw).hexdigest(),
                             "line": _line(source, needle), "evidence": needle, "disposition": reason})
    return {"complete": True, "scanned_sources": len(sources), "candidate_sources": len(found),
            "selected_sources": len(REQUIRED), "non_type_dispositions": dispositions,
            "unresolved_candidates": 0, "challenger_gaps": 0,
            "method": "all pinned GLSL extension texts scanned for addition/type/keyword amendments; "
                      "candidate paths must be extracted or source-cited as non-type"}


def challenge_registry_sources(sources, non_type_sources=REGISTRY_NON_TYPE_SOURCES,
                               independent_review=REGISTRY_DISPOSITION_REVIEWED):
    """Fail closed unless every broad Registry hit has a source-bound route."""
    candidate = re.compile(r"\b(?:add|introduce|define|new)\b[^.\n]{0,80}\b"
                           r"(?:type|types|keyword|keywords)\b", re.I)
    found = set()
    for path, raw in sources.items():
        source = raw.decode("utf-8", errors="replace")
        lower = source.lower()
        if (("shading language" in lower or "glsl" in lower) and
                (candidate.search(source) or ("keyword" in lower and "type" in lower))):
            found.add(path)
    routed = set(REGISTRY_TYPE_SOURCES) | set(non_type_sources)
    unresolved = found - routed
    stale = set(non_type_sources) - found
    dispositions = []
    for path, row in sorted(non_type_sources.items()):
        raw = sources.get(path)
        if raw is None:
            raise ValueError("missing Registry disposition source: " + path)
        source_hash, line, reason_index = row
        if hashlib.sha256(raw).hexdigest() != source_hash:
            raise ValueError("Registry disposition source digest mismatch: " + path)
        lines = raw.decode("utf-8", errors="strict").splitlines()
        if line < 1 or line > len(lines) or not lines[line - 1].strip():
            raise ValueError("Registry disposition citation mismatch: " + path)
        evidence_lines = [value.strip() for value in lines[line - 1:] if value.strip()][:2]
        evidence = "\n".join(evidence_lines)
        dispositions.append({"source_path": path, "source_sha256": source_hash,
                             "line": line, "evidence": evidence,
                             "disposition": REGISTRY_NON_TYPE_REASONS[reason_index]})
    review_gap = 0 if independent_review else 1
    return {"complete": not unresolved and not stale and independent_review,
            "scanned_sources": len(sources), "candidate_sources": len(found),
            "selected_sources": len(REGISTRY_TYPE_SOURCES),
            "non_type_dispositions": dispositions,
            "unresolved_candidates": len(unresolved),
            "unresolved_paths": sorted(unresolved),
            "stale_dispositions": sorted(stale),
            "independent_review": independent_review,
            "challenger_gaps": len(unresolved) + len(stale) + review_gap,
            "method": "all pinned Registry extension texts scanned for broad type/keyword amendments; "
                      "every candidate path must have extracted type facts or a source-hash/line-bound disposition"}


def challenge_type_headings(sources, routed, dispositions):
    """Independent heading/keyword matcher guards the broader challenger."""
    found = {path for path, raw in sources.items()
             if TYPE_HEADING.search(raw.decode("utf-8", errors="replace"))}
    unresolved = found - set(routed) - set(dispositions)
    receipts = []
    for path, (source_hash, line, reason) in sorted(dispositions.items()):
        raw = sources[path]
        lines = raw.decode("utf-8", errors="strict").splitlines()
        if hashlib.sha256(raw).hexdigest() != source_hash or line < 1 or line > len(lines):
            raise ValueError("type-heading disposition mismatch: " + path)
        receipts.append({"source_path": path, "source_sha256": source_hash, "line": line,
                         "evidence": lines[line - 1].strip(), "disposition": reason})
    return {"complete": not unresolved, "candidate_sources": len(found),
            "unresolved_candidates": len(unresolved), "unresolved_paths": sorted(unresolved),
            "non_type_dispositions": receipts}


def extract(glsl_archive: Path, registry_archive: Path):
    """Return a mechanically checked type catalog fragment, or fail closed."""
    sources = _archive_sources(glsl_archive, GLSL_SHA256, REQUIRED | set(NON_TYPE_SOURCES))
    glsl_audit = challenge_sources(sources)
    glsl_headings = challenge_type_headings(sources, set(REQUIRED) | set(NON_TYPE_SOURCES),
                                            GLSL_HEADING_NON_TYPES)
    registry = _archive_sources(registry_archive, REGISTRY_SHA256, REGISTRY_TYPE_SOURCES)
    registry_audit = challenge_registry_sources(registry)
    registry_headings = challenge_type_headings(
        registry, set(REGISTRY_TYPE_SOURCES) | set(REGISTRY_NON_TYPE_SOURCES),
        REGISTRY_HEADING_NON_TYPES)
    entries = []
    for path in sorted(REQUIRED):
        raw = sources[path]
        source = raw.decode("utf-8", errors="strict")
        source_hash = hashlib.sha256(raw).hexdigest()
        if path == ARITHMETIC:
            entries.extend(arithmetic_types(source, path, source_hash).values())
        elif path == IMAGE64:
            entries.extend(image64_types(source, path, source_hash).values())
        elif path == STORAGE:
            entries.extend(storage_types(source, path, source_hash).values())
        elif path == VULKAN:
            entries.extend(vulkan_types(source, path, source_hash).values())
        elif path == TENSOR2:
            entries.extend(tensor2_types(source, path, source_hash).values())
        elif path in SIMPLE:
            entries.extend(simple_types(source, SIMPLE[path], path, source_hash).values())
        else:
            for extension, names in NAMED[path].items():
                for name in names:
                    # EXT ray tracing explicitly specifies the acceleration-
                    # structure constructor; the opaque query/tensor keywords do not.
                    proven = (path == "ext/GLSL_EXT_ray_tracing.txt" and name == "accelerationStructureEXT"
                              and "accelerationStructureEXT constructor" in source)
                    entries.extend(named_type(source, name, extension, path, source_hash, proven).values())
    for path in sorted(REGISTRY_TYPE_SOURCES):
        raw = registry[path]
        entries.extend(registry_types(path, raw.decode("utf-8", errors="strict"),
                                      hashlib.sha256(raw).hexdigest()).values())
    by_extension = defaultdict(set)
    constructors_by_extension = defaultdict(set)
    provenance = defaultdict(list)
    for entry in entries:
        name, extension = entry["name"], entry["extension"]
        by_extension[extension].add(name)
        provenance[name].append(entry)
        if entry["constructor"] == "source_proven":
            constructors_by_extension[extension].add(name)
    return {"types": sorted(provenance),
            "types_by_extension": {key: sorted(value) for key, value in sorted(by_extension.items())},
            "constructors": sorted(set().union(*constructors_by_extension.values())),
            "constructors_by_extension": {key: sorted(value) for key, value in sorted(constructors_by_extension.items())},
            "type_provenance": {key: sorted(value, key=lambda row: (row["source_path"], row["line"]))
                                for key, value in sorted(provenance.items())},
            "type_sources": [{"repository": "KhronosGroup/GLSL", "revision": GLSL_REVISION,
                              "archive_sha256": GLSL_SHA256, "source_count": len(sources)},
                             {"repository": "KhronosGroup/OpenGL-Registry", "revision": REGISTRY_REVISION,
                              "archive_sha256": REGISTRY_SHA256, "source_count": len(registry)}],
            "verified_archives": {"KhronosGroup/GLSL": GLSL_SHA256,
                                  "KhronosGroup/OpenGL-Registry": REGISTRY_SHA256},
            "type_audit": {"complete": (glsl_audit["complete"] and registry_audit["complete"] and
                                           glsl_headings["complete"] and registry_headings["complete"]),
                           "glsl": glsl_audit, "registry": registry_audit,
                           "heading_challenge": {"glsl": glsl_headings, "registry": registry_headings},
                           "unresolved_candidates": (registry_audit["unresolved_candidates"] +
                                                     glsl_headings["unresolved_candidates"] +
                                                     registry_headings["unresolved_candidates"]),
                           "challenger_gaps": (registry_audit["challenger_gaps"] +
                                               glsl_headings["unresolved_candidates"] +
                                               registry_headings["unresolved_candidates"])}}
