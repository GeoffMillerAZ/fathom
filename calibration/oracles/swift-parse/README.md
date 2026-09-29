# swift_parse.py — tier-B Swift L1/L2 oracle from the compiler's own parser

ADR-048 amendment D2: universal-ctags 6.2.1 has no Swift parser, and SourceKit-LSP's index store (the compiler's typed index,
`swift build -Xswiftc -index-store-path`) is not SCIP: reading it needs a build of each subject (swift-nio's dependencies are
fetched from the network) and a reader of `libIndexStore`, which is a foundation task of its own. What every Swift toolchain
does ship is the compiler's parser: `swiftc -frontend -parse-as-library -dump-parse <file>` prints the syntax tree the compiler
parses (no name lookup, no type checking, no dependencies) with a source range for every declaration.

`swift_parse.py` reads that dump and writes `indexbench lang-oracle -tool-oracle`'s neutral JSON: the definitions the swift
dialect declares (the name's row and the declaration's last row; a property's range is its `let`/`var` binding's) and the
(enclosing definition, callee) call pairs. Bench-only, Python standard library only, never a product dependency. The tier is B
on the D2 precedent (JavaParser for Java, Roslyn for C#, the Kotlin PSI dump): an independent syntactic parser, so L1 can be
excellent and L2 (calls) at most good.

    python3 swift_parse.py --root <clone> [--swiftc swiftc] [--jobs 4] > out/swift-<subject>.json
    go run ./cmd/indexbench lang-oracle -dialect swift -subject <clone> -subject-name Alamofire/Alamofire \
        -tool-oracle out/swift-<subject>.json -out docs/bench/readings/lang/swift.json

Apple Swift 6.4 (swiftlang-6.4.0.34.1, the Command Line Tools toolchain), macOS arm64. The dump goes to stdout and the
diagnostics to stderr; a file with a syntax error still dumps what parsed (`with_syntax_errors` on stderr counts them: Alamofire
3 of 92, swift-nio 41 of 550 — the compiler's parser also rejects Swift's newest syntax in a few files, and `#if`
branches it keeps duplicated).

What the script does that the dump does not:

- **`#if` blocks.** `-dump-parse` prints nothing of an `#if` block (the compiler picks a branch by the build configuration and
  the dump omits it), so the directive lines (`#if`, `#elseif`, `#else`, `#endif`) are blanked first, every line number kept, and
  every branch's declarations are parsed, as the tree-sitter grammar reads them.
- **Structure from indentation, not from parentheses.** The dump is one node per line, two spaces per level, but the compiler leaves a
  parenthesis open after a function type (`typealias F = (Int) -> Void`), which would nest every later declaration of the file inside that
  alias; a node's parent is the last node one level up.
- **Definitions**: `class_decl` (an `actor` flag makes it an actor), `struct_decl`, `enum_decl`, `protocol`, `extension_decl` (named by the
  extended type: `Outer.Inner` is `Inner`, and an array or optional sugar type keeps its spelling), `typealias`, `associated_type_decl`,
  `enum_element_decl` (once, not the copy inside `enum_case_decl`), `func_decl` (a method inside a type body, a function elsewhere; `f(_:x:)`
  is `f`, an operator function keeps its operator; the compiler's own `$defer` and any `implicit` function are left out), `constructor_decl`
  (`init`), `destructor_decl` (`deinit`), `subscript_decl` (`subscript`), `macro_decl`, and `var_decl` only as a member of a type body or at
  file level (`property`, `variable`), never a local, with the range of its `pattern_binding_decl`. An accessor, a parameter, an operator
  declaration and a precedence group are not definitions.
- **Calls**: every `call_expr` whose callee is a plain name (`unresolved_decl_ref_expr`), a member (`unresolved_dot_expr`'s `field`) or an
  implicit member (`unresolved_member_expr`), through optional chaining, `!`, parentheses and generic specialisation; a call the compiler
  writes (`implicit`: string interpolation's `appendInterpolation`) and a `case` pattern (`pattern_expr`: `case .a(let x):` is a call
  expression to the parser and not a call) are left out. A call belongs to its innermost enclosing function, initialiser, deinitialiser,
  subscript, or property (an initialiser or an accessor); a call in a local variable's initialiser belongs to the enclosing definition, and one
  at file level outside a variable (a `main.swift` statement) to none, which is also how the dialect attaches calls.

Subjects (calibration/corpus.d/swift.yaml): Alamofire/Alamofire 5.9.1 (92 files, 3,716 definitions, 7,993 call pairs) and apple/swift-nio 2.99.0
(550 files, 18,281 definitions, 38,823 call pairs), the `dev/`, `Tests/`, `IntegrationTests/` and `Snippets/` trees included, as cloned.
