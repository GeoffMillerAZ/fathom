# objective-c — provenance

- Grammar: `tree-sitter-grammars/tree-sitter-objc` at `181a81b8f23a2d593e7ab4259981f50122909fda` (MIT, the
  top-level `LICENSE`), ABI 14, no scanner; built into
  `internal/extractor/tsengine/grammars/objective-c.wasm.gz` (2,361,937 bytes raw, 491,144 bytes gzip embedded).
  Pin and digests in `internal/extractor/tsengine/grammars/grammars.lock.json`
  (`internal/extractor/tsengine/grammars/PROVENANCE.md` regenerates its own tables from it).
- `tags.scm`: written in this repository (LB-Q-objective-c, docs/design/language-breadth.md §13). The grammar's own
  `queries/` carries no tags query, so nothing is drafted from an upstream file; the C shapes (functions, structs,
  unions, enums, typedefs, macros, fields, enumerators, globals) follow `dialects/c/tags.scm` of this repository, and
  the Objective-C shapes are written against the grammar's node types with `indexbench lang-probe`. A method is named
  by its selector: the keyword identifiers of a `method_declaration`, `method_definition` or `message_expression` are
  separate children split by their arguments, so a selector of n keywords is a pattern of n `@name.part` captures
  (one pattern per arity, 1 to 8) that the engine joins, each followed by `:` (`tsNameParts`,
  `internal/extractor/tsextract.go`); a zero-argument selector is a plain `@name`. A method with no `(return type)`
  and a selector of more than eight keywords are not matched.
- `imports.scm`, `references.scm`: written in this repository (the grammar ships no import or reference query).
  `#import` and `#include` are one `preproc_include` node; the manifest's `include-path` strategy resolves the quoted
  form relative to the importer, then the source roots. `@import Module;` is not captured: a module names a framework,
  not a file of the service.
- Pre-parse step `objc-macros` (`internal/extractor/tsextract_objcmacros.go`): `typedef NS_ENUM(T, Name)` and its
  siblings read as `enum Name`, availability and export macros are blanked where they decorate a declaration, at
  equal byte length, taken only when the file then parses with no error.
- Kinds: `ObjCInterface` (an `@interface` or class extension), `ObjCImplementation`, `ObjCCategory` (spans its own
  name only, so its methods qualify under the class), `ObjCProtocol`, `ObjCMethod`, `ObjCClassMethod`,
  `ObjCProperty`, `ObjCIvar`, and the C kinds `ObjCFunc`, `ObjCStruct`, `ObjCUnion`, `ObjCEnum`, `ObjCTypedef`,
  `ObjCMacro`, `ObjCField`, `ObjCEnumerator`, `ObjCVariable`.
- `package.strategy: file`: an `@interface` in `Shape.h` and its `@implementation` in `Shape.m` are one class named
  twice, so a package of the directory would give both one symbol key and a file edge to the class would land on
  whichever file the key was last read from. A package of the file's name keeps the declaration in the header the
  `#import` names apart from the definition in the source.
- `name_match: selector`: the harness compares the part of a name after the owner's `.`, whole, because a selector
  ends in `:`.
- Dispatch: `.mm` by extension; `.m` (MATLAB's too) by the manifest's `match.sniff`
  (`^\s*(@(interface|protocol|implementation|class|import)\b|#import\b)`, priority 0, before MATLAB's 50); `.h` by the
  header router (`extractor.HeaderDialect`, `cheaderextract.go`), never a second classifier.
