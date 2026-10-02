# clang-calls — the build's own compiler as the C and C++ call-site oracle (tier A, LB-F116)

scip-clang (../scip-clang) indexes each template's pattern, and its 0.4.0 index leaves facts about calls that no
syntactic extractor and no index reference settles. This command asks the compiler that builds the subject: it runs
every compilation database entry with its own flags as

    <entry's compiler> <entry's flags> -fsyntax-only -w -Xclang -ast-dump=json

streams each JSON AST (a translation unit's dump runs to gigabytes and is never held in memory), and writes a neutral
oracle file of every call-shaped site written in a file of the subject: the callee's name token (1-based line and byte
column) and the class the AST gives it.

| class | AST | what lang-oracle does with it (internal/langbench SCIPOptions.ClangSites) |
|---|---|---|
| dependent | CXXDependentScopeMemberExpr, UnresolvedMemberExpr, UnresolvedLookupExpr, DependentScopeDeclRefExpr, CXXUnresolvedConstructExpr in a template's pattern | no callee exists before instantiation: a fathom pair whose every written site is dependent is left out of both L2 counts and counted |
| resolved | a DeclRefExpr to a function, a MemberExpr to a method | a call of the truth, with or without an index reference |
| construct | CXXTemporaryObjectExpr, CXXConstructExpr, a functional cast through a constructor, written `T(` or `T{` | a call of the truth, scored against the index's class or constructor reference there |
| object | a call through a variable, parameter or field (a lambda, a function pointer, a callable object) | a call of the truth |
| cast | a functional cast with no constructor (`size_t(n)`) | no call |
| declaration | a function's name where it is declared (`int close(int fd);`) | an index reference there is no call |

A site in a template pattern is dumped again inside each instantiation, resolved there; the pattern's class wins
(dependent first), because the pattern is what the index and a lexical extractor read. The dump holds no expression
inside a type (an array bound, a `decltype` in a declarator): a call written there is in no class, never "no call".

## Run

    cd calibration/oracles/clang-calls && go run . -subject <checkout> -compdb <build>/compile_commands.json -jobs 5 > clang-calls.json

then pass the file to `indexbench lang-oracle` as a `-tool-oracle` beside the scip-clang index and the preproc
oracle (whose `active_lines` also leave out a call written only on a line the build discards). A unit the compiler
does not finish is run again (an unreadable file can be transient) and, failing that, the command fails: a unit that
stopped parses what remains with errors, and its sites are never committed. Bench-only; its own module, never
imported by fathom.

Readings of 2026-10-01 (Apple clang 21.0.0, macOS arm64, 5 jobs): fmtlib/fmt 12.2.0, 36 units, 16,008 sites,
1.5 min; nlohmann/json v3.12.0, 93 units, 24,579 sites, 5.6 min; gabime/spdlog v1.17.0, 34 units (the library, its
example and its tests, not the fetched Catch2), 7,283 sites, 1 min.
