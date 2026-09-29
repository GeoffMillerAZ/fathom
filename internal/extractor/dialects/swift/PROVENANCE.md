# swift — provenance

- Grammar: `alex-pinkus/tree-sitter-swift` at commit `35245fbfee2fccf16273c6f4299438fb76875970` (MIT, 2026-09-26, 24 commits past the 0.7.3
  tag and untagged, so the lock pins the commit, the way kotlin's and dart's do), ABI 15, external scanner. The repository ships no
  `src/parser.c` at that commit (only the `0.7.3-with-generated-files` tag does), so the lock row says `"generate": true` and
  `build.sh` runs the pinned `tree-sitter-cli@0.26.11 generate` in scratch. Built into
  `internal/extractor/tsengine/grammars/swift.wasm.gz` (2,435,516 bytes raw, 475,154 gzipped, 597 peak module pages on swift-nio); pin and
  digests in `internal/extractor/tsengine/grammars/grammars.lock.json`.
- Why not the 0.7.3 tag: the calibration subjects are Swift 6-era code, and 0.7.3 leaves 67 of swift-nio's 550 files (12.2%) with an ERROR
  node against 35 (6.4%) on this commit, and it builds `OP_SYMBOL_SUPPRESSOR` with `1UL << FAKE_TRY_BANG` (32), which is a 32-bit shift
  on wasm32 (zig warns `shift-count-overflow`); the commit after 0.7.3 that fixes it (`6ab8d1d`) is in this pin. The commits between the tag
  and the pin also fix `if let x = try await f() { ... }`, `#error` and other compilation directives, bitwise continuations,
  `where` on switch-case patterns and attributes with an empty argument list.
- Parse coverage (every `.swift` file of both subjects, 642): 36 have an ERROR or MISSING node (5.6%: swift-nio 35 of 550, Alamofire 1 of 92),
  over design section 14.2's 5% line, which the coordinator should weigh. Every one is Swift 5.10-6.x syntax the grammar lacks:
  `sending` parameters and results, `isolated (any Actor)?` and `#isolation`, generic value parameters (`<let count: Int>`), a `where` clause
  on a generic `typealias`, `#elseif (compiler(<6.1) && ...)` and `#error(...)`, `case .min..<0:`. The dialect is not refused: a file with an error
  still yields what parses (21,939 of 21,997 definitions found, sym_recall .9974), and the 58 misses are whole types or extensions the recovery
  swallowed (`FileSystem`, `FileSystemProtocol`, `CircularBuffer`).
- `tags.scm`: written in this repository against the grammar's own tree; the grammar's `queries/tags.scm` (MIT) tags a class, a protocol, a
  method, a property and a function and nothing else, with no end line, no `@body`, no callee names and no call patterns, so only its
  `class_declaration` field names were kept. One node, `class_declaration`, is a class, a struct, an enum, an actor and an extension; the
  anonymous `declaration_kind` token says which. A function is a method inside a type body (`class_body` for class, struct, actor and
  extension, `enum_class_body` for enum) and a function at file level or in a block of statements (a local function); an operator function
  (`static func == (l: T, r: T)`) has no simple_identifier, only an anonymous `name:` child, so it is read by a text test that keeps out the
  return type (the other `name:` child). `init`, `deinit`, `subscript` and `macro` are named by their keyword or identifier. A `@body`
  is the function body, the accessor block of a computed property or subscript, or an observer block, so a Signature reads
  `@discardableResult public func configure<T: Hashable>(...) async throws -> [T] where T: Codable`.
- Kinds: SwiftClass, SwiftStruct, SwiftEnum, SwiftActor (canonical class), SwiftExtension (canonical class; named by the extended type, a dotted path
  for a nested one, its generic arguments dropped, `extension [Header]` kept as spelled), SwiftProtocol (interface), SwiftTypeAlias and
  SwiftAssociatedType (type_alias), SwiftEnumCase (enum_member), SwiftFunc, SwiftMethod (operators too), SwiftInit and SwiftDeinit and
  SwiftSubscript (named `init`, `deinit`, `subscript`), SwiftProperty (a member of a type body or a protocol requirement, one per bound name,
  tuple patterns too), SwiftVariable (a file-level `let` or `var`), SwiftMacro. A callable, an enum case, a type alias and a property or variable
  are `callable` so they qualify under their type (`Box.Mode.fast`, `Vector.length`, `Shape.Unit`) and so the calls a property's initialiser,
  accessor or observer makes belong to it. An extension shares its symbol name with the type it extends, and both observations are stored.
