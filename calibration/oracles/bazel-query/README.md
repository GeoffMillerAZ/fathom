# bazel-query — tier-A oracle for Bazel (LB-Q-bazel)

Bazel's own loading phase, read as `indexbench lang-oracle`'s neutral oracle file. Python 3 standard library only,
bench-only, never a product dependency. Bazel 9.3.0 (bazelisk) and 8.7.0 (grpc's `tools/bazel` wrapper).

```sh
BAZELISK_HOME=<tools>/bazel/home python3 calibration/oracles/bazel-query/bazel_oracle.py \
    -subject <workspace dir> -output-user-root <scratch> > workspace/tmp/oracle/bazel-<subject>.json
python3 calibration/oracles/bazel-query/bazel_oracle.py -subject <workspace dir> -scope-globs    # for -only
indexbench lang-oracle -dialect bazel -subject <workspace dir> -subject-name <owner/repo> [-only <globs>] \
    -tool-oracle workspace/tmp/oracle/bazel-<subject>.json -out docs/bench/readings/lang/bazel.json
```

It runs `bazel query //... --output=streamed_jsonproto --proto:instantiation_stack --proto:definition_stack
--noimplicit_deps --keep_going` in the workspace. The query needs no network, but Bazel fetches the workspace's module
dependencies before it loads a package (O1347), so the first run per workspace does.

| Field | Source | What it is |
|---|---|---|
| `definitions` | a rule's `location` (the BUILD call) and the call's source text | `(BUILD file, x, line of the name literal, line of the closing parenthesis, target)` for each call whose literal `name = "x"` is a rule Bazel created, or the `generator_name` Bazel recorded on the rules a macro created |
| `references` | the same call | `(BUILD file, x, callee)` for the rule or macro the call names, and `(BUILD file, x, y)` for each label spelled `":y"` in the call that is a `rule_input` of a rule it created |
| `cross_references` | `instantiation_stack` and `definition_stack` | `(file, the target or function making the call, the .bzl file defining what it calls)`: consecutive frames of the instantiation stack (the BUILD call, the macro's frames) and the last frame to the stack of the `rule()` call. File granularity (`to` empty). A frame outside the workspace (`@rules_cc`) is dropped and counted |

L1 has no other oracle: ctags has no Bazel parser. A rule's `location` is the BUILD call, so the oracle's end line is
the call's closing parenthesis, found by scanning the text with strings and comments skipped.

What the oracle leaves out, counted on stderr per subject: a call whose `name` is computed or whose macro creates no
rule named it (`name_computed`, e.g. grpc's `grpc_package`), a spelled label that is no input of the rules it created
(`label_not_input`), a frame outside the workspace (`xref_external`). `//...` does not reach a directory named in
`.bazelignore` or one with its own MODULE.bazel, WORKSPACE, WORKSPACE.bazel or REPO.bazel; `-scope-globs` prints the
reachable files for `-only`.

Runs (2026-10-19): bazelbuild/examples `rules` (54 rules, 54 definitions, 76 references, 39 cross references),
`configurations` (50 call sites, 67 references, 25), grpc/grpc v1.84.0 (11,178 rules, 2,466 call sites, 2,450
definitions, 3,297 references, 2,024 cross references).
