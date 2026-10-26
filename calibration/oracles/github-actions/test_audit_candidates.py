import unittest
from audit_candidates import rejects_local_dot


class GuardedReturnTest(unittest.TestCase):
    def test_dot_guard_returns_undefined(self):
        source = 'export function parseActionReference(uses) { if (!uses || uses.startsWith("./")) { return undefined; } return resolve(uses); }'
        self.assertTrue(rejects_local_dot(source))

    def test_unrelated_undefined_return_cannot_mask_native_resolution(self):
        source = 'export function parseActionReference(uses) { if (uses.startsWith("./")) { return resolve(uses); } if (!uses) { return undefined; } }'
        self.assertFalse(rejects_local_dot(source))

    def test_other_function_is_not_the_parser_contract(self):
        source = 'export function parseActionReference(uses) { return resolve(uses); }\nexport function helper(uses) { if (uses.startsWith("./")) { return undefined; } }'
        self.assertFalse(rejects_local_dot(source))
