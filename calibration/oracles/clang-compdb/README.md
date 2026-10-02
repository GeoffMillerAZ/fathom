# clang-compdb — tier-A oracle for compile_commands.json (LB-Q-compile-commands)

libclang's own `CompilationDatabase` reads a subject's database; the script writes `indexbench lang-oracle`'s neutral
oracle file. Bench-only, never a product dependency.

```sh
python3 -m venv /Volumes/m2ext/tmp/lang-oracles/clang-compdb-venv
/Volumes/m2ext/tmp/lang-oracles/clang-compdb-venv/bin/pip install libclang        # the pip wheel bundles libclang.dylib
calibration/oracles/clang-compdb/gen_compdb.sh <checkout> <scratch build dir>     # writes <checkout>/compile_commands.json
/Volumes/m2ext/tmp/lang-oracles/clang-compdb-venv/bin/python calibration/oracles/clang-compdb/compdb_oracle.py \
    -subject <checkout> > workspace/tmp/oracle/cc-<subject>.json
indexbench lang-oracle -dialect compile-commands -subject <checkout> -subject-name <owner/repo> \
    -tool-oracle workspace/tmp/oracle/cc-<subject>.json -out docs/bench/readings/lang/compile-commands.json
```

`gen_compdb.sh` configures a CMake subject with `CMAKE_EXPORT_COMPILE_COMMANDS=ON` and copies the database it writes to
the checkout's root: that is the only place ingest reads a database from (`build/` is excluded by `.fathomignore`'s
CT-8 default, so one left in a build tree is never indexed; `includeRoots` reads `compile_commands.json` and
`build/compile_commands.json`). The copy is generated, never committed with a subject, and a reading discloses it.

| Field | Source | What it is |
|---|---|---|
| `definitions` | libclang's commands (one `step` per entry), the entry's `file` as written | at the lines the entry's object spans in the JSON text (clang reports no positions; the k-th object of the text is the k-th command clang returns, checked by file) |
| `references` | the arguments libclang returns (it tokenizes `command` with LLVM's shell-quoting rules) | `(file, flag)` for each include directory (`-I`, `-iquote`, `-isystem`, `-idirafter`), macro (`-D`) and the `-o` output, each written the flag and its argument joined; clang's driver reads the first four and `-D` joined or separate |

The dialect reads the flags as `fusion.includeRoots` does (whitespace-split `command`, `-I` joined or separate, the other
three separate only), so a joined `-isystem/dir` or a quoted argument holding a space is a pair the oracle has and the
dialect misses; the reading counts them (none occur in the two subjects).

First runs (2026-10-01): fmtlib/fmt 12.2.0 (51 entries, 178 pairs) and libuv/libuv v1.53.0 (477 entries, 2,008 pairs),
CMake 4.4.3 Unix Makefiles databases, libclang pip 18.1.1.
