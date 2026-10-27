# GLSL source patches

The lock pins tree-sitter-glsl v0.2.0 at `e47b8b62b59d0e3529f1c31b03e025d6bd475044` (MIT), then applies:

1. `0001-glsl-syntax.patch`: authored grammar fixes for layout-only declarations, precision statements and type qualifiers, `restrict`, array types and constructors. Precision statements accept macro identifiers as well as `highp`, `mediump` and `lowp`. Constructor precedence 18 exceeds inherited subscript precedence 17, so sized and unsized constructors have an `array_type_specifier` callee while ordinary and nested indexing retain `subscript_expression`. The patch includes three authored tree-sitter corpus sections for these regressions. It adds six conflicts relative to upstream, four for arrays and two for qualifiers.
2. `0002-pin-inherited-c-grammar.patch`: adds the inherited tree-sitter-c `grammar.js` with its full MIT license and copyright notice in a leading comment at `2a265d69a4caf57108a73ad2ed1e6922dd2f998c` under the module path the GLSL grammar already requires. The source archive SHA-256 is `b79cec7ba0b0133f7d1f406340b294466af514c3e1a042555ed0cc8d5cec5d29`. The original grammar body SHA-256 is `f362b9fe06db321464fc77f1e5ba2ce089baec9d18f4defddb0c9ca87885e546`. Only the two non-legal author contact addresses are omitted from the grammar comment; author names and the full licence notice remain. No licence file is edited or added. This is a build input, not a second runtime grammar.

The second patch makes generation independent of an unpinned npm dependency install. The existing pinned tree-sitter CLI generates the parser in scratch; no generated parser is edited by hand. Every patch digest is in the lock and generated provenance. The raw WASM checksum is the final build identity.

`regressions.py` preserves the original 11 positive and one negative grammar cases, with macro precision, seven constructor shapes and exact indexing counts added authored in `workspace/upstream/glsl/regressions.py` while developing `0001-glsl-syntax.patch`. After rebuilding and installing the patched `tree-sitter-glsl` Python binding into an environment with `tree-sitter==0.25.2`, run from the repository root:

```sh
python3 internal/extractor/tsengine/grammars/patches/glsl/regressions.py
```

Set `GLSL_NATIVE_LIBRARY` to the absolute path of an independently compiled grammar library to test those exact bytes without replacing the installed binding. The library stays loaded for the parser lifetime.

On the originating workstation, `workspace/tools/python/bin/python` is the prepared Python executable. The test checks parse errors and verifies that both indexing cases use `subscript_expression`; it is a grammar regression screen, not an oracle reading or compiler check.

`stress.py` runs four authored malformed/deep/large inputs in separate CLI processes. Build the current checkout's CLI, then run the committed screen:

```sh
go build -o workspace/tools/indexbench-glsl-source ./cmd/indexbench
python3 internal/extractor/tsengine/grammars/patches/glsl/stress.py workspace/tools/indexbench-glsl-source
```

A contained WASM trap is recorded separately from a clean parse or budget refusal. The 15-second child-process timeout is a screen bound, not a claim about a particular engine cap or peak memory.
