; dialects/objective-c/references.scm — the file resolver's references (LB-F74,
; vocabulary in internal/extractor/tsrefs.go), compiled apart from tags.scm and
; imports.scm once the manifest names it (queries.references). Written in this
; repository against tree-sitter-objc's grammar; no upstream query.
;
; What an Objective-C file names without sending it a message, and a header can
; define:
;   - every type name: a typedef name, a `struct x` tag, a class or protocol
;     written as a type (NSArray<Shape *> *, Shape *s, id<Drawable>);
;   - a class a message is sent to: `[Shape shapeWithName:@"a"]` (an
;     upper-case receiver, the class-name convention; a lower-case one is a
;     variable);
;   - a superclass, a protocol in a conformance list, the class a category or
;     extension extends, and a forward declaration (`@class Shape;`);
;   - an upper-case word (a macro, an enumerator or a constant by the all-capitals
;     convention) where it is read or invoked.
; The resolver binds a name only to a definition in a header the file reaches
; through its #import and #include graph, and never to a name the file defines
; itself. A lower-case word is read as no reference: written bare it is mostly a
; local, a parameter or a property, which binds nothing; that gap is disclosed
; in the reading, not guessed.
(type_identifier) @ref.name

(message_expression
  receiver: (identifier) @ref.name
  (#match? @ref.name "^[A-Z]"))

(class_interface
  superclass: (identifier) @ref.name)

(class_interface
  . (identifier) @ref.name
  category: (identifier))

(class_implementation
  . (identifier) @ref.name
  category: (identifier))

(protocol_reference_list
  (identifier) @ref.name)

(class_declaration
  (identifier) @ref.name)

((identifier) @ref.name
  (#match? @ref.name "^[A-Z][A-Z0-9_]*$"))