- Calls: the callee is the last name of the callee expression (`a?.b!.c(x)` is `c`, `Foo<Int>(x)` and `Foo.Bar<T>(x)` are `Foo` and `Bar`, `.make(x)` is
  `make`), taken only where the arguments open with `(` or are a trailing closure, because the grammar reads a subscript `a[0]` as a call.
  Four grammar quirks are compensated: a prefix operator that binds to the callee (`!items.contains(x)`), an implicit member after an infix operator
  (`now() + .hours(1)`, whose arguments the grammar hangs on the whole sum), `defer { }` and `nonisolated(unsafe)`, which it reads as calls, and a function
  reference that names its argument labels (`handle(_:)`), which it reads as a call and the compiler does not. A pattern in a `case` is left as the grammar
  writes it: `case UInt8(ascii: "a"):` is a call here and a pattern to the compiler's parser (5 pairs on swift-nio).
- Not extracted, on purpose or by grammar limit: an operator declaration (`infix operator <+>`) and a precedence group; a local variable, constant or
  parameter; a dictionary-type extension (`extension [String: Int]`); a computed subscript's or accessor's own definition (a `get` or `set` is no symbol);
  the second and later names of `var x, y: Double` and of a tuple pattern are emitted but unqualified (`y`, not `Vector.y`), because the engine takes a
  definition that shares its range with another as that one's child, which is why each `case a, b` is a definition at its own name and not at the
  whole line. `#if` branches are all parsed, so a declaration in each of two branches is two definitions.
- `imports.scm`: written in this repository. `import Foundation`, `@testable import Core`, `@_exported import Common` and `import Core.Sub` are a whole-module
  import (bound as the module text); `import struct Models.User` (also `class`, `enum`, `protocol`, `typealias`, `func`, `let`, `var`) names one declaration, a
  named import of its last identifier. `imports.strategy: swift-module` resolves `import M` to every Swift file of the SwiftPM target named M (its
  `path:` in Package.swift, else `Sources/M` beside the manifest, else any `Sources/M`), a whole-module import bringing every top-level name of the target
  into scope, and a module with no target in the repository (`Foundation`, `Missing`, a system module) resolves nowhere. A file of one module calling another
  file of the same module needs no import and resolves through nothing; that, and the fact that SourceKit-LSP's index store is not SCIP, is why L3 is not
  measured (`LB-Q-swift-rest`).
- `testdata/`: fixtures written in this repository, each with a golden written by hand from reading the fixture before the queries were finished:
  `types.swift` every type kind, nesting, protocol requirements, extensions, `#if`; `members.swift` methods, initialisers, subscripts, operators, properties
  (stored, computed, observed, lazy, static, tuple, multi-name), a local function, top-level variables, a macro and a multi-line signature with attributes;
  `calls.swift` every call shape, the grammar quirks and the calls a property owns; `imports.swift` each import form; `edge.swift` attributes and property
  wrappers, a protocol extension, unicode names, generic and sugar extension names and global operator functions; `broken.swift` a syntax error
  the parser must survive (`Good`, `Survivor` and `tail` extract on both sides of it). The goldens matched the engine's output entry for entry on the first
  run except two things the first run showed and the hand-written goldens then followed: the second name of a multi-name declaration is unqualified, and an
  enum case is a definition at its own name.
- **Measurement (docs/bench/readings/lang/swift.json, calibration/corpus.d/swift.yaml):** the Swift compiler's own parser, `swiftc -frontend -parse-as-library
  -dump-parse` (Apple Swift 6.4, tier B, `calibration/oracles/swift-parse`), on Alamofire 5.9.1 and swift-nio 2.99.0 (642 files). universal-ctags 6.2.1 has no
  Swift parser and SourceKit-LSP's index store cannot be read as SCIP, so the compiler's parser is the L1 oracle (definitions with the compiler's own ranges: tier
  B, so L1 can be excellent) and the L2 oracle (call pairs, tier B, so L2 is at most good). `level_met` is L2, L1 excellent and L2 good. L3 and L4 are not reached
  (`LB-Q-swift-rest` in `docs/plans/language-breadth.md`).
- Dispatch: Swift has no incumbent, so `fusion.New` registers `.swift` from the manifest like every dialect (no merge gate); every already-indexed `.swift` file was
  disclosed as unsupported ("no extractor registered"), so `ExtractorEpoch` is 27.
