"""Original GLSL grammar regressions. Run against locally built tree-sitter-glsl.

Requires tree-sitter==0.25.2. This screens a grammar patch, not a support reading.
"""
import ctypes
import os
import unittest
from tree_sitter import Language, Parser
import tree_sitter_glsl

CASES = {
    'layout_in': 'layout(local_size_x = 8) in;',
    'layout_out': 'layout(triangle_strip, max_vertices = 3) out;',
    'precision_parameter': 'float f(mediump vec2 x) { return x.x; }',
    'precision_return': 'mediump float f(mediump vec2 x) { return x.x; }',
    'precision_statement': 'precision highp float;',
    'precision_macro_statement': 'precision COMPAT_PRECISION float;',
    'array_return': 'vec4[2] f() { return vec4[2](vec4(1), vec4(2)); }',
    'unsized_array_type': 'void main() { vec4[] x = vec4[](vec4(1)); }',
    'scalar_constructor': 'void main() { float x = float(1) * 2.0; }',
    'restrict_block': 'layout(binding = 0) coherent restrict buffer Data { vec4 x; };',
    'indexing': 'void main() { vec4 a[2]; vec4 b = a[1]; a[0] = b; }',
    'nested_indexing': 'void main() { float a[2][2]; float x = a[0][1]; }',
}

ARRAY_CONSTRUCTORS = {
    'primitive_sized': 'void main() { float[3] x = float[3](1.0, 2.0, 3.0); }',
    'primitive_unsized': 'void main() { vec4[] x = vec4[](vec4(1)); }',
    'primitive_vector_sized': 'void main() { vec4[2] x = vec4[2](vec4(1), vec4(2)); }',
    'primitive_vector_sized_alt': 'void main() { vec3[3] x = vec3[3](vec3(1), vec3(2), vec3(3)); }',
    'matrix_sized': 'void main() { mat4[2] x = mat4[2](mat4(1), mat4(2)); }',
    'user_type_unsized': 'void main() { Sample[] x = Sample[](Sample(), Sample()); }',
    'user_type_sized': 'void main() { Sample[2] x = Sample[2](Sample(), Sample()); }',
    'multidimensional': 'void main() { vec4[1][1] x = vec4[1][1](); }',
}

# These C-style declarators already parse independently of the constructor at
# upstream HEAD. They isolate the constructor callee tree from the separate
# type-first T[N] x declarator correction exercised above.
C_STYLE_ARRAY_CONSTRUCTORS = {
    'keyword_sized': 'void main() { float x[3] = float[3](1.0, 2.0, 3.0); }',
    'int_corpus': 'void main() { int x[2] = int[2](1, 2); }',
    'vector_sized': 'void main() { vec4 x[2] = vec4[2](vec4(1), vec4(2)); }',
    'vector_sized_alt': 'void main() { vec3 x[3] = vec3[3](vec3(1), vec3(2), vec3(3)); }',
    'matrix_sized': 'void main() { mat4 x[2] = mat4[2](mat4(1), mat4(2)); }',
    'user_type_sized': 'void main() { Sample x[2] = Sample[2](Sample(), Sample()); }',
}

INDEXING_CASES = {
    'single': ('void main() { float a[2]; float x = a[0]; }', 1),
    'single_vector': ('void main() { vec4 b[2]; vec4 x = b[0]; }', 1),
    'nested': ('void main() { float b[2][2]; float x = b[0][1]; }', 2),
}

class GLSLRegressions(unittest.TestCase):
    def setUp(self):
        native = os.environ.get('GLSL_NATIVE_LIBRARY')
        if not native:
            self.parser = Parser(Language(tree_sitter_glsl.language()))
            return
        self.native_library = ctypes.CDLL(native)
        entry = self.native_library.tree_sitter_glsl
        entry.argtypes = []
        entry.restype = ctypes.c_void_p
        pointer = entry()
        if not pointer:
            raise ValueError('tree_sitter_glsl returned NULL')
        capsule = ctypes.pythonapi.PyCapsule_New
        capsule.argtypes = [ctypes.c_void_p, ctypes.c_char_p, ctypes.c_void_p]
        capsule.restype = ctypes.py_object
        self.parser = Parser(Language(capsule(pointer, b'tree_sitter.Language', None)))

    def test_supported_constructs(self):
        for name, source in CASES.items():
            with self.subTest(name=name):
                tree = self.parser.parse(source.encode())
                self.assertFalse(tree.root_node.has_error, str(tree.root_node))
                if name in ('indexing', 'nested_indexing'):
                    self.assertIn('subscript_expression', str(tree.root_node))

    def test_array_constructors_have_one_tree_shape(self):
        for name, source in ARRAY_CONSTRUCTORS.items():
            with self.subTest(name=name):
                tree = self.parser.parse(source.encode())
                root = tree.root_node
                self.assertFalse(root.has_error, str(root))
                constructors = [
                    node for node in self._walk(root)
                    if node.type == 'call_expression'
                    and node.child_by_field_name('function') is not None
                    and node.child_by_field_name('function').type == 'array_type_specifier'
                ]
                self.assertEqual(1, len(constructors), str(root))

    def test_c_style_constructor_callee_shape(self):
        expected = os.environ.get('GLSL_C_STYLE_CALLEE', 'array_type_specifier')
        if expected not in ('array_type_specifier', 'subscript_expression'):
            raise ValueError('invalid expected C-style constructor callee')
        for name, source in C_STYLE_ARRAY_CONSTRUCTORS.items():
            with self.subTest(name=name):
                root = self.parser.parse(source.encode()).root_node
                self.assertFalse(root.has_error, str(root))
                constructors = [
                    node for node in self._walk(root)
                    if node.type == 'call_expression'
                    and node.child_by_field_name('function') is not None
                    and node.child_by_field_name('function').type
                    in ('array_type_specifier', 'subscript_expression')
                ]
                self.assertEqual(1, len(constructors), str(root))
                self.assertEqual(expected, constructors[0].child_by_field_name('function').type)
                declarators = [node for node in self._walk(root) if node.type == 'array_declarator']
                self.assertEqual(1, len(declarators), str(root))

    def test_indexing_stays_subscript_expression(self):
        for name, (source, expected_count) in INDEXING_CASES.items():
            with self.subTest(name=name):
                tree = self.parser.parse(source.encode())
                root = tree.root_node
                self.assertFalse(root.has_error, str(root))
                subscripts = [node for node in self._walk(root) if node.type == 'subscript_expression']
                self.assertEqual(expected_count, len(subscripts), str(root))

    @staticmethod
    def _walk(node):
        yield node
        for child in node.children:
            yield from GLSLRegressions._walk(child)

    def test_incomplete_function_stays_invalid(self):
        tree = self.parser.parse(b'void broken( {')
        self.assertTrue(tree.root_node.has_error)

if __name__ == '__main__':
    unittest.main()
