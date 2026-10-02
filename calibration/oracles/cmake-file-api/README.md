# cmake-file-api — tier-A oracle for CMake (LB-Q-cmake)

CMake's own evaluation, read two ways, written as `indexbench lang-oracle`'s neutral oracle file. Python 3 standard
library only, bench-only, never a product dependency. CMake 4.4.3 (Homebrew).

```sh
python3 calibration/oracles/cmake-file-api/cmake_oracle.py -subject <checkout> -build <scratch build dir> \
    [-D VAR=VALUE]... > workspace/tmp/oracle/cmake-<subject>.json
indexbench lang-oracle -dialect cmake -subject <checkout> -subject-name <owner/repo> \
    -tool-oracle workspace/tmp/oracle/cmake-<subject>.json -out docs/bench/readings/lang/cmake.json
```

`-build` is a scratch directory outside the checkout (the script refuses one inside). It writes
`.cmake/api/v1/query/codemodel-v2` into it, then runs `cmake -S <subject> -B <build> --trace-expand
--trace-format=json-v1 --trace-redirect=<build>/trace.json`.

| Field | Source | What it is |
|---|---|---|
| `references` | the command trace | `(file, target, argument)` for every literal argument of an executed `target_link_libraries` or `add_dependencies` whose target the same file defines. The trace says the command ran (a command in a dead `if()` branch is not in it), the source says how each argument was spelled |
| `cross_references` | File API codemodel-v2 | `(file the dependency was added in, target, file the dependency target is defined in, dependency)` for each dependency two targets of different files have, file granularity |

L1 is universal-ctags (tier B); this file carries no definitions.

An argument or dependency written with a variable, a generator expression, a quote or a path is a name no static reader
of the file sees; it is counted on stderr and left out of the truth, as the make oracle leaves out a prerequisite
spelled with `$`. The stderr line is the accounting: for each subject it names the link commands, the arguments not
spelled literally, the targets defined in another file, and per dependency how many were dropped and why.

First runs (2026-10-01): fmtlib/fmt 12.2.0 (6 references, 3 cross_references of 56 dependencies; 37 of them written
through a function's parameter), nlohmann/json v3.12.0 (10 references, 5 of 90), bulletphysics/bullet3 3.25 (46 references,
46 of 130; configured with `-D CMAKE_POLICY_VERSION_MINIMUM=3.5`, which CMake 4 needs for its `cmake_minimum_required`).
SDL 3.4.8, glfw 3.4, google/benchmark and spdlog were tried and read too few literal link commands to measure
(5, 4, 1 and 3 references).
