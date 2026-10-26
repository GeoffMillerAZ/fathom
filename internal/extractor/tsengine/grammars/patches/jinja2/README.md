# Jinja optional-else source patch

Source: bennypowers/tree-sitter-jinja-dialects at
4f832fe6feeae8c9e5963d835bec0272a8331b47, MIT.

The patch admits Jinja conditionals without an else clause and preserves the
existing for-loop filter AST by giving for_condition higher precedence.
No template text is blanked or rewritten. The upstream source stays pinned;
P-R1 verifies the archive and patch digest before parser generation.

Native proof: 108/108 upstream corpus cases (105 existing plus three prepared
additions), eight independent Jinja 3.1.6 syntax controls, and 27/520 -> 3/520
ERROR-or-MISSING files, with zero newly failing files. Final WASM/accuracy proof
is in the Jinja reading and assessment after the production recipe runs.

The upstream PR is prepared only under workspace/upstream/jinja-optional-else/.
No PR or fork has been published.
