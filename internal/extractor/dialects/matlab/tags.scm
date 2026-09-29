; source: acristoffers/tree-sitter-matlab queries/neovim/tags.scm @ f03d0347acd8bb05d4edd8c845ac1718729e1fad (MIT)
; changes: the upstream file tags a class, every function and every call; this one tags classdef, its properties and
; methods (a function in a `methods` block is a method), a function at file level or nested in another function's block,
; and a `methods` signature without a body (an abstract method). It captures no call: a MATLAB call `f(x)` and an index
; `f(x)` are the same syntax, so the tags dialect leaves calls to L2.
;
; A callable's signature is read from its start to the start of its @body capture, so @body is the first node after the
; header: a leading help comment, else the block, else nothing (the signature ends at the first line). A pattern that
; anchors with `.` after an optional argument list is not exclusive when the list is absent (a function without
; parentheses matched once per leading comment and once more for its block), so each function context spells out the six
; header shapes — arguments or not, then comment, block or neither — every one anchored between nodes that are there.

; --- classdef ---

(class_definition
  name: (identifier) @name) @definition.class

; --- properties (one per `property`, in any `properties` block) ---

(properties
  (property
    name: (identifier) @name) @definition.property)

; --- a function at file level: the main function and its local functions ---

(source_file
  (function_definition
    name: (identifier) @name
    .
    (function_arguments)
    .
    (comment) @body) @definition.function)

(source_file
  (function_definition
    name: (identifier) @name
    .
    (function_arguments)
    .
    (block) @body) @definition.function)

(source_file
  (function_definition
    name: (identifier) @name
    .
    (function_arguments)
    .) @definition.function)

(source_file
  (function_definition
    name: (identifier) @name
    .
    (comment) @body) @definition.function)

(source_file
  (function_definition
    name: (identifier) @name
    .
    (block) @body) @definition.function)

(source_file
  (function_definition
    name: (identifier) @name
    .) @definition.function)

; --- a function nested in another function ---

(block
  (function_definition
    name: (identifier) @name
    .
    (function_arguments)
    .
    (comment) @body) @definition.function)

(block
  (function_definition
    name: (identifier) @name
    .
    (function_arguments)
    .
    (block) @body) @definition.function)

(block
  (function_definition
    name: (identifier) @name
    .
    (function_arguments)
    .) @definition.function)

(block
  (function_definition
    name: (identifier) @name
    .
    (comment) @body) @definition.function)

(block
  (function_definition
    name: (identifier) @name
    .
    (block) @body) @definition.function)

(block
  (function_definition
    name: (identifier) @name
    .) @definition.function)

; --- a method: a function in a `methods` block ---

(methods
  (function_definition
    name: (identifier) @name
    .
    (function_arguments)
    .
    (comment) @body) @definition.method)

(methods
  (function_definition
    name: (identifier) @name
    .
    (function_arguments)
    .
    (block) @body) @definition.method)

(methods
  (function_definition
    name: (identifier) @name
    .
    (function_arguments)
    .) @definition.method)

(methods
  (function_definition
    name: (identifier) @name
    .
    (comment) @body) @definition.method)

(methods
  (function_definition
    name: (identifier) @name
    .
    (block) @body) @definition.method)

(methods
  (function_definition
    name: (identifier) @name
    .) @definition.method)

; --- an abstract or externally defined method: a signature with no body ---

(methods
  (function_signature
    name: (identifier) @name) @definition.method)
