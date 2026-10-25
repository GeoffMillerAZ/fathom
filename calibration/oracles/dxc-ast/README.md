# dxc-ast — tier-B HLSL definitions and calls (LB-Q-hlsl)

A small Go program (its own module, `fathom.bench/oracles/dxc-ast`, never imported by fathom) that writes `indexbench
lang-oracle -tool-oracle`'s neutral `definitions` and `calls` for an HLSL subject, scored as L1 and L2 (design §1.2,
§10). The tool is the compiler's own front end, DXC's text AST:

    dxc -T lib_6_3 -HV 2021 -ast-dump [-I <dir>]... ./<file>

from the subject root, one process per measured file, with run-chosen flags and no build database (tier B: a shader's
build passes its own profile, entry point and defines; this run does not know them). A unit whose run writes an
`error:` is retried under `-HV 2018` and under the newest library profile DXC 1.9 lists (`-retry-target`, default
`lib_6_9`), each on every such unit, and read under the first that parses it (2018, then the profile), chosen per unit;
the `disclose` lines count what each retry admits and name the units admitted. A unit no setting parses is not read:
its first error is recorded, and the excluded units are disclosed by first-error class (the message with each quoted
name as `'_'`), the most frequent first, with two examples each, and by the bias they give the scope (a missing
include, a name declared elsewhere). A file is in scope when its own unit parses without error, or when it is a header that
`dxc -M` lists for such a unit; `-scope` writes that list for lang-oracle's `-only`, so a file the front end cannot
read is neither a recall miss nor a precision miss. The oracle file's `scope` names the same list, so `lang-oracle`
grades definitions and calls on those files alone while its run (tenancy, throughput, L3) covers every measured file.

The reading is driven by `reading.sh` (from the repository root, after `task calibrate-clone -- hlsl`):

    calibration/oracles/dxc-ast/reading.sh docs/bench/readings/lang/hlsl.json

It builds this program and `../clang-includes`, runs both on each subject's measured files, runs `lang-oracle` three
times over every file (no `-only`) and once with `-only` the scope, keeps the median-throughput run as the reading and
appends notes generated from those runs' tenancy output: files with a parse error per subject and pooled, the three
throughput values, and how the parse errors split between the files DXC reads and those it excludes.

`-files` is the harness's measured set (one subject-relative path per line: every `.hlsl`, `.hlsli`, `.fx` and `.fxh`
discovery lists). The imports oracle is `../clang-includes` run on the same list with `-lang c++ -I .`.

Definitions, each at its name's line with its range's last line: a function with a body (a template's pattern
included), a struct's method with a body (kind `method`, in the struct or defined out of line, with its calls), a struct or class with a body, a cbuffer or tbuffer (kind `cbuffer`, the dialect's
`oracle_kinds` maps it to `struct`), a typedef and a variable at file scope (a cbuffer's constants are the buffer's).
Call pairs: (the innermost enclosing definition, the callee's name) for every `CallExpr` naming a function, intrinsics
included, and every member call (`tex.Sample(...)`, the member's bare name); a vector constructor is a cast, never a
call. DXC places a node at its spelling location, so a definition or call a macro body writes sits on the `#define`
line: those are not written and are counted in the file's `disclose` lines. Declarations are read from every unit that
includes their file and deduplicated. Facts of the dump this reads: a location's file and line are written only where
they change (the parser keeps that state in print order, as `../clang-ast-defs` does for clang's JSON); `prev 0x..`
(a redeclaration) and `parent cbuffer 0x..` precede a node's range. `go test .` checks the parser on a hand-written
dump.

DXC is the release asset of microsoft/DirectXShaderCompiler, unpacked under `workspace/tools/dxc/` and never
committed. Measured 2026-10-25 with v1.9.2602 (`linux_dxc_2026_02_20.x86_64.tar.gz`, sha256
a1d3e3b5e1c5685b3eb27d5e8890e41d87df45def05112a2d6f1a63a931f7d60; `dxc --version`
`libdxcompiler.so: 1.9(5184-21d28f72)(1.9.0.5184)`). Bench-only; never a product dependency.
