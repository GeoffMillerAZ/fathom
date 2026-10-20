# clang-objc — the compiler's front end as the Objective-C oracle (tier B, LB-Q-objective-c)

scip-clang 0.4.0 (../scip-clang) cannot index Objective-C: its driver drops `.m` and `.mm` entries of a compilation
database (`compilation database has no entries that could be processed`), and a `.c` copy of a `.m` indexes the C
declarations only, no interface, method or message send (O1413; the host's toolchain is not the limit: the Command
Line Tools SDK parses Foundation and UIKit). universal-ctags 6.2.1 (`ObjectiveC` parser) is the L1 oracle (tier B).
This script asks the compiler front end for the rest: libclang, the Command Line Tools' own
`/Library/Developer/CommandLineTools/usr/lib/libclang.dylib` (Apple clang 21.0.0), driven through the `libclang==18.1.1`
Python bindings (their cursor-kind table is older than the library, so an unknown kind is skipped).

    uv pip install --target <tools>/pyclang libclang==18.1.1
    CLANG_MODULE_CACHE=<scratch>/modcache python3 clang_objc.py --subject <scratch copy of the checkout> --out <oracle.json> \
        --all-header-dirs --compdb <subject>/build/compile_commands.json --list <files.txt>

It parses every `.m`, `.mm` and Objective-C `.h` (a header as its own `objective-c-header` unit) with `-fobjc-arc`,
the macOS SDK first and Mac Catalyst (`-target arm64-apple-ios14.0-macabi` and the SDK's iOSSupport frameworks) when the
first fails, and `-fmodules` only for a file that writes `@import`. `--all-header-dirs` adds `-I` for the subject root
and every directory holding a header (an Xcode project's recursive user header search paths): the subjects ship no
compile database, so the include roots are this run's, which is why the tier is B and not A (as `../clang-includes`).
`--compdb` writes those roots as a `compile_commands.json` at `<subject>/build/`, where fathom's include-path
resolver reads them, so the oracle and the resolver search the same directories. Work in a scratch copy of the pinned
checkout (`git clone --local`), never the shared one.

A unit that stopped on a compile error (a missing `XCTest/XCTest.h` or `WatchKit/WatchKit.h`, which the Command Line
Tools lack, a module that cannot be built) is not trusted: its file is left out of every output and listed in
`<oracle>.errored.json`; `--list` writes the files that parsed clean, which `indexbench lang-oracle -only` takes so the
reading measures exactly the files the oracle answers for.

Outputs (the neutral shape, `../README.md`):

- `calls`: `{path, from, to}`, a message send (`to` the selector, `initWithFrame:style:`) or a C call written in a
  method or function of a subject file, paired with the innermost definition around it (a block's calls belong to the
  method that holds it). A call whose name token is not written there (a macro's expansion) is not a call of the truth.
  A property's dot syntax is no message send. `from` is a selector or a function name.
- `call_exclusions`: a selector every send of which, in one file, has a receiver of the bare type `id`: the compiler
  binds no class to it, so the pair is left out of both sides and counted (`call_pairs_excluded_*`).
- `imports`: `{path, line, to_path, module}`, each `#import`, `#include` and `@import` the preprocessor entered that
  resolves to a file of the subject (a conditional one it skipped is none), scored as `import_resolution`; and, with
  `same_package: true` (the harness's field for a pair no import statement writes), the file the compiler binds a
  reference to through an import chain (a class, protocol, typedef, function, constant, field, method or macro named
  in the importer and declared in a subject file it does not import directly): file-edge truth only, the compiler's
  cross-file reference graph, as a SCIP index's cross-file references are for C.
- `<oracle>.ranges.json`: the first and last line of every definition the compiler names, read by `range_agreement`.

Bench-only; never a product dependency.
